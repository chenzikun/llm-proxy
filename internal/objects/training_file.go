package objects

import (
	"fmt"

	model "github.com/zicorn/llm-proxy/internal/repo"
)

type TrainingFile struct {
	ObjectId  string
	Tokens    int
	Epochs    int
	ModelName string
}

func NewTrainingFile(userId int, objectId string, epochs int, modelName string) (*TrainingFile, error) {
	f, err := model.GetFileByObjectId(objectId, userId)
	if err != nil {
		return nil, err
	}
	return &TrainingFile{
		ObjectId:  objectId,
		Epochs:    epochs,
		ModelName: modelName,
		Tokens:    f.Tokens,
	}, nil
}

// GetPreConsumedQuota 计算微调任务的预扣配额。
//
// 旧实现乘的是 ratio.GetFineTuningRatio，该表为空时返回 -1，导致配额为负、
// 预扣变成给用户充值。现改为与其他模态一致地读 model_meta。
//
// 定价统一走 modelPricing，与 PostConsumeFineTuningQuota 共用同一入口，
// 保证预扣与结算不会各算一套价格。
func (file *TrainingFile) GetPreConsumedQuota(meta *Meta) (int64, error) {
	inputPriceCNY, _, groupRatio, err := modelPricing(meta)
	if err != nil {
		return 0, fmt.Errorf("模型 %s 未配置，请联系管理员在模型管理中添加", meta.ActualModelName)
	}
	// 训练按 token 计量，上游返回真实训练量之前只能按 epochs × 文件 token 数估算。
	return measuredQuota(inputPriceCNY, groupRatio, float64(file.Epochs)*float64(file.Tokens)), nil
}
