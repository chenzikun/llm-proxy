# 放大1000倍的逻辑分析

## 📐 完整的单位换算链路

### 第1步：OpenAI 官方价格（$/M tokens）

```
gpt-4o:
- Input: $2.50 / M tokens
- Output: $10.00 / M tokens
```

---

### 第2步：转换为 quota（系统内部单位）

#### 关键常量定义

```go
// relay/billing/ratio/model.go
const (
    USD = 500  // $0.002 = 1 quota -> $1 = 500 quota
)
```

**含义**: 
- 1 quota = $0.002 = 0.2 美分
- 500 quota = $1

#### 转换公式

```
ModelRatio = (OpenAI Price per M tokens) / ($0.002 per quota) / 1000

对于 gpt-4o Input:
ModelRatio = $2.50 / $0.002 / 1000
           = 1250 / 1000
           = 1.25

对于 gpt-4o Output:
CompletionRatio = $10.00 / $0.002 / 1000
                = 5000 / 1000
                = 5.0
```

**注意这里除以了 1000！这就是"放大1000倍"的来源！**

---

### 第3步：后端计算 quota

#### 代码逻辑 (`objects/billing.go`)

```go
quota = int64(math.Ceil(
    float64(promptTokens) * ratio + 
    float64(completionTokens) * completionRatio
))
```

#### 实例计算（100,000 input + 200,000 output）

```go
ratio = 1.25
completionRatio = 5.0

quota = 100000 * 1.25 + 200000 * 5.0
      = 125000 + 1000000
      = 1,125,000
```

**这里的 quota 是"放大了1000倍"的值！**

---

### 第4步：前端显示价格

#### QuotaPerUnit 定义

```go
// common/config/config.go
var QuotaPerUnit = 500 * 1000.0  // 500000
```

**这里也放大了1000倍！**

#### 前端计算

```javascript
// web/default/src/helpers/render.js
price = quota / quotaPerUnit
      = 1125000 / 500000
      = $2.25 ✅
```

**两个1000倍抵消了！结果正确！**

---

## 🎯 "放大1000倍"的完整逻辑

### 设计思路

1. **modelRatio 定义时除以1000**:
   ```
   ModelRatio = (Price per M tokens) / $0.002 / 1000
   ```
   这样 modelRatio 表示"每个 token 对应的 quota × 1000"

2. **后端计算时不除以1000**:
   ```go
   quota = tokens * modelRatio  // 直接相乘，结果放大了1000倍
   ```

3. **QuotaPerUnit 也放大1000倍**:
   ```go
   QuotaPerUnit = 500 * 1000 = 500000
   ```

4. **前端显示时抵消**:
   ```javascript
   price = quota / quotaPerUnit
         = (tokens * ratio * 1000) / (500 * 1000)
         = tokens * ratio / 500
   ```

**好处**: 避免 quota 是小数，数据库存整数更高效。

---

## 🐛 为什么会出现"2倍"关系？

### 可能性1：QuotaPerUnit 配置错误

如果 `QuotaPerUnit` 被误配置为 `1000000`（而不是 `500000`）：

```
price = quota / quotaPerUnit
      = 1125000 / 1000000
      = $1.125  ← 只有正确值的一半！

正确应该是: 1125000 / 500000 = $2.25
```

**比例**: $2.25 / $1.125 = **2倍** ✅

---

### 可能性2：QuotaPerUnit 配置为 250000

如果 `QuotaPerUnit = 250000`:

```
price = 1125000 / 250000 = $4.50

正确: $2.25

比例: $4.50 / $2.25 = 2倍 ✅
```

---

### 可能性3：modelRatio 和 completionRatio 没有按正确公式转换

如果在定义 modelRatio 时**没有除以1000**：

```
错误的定义:
ModelRatio = $2.50 / $0.002 = 1250  (而不是 1.25)

计算:
quota = 100000 * 1250 + 200000 * 5000
      = 125,000,000 + 1,000,000,000
      = 1,125,000,000

price = 1125000000 / 500000 = $2,250  (放大了1000倍！)

正确: $2.25

比例: $2250 / $2.25 = 1000倍 (不是2倍)
```

