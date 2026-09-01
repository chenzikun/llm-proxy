# 多模态计费统一设计（图片 / 音频 / 微调）

日期：2026-09-01

## 背景

统一计费管线（见 `2026-08-31-billing-pipeline-design.md`）把 `/gemini`、`/anthropic`、`/vertexai` 与 rerank、oneapi/proxy、TTS 接入了 `objects.PostConsumeQuota`。随后对 `/v1/*` 全量路由做静态审计，发现还有四条路由的计费与 `model_meta` 中配置的价格无关。

根因不是"忘记调用结算"，而是这四条仍在读迁移前的两张费率表，而按架构约定（代码中不硬编码默认模型）这两张表已是空 map：

```go
// internal/relay/billing/ratio/model.go:6-26
var ModelRatio = map[string]float64{}
var FineTuningRatio = map[string]float64{}

func GetModelRatio(name string, channelType int) float64 {
    if ratio, ok := ModelRatio[name]; ok { return ratio }
    return 0          // 任何模型 -> 0，即免费
}

func GetFineTuningRatio(name string) float64 {
    if ratio, ok := FineTuningRatio[name]; ok { return ratio }
    return -1         // 任何模型 -> -1，即负价
}
```

两个兜底值方向不同，导致两类不同的故障。

### 审计结论

| 路径 | 问题 | 位置 |
|---|---|---|
| `POST /v1/images/generations` | `ratio = GetModelRatio(...) * groupRatio = 0`，`quota` 恒为 0；且下方 `if quota != 0` 守卫使 `RecordConsumeLog` 整块被跳过，既不扣费也无消费日志。余额为 0 的用户可无限生成 | `internal/relay/controller/image.go:172-210` |
| `POST /v1/audio/transcriptions`<br>`POST /v1/audio/translations` | `quota = CountTokenText(text)` 直接取输出文本 token 数，未乘任何单价。因 `QuotaPerUnit = 1e6`，等价于把单价硬编码为 ¥1/1M token，`model_meta` 完全不参与 | `internal/relay/controller/audio.go:259` |
| `POST /v1/audio/transcriptions`<br>（`response_format=srt\|vtt`） | 转发给上游的 multipart 被重建，只写 `file` 与 `model`，客户端的 `response_format` 被丢弃。上游返回默认 json，代理却用 `getTextFromSRT` 解析——该函数靠 `-->` 定位字幕行，JSON 中无匹配，返回空串且不报错，`quota` 为 0。**加一个 `response_format=srt` 参数即可免费转写** | `internal/relay/controller/audio.go:170-174, 299-318` |
| `POST /v1/fine_tuning/jobs` | `GetFineTuningRatio` 返回 -1 使预扣配额为负，`PreCost` 中 `CacheDecreaseUserQuota(userId, 负数)` 等于给用户**增加**余额；`userQuota < preConsumedQuota` 因比较对象为负永不拦截；`PreCost` 的错误返回值在调用处被丢弃；事后无冲正 | `internal/objects/training_file.go:28`<br>`internal/handler/fineTuning.go:48`<br>`internal/objects/billing.go:163-171` |

附带缺陷：

- `PredictAudioPromptTokenCount` 对 TTS 返回 `len(input)`，即 UTF-8 **字节数**。上游按字符计价，中文一个字 3 字节，中文 TTS 被超收约 3 倍。位置 `internal/objects/token.go:250-256`。
- 转写重建 multipart 时同时丢弃了 `language`、`prompt`、`temperature`，在悄悄降低识别质量。
- 前端 `renderPrice` 把每百万 token 的价格标注成 `/1K`。位置 `web/src/views/ModelMeta/component/TableRow.js:178-182`。

## 目标

1. 五条链路（文本、图片、TTS、转写、微调）的计费全部由 `model_meta` 中管理员配置的价格决定
2. 计量维度与上游实际计价维度一致，不再出现"同样成本收费不同"
3. 收敛为单一计费实现：删除迁移前遗留的两套旧实现，全仓只保留一处消费日志写入点

## 非目标

- 不支持"输入按 token、输出按张"这类单行内混合计量（见下文限制）
- 不为每种图片尺寸单独建 `model_meta` 行
- 不引入音频解码依赖来本地测量时长

## 核心决策

### 价格字段的语义泛化

计费引擎的核心换算是：

```go
// internal/objects/billing.go:26-29
func tokenQuotaRatio(priceCNYPerM float64, groupRatio float64) float64 {
	return priceCNYPerM * config.QuotaPerUnit / 1000000.0 * groupRatio
}
```

因 `QuotaPerUnit = 1e6`，该式退化为 `price × groupRatio`。引擎真实语义是**"¥ per 100 万个计量单位"**，"token" 只是当前唯一用到的单位。

因此新增 `model_meta.billing_unit` 枚举字段，价格字段语义保持不变：**`input_price` / `output_price` 分别是输入侧与输出侧每 100 万个计量单位的价格**，单位由 `billing_unit` 决定。

