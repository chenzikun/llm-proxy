package usage

import (
	"github.com/zicorn/llm-proxy/internal/relay/entity"
	"github.com/zicorn/llm-proxy/internal/relay/wireformat"
)

// Extractor 从上游原始响应中提取 token 用量。
//
// body 为完整响应体（非流式）或累积的 SSE 文本（流式）。
// 第二个返回值为 false 表示解析失败，调用方应记异常日志并按 0 结算。
type Extractor func(body []byte, isStream bool) (*entity.Usage, bool)

var registry = map[wireformat.Format]Extractor{
	wireformat.OpenAI:    OpenAI,
	wireformat.Gemini:    Gemini,
	wireformat.Anthropic: Anthropic,
}

// Get 返回该 wire 格式的提取器，无对应实现时返回 nil。
func Get(f wireformat.Format) Extractor {
	return registry[f]
}
