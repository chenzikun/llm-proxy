package vertexai

import (
	"github.com/zicorn/llm-proxy/internal/objects"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"github.com/zicorn/llm-proxy/pkg/common/ctxkey"
	"github.com/zicorn/llm-proxy/internal/relay/adaptor/anthropic"

	"github.com/zicorn/llm-proxy/internal/relay/entity"
)

var ModelList = []string{
	"claude-3-haiku@20240307", "claude-3-opus@20240229", "claude-3-5-sonnet@20240620", "claude-3-sonnet@20240229",
}

const anthropicVersion = "vertex-2023-10-16"

type Adaptor struct {
}

func (a *Adaptor) ConvertRequest(c *gin.Context, relayMode int, request *entity.GeneralOpenAIRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}

	claudeReq := anthropic.ConvertRequest(*request)
	req := Request{
		AnthropicVersion: anthropicVersion,
		// Model:            claudeReq.Model,
		Messages:    claudeReq.Messages,
		System:      claudeReq.System,
		MaxTokens:   claudeReq.MaxTokens,
		Temperature: claudeReq.Temperature,
		TopP:        claudeReq.TopP,
		TopK:        claudeReq.TopK,
		Stream:      claudeReq.Stream,
		Tools:       claudeReq.Tools,
	}

	c.Set(ctxkey.RequestModel, request.Model)
	c.Set(ctxkey.ConvertedRequest, req)
	return req, nil
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, meta *objects.Meta) (usage *entity.Usage, responseText string, err *objects.ErrorWithStatusCode) {
	if meta.IsStream {
		err, usage = anthropic.StreamHandler(c, resp)
	} else {
		err, usage = anthropic.Handler(c, resp, meta.PromptTokens, meta.ActualModelName)
	}
	return
}
