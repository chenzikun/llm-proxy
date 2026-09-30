package inbound

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zicorn/llm-proxy/internal/objects"
	"github.com/zicorn/llm-proxy/internal/relay/adaptor/wan3"
	"github.com/zicorn/llm-proxy/internal/relay/nativeformat"
	"github.com/zicorn/llm-proxy/internal/relay/pipeline"
	"github.com/zicorn/llm-proxy/internal/relay/wireformat"
	"github.com/zicorn/llm-proxy/pkg/common"
	"github.com/zicorn/llm-proxy/pkg/common/ctxkey"
	"github.com/zicorn/llm-proxy/pkg/common/logger"
)

// resolveWan3 解析 /wan3 前缀的请求。
//
// 只有「创建任务」计费；查询任务（/api/v1/tasks/{id}）不产生新用量，透传即可。
func resolveWan3(c *gin.Context) (*pipeline.Operation, error) {
	stripProxyQueryParam(c, nativeformat.ModelQueryKey)

	op := &pipeline.Operation{
		// 裸转发：入站请求体就是上游格式，不做 wire 兼容校验。
		InboundWire: wireformat.Unspecified,
		Kind:        pipeline.KindMetadata,
		// TokenAuth 已解析过模型名（建单取请求体，查询取查询串），这里复用。
		Model: c.GetString(ctxkey.RequestModel),
	}
	if isWan3Create(c) {
		op.Action = "create"
		op.Kind = pipeline.KindGenerate
	}
	return op, nil
}

// isWan3Create 判断是不是「创建视频生成任务」。
//
// 后缀用 adaptor 导出的那个常量：本包据它决定计不计费，adaptor 据它决定加不加
// 异步头，两处必须认同一个值。真值判定交给 wan3.IsCreatePath，这里只补一个
// POST 的约束 —— 查询接口是 GET，不影响。
func isWan3Create(c *gin.Context) bool {
	return c.Request.Method == http.MethodPost && wan3.IsCreatePath(c.Request.URL.Path)
}

// wan3Billing 在建单时刻按秒结算一次视频生成。
//
// 百炼的查询响应里其实带着权威用量（usage.duration），但它要等任务跑完才有，
// 那时再结算就需要一张把 task_id 映射回用户/token 的表 —— 本期不做，代价是
// 建单即扣费、任务失败不退款（与图片、转写、Seedance 一致）。
type wan3Billing struct{}

func (wan3Billing) PreConsume(c *gin.Context, meta *objects.Meta, op *pipeline.Operation) (int64, *objects.ErrorWithStatusCode) {
	est, bizErr := wan3Estimate(c)
	if bizErr != nil {
		return 0, bizErr
	}
	return objects.PreConsumeVideoSecondsQuota(c.Request.Context(), meta, est)
}

func (wan3Billing) Settle(c *gin.Context, meta *objects.Meta, op *pipeline.Operation, preConsumed int64) {
	est, bizErr := wan3Estimate(c)
	if bizErr != nil {
		// 预扣阶段已经成功估过一次，能走到这里说明请求体读不出来了。
		// 此时不能不结算 —— 预扣会烂在账上，用户余额被扣走却没有任何消费记录。
		logger.Error(c.Request.Context(), "[wan3] 结算时无法重新估算用量，本次未扣费："+bizErr.Message)
		return
	}
	objects.PostConsumeVideoSecondsQuota(c.Request.Context(), meta, est, preConsumed)
}

// wan3Estimate 从缓存的请求体算出本次任务的计费秒数。
// 请求体已在 TokenAuth 阶段读过一次并被缓存，这里重读不会消耗 body。
func wan3Estimate(c *gin.Context) (objects.VideoSecondsUsage, *objects.ErrorWithStatusCode) {
	body, err := common.GetRequestBody(c)
	if err != nil {
		return objects.VideoSecondsUsage{}, objects.ErrorWrapper(err, "read_request_body_failed", http.StatusBadRequest)
	}
	est, err := wan3.EstimateFromBody(body)
	if err != nil {
		return objects.VideoSecondsUsage{}, objects.ErrorWrapper(err, "estimate_wan3_seconds_failed", http.StatusBadRequest)
	}
	if est.DurationFallback {
		// 秒数是兜底值 ⇒ 这一笔扣得可能不准。它是本渠道唯一"估算"的成分，
		// 值得在服务端日志里留痕，方便对账时解释偏差。
		logger.Warnf(c.Request.Context(),
			"[wan3] 模型 %s 未指定有效 duration，按 %.0f 秒估算计费",
			c.GetString(ctxkey.RequestModel), est.Seconds)
	}
	return objects.VideoSecondsUsage{
		BilledSeconds:    est.BillableSeconds(),
		RequestedSeconds: est.Seconds,
		Resolution:       est.Resolution,
		DurationFallback: est.DurationFallback,
	}, nil
}

func init() {
	pipeline.Register(&pipeline.RelaySpec{
		Name:       "wan3.native",
		Mode:       pipeline.ModePassthrough,
		PathPrefix: wan3.PathPrefix,
		Resolve:    resolveWan3,
		Billing:    wan3Billing{},
	})
}
