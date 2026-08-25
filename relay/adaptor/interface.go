package adaptor

import (
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/songquanpeng/one-api/objects"
	"github.com/songquanpeng/one-api/relay/entity"
)

type RelayAdaptor interface {
	Init(meta *objects.Meta) error
	GetRequestURL(meta *objects.Meta) (string, error)
	SetupRequestHeader(c *gin.Context, req *http.Request, meta *objects.Meta) error
	ConvertRequest(c *gin.Context, relayMode int, request *entity.GeneralOpenAIRequest) (any, error)
	ConvertImageRequest(request *entity.ImageRequest) (any, error)
	DoRequest(c *gin.Context, meta *objects.Meta, requestBody io.Reader) (*http.Response, error)
	DoResponse(c *gin.Context, resp *http.Response, meta *objects.Meta) (usage *entity.Usage, responseText string, err *objects.ErrorWithStatusCode)
	// GetModelList() []string
	GetChannelName() string
}
