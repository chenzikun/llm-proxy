package controller

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/zicorn/llm-proxy/pkg/common/client"
	"github.com/zicorn/llm-proxy/pkg/common/ctxkey"
	"github.com/zicorn/llm-proxy/pkg/common/logger"
	"github.com/zicorn/llm-proxy/internal/objects"
	"github.com/zicorn/llm-proxy/internal/relay/nativeformat"
)

// RelayNativeHelper 以协议原生透传方式转发请求。
// 客户端使用原生 SDK（如 Anthropic SDK、Google GenAI SDK）时，
// 只需把 base_url 指向本代理的对应前缀（/anthropic、/google、/vertexai），
// 代理负责：① 替换鉴权  ② 还原上游路径  ③ 透明转发请求体和响应体。
func RelayNativeHelper(c *gin.Context, format string) *objects.ErrorWithStatusCode {
	ctx := c.Request.Context()

	// 从 context 中取渠道信息（已由 Distribute 中间件设置）
	baseURL := c.GetString(ctxkey.BaseURL)
	apiKey := strings.TrimPrefix(c.Request.Header.Get("Authorization"), "Bearer ")

	// 构建上游 URL：去掉格式前缀，拼接渠道 BaseURL
	upstreamPath := stripFormatPrefix(c.Request.URL.Path, format)
	// 保留原始 query string（如 Google 的 ?alt=sse）
	if c.Request.URL.RawQuery != "" {
		upstreamPath = upstreamPath + "?" + c.Request.URL.RawQuery
	}

	upstreamURL, err := buildUpstreamURL(baseURL, upstreamPath, format, apiKey)
	if err != nil {
		return objects.ErrorWrapper(err, "build_url_failed", http.StatusInternalServerError)
	}
	logger.Infof(ctx, "[native] upstream url: %s", upstreamURL)

	// 构建上游请求
	req, err := http.NewRequestWithContext(ctx, c.Request.Method, upstreamURL, c.Request.Body)
	if err != nil {
		return objects.ErrorWrapper(err, "new_request_failed", http.StatusInternalServerError)
	}

	// 复制请求头，替换鉴权
	copyAndReplaceHeaders(c, req, format, apiKey)

	// 执行请求
	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return objects.ErrorWrapper(fmt.Errorf("upstream request failed: %w", err), "do_request_failed", http.StatusBadGateway)
	}
	defer resp.Body.Close()

	// 将响应原样返回给客户端
	for k, vs := range resp.Header {
		for _, v := range vs {
			c.Writer.Header().Add(k, v)
		}
	}
	c.Writer.WriteHeader(resp.StatusCode)
	if _, err := io.Copy(c.Writer, resp.Body); err != nil {
		logger.Warnf(ctx, "[native] copy response body failed: %v", err)
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
	// 需要跳过的 hop-by-hop 头
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

	// 根据格式设置上游鉴权头
	switch format {
	case nativeformat.FormatAnthropic:
		if apiKey != "" {
			req.Header.Set("x-api-key", apiKey)
		}
		// 确保 anthropic-version 存在
		if req.Header.Get("anthropic-version") == "" {
			req.Header.Set("anthropic-version", "2023-06-01")
		}
	case nativeformat.FormatGoogle, nativeformat.FormatVertexAI:
		// API Key 已附加到 URL 查询参数，不需要 Authorization 头
		// 如果是 Vertex AI OAuth 模式，保留原始 Bearer token
		if format == nativeformat.FormatVertexAI && apiKey != "" {
			req.Header.Set("Authorization", "Bearer "+apiKey)
		}
	default:
		if apiKey != "" {
			req.Header.Set("Authorization", "Bearer "+apiKey)
		}
	}
}
