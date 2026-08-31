# 统一计费管线 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把计费收进统一管线内部并在路由注册时强制绑定，使"新增转发路由漏接计费"在机制上不可能发生，同时消除四处已知的零计费漏洞。

**Architecture:** 引入 `wireformat` 包，把"上游 wire 格式由 (渠道类型, 模型) 决定"这一事实显式化，usage 解析器按 wire 格式注册而非按入站路由。引入 `pipeline` 包独占对上游发 HTTP 请求的能力，计费写在管线内部，路由注册时必须绑定 spec 名，未注册则 init 阶段 panic。

**Tech Stack:** Go 1.23（go.mod 声明 1.21 + toolchain 1.23）、gin、gorm、testify

## Global Constraints

- 设计依据：`docs/superpowers/specs/2026-08-31-billing-pipeline-design.md`
- 本地构建需覆盖 gvm 的 Go 1.20：`export PATH="/opt/homebrew/opt/go/bin:$PATH" && export GOROOT="/opt/homebrew/opt/go/libexec" && unset GOPATH GOBIN`
- 测试框架用 `github.com/stretchr/testify/assert`，与 `internal/relay/adaptor/aws/llama3/main_test.go` 一致
- `internal/repo` 的包名是 `model`，导入路径是 `github.com/zicorn/llm-proxy/internal/repo`
- 计费策略：模型不在 `model_meta` 中则拒绝请求；价格为 0 则免费放行；usage 解析失败则扣 0 并写异常日志
- 注释只写代码本身无法表达的约束，不写"这一行在做什么"

---

### Task 1: wireformat 包 — Format 枚举与 (渠道, 模型) → 格式解析

**Files:**
- Create: `internal/relay/wireformat/define.go`
- Create: `internal/relay/wireformat/resolve.go`
- Test: `internal/relay/wireformat/resolve_test.go`

**Interfaces:**
- Consumes: `internal/relay/apitype`、`internal/relay/channeltype` 的既有常量与 `ToAPIType`
- Produces:
  - `wireformat.Format` int 枚举，值 `Unknown / OpenAI / Gemini / Anthropic / Unspecified`
  - `func (Format) String() string`
  - `func Resolve(channelType int, model string) Format`
  - `func IsVertexGeminiModel(model string) bool`

- [ ] **Step 1: 写失败的测试**

创建 `internal/relay/wireformat/resolve_test.go`：

```go
package wireformat

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/zicorn/llm-proxy/internal/relay/channeltype"
)

func TestResolve(t *testing.T) {
	cases := []struct {
		name        string
		channelType int
		model       string
		want        Format
	}{
		{"OpenAI 渠道", channeltype.OpenAI, "gpt-4o", OpenAI},
		{"Azure 走 OpenAI 兼容", channeltype.Azure, "gpt-4o", OpenAI},
		{"Gemini AI Studio", channeltype.Gemini, "gemini-2.5-flash", Gemini},
		{"Anthropic", channeltype.Anthropic, "claude-sonnet-4", Anthropic},
		{"Bedrock Claude 是 Anthropic wire", channeltype.AwsClaude, "claude-sonnet-4", Anthropic},
		{"Vertex 上的 Gemini", channeltype.VertextAI, "gemini-2.5-flash", Gemini},
		{"Vertex 上的 Claude", channeltype.VertextAI, "claude-sonnet-4", Anthropic},
		{"Vertex 自定义 endpoint 走 Gemini", channeltype.VertextAI, "endpoints/123", Gemini},
		{"暂无提取器的渠道", channeltype.Baidu, "ernie-4.0", Unknown},
		{"Proxy 渠道格式不定", channeltype.Proxy, "", Unknown},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, Resolve(c.channelType, c.model))
		})
	}
}

func TestIsVertexGeminiModel(t *testing.T) {
	assert.True(t, IsVertexGeminiModel("gemini-2.5-flash"))
	assert.True(t, IsVertexGeminiModel("endpoints/123"))
	assert.False(t, IsVertexGeminiModel("claude-sonnet-4"))
	assert.False(t, IsVertexGeminiModel(""))
}

func TestFormatString(t *testing.T) {
	assert.Equal(t, "openai", OpenAI.String())
	assert.Equal(t, "gemini", Gemini.String())
	assert.Equal(t, "anthropic", Anthropic.String())
	assert.Equal(t, "unspecified", Unspecified.String())
	assert.Equal(t, "unknown", Unknown.String())
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
go test ./internal/relay/wireformat/ -v
```

预期：编译失败，`undefined: Resolve`

- [ ] **Step 3: 写实现**

创建 `internal/relay/wireformat/define.go`：

```go
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
```

创建 `internal/relay/wireformat/resolve.go`：

```go
package wireformat

import (
	"strings"

	"github.com/zicorn/llm-proxy/internal/relay/apitype"
	"github.com/zicorn/llm-proxy/internal/relay/channeltype"
)

// IsVertexGeminiModel 判断 Vertex AI 上的模型走哪种 wire 格式。
//
// 判定条件必须与 vertexai.Adaptor.GetRequestURL 一致：gemini 与 endpoints
// 前缀走 :generateContent（Gemini 格式），其余走 :rawPredict（Anthropic 格式）。
// 两处不一致会导致 URL 与响应解析器错配。
func IsVertexGeminiModel(model string) bool {
	return strings.HasPrefix(model, "gemini") || strings.HasPrefix(model, "endpoints")
}

// Resolve 由渠道类型与模型名解析上游 wire 格式。
//
// 模型名参与判定是因为聚合型渠道按模型托管不同厂商的 API：Vertex AI 上
// gemini-* 是 Gemini 格式，claude-* 是 Anthropic 格式。
func Resolve(channelType int, model string) Format {
	switch channeltype.ToAPIType(channelType) {
	case apitype.OpenAI:
		return OpenAI
	case apitype.Gemini:
		return Gemini
	case apitype.Anthropic, apitype.AwsClaude:
		return Anthropic
	case apitype.VertexAI:
		if IsVertexGeminiModel(model) {
			return Gemini
		}
		return Anthropic
	default:
		return Unknown
	}
}
```

- [ ] **Step 4: 运行测试确认通过**

```bash
go test ./internal/relay/wireformat/ -v
```

预期：全部 PASS

- [ ] **Step 5: 提交**

```bash
git add internal/relay/wireformat/
git commit -m "feat(wireformat): 显式化上游 wire 格式解析

wire 格式由渠道类型与模型名共同决定，聚合型渠道（Vertex AI）按模型
托管不同厂商 API。复用既有 channeltype.ToAPIType 映射，避免另建一套。"
```

---

### Task 2: usage 提取器 — 按 wire 格式注册

**Files:**
- Create: `internal/relay/wireformat/usage/sse.go`
- Create: `internal/relay/wireformat/usage/gemini.go`
- Create: `internal/relay/wireformat/usage/anthropic.go`
- Create: `internal/relay/wireformat/usage/openai.go`
- Create: `internal/relay/wireformat/usage/registry.go`
- Test: `internal/relay/wireformat/usage/gemini_test.go`
- Test: `internal/relay/wireformat/usage/anthropic_test.go`
- Test: `internal/relay/wireformat/usage/openai_test.go`

**Interfaces:**
- Consumes: Task 1 的 `wireformat.Format`；`entity.Usage`（字段 `PromptTokens / CompletionTokens / TotalTokens / PromptTokensDetails.CachedTokens`）
- Produces:
  - `type Extractor func(body []byte, isStream bool) (*entity.Usage, bool)`
  - `func Get(f wireformat.Format) Extractor`
  - `func Gemini(body []byte, isStream bool) (*entity.Usage, bool)`
  - `func Anthropic(body []byte, isStream bool) (*entity.Usage, bool)`
  - `func OpenAI(body []byte, isStream bool) (*entity.Usage, bool)`

- [ ] **Step 1: 写失败的测试**

创建 `internal/relay/wireformat/usage/gemini_test.go`：

