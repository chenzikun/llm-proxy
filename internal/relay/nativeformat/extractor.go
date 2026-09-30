package nativeformat

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/zicorn/llm-proxy/pkg/common"
)

// modelRequest 通用 model 字段（适用于 Anthropic / OpenAI 请求体）
type modelRequest struct {
	Model string `json:"model"`
}

// GetModelFromRequest 根据输入格式从请求中提取 model 名称。
// Google 和 VertexAI 的 model 在 URL 路径中，其余从请求体 JSON 提取。
func GetModelFromRequest(c *gin.Context, format string) (string, error) {
	switch format {
	case FormatGoogle:
		return extractModelFromGooglePath(c.Request.URL.Path)
	case FormatVertexAI:
		return extractModelFromVertexAIPath(c.Request.URL.Path)
	case FormatSeedance:
		return extractModelFromSeedance(c)
	default:
		// Anthropic / OpenAI: model 在请求体 {"model": "..."}
		return extractModelFromBody(c)
	}
}

// extractModelFromBody 从 JSON 请求体中读取 model 字段，支持 body 复用。
func extractModelFromBody(c *gin.Context) (string, error) {
	var req modelRequest
	if err := common.UnmarshalBodyReusable(c, &req); err != nil {
		return "", fmt.Errorf("failed to parse request body: %w", err)
	}
	if req.Model == "" {
		return "", fmt.Errorf("model field is required")
	}
	return req.Model, nil
}

// extractModelFromSeedance 从 Seedance 请求中取模型名。
//
// 创建任务的模型在请求体里（与 Anthropic 同形）；查询任务的请求体是空的，
// 上游的响应里也不回模型名，所以查询时必须由客户端在查询串上带 ?model=。
// 两条路都不通就报错 —— 让 Distribute 随便挑一个同组渠道，会把请求发到
// 一个可能并不持有该任务的账号上，而错误信息里看不出这一点。
func extractModelFromSeedance(c *gin.Context) (string, error) {
	if c.Request.Body != nil {
		var req modelRequest
		if err := common.UnmarshalBodyReusable(c, &req); err == nil && req.Model != "" {
			return req.Model, nil
		}
	}
	if m := c.Query(SeedanceModelQueryKey); m != "" {
		return m, nil
	}
	return "", fmt.Errorf("缺少模型名：创建任务需在请求体带 model，查询任务需在查询串带 ?model=")
}

// extractModelFromGooglePath 从 Google Gemini URL 路径提取 model。
// 路径格式：/gemini/v1beta/models/{model}:generateContent
//           /gemini/v1/models/{model}:generateContent
func extractModelFromGooglePath(path string) (string, error) {
	// 去掉 /gemini 前缀
	path = strings.TrimPrefix(path, "/gemini")
	return extractModelFromModelsPath(path)
}

// extractModelFromVertexAIPath 从 Vertex AI URL 路径提取 model。
// 路径格式（两种）：
//   /vertexai/v1/projects/{proj}/locations/{loc}/publishers/{pub}/models/{model}:{action}
//   /vertexai/{proj}/locations/{loc}/publishers/{pub}/models/{model}:{action}
func extractModelFromVertexAIPath(path string) (string, error) {
	path = strings.TrimPrefix(path, "/vertexai")
	return extractModelFromModelsPath(path)
}

// extractModelFromModelsPath 从形如 .../models/{model}[:{action}] 的路径提取 model。
func extractModelFromModelsPath(path string) (string, error) {
	const marker = "/models/"
	idx := strings.Index(path, marker)
	if idx == -1 {
		return "", fmt.Errorf("cannot extract model from path: %s", path)
	}
	rest := path[idx+len(marker):]
	// 去掉 :{action} 后缀（如 :generateContent）
	if colonIdx := strings.Index(rest, ":"); colonIdx != -1 {
		rest = rest[:colonIdx]
	}
	// 去掉尾部斜杠
	rest = strings.TrimSuffix(rest, "/")
	if rest == "" {
		return "", fmt.Errorf("empty model name in path: %s", path)
	}
	return rest, nil
}

// GetNativeRequestStream 从 Anthropic / 原生格式请求体中判断是否为流式请求。
func GetNativeRequestStream(c *gin.Context, format string) bool {
	switch format {
	case FormatAnthropic:
		body, err := common.GetRequestBody(c)
		if err != nil {
			return false
		}
		var req struct {
			Stream bool `json:"stream"`
		}
		_ = json.Unmarshal(body, &req)
		return req.Stream
	case FormatGoogle, FormatVertexAI:
		// Google 用 `:streamGenerateContent` 区分
		return strings.Contains(c.Request.URL.Path, "streamGenerateContent")
	default:
		return false
	}
}
