# 重构方案：改用百万 tokens 为单位

## 🎯 目标

让价格计算更直观：
```
price = (input_price_per_M * tokens / 1000000 + output_price_per_M * tokens / 1000000) * 0.002
```

等价于：
```
quota = tokens * model_ratio / 1000000 + tokens * completion_ratio / 1000000
price = quota / 500
```

---

## 📋 当前逻辑（复杂）

```
ModelRatio = 1.25          // per 1K tokens
QuotaPerUnit = 500000      // 放大了1000倍
quota = tokens * 1.25      // 不除以1000（隐式放大）
price = quota / 500000     // 两个1000倍抵消
```

---

## ✨ 新逻辑（简洁）

```
ModelRatio = 1250          // per M tokens (直接存 $/M tokens 转 quota 的值)
QuotaPerUnit = 500         // 不放大
quota = tokens * 1250 / 1000000  // 显式除以百万
price = quota / 500        // 清晰直观
```

---

## 🔧 需要修改的文件

### 1. common/config/config.go

**当前**:
```go
var QuotaPerUnit = 500 * 1000.0 // $0.002 / 1K tokens
```

**改为**:
```go
var QuotaPerUnit = 500.0 // $0.002 per quota, calculated per M tokens
```

---

### 2. relay/billing/ratio/model.go

**当前**:
```go
var ModelRatio = map[string]float64{
    "gpt-4o": 1.25,  // $2.50 / M tokens ÷ 0.002 ÷ 1000
    // ...
}

var CompletionRatio = map[string]float64{
    "gpt-4o": 5.0,  // $10.00 / M tokens ÷ 0.002 ÷ 1000
    // ...
}
```

**改为**:
```go
var ModelRatio = map[string]float64{
    "gpt-4o": 1250.0,  // $2.50 / M tokens ÷ $0.002 = 1250 quota / M tokens
    "gpt-4-turbo": 5000.0,  // $10.00 / M tokens ÷ $0.002
    "gpt-4-turbo-2024-04-09": 5000.0,
    "gpt-4-0125-preview": 5000.0,
    "gpt-4-1106-preview": 5000.0,
    "gpt-4": 15000.0,  // $30.00 / M tokens ÷ $0.002
    "gpt-4-32k": 30000.0,  // $60.00 / M tokens ÷ $0.002
    "gpt-3.5-turbo": 250.0,  // $0.50 / M tokens ÷ $0.002
    "gpt-3.5-turbo-0125": 250.0,
    "gpt-3.5-turbo-instruct": 750.0,  // $1.50 / M tokens ÷ $0.002
    // ... 其他模型都乘以1000
}

var CompletionRatio = map[string]float64{
    "gpt-4o": 5000.0,  // $10.00 / M tokens ÷ $0.002 = 5000 quota / M tokens
    "gpt-4-turbo": 15000.0,  // $30.00 / M tokens ÷ $0.002
    // ... 其他模型都乘以1000
}
```

---

### 3. objects/billing.go

**当前**:
```go
quota = int64(math.Ceil(
    float64(promptTokens) * ratio + 
    float64(completionTokens) * completionRatio
))
```

**改为**:
```go
quota = int64(math.Ceil(
    float64(promptTokens) * ratio / 1000000.0 + 
    float64(completionTokens) * completionRatio / 1000000.0
))
```

**或者更清晰**:
```go
// 计算消耗的 quota
// ratio 和 completionRatio 的单位是 quota/M tokens
quotaFromPrompt := float64(promptTokens) * ratio / 1000000.0
quotaFromCompletion := float64(completionTokens) * completionRatio / 1000000.0
quota = int64(math.Ceil(quotaFromPrompt + quotaFromCompletion))
```

---

### 4. relay/billing/billing.go

**当前**:
```go
// 这个文件中的 quota 计算逻辑也要改
totalQuota := int64(promptTokens) * int64(ratio*1000)
quota := modelRatio * float64(totalQuota) * groupRatio / 1000.0
```

**需要检查并统一除以1000000的逻辑**

---

### 5. 更新注释

在所有相关文件添加清晰的注释：
```go
// ModelRatio: quota per M tokens
// 例如: gpt-4o 的官方价格是 $2.50/M tokens
// 转换为 quota: $2.50 ÷ $0.002 = 1250 quota/M tokens
```

---

## 🧪 测试用例

### 计算示例

**输入**:
- model: gpt-4o
- prompt_tokens: 100,000
- completion_tokens: 200,000

**新逻辑计算**:
```
model_ratio = 1250 quota/M tokens
completion_ratio = 5000 quota/M tokens

quota = (100000 * 1250 / 1000000) + (200000 * 5000 / 1000000)
      = 125 + 1000
      = 1125

price = 1125 / 500 = $2.25 ✅
```

**验证公式**:
```
price = (2.50 * 100000 / 1000000) + (10.00 * 200000 / 1000000)
      = 0.25 + 2.00
      = $2.25 ✅
```

---

## ⚠️ 数据库迁移

### 需要更新 model_meta 表

```sql
-- 备份现有数据
CREATE TABLE model_meta_backup AS SELECT * FROM model_meta;

-- 将所有 model_ratio 和 completion_ratio 乘以 1000
UPDATE model_meta 
SET model_ratio = model_ratio * 1000,
    completion_ratio = completion_ratio * 1000;

-- 验证
SELECT model, model_ratio, completion_ratio 
FROM model_meta 
WHERE model = 'gpt-4o';
-- 应该返回: model_ratio=1250, completion_ratio=5000
```

### 更新配置表（如果存在）

```sql
-- 如果配置存在数据库中
UPDATE options 
SET value = '500' 
WHERE key = 'QuotaPerUnit';
```

---

## 📝 优势对比

### 当前逻辑（隐式放大1000倍）

❌ 不直观，ratio 值很小  
❌ 需要理解"放大1000倍"的设计  
❌ quota 值很大（1,125,000）  
✅ 避免小数

### 新逻辑（显式按百万）

✅ 直观，ratio 就是官方价格转换  
✅ 单位清晰：quota/M tokens  
✅ quota 值合理（1,125）  
✅ 公式易懂：tokens * ratio / 1000000  
⚠️ quota 可能有小数（用 Ceil 向上取整）

---

## 🎯 建议

### 方案A：完全重构（推荐）

1. 修改所有相关代码
2. 执行数据库迁移
3. 更新文档和注释
4. 全面测试

**优点**: 一劳永逸，代码清晰  
**缺点**: 需要迁移数据，有一定风险

---

### 方案B：保持兼容（保守）

1. 只在前端显示时做转换
2. 后端保持现有逻辑
3. 添加详细注释

**优点**: 风险小，无需迁移  
**缺点**: 代码仍然不够清晰

---

## ❓ 你的选择

我可以帮你实施方案A（完全重构），包括：
- ✅ 修改所有代码文件
- ✅ 生成数据库迁移脚本
- ✅ 提供测试验证步骤

需要我开始吗？



