package wireformat

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/zicorn/llm-proxy/internal/relay/channeltype"
)

func TestResolve(t *testing.T) {
	cases := []struct {
		name        string
		channelType int
		model       string
		want        Format
	}{
		{"OpenAI 渠道", channeltype.OpenAI, "gpt-4o", OpenAI},
		{"Azure 走 OpenAI 兼容", channeltype.Azure, "gpt-4o", OpenAI},
		{"Gemini AI Studio", channeltype.Gemini, "gemini-2.5-flash", Gemini},
		{"Anthropic", channeltype.Anthropic, "claude-sonnet-4", Anthropic},
		{"Bedrock Claude 是 Anthropic wire", channeltype.AwsClaude, "claude-sonnet-4", Anthropic},
		{"Vertex 上的 Gemini", channeltype.VertextAI, "gemini-2.5-flash", Gemini},
		{"Vertex 上的 Claude", channeltype.VertextAI, "claude-sonnet-4", Anthropic},
		{"Vertex 自定义 endpoint 走 Gemini", channeltype.VertextAI, "endpoints/123", Gemini},
		{"暂无提取器的渠道", channeltype.Baidu, "ernie-4.0", Unknown},
		{"Proxy 渠道格式不定", channeltype.Proxy, "", Unknown},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, Resolve(c.channelType, c.model))
		})
	}
}

func TestIsVertexGeminiModel(t *testing.T) {
	assert.True(t, IsVertexGeminiModel("gemini-2.5-flash"))
	assert.True(t, IsVertexGeminiModel("endpoints/123"))
	assert.False(t, IsVertexGeminiModel("claude-sonnet-4"))
	assert.False(t, IsVertexGeminiModel(""))
}

func TestFormatString(t *testing.T) {
	assert.Equal(t, "openai", OpenAI.String())
	assert.Equal(t, "gemini", Gemini.String())
	assert.Equal(t, "anthropic", Anthropic.String())
	assert.Equal(t, "unspecified", Unspecified.String())
	assert.Equal(t, "unknown", Unknown.String())
}
