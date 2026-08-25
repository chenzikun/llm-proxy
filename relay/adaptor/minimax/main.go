package minimax

import (
	"fmt"
	"github.com/songquanpeng/one-api/objects"
	"github.com/songquanpeng/one-api/relay/relaymode"
)

func GetRequestURL(meta *objects.Meta) (string, error) {
	if meta.Mode == relaymode.ChatCompletions {
		return fmt.Sprintf("%s/v1/text/chatcompletion_v2", meta.BaseURL), nil
	}
	return "", fmt.Errorf("unsupported relay mode %d for minimax", meta.Mode)
}
