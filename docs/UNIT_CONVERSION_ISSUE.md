# 价格计算单位转换问题分析

## 🔍 问题本质

用户期望的公式：
```
price = input_price * input_tokens / 1000000 + output_price * output_tokens / 1000000
```

其中价格单位是 **$/M tokens**（每百万 tokens）

但系统实际使用的单位是不同的！

---

## 📊 系统的实际逻辑

### 1. 后端计算（`objects/billing.go`）

```go
quota = int64(math.Ceil(
    float64(promptTokens) * ratio + 
    float64(completionTokens) * completionRatio
))
```

其中：
- `promptTokens` - 单位：**个 token**（例如 100）
- `completionTokens` - 单位：**个 token**（例如 200）
- `ratio = modelRatio * groupRatio` 
- `completionRatio` - 从数据库读取

### 2. Quota 单位定义

```go
// relay/billing/ratio/model.go
const (
    USD = 500  // $0.002 = 1 quota -> $1 = 500 quota
)
```

**含义**: 
- **1 quota = $0.002 = 0.2美分**
- **500 quota = $1**

### 3. ModelRatio 的定义

```go
// relay/billing/ratio/model.go
// 1 === $0.002，表示 1k token 的基础价格
var ModelRatio = map[string]float64{
    "gpt-4o": 1.25,  // $0.0025 / 1K tokens
}
```

**关键问题**: 注释说"1k token 的价格"，但代码计算时**没有除以 1000**！

---

## 🧮 实际计算验证

### 示例：100 input tokens + 200 output tokens (gpt-4o)

#### 系统当前计算（假设 completionRatio = 5.0）

```go
// 步骤 1: 计算 quota
ratio = 1.25 * 1.0 = 1.25
completionRatio = 5.0

quota = 100 * 1.25 + 200 * 5.0
      = 125 + 1000
      = 1125 quota

// 步骤 2: 转换为美元
price = 1125 * $0.002 = $2.25
```

#### OpenAI 官方价格

```
price = (100 / 1000) * $2.50 + (200 / 1000) * $10.00
      = 0.1 * $2.50 + 0.2 * $10.00
      = $0.25 + $2.00
      = $2.25 ✅
```

**结果：完全一致！**

---

## 💡 关键发现

### ModelRatio 的实际含义

虽然注释说"1k token 的价格"，但从计算结果看，**ModelRatio 实际上是"每个 token 对应的 quota"**，而不是"每 1K tokens 对应的 quota"！

**验证**:
```
gpt-4o Input: $0.0025 / 1K tokens = $0.0000025 / 1 token

每个 token 对应的 quota:
= $0.0000025 / $0.002 
= 0.00125 quota / token
= 1.25 quota / 1000 tokens

所以 modelRatio = 1.25 表示 1000 个 tokens = 1.25 quota
```

### 公式的正确理解

```go
quota = promptTokens * modelRatio + completionTokens * completionRatio
```

实际上等价于：

```
quota = (promptTokens / 1000) * (modelRatio * 1000) 
      + (completionTokens / 1000) * (completionRatio * 1000)
```

换句话说，虽然代码没有显式除以 1000，但 modelRatio 的数值已经预先调整为"每个 token"的 quota 值（实际上是每 1000 个 token 的值）。

---

## 🔄 单位换算关系

### OpenAI 官方单位（$/1K tokens）→ 系统 quota

```
Step 1: OpenAI 价格转换为 $/token
input_price_per_token = $0.0025 / 1000 = $0.0000025 / token
output_price_per_token = $0.01 / 1000 = $0.00001 / token

Step 2: $/token 转换为 quota/token
input_quota_per_token = $0.0000025 / $0.002 = 0.00125 quota/token
output_quota_per_token = $0.00001 / $0.002 = 0.005 quota/token

Step 3: 转换为 quota/1000tokens（这就是 ModelRatio）
modelRatio = 0.00125 * 1000 = 1.25
completionRatio = 0.005 * 1000 = 5.0
```

### 用户期望单位（$/M tokens）→ 系统 quota

如果用户看到的价格是：
- Input: $2.50 / M tokens
- Output: $10.00 / M tokens

转换为系统 quota：
```
modelRatio = ($2.50 / 1,000,000) / $0.002 * 1000
           = $0.0000025 / $0.002 * 1000
           = 1.25 ✅

completionRatio = ($10.00 / 1,000,000) / $0.002 * 1000
                = $0.00001 / $0.002 * 1000
                = 5.0 ✅
```

---

## 🎯 为什么用户觉得是"两倍"？

### 可能性 1: completionRatio 错误

如果数据库中 `completion_ratio = 3.75`（而不是正确的 5.0）:

```
# 100 input + 200 output
实际扣费 = (100 * 1.25 + 200 * 3.75) * $0.002
        = (125 + 750) * $0.002
        = $1.75

用户期望 = (100 / 1000) * $2.50 + (200 / 1000) * $10.00
        = $0.25 + $2.00
        = $2.25

比例 = $2.25 / $1.75 = 1.29 倍
```

**不是严格的 2 倍，但接近！**

---

### 可能性 2: 用户看到的是"quota"而不是"美元"

如果用户直接看日志中的 quota 值，可能会误解：

