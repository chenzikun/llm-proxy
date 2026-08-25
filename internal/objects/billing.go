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

// getPreConsumedQuota 根据token数量计算预先扣除的费用
// TODO: 这里是不是有问题，为什么要加上 MaxTokens？
func getPreConsumedQuota(maxTokens int, promptTokens int, ratio float64) int64 {
	preConsumedTokens := config.PreConsumedQuota + int64(promptTokens)
	if maxTokens != 0 {
		preConsumedTokens += int64(maxTokens)
	}
	return int64(float64(preConsumedTokens) * ratio)
}

// PreConsumeQuota 预先扣除费用
func PreConsumeQuota(ctx context.Context, textRequest *entity.GeneralOpenAIRequest, meta *Meta) (int64, *ErrorWithStatusCode) {
	// get model ratio & group ratio
	// TODO: rerank 模型扣费，不按照token数量，而是按照请求次数扣费 https://aws.amazon.com/cn/bedrock/pricing/
	modelMeta, err := model.GetModelMetaByModel(textRequest.Model)
	if err != nil {
		return 0, ErrorWrapper(err, "get_model_meta_failed", http.StatusInternalServerError)
	}
	// modelRatio := billingratio.GetModelRatio(textRequest.Model, meta.ChannelType)
	modelRatio := modelMeta.ModelRatio
	groupRatio := billingratio.GetGroupRatio(meta.Group)
	ratio := modelRatio * groupRatio
	// pre-consume quota
	promptTokens := PredictChatPromptTokenCount(textRequest, meta.Mode)
	meta.PromptTokens = promptTokens
	preConsumedQuota := getPreConsumedQuota(textRequest.MaxTokens, promptTokens, ratio)

	//userQuota, err := model.CacheGetUserQuota(ctx, meta.UserId)
	//if err != nil {
	//	return preConsumedQuota, ErrorWrapper(err, "get_user_quota_failed", http.StatusInternalServerError)
	//}
	//if userQuota-preConsumedQuota < 0 {
	//	return preConsumedQuota, ErrorWrapper(errors.New("user quota is not enough"), "insufficient_user_quota", http.StatusForbidden)
	//}
	//err = model.CacheDecreaseUserQuota(meta.UserId, preConsumedQuota)
	//if err != nil {
	//	return preConsumedQuota, ErrorWrapper(err, "decrease_user_quota_failed", http.StatusInternalServerError)
	//}
	//if userQuota > 100*preConsumedQuota {
	//	// in this case, we do not pre-consume quota
	//	// because the user has enough quota
	//	// preConsumedQuota = 0
	//	logger.Info(ctx, fmt.Sprintf("user %d has enough quota %d, trusted and no need to pre-consume", meta.UserId, userQuota))
	//	return 0, nil
	//}
	//if preConsumedQuota > 0 {
	//	err := model.PreConsumeTokenQuota(meta.TokenId, preConsumedQuota)
	//	if err != nil {
	//		return preConsumedQuota, ErrorWrapper(err, "pre_consume_token_quota_failed", http.StatusForbidden)
	//	}
	//}
	preConsumedQuota, err2 := PreCost(ctx, meta, preConsumedQuota)
	if err2 != nil {
		return preConsumedQuota, err2
	}
	return preConsumedQuota, nil
}

