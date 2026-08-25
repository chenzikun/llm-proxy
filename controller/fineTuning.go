package controller

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/songquanpeng/one-api/common"
	"github.com/songquanpeng/one-api/common/ctxkey"
	"github.com/songquanpeng/one-api/common/logger"
	"github.com/songquanpeng/one-api/objects"
	"github.com/songquanpeng/one-api/relay/entity"
)

func FineTuningRelay(c *gin.Context) {
	ctx := c.Request.Context()
	meta := objects.GetRequestMeta(c)
    userId := c.GetInt(ctxkey.UserId)

	// 创建代理对象
	proxy, err := common.NewProxy(c)
	if err != nil {
		logger.Errorf(ctx, "Failed to create proxy: %s", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "create_proxy_failed",
		})
		return
	}
	defer proxy.Close()

	// 预扣除费用
	content := proxy.GetRequestBody()
	ftRequest := entity.FineTuningRequest{}
	if err := json.Unmarshal(content, &ftRequest); err != nil {
		c.JSON(500, gin.H{
			"error": "failed to parse params",
		})
		return
	}
	trainingFile, err := objects.NewTrainingFile(userId, ftRequest.TrainingFile, ftRequest.HyperParameters.NumEpochs, meta.ActualModelName)
	if err != nil {
		c.JSON(500, gin.H{
			"error": "get_file_failed",
		})
		return
	}
	preConsumedQuota := trainingFile.GetPreConsumedQuota()
	objects.PreCost(ctx, meta, preConsumedQuota)

	// 发送代理请求
	if err := proxy.Request(); err != nil {
		logger.Errorf(ctx, "Request failed: %s", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "do_request_failed",
		})
		return
	}

	// 读取结果并解析
	body, err := proxy.ReadResponseBody()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "read_response_body_failed",
		})
		return
	}
	var ftResp entity.FineTuningResponse
	if err = json.Unmarshal(body, &ftResp); err != nil {
		objects.ErrorWrapper(err, "parse_request_body_failed", http.StatusInternalServerError)
	}
	// 计费
	logContent := fmt.Sprintf("模型微调: Model=%s, Token数量=%d", ftResp.Model, ftResp.TrainedTokens)
	if err = objects.PostCost(ctx, meta, preConsumedQuota, int64(ftResp.TrainedTokens), 0, 0, logContent); err != nil {
		c.JSON(500, gin.H{
			"error": "post_cost_failed",
		})
		return
	}

	// 返回
	proxy.WriteResponse()
}

func ListFineTuningJobsRelay(c *gin.Context) {
	ctx := c.Request.Context()
	// meta := objects.GetRequestMeta(c)
	// userId := c.GetInt(ctxkey.UserId)

    // 创建代理对象
	proxy, err := common.NewProxy(c)
	if err != nil {
		logger.Errorf(ctx, "Failed to create proxy: %s", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "create_proxy_failed",
		})
		return
	}
	defer proxy.Close()

    // 发送请求
	if err := proxy.Request(); err != nil {
		logger.Errorf(ctx, "Request failed: %s", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "do_request_failed",
		})
		return
	}

	// 返回
	proxy.WriteResponse()
}

func GetFineTuningJobRelay(c *gin.Context) {
	ctx := c.Request.Context()
	// meta := objects.GetRequestMeta(c)
	// userId := c.GetInt(ctxkey.UserId)

	// 创建代理对象
	proxy, err := common.NewProxy(c)
	if err != nil {
		logger.Errorf(ctx, "Failed to create proxy: %s", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "create_proxy_failed",
		})
		return
	}
	defer proxy.Close()

    // 发送请求
	if err := proxy.Request(); err != nil {
		logger.Errorf(ctx, "Request failed: %s", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "do_request_failed",
		})
	}

	// 返回
	proxy.WriteResponse()
}

func GetFineTuningJobCheckpointsRelay(c *gin.Context) {
	ctx := c.Request.Context()
	// meta := objects.GetRequestMeta(c)
	// userId := c.GetInt(ctxkey.UserId)

    // 创建代理对象
	proxy, err := common.NewProxy(c)
	if err != nil {
		logger.Errorf(ctx, "Failed to create proxy: %s", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "create_proxy_failed",
		})
		return
	}
	defer proxy.Close()

    // 发送请求
	if err := proxy.Request(); err != nil {
		logger.Errorf(ctx, "Request failed: %s", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "do_request_failed",
		})
		return
	}

	// 返回
	proxy.WriteResponse()
}