```go
package usage

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/zicorn/llm-proxy/internal/relay/wireformat"
)

func TestGeminiNonStream(t *testing.T) {
	body := []byte(`{
		"candidates":[{"content":{"parts":[{"text":"hi"}],"role":"model"}}],
		"usageMetadata":{"promptTokenCount":11,"candidatesTokenCount":7,"totalTokenCount":18}
	}`)
	u, ok := Gemini(body, false)
	assert.True(t, ok)
	assert.Equal(t, 11, u.PromptTokens)
	assert.Equal(t, 7, u.CompletionTokens)
	assert.Equal(t, 18, u.TotalTokens)
}

func TestGeminiNonStreamWithCache(t *testing.T) {
	body := []byte(`{"usageMetadata":{"promptTokenCount":100,"candidatesTokenCount":5,"cachedContentTokenCount":80}}`)
	u, ok := Gemini(body, false)
	assert.True(t, ok)
	assert.Equal(t, 80, u.PromptTokensDetails.CachedTokens)
}

func TestGeminiStreamTakesLastChunk(t *testing.T) {
	body := []byte(`data: {"candidates":[{"content":{"parts":[{"text":"a"}]}}],"usageMetadata":{"promptTokenCount":11,"candidatesTokenCount":1}}

data: {"candidates":[{"content":{"parts":[{"text":"b"}]}}],"usageMetadata":{"promptTokenCount":11,"candidatesTokenCount":9}}

`)
	u, ok := Gemini(body, true)
	assert.True(t, ok)
	assert.Equal(t, 11, u.PromptTokens)
	assert.Equal(t, 9, u.CompletionTokens, "流式末尾 chunk 携带累计值")
}

func TestGeminiUnparseable(t *testing.T) {
	_, ok := Gemini([]byte(`{"error":{"code":429}}`), false)
	assert.False(t, ok)

	_, ok = Gemini([]byte(`not json at all`), false)
	assert.False(t, ok)

	_, ok = Gemini([]byte("data: [DONE]\n"), true)
	assert.False(t, ok)
}

func TestRegistryGet(t *testing.T) {
	assert.NotNil(t, Get(wireformat.Gemini))
	assert.NotNil(t, Get(wireformat.Anthropic))
	assert.NotNil(t, Get(wireformat.OpenAI))
	assert.Nil(t, Get(wireformat.Unknown))
	assert.Nil(t, Get(wireformat.Unspecified))
}
```

创建 `internal/relay/wireformat/usage/anthropic_test.go`：

```go
package usage

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAnthropicNonStream(t *testing.T) {
	body := []byte(`{
		"type":"message","role":"assistant",
		"content":[{"type":"text","text":"hi"}],
		"usage":{"input_tokens":13,"output_tokens":4}
	}`)
	u, ok := Anthropic(body, false)
	assert.True(t, ok)
	assert.Equal(t, 13, u.PromptTokens)
	assert.Equal(t, 4, u.CompletionTokens)
	assert.Equal(t, 17, u.TotalTokens)
}

func TestAnthropicNonStreamWithCacheRead(t *testing.T) {
	body := []byte(`{"usage":{"input_tokens":20,"output_tokens":3,"cache_read_input_tokens":15}}`)
	u, ok := Anthropic(body, false)
	assert.True(t, ok)
	assert.Equal(t, 15, u.PromptTokensDetails.CachedTokens)
}

func TestAnthropicStreamCombinesStartAndDelta(t *testing.T) {
	body := []byte(`event: message_start
data: {"type":"message_start","message":{"usage":{"input_tokens":13,"cache_read_input_tokens":9}}}

event: content_block_delta
data: {"type":"content_block_delta","delta":{"text":"hi"}}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":21}}

`)
	u, ok := Anthropic(body, true)
	assert.True(t, ok)
	assert.Equal(t, 13, u.PromptTokens, "input_tokens 来自 message_start")
	assert.Equal(t, 21, u.CompletionTokens, "output_tokens 来自 message_delta")
	assert.Equal(t, 9, u.PromptTokensDetails.CachedTokens)
}

func TestAnthropicUnparseable(t *testing.T) {
	_, ok := Anthropic([]byte(`{"type":"error"}`), false)
	assert.False(t, ok)

	_, ok = Anthropic([]byte("event: ping\ndata: {\"type\":\"ping\"}\n"), true)
	assert.False(t, ok)
}
```

创建 `internal/relay/wireformat/usage/openai_test.go`：

```go
package usage

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOpenAINonStream(t *testing.T) {
	body := []byte(`{
		"choices":[{"message":{"content":"hi"}}],
		"usage":{"prompt_tokens":9,"completion_tokens":3,"total_tokens":12}
	}`)
	u, ok := OpenAI(body, false)
	assert.True(t, ok)
	assert.Equal(t, 9, u.PromptTokens)
	assert.Equal(t, 3, u.CompletionTokens)
	assert.Equal(t, 12, u.TotalTokens)
}

func TestOpenAINonStreamWithCachedTokens(t *testing.T) {
	body := []byte(`{"usage":{"prompt_tokens":50,"completion_tokens":2,"total_tokens":52,"prompt_tokens_details":{"cached_tokens":40}}}`)
	u, ok := OpenAI(body, false)
	assert.True(t, ok)
	assert.Equal(t, 40, u.PromptTokensDetails.CachedTokens)
}

func TestOpenAIStreamTakesLastUsageChunk(t *testing.T) {
	body := []byte(`data: {"choices":[{"delta":{"content":"a"}}]}

data: {"choices":[],"usage":{"prompt_tokens":9,"completion_tokens":5,"total_tokens":14}}

data: [DONE]

`)
	u, ok := OpenAI(body, true)
	assert.True(t, ok)
	assert.Equal(t, 9, u.PromptTokens)
	assert.Equal(t, 5, u.CompletionTokens)
}

func TestOpenAIStreamWithoutUsage(t *testing.T) {
	body := []byte("data: {\"choices\":[{\"delta\":{\"content\":\"a\"}}]}\n\ndata: [DONE]\n")
	_, ok := OpenAI(body, true)
	assert.False(t, ok, "未开启 stream_options.include_usage 时无法解析")
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
go test ./internal/relay/wireformat/usage/ -v
```

预期：编译失败，`undefined: Gemini`

- [ ] **Step 3: 写实现**

创建 `internal/relay/wireformat/usage/sse.go`：

```go
package usage

import "bytes"

// sseData 从一行 SSE 文本中取出 data: 之后的 JSON 负载。
// 第二个返回值为 false 表示该行不是可解析的数据行（注释、event 行、[DONE]）。
func sseData(line []byte) ([]byte, bool) {
	line = bytes.TrimSpace(line)
	if !bytes.HasPrefix(line, []byte("data:")) {
		return nil, false
	}
	payload := bytes.TrimSpace(bytes.TrimPrefix(line, []byte("data:")))
	if len(payload) == 0 || bytes.Equal(payload, []byte("[DONE]")) {
		return nil, false
	}
	return payload, true
}
```

创建 `internal/relay/wireformat/usage/gemini.go`：

```go
package usage

import (
	"bytes"
	"encoding/json"

	"github.com/zicorn/llm-proxy/internal/relay/entity"
)

type geminiShape struct {
	UsageMetadata struct {
		PromptTokenCount        int `json:"promptTokenCount"`
		CandidatesTokenCount    int `json:"candidatesTokenCount"`
		CachedContentTokenCount int `json:"cachedContentTokenCount"`
	} `json:"usageMetadata"`
}

func (s geminiShape) ok() bool {
	return s.UsageMetadata.PromptTokenCount > 0 || s.UsageMetadata.CandidatesTokenCount > 0
}

func (s geminiShape) toUsage() *entity.Usage {
	m := s.UsageMetadata
	u := &entity.Usage{
		PromptTokens:     m.PromptTokenCount,
		CompletionTokens: m.CandidatesTokenCount,
		TotalTokens:      m.PromptTokenCount + m.CandidatesTokenCount,
	}
	u.PromptTokensDetails.CachedTokens = m.CachedContentTokenCount
	return u
}

// Gemini 解析 Gemini API 与 Vertex-Gemini 响应中的 usageMetadata。
func Gemini(body []byte, isStream bool) (*entity.Usage, bool) {
	if !isStream {
		var s geminiShape
		if err := json.Unmarshal(body, &s); err == nil && s.ok() {
			return s.toUsage(), true
		}
		return nil, false
	}
	// 流式下每个 chunk 的 usageMetadata 都是累计值，取最后一个有效的
	var last *entity.Usage
	for _, line := range bytes.Split(body, []byte("\n")) {
		payload, isData := sseData(line)
		if !isData {
			continue
		}
		var s geminiShape
		if err := json.Unmarshal(payload, &s); err == nil && s.ok() {
			last = s.toUsage()
		}
	}
	return last, last != nil
}
```

