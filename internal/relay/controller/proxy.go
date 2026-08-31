// Package controller is a package for handling the relay controller
package controller

import (
	"fmt"
	"github.com/zicorn/llm-proxy/internal/objects"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zicorn/llm-proxy/pkg/common/logger"
	"github.com/zicorn/llm-proxy/internal/relay"
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
	usage, _, respErr := adaptor.DoResponse(c, resp, meta)
	if respErr != nil {
		logger.Errorf(ctx, "[RelayProxyHelper] respErr is not nil: %+v", respErr)
		return respErr
	}

	// rerank 与 oneapi/proxy 都走这条链路，此前 usage 被直接丢弃导致零计费
	if usage == nil {
		logger.Errorf(ctx, "[RelayProxyHelper] 用量解析异常：模型 %s 未返回 usage，按 0 结算，需补适配",
			meta.ActualModelName)
		return nil
	}
	objects.PostConsumeQuota(ctx, usage, meta, 0)

	return nil
}
