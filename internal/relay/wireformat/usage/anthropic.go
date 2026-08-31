package usage

import (
	"bytes"
	"encoding/json"

	"github.com/zicorn/llm-proxy/internal/relay/entity"
)

// Anthropic 解析 Anthropic API、Vertex-Claude 与 Bedrock-Claude 响应中的 usage。
func Anthropic(body []byte, isStream bool) (*entity.Usage, bool) {
	if !isStream {
		var r struct {
			Usage struct {
				InputTokens          int `json:"input_tokens"`
				OutputTokens         int `json:"output_tokens"`
				CacheReadInputTokens int `json:"cache_read_input_tokens"`
			} `json:"usage"`
		}
		if err := json.Unmarshal(body, &r); err == nil && (r.Usage.InputTokens > 0 || r.Usage.OutputTokens > 0) {
			return buildUsage(r.Usage.InputTokens, r.Usage.OutputTokens, r.Usage.CacheReadInputTokens), true
		}
		return nil, false
	}
	// 流式下 input_tokens 只出现在 message_start，output_tokens 只出现在 message_delta
	var input, output, cached int
	for _, line := range bytes.Split(body, []byte("\n")) {
		payload, isData := sseData(line)
		if !isData {
			continue
		}
		var ev struct {
			Type    string `json:"type"`
			Message struct {
				Usage struct {
					InputTokens          int `json:"input_tokens"`
					CacheReadInputTokens int `json:"cache_read_input_tokens"`
				} `json:"usage"`
			} `json:"message"`
			Usage struct {
				OutputTokens int `json:"output_tokens"`
			} `json:"usage"`
		}
		if err := json.Unmarshal(payload, &ev); err != nil {
			continue
		}
		switch ev.Type {
		case "message_start":
			input = ev.Message.Usage.InputTokens
			cached = ev.Message.Usage.CacheReadInputTokens
		case "message_delta":
			if ev.Usage.OutputTokens > 0 {
				output = ev.Usage.OutputTokens
			}
		}
	}
	if input == 0 && output == 0 {
		return nil, false
	}
	return buildUsage(input, output, cached), true
}

func buildUsage(prompt, completion, cached int) *entity.Usage {
	u := &entity.Usage{
		PromptTokens:     prompt,
		CompletionTokens: completion,
		TotalTokens:      prompt + completion,
	}
	u.PromptTokensDetails.CachedTokens = cached
	return u
}
