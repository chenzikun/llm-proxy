# 统一计费管线架构设计

日期：2026-08-31

## 背景

`/gemini/*`、`/anthropic/*`、`/vertexai/*` 三条原生透传路由完全绕过计费和日志系统，用户通过这些路径调用模型不会产生任何消费记录。排查过程中发现这不是个别遗漏，而是架构问题：计费逻辑散落在各个 Controller 里，由每个 Controller 自行决定是否调用，新增路由极易漏接。

审计后确认了四处漏洞：

| 路径 | 问题 |
|---|---|
| `ANY /v1/oneapi/proxy/:channelid/*` | `RelayProxyHelper` 零计费，且 `TokenAuth` 对 URL 参数形式的 `channelid` 未做管理员校验，任意令牌可指定任意渠道免费转发 |
| `POST /v1/rerank` | fallthrough 到 `RelayProxyHelper`，零计费 |
| `POST /v1/audio/speech` | `PostConsume` 整段被注释，且 `succeed` 从未置 true，预扣必然回滚，TTS 实际免费 |
| `/gemini` `/anthropic` `/vertexai` | 原生透传自行发请求、自行解析用量，未接入 `model_meta` 校验 |

另有两个相关缺陷：

- `vertexai.GetAdaptor` 的模型分派逻辑被注释，永远返回 Gemini 适配器。Vertex AI 上的 Claude 模型 URL 按 `rawPredict` 正确发出，响应却按 Gemini 格式解析，功能是坏的。
- `native.go` 把入站 URL 路径拼接到渠道 `BaseURL` 后转发。这只在渠道恰为 AI Studio 时成立；渠道为 Vertex AI 时 URL 结构完全不同，拼出的地址错误。

## 目标

1. 消除所有无计费的转发路径
2. 使"新增路由漏接计费"在机制上不可能发生，不依赖开发者记得写

## 非目标

不做响应侧的全量归一化，即不支持"任意入站 SDK 格式配任意渠道"。理由见下文「为什么不做响应归一化」。

## 核心决策

### 三个正交维度

| 维度 | 决定什么 | 由什么确定 |
|---|---|---|
| 入站接口形式 | 怎么读请求、模型名在哪 | URL 前缀 / SDK |
| 上游 wire 格式 | 怎么构造上游请求、**怎么解析 usage** | (渠道类型, 模型) |
| 模型配置 | 单价 | `model_meta` |

关键点：usage 解析属于上游 wire 格式维度，与入站接口形式无关。客户端用 OpenAI SDK 打 `/v1/chat/completions`、渠道是 Gemini 时，上游返回的是 Gemini 格式响应，usage 必须按 Gemini 解析。

wire 格式由 (渠道类型, 模型) 共同决定，而非仅由渠道类型决定：

| 渠道 | 模型 | 上游动作 | wire 格式 |
|---|---|---|---|
| Gemini (AI Studio) | `gemini-*` | `:generateContent` | Gemini |
| VertexAI | `gemini-*` | `:generateContent` | Gemini |
| VertexAI | `claude-*` | `:rawPredict` | Anthropic |
| Anthropic | `claude-*` | `/v1/messages` | Anthropic |
| AWS Bedrock | `claude-*` | Bedrock invoke | Anthropic |

因此 `UsageExtractor` 按 wire 格式注册，Gemini 解析器只需一份，AI Studio 与 Vertex-Gemini 共用。

### 计费策略

| 场景 | 处理 |
|---|---|
| 模型不在 `model_meta` 中 | **拒绝请求**，返回明确错误提示管理员配置 |
| 模型在 `model_meta` 中但价格为 0 | 正常放行，扣费 0 |
| 上游响应解析不出 usage | 扣费 0，写一条醒目的异常日志等人工补适配 |
| 非生成类接口（files / models / countTokens） | 跳过 `model_meta` 校验与结算 |

第一条与 `/v1/chat/completions` 现有行为一致（`objects.PreConsumeQuota` 查不到 model meta 即返回错误），属于对齐而非新增约束。

### 入站格式的不对称支持

| 入站形式 | 定位 | 支持的渠道 |
|---|---|---|
| `/v1/*` OpenAI | 规范交换格式，做请求与响应的双向归一化 | 任意 |
| `/gemini/*` `/anthropic/*` `/vertexai/*` | 保真透传 | 仅 wire 格式匹配的渠道，不匹配则明确报错 |

