# 多模态计费统一实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让图片、TTS、转写、微调四条链路的计费全部由 `model_meta` 中管理员配置的价格决定，并删除迁移前遗留的两套旧计费实现。

**Architecture:** 给 `model_meta` 增加 `billing_unit` 枚举字段（`token`/`char`/`second`/`image`），把价格字段语义从"每百万 token"泛化为"每百万计量单位"。各模态的配额计算集中在 `internal/objects` 包内（控制器只负责测量物理量：张数、秒数），统一经 `PostCost` 结算。

**Tech Stack:** Go 1.21+（go.mod 声明 toolchain 1.23.0）、Gin、GORM、React + MUI（Berry 主题）

**设计依据：** `docs/superpowers/specs/2026-09-01-multimodal-billing-design.md`

## Global Constraints

- 基准货币 ¥（人民币），代码中硬编码 ¥ 符号，不可配置
- `QuotaPerUnit = 1000 * 1000.0`（100 万额度 = ¥1），定义于 `pkg/common/config/config.go:22`
- `ExchangeRate` 默认 7.2，用于把 `price_unit = USD` 的价格换算成 ¥
- 价格字段语义：`input_price` / `output_price` / `cache_price` 均为**每 100 万个计量单位**的价格，单位由 `billing_unit` 决定
- 所有金额向上取整（`math.Ceil`），单价不为 0 但取整后配额为 0 时按 1 额度结算
- 本地 Go 可能是 1.20，会导致 `go build` 失败。执行前先切到 Homebrew 的 Go：
  `export PATH="/opt/homebrew/opt/go/bin:$PATH" && export GOROOT="/opt/homebrew/opt/go/libexec"`
- 消费日志只允许经 `objects.PostCost` 写入，不得新增 `RecordConsumeLog` 调用点

---

## File Structure

**新建：**

| 文件 | 职责 |
|---|---|
| `internal/objects/billing_multimodal.go` | 图片与转写的配额计算及结算入口，仅暴露 Pre/Post 四个函数 |
| `internal/objects/billing_multimodal_test.go` | 上述配额计算的单元测试 |
| `internal/relay/controller/audio_format.go` | `verbose_json` → `text`/`srt`/`vtt` 的响应格式转换 |
| `internal/relay/controller/audio_format_test.go` | 格式转换的单元测试 |

**修改：**

| 文件 | 改动 |
|---|---|
| `internal/repo/model-meta.go` | 增加 `BillingUnit` 字段与枚举常量、校验函数 |
| `internal/handler/model-meta.go` | 请求结构体增加字段，写入前校验枚举 |
| `internal/objects/billing.go` | `tokenQuotaRatio` 更名 `unitQuotaRatio` |
| `internal/objects/token.go` | TTS 字符计量由字节改为 rune |
| `internal/objects/training_file.go` | 微调配额改读 `model_meta` |
| `internal/handler/fineTuning.go` | 接住 `PreCost` 的错误返回值 |
| `internal/relay/controller/image.go` | 删除内联计费，改调 `objects` 入口 |
| `internal/relay/controller/audio.go` | 补齐转发字段、注入 `verbose_json`、按时长计费、转换响应 |
| `web/src/views/ModelMeta/**` | 6 个文件，见 Task 9 |

**删除：**

| 文件 | 原因 |
|---|---|
| `internal/relay/billing/ratio/model.go` | `ModelRatio` / `FineTuningRatio` 两张空表及其 getter 失去全部调用方 |
| `internal/relay/billing/billing.go` 中的 `PostConsumeQuota` | 唯一调用方 `audio.go` 已迁移 |

---

### Task 1: `billing_unit` 字段与写入校验

**Files:**
- Modify: `internal/repo/model-meta.go:14-26`
- Modify: `internal/handler/model-meta.go`
- Test: `internal/repo/model-meta_test.go`

**Interfaces:**
- Produces: `model.BillingUnitToken` / `BillingUnitChar` / `BillingUnitSecond` / `BillingUnitImage` 常量（字符串）；`model.IsValidBillingUnit(string) bool`；`ModelMeta.BillingUnit` 字段

- [ ] **Step 1: 写失败的测试**

创建 `internal/repo/model-meta_test.go`：

```go
package model

import "testing"

func TestIsValidBillingUnit(t *testing.T) {
	valid := []string{BillingUnitToken, BillingUnitChar, BillingUnitSecond, BillingUnitImage}
	for _, u := range valid {
		if !IsValidBillingUnit(u) {
			t.Errorf("IsValidBillingUnit(%q) = false, 期望 true", u)
		}
	}
	invalid := []string{"", "minute", "TOKEN", "images", "秒"}
	for _, u := range invalid {
		if IsValidBillingUnit(u) {
			t.Errorf("IsValidBillingUnit(%q) = true, 期望 false", u)
		}
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/repo/ -run TestIsValidBillingUnit -v`
Expected: FAIL，编译错误 `undefined: BillingUnitToken`

- [ ] **Step 3: 加字段与校验函数**

在 `internal/repo/model-meta.go` 的 `ModelMeta` 结构体中，`PriceUnit` 字段之后插入：

```go
	// BillingUnit 计量单位，决定 input_price / output_price 中"每百万"的单位是什么
	BillingUnit string `json:"billing_unit" gorm:"column:billing_unit;default:'token'" csv:"billing_unit"`
```

在同文件 `type ModelMeta struct {...}` 之后插入：

```go
// 计量单位。价格字段恒为"每 100 万个计量单位的价格"，本枚举决定这个单位是什么。
const (
	BillingUnitToken  = "token"  // 文本、按 token 计价的图片模型
	BillingUnitChar   = "char"   // TTS，按输入字符
	BillingUnitSecond = "second" // 转写 / 翻译，按音频秒数
	BillingUnitImage  = "image"  // 按张计价的图片模型
)

// IsValidBillingUnit 校验计量单位取值。空字符串不合法，写入方需显式落 token。
func IsValidBillingUnit(unit string) bool {
	switch unit {
	case BillingUnitToken, BillingUnitChar, BillingUnitSecond, BillingUnitImage:
		return true
	}
	return false
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/repo/ -run TestIsValidBillingUnit -v`
Expected: PASS

- [ ] **Step 5: 接入写入接口**

`internal/handler/model-meta.go` 中，`AddModelMetaRequest` 与 `UpdateModelMetaRequest` 结构体各增加一个字段（`Add` 用值类型，`Update` 用指针以支持部分更新）：

```go
	// AddModelMetaRequest 中
	BillingUnit string `json:"billing_unit"`

	// UpdateModelMetaRequest 中
	BillingUnit *string `json:"billing_unit"`
```

`AddModelMeta` 中在 `priceUnit` 处理之后、构造 `meta` 之前插入：

```go
	billingUnit := req.BillingUnit
	if billingUnit == "" {
		billingUnit = model.BillingUnitToken
	}
	if !model.IsValidBillingUnit(billingUnit) {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "billing_unit 取值非法，可选：token / char / second / image",
		})
		return
	}
```

并在 `meta := &model.ModelMeta{...}` 中增加 `BillingUnit: billingUnit,`。

`UpdateModelMeta` 的动态字段构建中，在 `req.PriceUnit` 分支之后插入：

```go
	if req.BillingUnit != nil {
		if !model.IsValidBillingUnit(*req.BillingUnit) {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "billing_unit 取值非法，可选：token / char / second / image",
			})
			return
		}
		updates["billing_unit"] = *req.BillingUnit
	}
```

`BatchAddModelMeta` 中，`for _, modelMeta := range modelMetas` 循环内 `if modelMeta.Model == ""` 判断之后插入（CSV 导入允许留空，留空即 token）：

