package vertexai

import (
	"testing"

	"github.com/stretchr/testify/assert"
	claude "github.com/zicorn/llm-proxy/internal/relay/adaptor/vertexai/claude"
	gemini "github.com/zicorn/llm-proxy/internal/relay/adaptor/vertexai/gemini"
)

func TestGetAdaptorDispatchesByModel(t *testing.T) {
	assert.IsType(t, &gemini.Adaptor{}, GetAdaptor("gemini-2.5-flash"))
	assert.IsType(t, &gemini.Adaptor{}, GetAdaptor("endpoints/123"))
	assert.IsType(t, &claude.Adaptor{}, GetAdaptor("claude-sonnet-4"),
		"Vertex 上的 Claude 走 rawPredict，响应是 Anthropic 格式，必须用 claude 适配器")
}

// 模型名不在硬编码 ModelList 中也必须能分派，新模型上线不应依赖改代码。
func TestGetAdaptorHandlesModelsOutsideModelList(t *testing.T) {
	assert.NotContains(t, modelList, "gemini-9.9-future")
	assert.IsType(t, &gemini.Adaptor{}, GetAdaptor("gemini-9.9-future"))

	assert.NotContains(t, modelList, "claude-opus-9")
	assert.IsType(t, &claude.Adaptor{}, GetAdaptor("claude-opus-9"))
}
