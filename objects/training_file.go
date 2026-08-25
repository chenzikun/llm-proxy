package objects

import (
	"github.com/songquanpeng/one-api/model"
	"github.com/songquanpeng/one-api/relay/billing/ratio"
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

func (file *TrainingFile) GetPreConsumedQuota() int64 {
	return int64(float64(file.Epochs) * float64(file.Tokens) / 1000 * ratio.GetFineTuningRatio(file.ModelName))
}
