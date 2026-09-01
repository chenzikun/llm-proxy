package objects

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"

	"github.com/zicorn/llm-proxy/pkg/common/config"
	"github.com/zicorn/llm-proxy/pkg/common/logger"
	"github.com/zicorn/llm-proxy/internal/repo"
	billingratio "github.com/zicorn/llm-proxy/internal/relay/billing/ratio"
	"github.com/zicorn/llm-proxy/internal/relay/entity"
)

// getModelPricesInCNY 将模型价格统一转换为人民币（¥），每百万 token
func getModelPricesInCNY(modelMeta *model.ModelMeta) (inputPrice, outputPrice, cachePrice float64) {
	rate := 1.0
	if modelMeta.PriceUnit == "USD" {
		rate = config.ExchangeRate
	}
	return modelMeta.InputPrice * rate, modelMeta.OutputPrice * rate, modelMeta.CachePrice * rate
}

// unitQuotaRatio 根据 CNY 单价（每百万计量单位）和分组倍率计算每个计量单位的额度消耗。
//
// 计量单位由 model_meta.billing_unit 决定：token / char / second / image。
// 本函数与单位无关，只做"单价 → 每单位额度"的换算。
func unitQuotaRatio(priceCNYPerM float64, groupRatio float64) float64 {
	return priceCNYPerM * config.QuotaPerUnit / 1000000.0 * groupRatio
}

// getPreConsumedQuota 根据 token 数量计算预先扣除的费用
func getPreConsumedQuota(maxTokens int, promptTokens int, inputRatio float64) int64 {
	preConsumedTokens := config.PreConsumedQuota + int64(promptTokens)
	if maxTokens != 0 {
		preConsumedTokens += int64(maxTokens)
	}
	return int64(float64(preConsumedTokens) * inputRatio)
}

// PreConsumeQuota 预先扣除费用
func PreConsumeQuota(ctx context.Context, textRequest *entity.GeneralOpenAIRequest, meta *Meta) (int64, *ErrorWithStatusCode) {
	// TODO: rerank 模型扣费，不按照token数量，而是按照请求次数扣费 https://aws.amazon.com/cn/bedrock/pricing/
	modelMeta, err := model.GetModelMetaByModel(textRequest.Model)
	if err != nil {
		return 0, ErrorWrapper(err, "get_model_meta_failed", http.StatusInternalServerError)
	}
	inputPriceCNY, _, _ := getModelPricesInCNY(modelMeta)
	groupRatio := billingratio.GetGroupRatio(meta.Group)
	inputRatio := unitQuotaRatio(inputPriceCNY, groupRatio)

	promptTokens := PredictChatPromptTokenCount(textRequest, meta.Mode)
	meta.PromptTokens = promptTokens
	preConsumedQuota := getPreConsumedQuota(textRequest.MaxTokens, promptTokens, inputRatio)

	preConsumedQuota, err2 := PreCost(ctx, meta, preConsumedQuota)
	if err2 != nil {
		return preConsumedQuota, err2
	}
	return preConsumedQuota, nil
}

// PostConsumeQuota 实际扣除费用（LLM 文本接口）
func PostConsumeQuota(ctx context.Context, usage *entity.Usage, meta *Meta, preConsumedQuota int64) {
	if usage == nil {
		logger.Error(ctx, "usage is nil, which is unexpected")
		return
	}
	modelMeta, err := model.GetModelMetaByModel(meta.ActualModelName)
	if err != nil {
		logger.Error(ctx, "failed to get model meta: "+err.Error())
		return
	}
	inputPriceCNY, outputPriceCNY, cachePriceCNY := getModelPricesInCNY(modelMeta)
	groupRatio := billingratio.GetGroupRatio(meta.Group)

	inputRatio := unitQuotaRatio(inputPriceCNY, groupRatio)
	outputRatio := unitQuotaRatio(outputPriceCNY, groupRatio)
	cacheRatio := unitQuotaRatio(cachePriceCNY, groupRatio)

	promptTokens := usage.PromptTokens
	completionTokens := usage.CompletionTokens
	// cached_tokens 是 prompt_tokens 的子集，表示命中缓存的部分。
	// 仅当 cache_price > 0 时才启用缓存折扣：
	//   非缓存部分按 input_price，缓存命中部分按 cache_price（更低）。
	// 若 cache_price = 0（未配置），全部 prompt_tokens 统一按 input_price 计费。
	cacheTokens := usage.PromptTokensDetails.CachedTokens
	var nonCachedTokens int
	var nonCachedQuota, cacheQuota int64
	if cachePriceCNY > 0 && cacheTokens > 0 {
		nonCachedTokens = promptTokens - cacheTokens
		if nonCachedTokens < 0 {
			nonCachedTokens = 0
		}
		nonCachedQuota = int64(math.Ceil(float64(nonCachedTokens) * inputRatio))
		cacheQuota = int64(math.Ceil(float64(cacheTokens) * cacheRatio))
	} else {
		nonCachedTokens = promptTokens
		nonCachedQuota = int64(math.Ceil(float64(promptTokens) * inputRatio))
		cacheQuota = 0
	}
	completionQuota := int64(math.Ceil(float64(completionTokens) * outputRatio))
	quota := nonCachedQuota + cacheQuota + completionQuota

	totalTokens := promptTokens + completionTokens
	if totalTokens == 0 {
		quota = 0
	}
	if inputRatio != 0 && quota <= 0 && totalTokens > 0 {
		quota = 1
	}

	var logContent string
	if cachePriceCNY > 0 && cacheTokens > 0 {
		logContent = fmt.Sprintf("输入 ¥%.4f/M，输出 ¥%.4f/M，缓存折扣 ¥%.4f/M，分组倍率 %.2f（非缓存 %d，缓存命中 %d，输出 %d tokens）",
			inputPriceCNY, outputPriceCNY, cachePriceCNY, groupRatio, nonCachedTokens, cacheTokens, completionTokens)
	} else {
		logContent = fmt.Sprintf("输入 ¥%.4f/M，输出 ¥%.4f/M，分组倍率 %.2f（%d 输入，%d 输出 tokens，缓存未配置）",
			inputPriceCNY, outputPriceCNY, groupRatio, promptTokens, completionTokens)
	}
	if err := PostCost(ctx, meta, preConsumedQuota, quota, promptTokens, completionTokens, cacheTokens, int(cacheQuota), logContent); err != nil {
		logger.Error(ctx, "error consuming token remain quota: "+err.Error())
	}
}

