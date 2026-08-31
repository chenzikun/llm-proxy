package controller

import (
	"bytes"
	"context"
	"fmt"
	"github.com/zicorn/llm-proxy/internal/objects"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zicorn/llm-proxy/pkg/common"
	"github.com/zicorn/llm-proxy/pkg/common/config"
	"github.com/zicorn/llm-proxy/pkg/common/ctxkey"
	"github.com/zicorn/llm-proxy/pkg/common/helper"
	"github.com/zicorn/llm-proxy/pkg/common/logger"
	"github.com/zicorn/llm-proxy/internal/middleware"
	dbmodel "github.com/zicorn/llm-proxy/internal/repo"
	"github.com/zicorn/llm-proxy/internal/monitor"
	relaycontroller "github.com/zicorn/llm-proxy/internal/relay/controller"
	"github.com/zicorn/llm-proxy/internal/relay/relaymode"
)

// https://platform.openai.com/docs/api-reference/chat

// 转发请求
func relayHelper(c *gin.Context, relayMode int) *objects.ErrorWithStatusCode {
	var err *objects.ErrorWithStatusCode
	switch relayMode {
	case relaymode.ImagesGenerations:
		err = relaycontroller.RelayImageHelper(c, relayMode)
	case relaymode.AudioSpeech:
		err = relaycontroller.RelayAudioSpeechHelper(c, relayMode)
	case relaymode.AudioTranslation:
		fallthrough
	case relaymode.AudioTranscription:
		err = relaycontroller.RelayAudioHelper(c, relayMode)
	case relaymode.Rerank:
		fallthrough
	case relaymode.Proxy:
		err = relaycontroller.RelayProxyHelper(c, relayMode)
	default:
		err = relaycontroller.RelayTextHelper(c, relayMode)
	}
	return err
}

func Relay(c *gin.Context) {
	ctx := c.Request.Context()
	relayMode := relaymode.GetByPath(c.Request.URL.Path)
	if config.DebugEnabled {
		requestBody, _ := common.GetRequestBody(c)
		logger.Debugf(ctx, "request body: %s", string(requestBody))
	}
	channelId := c.GetInt(ctxkey.ChannelId)
	userId := c.GetInt(ctxkey.UserId)
	// 转发请求
	bizErr := relayHelper(c, relayMode)
	if bizErr == nil {
		monitor.Emit(channelId, true)
		return
	}
	lastFailedChannelId := channelId
	channelName := c.GetString(ctxkey.ChannelName)
	group := c.GetString(ctxkey.Group)
	originalModel := c.GetString(ctxkey.OriginalModel)
	go processChannelRelayError(ctx, userId, channelId, channelName, bizErr)
	requestId := c.GetString(helper.RequestIdKey)
	retryTimes := config.RetryTimes
	if !shouldRetry(c, bizErr.StatusCode) {
		logger.Errorf(ctx, "relay error happen, status code is %d, won't retry in this case", bizErr.StatusCode)
		retryTimes = 0
	}
	for i := retryTimes; i > 0; i-- {
		channel, err := dbmodel.CacheGetRandomSatisfiedChannel(group, originalModel, i != retryTimes)
		if err != nil {
			logger.Errorf(ctx, "CacheGetRandomSatisfiedChannel failed: %+v", err)
			break
		}
		logger.Infof(ctx, "using channel #%d to retry (remain times %d)", channel.Id, i)
		if channel.Id == lastFailedChannelId {
			continue
		}
		middleware.SetupContextForSelectedChannel(c, channel, originalModel)
		requestBody, err := common.GetRequestBody(c)
		c.Request.Body = io.NopCloser(bytes.NewBuffer(requestBody))
		bizErr = relayHelper(c, relayMode)
		if bizErr == nil {
			return
		}
		channelId := c.GetInt(ctxkey.ChannelId)
		lastFailedChannelId = channelId
		channelName := c.GetString(ctxkey.ChannelName)
		// BUG: bizErr is in race condition
		go processChannelRelayError(ctx, userId, channelId, channelName, bizErr)
	}
	if bizErr != nil {
		if bizErr.StatusCode == http.StatusTooManyRequests {
			bizErr.Error.Message = "当前分组上游负载已饱和，请稍后再试"
		}

		// BUG: bizErr is in race condition
		bizErr.Error.Message = helper.MessageWithRequestId(bizErr.Error.Message, requestId)
		c.JSON(bizErr.StatusCode, gin.H{
			"error": bizErr.Error,
		})
	}
}

func shouldRetry(c *gin.Context, statusCode int) bool {
	if _, ok := c.Get(ctxkey.SpecificChannelId); ok {
		return false
	}
	if statusCode == http.StatusTooManyRequests {
		return true
	}
	if statusCode/100 == 5 {
		return true
	}
	if statusCode == http.StatusBadRequest {
		return false
	}
	if statusCode/100 == 2 {
		return false
	}
	return true
}

func processChannelRelayError(ctx context.Context, userId int, channelId int, channelName string, err *objects.ErrorWithStatusCode) {
	logger.Errorf(ctx, "relay error (channel id %d, user id: %d): %s", channelId, userId, err.Message)
	// https://platform.openai.com/docs/guides/error-codes/api-errors
	if monitor.ShouldDisableChannel(&err.Error, err.StatusCode) {
		monitor.DisableChannel(channelId, channelName, err.Message)
	} else {
		monitor.Emit(channelId, false)
	}
}

func RelayNotImplemented(c *gin.Context) {
	err := objects.Error{
		Message: "API not implemented",
		Type:    "one_api_error",
		Param:   "",
		Code:    "api_not_implemented",
	}
	c.JSON(http.StatusNotImplemented, gin.H{
		"error": err,
	})
}

func RelayNotFound(c *gin.Context) {
	err := objects.Error{
		Message: fmt.Sprintf("Invalid URL (%s %s)", c.Request.Method, c.Request.URL.Path),
		Type:    "invalid_request_error",
		Param:   "",
		Code:    "",
	}
	c.JSON(http.StatusNotFound, gin.H{
		"error": err,
	})
}

// 原生格式（/anthropic、/gemini、/vertexai）的转发已收归 relay/pipeline，
// 由 pipeline.Handler 直接绑定路由，计费与日志在管线内部强制生效。