```go
		if modelMeta.BillingUnit == "" {
			modelMeta.BillingUnit = model.BillingUnitToken
		}
		if !model.IsValidBillingUnit(modelMeta.BillingUnit) {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": fmt.Sprintf("模型 %s 的 billing_unit 取值非法：%s", modelMeta.Model, modelMeta.BillingUnit),
			})
			return
		}
```

- [ ] **Step 6: 编译验证**

Run: `go build ./...`
Expected: 无输出（成功）。若报 `fmt` 未导入，在 `internal/handler/model-meta.go` 补上。

- [ ] **Step 7: 提交**

```bash
git add internal/repo/model-meta.go internal/repo/model-meta_test.go internal/handler/model-meta.go
git commit -m "feat(model-meta): 增加 billing_unit 计量单位字段"
```

---

### Task 2: 按单位结算入口

**Files:**
- Create: `internal/objects/billing_multimodal.go`
- Create: `internal/objects/billing_multimodal_test.go`
- Modify: `internal/objects/billing.go:26-29, 49, 76-78, 133, 157`

**Interfaces:**
- Consumes: Task 1 的 `model.BillingUnit*` 常量
- Produces:
  - `objects.PreConsumeImageQuota(ctx context.Context, meta *Meta, n int, sizeRatio float64) (int64, *ErrorWithStatusCode)`
  - `objects.PostConsumeImageQuota(ctx context.Context, meta *Meta, n int, sizeRatio float64, preConsumedQuota int64)`
  - `objects.PreConsumeTranscriptionQuota(ctx context.Context, meta *Meta) (int64, *ErrorWithStatusCode)`
  - `objects.PostConsumeTranscriptionQuota(ctx context.Context, meta *Meta, durationSeconds float64, preConsumedQuota int64)`
  - 包内私有：`unitQuotaRatio(priceCNYPerM, groupRatio float64) float64`

- [ ] **Step 1: 写失败的测试**

创建 `internal/objects/billing_multimodal_test.go`：

```go
package objects

import (
	"math"
	"testing"

	model "github.com/zicorn/llm-proxy/internal/repo"
	"github.com/zicorn/llm-proxy/pkg/common/config"
)

func TestUnitQuotaRatio(t *testing.T) {
	// QuotaPerUnit = 1e6 时，比率退化为 单价 × 分组倍率
	got := unitQuotaRatio(3.0, 1.0)
	if math.Abs(got-3.0) > 1e-9 {
		t.Errorf("unitQuotaRatio(3.0, 1.0) = %v, 期望 3.0", got)
	}
	got = unitQuotaRatio(3.0, 0.5)
	if math.Abs(got-1.5) > 1e-9 {
		t.Errorf("unitQuotaRatio(3.0, 0.5) = %v, 期望 1.5", got)
	}
}

func TestComputeImageQuota(t *testing.T) {
	// 每张 ¥0.3 => output_price = 300000（¥/1M 张）
	// 2 张，尺寸系数 2.0 => 0.3 × 2 × 2 = ¥1.2 => 1_200_000 额度
	quota := computeImageQuota(300000, 1.0, 2, 2.0)
	if quota != 1_200_000 {
		t.Errorf("computeImageQuota = %d, 期望 1200000", quota)
	}
	// 单价为 0 时不收费
	if q := computeImageQuota(0, 1.0, 3, 1.0); q != 0 {
		t.Errorf("单价为 0 时 quota = %d, 期望 0", q)
	}
	// 单价极小时至少收 1 额度，不能因取整而免费
	if q := computeImageQuota(0.0001, 1.0, 1, 1.0); q != 1 {
		t.Errorf("极小单价 quota = %d, 期望 1", q)
	}
}

func TestComputeTranscriptionQuota(t *testing.T) {
	// ¥720/1M 秒，60 秒 => 720 × 60 = 43200 额度
	if q := computeTranscriptionQuota(720, 1.0, 60); q != 43200 {
		t.Errorf("computeTranscriptionQuota = %d, 期望 43200", q)
	}
	// 秒数向上取整：0.4s -> 1s，1.0s -> 1s，1.1s -> 2s
	cases := []struct {
		seconds float64
		want    int64
	}{{0.4, 720}, {1.0, 720}, {1.1, 1440}}
	for _, c := range cases {
		if q := computeTranscriptionQuota(720, 1.0, billedSeconds(c.seconds)); q != c.want {
			t.Errorf("%.1fs 的 quota = %d, 期望 %d", c.seconds, q, c.want)
		}
	}
	if q := computeTranscriptionQuota(0, 1.0, 60); q != 0 {
		t.Errorf("单价为 0 时 quota = %d, 期望 0", q)
	}
}

func TestBilledSeconds(t *testing.T) {
	cases := []struct {
		in   float64
		want int64
	}{{0, 0}, {0.001, 1}, {0.4, 1}, {1.0, 1}, {1.1, 2}, {59.9, 60}}
	for _, c := range cases {
		if got := billedSeconds(c.in); got != c.want {
			t.Errorf("billedSeconds(%v) = %d, 期望 %d", c.in, got, c.want)
		}
	}
}

func TestUSDConversionAppliesToImage(t *testing.T) {
	// 以 USD 计价的每张价格需按汇率换算成 ¥
	config.ExchangeRate = 7.2
	meta := &model.ModelMeta{OutputPrice: 100000, PriceUnit: "USD"}
	_, outputCNY, _ := getModelPricesInCNY(meta)
	if math.Abs(outputCNY-720000) > 1e-6 {
		t.Errorf("USD 换算后 = %v, 期望 720000", outputCNY)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/objects/ -run 'TestUnitQuotaRatio|TestComputeImageQuota|TestComputeTranscriptionQuota|TestBilledSeconds' -v`
Expected: FAIL，编译错误 `undefined: unitQuotaRatio`、`undefined: computeImageQuota`

- [ ] **Step 3: 重命名 tokenQuotaRatio**

`internal/objects/billing.go` 中把 `tokenQuotaRatio` 改名为 `unitQuotaRatio`，注释同步更新。共 5 处引用：定义处（26-29 行）、`PreConsumeQuota`（49 行）、`PostConsumeQuota`（76-78 行三处）、`PreConsumeQuotaForAudio`（133 行）、`PreConsumeQuotaByTokens`（157 行）。

新的定义：

```go
// unitQuotaRatio 根据 CNY 单价（每百万计量单位）和分组倍率计算每个计量单位的额度消耗。
//
// 计量单位由 model_meta.billing_unit 决定：token / char / second / image。
// 本函数与单位无关，只做"单价 → 每单位额度"的换算。
func unitQuotaRatio(priceCNYPerM float64, groupRatio float64) float64 {
	return priceCNYPerM * config.QuotaPerUnit / 1000000.0 * groupRatio
}
```

- [ ] **Step 4: 实现多模态结算入口**

创建 `internal/objects/billing_multimodal.go`：

