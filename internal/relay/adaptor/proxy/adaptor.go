package proxy

import (
	"fmt"
	"github.com/zicorn/llm-proxy/internal/objects"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"github.com/zicorn/llm-proxy/internal/relay/adaptor"
	channelhelper "github.com/zicorn/llm-proxy/internal/relay/adaptor"
	"github.com/zicorn/llm-proxy/internal/relay/entity"
)

var _ adaptor.RelayAdaptor = new(ProxyAdaptor)

const channelName = "proxy"

type ProxyAdaptor struct{}

func (a *ProxyAdaptor) Init(meta *objects.Meta) error {
	return nil
}

func (a *ProxyAdaptor) ConvertRequest(c *gin.Context, relayMode int, request *entity.GeneralOpenAIRequest) (any, error) {
	return nil, errors.New("notimplement")
}

func (a *ProxyAdaptor) DoResponse(c *gin.Context, resp *http.Response, meta *objects.Meta) (usage *entity.Usage, responseText string, err *objects.ErrorWithStatusCode) {
	for k, v := range resp.Header {
		for _, vv := range v {
			c.Writer.Header().Set(k, vv)
		}
	}

	c.Writer.WriteHeader(resp.StatusCode)
	if _, gerr := io.Copy(c.Writer, resp.Body); gerr != nil {
		return nil, "", &objects.ErrorWithStatusCode{
			StatusCode: http.StatusInternalServerError,
			Error: objects.Error{
				Message: gerr.Error(),
			},
		}
	}

	return nil, "", nil
}

func (a *ProxyAdaptor) GetModelList() (models []string) {
	return nil
}

func (a *ProxyAdaptor) GetChannelName() string {
	return channelName
}

// GetRequestURL remove static prefix, and return the real request url to the upstream service
func (a *ProxyAdaptor) GetRequestURL(meta *objects.Meta) (string, error) {
	prefix := fmt.Sprintf("/v1/oneapi/proxy/%d", meta.ChannelId)
	return meta.BaseURL + strings.TrimPrefix(meta.RequestURLPath, prefix), nil

}

func (a *ProxyAdaptor) SetupRequestHeader(c *gin.Context, req *http.Request, meta *objects.Meta) error {
	for k, v := range c.Request.Header {
		req.Header.Set(k, v[0])
	}

	// remove unnecessary headers
	req.Header.Del("Host")
	req.Header.Del("Content-Length")
	req.Header.Del("Accept-Encoding")
	req.Header.Del("Connection")
	req.Header.Del("X-Session-ID") // 内部使用，不透传给下游

	// set authorization header
	req.Header.Set("Authorization", meta.APIKey)

	return nil
}

func (a *ProxyAdaptor) ConvertImageRequest(request *entity.ImageRequest) (any, error) {
	return nil, errors.Errorf("not implement")
}

func (a *ProxyAdaptor) DoRequest(c *gin.Context, meta *objects.Meta, requestBody io.Reader) (*http.Response, error) {
	return channelhelper.DoRequestHelper(a, c, meta, requestBody)
}
