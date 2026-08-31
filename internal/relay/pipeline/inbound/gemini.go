package inbound

import (
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/zicorn/llm-proxy/internal/relay/pipeline"
	"github.com/zicorn/llm-proxy/internal/relay/wireformat"
)

// geminiModelActionRe 匹配 Gemini 路径中的 /models/{model}:{action} 段。
var geminiModelActionRe = regexp.MustCompile(`/models/([^/:?]+)(?::([^/?]+))?`)

// geminiActionKinds 列出各原生操作的处置方式。
//
// 未列出的操作按 KindMetadata 处理（模型列表等只读接口）。会产生 token 用量的
// 操作必须显式列为 KindGenerate 或 KindUnsupported，绝不能落到 KindMetadata，
// 否则就是静默漏计费。
var geminiActionKinds = map[string]pipeline.Kind{
	"generateContent":       pipeline.KindGenerate,
	"streamGenerateContent": pipeline.KindGenerate,
	"countTokens":           pipeline.KindMetadata,
	// 嵌入接口的上游路径无法由渠道适配器构造（适配器只会生成
	// batchEmbedContents，与入站的 embedContent 请求体结构不同），
	// 拒绝而非放行，避免无法计费的请求通过。
	"embedContent":       pipeline.KindUnsupported,
	"batchEmbedContents": pipeline.KindUnsupported,
}

func resolveGemini(c *gin.Context) (*pipeline.Operation, error) {
	op := &pipeline.Operation{InboundWire: wireformat.Gemini, Kind: pipeline.KindMetadata}

	m := geminiModelActionRe.FindStringSubmatch(c.Request.URL.Path)
	if m == nil {
		return op, nil
	}
	op.Model = m[1]
	op.Action = m[2]
	if op.Action == "" {
		return op, nil
	}
	if kind, known := geminiActionKinds[op.Action]; known {
		op.Kind = kind
	}
	op.IsStream = strings.HasPrefix(op.Action, "stream")
	return op, nil
}

func init() {
	pipeline.Register(&pipeline.RelaySpec{
		Name:       "gemini.native",
		Mode:       pipeline.ModePassthrough,
		PathPrefix: "/gemini",
		Resolve:    resolveGemini,
	})
}