```go
package objects

import (
	"context"
	"fmt"
	"math"
	"net/http"

	model "github.com/zicorn/llm-proxy/internal/repo"
	billingratio "github.com/zicorn/llm-proxy/internal/relay/billing/ratio"
	"github.com/zicorn/llm-proxy/pkg/common/logger"
)

// preConsumedAudioSeconds 转写请求的保底预扣秒数。
//
// 音频时长要等上游响应才能得知，预扣阶段只能按一个固定值占位，结算时按真实
// duration 补差。取 600 秒是在"拦住余额不足"与"不过度阻挡正常请求"之间折中。
const preConsumedAudioSeconds = 600

// billedSeconds 按秒向上取整，与 Whisper 上游的计价方式一致。
func billedSeconds(duration float64) int64 {
	if duration <= 0 {
		return 0
	}
	return int64(math.Ceil(duration))
}

// computeImageQuota 计算按张计价的图片配额。
//
// sizeRatio 是尺寸与画质的相对系数（1024x1024 为基准 1.0），由调用方从
// ImageSizeRatios 取得；outputPriceCNY 是每百万张的 ¥ 价格。
func computeImageQuota(outputPriceCNY, groupRatio float64, n int, sizeRatio float64) int64 {
	ratio := unitQuotaRatio(outputPriceCNY, groupRatio)
	quota := int64(math.Ceil(float64(n) * sizeRatio * ratio))
	if ratio != 0 && quota <= 0 && n > 0 {
		quota = 1
	}
	return quota
}

// computeTranscriptionQuota 计算按秒计价的转写配额。
func computeTranscriptionQuota(inputPriceCNY, groupRatio float64, seconds int64) int64 {
	ratio := unitQuotaRatio(inputPriceCNY, groupRatio)
	quota := int64(math.Ceil(float64(seconds) * ratio))
	if ratio != 0 && quota <= 0 && seconds > 0 {
		quota = 1
	}
	return quota
}

// imagePricing 取图片模型的每百万张价格与分组倍率。
func imagePricing(meta *Meta) (outputPriceCNY, groupRatio float64, err error) {
	modelMeta, err := model.GetModelMetaByModel(meta.ActualModelName)
	if err != nil {
		return 0, 0, err
	}
	_, outputPriceCNY, _ = getModelPricesInCNY(modelMeta)
	return outputPriceCNY, billingratio.GetGroupRatio(meta.Group), nil
}

// audioPricing 取音频模型的每百万秒价格与分组倍率。
func audioPricing(meta *Meta) (inputPriceCNY, groupRatio float64, err error) {
	modelMeta, err := model.GetModelMetaByModel(meta.ActualModelName)
	if err != nil {
		return 0, 0, err
	}
	inputPriceCNY, _, _ = getModelPricesInCNY(modelMeta)
	return inputPriceCNY, billingratio.GetGroupRatio(meta.Group), nil
}

// PreConsumeImageQuota 预扣图片生成配额。
//
// 张数与尺寸在请求体中已知，因此预扣值等于最终配额，结算时差额为 0。
func PreConsumeImageQuota(ctx context.Context, meta *Meta, n int, sizeRatio float64) (int64, *ErrorWithStatusCode) {
	outputPriceCNY, groupRatio, err := imagePricing(meta)
	if err != nil {
		return 0, ErrorWrapper(err, "get_model_meta_failed", http.StatusInternalServerError)
	}
	return PreCost(ctx, meta, computeImageQuota(outputPriceCNY, groupRatio, n, sizeRatio))
}

// PostConsumeImageQuota 结算图片生成配额。
//
// 单价为 0 时仍会写消费日志，否则图片请求在日志中不可见、无法审计。
func PostConsumeImageQuota(ctx context.Context, meta *Meta, n int, sizeRatio float64, preConsumedQuota int64) {
	outputPriceCNY, groupRatio, err := imagePricing(meta)
	if err != nil {
		logger.Error(ctx, "failed to get model meta for image billing: "+err.Error())
		return
	}
	quota := computeImageQuota(outputPriceCNY, groupRatio, n, sizeRatio)
	logContent := fmt.Sprintf("图片 ¥%.6f/张，尺寸系数 %.2f，分组倍率 %.2f（%d 张）",
		outputPriceCNY/1000000.0, sizeRatio, groupRatio, n)
	if err := PostCost(ctx, meta, preConsumedQuota, quota, 0, 0, 0, 0, logContent); err != nil {
		logger.Error(ctx, "error consuming image quota: "+err.Error())
	}
}

// PreConsumeTranscriptionQuota 按保底秒数预扣转写配额。
func PreConsumeTranscriptionQuota(ctx context.Context, meta *Meta) (int64, *ErrorWithStatusCode) {
	inputPriceCNY, groupRatio, err := audioPricing(meta)
	if err != nil {
		return 0, ErrorWrapper(err, "get_model_meta_failed", http.StatusInternalServerError)
	}
	return PreCost(ctx, meta, computeTranscriptionQuota(inputPriceCNY, groupRatio, preConsumedAudioSeconds))
}

// PostConsumeTranscriptionQuota 按上游返回的真实时长结算转写配额。
func PostConsumeTranscriptionQuota(ctx context.Context, meta *Meta, durationSeconds float64, preConsumedQuota int64) {
	inputPriceCNY, groupRatio, err := audioPricing(meta)
	if err != nil {
		logger.Error(ctx, "failed to get model meta for audio billing: "+err.Error())
		return
	}
	seconds := billedSeconds(durationSeconds)
	if seconds == 0 {
		logger.Error(ctx, fmt.Sprintf(
			"[转写计费] 模型 %s 未能从上游响应解析出音频时长，按 0 结算，需补适配",
			meta.ActualModelName))
	}
	quota := computeTranscriptionQuota(inputPriceCNY, groupRatio, seconds)
	logContent := fmt.Sprintf("音频 ¥%.6f/秒，分组倍率 %.2f（%d 秒，按秒向上取整）",
		inputPriceCNY/1000000.0, groupRatio, seconds)
	if err := PostCost(ctx, meta, preConsumedQuota, quota, 0, 0, 0, 0, logContent); err != nil {
		logger.Error(ctx, "error consuming audio quota: "+err.Error())
	}
}
```

- [ ] **Step 5: 运行测试确认通过**

Run: `go test ./internal/objects/ -run 'TestUnitQuotaRatio|TestComputeImageQuota|TestComputeTranscriptionQuota|TestBilledSeconds|TestUSDConversionAppliesToImage' -v`
Expected: 全部 PASS

- [ ] **Step 6: 全包编译**

Run: `go build ./...`
Expected: 无输出

- [ ] **Step 7: 提交**

```bash
git add internal/objects/billing_multimodal.go internal/objects/billing_multimodal_test.go internal/objects/billing.go
git commit -m "feat(billing): 增加按张 / 按秒的配额计算与结算入口"
```

---

### Task 3: TTS 字符计量修正

**Files:**
- Modify: `internal/objects/token.go:249-257`
- Test: `internal/objects/token_test.go`

**Interfaces:**
- Produces: `PredictAudioPromptTokenCount` 行为变更——TTS 分支返回字符数而非字节数

- [ ] **Step 1: 写失败的测试**

创建或追加 `internal/objects/token_test.go`：

```go
package objects

import (
	"testing"

	"github.com/zicorn/llm-proxy/internal/relay/relaymode"
)

func TestPredictAudioPromptTokenCountCountsRunes(t *testing.T) {
	// TTS 上游按字符计价。中文在 UTF-8 下每字 3 字节，按字节计会超收 3 倍。
	if got := PredictAudioPromptTokenCount("你好世界", relaymode.AudioSpeech); got != 4 {
		t.Errorf("中文 4 字得到 %d，期望 4（按字符而非字节）", got)
	}
	if got := PredictAudioPromptTokenCount("hello", relaymode.AudioSpeech); got != 5 {
		t.Errorf("英文 5 字母得到 %d，期望 5", got)
	}
	if got := PredictAudioPromptTokenCount("", relaymode.AudioSpeech); got != 0 {
		t.Errorf("空串得到 %d，期望 0", got)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/objects/ -run TestPredictAudioPromptTokenCountCountsRunes -v`
Expected: FAIL，`中文 4 字得到 12，期望 4`

- [ ] **Step 3: 改为按 rune 计数**

`internal/objects/token.go` 中把 TTS 分支改为：

