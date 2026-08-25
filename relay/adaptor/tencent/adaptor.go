package tencent

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/songquanpeng/one-api/common/helper"
	"github.com/songquanpeng/one-api/objects"
	"github.com/songquanpeng/one-api/relay/adaptor"
	"github.com/songquanpeng/one-api/relay/adaptor/openai"
	"github.com/songquanpeng/one-api/relay/entity"
)

// https://cloud.tencent.com/document/api/1729/101837

type Adaptor struct {
	Sign      string
	Action    string
	Version   string
	Timestamp int64
}

func (a *Adaptor) Init(meta *objects.Meta) error {
	a.Action = "ChatCompletions"
	a.Version = "2023-09-01"
	a.Timestamp = helper.GetTimestamp()
	return nil
}

func (a *Adaptor) GetRequestURL(meta *objects.Meta) (string, error) {
	return meta.BaseURL + "/", nil
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Request, meta *objects.Meta) error {
	adaptor.SetupCommonRequestHeader(c, req, meta)
	req.Header.Set("Authorization", a.Sign)
	req.Header.Set("X-TC-Action", a.Action)
	req.Header.Set("X-TC-Version", a.Version)
	req.Header.Set("X-TC-Timestamp", strconv.FormatInt(a.Timestamp, 10))
	return nil
}

func (a *Adaptor) ConvertRequest(c *gin.Context, relayMode int, request *entity.GeneralOpenAIRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}
	apiKey := c.Request.Header.Get("Authorization")
	apiKey = strings.TrimPrefix(apiKey, "Bearer ")
	_, secretId, secretKey, err := ParseConfig(apiKey)
	if err != nil {
		return nil, err
	}
	tencentRequest := ConvertRequest(*request)
	// we have to calculate the sign here
	a.Sign = GetSign(*tencentRequest, a, secretId, secretKey)
	return tencentRequest, nil
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
		var responseText string
		err, responseText = StreamHandler(c, resp)
		usage = openai.ResponseText2Usage(responseText, meta.ActualModelName, meta.PromptTokens)
	} else {
		err, usage = Handler(c, resp)
	}
	return
}

// func (a *Adaptor) GetModelList() []string {
// 	return ModelList
// }

func (a *Adaptor) GetChannelName() string {
	return "tencent"
}
