package vertexai

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"github.com/songquanpeng/one-api/common/ctxkey"
	"github.com/songquanpeng/one-api/objects"
	"github.com/songquanpeng/one-api/relay/adaptor/gemini"
	"github.com/songquanpeng/one-api/relay/adaptor/openai"
	"github.com/songquanpeng/one-api/relay/entity"
	"github.com/songquanpeng/one-api/relay/relaymode"
)

var ModelList = []string{
	"gemini-pro", "gemini-pro-vision",
	"gemini-exp-1206",
	"gemini-1.5-pro-001", "gemini-1.5-pro-002",
	"gemini-1.5-flash-001", "gemini-1.5-flash-002",
	"gemini-2.0-flash-exp", "gemini-2.0-flash-001",
	"gemini-2.0-pro-exp-02-05",
	"gemini-2.0-flash-lite-preview-02-05",
	"gemini-2.0-flash-thinking-exp-01-21",
}

type Adaptor struct {
}

func (a *Adaptor) ConvertRequest(c *gin.Context, relayMode int, request *entity.GeneralOpenAIRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}

	geminiRequest := gemini.ConvertRequest(*request)
	c.Set(ctxkey.RequestModel, request.Model)
	c.Set(ctxkey.ConvertedRequest, geminiRequest)
	return geminiRequest, nil
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, meta *objects.Meta) (usage *entity.Usage, responseText string, err *objects.ErrorWithStatusCode) {
	if meta.IsStream {
		var responseText string
		err, responseText = gemini.StreamHandler(c, resp)
		usage = openai.ResponseText2Usage(responseText, meta.ActualModelName, meta.PromptTokens)
	} else {
		switch meta.Mode {
		case relaymode.Embeddings:
			err, usage = gemini.EmbeddingHandler(c, resp)
		default:
			err, usage = gemini.Handler(c, resp, meta.PromptTokens, meta.ActualModelName)
		}
	}
	return
}