```go
// PredictAudioPromptTokenCount 预测语音任务的计量单位数量
func PredictAudioPromptTokenCount(input string, relayMode int) int {
	switch relayMode {
	case relaymode.AudioSpeech:
		// TTS 上游按字符计价，必须数 rune。用 len() 会得到 UTF-8 字节数，
		// 中文每字 3 字节，会导致超收 3 倍。
		return utf8.RuneCountInString(input)
	default:
		return int(config.PreConsumedQuota)
	}
}
```

在文件的 import 块中加入 `"unicode/utf8"`。

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/objects/ -run TestPredictAudioPromptTokenCountCountsRunes -v`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add internal/objects/token.go internal/objects/token_test.go
git commit -m "fix(billing): TTS 按字符计量，修正中文超收 3 倍"
```

---

### Task 4: 图片按张计费

**Files:**
- Modify: `internal/relay/controller/image.go:172-211`

**Interfaces:**
- Consumes: Task 2 的 `objects.PreConsumeImageQuota` / `objects.PostConsumeImageQuota`；Task 1 的 `model.BillingUnitImage`

- [ ] **Step 1: 替换预扣与余额校验**

把 `internal/relay/controller/image.go` 中这一段（172-181 行）：

```go
	modelRatio := billingratio.GetModelRatio(imageModel, meta.ChannelType)
	groupRatio := billingratio.GetGroupRatio(meta.Group)
	ratio := modelRatio * groupRatio
	userQuota, err := model.CacheGetUserQuota(ctx, meta.UserId)

	quota := int64(ratio*imageCostRatio*1000) * int64(imageRequest.N)

	if userQuota-quota < 0 {
		return objects.ErrorWrapper(errors.New("user quota is not enough"), "insufficient_user_quota", http.StatusForbidden)
	}
```

替换为：

```go
	modelMeta, err := model.GetModelMetaByModel(meta.ActualModelName)
	if err != nil {
		return objects.ErrorWrapper(
			fmt.Errorf("模型 %s 未配置，请联系管理员在模型管理中添加", meta.ActualModelName),
			"model_not_configured", http.StatusBadRequest)
	}
	// 按 token 计价的图片模型（gpt-image-1 等）走响应中的 usage 结算，
	// 与按张计价的模型是两条路径，不能共用尺寸系数。
	billByImage := modelMeta.BillingUnit == model.BillingUnitImage
	if !billByImage && modelMeta.BillingUnit != model.BillingUnitToken {
		return objects.ErrorWrapper(
			fmt.Errorf("模型 %s 的计量单位为 %s，图片接口仅支持 image 或 token",
				meta.ActualModelName, modelMeta.BillingUnit),
			"billing_unit_mismatch", http.StatusBadRequest)
	}

	var preConsumedQuota int64
	if billByImage {
		var bizErr2 *objects.ErrorWithStatusCode
		preConsumedQuota, bizErr2 = objects.PreConsumeImageQuota(ctx, meta, imageRequest.N, imageCostRatio)
		if bizErr2 != nil {
			return bizErr2
		}
	}
```

- [ ] **Step 2: 替换结算块**

把 190-211 行的 `defer func(ctx context.Context) {...}(c.Request.Context())` 整块替换为：

```go
	defer func(ctx context.Context) {
		if resp != nil && resp.StatusCode != http.StatusOK {
			return
		}
		if !billByImage {
			// billing_unit=token 的图片模型由适配器解析 usage 后结算，此处不重复扣费
			return
		}
		go objects.PostConsumeImageQuota(ctx, meta, imageRequest.N, imageCostRatio, preConsumedQuota)
	}(c.Request.Context())
```

- [ ] **Step 3: 清理无用导入与变量**

`image.go` 中若 `billingratio` 仅剩 `ImageOriginModelName` 一处引用（133 行）则保留导入；`errors` 若不再被使用则删除该导入。执行 `go build ./...` 依据报错逐个清理。

- [ ] **Step 4: 编译验证**

Run: `go build ./...`
Expected: 无输出

- [ ] **Step 5: 确认零计费守卫已消失**

Run: `rg -n 'if quota != 0' internal/relay/controller/`
Expected: 无输出（该守卫是图片不写日志的直接原因）

- [ ] **Step 6: 提交**

```bash
git add internal/relay/controller/image.go
git commit -m "fix(billing): 图片按张计费改读 model_meta，消除恒零计费"
```

---

### Task 5: 转写请求补齐字段并注入 verbose_json

**Files:**
- Modify: `internal/relay/controller/audio.go:144-191`

**Interfaces:**
- Produces: 转发给上游的 multipart 包含 `response_format=verbose_json` 及客户端原始的 `language` / `prompt` / `temperature`；局部变量 `clientResponseFormat` 供 Task 6 使用

- [ ] **Step 1: 补齐转发字段**

`internal/relay/controller/audio.go` 中，把 `err = writer.WriteField("model", audioModel)` 那一段（170-174 行）替换为：

```go
		// 添加其他表单字段
		err = writer.WriteField("model", audioModel)
		if err != nil {
			return objects.ErrorWrapper(err, "write_field_failed", http.StatusInternalServerError)
		}

		// 固定以 verbose_json 请求上游：只有该格式的响应带 duration 字段，
		// 而按秒计费必须知道时长。客户端要的格式在响应阶段再转换。
		err = writer.WriteField("response_format", "verbose_json")
		if err != nil {
			return objects.ErrorWrapper(err, "write_field_failed", http.StatusInternalServerError)
		}

		// 这些字段此前被整体丢弃，导致 language 等参数失效、识别质量下降
		for _, field := range []string{"language", "prompt", "temperature"} {
			if v := c.PostForm(field); v != "" {
				if err := writer.WriteField(field, v); err != nil {
					return objects.ErrorWrapper(err, "write_field_failed", http.StatusInternalServerError)
				}
			}
		}
```

- [ ] **Step 2: 记录客户端要求的格式**

把 191 行 `responseFormat := c.DefaultPostForm("response_format", "json")` 改为：

```go
	// 客户端要求的格式，与发给上游的 verbose_json 无关
	clientResponseFormat := c.DefaultPostForm("response_format", "json")
```

同时把下游 `switch responseFormat` 改为 `switch clientResponseFormat`，使本任务独立可编译。该 switch 整块会在 Task 6 中被替换掉，此处只做重命名，不改语义。

- [ ] **Step 3: 编译验证**

Run: `go build ./...`
Expected: 无输出

- [ ] **Step 4: 手工确认字段已写入**

在 `writer.Close()` 之前临时插入 `logger.Debugf(ctx, "upstream multipart fields: %s", requestBody.String()[:200])`，以 `DEBUG=true` 启动并发一次转写请求，确认日志中出现 `response_format` 与 `verbose_json`。确认后删除该临时日志。

- [ ] **Step 5: 提交**

```bash
git add internal/relay/controller/audio.go
git commit -m "fix(audio): 转写补齐被丢弃的转发字段并固定请求 verbose_json"
```

---

### Task 6: 转写响应格式转换与按时长计费

**Files:**
- Create: `internal/relay/controller/audio_format.go`
- Create: `internal/relay/controller/audio_format_test.go`
- Modify: `internal/relay/controller/audio.go:57-66, 236-280`

**Interfaces:**
- Consumes: Task 2 的 `objects.PreConsumeTranscriptionQuota` / `PostConsumeTranscriptionQuota`；Task 5 的 `clientResponseFormat`
- Produces: `convertVerboseJSON(resp *openai.WhisperVerboseJSONResponse, format string) (body string, contentType string, err error)`

- [ ] **Step 1: 写失败的测试**

创建 `internal/relay/controller/audio_format_test.go`：

