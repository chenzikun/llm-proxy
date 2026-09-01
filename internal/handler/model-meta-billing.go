package controller

import model "github.com/zicorn/llm-proxy/internal/repo"

// validateModelMetaBillingUnits 为每条有效模型记录补齐默认 billing_unit 并校验取值。
// 就地修改 metas；若存在非法取值，返回对应模型名与 billing_unit。
func validateModelMetaBillingUnits(metas []model.ModelMeta) (invalidModel, invalidUnit string, ok bool) {
	for i := range metas {
		if metas[i].Model == "" {
			continue
		}
		if metas[i].BillingUnit == "" {
			metas[i].BillingUnit = model.BillingUnitToken
		}
		if !model.IsValidBillingUnit(metas[i].BillingUnit) {
			return metas[i].Model, metas[i].BillingUnit, false
		}
	}
	return "", "", true
}
