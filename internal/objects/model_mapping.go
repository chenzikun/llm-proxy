package objects

// ResolveModelName 按渠道配置的映射表把用户请求的模型名换成上游实际模型名。
// 第二个返回值表示是否发生了映射。
func ResolveModelName(modelName string, mapping map[string]string) (string, bool) {
	if mapping == nil {
		return modelName, false
	}
	if mapped := mapping[modelName]; mapped != "" {
		return mapped, true
	}
	return modelName, false
}