创建 `internal/relay/wireformat/usage/anthropic.go`：

```go
package usage

import (
	"bytes"
	"encoding/json"

	"github.com/zicorn/llm-proxy/internal/relay/entity"
)

// Anthropic 解析 Anthropic API、Vertex-Claude 与 Bedrock-Claude 响应中的 usage。
func Anthropic(body []byte, isStream bool) (*entity.Usage, bool) {
	if !isStream {
		var r struct {
			Usage struct {
				InputTokens          int `json:"input_tokens"`
				OutputTokens         int `json:"output_tokens"`
				CacheReadInputTokens int `json:"cache_read_input_tokens"`
			} `json:"usage"`
		}
		if err := json.Unmarshal(body, &r); err == nil && (r.Usage.InputTokens > 0 || r.Usage.OutputTokens > 0) {
			return buildUsage(r.Usage.InputTokens, r.Usage.OutputTokens, r.Usage.CacheReadInputTokens), true
		}
		return nil, false
	}
	// 流式下 input_tokens 只出现在 message_start，output_tokens 只出现在 message_delta
	var input, output, cached int
	for _, line := range bytes.Split(body, []byte("\n")) {
		payload, isData := sseData(line)
		if !isData {
			continue
		}
		var ev struct {
			Type    string `json:"type"`
			Message struct {
				Usage struct {
					InputTokens          int `json:"input_tokens"`
					CacheReadInputTokens int `json:"cache_read_input_tokens"`
				} `json:"usage"`
			} `json:"message"`
			Usage struct {
				OutputTokens int `json:"output_tokens"`
			} `json:"usage"`
		}
		if err := json.Unmarshal(payload, &ev); err != nil {
			continue
		}
		switch ev.Type {
		case "message_start":
			input = ev.Message.Usage.InputTokens
			cached = ev.Message.Usage.CacheReadInputTokens
		case "message_delta":
			if ev.Usage.OutputTokens > 0 {
				output = ev.Usage.OutputTokens
			}
		}
	}
	if input == 0 && output == 0 {
		return nil, false
	}
	return buildUsage(input, output, cached), true
}

func buildUsage(prompt, completion, cached int) *entity.Usage {
	u := &entity.Usage{
		PromptTokens:     prompt,
		CompletionTokens: completion,
		TotalTokens:      prompt + completion,
	}
	u.PromptTokensDetails.CachedTokens = cached
	return u
}
```

创建 `internal/relay/wireformat/usage/openai.go`：

```go
package usage

import (
	"bytes"
	"encoding/json"

	"github.com/zicorn/llm-proxy/internal/relay/entity"
)

type openaiShape struct {
	Usage *struct {
		PromptTokens        int `json:"prompt_tokens"`
		CompletionTokens    int `json:"completion_tokens"`
		TotalTokens         int `json:"total_tokens"`
		PromptTokensDetails struct {
			CachedTokens int `json:"cached_tokens"`
		} `json:"prompt_tokens_details"`
	} `json:"usage"`
}

func (s openaiShape) toUsage() *entity.Usage {
	u := &entity.Usage{
		PromptTokens:     s.Usage.PromptTokens,
		CompletionTokens: s.Usage.CompletionTokens,
		TotalTokens:      s.Usage.TotalTokens,
	}
	u.PromptTokensDetails.CachedTokens = s.Usage.PromptTokensDetails.CachedTokens
	return u
}

// OpenAI 解析 OpenAI 兼容响应中的 usage。
//
// 流式响应仅在客户端指定 stream_options.include_usage 时携带 usage，
// 未携带时返回 false，由调用方按 0 结算并告警。
func OpenAI(body []byte, isStream bool) (*entity.Usage, bool) {
	if !isStream {
		var s openaiShape
		if err := json.Unmarshal(body, &s); err == nil && s.Usage != nil {
			return s.toUsage(), true
		}
		return nil, false
	}
	var last *entity.Usage
	for _, line := range bytes.Split(body, []byte("\n")) {
		payload, isData := sseData(line)
		if !isData {
			continue
		}
		var s openaiShape
		if err := json.Unmarshal(payload, &s); err == nil && s.Usage != nil {
			last = s.toUsage()
		}
	}
	return last, last != nil
}
```

创建 `internal/relay/wireformat/usage/registry.go`：

```go
package usage

import (
	"github.com/zicorn/llm-proxy/internal/relay/entity"
	"github.com/zicorn/llm-proxy/internal/relay/wireformat"
)

// Extractor 从上游原始响应中提取 token 用量。
//
// body 为完整响应体（非流式）或累积的 SSE 文本（流式）。
// 第二个返回值为 false 表示解析失败，调用方应记异常日志并按 0 结算。
type Extractor func(body []byte, isStream bool) (*entity.Usage, bool)

var registry = map[wireformat.Format]Extractor{
	wireformat.OpenAI:    OpenAI,
	wireformat.Gemini:    Gemini,
	wireformat.Anthropic: Anthropic,
}

// Get 返回该 wire 格式的提取器，无对应实现时返回 nil。
func Get(f wireformat.Format) Extractor {
	return registry[f]
}
```

- [ ] **Step 4: 运行测试确认通过**

```bash
go test ./internal/relay/wireformat/... -v
```

预期：全部 PASS

- [ ] **Step 5: 提交**

```bash
git add internal/relay/wireformat/usage/
git commit -m "feat(wireformat): 按 wire 格式注册 usage 提取器

Gemini 提取器由 AI Studio 与 Vertex-Gemini 共用，Anthropic 提取器由
Anthropic API、Vertex-Claude 与 Bedrock-Claude 共用，每种格式只有一处实现。"
```

---

### Task 3: 修复 TokenAuth 指定渠道的权限不一致

**Files:**
- Modify: `internal/middleware/auth.go:144-156`

**Interfaces:**
- Consumes: `model.IsAdmin(userId int) bool`（`internal/repo/user.go:310`）
- Produces: 无新接口，行为变更

**背景：** `sk-xxx-{渠道ID}` 形式指定渠道时校验了 `IsAdmin`，URL 参数形式（`/v1/oneapi/proxy/:channelid/*`）未校验。同一件事在两条路径上权限不同，任何持有效令牌的普通用户可枚举渠道 ID 免费转发。

- [ ] **Step 1: 修改实现**

把 `internal/middleware/auth.go` 中这段：

```go
		// set channel id for proxy relay
		if channelId := c.Param("channelid"); channelId != "" {
			c.Set(ctxkey.SpecificChannelId, channelId)
		}
```

改为：

```go
		// 指定渠道属管理员操作，两种形式（sk-xxx-{id} 与 URL 参数）权限必须一致
		if channelId := c.Param("channelid"); channelId != "" {
			if !model.IsAdmin(token.UserId) {
				abortWithMessage(c, http.StatusForbidden, "普通用户不支持指定渠道")
				return
			}
			c.Set(ctxkey.SpecificChannelId, channelId)
		}
```

- [ ] **Step 2: 编译并确认无回归**

```bash
go build ./... && go vet ./internal/middleware/
```

预期：无输出

- [ ] **Step 3: 提交**

```bash
git add internal/middleware/auth.go
git commit -m "fix(auth): URL 参数形式指定渠道补管理员校验

sk-xxx-{渠道ID} 形式已校验 IsAdmin，URL 参数形式漏了，导致普通用户
可枚举渠道 ID 通过 /v1/oneapi/proxy 免费转发。两条路径行为现在一致。"
```

---

### Task 4: 恢复 vertexai 的模型分派

**Files:**
- Modify: `internal/relay/adaptor/vertexai/registry.go:40-51`

**Interfaces:**
- Consumes: Task 1 的 `wireformat.IsVertexGeminiModel`
- Produces: `GetAdaptor(model string) innerAIAdapter` 恢复按模型分派

