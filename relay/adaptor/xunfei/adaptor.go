package xunfei

import (
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/songquanpeng/one-api/objects"
	"github.com/songquanpeng/one-api/relay/adaptor"
	"github.com/songquanpeng/one-api/relay/entity"
)

type Adaptor struct {
	request *entity.GeneralOpenAIRequest
	meta    *objects.Meta
}

func (a *Adaptor) Init(meta *objects.Meta) error {
	a.meta = meta
	return nil
}

func (a *Adaptor) GetRequestURL(meta *objects.Meta) (string, error) {
	return "", nil
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Request, meta *objects.Meta) error {
	adaptor.SetupCommonRequestHeader(c, req, meta)
	// check DoResponse for auth part
	return nil
}

func (a *Adaptor) ConvertRequest(c *gin.Context, relayMode int, request *entity.GeneralOpenAIRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}
	a.request = request
	return nil, nil
}

func (a *Adaptor) ConvertImageRequest(request *entity.ImageRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}
	return request, nil
}

func (a *Adaptor) DoRequest(c *gin.Context, meta *objects.Meta, requestBody io.Reader) (*http.Response, error) {
	// xunfei's request is not http request, so we don't need to do anything here
	dummyResp := &http.Response{}
	dummyResp.StatusCode = http.StatusOK
	return dummyResp, nil
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, meta *objects.Meta) (usage *entity.Usage, responseText string, err *objects.ErrorWithStatusCode) {
	splits := strings.Split(meta.APIKey, "|")
	if len(splits) != 3 {
		return nil, "", objects.ErrorWrapper(errors.New("invalid auth"), "invalid_auth", http.StatusBadRequest)
	}
	if a.request == nil {
		return nil, "", objects.ErrorWrapper(errors.New("request is nil"), "request_is_nil", http.StatusBadRequest)
	}
	version := parseAPIVersionByModelName(meta.ActualModelName)
	if version == "" {
		version = a.meta.Config.APIVersion
	}
	if version == "" {
		version = "v1.1"
	}
	a.meta.Config.APIVersion = version
	if meta.IsStream {
		err, usage = StreamHandler(c, meta, *a.request, splits[0], splits[1], splits[2])
	} else {
		err, usage = Handler(c, meta, *a.request, splits[0], splits[1], splits[2])
	}
	return
}

// func (a *Adaptor) GetModelList() []string {
// 	return ModelList
// }

func (a *Adaptor) GetChannelName() string {
	return "xunfei"
}