```go
package controller

import (
	"strings"
	"testing"

	"github.com/zicorn/llm-proxy/internal/relay/adaptor/openai"
)

func sampleVerbose() *openai.WhisperVerboseJSONResponse {
	return &openai.WhisperVerboseJSONResponse{
		Task:     "transcribe",
		Language: "chinese",
		Duration: 3.5,
		Text:     "你好 世界",
		Segments: []openai.Segment{
			{Id: 0, Start: 0, End: 1.5, Text: " 你好"},
			{Id: 1, Start: 1.5, End: 3.5, Text: " 世界"},
		},
	}
}

func TestConvertVerboseJSONToText(t *testing.T) {
	body, ct, err := convertVerboseJSON(sampleVerbose(), "text")
	if err != nil {
		t.Fatalf("意外错误: %v", err)
	}
	if body != "你好 世界" {
		t.Errorf("text 格式得到 %q，期望 %q", body, "你好 世界")
	}
	if !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("Content-Type = %q，期望 text/plain", ct)
	}
}

func TestConvertVerboseJSONToSRT(t *testing.T) {
	body, ct, err := convertVerboseJSON(sampleVerbose(), "srt")
	if err != nil {
		t.Fatalf("意外错误: %v", err)
	}
	want := "1\n00:00:00,000 --> 00:00:01,500\n你好\n\n2\n00:00:01,500 --> 00:00:03,500\n世界\n\n"
	if body != want {
		t.Errorf("srt 格式得到:\n%q\n期望:\n%q", body, want)
	}
	if !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("Content-Type = %q，期望 text/plain", ct)
	}
}

func TestConvertVerboseJSONToVTT(t *testing.T) {
	body, _, err := convertVerboseJSON(sampleVerbose(), "vtt")
	if err != nil {
		t.Fatalf("意外错误: %v", err)
	}
	if !strings.HasPrefix(body, "WEBVTT\n\n") {
		t.Errorf("vtt 缺少 WEBVTT 头: %q", body)
	}
	if !strings.Contains(body, "00:00:01.500 --> 00:00:03.500") {
		t.Errorf("vtt 时间戳格式错误（应用点号分隔毫秒）: %q", body)
	}
}

func TestConvertVerboseJSONToJSON(t *testing.T) {
	body, ct, err := convertVerboseJSON(sampleVerbose(), "json")
	if err != nil {
		t.Fatalf("意外错误: %v", err)
	}
	if body != `{"text":"你好 世界"}` {
		t.Errorf("json 格式得到 %q", body)
	}
	if !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q，期望 application/json", ct)
	}
}

func TestConvertVerboseJSONPassthrough(t *testing.T) {
	body, ct, err := convertVerboseJSON(sampleVerbose(), "verbose_json")
	if err != nil {
		t.Fatalf("意外错误: %v", err)
	}
	if !strings.Contains(body, `"duration":3.5`) {
		t.Errorf("verbose_json 应保留 duration: %q", body)
	}
	if !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q，期望 application/json", ct)
	}
}

func TestFormatTimestamp(t *testing.T) {
	cases := []struct {
		in       float64
		sep      string
		expected string
	}{
		{0, ",", "00:00:00,000"},
		{1.5, ",", "00:00:01,500"},
		{61.25, ".", "00:01:01.250"},
		{3661.001, ".", "01:01:01.001"},
		{1.9995, ",", "00:00:02,000"},
	}
	for _, c := range cases {
		if got := formatTimestamp(c.in, c.sep); got != c.expected {
			t.Errorf("formatTimestamp(%v, %q) = %q，期望 %q", c.in, c.sep, got, c.expected)
		}
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/relay/controller/ -run 'TestConvertVerboseJSON|TestFormatTimestamp' -v`
Expected: FAIL，编译错误 `undefined: convertVerboseJSON`

- [ ] **Step 3: 实现格式转换**

创建 `internal/relay/controller/audio_format.go`：

```go
package controller

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/zicorn/llm-proxy/internal/relay/adaptor/openai"
)

const (
	contentTypeJSON = "application/json; charset=utf-8"
	contentTypeText = "text/plain; charset=utf-8"
)

// formatTimestamp 把秒数格式化为字幕时间戳。sep 为毫秒分隔符：SRT 用逗号，WebVTT 用点号。
//
// 先整体换算成毫秒再拆分，避免"秒取整 + 单独算毫秒"在 1.9995 这类值上得到
// 00:00:01,1000 的越界结果。
func formatTimestamp(seconds float64, sep string) string {
	if seconds < 0 {
		seconds = 0
	}
	totalMs := int64(math.Round(seconds * 1000))
	h := totalMs / 3600000
	m := (totalMs % 3600000) / 60000
	s := (totalMs % 60000) / 1000
	ms := totalMs % 1000
	return fmt.Sprintf("%02d:%02d:%02d%s%03d", h, m, s, sep, ms)
}

// convertVerboseJSON 把上游的 verbose_json 响应转换为客户端要求的格式。
//
// 代理固定以 verbose_json 请求上游以获得 duration 用于计费，因此必须在这里
// 降级回客户端原本要求的格式，否则客户端会收到与其请求不符的响应体。
func convertVerboseJSON(resp *openai.WhisperVerboseJSONResponse, format string) (string, string, error) {
	switch format {
	case "text":
		return resp.Text, contentTypeText, nil

	case "srt":
		var b strings.Builder
		for i, seg := range resp.Segments {
			fmt.Fprintf(&b, "%d\n%s --> %s\n%s\n\n",
				i+1,
				formatTimestamp(seg.Start, ","),
				formatTimestamp(seg.End, ","),
				strings.TrimSpace(seg.Text))
		}
		return b.String(), contentTypeText, nil

	case "vtt":
		var b strings.Builder
		b.WriteString("WEBVTT\n\n")
		for _, seg := range resp.Segments {
			fmt.Fprintf(&b, "%s --> %s\n%s\n\n",
				formatTimestamp(seg.Start, "."),
				formatTimestamp(seg.End, "."),
				strings.TrimSpace(seg.Text))
		}
		return b.String(), contentTypeText, nil

	case "verbose_json":
		payload, err := json.Marshal(resp)
		if err != nil {
			return "", "", fmt.Errorf("marshal verbose_json failed: %w", err)
		}
		return string(payload), contentTypeJSON, nil

	default: // json
		payload, err := json.Marshal(openai.WhisperJSONResponse{Text: resp.Text})
		if err != nil {
			return "", "", fmt.Errorf("marshal json failed: %w", err)
		}
		return string(payload), contentTypeJSON, nil
	}
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/relay/controller/ -run 'TestConvertVerboseJSON|TestFormatTimestamp' -v`
Expected: 全部 PASS

- [ ] **Step 5: 改造 audio.go 的预扣**

把 `audio.go` 57-66 行这一段：

```go
	modelRatio := billingratio.GetModelRatio(audioModel, channelType)
	groupRatio := billingratio.GetGroupRatio(group)
	ratio := modelRatio * groupRatio
	var quota, preConsumedQuota int64
	switch relayMode {
	case relaymode.AudioSpeech:
		preConsumedQuota = int64(float64(len(ttsRequest.Input)) * ratio)
		quota = preConsumedQuota
	default:
		preConsumedQuota = int64(float64(config.PreConsumedQuota) * ratio)
	}
```

替换为（本函数只服务转写与翻译，TTS 走 `RelayAudioSpeechHelper`）：

```go
	var preConsumedQuota int64
	{
		var bizErr *objects.ErrorWithStatusCode
		preConsumedQuota, bizErr = objects.PreConsumeTranscriptionQuota(ctx, meta)
		if bizErr != nil {
			return bizErr
		}
	}
```

