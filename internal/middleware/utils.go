package middleware

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/zicorn/llm-proxy/pkg/common"
	"github.com/zicorn/llm-proxy/pkg/common/config"
	"github.com/zicorn/llm-proxy/pkg/common/helper"
	"github.com/zicorn/llm-proxy/pkg/common/logger"
	"github.com/zicorn/llm-proxy/internal/relay/nativeformat"
)

func abortWithMessage(c *gin.Context, statusCode int, message string) {
	c.JSON(statusCode, gin.H{
		"error": gin.H{
			"message": helper.MessageWithRequestId(message, c.GetString(helper.RequestIdKey)),
			"type":    "one_api_error",
		},
	})
	c.Abort()
	logger.Error(c.Request.Context(), message)
}

func getRequestModel(c *gin.Context) (string, error) {
	path := c.Request.URL.Path

	// 原生格式：model 提取方式与 OpenAI 格式不同
	for prefix, format := range nativeformat.URLPrefixToFormat {
		if strings.HasPrefix(path, prefix) {
			return nativeformat.GetModelFromRequest(c, format)
		}
	}

	// OpenAI 格式
	var modelRequest ModelRequest
	if strings.HasPrefix(path, "/v1/files") {
		return config.DefaultFilesModel, nil
	}
	if strings.HasPrefix(path, "/v1/fine_tuning/jobs") {
		return config.DefaultFineTuningModel, nil
	}
	err := common.UnmarshalBodyReusable(c, &modelRequest)
	if err != nil {
		return "", fmt.Errorf("common.UnmarshalBodyReusable failed: %w", err)
	}
	if strings.HasPrefix(path, "/v1/moderations") {
		if modelRequest.Model == "" {
			modelRequest.Model = config.DefaultModerationModel
		}
	}
	if strings.HasSuffix(path, "embeddings") {
		if modelRequest.Model == "" {
			modelRequest.Model = c.Param("model")
		}
	}
	if strings.HasPrefix(path, "/v1/images/generations") {
		if modelRequest.Model == "" {
			modelRequest.Model = config.DefaultImageModel
		}
	}
	if strings.HasPrefix(path, "/v1/audio/transcriptions") || strings.HasPrefix(path, "/v1/audio/translations") {
		if modelRequest.Model == "" {
			modelRequest.Model = config.DefaultAudioModel
		}
	}
	return modelRequest.Model, nil
}

func isModelInList(modelName string, models string) bool {
	modelList := strings.Split(models, ",")
	for _, model := range modelList {
		if modelName == model {
			return true
		}
	}
	return false
}
