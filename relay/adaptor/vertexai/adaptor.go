package vertexai

import (
	"errors"
	"fmt"
	"github.com/songquanpeng/one-api/objects"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/songquanpeng/one-api/relay/adaptor"
	channelhelper "github.com/songquanpeng/one-api/relay/adaptor"
	"github.com/songquanpeng/one-api/relay/entity"
)

var _ adaptor.RelayAdaptor = new(Adaptor)

const channelName = "vertexai"

type Adaptor struct{}

func (a *Adaptor) Init(meta *objects.Meta) error {
	return nil
}

func (a *Adaptor) ConvertRequest(c *gin.Context, relayMode int, request *entity.GeneralOpenAIRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}

	adaptor := GetAdaptor(request.Model)
	if adaptor == nil {
		return nil, errors.New("adaptor not found")
	}

	return adaptor.ConvertRequest(c, relayMode, request)
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, meta *objects.Meta) (usage *entity.Usage, responseText string, err *objects.ErrorWithStatusCode) {
	adaptor := GetAdaptor(meta.ActualModelName)
	if adaptor == nil {
		return nil, "", &objects.ErrorWithStatusCode{
			StatusCode: http.StatusInternalServerError,
			Error: objects.Error{
				Message: "adaptor not found",
			},
		}
	}
	return adaptor.DoResponse(c, resp, meta)
}

func (a *Adaptor) GetModelList() (models []string) {
	models = modelList
	return
}

func (a *Adaptor) GetChannelName() string {
	return channelName
}

func (a *Adaptor) GetRequestURL(meta *objects.Meta) (string, error) {
	suffix := ""
	if strings.HasPrefix(meta.ActualModelName, "gemini") || strings.HasPrefix(meta.ActualModelName, "endpoints") {
		if meta.IsStream {
			suffix = "streamGenerateContent?alt=sse"
		} else {
			suffix = "generateContent"
		}
	} else {
		if meta.IsStream {
			suffix = "streamRawPredict?alt=sse"
		} else {
			suffix = "rawPredict"
		}
	}

	if strings.HasPrefix(meta.ActualModelName, "endpoints") {
		return fmt.Sprintf(
			"https://%s-aiplatform.googleapis.com/v1/projects/%s/locations/%s/%s:%s",
			meta.Config.Region,
			meta.Config.VertexAIProjectID,
			meta.Config.Region,
			meta.ActualModelName,
			suffix,
		), nil
	}

	if meta.BaseURL != "" {
		return fmt.Sprintf(
			"%s/v1/projects/%s/locations/%s/publishers/google/models/%s:%s",
			meta.BaseURL,
			meta.Config.VertexAIProjectID,
			meta.Config.Region,
			meta.ActualModelName,
			suffix,
		), nil
	}
	return fmt.Sprintf(
		"https://%s-aiplatform.googleapis.com/v1/projects/%s/locations/%s/publishers/google/models/%s:%s",
		meta.Config.Region,
		meta.Config.VertexAIProjectID,
		meta.Config.Region,
		meta.ActualModelName,
		suffix,
	), nil
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Request, meta *objects.Meta) error {
	adaptor.SetupCommonRequestHeader(c, req, meta)
	token, err := getToken(c, meta.ChannelId, meta.Config.VertexAIADC)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	return nil
}

func (a *Adaptor) ConvertImageRequest(request *entity.ImageRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}
	return request, nil
}

func (a *Adaptor) DoRequest(c *gin.Context, meta *objects.Meta, requestBody io.Reader) (*http.Response, error) {
	return channelhelper.DoRequestHelper(a, c, meta, requestBody)
}
