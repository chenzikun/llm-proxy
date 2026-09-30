package inbound

import (
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/zicorn/llm-proxy/internal/objects"
	"github.com/zicorn/llm-proxy/internal/relay/adaptor/seedance"
	"github.com/zicorn/llm-proxy/internal/relay/nativeformat"
	"github.com/zicorn/llm-proxy/internal/relay/pipeline"
	"github.com/zicorn/llm-proxy/internal/relay/wireformat"
	"github.com/zicorn/llm-proxy/pkg/common"
	"github.com/zicorn/llm-proxy/pkg/common/ctxkey"
	"github.com/zicorn/llm-proxy/pkg/common/logger"
)

// seedanceTasksPath 是任务集合路径的结尾段（不含 API 版本）。
const seedanceTasksPath = "/contents/generations/tasks"

// seedanceMaxInputVideoSeconds 是 input_video_duration 的上限。
//
// 上游把参考视频的时长也计入用量，客户端声明错了会直接反映成多扣/少扣。
// 设上限是为了拦住"把毫秒当秒填"这类整数级错误（5000ms 填成 5000），
// Seedance 的输出上限才 30 秒，参考视频超过 10 分钟不现实。
const seedanceMaxInputVideoSeconds = 600

// seedanceInputSecondsCtxKey 暂存解析好的输入视频时长。
//
// Resolve 一读完就得把参数从查询串摘掉（上游不认识它），而计费要等到
// PreConsume 才发生，两者之间只能靠 context 传递。
const seedanceInputSecondsCtxKey = "seedance_input_video_seconds"

// resolveSeedance 解析 /seedance 前缀的请求。
//
// 只有「创建任务」计费，其余路径（查询任务、查询列表，以及 KYC 资产库那几个
// 需要 AK/SK 签名的接口）一律放行为 KindMetadata，原样透传。
func resolveSeedance(c *gin.Context) (*pipeline.Operation, error) {
	// 先把本代理自己的参数读出来再摘掉：它们是给代理看的，转发到 BytePlus
	// 只会是未知参数。读在前、摘在后，顺序不能反。
	inputSeconds, err := takeInputVideoSeconds(c)
	if err != nil {
		return nil, err
	}
	c.Set(seedanceInputSecondsCtxKey, inputSeconds)
	// 两个参数都只给代理看，BytePlus 不认识它们。
	stripProxyQueryParam(c, nativeformat.ModelQueryKey)
	stripProxyQueryParam(c, nativeformat.SeedanceInputDurationQueryKey)

	op := &pipeline.Operation{
		// 裸转发：入站请求体就是上游格式，不做 wire 兼容校验。
		InboundWire: wireformat.Unspecified,
		Kind:        pipeline.KindMetadata,
		// TokenAuth 已解析过模型名（创建取请求体，查询取查询串），这里复用。
		Model: c.GetString(ctxkey.RequestModel),
	}
	if isSeedanceCreate(c) {
		op.Action = "create"
		op.Kind = pipeline.KindGenerate
	}
	return op, nil
}

// takeInputVideoSeconds 读取 input_video_duration，返回声明的参考视频总时长。
//
// 不传返回 0（退回标准行为：只按输出时长估），传了但值不合法则报错 ——
// 静默忽略一个填错的时长，等于按错误的口径扣钱，而且账单上看不出来。
func takeInputVideoSeconds(c *gin.Context) (float64, error) {
	key := nativeformat.SeedanceInputDurationQueryKey
	raw := c.Query(key)
	if raw == "" {
		return 0, nil
	}
	seconds, err := strconv.ParseFloat(raw, 64)
	if err != nil || math.IsNaN(seconds) || seconds < 0 {
		return 0, fmt.Errorf("%s 必须是不小于 0 的秒数，收到 %q", key, raw)
	}
	if seconds > seedanceMaxInputVideoSeconds {
		return 0, fmt.Errorf("%s 超出上限 %d 秒，收到 %q（注意单位是秒，不是毫秒）",
			key, seedanceMaxInputVideoSeconds, raw)
	}
	return seconds, nil
}

// seedanceInputVideoSeconds 取出 resolveSeedance 暂存的输入视频时长。
func seedanceInputVideoSeconds(c *gin.Context) float64 {
	v, ok := c.Get(seedanceInputSecondsCtxKey)
	if !ok {
		return 0
	}
	seconds, _ := v.(float64)
	return seconds
}

