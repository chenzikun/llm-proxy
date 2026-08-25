# 价格计算问题分析

## 🔍 问题描述

用户反馈：使用公式 `price = input_price * input_token / 1000000 + output_price * output_token / 1000000` 计算的价格，与实际记录的价格存在差异（约为两倍）。

---

## 📊 当前计费逻辑

### 1. 基础单位定义

```go
// relay/billing/ratio/model.go
const (
    USD = 500  // $0.002 = 1 quota -> $1 = 500 quota
)
```

**含义**:
- **1 quota = $0.002**
- **500 quota = $1**

---

### 2. 模型费率定义（以 gpt-4o 为例）

#### 方式 A: `ratio/model.go` （旧）

```go
// relay/billing/ratio/model.go
var ModelRatio = map[string]float64{
    "gpt-4o": 1.25,  // $0.0025 / 1K tokens
}

// GetCompletionRatio 默认返回 3 (如果没有特殊配置)
func GetCompletionRatio(name string, channelType int) float64 {
    if strings.HasPrefix(name, "gpt-4o") {
        return 3  // ← 相对于 ModelRatio 的倍数
    }
    return 1
}
```

**计算**:
- Input: 1K tokens × 1.25 = 1.25 quota = $0.0025 ✅
- Output: 1K tokens × 1.25 × 3 = 3.75 quota = $0.0075 ❌

**OpenAI 官方价格**:
- Input: $0.0025 / 1K tokens
- Output: $0.01 / 1K tokens

**问题**: Output 应该是 $0.01，但实际只有 $0.0075！

---

#### 方式 B: `model/models.go` （新）

```go
// model/models.go
var ModelMetaMap = map[string]ModelMeta{
    "gpt-4o": {
        Model: "gpt-4o",
        ChannelType: 1,
        ModelRatio: 1.25,       // Input 价格（quota）
        CompletionRatio: 3.75,  // Output 价格（quota，绝对值）
    },
}
```

**当前计算方式** (`objects/billing.go`):
```go
func PostConsumeQuota(ctx context.Context, usage *entity.Usage, meta *Meta, preConsumedQuota int64) {
    modelMeta, err := model.GetModelMetaByModel(meta.ActualModelName)
    modelRatio := modelMeta.ModelRatio          // 1.25
    completionRatio := modelMeta.CompletionRatio // 3.75 ← 这是绝对值！
    groupRatio := billingratio.GetGroupRatio(meta.Group)  // 1.0
    
    // 关键公式
    quota = int64(math.Ceil(
        float64(promptTokens) * modelRatio * groupRatio +      // 方式 1
        float64(completionTokens) * completionRatio * groupRatio  // 方式 2：直接用绝对值
    ))
}
```

---

## 🚨 核心问题

### 问题 1: completionRatio 的理解混乱

代码中存在两种理解：

| 方式 | completionRatio 含义 | Input 计算 | Output 计算 |
|------|---------------------|-----------|------------|
| **旧代码** | 相对于 ModelRatio 的倍数 | tokens × modelRatio | tokens × modelRatio × completionRatio |
| **新代码** | 绝对quota值 | tokens × modelRatio | tokens × completionRatio |

**当前实际使用**: 新代码（绝对值）

---

### 问题 2: gpt-4o 的 CompletionRatio 不正确

**当前值**: 3.75 quota = $0.0075 / 1K tokens

**正确值**: 5.0 quota = $0.01 / 1K tokens

**计算**:
```
OpenAI Output Price = $0.01 / 1K tokens
正确的 CompletionRatio = $0.01 / $0.002 = 5.0
```

---

## 🧮 实际案例验证

### 假设场景
- 模型: gpt-4o
- Prompt tokens: 100
- Completion tokens: 200
- Group ratio: 1.0

### 当前计算（错误）

```go
modelRatio = 1.25
completionRatio = 3.75  // ← 错误！应该是 5.0
groupRatio = 1.0

// Prompt quota
promptQuota = 100 × 1.25 × 1.0 = 125 quota = $0.25

// Completion quota
completionQuota = 200 × 3.75 × 1.0 = 750 quota = $1.50  // ← 错误！

// 总计
totalQuota = 125 + 750 = 875 quota = $1.75  // ← 错误！
```

