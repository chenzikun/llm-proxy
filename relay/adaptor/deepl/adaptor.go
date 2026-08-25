package deepl

import (
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/songquanpeng/one-api/objects"
	"github.com/songquanpeng/one-api/relay/adaptor"
	"github.com/songquanpeng/one-api/relay/entity"
)

type Adaptor struct {
	meta       *objects.Meta
	promptText string
}

func (a *Adaptor) Init(meta *objects.Meta) error {
	a.meta = meta
	return nil
}

func (a *Adaptor) GetRequestURL(meta *objects.Meta) (string, error) {
	return fmt.Sprintf("%s/v2/translate", meta.BaseURL), nil
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Request, meta *objects.Meta) error {
	adaptor.SetupCommonRequestHeader(c, req, meta)
	req.Header.Set("Authorization", "DeepL-Auth-Key "+meta.APIKey)
	return nil
}

func (a *Adaptor) ConvertRequest(c *gin.Context, relayMode int, request *entity.GeneralOpenAIRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}
	convertedRequest, text := ConvertRequest(*request)
	a.promptText = text
	return convertedRequest, nil
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
		err = StreamHandler(c, resp, meta.ActualModelName)
	} else {
		err = Handler(c, resp, meta.ActualModelName)
	}
	promptTokens := len(a.promptText)
	usage = &entity.Usage{
		PromptTokens: promptTokens,
		TotalTokens:  promptTokens,
	}
	return
}

// func (a *Adaptor) GetModelList() []string {
// 	return ModelList
// }

func (a *Adaptor) GetChannelName() string {
	return "deepl"
}
