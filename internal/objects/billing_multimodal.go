package objects

import (
	"context"
	"fmt"
	"math"
	"net/http"

	model "github.com/zicorn/llm-proxy/internal/repo"
	billingratio "github.com/zicorn/llm-proxy/internal/relay/billing/ratio"
	"github.com/zicorn/llm-proxy/pkg/common/logger"
)

// preConsumedAudioSeconds 转写请求的保底预扣秒数。
//
// 音频时长要等上游响应才能得知，预扣阶段只能按一个固定值占位，结算时按真实
// duration 补差。取 600 秒是在"拦住余额不足"与"不过度阻挡正常请求"之间折中。
const preConsumedAudioSeconds = 600

// billedSeconds 按秒向上取整，与 Whisper 上游的计价方式一致。
func billedSeconds(duration float64) int64 {
	if duration <= 0 {
		return 0
	}
	return int64(math.Ceil(duration))
}

// measuredQuota 按"计量数 × 每单位额度"计算配额，是所有模态共用的唯一算法。
//
// units 是该模态的计量数：图片传 张数 × 尺寸系数，转写传取整后的秒数，
// 微调传 epochs × tokens。单位语义由 model_meta.billing_unit 声明，本函数不关心。
func measuredQuota(priceCNYPerM, groupRatio, units float64) int64 {
	if units <= 0 {
		return 0
	}
	ratio := unitQuotaRatio(priceCNYPerM, groupRatio)
	quota := int64(math.Ceil(units * ratio))
	// math.Ceil 对任意正数至少得 1，常规路径不会触发本 guard；仅在 units×ratio 恰好为 0
	// 或负数的边界上兜底，防止单价非零却算出零配额。
	if ratio != 0 && quota <= 0 {
		quota = 1
	}
	return quota
}

// modelPricing 取模型的 ¥ 单价与分组倍率。计量单位语义由 model_meta.billing_unit 声明。
func modelPricing(meta *Meta) (inputPriceCNY, outputPriceCNY, groupRatio float64, err error) {
	modelMeta, err := model.GetModelMetaByModel(meta.ActualModelName)
	if err != nil {
		return 0, 0, 0, err
	}
	inputPriceCNY, outputPriceCNY, _ = getModelPricesInCNY(modelMeta)
	return inputPriceCNY, outputPriceCNY, billingratio.GetGroupRatio(meta.Group), nil
}

// PreConsumeImageQuota 预扣图片生成配额。
//
// 张数与尺寸在请求体中已知，因此预扣值等于最终配额，结算时差额为 0。
func PreConsumeImageQuota(ctx context.Context, meta *Meta, n int, sizeRatio float64) (int64, *ErrorWithStatusCode) {
	_, outputPriceCNY, groupRatio, err := modelPricing(meta)
	if err != nil {
		return 0, ErrorWrapper(err, "get_model_meta_failed", http.StatusInternalServerError)
	}
	return PreCost(ctx, meta, measuredQuota(outputPriceCNY, groupRatio, float64(n)*sizeRatio))
}

// PostConsumeImageQuota 结算图片生成配额。
//
// 单价为 0 时仍会写消费日志，否则图片请求在日志中不可见、无法审计。
func PostConsumeImageQuota(ctx context.Context, meta *Meta, n int, sizeRatio float64, preConsumedQuota int64) {
	_, outputPriceCNY, groupRatio, err := modelPricing(meta)
	if err != nil {
		logger.Error(ctx, "获取图片模型元数据失败: "+err.Error())
		return
	}
	quota := measuredQuota(outputPriceCNY, groupRatio, float64(n)*sizeRatio)
	logContent := fmt.Sprintf("图片 ¥%.6f/张，尺寸系数 %.2f，分组倍率 %.2f（%d 张）",
		outputPriceCNY/1000000.0, sizeRatio, groupRatio, n)
	if err := PostCost(ctx, meta, preConsumedQuota, quota, 0, 0, 0, 0, logContent); err != nil {
		logger.Error(ctx, "error consuming image quota: "+err.Error())
	}
}

// PreConsumeTranscriptionQuota 按保底秒数预扣转写配额。
func PreConsumeTranscriptionQuota(ctx context.Context, meta *Meta) (int64, *ErrorWithStatusCode) {
	inputPriceCNY, _, groupRatio, err := modelPricing(meta)
	if err != nil {
		return 0, ErrorWrapper(err, "get_model_meta_failed", http.StatusInternalServerError)
	}
	return PreCost(ctx, meta, measuredQuota(inputPriceCNY, groupRatio, preConsumedAudioSeconds))
}

// PostConsumeTranscriptionQuota 按上游返回的真实时长结算转写配额。
func PostConsumeTranscriptionQuota(ctx context.Context, meta *Meta, durationSeconds float64, preConsumedQuota int64) {
	inputPriceCNY, _, groupRatio, err := modelPricing(meta)
	if err != nil {
		logger.Error(ctx, "获取音频模型元数据失败: "+err.Error())
		return
	}
	seconds := billedSeconds(durationSeconds)
	if seconds == 0 {
		logger.Error(ctx, fmt.Sprintf(
			"[转写计费] 模型 %s 未能从上游响应解析出音频时长，按 0 结算，需补适配",
			meta.ActualModelName))
	}
	quota := measuredQuota(inputPriceCNY, groupRatio, float64(seconds))
	logContent := fmt.Sprintf("音频 ¥%.6f/秒，分组倍率 %.2f（%d 秒，按秒向上取整）",
		inputPriceCNY/1000000.0, groupRatio, seconds)
	if err := PostCost(ctx, meta, preConsumedQuota, quota, 0, 0, 0, 0, logContent); err != nil {
		logger.Error(ctx, "error consuming audio quota: "+err.Error())
	}
}
