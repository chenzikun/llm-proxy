package novita

import (
	"fmt"
	"github.com/zicorn/llm-proxy/internal/objects"

	"github.com/zicorn/llm-proxy/internal/relay/relaymode"
)

func GetRequestURL(meta *objects.Meta) (string, error) {
	if meta.Mode == relaymode.ChatCompletions {
		return fmt.Sprintf("%s/chat/completions", meta.BaseURL), nil
	}
	return "", fmt.Errorf("unsupported relay mode %d for novita", meta.Mode)
}
