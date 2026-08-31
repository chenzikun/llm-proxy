package usage

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/zicorn/llm-proxy/internal/relay/wireformat"
)

func TestGeminiNonStream(t *testing.T) {
	body := []byte(`{
		"candidates":[{"content":{"parts":[{"text":"hi"}],"role":"model"}}],
		"usageMetadata":{"promptTokenCount":11,"candidatesTokenCount":7,"totalTokenCount":18}
	}`)
	u, ok := Gemini(body, false)
	assert.True(t, ok)
	assert.Equal(t, 11, u.PromptTokens)
	assert.Equal(t, 7, u.CompletionTokens)
	assert.Equal(t, 18, u.TotalTokens)
}

func TestGeminiNonStreamWithCache(t *testing.T) {
	body := []byte(`{"usageMetadata":{"promptTokenCount":100,"candidatesTokenCount":5,"cachedContentTokenCount":80}}`)
	u, ok := Gemini(body, false)
	assert.True(t, ok)
	assert.Equal(t, 80, u.PromptTokensDetails.CachedTokens)
}

func TestGeminiStreamTakesLastChunk(t *testing.T) {
	body := []byte(`data: {"candidates":[{"content":{"parts":[{"text":"a"}]}}],"usageMetadata":{"promptTokenCount":11,"candidatesTokenCount":1}}

data: {"candidates":[{"content":{"parts":[{"text":"b"}]}}],"usageMetadata":{"promptTokenCount":11,"candidatesTokenCount":9}}

`)
	u, ok := Gemini(body, true)
	assert.True(t, ok)
	assert.Equal(t, 11, u.PromptTokens)
	assert.Equal(t, 9, u.CompletionTokens, "流式末尾 chunk 携带累计值")
}

func TestGeminiUnparseable(t *testing.T) {
	_, ok := Gemini([]byte(`{"error":{"code":429}}`), false)
	assert.False(t, ok)

	_, ok = Gemini([]byte(`not json at all`), false)
	assert.False(t, ok)

	_, ok = Gemini([]byte("data: [DONE]\n"), true)
	assert.False(t, ok)
}

func TestRegistryGet(t *testing.T) {
	assert.NotNil(t, Get(wireformat.Gemini))
	assert.NotNil(t, Get(wireformat.Anthropic))
	assert.NotNil(t, Get(wireformat.OpenAI))
	assert.Nil(t, Get(wireformat.Unknown))
	assert.Nil(t, Get(wireformat.Unspecified))
}