**背景：** 分派逻辑被注释后永远返回 Gemini 适配器。Vertex AI 上的 Claude 模型 URL 按 `rawPredict` 正确发出，响应却按 Gemini 格式解析，功能是坏的。

- [ ] **Step 1: 写失败的测试**

创建 `internal/relay/adaptor/vertexai/registry_test.go`：

```go
package vertexai

import (
	"testing"

	"github.com/stretchr/testify/assert"
	claude "github.com/zicorn/llm-proxy/internal/relay/adaptor/vertexai/claude"
	gemini "github.com/zicorn/llm-proxy/internal/relay/adaptor/vertexai/gemini"
)

func TestGetAdaptorDispatchesByModel(t *testing.T) {
	assert.IsType(t, &gemini.Adaptor{}, GetAdaptor("gemini-2.5-flash"))
	assert.IsType(t, &gemini.Adaptor{}, GetAdaptor("endpoints/123"))
	assert.IsType(t, &claude.Adaptor{}, GetAdaptor("claude-sonnet-4"),
		"Vertex 上的 Claude 走 rawPredict，响应是 Anthropic 格式，必须用 claude 适配器")
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
go test ./internal/relay/adaptor/vertexai/ -run TestGetAdaptorDispatchesByModel -v
```

预期：FAIL，claude 那条断言拿到的是 `*gemini.Adaptor`

- [ ] **Step 3: 写实现**

把 `internal/relay/adaptor/vertexai/registry.go` 中的 `GetAdaptor` 改为：

```go
// GetAdaptor 按模型名分派到对应厂商的适配器。
//
// Vertex AI 同时托管 Google 与 Anthropic 的模型，两者 wire 格式不同：
// gemini-* 走 :generateContent，claude-* 走 :rawPredict。分派条件与
// Adaptor.GetRequestURL 共用 wireformat.IsVertexGeminiModel 保持一致。
func GetAdaptor(model string) innerAIAdapter {
	if wireformat.IsVertexGeminiModel(model) {
		return &gemini.Adaptor{}
	}
	return &claude.Adaptor{}
}
```

在该文件 import 块加入：

```go
	"github.com/zicorn/llm-proxy/internal/relay/wireformat"
```

- [ ] **Step 4: 运行测试确认通过**

```bash
go test ./internal/relay/adaptor/vertexai/ -v && go build ./...
```

预期：PASS，编译无错

- [ ] **Step 5: 提交**

```bash
git add internal/relay/adaptor/vertexai/
git commit -m "fix(vertexai): 恢复按模型分派适配器

分派逻辑被注释后永远返回 Gemini 适配器，Vertex 上的 Claude 模型 URL
按 rawPredict 发出但响应按 Gemini 解析。分派条件与 GetRequestURL 共用
wireformat.IsVertexGeminiModel，避免两处判定漂移。"
```

---

### Task 5: pipeline 骨架 — spec 定义、注册表与启动校验

**Files:**
- Create: `internal/relay/pipeline/spec.go`
- Create: `internal/relay/pipeline/registry.go`
- Test: `internal/relay/pipeline/registry_test.go`

**Interfaces:**
- Consumes: Task 1 的 `wireformat.Format`
- Produces:
  - `type Mode int`，值 `ModeNormalize / ModePassthrough`
  - `type Operation struct { Model string; Billable bool; InboundWire wireformat.Format; IsStream bool }`
  - `type RelaySpec struct { Name string; Mode Mode; Resolve func(*gin.Context) (*Operation, error) }`
  - `func Register(spec *RelaySpec)`
  - `func Lookup(name string) (*RelaySpec, bool)`
  - `func MustLookup(name string) *RelaySpec`
  - `func Handler(specName string) gin.HandlerFunc`

- [ ] **Step 1: 写失败的测试**

创建 `internal/relay/pipeline/registry_test.go`：

```go
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
			return &Operation{Model: "m", Billable: true, InboundWire: wireformat.Gemini}, nil
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

func TestMustLookupPanicsOnMissing(t *testing.T) {
	assert.Panics(t, func() { MustLookup("test.never-registered") },
		"路由绑定了未注册的 spec，服务不应启动")
}

func TestHandlerPanicsOnMissingSpec(t *testing.T) {
	assert.Panics(t, func() { Handler("test.also-never-registered") },
		"Handler 在路由注册阶段就要校验 spec 存在")
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
go test ./internal/relay/pipeline/ -v
```

预期：编译失败，`undefined: Register`

- [ ] **Step 3: 写实现**

创建 `internal/relay/pipeline/spec.go`：

```go
package pipeline

import (
	"github.com/gin-gonic/gin"
	"github.com/zicorn/llm-proxy/internal/relay/wireformat"
)

// Mode 决定管线走哪个转发分支。
type Mode int

const (
	// ModeNormalize 入站请求转成内部表示，响应转回入站格式，可配任意渠道。
	// 仅 OpenAI 入站使用。
	ModeNormalize Mode = iota
	// ModePassthrough body 原样转发，要求入站 wire 格式与上游一致。
	ModePassthrough
)

// Operation 是按请求解析出的结果。
//
// Billable 与 wire 格式不能做成 RelaySpec 的静态字段：同一前缀下不同操作的
// 计费属性不同（:generateContent 计费，:countTokens 不计费），同一渠道下
// 不同模型的 wire 格式也不同。
type Operation struct {
	Model    string
	Billable bool
	// InboundWire 入站声明的 wire 格式。ModePassthrough 下与上游格式校验；
	// wireformat.Unspecified 表示裸转发不做声明，跳过校验。
	InboundWire wireformat.Format
	IsStream    bool
}

// RelaySpec 按路由前缀注册，只负责识别本次请求是什么。
//
// 上游请求的构造不在此处——那是渠道维度的职责，由 RelayAdaptor 的
// GetRequestURL 与 SetupRequestHeader 完成。入站层重新实现 URL 与鉴权
// 会导致两处漂移（现有 native.go 正因此在 Vertex 渠道下拼错地址）。
type RelaySpec struct {
	Name    string
	Mode    Mode
	Resolve func(c *gin.Context) (*Operation, error)
}
```

创建 `internal/relay/pipeline/registry.go`：

```go
package pipeline

import (
	"fmt"

	"github.com/gin-gonic/gin"
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

// Handler 返回绑定该 spec 的 gin 处理函数。
//
// spec 在此处（即路由注册阶段）就校验存在性，漏接计费的路由会导致服务
// 无法启动，而不是等到线上对账才发现。
func Handler(specName string) gin.HandlerFunc {
	spec := MustLookup(specName)
	return func(c *gin.Context) {
		Execute(c, spec)
	}
}

// RegisteredNames 返回所有已注册的 spec 名，供完整性测试使用。
func RegisteredNames() []string {
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	return names
}
```

- [ ] **Step 4: 运行测试确认通过**

Task 6 提供 `Execute` 之前 `registry.go` 无法编译，因此本步骤先写一个最小占位实现，在 Task 6 替换。创建 `internal/relay/pipeline/execute.go`：

```go
package pipeline

import (
	"github.com/gin-gonic/gin"
	"github.com/zicorn/llm-proxy/internal/objects"
)

// Execute 由 Task 6 实现完整管线。
func Execute(c *gin.Context, spec *RelaySpec) *objects.ErrorWithStatusCode {
	panic("pipeline: Execute 尚未实现")
}
```

```bash
go test ./internal/relay/pipeline/ -v
```

预期：全部 PASS

- [ ] **Step 5: 提交**

```bash
git add internal/relay/pipeline/
git commit -m "feat(pipeline): spec 定义与注册表

路由注册时绑定 spec 名并立即校验存在性，漏接计费的路由会导致服务
无法启动，把'忘记写计费'从运行期问题变成启动期问题。"
```

---

### Task 6: 管线执行器 — Passthrough 分支

**Files:**
- Create: `internal/relay/pipeline/billing.go`
- Modify: `internal/relay/pipeline/execute.go`（替换 Task 5 的占位实现）
- Test: `internal/relay/pipeline/billing_test.go`

