package ollama

import (
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/zicorn/llm-proxy/internal/objects"
	"github.com/zicorn/llm-proxy/internal/relay/relaymode"

	"github.com/gin-gonic/gin"
	"github.com/zicorn/llm-proxy/internal/relay/adaptor"
	"github.com/zicorn/llm-proxy/internal/relay/entity"
)

type Adaptor struct {
}

func (a *Adaptor) Init(meta *objects.Meta) error {
	return nil
}

func (a *Adaptor) GetRequestURL(meta *objects.Meta) (string, error) {
	// https://github.com/ollama/ollama/blob/main/docs/api.md
	fullRequestURL := fmt.Sprintf("%s/api/chat", meta.BaseURL)
	if meta.Mode == relaymode.Embeddings {
		fullRequestURL = fmt.Sprintf("%s/api/embed", meta.BaseURL)
	}
	return fullRequestURL, nil
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Request, meta *objects.Meta) error {
	adaptor.SetupCommonRequestHeader(c, req, meta)
	req.Header.Set("Authorization", "Bearer "+meta.APIKey)
	return nil
}

func (a *Adaptor) ConvertRequest(c *gin.Context, relayMode int, request *entity.GeneralOpenAIRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}
	switch relayMode {
	case relaymode.Embeddings:
		ollamaEmbeddingRequest := ConvertEmbeddingRequest(*request)
		return ollamaEmbeddingRequest, nil
	default:
		return ConvertRequest(*request), nil
	}
}

func (a *Adaptor) ConvertImageRequest(request *entity.ImageRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}
	return request, nil
}

func (a *Adaptor) DoRequest(c *gin.Context, meta *objects.Meta, requestBody io.Reader) (*http.Response, error) {
	return adaptor.DoRequestHelper(a, c, meta, requestBody)
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, meta *objects.Meta) (usage *entity.Usage, responseText string, err *objects.ErrorWithStatusCode) {
	if meta.IsStream {
		err, usage = StreamHandler(c, resp)
	} else {
		switch meta.Mode {
		case relaymode.Embeddings:
			err, usage = EmbeddingHandler(c, resp)
		default:
			err, usage = Handler(c, resp)
		}
	}
	return
}

// func (a *Adaptor) GetModelList() []string {
// 	return ModelList
// }

func (a *Adaptor) GetChannelName() string {
	return "ollama"
}