### 正确计算

```go
modelRatio = 1.25
completionRatio = 5.0  // ← 正确！
groupRatio = 1.0

// Prompt quota
promptQuota = 100 × 1.25 × 1.0 = 125 quota = $0.25

// Completion quota
completionQuota = 200 × 5.0 × 1.0 = 1000 quota = $2.00  // ← 正确！

// 总计
totalQuota = 125 + 1000 = 1125 quota = $2.25  // ← 正确！
```

### OpenAI 官方价格验证

```
Input: 100 tokens × $0.0025 / 1K tokens = $0.25 ✅
Output: 200 tokens × $0.01 / 1K tokens = $2.00 ✅
Total: $0.25 + $2.00 = $2.25 ✅
```

**误差**: 
- 当前计算: $1.75
- 正确价格: $2.25
- 差额: $0.50
- **少收了 22%！**

---

## 🔍 为什么看起来是"两倍"？

用户提到的公式：
```
price = input_price * input_token / 1000000 + output_price * output_token / 1000000
```

这个 `/1000000` 可能有以下几种可能：

### 可能性 1: 单位转换（最可能）

如果用户从某处获取的价格单位是"每百万 tokens"：

```python
# 如果 input_price 单位是 $/M tokens
input_price_per_m = 2.50   # $2.50 / M tokens = $0.0025 / 1K tokens
output_price_per_m = 10.00  # $10.00 / M tokens = $0.01 / 1K tokens

# 计算
price = (input_price_per_m * input_tokens / 1000000) + 
        (output_price_per_m * output_tokens / 1000000)

# 示例: 100 input + 200 output
price = (2.50 * 100 / 1000000) + (10.00 * 200 / 1000000)
      = 0.00025 + 0.002
      = $0.00225
```

但这个结果太小了，不对。

---

### 可能性 2: Quota 转实际美元

如果用户是从日志中的 quota 值计算：

```python
# 当前系统记录的 quota
prompt_quota = 125
completion_quota = 750
total_quota = 875

# 转换为美元 (1 quota = $0.002)
price_usd = total_quota * 0.002 = $1.75
```

但如果用户按照 OpenAI 官方价格手动计算：
```python
price_official = (100 * 0.0025) + (200 * 0.01) = $2.25
```

**比例**: $2.25 / $1.75 = 1.29倍（不是严格的2倍）

---

### 可能性 3: ModelRatio 与 CompletionRatio 混淆

如果理解为"相对倍数"模式：

```go
// 错误理解：completionRatio = 3 是相对倍数
completionQuota = promptTokens × modelRatio × completionRatio
                = 200 × 1.25 × 3
                = 750 quota = $1.50
```

正确理解：completionRatio = 3.75 是绝对值
```go
completionQuota = completionTokens × completionRatio
                = 200 × 3.75
                = 750 quota = $1.50
```

两种理解结果相同，所以不是这个问题。

---

## 📋 所有模型的 CompletionRatio 问题

检查 `model/models.go` 中的其他模型：

| 模型 | ModelRatio | CompletionRatio | OpenAI Input | OpenAI Output | 是否正确 |
|------|-----------|----------------|--------------|---------------|---------|
| gpt-4o | 1.25 | 3.75 | $0.0025 | $0.01 | ❌ 应为 5.0 |
| gpt-4o-2024-08-06 | 1.25 | 5.0 | $0.0025 | $0.01 | ✅ 正确 |
| gpt-4o-mini | 0.075 | 0.3 | $0.00015 | $0.0006 | ✅ 正确 |
| gpt-4 | 15 | 30 | $0.03 | $0.06 | ✅ 正确 |
| gpt-3.5-turbo | 0.25 | 0.75 | $0.0005 | $0.0015 | ✅ 正确 |

**发现**: 
- ✅ 大部分模型的 CompletionRatio 是正确的
- ❌ `gpt-4o` 的 CompletionRatio = 3.75 是错误的，应该是 5.0
- ✅ `gpt-4o-2024-08-06` 已经正确设置为 5.0

---

## 🔧 解决方案

