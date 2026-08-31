package controller

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/zicorn/llm-proxy/internal/objects"
	"github.com/zicorn/llm-proxy/internal/relay/channeltype"
	"github.com/zicorn/llm-proxy/internal/relay/nativeformat"
	"github.com/zicorn/llm-proxy/pkg/common/client"
	"github.com/zicorn/llm-proxy/pkg/common/ctxkey"
	"github.com/zicorn/llm-proxy/pkg/common/logger"
)

// RelayNativeHelper 以协议原生透传方式转发请求，同时完成计费和日志记录。
// 客户端使用原生 SDK（如 Anthropic SDK、Google GenAI SDK）时，
// 只需把 base_url 指向本代理的对应前缀（/anthropic、/gemini、/vertexai），
// 代理负责：① 替换鉴权  ② 还原上游路径  ③ 透明转发请求体和响应体  ④ 解析 token 用量并扣费。
func RelayNativeHelper(c *gin.Context, format string) *objects.ErrorWithStatusCode {
	ctx := c.Request.Context()
	meta := objects.GetRequestMeta(c)

	// ── 从 context 中取渠道信息（已由 Distribute 中间件设置）────────────────────
	baseURL := c.GetString(ctxkey.BaseURL)
	if baseURL == "" {
		channelType := c.GetInt(ctxkey.ChannelType)
		if channelType > 0 && channelType < len(channeltype.ChannelBaseURLs) {
			baseURL = channeltype.ChannelBaseURLs[channelType]
		}
	}
	apiKey := strings.TrimPrefix(c.Request.Header.Get("Authorization"), "Bearer ")

	// ── 构建上游路径 ─────────────────────────────────────────────────────────────
	upstreamPath := stripFormatPrefix(c.Request.URL.Path, format)
	if c.Request.URL.RawQuery != "" {
		upstreamPath = upstreamPath + "?" + c.Request.URL.RawQuery
	}

	// ── 提取模型名：Gemini 从 URL，Anthropic 需要先缓冲请求 body ─────────────────
	modelName := extractModelFromNativePath(upstreamPath, format)

	var reqBody io.Reader = c.Request.Body
	if modelName == "" && format == nativeformat.FormatAnthropic {
		reqBodyBytes, err := io.ReadAll(c.Request.Body)
		if err == nil {
			modelName = extractModelFromRequestBody(reqBodyBytes, format)
			reqBody = bytes.NewReader(reqBodyBytes)
		}
	}
	meta.ActualModelName = modelName
	meta.OriginModelName = modelName

	// ── 构建上游 URL ──────────────────────────────────────────────────────────────
	upstreamURL, err := buildUpstreamURL(baseURL, upstreamPath, format, apiKey)
	if err != nil {
		return objects.ErrorWrapper(err, "build_url_failed", http.StatusInternalServerError)
	}
	logger.Infof(ctx, "[native] upstream url: %s", upstreamURL)

	// ── 构建上游请求 ──────────────────────────────────────────────────────────────
	req, err := http.NewRequestWithContext(ctx, c.Request.Method, upstreamURL, reqBody)
	if err != nil {
		return objects.ErrorWrapper(err, "new_request_failed", http.StatusInternalServerError)
	}
	copyAndReplaceHeaders(c, req, format, apiKey)

	// ── 执行请求 ──────────────────────────────────────────────────────────────────
	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return objects.ErrorWrapper(fmt.Errorf("upstream request failed: %w", err), "do_request_failed", http.StatusBadGateway)
	}
	defer resp.Body.Close()

	// ── 将响应头写给客户端 ────────────────────────────────────────────────────────
	for k, vs := range resp.Header {
		for _, v := range vs {
			c.Writer.Header().Add(k, v)
		}
	}
	c.Writer.WriteHeader(resp.StatusCode)

	// ── TeeReader：同时流式返回响应并缓冲用于计费解析 ─────────────────────────────
	var respBuf bytes.Buffer
	tee := io.TeeReader(resp.Body, &respBuf)
	if _, err := io.Copy(c.Writer, tee); err != nil {
		logger.Warnf(ctx, "[native] copy response body failed: %v", err)
	}

	// ── 请求成功后异步计费 ────────────────────────────────────────────────────────
	if nativeStatusOK(resp.StatusCode) {
		respBytes := respBuf.Bytes()
		metaCopy := *meta // 浅拷贝，避免 goroutine 持有 gin.Context
		go func() {
			usage := parseNativeResponseUsage(respBytes, format)
			postNativeBilling(ctx, &metaCopy, usage)
		}()
	}

	return nil
}

// stripFormatPrefix 从请求路径中去掉格式前缀。
// 例：/anthropic/v1/messages → /v1/messages
func stripFormatPrefix(path, format string) string {
	prefix := "/" + format
	trimmed := strings.TrimPrefix(path, prefix)
	if trimmed == "" {
		trimmed = "/"
	}
	return trimmed
}

// buildUpstreamURL 根据格式差异构建上游 URL，并在必要时追加 API Key。
func buildUpstreamURL(baseURL, upstreamPath, format, apiKey string) (string, error) {
	baseURL = strings.TrimRight(baseURL, "/")
	switch format {
	case nativeformat.FormatGoogle, nativeformat.FormatVertexAI:
		// Google Gemini / Vertex AI：API Key 以 ?key= 查询参数传递
		url := baseURL + upstreamPath
		if apiKey != "" {
			if strings.Contains(url, "?") {
				url += "&key=" + apiKey
			} else {
				url += "?key=" + apiKey
			}
		}
		return url, nil
	default:
		// Anthropic 等：API Key 通过请求头传递
		return baseURL + upstreamPath, nil
	}
}

// copyAndReplaceHeaders 复制客户端请求头并替换/补充鉴权头。
func copyAndReplaceHeaders(c *gin.Context, req *http.Request, format, apiKey string) {
	skipHeaders := map[string]bool{
		"authorization": true,
		"x-api-key":     true,
		"host":          true,
	}

	for k, vs := range c.Request.Header {
		if skipHeaders[strings.ToLower(k)] {
			continue
		}
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}

	switch format {
	case nativeformat.FormatAnthropic:
		if apiKey != "" {
			req.Header.Set("x-api-key", apiKey)
		}
		if req.Header.Get("anthropic-version") == "" {
			req.Header.Set("anthropic-version", "2023-06-01")
		}
	case nativeformat.FormatGoogle, nativeformat.FormatVertexAI:
		if format == nativeformat.FormatVertexAI && apiKey != "" {
			req.Header.Set("Authorization", "Bearer "+apiKey)
		}
	default:
		if apiKey != "" {
			req.Header.Set("Authorization", "Bearer "+apiKey)
		}
	}
}
