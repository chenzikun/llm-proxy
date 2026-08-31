package inbound

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/zicorn/llm-proxy/internal/relay/pipeline"
	"github.com/zicorn/llm-proxy/internal/relay/wireformat"
	"github.com/zicorn/llm-proxy/pkg/common"
	"github.com/zicorn/llm-proxy/pkg/common/ctxkey"
)

// resolveAnthropic 解析 /anthropic 前缀的请求。
//
// 与 Gemini 不同，Anthropic 的模型名在请求体里而非 URL 上，需要读 body。
func resolveAnthropic(c *gin.Context) (*pipeline.Operation, error) {
	op := &pipeline.Operation{InboundWire: wireformat.Anthropic, Kind: pipeline.KindMetadata}
	path := c.Request.URL.Path

	// count_tokens 只估算 token 数，不产生用量
	if strings.Contains(path, "count_tokens") {
		op.Action = "count_tokens"
		return op, nil
	}
	if !strings.Contains(path, "/messages") {
		return op, nil
	}

	op.Action = "messages"
	op.Kind = pipeline.KindGenerate

	var body struct {
		Model  string `json:"model"`
		Stream bool   `json:"stream"`
	}
	if err := common.UnmarshalBodyReusable(c, &body); err == nil {
		op.Model = body.Model
		op.IsStream = body.Stream
	}
	if op.Model == "" {
		// TokenAuth 已解析过一次模型名，复用其结果
		op.Model = c.GetString(ctxkey.RequestModel)
	}
	return op, nil
}

func init() {
	pipeline.Register(&pipeline.RelaySpec{
		Name:       "anthropic.native",
		Mode:       pipeline.ModePassthrough,
		PathPrefix: "/anthropic",
		Resolve:    resolveAnthropic,
	})
}