---

## 🔍 精确定位问题

### 需要检查的配置

#### 1. QuotaPerUnit 的值

```sql
SELECT value FROM options WHERE key = 'QuotaPerUnit';
```

**期望值**: `500000`

**如果是其他值**:
- `1000000` → 价格是正确的 1/2
- `250000` → 价格是正确的 2倍

---

#### 2. ModelRatio 的计算方式

查看数据库中的实际值：

```sql
SELECT model, model_ratio, completion_ratio 
FROM model_meta 
WHERE model = 'gpt-4o';
```

**期望值**:
- `model_ratio`: **1.25**（不是1250！）
- `completion_ratio`: **5.0**（不是5000！）

**验证公式**:
```
model_ratio = (OpenAI Input Price per M tokens) / $0.002 / 1000
            = $2.50 / $0.002 / 1000
            = 1.25 ✅

completion_ratio = (OpenAI Output Price per M tokens) / $0.002 / 1000
                 = $10.00 / $0.002 / 1000
                 = 5.0 ✅
```

---

#### 3. 计算公式验证

给定实际日志数据：
- `prompt_tokens = ?`
- `completion_tokens = ?`
- `quota = ?`

**手动计算**:
```
预期 quota = prompt_tokens * model_ratio + completion_tokens * completion_ratio

预期 price = quota / quotaPerUnit
```

**对比**:
```
实际 price（用户按公式算）= ?
系统 price（日志显示）= ?

比例 = 系统 price / 实际 price
```

---

## 🎯 三个关键检查点

### ✅ 检查点1：modelRatio 定义

```go
// relay/billing/ratio/model.go 第38行
"gpt-4o": 1.25,  // 应该是 1.25，不是 1250！
```

如果这里是 `1250`，那计算出的 quota 会放大1000倍，最终价格也会放大1000倍（不是2倍）。

---

### ✅ 检查点2：QuotaPerUnit 配置

```go
// common/config/config.go 第22行
var QuotaPerUnit = 500 * 1000.0  // 应该是 500000
```

或者查数据库：
```sql
SELECT value FROM options WHERE key = 'QuotaPerUnit';
-- 应该返回: 500000
```

**如果是 1000000** → 显示价格是正确的 **1/2** → 用户看到的价格少了一半 → **实际扣费是2倍** ✅

---

### ✅ 检查点3：前端 localStorage

前端从后端获取 `quota_per_unit` 后存储在 localStorage：

```javascript
localStorage.getItem('quota_per_unit')
```

可能的问题：
1. 后端返回的是 `500000`
2. 但前端解析或存储时出错，变成了其他值

---

## 💡 最可能的原因

基于"2倍"关系，最可能的是：

### **QuotaPerUnit = 1000000（错误）而不是 500000（正确）**

**验证**:
```sql
SELECT value FROM options WHERE key = 'QuotaPerUnit';
```

**如果确实是 1000000**，修改为：
```sql
UPDATE options SET value = '500000' WHERE key = 'QuotaPerUnit';
```

然后刷新前端，清除 localStorage，重新加载配置。

---

## 📋 请用户提供数据

为了精确定位，请提供：

### 1. 配置值
```sql
SELECT value FROM options WHERE key = 'QuotaPerUnit';
```

### 2. 模型费率
```sql
SELECT model, model_ratio, completion_ratio 
FROM model_meta 
WHERE model = 'gpt-4o';
```

### 3. 实际日志
```sql
SELECT 
    prompt_tokens,
    completion_tokens,
    quota,
    model_name
FROM logs 
WHERE model_name = 'gpt-4o'
ORDER BY created_at DESC
LIMIT 1;
```

### 4. 前端 localStorage
打开浏览器控制台执行：
```javascript
console.log('quota_per_unit:', localStorage.getItem('quota_per_unit'));
```

---

有了这些数据，我就能准确告诉你问题出在哪里！

---

**创建日期**: 2025-11-10  
**关键**: 最可能是 QuotaPerUnit 配置错误导致显示价格翻倍