随后删除紧跟其后的手工余额校验与 `CacheDecreaseUserQuota` / `PreConsumeTokenQuota` 段落（68-91 行），因为 `PreConsumeTranscriptionQuota` 内部的 `PreCost` 已经完成这些动作。回滚用的 `succeed` 与 `defer` 保留。

- [ ] **Step 6: 改造响应解析**

注意作用域：`responseBody` 是在 `if relayMode != relaymode.AudioSpeech {` 块内用 `:=` 声明的，只在该块内可见；而 260 行的 `resp.Body = io.NopCloser(...)` 是块外的 `RelayErrorHandler` 与函数尾部 `io.Copy(c.Writer, resp.Body)` 共同依赖的。因此转换结果要写回 `resp.Body`，尾部拷贝逻辑不必改动。

先在 `preConsumedQuota` 声明处（Step 5 改过的位置）追加三个块外变量：

```go
	var audioDuration float64
	var convertedBody, contentType string
```

然后把块内 241-260 行（`var text string` 的 switch 直到 `resp.Body = io.NopCloser(bytes.NewBuffer(responseBody))`）整段替换为：

```go
		// 上游固定按 verbose_json 返回：先取 duration 用于按秒计费，
		// 再降级成客户端原本请求的格式写回 resp.Body，供尾部统一拷贝。
		var verbose openai.WhisperVerboseJSONResponse
		if err = json.Unmarshal(responseBody, &verbose); err != nil {
			return objects.ErrorWrapper(err, "unmarshal_verbose_json_failed", http.StatusInternalServerError)
		}
		audioDuration = verbose.Duration

		convertedBody, contentType, err = convertVerboseJSON(&verbose, clientResponseFormat)
		if err != nil {
			return objects.ErrorWrapper(err, "convert_response_format_failed", http.StatusInternalServerError)
		}
		resp.Body = io.NopCloser(bytes.NewBufferString(convertedBody))
```

- [ ] **Step 7: 改造结算与响应头**

把块外 266-273 行（`quotaDelta := quota - preConsumedQuota` 到 `c.Writer.WriteHeader(resp.StatusCode)`）替换为：

```go
	defer func(ctx context.Context) {
		go objects.PostConsumeTranscriptionQuota(ctx, meta, audioDuration, preConsumedQuota)
	}(c.Request.Context())

	for k, v := range resp.Header {
		c.Writer.Header().Set(k, v[0])
	}
	// 上游返回的是 verbose_json 的类型与长度，必须按转换后的实际内容覆盖
	if contentType != "" {
		c.Writer.Header().Set("Content-Type", contentType)
		c.Writer.Header().Set("Content-Length", strconv.Itoa(len(convertedBody)))
	}
	c.Writer.WriteHeader(resp.StatusCode)
```

函数尾部的 `io.Copy(c.Writer, resp.Body)` 与 `resp.Body.Close()` 保持不变——此时 `resp.Body` 已是转换后的内容。

import 中加入 `"strconv"`，并删除不再使用的 `billing`、`billingratio`、`bufio`、`config` 等（依 `go build` 报错逐个清理）。

- [ ] **Step 8: 删除 TTS 死代码**

`RelayAudioHelper` 只由 `relaymode.AudioTranslation` 与 `AudioTranscription` 触发（见 `internal/handler/relay.go:29-44`，`AudioSpeech` 走 `RelayAudioSpeechHelper`），因此 42-55 行解析 `ttsRequest` 的分支与 `if relayMode != relaymode.AudioSpeech` 的条件都恒定成立/不成立。删除 `ttsRequest` 解析块与 `openai.TextToSpeechRequest` 变量声明，把 `if relayMode != relaymode.AudioSpeech {` 的条件包裹去掉（保留块内语句，注意调整缩进）。

保留 `succeed` 标志与其 `defer` 回滚逻辑不变。

- [ ] **Step 9: 删除失效的旧解析函数**

删除 `getTextFromJSON`、`getTextFromText`、`getTextFromSRT`、`getTextFromVTT`、`getTextFromVerboseJSON` 五个函数——`getTextFromSRT` 正是"`response_format=srt` 免费转写"的成因（解析 JSON 得到空串且不报错），它们已被 `convertVerboseJSON` 取代。

- [ ] **Step 10: 编译与全量测试**

Run: `go build ./... && go test ./internal/relay/controller/ ./internal/objects/ -v`
Expected: 编译无输出，测试全部 PASS

- [ ] **Step 11: 提交**

```bash
git add internal/relay/controller/audio.go internal/relay/controller/audio_format.go internal/relay/controller/audio_format_test.go
git commit -m "fix(audio): 转写按时长计费并按客户端格式转换响应"
```

---

### Task 7: 微调改读 model_meta

**Files:**
- Modify: `internal/objects/training_file.go:28-30`
- Modify: `internal/handler/fineTuning.go:48-49`
- Test: `internal/objects/billing_multimodal_test.go`（追加）

**Interfaces:**
- Produces: `(*TrainingFile).GetPreConsumedQuota() (int64, error)`（签名变更：增加 error 返回值）

- [ ] **Step 1: 写失败的测试**

在 `internal/objects/billing_multimodal_test.go` 追加：

```go
func TestComputeFineTuningQuota(t *testing.T) {
	// ¥3/1M token，3 epochs × 100000 tokens = 300000 训练 token => 900000 额度
	if q := computeFineTuningQuota(3, 1.0, 3, 100000); q != 900000 {
		t.Errorf("computeFineTuningQuota = %d, 期望 900000", q)
	}
	// 单价为 0 时不收费
	if q := computeFineTuningQuota(0, 1.0, 3, 100000); q != 0 {
		t.Errorf("单价为 0 时 = %d, 期望 0", q)
	}
	// 绝不返回负值——旧实现因 ratio 兜底为 -1 会给用户加余额
	if q := computeFineTuningQuota(3, 1.0, 0, 0); q < 0 {
		t.Errorf("配额为负: %d", q)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/objects/ -run TestComputeFineTuningQuota -v`
Expected: FAIL，`undefined: computeFineTuningQuota`

- [ ] **Step 3: 实现并改造**

在 `internal/objects/billing_multimodal.go` 追加：

```go
// computeFineTuningQuota 计算微调配额。训练按 token 计量，epochs × tokens 为总训练量。
func computeFineTuningQuota(inputPriceCNY, groupRatio float64, epochs, tokens int) int64 {
	ratio := unitQuotaRatio(inputPriceCNY, groupRatio)
	total := float64(epochs) * float64(tokens)
	if total <= 0 {
		return 0
	}
	quota := int64(math.Ceil(total * ratio))
	if ratio != 0 && quota <= 0 {
		quota = 1
	}
	return quota
}
```

把 `internal/objects/training_file.go` 的 `GetPreConsumedQuota` 改为：

```go
// GetPreConsumedQuota 计算微调任务的预扣配额。
//
// 旧实现乘的是 ratio.GetFineTuningRatio，该表为空时返回 -1，导致配额为负、
// 预扣变成给用户充值。现改为与其他模态一致地读 model_meta。
func (file *TrainingFile) GetPreConsumedQuota(meta *Meta) (int64, error) {
	modelMeta, err := model.GetModelMetaByModel(file.ModelName)
	if err != nil {
		return 0, fmt.Errorf("模型 %s 未配置，请联系管理员在模型管理中添加", file.ModelName)
	}
	inputPriceCNY, _, _ := getModelPricesInCNY(modelMeta)
	groupRatio := billingratio.GetGroupRatio(meta.Group)
	return computeFineTuningQuota(inputPriceCNY, groupRatio, file.Epochs, file.Tokens), nil
}
```

该文件的 import 改为：

