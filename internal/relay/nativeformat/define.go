package nativeformat

// 支持的原生输入格式前缀
const (
	FormatOpenAI    = "openai"
	FormatAnthropic = "anthropic"
	FormatGoogle    = "gemini"
	FormatVertexAI  = "vertexai"
)

// URLPrefixToFormat 路径前缀 → 格式名
var URLPrefixToFormat = map[string]string{
	"/anthropic": FormatAnthropic,
	"/gemini":    FormatGoogle,
	"/vertexai":  FormatVertexAI,
}