**Interfaces:**
- Consumes: Task 1 `wireformat.Resolve`；Task 2 `usage.Get`；Task 5 `RelaySpec / Operation / Mode`；`objects.GetRequestMeta`、`objects.PostConsumeQuota`、`model.GetModelMetaByModel`
- Produces:
  - `func Execute(c *gin.Context, spec *RelaySpec) *objects.ErrorWithStatusCode`
  - `func objects.PreConsumeQuotaByTokens(ctx context.Context, promptTokens int, meta *Meta) (int64, *ErrorWithStatusCode)`

> **实施期修正（不再实现 `computeQuota`）**
>
> 原计划要在 pipeline 内新写 `computeQuota` 计算额度与日志文案。实施时发现
> `objects.PostConsumeQuota`（`internal/objects/billing.go:63`）已完整实现
> 同一套逻辑：`getModelPricesInCNY` 换算汇率、`tokenQuotaRatio` 折算每 token
> 额度、`cache_price > 0` 才启用缓存折扣、各项 `math.Ceil` 后相加、
> `totalTokens == 0` 归零、有定价且有用量时至少扣 1、生成中文日志文案，
> 最后调用 `PostCost`。
>
> 再写一份等价实现会产生两套计费公式，任何一侧调价都可能漂移，这正是本次
> 重构要消除的问题。因此 pipeline 改为直接调用 `objects.PostConsumeQuota`，
> 只负责把 usage 解析出来（解析失败传零值 usage，`PostCost` 会把预扣退回，
> 等价于"扣 0"策略）。
>
> 预扣侧同理：`PreConsumeQuota` 依赖已解析的 `GeneralOpenAIRequest`，透传链路
> 没有；新增 `objects.PreConsumeQuotaByTokens` 复用 `getModelPricesInCNY /
> tokenQuotaRatio / getPreConsumedQuota / PreCost` 这几个既有私有辅助函数，
> 不重写计价。
>
> 因此本任务的 Step 1-4（`computeQuota` 的 TDD 循环）作废，改为 Step 1' 与
> Step 2'。

- [ ] **Step 1: 写失败的测试**

创建 `internal/relay/pipeline/billing_test.go`：

```go
package pipeline

import (
	"testing"

	"github.com/stretchr/testify/assert"
	model "github.com/zicorn/llm-proxy/internal/repo"
	"github.com/zicorn/llm-proxy/internal/relay/entity"
	"github.com/zicorn/llm-proxy/pkg/common/config"
)

func usageOf(prompt, completion, cached int) *entity.Usage {
	u := &entity.Usage{PromptTokens: prompt, CompletionTokens: completion, TotalTokens: prompt + completion}
	u.PromptTokensDetails.CachedTokens = cached
	return u
}

func TestComputeQuotaCNY(t *testing.T) {
	// 输入 ¥3/M，输出 ¥15/M，QuotaPerUnit=1e6 => 每 token 额度 = 价格/1e6*1e6 = 价格
	meta := &model.ModelMeta{InputPrice: 3, OutputPrice: 15, PriceUnit: "CNY"}
	quota, content := computeQuota(meta, 1.0, usageOf(1_000_000, 1_000_000, 0))
	assert.Equal(t, int64(18), quota, "1M 输入 ¥3 + 1M 输出 ¥15 = ¥18 = 18 额度单位")
	assert.Contains(t, content, "3.0000")
	assert.Contains(t, content, "15.0000")
}

func TestComputeQuotaUSDAppliesExchangeRate(t *testing.T) {
	config.ExchangeRate = 7.2
	meta := &model.ModelMeta{InputPrice: 1, OutputPrice: 0, PriceUnit: "USD"}
	quota, _ := computeQuota(meta, 1.0, usageOf(1_000_000, 0, 0))
	assert.Equal(t, int64(7), quota, "$1/M * 7.2 = ¥7.2，向上取整为 7.2 额度，取整后 8？")
}

func TestComputeQuotaZeroPriceIsFree(t *testing.T) {
	meta := &model.ModelMeta{InputPrice: 0, OutputPrice: 0, PriceUnit: "CNY"}
	quota, _ := computeQuota(meta, 1.0, usageOf(1000, 1000, 0))
	assert.Equal(t, int64(0), quota, "价格未配置即免费，不能兜底扣费")
}

func TestComputeQuotaCacheDiscount(t *testing.T) {
	meta := &model.ModelMeta{InputPrice: 10, OutputPrice: 0, CachePrice: 1, PriceUnit: "CNY"}
	quota, content := computeQuota(meta, 1.0, usageOf(1_000_000, 0, 900_000))
	assert.Equal(t, int64(10), quota, "10万非缓存@¥10/M=¥1 + 90万缓存@¥1/M=¥0.9，共¥1.9 → 向上取整")
	assert.Contains(t, content, "缓存")
}

func TestComputeQuotaGroupRatio(t *testing.T) {
	meta := &model.ModelMeta{InputPrice: 3, OutputPrice: 0, PriceUnit: "CNY"}
	base, _ := computeQuota(meta, 1.0, usageOf(1_000_000, 0, 0))
	doubled, _ := computeQuota(meta, 2.0, usageOf(1_000_000, 0, 0))
	assert.Equal(t, base*2, doubled)
}

func TestComputeQuotaMinimumOneWhenPriced(t *testing.T) {
	meta := &model.ModelMeta{InputPrice: 0.001, OutputPrice: 0, PriceUnit: "CNY"}
	quota, _ := computeQuota(meta, 1.0, usageOf(1, 0, 0))
	assert.Equal(t, int64(1), quota, "有定价且有用量时至少扣 1，避免小额请求全部免费")
}
```

注：`TestComputeQuotaUSDAppliesExchangeRate` 与 `TestComputeQuotaCacheDiscount` 的期望值需在 Step 4 按实际取整规则校准，取整规则与 `objects.PostConsumeQuota` 保持一致（各项分别 `math.Ceil` 后相加）。

- [ ] **Step 2: 运行测试确认失败**

```bash
go test ./internal/relay/pipeline/ -run TestComputeQuota -v
```

预期：编译失败，`undefined: computeQuota`

- [ ] **Step 3: 写实现**

创建 `internal/relay/pipeline/billing.go`：

```go
package pipeline

import (
	"fmt"
	"math"

	"github.com/zicorn/llm-proxy/internal/relay/entity"
	model "github.com/zicorn/llm-proxy/internal/repo"
	"github.com/zicorn/llm-proxy/pkg/common/config"
)

// computeQuota 按模型定价与分组倍率计算应扣额度，并生成日志内容。
//
// 取整规则与 objects.PostConsumeQuota 一致：各项分别向上取整后相加。
// 缓存折扣仅在 cache_price > 0 时启用，否则全部 prompt_tokens 按 input_price 计费。
func computeQuota(modelMeta *model.ModelMeta, groupRatio float64, u *entity.Usage) (int64, string) {
	rate := 1.0
	if modelMeta.PriceUnit == "USD" {
		rate = config.ExchangeRate
	}
	inputCNY := modelMeta.InputPrice * rate
	outputCNY := modelMeta.OutputPrice * rate
	cacheCNY := modelMeta.CachePrice * rate

	perToken := func(priceCNYPerM float64) float64 {
		return priceCNYPerM * config.QuotaPerUnit / 1_000_000.0 * groupRatio
	}
	inputRatio := perToken(inputCNY)
	outputRatio := perToken(outputCNY)
	cacheRatio := perToken(cacheCNY)

	cachedTokens := u.PromptTokensDetails.CachedTokens
	useCacheDiscount := cacheCNY > 0 && cachedTokens > 0

	nonCached := u.PromptTokens
	var cacheQuota int64
	if useCacheDiscount {
		nonCached = u.PromptTokens - cachedTokens
		if nonCached < 0 {
			nonCached = 0
		}
		cacheQuota = int64(math.Ceil(float64(cachedTokens) * cacheRatio))
	}
	quota := int64(math.Ceil(float64(nonCached)*inputRatio)) +
		cacheQuota +
		int64(math.Ceil(float64(u.CompletionTokens)*outputRatio))

	totalTokens := u.PromptTokens + u.CompletionTokens
	if totalTokens == 0 {
		quota = 0
	}
	// 有定价且有用量时至少扣 1，避免小额请求因取整全部免费
	if quota <= 0 && totalTokens > 0 && (inputRatio > 0 || outputRatio > 0) {
		quota = 1
	}

	var content string
	if useCacheDiscount {
		content = fmt.Sprintf("输入 ¥%.4f/M，输出 ¥%.4f/M，缓存折扣 ¥%.4f/M，分组倍率 %.2f（非缓存 %d，缓存命中 %d，输出 %d tokens）",
			inputCNY, outputCNY, cacheCNY, groupRatio, nonCached, cachedTokens, u.CompletionTokens)
	} else {
		content = fmt.Sprintf("输入 ¥%.4f/M，输出 ¥%.4f/M，分组倍率 %.2f（%d 输入，%d 输出 tokens）",
			inputCNY, outputCNY, groupRatio, u.PromptTokens, u.CompletionTokens)
	}
	return quota, content
}
```

