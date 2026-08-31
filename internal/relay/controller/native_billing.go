package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"net/http"
	"regexp"
	"strings"

	"github.com/zicorn/llm-proxy/internal/objects"
	model "github.com/zicorn/llm-proxy/internal/repo"
	billingratio "github.com/zicorn/llm-proxy/internal/relay/billing/ratio"
	"github.com/zicorn/llm-proxy/internal/relay/nativeformat"
	"github.com/zicorn/llm-proxy/pkg/common/config"
	"github.com/zicorn/llm-proxy/pkg/common/logger"
)

// nativeUsage holds token usage extracted from a native API response.
type nativeUsage struct {
	PromptTokens     int
	CompletionTokens int
}

var geminiModelPathRe = regexp.MustCompile(`/models/([^/:?]+)`)

// extractModelFromNativePath extracts the model name from the upstream path (format prefix already stripped).
//
//	Gemini:   /v1beta/models/gemini-2.5-flash:generateContent  → gemini-2.5-flash
//	Anthropic: model is in the request body, not the URL        → ""
func extractModelFromNativePath(upstreamPath, format string) string {
	switch format {
	case nativeformat.FormatGoogle, nativeformat.FormatVertexAI:
		m := geminiModelPathRe.FindStringSubmatch(upstreamPath)
		if len(m) >= 2 {
			model := m[1]
			// strip action suffix like :generateContent, :streamGenerateContent
			if idx := strings.Index(model, ":"); idx >= 0 {
				model = model[:idx]
			}
			return model
		}
	}
	return ""
}

// extractModelFromRequestBody parses the "model" field from a JSON request body.
// Used for Anthropic, where the model is specified in the body rather than the URL.
func extractModelFromRequestBody(body []byte, format string) string {
	if format != nativeformat.FormatAnthropic {
		return ""
	}
	var req struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal(body, &req); err == nil {
		return req.Model
	}
	return ""
}

// parseNativeResponseUsage extracts token usage from a native API response body.
// body may be raw JSON (non-streaming) or accumulated SSE lines (streaming).
func parseNativeResponseUsage(body []byte, format string) nativeUsage {
	switch format {
	case nativeformat.FormatGoogle, nativeformat.FormatVertexAI:
		return parseGeminiUsage(body)
	case nativeformat.FormatAnthropic:
		return parseAnthropicUsage(body)
	}
	return nativeUsage{}
}

// parseGeminiUsage parses usageMetadata from a Gemini response (streaming or non-streaming).
func parseGeminiUsage(body []byte) nativeUsage {
	type usageMeta struct {
		UsageMetadata struct {
			PromptTokenCount     int `json:"promptTokenCount"`
			CandidatesTokenCount int `json:"candidatesTokenCount"`
		} `json:"usageMetadata"`
	}

	// Non-streaming: try direct unmarshal
	var resp usageMeta
	if err := json.Unmarshal(body, &resp); err == nil && resp.UsageMetadata.PromptTokenCount > 0 {
		return nativeUsage{
			PromptTokens:     resp.UsageMetadata.PromptTokenCount,
			CompletionTokens: resp.UsageMetadata.CandidatesTokenCount,
		}
	}

	// Streaming SSE: scan lines, keep the last chunk that has usageMetadata
	lastUsage := nativeUsage{}
	for _, line := range bytes.Split(body, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if !bytes.HasPrefix(line, []byte("data: ")) {
			continue
		}
		jsonPart := bytes.TrimPrefix(line, []byte("data: "))
		var chunk usageMeta
		if err := json.Unmarshal(jsonPart, &chunk); err == nil && chunk.UsageMetadata.PromptTokenCount > 0 {
			lastUsage = nativeUsage{
				PromptTokens:     chunk.UsageMetadata.PromptTokenCount,
				CompletionTokens: chunk.UsageMetadata.CandidatesTokenCount,
			}
		}
	}
	return lastUsage
}

// parseAnthropicUsage parses usage from an Anthropic response (streaming or non-streaming).
func parseAnthropicUsage(body []byte) nativeUsage {
	// Non-streaming: single JSON object with usage field
	var resp struct {
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &resp); err == nil && resp.Usage.InputTokens > 0 {
		return nativeUsage{
			PromptTokens:     resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
		}
	}

	// Streaming SSE: parse message_start (input tokens) and message_delta (output tokens)
	inputTokens := 0
	outputTokens := 0
	for _, line := range bytes.Split(body, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if !bytes.HasPrefix(line, []byte("data: ")) {
			continue
		}
		jsonPart := bytes.TrimPrefix(line, []byte("data: "))
		var event struct {
			Type    string `json:"type"`
			Message struct {
				Usage struct {
					InputTokens int `json:"input_tokens"`
				} `json:"usage"`
			} `json:"message"`
			Usage struct {
				OutputTokens int `json:"output_tokens"`
			} `json:"usage"`
		}
		if err := json.Unmarshal(jsonPart, &event); err != nil {
			continue
		}
		switch event.Type {
		case "message_start":
			inputTokens = event.Message.Usage.InputTokens
		case "message_delta":
			if event.Usage.OutputTokens > 0 {
				outputTokens = event.Usage.OutputTokens
			}
		}
	}
	if inputTokens > 0 || outputTokens > 0 {
		return nativeUsage{PromptTokens: inputTokens, CompletionTokens: outputTokens}
	}
	return nativeUsage{}
}

// postNativeBilling deducts quota and records a consume log after a native relay request.
// It is designed to be called asynchronously (go postNativeBilling(...)).
func postNativeBilling(ctx context.Context, meta *objects.Meta, usage nativeUsage) {
	if meta.ActualModelName == "" {
		// 无法确定模型，记录一条不含扣费的系统日志
		model.RecordLog(meta.UserId, model.LogTypeSystem, "[native relay] model name unknown, billing skipped")
		return
	}

	modelMeta, err := model.GetModelMetaByModel(meta.ActualModelName)
	if err != nil {
		logger.Warnf(ctx, "[native billing] model meta not found for %q, billing skipped: %v", meta.ActualModelName, err)
		return
	}

	rate := 1.0
	if modelMeta.PriceUnit == "USD" {
		rate = config.ExchangeRate
	}
	inputPriceCNY := modelMeta.InputPrice * rate
	outputPriceCNY := modelMeta.OutputPrice * rate

	groupRatio := billingratio.GetGroupRatio(meta.Group)
	inputRatio := inputPriceCNY * config.QuotaPerUnit / 1_000_000.0 * groupRatio
	outputRatio := outputPriceCNY * config.QuotaPerUnit / 1_000_000.0 * groupRatio

	quota := int64(math.Ceil(float64(usage.PromptTokens)*inputRatio)) +
		int64(math.Ceil(float64(usage.CompletionTokens)*outputRatio))
	if quota <= 0 && (usage.PromptTokens+usage.CompletionTokens) > 0 {
		quota = 1
	}

	logContent := "[native] " + meta.ActualModelName

	if err := objects.PostCost(ctx, meta, 0, quota,
		usage.PromptTokens, usage.CompletionTokens, 0, 0, logContent); err != nil {
		logger.Errorf(ctx, "[native billing] PostCost failed: %v", err)
	}
}

// nativeStatusOK reports whether the HTTP status should trigger billing.
func nativeStatusOK(statusCode int) bool {
	return statusCode == http.StatusOK
}