归一化只发生在 OpenAI 这一种入站格式上，且该部分代码已存在并在运行（`responseGeminiChat2OpenAI` 等函数）。组合规模是 1×N（已完成）加 3×1（透传），不存在 3×N 的组合爆炸。

#### 为什么不做响应归一化

OpenAI 的响应格式不是其他格式的超集，往其中映射再取出是有损的：

- Anthropic 独有 `thinking` 块、`citations`、细分 `stop_reason`
- Gemini 独有 `safetyRatings`、`promptFeedback`、`groundingMetadata`、`citationMetadata`

请求侧归一化可行，因为请求语义收敛（messages 加参数），丢失的是可选控制项。响应侧不可行：客户端用 Google SDK 就期望拿到 `safetyRatings`，若背后是 OpenAI 渠道，该字段无法凭空构造。

这会让系统对外呈现"任意 SDK 配任意渠道"的假象，实际是静默降级——客户端收到 200 而非错误，问题以字段缺失的形式出现在客户端代码里。一个诚实的 400 优于一个静默失真的 200。

此外这是永久的维护负担：上游每新增一个特性（Gemini 的 grounding、Anthropic 的 extended thinking），都要决定如何塞进 OpenAI 结构，或接受再丢一块信息。

价值也是不对称的。`/v1/*` 配任意渠道价值高，OpenAI 格式是事实标准，生态工具都说这个方言。反过来，特意使用 Google SDK 的用户要的就是 Gemini 行为，他把 base_url 指向代理是为了统一密钥管理、额度和日志，不是为了换模型。

以后若确需 Gemini 入站配 OpenAI 渠道，为这一对补一个响应转换器即可，架构中留有位置，但不预先为所有组合付代价。

### 透传的是 body，不是 URL

原生透传路径不再拼接入站 URL。入站层只从请求中取出模型名和动作，body 原样保留；上游 URL 构造与鉴权交给渠道适配器，它已实现 `GetRequestURL` 与 `SetupRequestHeader`。

```
/gemini/v1beta/models/gemini-2.5-flash:generateContent
        ↓ 取出 model 与 action，body 不碰
渠道 = Gemini(AI Studio) → generativelanguage.googleapis.com/... + ?key= 鉴权
渠道 = VertexAI + gemini → ...-aiplatform.googleapis.com/v1/projects/... + OAuth 鉴权
        ↓ body 原样发送（wire 格式相同）
响应原样返回，usage 用同一个 Gemini 解析器
```

这使"Gemini 与 VertexAI 支持同样的入站接口"成为分层正确的自然结果，而非特殊处理。同时修正了现有实现在 Vertex 渠道下 URL 拼接错误的问题。

## 架构

### 组件划分

```
internal/relay/pipeline/
  spec.go        RelaySpec、Mode、Operation 定义
  registry.go    spec 注册表、路由注册辅助函数、启动校验
  execute.go     管线执行器，独占对上游发请求的能力
  inbound/       各入站格式的 Resolve 实现：openai / gemini / anthropic / vertexai
internal/relay/wireformat/
  define.go      wire 格式枚举
  resolve.go     (渠道类型, 模型) → wire 格式
  usage/         按 wire 格式组织的 usage 解析器
```

### RelaySpec

按路由前缀注册，只负责"识别本次请求是什么"。上游请求的构造不在 spec 里——那是渠道维度的职责，由 `RelayAdaptor` 的 `GetRequestURL` 与 `SetupRequestHeader` 完成，避免入站层重新实现一遍 URL 与鉴权逻辑（现有 `native.go` 正是犯了这个错才导致 Vertex 渠道地址拼错）。

```go
type RelaySpec struct {
    Name string

    // Mode 决定管线走归一化分支还是透传分支
    Mode Mode

    // Resolve 识别本次请求：模型名、是否计费、入站 wire 格式
    Resolve func(c *gin.Context) (*Operation, error)
}

type Mode int

const (
    // ModeNormalize 入站请求转成内部表示，响应转回入站格式。
    // 仅 OpenAI 入站使用，可配任意渠道。
    ModeNormalize Mode = iota
    // ModePassthrough body 原样转发，要求入站 wire 格式与上游一致。
    // 原生前缀使用。
    ModePassthrough
)

type Operation struct {
    Model    string
    Billable bool
    // InboundWire 入站 wire 格式。ModePassthrough 下用于与上游 wire 格式校验，
    // ModeNormalize 下恒为 wireformat.OpenAI 且不参与校验。
    InboundWire wireformat.Format
}
```

