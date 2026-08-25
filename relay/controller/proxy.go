// Package controller is a package for handling the relay controller
package controller

import (
	"fmt"
	"github.com/songquanpeng/one-api/objects"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/songquanpeng/one-api/common/logger"
	"github.com/songquanpeng/one-api/relay"
)

// RelayProxyHelper is a helper function to proxy the request to the upstream service
func RelayProxyHelper(c *gin.Context, relayMode int) *objects.ErrorWithStatusCode {
	ctx := c.Request.Context()
	meta := objects.GetRequestMeta(c)

	textRequest, err := getAndValidateTextRequest(c, meta.Mode)
	if err != nil {
		// 这里的异常可能并不重要，所以只记录日志，不返回错误
		logger.Errorf(ctx, "getAndValidateTextRequest failed: %s", err.Error())
	}
	textRequest.Model, _ = getMappedModelName(textRequest.Model, meta.ModelMapping)
	meta.IsStream = textRequest.Stream
	meta.OriginModelName = textRequest.Model
	meta.ActualModelName = textRequest.Model

	adaptor := relay.GetAdaptor(meta.APIType)
	if adaptor == nil {
		return objects.ErrorWrapper(fmt.Errorf("invalid api type: %d", meta.APIType), "invalid_api_type", http.StatusBadRequest)
	}
	if err := adaptor.Init(meta); err != nil {
		logger.Errorf(ctx, "Init failed: %s", err.Error())
		return objects.ErrorWrapper(err, "init_failed", http.StatusInternalServerError)
	}

	if _, err := adaptor.ConvertRequest(c, relayMode, textRequest); err != nil {
		logger.Errorf(ctx, "ConvertRequest failed: %s", err.Error())
		return objects.ErrorWrapper(err, "convert_request_failed", http.StatusInternalServerError)
	}

	resp, err := adaptor.DoRequest(c, meta, c.Request.Body)
	if err != nil {
		logger.Errorf(ctx, "DoRequest failed: %s", err.Error())
		return objects.ErrorWrapper(err, "do_request_failed", http.StatusInternalServerError)
	}

	// do response
	_, _, respErr := adaptor.DoResponse(c, resp, meta)
	if respErr != nil {
		logger.Errorf(ctx, "[RelayProxyHelper] respErr is not nil: %+v", respErr)
		return respErr
	}

	return nil
}
