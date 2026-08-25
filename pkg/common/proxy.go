package common

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zicorn/llm-proxy/pkg/common/client"
	"github.com/zicorn/llm-proxy/pkg/common/logger"
)

type Proxy struct {
	SrcRequest   *http.Request
	SrcWriter    http.ResponseWriter
	DstResponse  *http.Response
	DstUrl       string
	requestBody  []byte
	responseBody []byte
	isClosed     bool
	ctx          context.Context
}

func NewProxy(c *gin.Context) (*Proxy, error) {
	body, err := GetRequestBody(c)
	if err != nil {
		return nil, err
	}
	fullRequestURL := GetFullRequestUrl(c)
	logger.Debugf(c.Request.Context(), "NewProxy fullRequestURL=%s", fullRequestURL)
	return &Proxy{
		SrcRequest:  c.Request,
		SrcWriter:   c.Writer,
		DstUrl:      fullRequestURL,
		requestBody: body,
		isClosed:    false,
		ctx:         c.Request.Context(),
	}, nil
}

func (proxy *Proxy) GetRequestBody() []byte {
	return proxy.requestBody
}

// Request 发送请求
func (proxy *Proxy) Request() error {
	// 构造请求
	var reader io.Reader
	if proxy.requestBody != nil {
		reader = bytes.NewReader(proxy.requestBody)
	} else if proxy.SrcRequest.Body != nil {
		reader = proxy.SrcRequest.Body
	} else {
		return errors.New("request body is nil")
	}
	req, err := http.NewRequest(proxy.SrcRequest.Method, proxy.DstUrl, reader)
	if err != nil {
		return err
	}

	req.Header.Set("Accept", proxy.SrcRequest.Header.Get("Accept"))
	req.Header.Set("Content-Type", proxy.SrcRequest.Header.Get("Content-Type"))
	req.Header.Set("Authorization", proxy.SrcRequest.Header.Get("Authorization"))
	// 发起请求
	proxy.DstResponse, err = client.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer func(Body io.ReadCloser) {
		if err := Body.Close(); err != nil {
			logger.Errorf(proxy.ctx, "Failed to close request body: %s", err.Error())
		}
	}(req.Body)
	return nil
}

// ReadResponseBody 读取返回值
func (proxy *Proxy) ReadResponseBody() ([]byte, error) {
	if proxy.responseBody != nil {
		return proxy.responseBody, nil
	}
	var err error
	proxy.responseBody, err = io.ReadAll(proxy.DstResponse.Body)
	if err != nil {
		return nil, err
	}
	if err = proxy.DstResponse.Body.Close(); err != nil {
		logger.Errorf(proxy.ctx, "Failed to close response body: %s", err.Error())
	}
	return proxy.responseBody, nil
}

// WriteResponse 写回结果
func (proxy *Proxy) WriteResponse() {
	var reader io.Reader
	if proxy.responseBody != nil {
		reader = bytes.NewReader(proxy.responseBody)
	} else {
		reader = proxy.DstResponse.Body
	}
	for k, v := range proxy.DstResponse.Header {
		proxy.SrcWriter.Header().Set(k, v[0])
	}
	proxy.SrcWriter.WriteHeader(proxy.DstResponse.StatusCode)
	if _, err := io.Copy(proxy.SrcWriter, reader); err != nil {
		logger.Errorf(proxy.ctx, "Failed to copy response body: %s", err.Error())
	}
	if err := proxy.DstResponse.Body.Close(); err != nil {
		logger.Errorf(proxy.ctx, "Failed to close response body: %s", err.Error())
	}
}

// WriteResponseWithContent 写回结果
func (proxy *Proxy) WriteResponseWithContent(content []byte) error {
	for k, v := range proxy.DstResponse.Header {
		proxy.SrcWriter.Header().Set(k, v[0])
	}
	proxy.SrcWriter.WriteHeader(proxy.DstResponse.StatusCode)
	_, err := io.Copy(proxy.SrcWriter, bytes.NewBuffer(content))
	return err
}

func (proxy *Proxy) Close() {
	if proxy.isClosed {
		return
	}
	if err := proxy.DstResponse.Body.Close(); err != nil {
		logger.Errorf(proxy.ctx, "Failed to close response body: %s", err.Error())
	}
	proxy.isClosed = true
}