// PostConsumeQuota 实际扣除费用
func PostConsumeQuota(ctx context.Context, usage *entity.Usage, meta *Meta, preConsumedQuota int64) {
	if usage == nil {
		logger.Error(ctx, "usage is nil, which is unexpected")
		return
	}
	// get model ratio & group ratio
	// modelRatio := billingratio.GetModelRatio(meta.ActualModelName, meta.ChannelType)
	modelMeta, err := model.GetModelMetaByModel(meta.ActualModelName)
	if err != nil {
		return
	}
	modelRatio := modelMeta.ModelRatio
	groupRatio := billingratio.GetGroupRatio(meta.Group)
	ratio := modelRatio * groupRatio

	var quota int64
	// completionRatio := billingratio.GetCompletionRatio(meta.ActualModelName, meta.ChannelType)
	completionRatio := modelMeta.CompletionRatio
	promptTokens := usage.PromptTokens
	completionTokens := usage.CompletionTokens
	quota = int64(math.Ceil((float64(promptTokens)*ratio + float64(completionTokens)*completionRatio)))
	if ratio != 0 && quota <= 0 {
		quota = 1
	}
	totalTokens := promptTokens + completionTokens
	if totalTokens == 0 {
		// in this case, must be some error happened
		// we cannot just return, because we may have to return the pre-consumed quota
		quota = 0
	}
	logContent := fmt.Sprintf("模型倍率 %.2f，补全倍率 %.2f", modelRatio, completionRatio)
	if err := PostCost(ctx, meta, preConsumedQuota, quota, promptTokens, completionTokens, logContent); err != nil {
		logger.Error(ctx, "error consuming token remain quota: "+err.Error())
		//return
	}
	//quotaDelta := quota - preConsumedQuota
	//err := model.PostConsumeTokenQuota(meta.TokenId, quotaDelta)
	//if err != nil {
	//	logger.Error(ctx, "error consuming token remain quota: "+err.Error())
	//}
	//err = model.CacheUpdateUserQuota(ctx, meta.UserId)
	//if err != nil {
	//	logger.Error(ctx, "error update user quota cache: "+err.Error())
	//}
	//model.UpdateUserUsedQuotaAndRequestCount(meta.UserId, quota)
	//model.UpdateChannelUsedQuota(meta.ChannelId, quota)
	//model.RecordConsumeLog(ctx, meta.UserId, meta.ChannelId, promptTokens, completionTokens, meta.OriginModelName, meta.TokenName, quota, logContent)
}

// PreConsumeQuotaForAudio 预先扣除音频任务的费用
func PreConsumeQuotaForAudio(ctx context.Context, input string, meta *Meta) (int64, *ErrorWithStatusCode) {
	// modelRatio := billingratio.GetModelRatio(meta.ActualModelName, meta.ChannelType)
	modelMeta, err := model.GetModelMetaByModel(meta.ActualModelName)
	if err != nil {
		return 0, ErrorWrapper(err, "get_model_meta_failed", http.StatusInternalServerError)
	}
	modelRatio := modelMeta.ModelRatio
	groupRatio := billingratio.GetGroupRatio(meta.Group)
	ratio := modelRatio * groupRatio

	promptTokens := PredictAudioPromptTokenCount(input, meta.Mode)
	preConsumedQuota := getPreConsumedQuota(0, promptTokens, ratio)

	//userQuota, err := model.CacheGetUserQuota(ctx, meta.UserId)
	//if err != nil {
	//	return preConsumedQuota, ErrorWrapper(err, "get_user_quota_failed", http.StatusInternalServerError)
	//}
	//
	//// Check if user quota is enough
	//if userQuota-preConsumedQuota < 0 {
	//	return preConsumedQuota, ErrorWrapper(errors.New("user quota is not enough"), "insufficient_user_quota", http.StatusForbidden)
	//}
	//err = model.CacheDecreaseUserQuota(meta.UserId, preConsumedQuota)
	//if err != nil {
	//	return preConsumedQuota, ErrorWrapper(err, "decrease_user_quota_failed", http.StatusInternalServerError)
	//}
	//if userQuota > 100*preConsumedQuota {
	//	// in this case, we do not pre-consume quota
	//	// because the user has enough quota
	//	preConsumedQuota = 0
	//}
	//if preConsumedQuota > 0 {
	//	err := model.PreConsumeTokenQuota(meta.TokenId, preConsumedQuota)
	//	if err != nil {
	//		return preConsumedQuota, ErrorWrapper(err, "pre_consume_token_quota_failed", http.StatusForbidden)
	//	}
	//}
	preConsumedQuota, err2 := PreCost(ctx, meta, preConsumedQuota)
	if err2 != nil {
		return preConsumedQuota, err2
	}
	return preConsumedQuota, nil
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

func PostCost(ctx context.Context, meta *Meta, preConsumedQuota int64, actuallyConsumedQuota int64, promptTokens, completionTokens int, logContent string) error {
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
	model.RecordConsumeLog(ctx, meta.UserId, meta.ChannelId, promptTokens, completionTokens, meta.ActualModelName, meta.TokenName, actuallyConsumedQuota, logContent, meta.SessionId)
	return nil
}
