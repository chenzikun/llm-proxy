package pipeline

import (
	"github.com/gin-gonic/gin"
	"github.com/zicorn/llm-proxy/internal/relay/wireformat"
)

// Mode 决定管线走哪个转发分支。
type Mode int

const (
	// ModeNormalize 入站请求转成内部表示再发给上游，响应转回入站格式，可配任意渠道。
	ModeNormalize Mode = iota
	// ModePassthrough 请求体原样转发，要求入站 wire 格式与上游一致。
	ModePassthrough
)

// Kind 区分一次原生请求属于哪类操作，决定上游 URL 怎么来。
type Kind int

const (
	// KindGenerate 会话生成类操作，计费。上游 URL 由渠道适配器构造，
	// 因此 Vertex 这类路径结构特殊、需要 OAuth 的渠道也能正确寻址。
	KindGenerate Kind = iota
	// KindMetadata 不产生 token 用量的元数据操作（countTokens、模型列表）。
	// 不计费，上游 URL 按「渠道 BaseURL + 入站路径」直接拼接。
	KindMetadata
	// KindUnsupported 会产生用量但管线尚不支持寻址的操作（如 :embedContent）。
	// 直接拒绝，不能放行——放行就是静默漏计费。
	KindUnsupported
)

// Operation 是按请求解析出的结果。
//
// Billable 与 wire 格式不能做成 RelaySpec 的静态字段：同一前缀下不同操作的计费
// 属性不同（:generateContent 计费，:countTokens 不计费），同一渠道下不同模型的
// wire 格式也不同（Vertex 上 gemini-* 与 claude-* 各走一套）。
type Operation struct {
	Model string
	Kind  Kind
	// InboundWire 入站声明的 wire 格式，ModePassthrough 下与上游格式做兼容校验。
	// wireformat.Unspecified 表示裸转发不做声明，跳过校验。
	InboundWire wireformat.Format
	IsStream    bool
	// Action 仅用于错误信息，便于定位是哪个原生操作未被支持。
	Action string
}

// Billable 报告该操作是否需要计费。
func (o *Operation) Billable() bool {
	return o.Kind == KindGenerate
}

// RelaySpec 按路由前缀注册，只负责识别本次请求是什么。
//
// 上游请求的构造不在此处——那是渠道维度的职责，由 RelayAdaptor 的 GetRequestURL
// 与 SetupRequestHeader 完成。入站层重新实现 URL 与鉴权会导致两处漂移，被删掉的
// native.go 正因自行拼 URL 而在 Vertex 渠道下发错地址。
type RelaySpec struct {
	Name string
	Mode Mode
	// PathPrefix 本代理暴露的路由前缀（如 /gemini），KindMetadata 透传时用于还原上游路径。
	PathPrefix string
	Resolve    func(c *gin.Context) (*Operation, error)
}