```sql
SELECT quota FROM logs WHERE model_name = 'gpt-4o' LIMIT 1;
-- 结果: 1125
```

用户可能以为这是"美分"或其他单位，然后手动计算：

```
用户计算（按 OpenAI 官方）:
= (100 / 1000) * $2.50 + (200 / 1000) * $10.00
= $2.25

但看到 quota = 1125，以为是 $11.25 或其他值
```

---

### 可能性 3: groupRatio 不是 1.0

如果用户组的 `groupRatio = 2.0`：

```go
ratio = modelRatio * groupRatio = 1.25 * 2.0 = 2.5
completionRatio 保持不变（注意这里有 bug！）

quota = 100 * 2.5 + 200 * 5.0
      = 250 + 1000
      = 1250 quota

price = 1250 * $0.002 = $2.50

预期（如果 completionRatio 也乘以 groupRatio）:
price = (100 * 2.5 + 200 * 10.0) * $0.002
      = (250 + 2000) * $0.002
      = $4.50
```

**等等！这里发现一个 BUG！**

看代码：
```go
ratio := modelRatio * groupRatio  // ← modelRatio 乘了 groupRatio
completionRatio := modelMeta.CompletionRatio  // ← 但这里没有乘！

quota = promptTokens * ratio + completionTokens * completionRatio
```

**这是不对的！** `completionRatio` 也应该乘以 `groupRatio`！

正确的代码应该是：
```go
ratio := modelRatio * groupRatio
completionRatio := modelMeta.CompletionRatio * groupRatio  // ← 需要修复！

quota = promptTokens * ratio + completionTokens * completionRatio
```

---

## 🐛 发现的 Bug

### Bug 位置：`objects/billing.go` 第 96 行

**当前代码**:
```go
modelRatio := modelMeta.ModelRatio
groupRatio := billingratio.GetGroupRatio(meta.Group)
ratio := modelRatio * groupRatio  // ← Input 乘了 groupRatio

completionRatio := modelMeta.CompletionRatio  // ← Output 没乘！

quota = int64(math.Ceil(
    float64(promptTokens) * ratio + 
    float64(completionTokens) * completionRatio  // ← BUG: 应该也乘 groupRatio
))
```

**正确代码**:
```go
modelRatio := modelMeta.ModelRatio
groupRatio := billingratio.GetGroupRatio(meta.Group)
ratio := modelRatio * groupRatio

completionRatio := modelMeta.CompletionRatio * groupRatio  // ← 修复！

quota = int64(math.Ceil(
    float64(promptTokens) * ratio + 
    float64(completionTokens) * completionRatio
))
```

或者直接在公式中乘：
```go
quota = int64(math.Ceil(
    (float64(promptTokens) * modelRatio + 
     float64(completionTokens) * completionRatio) * groupRatio
))
```

---

## 🔧 修复方案

### 方案 1: 修复 groupRatio 的应用

```go
// objects/billing.go
func PostConsumeQuota(ctx context.Context, usage *entity.Usage, meta *Meta, preConsumedQuota int64) {
    modelMeta, err := model.GetModelMetaByModel(meta.ActualModelName)
    if err != nil {
        return
    }
    
    modelRatio := modelMeta.ModelRatio
    completionRatio := modelMeta.CompletionRatio
    groupRatio := billingratio.GetGroupRatio(meta.Group)
    
    // 修复：将 groupRatio 应用到整个计算
    quota = int64(math.Ceil(
        (float64(promptTokens) * modelRatio + 
         float64(completionTokens) * completionRatio) * groupRatio
    ))
    
    // 或者：
    // ratio := modelRatio * groupRatio
    // adjustedCompletionRatio := completionRatio * groupRatio
    // quota = int64(math.Ceil(
    //     float64(promptTokens) * ratio + 
    //     float64(completionTokens) * adjustedCompletionRatio
    // ))
}
```

---

## 📋 总结

### ✅ 正确理解

1. **ModelRatio 单位**: 每 1000 个 tokens 对应的 quota（不是每 1 个 token）
2. **Quota 单位**: 1 quota = $0.002
3. **计算公式**: `quota = (tokens * ratio_per_1000_tokens) / 实际没有除法，因为数值已调整`

### 🐛 发现的问题

1. **completionRatio 没有乘 groupRatio** - 这会导致 Input 和 Output 的分组倍率不一致
2. **可能的 completionRatio 值错误** - 部分模型的 completion_ratio 可能不正确（如 gpt-4o 应该是 5.0 而不是 3.75）

### 🔧 用户应该做的

1. **检查数据库中的 completion_ratio**:
   ```sql
   SELECT model, model_ratio, completion_ratio 
   FROM model_meta 
   WHERE model LIKE 'gpt-4o%';
   ```

2. **通过前端修改错误的值**:
   - 访问 `/panel/models`
   - 编辑对应模型
   - 修改 `completion_ratio` 为正确值

3. **检查 groupRatio 设置**:
   - 确认你的用户组的倍率是否为 1.0
   - 如果不是，可能导致价格翻倍

---

**创建日期**: 2025-11-10  
**严重程度**: 🔴 高（计费逻辑 bug）  
**影响**: groupRatio != 1.0 时，Output tokens 计费错误



