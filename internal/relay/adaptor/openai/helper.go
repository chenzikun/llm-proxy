package openai

import (
	"fmt"
	"strings"

	"github.com/zicorn/llm-proxy/internal/objects"
	"github.com/zicorn/llm-proxy/internal/relay/channeltype"
	"github.com/zicorn/llm-proxy/internal/relay/entity"
)

func ResponseText2Usage(responseText string, modeName string, promptTokens int) *entity.Usage {
	usage := &entity.Usage{}
	usage.PromptTokens = promptTokens
	usage.CompletionTokens = objects.CountTokenText(responseText, modeName)
	usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	return usage
}

func GetFullRequestURL(baseURL string, requestURL string, channelType int) string {
	if strings.HasPrefix(requestURL, "/v1") && strings.HasSuffix(baseURL, "/v1") {
		baseURL = strings.TrimSuffix(baseURL, "/v1")
	}
	// gemini 的url没有v1
	if strings.Contains(baseURL, "googleapis") {
		requestURL = strings.TrimPrefix(requestURL, "/v1")
	}
	fullRequestURL := fmt.Sprintf("%s%s", baseURL, requestURL)
	if strings.HasPrefix(baseURL, "https://gateway.ai.cloudflare.com") {
		switch channelType {
		case channeltype.OpenAI:
			fullRequestURL = fmt.Sprintf("%s%s", baseURL, strings.TrimPrefix(requestURL, "/v1"))
		case channeltype.Azure:
			fullRequestURL = fmt.Sprintf("%s%s", baseURL, strings.TrimPrefix(requestURL, "/openai/deployments"))
		}
	}
	return fullRequestURL
}