// isSeedanceCreate 判断是不是「创建视频生成任务」。
//
// 按路径**后缀**匹配而不是全等：客户端把 base URL 配成 .../seedance 时路径是
// /api/v3/contents/generations/tasks，配成 .../seedance/api 时就是 /v3/...，
// 全等匹配会在版本段变化时静默落到 KindMetadata —— 那就是静默漏计费。
func isSeedanceCreate(c *gin.Context) bool {
	if c.Request.Method != http.MethodPost {
		return false
	}
	path := strings.TrimPrefix(c.Request.URL.Path, seedance.PathPrefix)
	return strings.HasSuffix(strings.TrimRight(path, "/"), seedanceTasksPath)
}

// stripProxyQueryParam 摘掉只属于本代理的查询参数。
//
// 不摘掉的话它会随请求一起发给 BytePlus —— 上游没有这个参数，轻则被忽略，
// 重则因未知参数报错。
func stripProxyQueryParam(c *gin.Context, key string) {
	q := c.Request.URL.Query()
	if _, ok := q[key]; !ok {
		return
	}
	q.Del(key)
	c.Request.URL.RawQuery = q.Encode()
}

// seedanceBilling 在**提交时刻**按上游的估价公式结算一次视频生成。
//
// 之所以不用通用链路：创建任务的响应里没有用量（只有 task id 与 created_at），
// 通用链路的 wireformat 提取器对它永远只能结算 0，且不报错。真正的 usage 要等
// 查询接口才有，但那时再结算就需要一张把 task_id 映射回用户/token 的表 ——
// 本期不做，代价是提交即扣费、任务失败不退款（与图片、转写一致）。
type seedanceBilling struct{}

func (seedanceBilling) PreConsume(c *gin.Context, meta *objects.Meta, op *pipeline.Operation) (int64, *objects.ErrorWithStatusCode) {
	est, bizErr := seedanceEstimate(c, meta)
	if bizErr != nil {
		return 0, bizErr
	}
	return objects.PreConsumeVideoQuota(c.Request.Context(), meta, est)
}

func (seedanceBilling) Settle(c *gin.Context, meta *objects.Meta, op *pipeline.Operation, preConsumed int64) {
	est, bizErr := seedanceEstimate(c, meta)
	if bizErr != nil {
		// 预扣阶段已经成功估过一次，能走到这里说明请求体读不出来了。
		// 此时不能不结算 —— 预扣会烂在账上，用户余额被扣走却没有任何消费记录。
		logger.Error(c.Request.Context(), "[seedance] 结算时无法重新估算用量，本次未扣费："+bizErr.Message)
		return
	}
	objects.PostConsumeVideoQuota(c.Request.Context(), meta, est, preConsumed)
}

// seedanceEstimate 从缓存的请求体算出本次任务的用量。
// 请求体已在 TokenAuth 阶段读过一次并被缓存，这里重读不会消耗 body。
func seedanceEstimate(c *gin.Context, meta *objects.Meta) (objects.VideoUsage, *objects.ErrorWithStatusCode) {
	body, err := common.GetRequestBody(c)
	if err != nil {
		return objects.VideoUsage{}, objects.ErrorWrapper(err, "read_request_body_failed", http.StatusBadRequest)
	}
	est, err := seedance.EstimateFromBody(body, meta.ActualModelName, seedanceInputVideoSeconds(c))
	if err != nil {
		return objects.VideoUsage{}, objects.ErrorWrapper(err, "estimate_seedance_tokens_failed", http.StatusBadRequest)
	}
	if declared := seedanceInputVideoSeconds(c); declared > 0 && !est.HasVideoInput {
		// 客户端声明了参考视频时长，请求里却没有 video_url。按上游公式那一项就是 0，
		// 所以这里忽略它 —— 但要说一声，否则客户端会以为这笔已经算进去了。
		logger.Warnf(c.Request.Context(),
			"[seedance] 请求声明了 %s=%.1f 但没有参考视频，该时长未参与计费",
			nativeformat.SeedanceInputDurationQueryKey, declared)
	}
	return toVideoUsage(est), nil
}

// toVideoUsage 把渠道侧的估算结果翻成计费层认的用量。
//
// objects 不能反向 import 本包的 adaptor（adaptor 依赖 objects），所以两边各有一个
// 结构体，转换点就摆在这里一处。
func toVideoUsage(est seedance.Estimate) objects.VideoUsage {
	return objects.VideoUsage{
		Tokens:            est.Tokens,
		HasVideoInput:     est.HasVideoInput,
		InputVideoSeconds: est.InputVideoSeconds,
	}
}

func init() {
	pipeline.Register(&pipeline.RelaySpec{
		Name:       "seedance.native",
		Mode:       pipeline.ModePassthrough,
		PathPrefix: seedance.PathPrefix,
		Resolve:    resolveSeedance,
		Billing:    seedanceBilling{},
	})
}
