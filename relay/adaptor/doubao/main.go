package doubao

import (
	"fmt"
	"github.com/songquanpeng/one-api/objects"
	"github.com/songquanpeng/one-api/relay/relaymode"
)

func GetRequestURL(meta *objects.Meta) (string, error) {
	switch meta.Mode {
	case relaymode.ChatCompletions:
		return fmt.Sprintf("%s/api/v3/chat/completions", meta.BaseURL), nil
	case relaymode.Embeddings:
		return fmt.Sprintf("%s/api/v3/embeddings", meta.BaseURL), nil
	default:
	}
	return "", fmt.Errorf("unsupported relay mode %d for doubao", meta.Mode)
}
