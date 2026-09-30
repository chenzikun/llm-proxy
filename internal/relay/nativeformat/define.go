package nativeformat

// 支持的原生输入格式前缀
const (
	FormatOpenAI    = "openai"
	FormatAnthropic = "anthropic"
	FormatGoogle    = "gemini"
	FormatVertexAI  = "vertexai"
	FormatSeedance  = "seedance"
	FormatWan3      = "wan3"
)

// 下面两个查询参数只属于本代理，转发前会被摘掉。它们定义在这里而不是各自包内：
// 读它的（nativeformat）与摘它的（pipeline/inbound）是两个包，各写一份字面量，
// 改一处漏一处时的症状是"参数一路发到上游"或"选不到渠道"，都不会报错。
const (
	// ModelQueryKey 指定模型名。
	//
	// 异步视频渠道（Seedance / Wan3）的查询任务接口请求体是空的、路径里也没有
	// 模型名，上游的响应里同样不回，没有它 Distribute 无法挑渠道。
	ModelQueryKey = "model"

	// SeedanceInputDurationQueryKey 声明参考视频的总时长（秒）。
	// 仅 Seedance 需要：它把输入视频时长也计入用量，而请求里只有 URL。
	// 不传即退回标准行为（只按输出时长估），不会报错。
	SeedanceInputDurationQueryKey = "input_video_duration"
)

// URLPrefixToFormat 路径前缀 → 格式名
var URLPrefixToFormat = map[string]string{
	"/anthropic": FormatAnthropic,
	"/gemini":    FormatGoogle,
	"/vertexai":  FormatVertexAI,
	"/seedance":  FormatSeedance,
	"/wan3":      FormatWan3,
}