| `billing_unit` | 适用 | `input_price` | `output_price` |
|---|---|---|---|
| `token`（默认） | 文本、gpt-image-1、Gemini Image | ¥/1M 输入 token | ¥/1M 输出 token |
| `char` | TTS | ¥/1M 字符 | 0（不计） |
| `second` | 转写、翻译 | ¥/1M 秒 | 0（不计） |
| `image` | DALL·E、Imagen、万相 | 0（不计 prompt） | ¥/1M 张 |

各模态共用同一套价格字段与同一个 `getModelPricesInCNY` 换汇逻辑。`tokenQuotaRatio` 无需改动。

### 统一结算出口是 PostCost

`objects.PostConsumeQuota` 接收 `*entity.Usage`，从 `promptTokens` / `completionTokens` 反推配额，是 token 模态的便捷封装。图片的"张数 × 尺寸系数"与转写的"秒数"无法塞进这个形状，不应为了复用而把张数伪装成 completionTokens。

真正的统一出口是下层的 `PostCost`（`internal/objects/billing.go:191`），它接收一个**已算好的配额**并负责扣费、更新缓存与用量、写消费日志：

```go
func PostCost(ctx, meta, preConsumedQuota, actuallyConsumedQuota int64,
    promptTokens, completionTokens, cacheTokens int, cacheQuota int, logContent string) error
```

分工：

| 模态 | 结算调用 | 配额计算位置 |
|---|---|---|
| 文本、按 token 的图片 | `PostConsumeQuota` → `PostCost` | `PostConsumeQuota` 内，按 usage |
| TTS | `PostConsumeQuota` → `PostCost` | 同上，字符数填入 `PromptTokens` |
| 按张的图片 | 直接 `PostCost` | 调用方按张数 × 系数算 |
| 转写 / 翻译 | 直接 `PostCost` | 调用方按秒数算 |

`PostCost` 无条件调用 `RecordConsumeLog`，不存在 `billing.PostConsumeQuota` 那种 `if totalQuota != 0` 守卫，因此配额为 0 时仍会产生消费记录。

沿用 `PostConsumeQuota` 中的最小额度规则：单价不为 0 但配额向上取整后仍为 0 时，按 1 额度结算，避免小额请求完全免费。

### 预扣策略

预扣的作用是拦住余额不足的请求，精度不重要，实际结算时按真实用量补差。各模态能否精确预扣取决于请求本身是否包含计量信息：

| 模态 | 请求前可知的计量 | 预扣 |
|---|---|---|
| 图片（按张） | `N` 与 `size` 均在请求体中 | 可精确预扣，等于最终配额 |
| TTS | 输入文本长度已知 | 可精确预扣（现有行为） |
| 微调 | `epochs` 与训练文件 `tokens` 已知 | 可精确预扣 |
| 转写 / 翻译 | **时长未知**，需上游响应才拿到 | 按 `config.PreConsumedQuota` 保底预扣，结算时按 `duration` 补差 |

转写的保底预扣无法防住"上传超长音频但余额仅够保底额度"的情况，会产生小额负余额。这与文本流式接口的现有风险同源（预扣按预估、实际用量可能更高），不在本次范围内单独处理。

**已知限制**：一行只能有一个 `billing_unit`，无法表达"输入按 token、输出按张"。当前不成问题——传统按张模型不收 prompt 费（`input_price` 填 0），gpt-image-1 输入输出均为 token。若将来出现混合计价模型需扩展 schema。

### 存储与展示分离

`billing_unit = image` 时，管理员在界面填"每张 ¥0.3"，前端提交前乘 1e6 存为 300000，读回时除 1e6 显示。存储层与计算层统一为"每 1M 单位"，管理员不面对天文数字。其余单位不做换算。

## 各模态实现

### 图片

`ImageSizeRatios` 不是空表，`dall-e-2/3`、`ali-stable-diffusion-*`、`wanx-v1`、`step-1x-medium` 均有值，且它是**相对系数**（1024² 为基准 1.0，dall-e-3 的 1024×1792 为 2.0），不是价格。保留该表，仅替换被乘为 0 的价格来源：

```
outputRatio = output_price(¥/1M张) × QuotaPerUnit / 1e6 × groupRatio
quota       = ceil(N × imageCostRatio × outputRatio)
```

`imageCostRatio` 继续由 `getImageCostRatio` 提供（含 dall-e-3 的 hd 画质加成）。`billing_unit = image` 时 `input_price` 与 `cache_price` 被忽略，不参与计算。

删除 `if quota != 0` 守卫与内联的 `PostConsumeTokenQuota` / `RecordConsumeLog` / `UpdateChannelUsedQuota` 三段重复实现，改为调用 `PostCost`，价格为 0 时也产生消费记录，否则无法审计。

`billing_unit = token` 的图片模型（gpt-image-1、Gemini Image）不走 `imageCostRatio`，按响应中的 `usage` 正常结算。

### 转写 / 翻译