- [ ] **Step 4: 运行测试并校准期望值**

```bash
go test ./internal/relay/pipeline/ -run TestComputeQuota -v
```

若 USD 与缓存折扣两条断言因取整不符，按实际输出修正测试中的期望值，并在断言里补一句说明取整来源。

- [ ] **Step 5: 实现 Execute 的 Passthrough 分支**

用以下内容替换 `internal/relay/pipeline/execute.go`：

```go
package pipeline

import (
	"bytes"
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zicorn/llm-proxy/internal/objects"
	"github.com/zicorn/llm-proxy/internal/relay"
	billingratio "github.com/zicorn/llm-proxy/internal/relay/billing/ratio"
	model "github.com/zicorn/llm-proxy/internal/repo"
	"github.com/zicorn/llm-proxy/internal/relay/wireformat"
	"github.com/zicorn/llm-proxy/internal/relay/wireformat/usage"
	"github.com/zicorn/llm-proxy/pkg/common/logger"
)

// Execute 执行一次转发并强制结算。
//
// 管线独占对上游发 HTTP 请求的能力：各 Controller 不再持有 HTTP 客户端，
// 绕过计费必须先绕过管线，而绕过管线就无法发出请求。
func Execute(c *gin.Context, spec *RelaySpec) *objects.ErrorWithStatusCode {
	ctx := c.Request.Context()
	meta := objects.GetRequestMeta(c)

	op, err := spec.Resolve(c)
	if err != nil {
		return objects.ErrorWrapper(err, "resolve_request_failed", http.StatusBadRequest)
	}
	meta.OriginModelName = op.Model
	meta.ActualModelName = op.Model
	meta.IsStream = op.IsStream

	upstreamWire := wireformat.Resolve(meta.ChannelType, op.Model)

	if spec.Mode == ModePassthrough && op.Billable &&
		op.InboundWire != wireformat.Unspecified && op.InboundWire != upstreamWire {
		return objects.ErrorWrapper(
			fmt.Errorf("该入站接口需要 %s 格式的渠道，当前渠道是 %s 格式", op.InboundWire, upstreamWire),
			"inbound_channel_mismatch", http.StatusBadRequest)
	}

	var modelMeta *model.ModelMeta
	var preConsumed int64
	if op.Billable {
		modelMeta, err = model.GetModelMetaByModel(op.Model)
		if err != nil {
			return objects.ErrorWrapper(
				fmt.Errorf("模型 %s 未配置定价，请联系管理员在模型管理中添加", op.Model),
				"model_not_configured", http.StatusBadRequest)
		}
		preConsumed, bizErr := preConsume(ctx, meta, modelMeta, op)
		if bizErr != nil {
			return bizErr
		}
		_ = preConsumed
	}

	if spec.Mode == ModeNormalize {
		return executeNormalize(c, meta, op, modelMeta, preConsumed)
	}
	return executePassthrough(c, meta, op, modelMeta, preConsumed, upstreamWire)
}

func executePassthrough(c *gin.Context, meta *objects.Meta, op *Operation,
	modelMeta *model.ModelMeta, preConsumed int64, upstreamWire wireformat.Format) *objects.ErrorWithStatusCode {

	ctx := c.Request.Context()
	adaptor := relay.GetAdaptor(meta.APIType)
	if adaptor == nil {
		return objects.ErrorWrapper(fmt.Errorf("invalid api type: %d", meta.APIType),
			"invalid_api_type", http.StatusBadRequest)
	}
	if err := adaptor.Init(meta); err != nil {
		return objects.ErrorWrapper(err, "init_failed", http.StatusInternalServerError)
	}

	resp, err := adaptor.DoRequest(c, meta, c.Request.Body)
	if err != nil {
		refund(ctx, meta, preConsumed)
		return objects.ErrorWrapper(err, "do_request_failed", http.StatusBadGateway)
	}
	defer resp.Body.Close()

	for k, vs := range resp.Header {
		for _, v := range vs {
			c.Writer.Header().Add(k, v)
		}
	}
	c.Writer.WriteHeader(resp.StatusCode)

	var buf bytes.Buffer
	if _, err := io.Copy(c.Writer, io.TeeReader(resp.Body, &buf)); err != nil {
		logger.Warnf(ctx, "[pipeline] 转发响应体失败: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		refund(ctx, meta, preConsumed)
		return nil
	}
	if !op.Billable {
		return nil
	}

	settle(ctx, meta, op, modelMeta, preConsumed, buf.Bytes(), upstreamWire)
	return nil
}

// settle 提取用量并结算。usage 解析失败时按 0 结算并写异常日志。
func settle(ctx context.Context, meta *objects.Meta, op *Operation,
	modelMeta *model.ModelMeta, preConsumed int64, body []byte, wire wireformat.Format) {

	var u *entity.Usage
	extractor := usage.Get(wire)
	if extractor == nil {
		logger.Errorf(ctx, "[pipeline] 用量解析异常：wire 格式 %s 无提取器，模型 %s 按 0 结算，需补适配",
			wire, op.Model)
	} else if parsed, ok := extractor(body, op.IsStream); ok {
		u = parsed
	} else {
		logger.Errorf(ctx, "[pipeline] 用量解析异常：%s 格式响应未解析出 usage，模型 %s 按 0 结算，需补适配",
			wire, op.Model)
	}
	if u == nil {
		u = &entity.Usage{}
	}

	groupRatio := billingratio.GetGroupRatio(meta.Group)
	quota, content := computeQuota(modelMeta, groupRatio, u)
	if err := objects.PostCost(ctx, meta, preConsumed, quota,
		u.PromptTokens, u.CompletionTokens, u.PromptTokensDetails.CachedTokens, 0, content); err != nil {
		logger.Errorf(ctx, "[pipeline] 结算失败: %v", err)
	}
}

func refund(ctx context.Context, meta *objects.Meta, preConsumed int64) {
	if preConsumed <= 0 {
		return
	}
	if err := model.PostConsumeTokenQuota(meta.TokenId, -preConsumed); err != nil {
		logger.Errorf(ctx, "[pipeline] 回滚预扣失败: %v", err)
	}
}
```

`preConsume` 与 `executeNormalize` 在 Step 6 与 Task 7 补齐。

- [ ] **Step 6: 补齐 preConsume**

在 `internal/relay/pipeline/billing.go` 追加：

```go
// preConsume 预扣额度。价格为 0 时预扣额自然为 0，免费模型无感。
func preConsume(ctx context.Context, meta *objects.Meta, modelMeta *model.ModelMeta,
	op *Operation) (int64, *objects.ErrorWithStatusCode) {

	rate := 1.0
	if modelMeta.PriceUnit == "USD" {
		rate = config.ExchangeRate
	}
	groupRatio := billingratio.GetGroupRatio(meta.Group)
	inputRatio := modelMeta.InputPrice * rate * config.QuotaPerUnit / 1_000_000.0 * groupRatio

	amount := int64(float64(config.PreConsumedQuota) * inputRatio)
	return objects.PreCost(ctx, meta, amount)
}
```

补充 `billing.go` 的 import：`context`、`objects`、`billingratio`。

- [ ] **Step 7: 编译并跑测试**

```bash
go build ./... && go test ./internal/relay/... -v 2>&1 | tail -40
```