// PreConsumeQuotaForAudio 预先扣除音频任务的费用
func PreConsumeQuotaForAudio(ctx context.Context, input string, meta *Meta) (int64, *ErrorWithStatusCode) {
	modelMeta, err := model.GetModelMetaByModel(meta.ActualModelName)
	if err != nil {
		return 0, ErrorWrapper(err, "get_model_meta_failed", http.StatusInternalServerError)
	}
	inputPriceCNY, _, _ := getModelPricesInCNY(modelMeta)
	groupRatio := billingratio.GetGroupRatio(meta.Group)
	inputRatio := unitQuotaRatio(inputPriceCNY, groupRatio)

	promptTokens := PredictAudioPromptTokenCount(input, meta.Mode)
	preConsumedQuota := getPreConsumedQuota(0, promptTokens, inputRatio)

	preConsumedQuota, err2 := PreCost(ctx, meta, preConsumedQuota)
	if err2 != nil {
		return preConsumedQuota, err2
	}
	return preConsumedQuota, nil
}

// PreConsumeQuotaByTokens 按已知的 prompt token 数预扣额度。
//
// 供不解析请求体的透传链路使用（原生 Gemini / Anthropic 前缀）：这些链路拿不到
// GeneralOpenAIRequest，无法走 PreConsumeQuota。promptTokens 传 0 表示无法预估，
// 此时只按 config.PreConsumedQuota 的保底额度做余额校验。
func PreConsumeQuotaByTokens(ctx context.Context, promptTokens int, meta *Meta) (int64, *ErrorWithStatusCode) {
	modelMeta, err := model.GetModelMetaByModel(meta.ActualModelName)
	if err != nil {
		return 0, ErrorWrapper(err, "get_model_meta_failed", http.StatusInternalServerError)
	}
	inputPriceCNY, _, _ := getModelPricesInCNY(modelMeta)
	groupRatio := billingratio.GetGroupRatio(meta.Group)
	inputRatio := unitQuotaRatio(inputPriceCNY, groupRatio)

	meta.PromptTokens = promptTokens
	return PreCost(ctx, meta, getPreConsumedQuota(0, promptTokens, inputRatio))
}

func PreCost(ctx context.Context, meta *Meta, preConsumedQuota int64) (int64, *ErrorWithStatusCode) {
	userQuota, err := model.CacheGetUserQuota(ctx, meta.UserId)
	if err != nil {
		return preConsumedQuota, ErrorWrapper(err, "get_user_quota_failed", http.StatusInternalServerError)
	}
	if userQuota < preConsumedQuota {
		return preConsumedQuota, ErrorWrapper(errors.New("user quota is not enough"), "insufficient_user_quota", http.StatusForbidden)
	}
	err = model.CacheDecreaseUserQuota(meta.UserId, preConsumedQuota)
	if err != nil {
		return preConsumedQuota, ErrorWrapper(err, "decrease_user_quota_failed", http.StatusInternalServerError)
	}
	if userQuota > 100*preConsumedQuota {
		// in this case, we do not pre-consume quota
		// because the user has enough quota
		// preConsumedQuota = 0
		logger.Info(ctx, fmt.Sprintf("user %d has enough quota %d, trusted and no need to pre-consume", meta.UserId, userQuota))
		return 0, nil
	}
	if preConsumedQuota > 0 {
		err := model.PreConsumeTokenQuota(meta.TokenId, preConsumedQuota)
		if err != nil {
			return preConsumedQuota, ErrorWrapper(err, "pre_consume_token_quota_failed", http.StatusForbidden)
		}
	}
	return preConsumedQuota, nil
}

func PostCost(ctx context.Context, meta *Meta, preConsumedQuota int64, actuallyConsumedQuota int64, promptTokens, completionTokens, cacheTokens int, cacheQuota int, logContent string) error {
	quotaDelta := actuallyConsumedQuota - preConsumedQuota
	err := model.PostConsumeTokenQuota(meta.TokenId, quotaDelta)
	if err != nil {
		logger.Error(ctx, "error consuming token remain quota: "+err.Error())
		return err
	}
	err = model.CacheUpdateUserQuota(ctx, meta.UserId)
	if err != nil {
		logger.Error(ctx, "error update user quota cache: "+err.Error())
		return err
	}
	model.UpdateUserUsedQuotaAndRequestCount(meta.UserId, actuallyConsumedQuota)
	model.UpdateChannelUsedQuota(meta.ChannelId, actuallyConsumedQuota)
	model.RecordConsumeLog(ctx, meta.UserId, meta.ChannelId, promptTokens, completionTokens, cacheTokens, meta.ActualModelName, meta.TokenName, actuallyConsumedQuota, cacheQuota, logContent, meta.SessionId)
	return nil
}
