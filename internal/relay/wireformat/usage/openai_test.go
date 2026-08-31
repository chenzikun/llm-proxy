package usage

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOpenAINonStream(t *testing.T) {
	body := []byte(`{
		"choices":[{"message":{"content":"hi"}}],
		"usage":{"prompt_tokens":9,"completion_tokens":3,"total_tokens":12}
	}`)
	u, ok := OpenAI(body, false)
	assert.True(t, ok)
	assert.Equal(t, 9, u.PromptTokens)
	assert.Equal(t, 3, u.CompletionTokens)
	assert.Equal(t, 12, u.TotalTokens)
}

func TestOpenAINonStreamWithCachedTokens(t *testing.T) {
	body := []byte(`{"usage":{"prompt_tokens":50,"completion_tokens":2,"total_tokens":52,"prompt_tokens_details":{"cached_tokens":40}}}`)
	u, ok := OpenAI(body, false)
	assert.True(t, ok)
	assert.Equal(t, 40, u.PromptTokensDetails.CachedTokens)
}

func TestOpenAIStreamTakesLastUsageChunk(t *testing.T) {
	body := []byte(`data: {"choices":[{"delta":{"content":"a"}}]}

data: {"choices":[],"usage":{"prompt_tokens":9,"completion_tokens":5,"total_tokens":14}}

data: [DONE]

`)
	u, ok := OpenAI(body, true)
	assert.True(t, ok)
	assert.Equal(t, 9, u.PromptTokens)
	assert.Equal(t, 5, u.CompletionTokens)
}

func TestOpenAIStreamWithoutUsage(t *testing.T) {
	body := []byte("data: {\"choices\":[{\"delta\":{\"content\":\"a\"}}]}\n\ndata: [DONE]\n")
	_, ok := OpenAI(body, true)
	assert.False(t, ok, "未开启 stream_options.include_usage 时无法解析")
}