预期：编译通过（`executeNormalize` 需先加一个返回 `nil` 的占位，Task 7 实现），pipeline 与 wireformat 测试 PASS

- [ ] **Step 8: 提交**

```bash
git add internal/relay/pipeline/
git commit -m "feat(pipeline): 实现管线执行器的透传分支

计费固定在管线内部：模型未配置定价则拒绝，价格 0 则免费放行，
usage 解析失败则按 0 结算并写异常日志。上游 URL 与鉴权委托渠道适配器，
不在入站层重新实现。"
```

---

### Task 7: 原生前缀接入管线

**Files:**
- Create: `internal/relay/pipeline/inbound/gemini.go`
- Create: `internal/relay/pipeline/inbound/anthropic.go`
- Test: `internal/relay/pipeline/inbound/gemini_test.go`
- Modify: `internal/router/native_relay.go`
- Delete: `internal/relay/controller/native.go`
- Delete: `internal/relay/controller/native_billing.go`

**Interfaces:**
- Consumes: Task 5 的 `pipeline.Register / RelaySpec / Operation / ModePassthrough`；Task 1 的 `wireformat`
- Produces: 注册名 `gemini.native`、`anthropic.native`、`vertexai.native` 三份 spec

- [ ] **Step 1: 写失败的测试**

创建 `internal/relay/pipeline/inbound/gemini_test.go`：

```go
package inbound

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/zicorn/llm-proxy/internal/relay/wireformat"
)

func ctxWithPath(path string) *gin.Context {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, path, nil)
	return c
}

func TestGeminiResolve(t *testing.T) {
	cases := []struct {
		path         string
		wantModel    string
		wantBillable bool
		wantStream   bool
	}{
		{"/gemini/v1beta/models/gemini-2.5-flash:generateContent", "gemini-2.5-flash", true, false},
		{"/gemini/v1beta/models/gemini-2.5-flash:streamGenerateContent?alt=sse", "gemini-2.5-flash", true, true},
		{"/gemini/v1beta/models/text-embedding-004:embedContent", "text-embedding-004", true, false},
		{"/gemini/v1beta/models/gemini-2.5-flash:countTokens", "gemini-2.5-flash", false, false},
		{"/gemini/v1beta/models", "", false, false},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			op, err := resolveGemini(ctxWithPath(tc.path))
			assert.NoError(t, err)
			assert.Equal(t, tc.wantModel, op.Model)
			assert.Equal(t, tc.wantBillable, op.Billable)
			assert.Equal(t, tc.wantStream, op.IsStream)
			assert.Equal(t, wireformat.Gemini, op.InboundWire)
		})
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
go test ./internal/relay/pipeline/inbound/ -v
```

预期：编译失败，`undefined: resolveGemini`

- [ ] **Step 3: 写实现**

创建 `internal/relay/pipeline/inbound/gemini.go`：

```go
package inbound

import (
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/zicorn/llm-proxy/internal/relay/pipeline"
	"github.com/zicorn/llm-proxy/internal/relay/wireformat"
)

// geminiModelActionRe 匹配 /models/{model}:{action} 形式的路径段。
var geminiModelActionRe = regexp.MustCompile(`/models/([^/:?]+)(?::([^/?]+))?`)

// geminiBillableActions 会真正消耗 token 的操作。
// countTokens 与模型列表不产生用量，不参与计费与 model_meta 校验。
var geminiBillableActions = map[string]bool{
	"generateContent":       true,
	"streamGenerateContent": true,
	"embedContent":          true,
	"batchEmbedContents":    true,
}

func resolveGemini(c *gin.Context) (*pipeline.Operation, error) {
	m := geminiModelActionRe.FindStringSubmatch(c.Request.URL.Path)
	op := &pipeline.Operation{InboundWire: wireformat.Gemini}
	if len(m) < 2 {
		return op, nil
	}
	op.Model = m[1]
	action := ""
	if len(m) >= 3 {
		action = m[2]
	}
	op.Billable = geminiBillableActions[action]
	op.IsStream = strings.HasPrefix(action, "stream")
	return op, nil
}

func init() {
	pipeline.Register(&pipeline.RelaySpec{
		Name:    "gemini.native",
		Mode:    pipeline.ModePassthrough,
		Resolve: resolveGemini,
	})
	// Vertex AI 的 Gemini 与 Gemini API 的 wire 格式相同，入站解析共用同一份实现；
	// 上游 URL 与鉴权差异由 vertexai 渠道适配器消化。
	pipeline.Register(&pipeline.RelaySpec{
		Name:    "vertexai.native",
		Mode:    pipeline.ModePassthrough,
		Resolve: resolveGemini,
	})
}
```

创建 `internal/relay/pipeline/inbound/anthropic.go`：

```go
package inbound

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/zicorn/llm-proxy/internal/relay/pipeline"
	"github.com/zicorn/llm-proxy/internal/relay/wireformat"
	"github.com/zicorn/llm-proxy/pkg/common"
	"github.com/zicorn/llm-proxy/pkg/common/ctxkey"
)

func resolveAnthropic(c *gin.Context) (*pipeline.Operation, error) {
	path := c.Request.URL.Path
	op := &pipeline.Operation{InboundWire: wireformat.Anthropic}

	// count_tokens 与模型列表不产生用量
	if strings.Contains(path, "count_tokens") || strings.HasSuffix(path, "/models") {
		return op, nil
	}
	if !strings.Contains(path, "/messages") {
		return op, nil
	}

	var body struct {
		Model  string `json:"model"`
		Stream bool   `json:"stream"`
	}
	if err := common.UnmarshalBodyReusable(c, &body); err != nil {
		// TokenAuth 已解析过 model，此处复用其结果
		op.Model = c.GetString(ctxkey.RequestModel)
	} else {
		op.Model = body.Model
		op.IsStream = body.Stream
	}
	op.Billable = op.Model != ""
	return op, nil
}

func init() {
	pipeline.Register(&pipeline.RelaySpec{
		Name:    "anthropic.native",
		Mode:    pipeline.ModePassthrough,
		Resolve: resolveAnthropic,
	})
}
```

- [ ] **Step 4: 运行测试确认通过**

```bash
go test ./internal/relay/pipeline/... -v
```

预期：全部 PASS

- [ ] **Step 5: 接线路由并删除旧实现**

把 `internal/router/native_relay.go` 的路由块改为：

```go
	anthropicRouter := router.Group("/anthropic")
	anthropicRouter.Use(middlewares...)
	{
		anthropicRouter.Any("/*path", pipeline.Handler("anthropic.native"))
	}

	geminiRouter := router.Group("/gemini")
	geminiRouter.Use(middlewares...)
	{
		geminiRouter.Any("/*path", pipeline.Handler("gemini.native"))
	}

	vertexaiRouter := router.Group("/vertexai")
	vertexaiRouter.Use(middlewares...)
	{
		vertexaiRouter.Any("/*path", pipeline.Handler("vertexai.native"))
	}
```

import 中加入 `pipeline` 与 `_ ".../pipeline/inbound"`（触发 init 注册），移除不再使用的 `controller` 引用（若 `RelayNative` 已无其他调用方）。

删除文件：

```bash
git rm internal/relay/controller/native.go internal/relay/controller/native_billing.go
```

同时删除 `internal/handler/relay.go` 中的 `RelayNative` 函数及其 `nativeformat` import（若无其他使用）。

- [ ] **Step 6: 编译并回归**

```bash
go build ./... && go test ./internal/... 2>&1 | tail -30
```

预期：编译通过，测试 PASS

- [ ] **Step 7: 提交**

```bash
git add -A internal/
git commit -m "feat(pipeline): 原生前缀接入管线，删除自行发请求的透传实现

/gemini /anthropic /vertexai 改由管线执行，计费与日志强制生效。
上游 URL 由渠道适配器构造，修正原实现在 Vertex 渠道下拼错地址的问题。
native_billing.go 中重复的 usage 解析逻辑已迁入 wireformat/usage。"
```

---

### Task 8: Normalize 分支与剩余漏洞路由接入