`Billable` 与 wire 格式必须按请求解析，不能是 spec 的静态字段。同一前缀下不同操作的计费属性不同（`:generateContent` 计费，`:countTokens` 与 `GET /models` 不计费），同一渠道下不同模型的 wire 格式也不同。

`Mode` 是 spec 的静态属性，因为一个入站前缀的定位不会随请求变化。

### UsageExtractor

按 wire 格式注册的纯函数，不做格式转换、不写响应。

```go
type UsageExtractor func(body []byte, isStream bool) (*entity.Usage, bool)
```

返回 false 表示解析失败，由管线记异常日志并按 0 结算。

每个 wire 格式只有一处实现，`DoResponse` 内部亦复用它，消除现有 `native_billing.go` 中的重复解析逻辑。

### 管线执行器

```go
func Execute(c *gin.Context, spec *RelaySpec) *objects.ErrorWithStatusCode
```

前置阶段两种模式共用：

1. `spec.Resolve` 得到 `Operation`
2. 由 (渠道类型, 模型) 解析上游 wire 格式
3. 若 `spec.Mode == ModePassthrough` 且 `Billable` 且 `Operation.InboundWire` 与上游 wire 格式不匹配，返回明确错误。`ModeNormalize` 跳过此校验，格式差异由适配器转换消化；非计费操作（文件下载、模型列表、`:countTokens`）无用量可解析亦无模型可定价，wire 格式与之无关，同样跳过
4. 若 `Billable`，查 `model_meta`，查不到则拒绝
5. 预扣额度（价格为 0 时预扣额自然为 0）

转发阶段按模式分支：

**ModeNormalize**（`/v1/*`，复用现有链路）

6. `adaptor.ConvertRequest` 把内部表示转成渠道格式
7. `adaptor.DoRequest` 发起请求
8. `adaptor.DoResponse` 转换并写回响应，同时返回 usage

**ModePassthrough**（原生前缀）

6. 委托渠道适配器的 `GetRequestURL` 与 `SetupRequestHeader` 构造上游请求，body 原样带上
7. 发起请求
8. 响应原样透传，`TeeReader` 同步缓冲，流式不影响首字延迟；缓冲内容交给该 wire 格式的 `UsageExtractor` 提取 usage

结算阶段两种模式共用：

9. usage 为空则记异常日志并按 0 计
10. 结算并写消费日志

管线独占对上游发 HTTP 请求的能力。各 Controller 不再持有 `client.HTTPClient`，绕过计费必须先绕过管线，而绕过管线就无法发出请求。

两个分支的差异只在第 6 到 8 步，且都必须经过第 4、5、9、10 步的计费。`ModeNormalize` 分支复用现有 `RelayAdaptor` 全部能力，改动仅是把 `DoResponse` 返回的 usage 从"可以丢弃"变成"必须交给管线结算"——这正是 `RelayProxyHelper` 现在丢掉 usage 造成零计费的根因。

### 强制机制

路由注册时必须绑定 spec 名，spec 未注册则 init 阶段 panic：

```go
pipeline.POST(relayV1Router, "/chat/completions", "openai.chat")
pipeline.POST(relayV1Router, "/embeddings",       "openai.embeddings")
pipeline.ANY(anthropicRouter, "/*path", "anthropic.native")
pipeline.ANY(geminiRouter,    "/*path", "gemini.native")
pipeline.ANY(vertexaiRouter,  "/*path", "vertexai.native")
```

漏接计费的路由会导致服务无法启动，在 CI 阶段即暴露。

## 现有路由到 spec 的映射

