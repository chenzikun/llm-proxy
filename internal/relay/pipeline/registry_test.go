package pipeline

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/zicorn/llm-proxy/internal/relay/wireformat"
)

func testSpec(name string) *RelaySpec {
	return &RelaySpec{
		Name: name,
		Mode: ModePassthrough,
		Resolve: func(c *gin.Context) (*Operation, error) {
			return &Operation{Model: "m", Kind: KindGenerate, InboundWire: wireformat.Gemini}, nil
		},
	}
}

func TestRegisterAndLookup(t *testing.T) {
	Register(testSpec("test.lookup"))

	got, ok := Lookup("test.lookup")
	assert.True(t, ok)
	assert.Equal(t, "test.lookup", got.Name)

	_, ok = Lookup("test.absent")
	assert.False(t, ok)
}

func TestRegisterDuplicatePanics(t *testing.T) {
	Register(testSpec("test.dup"))
	assert.Panics(t, func() { Register(testSpec("test.dup")) },
		"重复注册说明命名冲突，应在启动阶段暴露")
}

func TestRegisterRejectsIncompleteSpec(t *testing.T) {
	assert.Panics(t, func() { Register(nil) })
	assert.Panics(t, func() { Register(&RelaySpec{Name: ""}) })
	assert.Panics(t, func() { Register(&RelaySpec{Name: "test.no-resolve"}) },
		"没有 Resolve 的 spec 无法识别模型，注册即错误")
}

func TestMustLookupPanicsOnMissing(t *testing.T) {
	assert.Panics(t, func() { MustLookup("test.never-registered") },
		"路由绑定了未注册的 spec，服务不应启动")
}

func TestHandlerPanicsOnMissingSpec(t *testing.T) {
	assert.Panics(t, func() { Handler("test.also-never-registered") },
		"Handler 在路由注册阶段就要校验 spec 存在")
}

func TestOperationBillableOnlyForGenerate(t *testing.T) {
	assert.True(t, (&Operation{Kind: KindGenerate}).Billable())
	assert.False(t, (&Operation{Kind: KindMetadata}).Billable(),
		"countTokens 之类不产生用量，不应计费")
	assert.False(t, (&Operation{Kind: KindUnsupported}).Billable())
}