```go
import (
	"fmt"

	billingratio "github.com/zicorn/llm-proxy/internal/relay/billing/ratio"
	model "github.com/zicorn/llm-proxy/internal/repo"
)
```

- [ ] **Step 4: 接住错误返回值**

`internal/handler/fineTuning.go` 中把这两行：

```go
	preConsumedQuota := trainingFile.GetPreConsumedQuota()
	objects.PreCost(ctx, meta, preConsumedQuota)
```

替换为：

```go
	preConsumedQuota, err := trainingFile.GetPreConsumedQuota(meta)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}
	// 此前该返回值被丢弃，余额不足也会继续转发
	if _, bizErr := objects.PreCost(ctx, meta, preConsumedQuota); bizErr != nil {
		c.JSON(bizErr.StatusCode, gin.H{
			"error": bizErr.Message,
		})
		return
	}
```

若 `err` 与上文已有变量冲突，改用 `quotaErr` 等新名。

- [ ] **Step 5: 运行测试确认通过**

Run: `go test ./internal/objects/ -run TestComputeFineTuningQuota -v && go build ./...`
Expected: PASS，编译无输出

- [ ] **Step 6: 提交**

```bash
git add internal/objects/training_file.go internal/objects/billing_multimodal.go internal/objects/billing_multimodal_test.go internal/handler/fineTuning.go
git commit -m "fix(billing): 微调改读 model_meta，消除负配额反向充值"
```

---

### Task 8: 删除旧计费实现

**Files:**
- Delete: `internal/relay/billing/ratio/model.go`
- Modify: `internal/relay/billing/billing.go`（删除 `PostConsumeQuota`）

**Interfaces:**
- Consumes: Task 4、6、7 必须全部完成，否则仍有调用方

- [ ] **Step 1: 确认已无调用方**

Run:
```bash
rg -n 'GetModelRatio|GetFineTuningRatio|billing\.PostConsumeQuota' --glob '!docs/**'
```
Expected: 仅匹配到定义处本身，无任何调用点。若仍有调用，回到对应 Task 处理。

- [ ] **Step 2: 删除**

```bash
git rm internal/relay/billing/ratio/model.go
```

然后从 `internal/relay/billing/billing.go` 中删除 `PostConsumeQuota` 函数。若删除后该文件已无任何声明，一并 `git rm` 掉；若其中还有其他在用函数则保留文件。

- [ ] **Step 3: 编译验证**

Run: `go build ./...`
Expected: 无输出。若报 `ratio.ModelRatio` 未定义，说明还有未迁移的引用，逐个排查。

- [ ] **Step 4: 验收——全仓只剩一处消费日志写入点**

Run: `rg -n 'RecordConsumeLog\(' --glob '!docs/**' --glob '!*_test.go'`
Expected: 恰好两处——`internal/repo/log.go` 中的定义，以及 `internal/objects/billing.go` 的 `PostCost` 中的唯一调用点。

- [ ] **Step 5: 全量测试**

Run: `go test ./internal/... 2>&1 | rg -v '^ok'`
Expected: 无 FAIL。注意 `pkg/common/image` 的测试依赖外网下载图片，本地无网时会失败，属既有问题，与本次改动无关。

- [ ] **Step 6: 提交**

```bash
git add -A internal/relay/billing/
git commit -m "refactor(billing): 删除迁移前遗留的空费率表与旧结算实现"
```

---

### Task 9: 前端计量单位支持

**Files:**
- Modify: `web/src/views/ModelMeta/type/Config.js`
- Modify: `web/src/views/ModelMeta/component/EditModal.js`
- Modify: `web/src/views/ModelMeta/component/TableHead.js`
- Modify: `web/src/views/ModelMeta/component/TableRow.js`
- Modify: `web/src/views/ModelMeta/component/BatchModal.js`

**Interfaces:**
- Consumes: Task 1 的 `billing_unit` 字段与四个枚举值

- [ ] **Step 1: 扩展默认配置**

`web/src/views/ModelMeta/type/Config.js` 全文替换为：

```javascript
const BILLING_UNITS = [
  { value: 'token', label: 'token（文本 / 按 token 计价的图片模型）', priceUnit: '百万 token' },
  { value: 'char', label: 'char 字符（TTS 语音合成）', priceUnit: '百万字符' },
  { value: 'second', label: 'second 秒（语音转写 / 翻译）', priceUnit: '百万秒' },
  { value: 'image', label: 'image 张（按张计价的图片模型）', priceUnit: '张' }
];

const defaultConfig = {
  input: {
    channel_type: 1,
    model: '',
    status: 1,
    input_price: 0.0,
    output_price: 0.0,
    cache_price: 0.0,
    price_unit: 'CNY',
    billing_unit: 'token'
  },
  inputLabel: {
    channel_type: '渠道类型',
    model: '模型名称',
    status: '启用',
    input_price: '输入价格',
    output_price: '输出价格',
    cache_price: '缓存价格',
    price_unit: '价格单位',
    billing_unit: '计量单位'
  },
  prompt: {
    channel_type: '请选择渠道类型',
    model: '请添加模型名称',
    status: '是否启用',
    input_price: '',
    output_price: '',
    cache_price: '0 = 未配置（缓存 token 按输入价格收费）；> 0 才启用缓存折扣',
    price_unit: '选择价格货币单位',
    billing_unit: '决定价格字段中"每百万"的单位是什么'
  }
};

export { defaultConfig, BILLING_UNITS };
```

- [ ] **Step 2: 表单增加单位下拉与换算**

`EditModal.js` 中：

导入处加入 `BILLING_UNITS`：

```javascript
import { defaultConfig, BILLING_UNITS } from '../type/Config';
```

校验 schema 增加一行：

```javascript
  billing_unit: Yup.string().oneOf(['token', 'char', 'second', 'image'], '请选择有效的计量单位'),
```

在 `const EditModal = ({...}) => {` 内部、`return` 之前加入三个辅助函数。

换算方向以"每张 ¥0.3 对应存储值 300000"为准：**显示值 = 存储值 ÷ 1e6**，**存储值 = 显示值 × 1e6**。

```javascript
  // billing_unit=image 时后端存的是"每百万张"的价格，界面按"每张"展示，
  // 避免管理员为 ¥0.3/张 填写 300000 这种反直觉的数字
  const toDisplayPrice = (value, unit) => (unit === 'image' ? (Number(value) || 0) / 1000000 : Number(value) || 0);
  const toStoredPrice = (value, unit) => (unit === 'image' ? (Number(value) || 0) * 1000000 : Number(value) || 0);

  const priceSuffix = (unit) => {
    const found = BILLING_UNITS.find((u) => u.value === unit);
    return found ? found.priceUnit : '百万 token';
  };
```

`submit` 中的三行 `parseFloat` 替换为：

```javascript
    const unit = values.billing_unit || 'token';
    values.input_price = toStoredPrice(values.input_price, unit);
    values.output_price = toStoredPrice(values.output_price, unit);
    values.cache_price = toStoredPrice(values.cache_price, unit);
```

`loadModelMeta` 中 `setInitialInput(data)` 替换为：

```javascript
        const unit = data.billing_unit || 'token';
        setInitialInput({
          ...data,
          billing_unit: unit,
          input_price: toDisplayPrice(data.input_price, unit),
          output_price: toDisplayPrice(data.output_price, unit),
          cache_price: toDisplayPrice(data.cache_price, unit)
        });
```

在「价格单位」那个 `FormControl` 之前插入计量单位下拉：