计量单位为音频秒数，`quota = ceil(duration) × inputRatio`，与 Whisper 上游"按秒取整"一致。

时长来源：对上游固定请求 `response_format=verbose_json`，从响应的 `duration` 字段读取。代理已在重建 multipart 请求，注入该字段只是增加一次 `writer.WriteField`。

响应处理改为：以 `verbose_json` 收上游响应 → 取 `duration` 计费 → 按客户端原本请求的格式转换后返回。`verbose_json` 的 `segments` 含时间戳与文本，重建 `text` / `srt` / `vtt` 信息充足；客户端本就请求 `verbose_json` 时直接透传。

同时补齐转发被丢弃的 `language`、`prompt`、`temperature` 字段。

此改动会改变客户端收到的响应内容——这是本批改动中唯一影响响应体的部分。已与需求方确认：当前非 json 格式本就是坏的（返回 json 且计费为 0），可以直接修正。

### TTS

计量维度本已正确（上游按字符），仅实现有误。`PredictAudioPromptTokenCount` 中 `len(input)` 改为 `utf8.RuneCountInString(input)`。`billing_unit = char`，无其他改动。

### 微调

仅把 `GetFineTuningRatio` 的兜底值从 -1 改成 0 可以消除反向计费，但 `FineTuningRatio` 同样是永久空表，结果是微调永久免费——换一个漏洞而已。因此微调也改为读 `model_meta`：

```
quota = ceil(epochs × tokens × inputRatio)
```

训练按 token 计量，`billing_unit = token` 即可覆盖，无需新单位。模型未在 `model_meta` 中配置时拒绝请求，与其他模态一致。

`handler/fineTuning.go` 中 `objects.PreCost(...)` 的错误返回值必须接住并中断请求，当前被丢弃。

### 旧实现清理

本次改动后，迁移前的计费实现全部失去调用方，应一并删除，使系统只剩一套计费路径：

| 目标 | 当前调用方 | 改动后 |
|---|---|---|
| `billing.PostConsumeQuota`<br>（`internal/relay/billing/billing.go`） | 仅 `audio.go:268` | 无调用方，删除 |
| `GetModelRatio` + `ModelRatio`<br>（`internal/relay/billing/ratio/model.go`） | 仅 `audio.go:57`、`image.go:172` | 无调用方，删除 |
| `GetFineTuningRatio` + `FineTuningRatio`<br>（同上文件） | 仅 `training_file.go:29` | 无调用方，删除 |

`ratio/model.go` 整个文件可删除。`ratio/group.go` 中的 `GetGroupRatio` 与 `ratio/image.go` 中的尺寸系数表、`ImageOriginModelName` 仍在使用，保留。

删除后全仓应只存在一处 `RecordConsumeLog` 调用点（`PostCost` 内），可作为验收条件。

## 数据迁移

`ModelMeta` 增加字段：

```go
BillingUnit string `json:"billing_unit" gorm:"column:billing_unit;default:'token'" csv:"billing_unit"`
```

由 GORM `AutoMigrate` 自动加列，存量行走 `default:'token'`，行为不变，无需迁移脚本。

后端在 `model-meta` 的写入接口校验枚举白名单，非法值拒绝。

## 前端改动

| 文件 | 改动 |
|---|---|
| `type/Config.js` | 增加 `billing_unit` 默认值、标签、提示；价格标签改为按单位动态生成 |
| `component/EditModal.js` | 增加单位下拉；价格字段标签与提示随单位变化；校验白名单；`image` 单位下提交乘 1e6、读回除 1e6 |
| `component/TableHead.js` | 增加「计量单位」列 |
| `component/TableRow.js` | 展示计量单位；修正 `renderPrice` 把每百万标成 `/1K` 的错误 |
| `component/BatchModal.js` | 管道分隔头部增加 `billing_unit` |
| `component/UploadFileModal.js` | 依赖后端 `csv` tag，无需改动逻辑 |

## 测试策略

单元测试：

- `billing_unit` 四种取值下的 quota 计算，含 USD 换汇与分组倍率
- 图片：N 张 × 尺寸系数的组合，验证 dall-e-3 的 1024×1792 为基准价 2 倍
- 图片：价格为 0 时仍写消费日志
- 转写：`duration` 提取与 `ceil` 取整边界（0.4s → 1，1.0s → 1，1.1s → 2）
- 转写：`verbose_json` → `text` / `srt` / `vtt` 的格式转换正确性
- TTS：中文输入的字符数计量，验证不再按字节超收
- 微调：`epochs × tokens` 的配额计算；模型未配置时请求被拒绝而非产生负配额；`PreCost` 返回错误时请求中断

端到端：以 mock 上游覆盖图片与转写两条路径，确认消费日志落库且额度与配置价格一致。

## 不做的事

- 不改 `tokenQuotaRatio`、`getModelPricesInCNY`、`PostConsumeQuota` 的计算逻辑
- 不为图片尺寸建立数据库配置（保留代码内的相对系数表）
- 不本地解码音频测量时长
