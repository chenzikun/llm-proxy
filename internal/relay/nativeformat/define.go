package nativeformat

// 支持的原生输入格式前缀
const (
	FormatOpenAI    = "openai"
	FormatAnthropic = "anthropic"
	FormatGoogle    = "gemini"
	FormatVertexAI  = "vertexai"
	FormatSeedance  = "seedance"
)

// Seedance 的两个查询参数只属于本代理，转发前会被摘掉。它们定义在这里而不是
// 各自包内：读它的（nativeformat）与摘它的（pipeline/inbound）是两个包，
// 各写一份字面量，改一处漏一处时的症状是"参数一路发到上游"或"选不到渠道"，
// 都不会报错。
const (
	// SeedanceModelQueryKey 指定模型名。查询任务的请求体是空的，上游的响应里
	// 也不回模型名，没有它 Distribute 无法挑渠道。
	SeedanceModelQueryKey = "model"

	// SeedanceInputDurationQueryKey 声明参考视频的总时长（秒）。
	// 上游把输入视频时长也计入用量，而请求里只有 URL，本地算不出来。
	// 不传即退回标准行为（只按输出时长估），不会报错。
	SeedanceInputDurationQueryKey = "input_video_duration"
)

// URLPrefixToFormat 路径前缀 → 格式名
var URLPrefixToFormat = map[string]string{
	"/anthropic": FormatAnthropic,
	"/gemini":    FormatGoogle,
	"/vertexai":  FormatVertexAI,
	"/seedance":  FormatSeedance,
}