### 修复 1: 更正 gpt-4o 的 CompletionRatio

```go
// model/models.go
var ModelMetaMap = map[string]ModelMeta{
    "gpt-4o": {
        Model: "gpt-4o",
        ChannelType: 1,
        ModelRatio: 1.25,
        CompletionRatio: 5.0,  // 修改：3.75 → 5.0
    },
    "gpt-4o-2024-05-13": {
        Model: "gpt-4o-2024-05-13",
        ChannelType: 1,
        ModelRatio: 1.25,
        CompletionRatio: 5.0,  // 修改：3.75 → 5.0
    },
    "gpt-4o-2024-11-20": {
        Model: "gpt-4o-2024-11-20",
        ChannelType: 1,
        ModelRatio: 1.25,
        CompletionRatio: 5.0,  // 修改：3.75 → 5.0
    },
    "chatgpt-4o-latest": {
        Model: "chatgpt-4o-latest",
        ChannelType: 1,
        ModelRatio: 1.25,
        CompletionRatio: 5.0,  // 修改：3.75 → 5.0
    },
}
```

---

### 修复 2: 同步更新 ratio/model.go（如果还在使用）

```go
// relay/billing/ratio/model.go
var CompletionRatio = map[string]float64{
    "gpt-4o": 4.0,  // 5.0 / 1.25 = 4（相对倍数）
}

func GetCompletionRatio(name string, channelType int) float64 {
    if strings.HasPrefix(name, "gpt-4o") {
        if name == "gpt-4o-2024-08-06" {
            return 4  // 已经正确
        }
        return 4  // 修改：3 → 4
    }
    // ...
}
```

**注意**: 根据代码逻辑，系统优先使用 `ModelMetaMap` 中的值，所以重点是修复 `model/models.go`。

---

## 🧪 修复后验证

### 修复后计算

```go
modelRatio = 1.25
completionRatio = 5.0  // ← 修复后
groupRatio = 1.0

// Prompt: 100 tokens
promptQuota = 100 × 1.25 × 1.0 = 125 quota = $0.25

// Completion: 200 tokens
completionQuota = 200 × 5.0 × 1.0 = 1000 quota = $2.00

// 总计
totalQuota = 125 + 1000 = 1125 quota = $2.25 ✅
```

### 与 OpenAI 官方对比

```
OpenAI: (100 × $0.0025) + (200 × $0.01) = $0.25 + $2.00 = $2.25 ✅
系统: 1125 quota × $0.002 = $2.25 ✅
```

**完全匹配！**

---

## 📈 影响范围

### 受影响的模型
- `gpt-4o`
- `gpt-4o-2024-05-13`
- `gpt-4o-2024-11-20`
- `chatgpt-4o-latest`

### 不受影响的模型
- `gpt-4o-2024-08-06` ✅ 已正确
- `gpt-4o-mini` ✅ 已正确
- `gpt-4` ✅ 已正确
- 其他所有模型 ✅

### 财务影响
- **对用户**: 少收费约 **22%**
- **对平台**: 收入损失约 **22%**

**示例**:
- 每天处理 1,000,000 completion tokens (gpt-4o)
- 当前收费: 1,000,000 × 3.75 / 1000 × $0.002 = $7.50
- 正确收费: 1,000,000 × 5.0 / 1000 × $0.002 = $10.00
- **每天损失: $2.50**

---

## 💡 总结

### 根本原因
`gpt-4o` 系列模型的 `CompletionRatio` 设置错误：
- 当前: 3.75 quota（$0.0075 / 1K tokens）
- 正确: 5.0 quota（$0.01 / 1K tokens）

### 修复方法
修改 `model/models.go` 中的 `ModelMetaMap`，将受影响模型的 `CompletionRatio` 从 `3.75` 改为 `5.0`。

### 验证公式
```
正确的 CompletionRatio = OpenAI Output Price / $0.002

对于 gpt-4o:
CompletionRatio = $0.01 / $0.002 = 5.0 ✅
```

---

**创建日期**: 2025-11-10  
**严重程度**: 🔴 高（财务影响）  
**建议**: 立即修复并通知用户可能的补偿政策



