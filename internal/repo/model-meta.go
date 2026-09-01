package model

import (
	"github.com/zicorn/llm-proxy/pkg/common/helper"
	"gorm.io/gorm"
)

const (
	ModelStatusOn              int = iota + 1 // 已启用
	MoldeStatusForbiddenByHand                // 本渠道被手动禁用
	MoldeStatusForbiddenByAuto                // 本渠道被程序自动禁用
)

type ModelMeta struct {
	Id          int     `json:"id" gorm:"primaryKey;autoIncrement"`
	Model       string  `json:"model" gorm:"type:varchar(255);model" csv:"model"`
	ChannelType int     `json:"channel_type" gorm:"index:idx_channel_type;type:int;channel_type" csv:"channel_type"`
	Status      int     `json:"status" gorm:"default:1"`
	// 直接定价字段（每百万 token 的价格，货币由 PriceUnit 决定）
	InputPrice  float64 `json:"input_price" gorm:"column:input_price;default:0" csv:"input_price"`
	OutputPrice float64 `json:"output_price" gorm:"column:output_price;default:0" csv:"output_price"`
	CachePrice  float64 `json:"cache_price" gorm:"column:cache_price;default:0" csv:"cache_price"`
	PriceUnit   string  `json:"price_unit" gorm:"column:price_unit;default:'CNY'" csv:"price_unit"` // "CNY" 或 "USD"
	// BillingUnit 计量单位，决定 input_price / output_price 中"每百万"的单位是什么
	BillingUnit string  `json:"billing_unit" gorm:"column:billing_unit;default:'token'" csv:"billing_unit"`
	CreatedTime int64   `json:"created_time" gorm:"bigint"`
	UpdateTime  int64   `json:"update_time" gorm:"bigint"`
}

// 计量单位。价格字段恒为"每 100 万个计量单位的价格"，本枚举决定这个单位是什么。
const (
	BillingUnitToken  = "token"  // 文本、按 token 计价的图片模型
	BillingUnitChar   = "char"   // TTS，按输入字符
	BillingUnitSecond = "second" // 转写 / 翻译，按音频秒数
	BillingUnitImage  = "image"  // 按张计价的图片模型
)

// IsValidBillingUnit 校验计量单位取值。空字符串不合法，写入方需显式落 token。
func IsValidBillingUnit(unit string) bool {
	switch unit {
	case BillingUnitToken, BillingUnitChar, BillingUnitSecond, BillingUnitImage:
		return true
	}
	return false
}

func GetModelMetaByModel(model string) (*ModelMeta, error) {
	var modelMeta ModelMeta
	err := DB.Where("model = ?", model).First(&modelMeta).Error
	return &modelMeta, err
}

func GetModelListByChannelType(channelType int) ([]string, error) {
	var modelMetas []*ModelMeta
	err := DB.Where("channel_type = ?", channelType).Find(&modelMetas).Error
	if err != nil {
		return nil, err
	}
	var modelList []string
	for _, modelMeta := range modelMetas {
		modelList = append(modelList, modelMeta.Model)
	}
	return modelList, nil
}

// ========================= 列表操作 =========================

/*
GetModelMetaList
获取列表
*/

func GetModelMetaList(startIdx, pageSize int) ([]*ModelMeta, error) {
	var modelMetas []*ModelMeta
	err := DB.Offset(startIdx).Limit(pageSize).Find(&modelMetas).Error
	return modelMetas, err
}

func ModelMetaExists(model string) (bool, error) {
	var modelMeta ModelMeta
	err := DB.Where("model = ?", model).First(&modelMeta).Error
	if err == gorm.ErrRecordNotFound {
		return false, nil
	}
	return modelMeta.Id != 0, err
}

/*
AddModelMeta
添加新的 ModelMeta 记录
*/
func AddModelMeta(meta *ModelMeta) error {
	// 设置创建时间和更新时间
	meta.CreatedTime = helper.GetTimestamp()
	meta.UpdateTime = meta.CreatedTime

	// 插入新记录
	return DB.Create(meta).Error
}

/*Get
 */
func (model_meta *ModelMeta) Get() error {
	err := DB.First(model_meta).Error
	return err
}

/*
Update 删除模型
*/
func ModelMetaUpdate(Id int, updates map[string]interface{}) error {
	// 设置创建时间和更新时间
	updates["update_time"] = helper.GetTimestamp()
	err := DB.Model(&ModelMeta{}).
		Where("id = ?", Id).
		Updates(updates).Error
	//err := DB.Model(model_meta).Updates(model_meta).Error
	return err
}

/*
ModelMetaUpdateByModel
根据model更新模型元数据
*/
func ModelMetaUpdateByModel(model string, updates map[string]interface{}) error {
	// 设置创建时间和更新时间
	updates["update_time"] = helper.GetTimestamp()
	return DB.Model(&ModelMeta{}).Where("model = ?", model).Updates(updates).Error
}

/*
CreateOrUpdateModelMeta
插入or更新模型元数据
*/
func CreateOrUpdateModelMeta(modelMeta *ModelMeta) error {
	exist, err := ModelMetaExists(modelMeta.Model)
	if err != nil {
		return err
	}
	if exist {
		return ModelMetaUpdateByModel(modelMeta.Model, map[string]interface{}{
			"input_price":  modelMeta.InputPrice,
			"output_price": modelMeta.OutputPrice,
			"cache_price":  modelMeta.CachePrice,
			"price_unit":   modelMeta.PriceUnit,
			"channel_type": modelMeta.ChannelType,
		})
	}
	return AddModelMeta(modelMeta)
}

/*
Delete 删除模型
*/
func (model_meta *ModelMeta) Delete() error {
	// 设置创建时间和更新时间
	return DB.Delete(model_meta).Error
}

// SearchModelMetas 根据关键字搜索模型元数据
func SearchModelMetas(keyword string) ([]*ModelMeta, error) {
	var modelMetas []*ModelMeta
	err := DB.Where("model LIKE ?", "%"+keyword+"%").Find(&modelMetas).Error
	return modelMetas, err
}

// ------------- 用户程序内部，获取可用模型列表 ------------------

/*
GetModelListByChannel
根据chanel, Model获取model详情, Status=true
*/
func GetModelListByChannel(channelType int) ([]string, error) {
	var models []string
	err := DB.Model(&ModelMeta{}).Where(" channel_type = ? AND status = ? ", channelType, ModelStatusOn).Pluck("model", &models).Error
	if err != nil {
		return nil, err
	}
	return models, nil
}

/*
GetModelMeta
根据channel, model获取模型详情，Status=1
*/
func GetModelMeta(channelType int, model string) (*ModelMeta, error) {
	var modelMeta ModelMeta
	err := DB.Where("channel_type = ? AND model = ? AND status", channelType, model, ModelStatusOn).First(&modelMeta).Error
	if err != nil {
		return nil, err
	}
	return &modelMeta, nil
}

/*_GetModelMetaList 没有分页，用于内部请求*/
// func _GetModelMetaList() ([]*ModelMeta, error) {
// 	var modelMetas []*ModelMeta
// 	err := DB.Find(&modelMetas).Error
// 	return modelMetas, err
// }
