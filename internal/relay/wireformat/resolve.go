package wireformat

import (
	"strings"

	"github.com/zicorn/llm-proxy/internal/relay/apitype"
	"github.com/zicorn/llm-proxy/internal/relay/channeltype"
)

// IsVertexGeminiModel 判断 Vertex AI 上的模型走哪种 wire 格式。
//
// 判定条件必须与 vertexai.Adaptor.GetRequestURL 一致：gemini 与 endpoints
// 前缀走 :generateContent（Gemini 格式），其余走 :rawPredict（Anthropic 格式）。
// 两处不一致会导致 URL 与响应解析器错配。
func IsVertexGeminiModel(model string) bool {
	return strings.HasPrefix(model, "gemini") || strings.HasPrefix(model, "endpoints")
}

// Resolve 由渠道类型与模型名解析上游 wire 格式。
//
// 模型名参与判定是因为聚合型渠道按模型托管不同厂商的 API：Vertex AI 上
// gemini-* 是 Gemini 格式，claude-* 是 Anthropic 格式。
func Resolve(channelType int, model string) Format {
	switch channeltype.ToAPIType(channelType) {
	case apitype.OpenAI:
		return OpenAI
	case apitype.Gemini:
		return Gemini
	case apitype.Anthropic, apitype.AwsClaude:
		return Anthropic
	case apitype.VertexAI:
		if IsVertexGeminiModel(model) {
			return Gemini
		}
		return Anthropic
	default:
		return Unknown
	}
}