| 路由 | spec | Mode | Billable |
|---|---|---|---|
| `POST /v1/chat/completions` | `openai.chat` | Normalize | 是 |
| `POST /v1/completions` | `openai.completions` | Normalize | 是 |
| `POST /v1/embeddings` | `openai.embeddings` | Normalize | 是 |
| `POST /v1/engines/:model/embeddings` | `openai.embeddings` | Normalize | 是 |
| `POST /v1/edits` | `openai.edits` | Normalize | 是 |
| `POST /v1/moderations` | `openai.moderations` | Normalize | 是 |
| `POST /v1/rerank` | `openai.rerank` | Normalize | 是（修复零计费） |
| `POST /v1/images/generations` | `openai.images` | Normalize | 是 |
| `POST /v1/audio/transcriptions` | `openai.audio.transcription` | Normalize | 是 |
| `POST /v1/audio/translations` | `openai.audio.translation` | Normalize | 是 |
| `POST /v1/audio/speech` | `openai.audio.speech` | Normalize | 是（恢复被注释的结算） |
| `GET /v1/files/:id/content` | `openai.files.content` | Passthrough | 否 |
| `POST /v1/fine_tuning/jobs/:id/cancel` | `openai.finetuning.cancel` | Passthrough | 否 |
| `GET /v1/fine_tuning/jobs/:id/events` | `openai.finetuning.events` | Passthrough | 否 |
| `ANY /gemini/*path` | `gemini.native` | Passthrough | 按操作解析 |
| `ANY /anthropic/*path` | `anthropic.native` | Passthrough | 按操作解析 |
| `ANY /vertexai/*path` | `vertexai.native` | Passthrough | 按操作解析 |

`ANY /v1/oneapi/proxy/:channelid/*target` 需单独处置。它是无计费的任意渠道裸转发，且缺少管理员校验，与本设计的强制计费原则直接冲突。建议下线该路由；若业务确需保留，须改为管理员专用并接入计费。此项待确认。

原生前缀下的非计费操作：

| 操作 | Billable |
|---|---|
| `:generateContent` `:streamGenerateContent` | 是 |
| `:embedContent` `:batchEmbedContents` | 是 |
| `:countTokens` | 否 |
| `GET /v1beta/models` | 否 |
| `/v1/messages` | 是 |
| `/v1/messages/count_tokens` | 否 |

## 错误处理

| 情况 | 响应 | 副作用 |
|---|---|---|
| 模型名解析失败 | 400 | 无 |
| 模型不在 `model_meta` | 400，提示配置模型价格 | 无 |
| 入站 wire 格式与渠道不匹配 | 400，提示该入站接口不支持当前渠道类型 | 无 |
| 用户额度不足 | 403 | 无 |
| 上游返回错误 | 原样透传上游状态码与响应体 | 回滚预扣 |
| usage 解析失败 | 200，响应正常返回 | 扣 0，写异常日志 |

错误响应格式按入站形式决定。Gemini SDK 期望 Google 的错误结构，Anthropic SDK 期望 Anthropic 的错误结构，不能统一返回 OpenAI 格式的错误。

## 与重试逻辑的关系

`handler.Relay` 现有的多渠道重试循环在管线之外。预扣在首次进入管线时执行一次，重试切换渠道时不重复预扣；结算在最终成功的那次执行。重试导致渠道变化时，wire 格式需重新解析，因为新渠道的 wire 格式可能不同。

## 测试策略

| 层次 | 内容 |
|---|---|
| `Resolve` 单测 | 表驱动，给定 URL 与 body，断言模型名、Billable、入站 wire 格式 |
| `UsageExtractor` 单测 | 给定各 wire 格式的真实响应 fixture（流式与非流式），断言 usage |
| wire 格式解析单测 | 给定 (渠道类型, 模型)，断言 wire 格式，覆盖 Vertex 的 Gemini/Claude 分派 |
| 管线单测 | model_meta 缺失时拒绝；价格 0 时放行且扣 0；usage 解析失败时扣 0 并记日志；wire 不匹配时拒绝 |
| 注册表完整性测试 | 遍历所有已注册路由，断言均有对应 spec |

`UsageExtractor` 的 fixture 需覆盖已知的四处真实格式：Gemini 非流式 `usageMetadata`、Gemini SSE 末尾 chunk、Anthropic 非流式 `usage`、Anthropic SSE 的 `message_start` 加 `message_delta`。

## 附带修复

1. `TokenAuth` 中 URL 参数形式的 `channelid` 补管理员校验，与 `parts[1]` 形式一致
2. 恢复 `vertexai.GetAdaptor` 被注释的模型分派逻辑
3. 删除 `internal/relay/controller/native_billing.go`，其解析逻辑迁入按 wire 格式组织的 usage 包

## 待确认事项

`ANY /v1/oneapi/proxy/:channelid/*target` 的处置方式：下线，还是改为管理员专用并接入计费。
