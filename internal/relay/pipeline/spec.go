package pipeline

import (
	"github.com/gin-gonic/gin"
	"github.com/zicorn/llm-proxy/internal/objects"
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
	// APIVersion 客户端在入站路径上指定的上游 API 版本（如 v1beta）。
	//
	// 透传模式下必须尊重客户端的选择：原生 SDK 大多打 v1beta，而渠道配置的
	// 默认值是 v1，用默认值覆盖会让新模型的特性不可用。为空表示路径中没有
	// 版本段，此时沿用渠道配置。
	APIVersion string
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
	// Billing 非 nil 时由它接管计费，通用链路不再介入。
	Billing Billing
}

// Billing 让「用量不在响应里」的渠道自己决定扣多少。
//
// 通用链路的用量来自响应体（wireformat 的提取器）。Seedance 这类异步视频渠道
// 恰好相反：创建任务的响应里只有 task id，真正的 token 用量要等任务跑完、查
// 询接口才给得出来，而费用在提交那一刻就已经确定（上游的估价公式只依赖请求
// 参数）。这种渠道必须自己算 —— 让通用提取器去啃它们的响应，只会静默结算 0：
// 不报错、不计费，在日志里与"这笔是免费的"长得一模一样。
type Billing interface {
	// PreConsume 预扣额度并返回预扣值；返回错误则拒绝本次请求。
	PreConsume(c *gin.Context, meta *objects.Meta, op *Operation) (int64, *objects.ErrorWithStatusCode)
	// Settle 结算。preConsumed 是 PreConsume 的返回值。
	Settle(c *gin.Context, meta *objects.Meta, op *Operation, preConsumed int64)
}
