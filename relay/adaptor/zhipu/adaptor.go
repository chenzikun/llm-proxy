package zhipu

import (
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/songquanpeng/one-api/objects"
	"github.com/songquanpeng/one-api/relay/adaptor"
	"github.com/songquanpeng/one-api/relay/adaptor/openai"
	"github.com/songquanpeng/one-api/relay/entity"
	"github.com/songquanpeng/one-api/relay/relaymode"
)

type Adaptor struct {
	APIVersion string
}

func (a *Adaptor) Init(meta *objects.Meta) error {
	return nil
}

func (a *Adaptor) SetVersionByModeName(modelName string) {
	if strings.HasPrefix(modelName, "glm-") {
		a.APIVersion = "v4"
	} else {
		a.APIVersion = "v3"
	}
}

func (a *Adaptor) GetRequestURL(meta *objects.Meta) (string, error) {
	switch meta.Mode {
	case relaymode.ImagesGenerations:
		return fmt.Sprintf("%s/api/paas/v4/images/generations", meta.BaseURL), nil
	case relaymode.Embeddings:
		return fmt.Sprintf("%s/api/paas/v4/embeddings", meta.BaseURL), nil
	}
	a.SetVersionByModeName(meta.ActualModelName)
	if a.APIVersion == "v4" {
		return fmt.Sprintf("%s/api/paas/v4/chat/completions", meta.BaseURL), nil
	}
	method := "invoke"
	if meta.IsStream {
		method = "sse-invoke"
	}
	return fmt.Sprintf("%s/api/paas/v3/model-api/%s/%s", meta.BaseURL, meta.ActualModelName, method), nil
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Request, meta *objects.Meta) error {
	adaptor.SetupCommonRequestHeader(c, req, meta)
	token := GetToken(meta.APIKey)
	req.Header.Set("Authorization", token)
	return nil
}

func (a *Adaptor) ConvertRequest(c *gin.Context, relayMode int, request *entity.GeneralOpenAIRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}
	switch relayMode {
	case relaymode.Embeddings:
		baiduEmbeddingRequest, err := ConvertEmbeddingRequest(*request)
		return baiduEmbeddingRequest, err
	default:
		// TopP (0.0, 1.0)
		request.TopP = math.Min(0.99, request.TopP)
		request.TopP = math.Max(0.01, request.TopP)

		// Temperature (0.0, 1.0)
		request.Temperature = math.Min(0.99, request.Temperature)
		request.Temperature = math.Max(0.01, request.Temperature)
		a.SetVersionByModeName(request.Model)
		if a.APIVersion == "v4" {
			return request, nil
		}
		return ConvertRequest(*request), nil
	}
}

func (a *Adaptor) ConvertImageRequest(request *entity.ImageRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}
	newRequest := ImageRequest{
		Model:  request.Model,
		Prompt: request.Prompt,
		UserId: request.User,
	}
	return newRequest, nil
}

func (a *Adaptor) DoRequest(c *gin.Context, meta *objects.Meta, requestBody io.Reader) (*http.Response, error) {
	return adaptor.DoRequestHelper(a, c, meta, requestBody)
}

func (a *Adaptor) DoResponseV4(c *gin.Context, resp *http.Response, meta *objects.Meta) (usage *entity.Usage, responseText string, err *objects.ErrorWithStatusCode) {
	if meta.IsStream {
		err, usage, responseText = openai.StreamHandler(c, resp, meta.Mode)
	} else {
		err, usage, responseText = openai.Handler(c, resp, meta.PromptTokens, meta.ActualModelName)
	}
	return
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, meta *objects.Meta) (usage *entity.Usage, responseText string, err *objects.ErrorWithStatusCode) {
	switch meta.Mode {
	case relaymode.Embeddings:
		err, usage = EmbeddingsHandler(c, resp)
		return
	case relaymode.ImagesGenerations:
		err, usage = openai.ImageHandler(c, resp)
		return
	}
	if a.APIVersion == "v4" {
		return a.DoResponseV4(c, resp, meta)
	}
	if meta.IsStream {
		err, usage = StreamHandler(c, resp)
	} else {
		if meta.Mode == relaymode.Embeddings {
			err, usage = EmbeddingsHandler(c, resp)
		} else {
			err, usage = Handler(c, resp)
		}
	}
	return
}

func ConvertEmbeddingRequest(request entity.GeneralOpenAIRequest) (*EmbeddingRequest, error) {
	inputs := request.ParseInput()
	if len(inputs) != 1 {
		return nil, errors.New("invalid input length, zhipu only support one input")
	}
	return &EmbeddingRequest{
		Model: request.Model,
		Input: inputs[0],
	}, nil
}

// func (a *Adaptor) GetModelList() []string {
// 	return ModelList
// }

func (a *Adaptor) GetChannelName() string {
	return "zhipu"
}
