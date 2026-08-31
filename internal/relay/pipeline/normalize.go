package pipeline

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zicorn/llm-proxy/internal/objects"
	"github.com/zicorn/llm-proxy/internal/relay"
	"github.com/zicorn/llm-proxy/internal/relay/entity"
	"github.com/zicorn/llm-proxy/pkg/common/logger"
)

// executeNormalize 走归一化链路：请求转成渠道格式发出，响应由渠道适配器解析。
//
// 这里不需要 wireformat 提取器——DoResponse 已经按渠道格式解析出 usage，
// 那正是"usage 归渠道维度"这一事实在既有接口上的体现。
func executeNormalize(c *gin.Context, meta *objects.Meta, op *Operation,
	preConsumed int64) *objects.ErrorWithStatusCode {

	ctx := c.Request.Context()

	adaptor := relay.GetAdaptor(meta.APIType)
	if adaptor == nil {
		return objects.ErrorWrapper(fmt.Errorf("invalid api type: %d", meta.APIType),
			"invalid_api_type", http.StatusBadRequest)
	}
	if err := adaptor.Init(meta); err != nil {
		return objects.ErrorWrapper(err, "init_adaptor_failed", http.StatusInternalServerError)
	}

	reqBody, err := requestBodyReader(c)
	if err != nil {
		refund(ctx, meta, preConsumed)
		return objects.ErrorWrapper(err, "read_request_body_failed", http.StatusBadRequest)
	}

	resp, err := adaptor.DoRequest(c, meta, reqBody)
	if err != nil {
		refund(ctx, meta, preConsumed)
		return objects.ErrorWrapper(err, "do_request_failed", http.StatusBadGateway)
	}

	u, _, respErr := adaptor.DoResponse(c, resp, meta)
	if respErr != nil {
		refund(ctx, meta, preConsumed)
		return respErr
	}
	if !op.Billable() {
		return nil
	}

	if u == nil {
		logger.Errorf(ctx, "[pipeline] 用量解析异常：渠道适配器未返回 usage，模型 %s 按 0 结算，需补适配",
			meta.ActualModelName)
		u = &entity.Usage{}
	}
	objects.PostConsumeQuota(ctx, u, meta, preConsumed)
	return nil
}
