# 调试步骤：找出2倍差异的原因

## 步骤1：获取实际日志数据

```sql
SELECT 
    id,
    model_name,
    prompt_tokens,
    completion_tokens,
    quota,
    content,
    created_at
FROM logs 
WHERE model_name = 'gpt-4o'
  AND type = 2  -- 消费类型
ORDER BY created_at DESC
LIMIT 1;
```

**请记录**:
- prompt_tokens = ?
- completion_tokens = ?
- quota = ?
- content = ? (应该显示"模型倍率 xx，补全倍率 xx")

---

## 步骤2：获取模型费率

```sql
SELECT model, model_ratio, completion_ratio 
FROM model_meta 
WHERE model = 'gpt-4o';
```

**请记录**:
- model_ratio = ?
- completion_ratio = ?

---

## 步骤3：手动计算（用你的公式）

假设从步骤1和步骤2得到：
- prompt_tokens = A
- completion_tokens = B  
- model_ratio = M
- completion_ratio = C

### 你的公式（$/M tokens）

```
你的计算价格 = (M * A / 1000000) + (C * B / 1000000)
```

**请计算并记录结果**: 你的计算价格 = ?

---

## 步骤4：系统应该计算的 quota

```
系统 quota = A * M + B * C
```

**请计算并记录结果**: 系统应该计算的 quota = ?

---

## 步骤5：对比

| 项目 | 值 | 计算方式 |
|------|-----|---------|
| **步骤1：日志中的 quota** | ? | 从数据库查询 |
| **步骤4：系统应该的 quota** | ? | A×M + B×C |
| **是否一致？** | ? | 如果不一致，说明计算逻辑有bug |

---

## 步骤6：价格转换

### 系统显示的价格

```
系统价格 = (步骤1的quota) / 500000
```

**请计算**: 系统价格 = ?

### 你期望的价格

```
期望价格 = (步骤3的结果) * 0.002
```

**请计算**: 期望价格 = ?

### 对比

```
比例 = 系统价格 / 期望价格 = ?
```

**这个比例应该就是你说的"2倍"！**

---

## 步骤7：定位问题

### 情况A：步骤5中 quota 不一致

**说明**: 后端计算逻辑有bug（可能 groupRatio 或其他倍率）

**需要检查**:
```sql
-- 检查用户的分组倍率
SELECT `group` FROM users WHERE id = YOUR_USER_ID;

-- 然后查看这个 group 的 ratio
-- （在代码 relay/billing/ratio/group.go 中）
```

---

### 情况B：步骤5中 quota 一致，但步骤6比例是2倍

**说明**: 

1. **如果比例 = 2**，说明你的公式单位理解有误
2. **如果比例 = 0.5**，说明系统少算了一半

---

## 🎯 关键：你的公式单位

你说公式是：
```
price = input_price * input_tokens / 1000000 + output_price * output_tokens / 1000000
```

**请确认**：
- `input_price` 的单位是什么？是 **$/M tokens** 吗？
- 如果 model_ratio = 1.25，你认为它表示什么？

### 单位换算

```
OpenAI 官方: $2.50 / M tokens = $0.0025 / 1K tokens

转换为 quota:
1 quota = $0.002

所以: $0.0025 / 1K tokens ÷ $0.002 = 1.25 quota / 1K tokens

这就是 model_ratio = 1.25 的含义！
```

### 你的公式应该改为

```
price = (model_ratio * input_tokens / 1000 + completion_ratio * output_tokens / 1000) * 0.002
```

**或者**:

```
quota = model_ratio * input_tokens + completion_ratio * output_tokens
price = quota / 500000
```

**注意**: 这里 tokens 是"个数"，ratio 是"per 1K tokens"，但代码**没有除以1000**，这是因为系统设计就是"放大1000倍"！

---

## 🧮 完整示例

假设：
- prompt_tokens = 100,000
- completion_tokens = 200,000
- model_ratio = 1.25
- completion_ratio = 5.0

### 系统计算

```
quota = 100000 × 1.25 + 200000 × 5.0
      = 125000 + 1000000
      = 1,125,000

price = 1125000 / 500000 = $2.25
```

### 你的公式（如果理解为 $/M tokens）

```
input_price = model_ratio × 0.002 = 1.25 × 0.002 = $0.0025 / 1K tokens
                                                   = $2.50 / M tokens ✅

output_price = completion_ratio × 0.002 = 5.0 × 0.002 = $0.01 / 1K tokens
                                                       = $10.00 / M tokens ✅

price = (2.50 × 100000 / 1000000) + (10.00 × 200000 / 1000000)
      = 0.25 + 2.00
      = $2.25 ✅
```

**完全匹配！所以理论上不应该有2倍差异！**

---

## 📝 请提供数据

请按照**步骤1-6**执行，并提供：

1. 日志中的实际数据（步骤1）
2. 模型费率配置（步骤2）
3. 你的手动计算结果（步骤3-6）

这样我才能准确定位问题在哪里！