```javascript
              {/*计量单位*/}
              <FormControl fullWidth error={Boolean(touched.billing_unit && errors.billing_unit)}
                           sx={{ ...theme.typography.otherInput }}>
                <InputLabel htmlFor="billing-unit-label">{inputLabel.billing_unit}</InputLabel>
                <Select
                  id="billing-unit-label"
                  label={inputLabel.billing_unit}
                  value={values.billing_unit || 'token'}
                  name="billing_unit"
                  onBlur={handleBlur}
                  onChange={handleChange}
                >
                  {BILLING_UNITS.map((u) => (
                    <MenuItem key={u.value} value={u.value}>
                      {u.label}
                    </MenuItem>
                  ))}
                </Select>
                <FormHelperText id="helper-tex-billing-unit-label">{inputPrompt.billing_unit}</FormHelperText>
              </FormControl>
```

三个价格字段的 `<InputLabel>` 与 `label` 属性改为带单位后缀，例如输入价格：

```javascript
                  <InputLabel htmlFor="input-price-label">
                    {`${inputLabel.input_price}（每${priceSuffix(values.billing_unit)}）`}
                  </InputLabel>
                  <OutlinedInput
                    id="input-price-label"
                    label={`${inputLabel.input_price}（每${priceSuffix(values.billing_unit)}）`}
```

`output_price` 与 `cache_price` 同法处理。另在 `billing_unit === 'image'` 时给输入价格与缓存价格加一句提示：

```javascript
                    <FormHelperText>
                      {values.billing_unit === 'image'
                        ? '按张计价时该字段不参与计算'
                        : inputPrompt.input_price}
                    </FormHelperText>
```

- [ ] **Step 3: 列表增加单位列并修正显示错误**

`TableHead.js` 中在「缓存价格」与「单位」之间插入：

```javascript
        <TableCell>计量单位</TableCell>
```

`TableRow.js` 中在 `cache_price` 那个 `TableCell` 之后插入：

```javascript
        <TableCell>{item.billing_unit || 'token'}</TableCell>
```

并把文件底部的 `renderPrice` 替换为（原实现把每百万的价格标成 `/1K`）：

```javascript
function renderPrice(price, priceUnit, billingUnit) {
  const symbol = priceUnit === 'USD' ? '$' : '¥';
  const val = typeof price === 'number' ? price : 0;
  // 按张计价时存储的是每百万张，换算回每张展示
  if (billingUnit === 'image') {
    return (
      <span>
        {symbol}
        {(val / 1000000).toFixed(6)}/张
      </span>
    );
  }
  const suffix = billingUnit === 'char' ? '/1M字符' : billingUnit === 'second' ? '/1M秒' : '/1M token';
  return (
    <span>
      {symbol}
      {val.toFixed(4)}
      {suffix}
    </span>
  );
}
```

三处调用同步改为传入第三个参数，例如：

```javascript
        <TableCell>{renderPrice(item.input_price, item.price_unit, item.billing_unit)}</TableCell>
```

- [ ] **Step 4: 批量导入表头增加字段**

`BatchModal.js` 中把：

```javascript
        values.content = 'model|channel_type|input_price|output_price|cache_price|price_unit\n' + values.content;
```

改为：

```javascript
        values.content =
          'model|channel_type|input_price|output_price|cache_price|price_unit|billing_unit\n' + values.content;
```

- [ ] **Step 5: 构建验证**

Run: `cd web && npm run build`
Expected: 构建成功，产物输出到 `internal/webstatic/build/`。无 ESLint 报错。

- [ ] **Step 6: 提交**

```bash
git add web/src/views/ModelMeta/
git commit -m "feat(web): 模型管理支持计量单位，修正价格单位显示错误"
```

---

## 端到端验证

在全部 Task 完成后执行，不单独提交代码。

- [ ] **Step 1: 起 mock 上游**

写一个临时 Python 脚本 `scripts/mock_multimodal_upstream.py`，监听 880 端口：
- `POST /v1/images/generations` 返回 `{"created":1,"data":[{"url":"http://example.com/a.png"}]}`
- `POST /v1/audio/transcriptions` 返回 `{"task":"transcribe","language":"chinese","duration":12.3,"text":"你好世界","segments":[{"id":0,"start":0,"end":12.3,"text":"你好世界"}]}`

以 `block_until_ms: 0` 后台启动，用 curl 探测就绪。

- [ ] **Step 2: 配置渠道与模型价格**

通过管理 API 建一个 OpenAI 类型渠道指向 `http://127.0.0.1:880`，并配置两个模型：
- `dall-e-3`：`billing_unit=image`，`output_price=300000`（即每张 ¥0.3）
- `whisper-1`：`billing_unit=second`，`input_price=720`（即每秒 ¥0.00072）

- [ ] **Step 3: 验证图片计费**

发一次 `n=1`、`size=1024x1024` 的图片请求，查 `logs` 表确认：
- 产生了一条消费记录
- `quota` 为 300000（¥0.3，尺寸系数 1.0）

再发一次 `size=1024x1792`（dall-e-3 系数 2.0），确认 `quota` 为 600000。

- [ ] **Step 4: 验证转写计费与格式**

发 `response_format=json` 的转写请求，确认 `quota` 为 `ceil(12.3) × 720 = 9360`。

分别发 `response_format=srt`、`vtt`、`text`、`verbose_json`，确认：
- 四次都产生了相同的 `quota`（9360），不再有 srt 免费的情况
- 响应体格式与请求的格式一致（srt 有 `-->` 时间轴，vtt 以 `WEBVTT` 开头，text 是纯文本）

- [ ] **Step 5: 清理**

删除 mock 脚本，确认 `git status` 干净，杀掉后台进程。

---

## Self-Review 记录

**Spec 覆盖检查：** spec 的每一节都有对应任务——`billing_unit` 字段（Task 1）、结算出口 `PostCost`（Task 2）、预扣策略（Task 2 的 `preConsumedAudioSeconds` 与 Task 4 的精确预扣）、图片（Task 4）、转写（Task 5 + 6）、TTS（Task 3）、微调（Task 7）、旧实现清理（Task 8）、前端 6 个文件（Task 9）、测试策略（各任务的 TDD 步骤 + 端到端）。

**类型一致性：** `GetPreConsumedQuota` 在 Task 7 中签名由 `() int64` 变为 `(meta *Meta) (int64, error)`，调用方 `handler/fineTuning.go` 在同一任务内同步修改。`unitQuotaRatio` 在 Task 2 完成重命名后，Task 4/6/7 均使用新名。`convertVerboseJSON` 在 Task 6 定义并在同任务内被 `audio.go` 使用。

**已知顺序约束：** Task 4、6、7 依赖 Task 1 与 Task 2；Task 6 依赖 Task 5（`clientResponseFormat` 变量与上游 `verbose_json` 注入）；Task 8 依赖 4、6、7 全部完成；Task 9 依赖 Task 1。Task 3 无前置依赖，可提前执行。

**Review 中修正的问题：**

1. Task 6 原先把响应转换写成"直接 `c.Writer.WriteString` 并删除尾部 `io.Copy`"，但 `responseBody` 的作用域限于 `if relayMode != relaymode.AudioSpeech` 块内，且块外的 `RelayErrorHandler(resp)` 与尾部 `io.Copy(c.Writer, resp.Body)` 都依赖 260 行对 `resp.Body` 的重置。改为把转换结果写回 `resp.Body`，尾部拷贝逻辑不动，并拆成 Step 6（块内解析）与 Step 7（块外结算与响应头）。
2. Task 9 原先给出了两份方向相反的 `toDisplayPrice`，会直接误导实现者。已明确为"显示值 = 存储值 ÷ 1e6"。
3. Task 5 原先对是否同步重命名 `switch responseFormat` 留了模糊表述，导致该任务可能无法独立编译。已改为必做。
4. Task 2 的测试文件缺少 `model` 包导入。已补。
