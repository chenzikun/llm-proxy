package pipeline

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/zicorn/llm-proxy/internal/monitor"
	"github.com/zicorn/llm-proxy/internal/objects"
	"github.com/zicorn/llm-proxy/pkg/common/ctxkey"
	"github.com/zicorn/llm-proxy/pkg/common/logger"
)

var registry = map[string]*RelaySpec{}

// Register 注册一份 spec。重复名称直接 panic，命名冲突应在启动阶段暴露。
func Register(spec *RelaySpec) {
	if spec == nil || spec.Name == "" {
		panic("pipeline: spec 必须非空且有名称")
	}
	if spec.Resolve == nil {
		panic(fmt.Sprintf("pipeline: spec %q 缺少 Resolve", spec.Name))
	}
	if _, exists := registry[spec.Name]; exists {
		panic(fmt.Sprintf("pipeline: spec %q 重复注册", spec.Name))
	}
	registry[spec.Name] = spec
}

func Lookup(name string) (*RelaySpec, bool) {
	spec, ok := registry[name]
	return spec, ok
}

// MustLookup 取出 spec，不存在则 panic。
func MustLookup(name string) *RelaySpec {
	spec, ok := Lookup(name)
	if !ok {
		panic(fmt.Sprintf("pipeline: spec %q 未注册", name))
	}
	return spec
}

// RegisteredNames 返回所有已注册的 spec 名，供完整性测试使用。
func RegisteredNames() []string {
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	return names
}

// Handler 返回绑定该 spec 的 gin 处理函数。
//
// spec 的存在性在此处（路由注册阶段）就校验，绑错名字的路由会让服务起不来，
// 而不是等到线上对账才发现计费漏了。
func Handler(specName string) gin.HandlerFunc {
	spec := MustLookup(specName)
	return func(c *gin.Context) {
		bizErr := Execute(c, spec)
		if bizErr == nil {
			emitChannelSuccess(c)
			return
		}
		emitChannelFailure(c, bizErr)
		// 透传分支可能已经把上游响应头和状态码写出去了，此时不能再写 JSON
		if c.Writer.Written() {
			return
		}
		c.JSON(bizErr.StatusCode, gin.H{"error": bizErr.Error})
	}
}

func emitChannelSuccess(c *gin.Context) {
	monitor.Emit(c.GetInt(ctxkey.ChannelId), true)
}

func emitChannelFailure(c *gin.Context, bizErr *objects.ErrorWithStatusCode) {
	ctx := c.Request.Context()
	channelId := c.GetInt(ctxkey.ChannelId)
	logger.Errorf(ctx, "[pipeline] 转发失败（渠道 %d，用户 %d）：%s",
		channelId, c.GetInt(ctxkey.UserId), bizErr.Message)
	if monitor.ShouldDisableChannel(&bizErr.Error, bizErr.StatusCode) {
		monitor.DisableChannel(channelId, c.GetString(ctxkey.ChannelName), bizErr.Message)
		return
	}
	monitor.Emit(channelId, false)
}
