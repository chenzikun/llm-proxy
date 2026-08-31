package usage

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAnthropicNonStream(t *testing.T) {
	body := []byte(`{
		"type":"message","role":"assistant",
		"content":[{"type":"text","text":"hi"}],
		"usage":{"input_tokens":13,"output_tokens":4}
	}`)
	u, ok := Anthropic(body, false)
	assert.True(t, ok)
	assert.Equal(t, 13, u.PromptTokens)
	assert.Equal(t, 4, u.CompletionTokens)
	assert.Equal(t, 17, u.TotalTokens)
}

func TestAnthropicNonStreamWithCacheRead(t *testing.T) {
	body := []byte(`{"usage":{"input_tokens":20,"output_tokens":3,"cache_read_input_tokens":15}}`)
	u, ok := Anthropic(body, false)
	assert.True(t, ok)
	assert.Equal(t, 15, u.PromptTokensDetails.CachedTokens)
}

func TestAnthropicStreamCombinesStartAndDelta(t *testing.T) {
	body := []byte(`event: message_start
data: {"type":"message_start","message":{"usage":{"input_tokens":13,"cache_read_input_tokens":9}}}

event: content_block_delta
data: {"type":"content_block_delta","delta":{"text":"hi"}}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":21}}

`)
	u, ok := Anthropic(body, true)
	assert.True(t, ok)
	assert.Equal(t, 13, u.PromptTokens, "input_tokens 来自 message_start")
	assert.Equal(t, 21, u.CompletionTokens, "output_tokens 来自 message_delta")
	assert.Equal(t, 9, u.PromptTokensDetails.CachedTokens)
}

func TestAnthropicUnparseable(t *testing.T) {
	_, ok := Anthropic([]byte(`{"type":"error"}`), false)
	assert.False(t, ok)

	_, ok = Anthropic([]byte("event: ping\ndata: {\"type\":\"ping\"}\n"), true)
	assert.False(t, ok)
}
