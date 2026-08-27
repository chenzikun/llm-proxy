package ratio

// ModelRatio 历史遗留，已清空。
// LLM 文本模型的计费现在使用 model_meta 表中的 input_price/output_price/cache_price 字段。
// 此映射仅保留以兼容音频/图片等旧计费路径（这些路径目前返回 0 即免费）。
var ModelRatio = map[string]float64{}

// FineTuningRatio 微调费率（每 1K token），单位为额度
var FineTuningRatio = map[string]float64{}

// GetModelRatio 获取模型的额度费率。
// 对于 LLM 文本接口，计费已迁移到 model_meta 表，此函数仅供音频/图片接口使用。
// 若模型未在映射中找到，返回 0（即免费）。
func GetModelRatio(name string, channelType int) float64 {
	if ratio, ok := ModelRatio[name]; ok {
		return ratio
	}
	return 0
}

func GetFineTuningRatio(name string) float64 {
	if ratio, ok := FineTuningRatio[name]; ok {
		return ratio
	}
	return -1
}
