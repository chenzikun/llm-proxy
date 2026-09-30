package objects

import (
	"context"
	"fmt"
	"net/http"

	"github.com/zicorn/llm-proxy/pkg/common/logger"
)

// 视频生成（Seedance）按上游的 token 口径计费：单价是「每百万 output token」，
// 与文本模型用的是同一套 model_meta 字段，因此 billing_unit 填 token。
//
// 与图片/转写的差别在**结算时机**：上游的创建任务接口不回用量，真正的
// usage 要等任务跑完才由查询接口给出。这里选择在提交时刻按上游的估价公式
// 结算（公式见 adaptor/seedance.Estimate），代价是提交即扣费、失败不退款，
// 换来的是不需要任务表，也不会出现"预扣挂在那里没人结算"的泄漏。

// VideoUsage 是一次视频生成任务的计费用量，由渠道侧估算后交过来。
//
// 渠道包（adaptor/seedance）不能把它的结构体直接给过来：adaptor 依赖 objects，
// 反向引用会成环。所以这里另立一个结构，由 inbound 在一处做转换。
type VideoUsage struct {
	// Tokens 是按上游公式算出的 output token 数。
	Tokens int
	// HasVideoInput 表示请求带了参考视频，走的是更低的那一档单价。
	HasVideoInput bool
	// InputVideoSeconds 是实际计入公式的参考视频时长。
	// 调用方没声明时为 0 —— 那一档会偏低，日志里要写明。
	InputVideoSeconds float64
	// PriceFactor 是按分辨率档位对**单价**的倍数，由渠道侧给出。
	//
	// 它乘在单价上而不是 Tokens 上：Tokens 要如实等于上游控制台显示的用量，
	// 对账时两边必须逐位一致。为 0 时按 1 处理，免得调用方忘了赋值就把价格乘没。
	PriceFactor float64
}

// effectivePriceFactor 把未赋值的 PriceFactor 归一为 1。
func effectivePriceFactor(f float64) float64 {
	if f <= 0 {
		return 1
	}
	return f
}

// VideoSecondsUsage 是按秒计费的视频用量（Wan3.0）。
//
// 与 Seedance 的 VideoUsage 并列而不是合一：两者计量单位不同（秒 / token），
// 单价字段的含义也就不同，硬塞进一个结构只会让"这个数到底是秒还是 token"
// 在调用点看不出来。
type VideoSecondsUsage struct {
	// BilledSeconds 是**已折算到基准档**的计费秒数，直接乘单价即可。
	//
	// 折算（分辨率系数）由渠道侧完成：分辨率档位是渠道的词汇，计费层不该认识
	// "480P / 1080P" 这些字面量，它只认"多少秒"。
	BilledSeconds float64
	// RequestedSeconds 是请求里的原始输出时长，仅用于日志对照。
	RequestedSeconds float64
	// Resolution 仅用于日志。
	Resolution string
	// DurationFallback 表示秒数是兜底值（duration 缺省或为 -1），不是请求里写死的。
	DurationFallback bool
}

// PreConsumeVideoSecondsQuota 按秒预扣视频生成配额（Wan3.0）。
func PreConsumeVideoSecondsQuota(ctx context.Context, meta *Meta, usage VideoSecondsUsage) (int64, *ErrorWithStatusCode) {
	_, outputPriceCNY, groupRatio, err := modelPricing(meta)
	if err != nil {
		return 0, ErrorWrapper(err, "get_model_meta_failed", http.StatusInternalServerError)
	}
	return PreCost(ctx, meta, measuredQuota(outputPriceCNY, groupRatio, usage.BilledSeconds))
}

// PostConsumeVideoSecondsQuota 结算按秒计费的视频配额。
//
func PostConsumeVideoSecondsQuota(ctx context.Context, meta *Meta, usage VideoSecondsUsage, preConsumedQuota int64) {
	_, outputPriceCNY, groupRatio, err := modelPricing(meta)
	if err != nil {
		logger.Error(ctx, "获取视频模型元数据失败: "+err.Error())
		return
	}
	if usage.BilledSeconds <= 0 {
		// 兜底路径保证秒数不会为 0，走到这里说明调用方传错了，按 0 结算等于白送。
		logger.Error(ctx, fmt.Sprintf(
			"[视频计费] 模型 %s 算出 0 计费秒，按 0 结算，请检查 duration 参数",
			meta.ActualModelName))
	}
	if outputPriceCNY <= 0 {
		// ⚠ billing_unit=second 底下，转写读「输入价格」、视频读「输出价格」——
		// 同一个计量单位两个字段。管理员按转写的习惯把价填到输入价格上，这里就会
		// 一分不收，且除了本条日志没有任何提示。所以单价为 0 必须喊出来。
		logger.Error(ctx, fmt.Sprintf(
			"[视频计费] 模型 %s 的「输出价格」为 0，本次按 0 结算 —— 按秒计费的视频模型只读「输出价格」，请确认价格没有填到「输入价格」上",
			meta.ActualModelName))
	}
	quota := measuredQuota(outputPriceCNY, groupRatio, usage.BilledSeconds)
	logContent := videoSecondsLogContent(outputPriceCNY, groupRatio, usage)
	if err := PostCost(ctx, meta, preConsumedQuota, quota, 0, 0, 0, 0, logContent); err != nil {
		logger.Error(ctx, "error consuming video quota: "+err.Error())
	}
}

