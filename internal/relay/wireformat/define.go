package wireformat

// Format 标识上游 API 的 wire 格式，决定响应体如何解析。
//
// 与入站接口形式无关：客户端用 OpenAI SDK 调用 Gemini 渠道时，
// 上游返回的仍是 Gemini 格式，usage 必须按 Gemini 解析。
type Format int

const (
	// Unknown 该渠道的 wire 格式尚无对应的 usage 解析器。
	Unknown Format = iota
	OpenAI
	Gemini
	Anthropic
	// Unspecified 入站不对格式做任何声明（裸转发），跳过兼容性校验。
	Unspecified
)

func (f Format) String() string {
	switch f {
	case OpenAI:
		return "openai"
	case Gemini:
		return "gemini"
	case Anthropic:
		return "anthropic"
	case Unspecified:
		return "unspecified"
	default:
		return "unknown"
	}
}