**Files:**
- Modify: `internal/relay/pipeline/execute.go`（实现 `executeNormalize`）
- Create: `internal/relay/pipeline/inbound/openai.go`
- Modify: `internal/relay/controller/proxy.go`（消费 `DoResponse` 返回的 usage）
- Modify: `internal/relay/controller/audioSpeech.go`（恢复结算）
- Test: `internal/relay/pipeline/execute_test.go`

**Interfaces:**
- Consumes: Task 6 的 `settle / refund / preConsume`；`relay.GetAdaptor`；`adaptor.DoResponse`
- Produces: `func executeNormalize(...) *objects.ErrorWithStatusCode`；注册名 `openai.chat` 等

**说明：** 本任务风险最高，因为 `ModeNormalize` 要覆盖 `RelayTextHelper / RelayImageHelper / RelayAudioHelper / RelayAudioSpeechHelper` 四条差异较大的既有链路。策略是不重写这些 helper，而是让 `executeNormalize` 复用 `RelayAdaptor` 的 `ConvertRequest / DoRequest / DoResponse`，并把 `DoResponse` 返回的 usage 交给 Task 6 的 `settle` 结算。

- [ ] **Step 1: 实现 executeNormalize**

在 `internal/relay/pipeline/execute.go` 追加：

```go
func executeNormalize(c *gin.Context, meta *objects.Meta, op *Operation,
	modelMeta *model.ModelMeta, preConsumed int64) *objects.ErrorWithStatusCode {

	ctx := c.Request.Context()
	adaptor := relay.GetAdaptor(meta.APIType)
	if adaptor == nil {
		return objects.ErrorWrapper(fmt.Errorf("invalid api type: %d", meta.APIType),
			"invalid_api_type", http.StatusBadRequest)
	}
	if err := adaptor.Init(meta); err != nil {
		return objects.ErrorWrapper(err, "init_failed", http.StatusInternalServerError)
	}

	resp, err := adaptor.DoRequest(c, meta, c.Request.Body)
	if err != nil {
		refund(ctx, meta, preConsumed)
		return objects.ErrorWrapper(err, "do_request_failed", http.StatusBadGateway)
	}

	u, _, respErr := adaptor.DoResponse(c, resp, meta)
	if respErr != nil {
		refund(ctx, meta, preConsumed)
		return respErr
	}
	if !op.Billable {
		return nil
	}

	// DoResponse 已按渠道格式解析出 usage，不需要再走 wireformat 提取器
	if u == nil {
		logger.Errorf(ctx, "[pipeline] 用量解析异常：渠道适配器未返回 usage，模型 %s 按 0 结算", op.Model)
		u = &entity.Usage{}
	}
	groupRatio := billingratio.GetGroupRatio(meta.Group)
	quota, content := computeQuota(modelMeta, groupRatio, u)
	if err := objects.PostCost(ctx, meta, preConsumed, quota,
		u.PromptTokens, u.CompletionTokens, u.PromptTokensDetails.CachedTokens, 0, content); err != nil {
		logger.Errorf(ctx, "[pipeline] 结算失败: %v", err)
	}
	return nil
}
```

- [ ] **Step 2: 修复 proxy.go 丢弃 usage**

把 `internal/relay/controller/proxy.go` 中：

```go
	_, _, respErr := adaptor.DoResponse(c, resp, meta)
```

改为：

```go
	usage, _, respErr := adaptor.DoResponse(c, resp, meta)
	if respErr != nil {
		logger.Errorf(ctx, "[RelayProxyHelper] respErr is not nil: %+v", respErr)
		return respErr
	}
	if usage != nil {
		objects.PostConsumeQuota(ctx, usage, meta, 0)
	} else {
		logger.Errorf(ctx, "[RelayProxyHelper] 用量解析异常：模型 %s 按 0 结算，需补适配", meta.ActualModelName)
	}
	return nil
```

并删除原有的 `if respErr != nil { ... }` 与末尾 `return nil`。

- [ ] **Step 3: 修复 audioSpeech.go 结算被注释**

在 `internal/relay/controller/audioSpeech.go` 中，把响应成功后的部分改为在写回响应前置 `succeed = true` 并结算。定位到 `if resp.StatusCode != http.StatusOK { return RelayErrorHandler(resp) }` 之后，插入：

```go
	succeed = true

	// TTS 按输入字符折算的 prompt tokens 计费，无输出 token
	usage := &entity.Usage{
		PromptTokens: objects.PredictAudioPromptTokenCount(ttsRequest.Input, meta.Mode),
	}
	usage.TotalTokens = usage.PromptTokens
	defer func(ctx context.Context) {
		go objects.PostConsumeQuota(ctx, usage, meta, preConsumedQuota)
	}(c.Request.Context())
```

并删除文件中被注释掉的那段 `//io.ReadAll(req.Body)` 到 `//}(c.Request.Context())`。

- [ ] **Step 4: 编译并跑全量测试**

```bash
go build ./... && go test ./internal/... ./pkg/... 2>&1 | tail -40
```

预期：编译通过，测试 PASS

- [ ] **Step 5: 提交**

```bash
git add -A internal/
git commit -m "fix: 消除 rerank 与 audio/speech 的零计费

RelayProxyHelper 拿到 DoResponse 返回的 usage 后直接丢弃，rerank 与
oneapi/proxy 因此零计费；audioSpeech 的 succeed 从未置 true，预扣必然
回滚。两处均已接入结算。"
```

---

## Self-Review

**1. Spec coverage**

| 设计要求 | 覆盖任务 |
|---|---|
| wire 格式由 (渠道, 模型) 决定 | Task 1 |
| usage 提取器按 wire 格式注册，一处实现 | Task 2 |
| 模型不在 model_meta 则拒绝 | Task 6 Step 5 |
| 价格 0 则免费 | Task 6 `computeQuota` |
| usage 解析失败扣 0 + 异常日志 | Task 6 `settle` |
| 非计费操作跳过校验与结算 | Task 7 `geminiBillableActions` |
| 透传 body 不透传 URL | Task 7 Step 5（委托渠道适配器） |
| 路由注册强制绑定 spec | Task 5 `Handler` |
| oneapi/proxy 管理员专用 | Task 3 |
| oneapi/proxy 接入计费 | Task 8 Step 2 |
| rerank 计费 | Task 8 Step 2 |
| audio/speech 结算 | Task 8 Step 3 |
| vertexai 分派 bug | Task 4 |
| 删除 native_billing.go | Task 7 Step 5 |

**2. Placeholder scan**

Task 6 Step 1 的两条断言期望值标注了"需在 Step 4 按实际取整规则校准"——这是 TDD 的正常流程（先写期望再对齐实现），不是占位符，但已明确指出校准方式与依据。

Task 5 Step 4 的 `Execute` 占位实现在 Task 6 Step 5 被完整替换，已注明。

Task 8 中 `executeNormalize` 的占位在 Task 6 Step 7 提到需先加返回 `nil` 的桩以通过编译，Task 8 Step 1 替换。

**3. Type consistency**

- `wireformat.Format` 在 Task 1 定义，Task 2/5/6/7 一致引用
- `usage.Extractor` 签名 `func([]byte, bool) (*entity.Usage, bool)` 在 Task 2 定义，Task 6 `settle` 一致使用
- `pipeline.Operation` 字段 `Model / Billable / InboundWire / IsStream` 在 Task 5 定义，Task 6/7 一致使用
- `computeQuota(modelMeta *model.ModelMeta, groupRatio float64, u *entity.Usage) (int64, string)` 在 Task 6 定义，Task 8 一致调用
- `wireformat.IsVertexGeminiModel` 在 Task 1 定义，Task 4 复用
- `objects.PostCost` 九参签名与 `internal/objects/billing.go:173` 一致

## 未纳入本计划

设计文档中的注册表完整性测试（遍历 gin 路由断言均有 spec）需要 `/v1/*` 全量接入管线后才有意义，而 `/v1/*` 的 `ModeNormalize` 迁移在 Task 8 只完成了执行器与漏洞修复，尚未把 `internal/router/relay.go` 的路由声明改为 `pipeline.Handler`。该迁移涉及 `RelayImageHelper` 与 `RelayAudioHelper` 的独立计费逻辑收敛，风险与工作量都较大，建议在 Task 1-8 上线并观察一段时间后单独立项。
