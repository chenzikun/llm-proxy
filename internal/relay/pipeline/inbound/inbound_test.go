package inbound

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/zicorn/llm-proxy/internal/relay/pipeline"
	"github.com/zicorn/llm-proxy/internal/relay/wireformat"
)

func ctxWithRequest(method, path, body string) *gin.Context {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(method, path, strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	return c
}

func TestResolveGemini(t *testing.T) {
	cases := []struct {
		path       string
		wantModel  string
		wantKind   pipeline.Kind
		wantStream bool
	}{
		{"/gemini/v1beta/models/gemini-2.5-flash:generateContent",
			"gemini-2.5-flash", pipeline.KindGenerate, false},
		{"/gemini/v1beta/models/gemini-2.5-flash:streamGenerateContent?alt=sse",
			"gemini-2.5-flash", pipeline.KindGenerate, true},
		{"/gemini/v1beta/models/gemini-2.5-flash:countTokens",
			"gemini-2.5-flash", pipeline.KindMetadata, false},
		{"/gemini/v1beta/models/text-embedding-004:embedContent",
			"text-embedding-004", pipeline.KindUnsupported, false},
		{"/gemini/v1beta/models", "", pipeline.KindMetadata, false},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			op, err := resolveGemini(ctxWithRequest(http.MethodPost, tc.path, ""))
			assert.NoError(t, err)
			assert.Equal(t, tc.wantModel, op.Model)
			assert.Equal(t, tc.wantKind, op.Kind)
			assert.Equal(t, tc.wantStream, op.IsStream)
			assert.Equal(t, wireformat.Gemini, op.InboundWire)
		})
	}
}

func TestResolveGeminiBillableOnlyForGeneration(t *testing.T) {
	op, _ := resolveGemini(ctxWithRequest(http.MethodPost,
		"/gemini/v1beta/models/gemini-2.5-flash:generateContent", ""))
	assert.True(t, op.Billable())

	op, _ = resolveGemini(ctxWithRequest(http.MethodPost,
		"/gemini/v1beta/models/gemini-2.5-flash:countTokens", ""))
	assert.False(t, op.Billable())
}

func TestResolveVertexAIWireFormatFollowsModel(t *testing.T) {
	op, err := resolveVertexAI(ctxWithRequest(http.MethodPost,
		"/vertexai/v1/models/gemini-2.5-flash:generateContent", ""))
	assert.NoError(t, err)
	assert.Equal(t, wireformat.Gemini, op.InboundWire)
	assert.Equal(t, pipeline.KindGenerate, op.Kind)

	op, err = resolveVertexAI(ctxWithRequest(http.MethodPost,
		"/vertexai/v1/models/claude-sonnet-4:rawPredict", ""))
	assert.NoError(t, err)
	assert.Equal(t, wireformat.Anthropic, op.InboundWire,
		"Vertex 上的 Claude 请求体是 Anthropic 格式")
	assert.Equal(t, pipeline.KindGenerate, op.Kind)

	op, _ = resolveVertexAI(ctxWithRequest(http.MethodPost,
		"/vertexai/v1/models/claude-sonnet-4:streamRawPredict", ""))
	assert.True(t, op.IsStream)
}

func TestResolveAnthropic(t *testing.T) {
	op, err := resolveAnthropic(ctxWithRequest(http.MethodPost, "/anthropic/v1/messages",
		`{"model":"claude-sonnet-4","stream":false,"messages":[]}`))
	assert.NoError(t, err)
	assert.Equal(t, "claude-sonnet-4", op.Model)
	assert.Equal(t, pipeline.KindGenerate, op.Kind)
	assert.False(t, op.IsStream)
	assert.Equal(t, wireformat.Anthropic, op.InboundWire)

	op, _ = resolveAnthropic(ctxWithRequest(http.MethodPost, "/anthropic/v1/messages",
		`{"model":"claude-sonnet-4","stream":true,"messages":[]}`))
	assert.True(t, op.IsStream)
}

func TestResolveAnthropicCountTokensNotBillable(t *testing.T) {
	op, err := resolveAnthropic(ctxWithRequest(http.MethodPost,
		"/anthropic/v1/messages/count_tokens", `{"model":"claude-sonnet-4"}`))
	assert.NoError(t, err)
	assert.False(t, op.Billable())
}

// 三份 spec 必须都在 init 中注册，否则路由绑定时服务会 panic。
func TestSpecsRegistered(t *testing.T) {
	for _, name := range []string{"gemini.native", "anthropic.native", "vertexai.native"} {
		_, ok := pipeline.Lookup(name)
		assert.True(t, ok, "spec %s 未注册", name)
	}
}
