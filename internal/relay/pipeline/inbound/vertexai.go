package inbound

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/zicorn/llm-proxy/internal/relay/pipeline"
	"github.com/zicorn/llm-proxy/internal/relay/wireformat"
)

// vertexActionKinds 列出 Vertex AI 的原生操作。
// rawPredict 系列是 Anthropic 模型在 Vertex 上的调用入口。
var vertexActionKinds = map[string]pipeline.Kind{
	"generateContent":       pipeline.KindGenerate,
	"streamGenerateContent": pipeline.KindGenerate,
	"rawPredict":            pipeline.KindGenerate,
	"streamRawPredict":      pipeline.KindGenerate,
	"countTokens":           pipeline.KindMetadata,
}

// resolveVertexAI 解析 /vertexai 前缀的请求。
//
// 与 /gemini 的区别在于 wire 格式随模型变化：Vertex 同时托管 Gemini 与 Claude，
// gemini-* 的请求体是 Gemini 格式，claude-* 是 Anthropic 格式。这里按模型判定，
// 与 wireformat.Resolve 对上游的判定使用同一个谓词。
func resolveVertexAI(c *gin.Context) (*pipeline.Operation, error) {
	op := &pipeline.Operation{
		Kind:       pipeline.KindMetadata,
		APIVersion: inboundAPIVersion(c.Request.URL.Path),
	}

	m := geminiModelActionRe.FindStringSubmatch(c.Request.URL.Path)
	if m == nil {
		op.InboundWire = wireformat.Unspecified
		return op, nil
	}
	op.Model = m[1]
	op.Action = m[2]

	if wireformat.IsVertexGeminiModel(op.Model) {
		op.InboundWire = wireformat.Gemini
	} else {
		op.InboundWire = wireformat.Anthropic
	}

	if op.Action == "" {
		return op, nil
	}
	if kind, known := vertexActionKinds[op.Action]; known {
		op.Kind = kind
	}
	op.IsStream = strings.HasPrefix(op.Action, "stream")
	return op, nil
}

func init() {
	pipeline.Register(&pipeline.RelaySpec{
		Name:       "vertexai.native",
		Mode:       pipeline.ModePassthrough,
		PathPrefix: "/vertexai",
		Resolve:    resolveVertexAI,
	})
}
