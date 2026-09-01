package controller

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zicorn/llm-proxy/pkg/common"
	"github.com/zicorn/llm-proxy/pkg/common/ctxkey"
	"github.com/zicorn/llm-proxy/pkg/common/logger"
	"github.com/zicorn/llm-proxy/internal/objects"
	"github.com/zicorn/llm-proxy/internal/relay/billing"
	"github.com/zicorn/llm-proxy/internal/relay/entity"
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
	// 计费与消费日志都用 ActualModelName 查 model_meta，而 GetRequestMeta 不设该字段。
	// 该端点的 OriginModelName 又被 middleware 固定成 config.DefaultFineTuningModel（只用于
	// 选渠道），不是用户要微调的模型，因此必须取请求体里的 model，否则会按占位模型定价。
	meta.OriginModelName = ftRequest.Model
	meta.ActualModelName, _ = objects.ResolveModelName(ftRequest.Model, meta.ModelMapping)

	trainingFile, err := objects.NewTrainingFile(userId, ftRequest.TrainingFile, ftRequest.HyperParameters.NumEpochs, meta.ActualModelName)
	if err != nil {
		c.JSON(500, gin.H{
			"error": "get_file_failed",
		})
		return
	}
	preConsumedQuota, quotaErr := trainingFile.GetPreConsumedQuota(meta)
	if quotaErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": quotaErr.Error(),
		})
		return
	}
	// 此前该返回值被丢弃，余额不足也会继续转发。
	// PreCost 对额度充裕的用户会返回 0（不真正预扣），退款与结算都必须用它的返回值。
	preConsumedQuota, bizErr := objects.PreCost(ctx, meta, preConsumedQuota)
	if bizErr != nil {
		c.JSON(bizErr.StatusCode, gin.H{
			"error": bizErr.Message,
		})
		return
	}

	// 退款守卫必须在任何可能 return 的语句之前注册，否则转发失败、读响应失败、
	// 反序列化失败等路径会把预扣配额永久留在用户账上。
	succeed := false
	defer func() {
		if succeed {
			return
		}
		billing.ReturnPreConsumedQuota(ctx, preConsumedQuota, meta.TokenId)
	}()

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
	if err := json.Unmarshal(body, &ftResp); err != nil {
		logger.Errorf(ctx, "parse fine-tuning response failed: %s", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "parse_response_body_failed",
		})
		return
	}
	// 计费
	if err := objects.PostConsumeFineTuningQuota(ctx, meta, ftResp.TrainedTokens, preConsumedQuota); err != nil {
		logger.Errorf(ctx, "post fine-tuning cost failed: %s", err.Error())
		c.JSON(500, gin.H{
			"error": "post_cost_failed",
		})
		return
	}
	// 预扣已在结算中冲抵，此后不能再退款，否则变成双重退还
	succeed = true

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