// videoSecondsLogContent 拼消费日志。单价是"每百万秒"，日志里同时给出
// 换算后的每秒单价 —— 管理员填的是每秒多少钱，账单上要能一眼对上。
//
// 分辨率折算必须写出来：管理员按 720P 定价，1080P 的请求要收两倍，
// 只写"16 计费秒"而不说为什么是 16，对账时就成了谜。
func videoSecondsLogContent(priceCNYPerM, groupRatio float64, usage VideoSecondsUsage) string {
	source := fmt.Sprintf("%.0f 秒", usage.RequestedSeconds)
	if usage.DurationFallback {
		source = fmt.Sprintf("%.0f 秒（duration 未指定或为 -1，按默认时长估）", usage.RequestedSeconds)
	}
	if usage.RequestedSeconds > 0 && usage.BilledSeconds != usage.RequestedSeconds {
		source = fmt.Sprintf("%s × 分辨率系数 %.2f = %.2f 秒等效",
			source, usage.BilledSeconds/usage.RequestedSeconds, usage.BilledSeconds)
	}
	return fmt.Sprintf("视频 ¥%.6f/秒（基准 720P，¥%.4f/M 秒），分组倍率 %.2f，分辨率 %s（%s）",
		priceCNYPerM/1000000.0, priceCNYPerM, groupRatio, usage.Resolution, source)
}

// videoPricing 取视频计费用的人民币单价与分组倍率。
//
// 上游按「请求里有没有参考视频」定两档单价：带参考视频时单价更低，但输入视频
// 的时长也计入用量 —— 价与量一起变。本代理把这两档分别落在 model_meta 的
// input_price（含参考视频）与 output_price（文生视频）上。字段名对视频模型
// 读起来别扭，但这是模型管理里仅有的两个价位，为它单引一个字段的代价更大。
func videoPricing(meta *Meta, hasVideoInput bool) (priceCNY, groupRatio float64, err error) {
	inputPriceCNY, outputPriceCNY, groupRatio, err := modelPricing(meta)
	if err != nil {
		return 0, 0, err
	}
	if hasVideoInput {
		return inputPriceCNY, groupRatio, nil
	}
	return outputPriceCNY, groupRatio, nil
}

// PreConsumeVideoQuota 预扣视频生成配额。
//
// usage.Tokens 是估价公式算出的 output token 数，因此计量单位就是 token，
// 计价与文本模型共用 measuredQuota。
func PreConsumeVideoQuota(ctx context.Context, meta *Meta, usage VideoUsage) (int64, *ErrorWithStatusCode) {
	priceCNY, groupRatio, err := videoPricing(meta, usage.HasVideoInput)
	if err != nil {
		return 0, ErrorWrapper(err, "get_model_meta_failed", http.StatusInternalServerError)
	}
	priceCNY *= effectivePriceFactor(usage.PriceFactor)
	return PreCost(ctx, meta, measuredQuota(priceCNY, groupRatio, float64(usage.Tokens)))
}

// PostConsumeVideoQuota 结算视频生成配额。
//
// 与预扣用同一个估价，差额为 0 —— 但**不能因此省掉结算**：PreCost 在用户余额
// 充足时会跳过预扣（返回 0），只有走 PostCost 才会真正扣钱并写消费日志。
func PostConsumeVideoQuota(ctx context.Context, meta *Meta, usage VideoUsage, preConsumedQuota int64) {
	priceCNY, groupRatio, err := videoPricing(meta, usage.HasVideoInput)
	if err != nil {
		logger.Error(ctx, "获取视频模型元数据失败: "+err.Error())
		return
	}
	if usage.Tokens <= 0 {
		// 公式对任何合法请求都给出正数，走到这里说明请求参数已经超出了
		// 本地能理解的范围，按 0 结算等于白送，必须留下痕迹。
		logger.Error(ctx, fmt.Sprintf(
			"[视频计费] 模型 %s 估算出 0 token，按 0 结算，请检查分辨率/时长参数",
			meta.ActualModelName))
	}
	priceCNY *= effectivePriceFactor(usage.PriceFactor)
	quota := measuredQuota(priceCNY, groupRatio, float64(usage.Tokens))
	logContent := videoLogContent(priceCNY, groupRatio, usage)
	if err := PostCost(ctx, meta, preConsumedQuota, quota, 0, usage.Tokens, 0, 0, logContent); err != nil {
		logger.Error(ctx, "error consuming video quota: "+err.Error())
	}
}

// videoLogContent 拼消费日志。日志要能独立回答"这笔为什么是这个数"，
// 因此把单价、档位与估算出的 token 数都写进去。三种档位要能一眼分开：
// 到底有没有把参考视频的时长算进去，直接决定了这个数偏低多少。
func videoLogContent(priceCNYPerM, groupRatio float64, usage VideoUsage) string {
	var tier string
	switch {
	case !usage.HasVideoInput:
		tier = "文生视频"
	case usage.InputVideoSeconds > 0:
		tier = fmt.Sprintf("含参考视频（已计入输入 %.1f 秒）", usage.InputVideoSeconds)
	default:
		// 上游会把参考视频的时长也计进用量，而那份时长不在请求里（只有一个 URL），
		// 调用方也没声明 ⇒ 只能按输出时长算，系统性偏低。写进日志，便于对账时解释差异。
		tier = "含参考视频（输入视频时长未声明，未计入估算，实际用量会更高）"
	}
	// 单价里已含分辨率台阶，日志要说明它是怎么来的：否则管理员按 720P 的价目
	// 填了单价，看到 1080P 的账单会以为算错了。
	if f := effectivePriceFactor(usage.PriceFactor); f != 1 {
		tier += fmt.Sprintf("，按分辨率单价 ×%.4f", f)
	}
	return fmt.Sprintf("视频 ¥%.4f/M tokens，分组倍率 %.2f，%s（预估 %d output tokens）",
		priceCNYPerM, groupRatio, tier, usage.Tokens)
}
