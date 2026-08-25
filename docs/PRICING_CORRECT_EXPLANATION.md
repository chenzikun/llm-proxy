# 价格计算机制正确解释

## 🏗️ 系统架构

### 1. 数据流向

```
代码 ModelMetaMap (初始化) 
    ↓
数据库 model_meta 表 (实际存储)
    ↓
前端管理界面 (可修改)
    ↓
计费逻辑读取 (GetModelMetaByModel)
```

---

## 📊 三层结构

### 第 1 层：代码中的预定义（`model/models.go`）

```go
// 只用于初始化，不直接参与计费！
var ModelMetaMap = map[string]ModelMeta{
    "gpt-4o": {
        Model: "gpt-4o",
        ChannelType: 1,
        ModelRatio: 1.25,       // 仅用于首次写入数据库
        CompletionRatio: 3.75,  // 仅用于首次写入数据库
    },
}

// 启动时调用，将 ModelMetaMap 同步到数据库
func InitModelMetaFromMap() {
    for _, modelMeta := range ModelMetaMap {
        CreateOrUpdateModelMeta(&modelMetaCopy)  // 写入数据库
    }
}
```

**作用**: 
- ✅ 首次启动时初始化数据库
- ✅ 新增模型时提供默认值
- ❌ **不直接参与计费计算**

---

### 第 2 层：数据库表（`model_meta`）

```sql
CREATE TABLE model_meta (
    id INT PRIMARY KEY AUTO_INCREMENT,
    model VARCHAR(255),
    channel_type INT,
    status INT DEFAULT 1,
    model_ratio FLOAT,        -- Input 价格（实际使用）
    completion_ratio FLOAT,   -- Output 价格（实际使用）
    created_time BIGINT,
    update_time BIGINT,
    INDEX idx_channel_type (channel_type)
);
```

**示例数据**:
```
| id | model    | channel_type | model_ratio | completion_ratio |
|----|----------|--------------|-------------|------------------|
| 1  | gpt-4o   | 1            | 1.25        | 3.75             |
| 2  | gpt-4    | 1            | 15.0        | 30.0             |
```

**作用**: 
- ✅ **实际计费时读取的数据源**
- ✅ 可通过前端或 API 修改
- ✅ 修改后立即生效

---

### 第 3 层：前端管理界面

#### Berry 主题的模型管理页面

路径: `web/berry/src/views/ModelMeta/`

**功能**:
1. ✅ 查看所有模型费率
2. ✅ 编辑单个模型费率
3. ✅ 批量添加/导入模型
4. ✅ 搜索模型

**访问路径**: `/panel/models` 或 `/model-meta`

**前端组件**:
- `index.js` - 主页面
- `component/EditModal.js` - 编辑模态框
- `component/TableRow.js` - 表格行
- `component/BatchModal.js` - 批量导入

---

## 💰 实际计费逻辑

### 读取费率（从数据库）

```go
// objects/billing.go
func PostConsumeQuota(ctx context.Context, usage *entity.Usage, meta *Meta, preConsumedQuota int64) {
    // 从数据库读取费率 ← 关键！
    modelMeta, err := model.GetModelMetaByModel(meta.ActualModelName)
    if err != nil {
        return
    }
    
    // 使用数据库中的值
    modelRatio := modelMeta.ModelRatio          // 从数据库读取
    completionRatio := modelMeta.CompletionRatio // 从数据库读取
    groupRatio := billingratio.GetGroupRatio(meta.Group)
    
    // 计算实际消费
    quota = int64(math.Ceil(
        float64(promptTokens) * modelRatio * groupRatio +
        float64(completionTokens) * completionRatio * groupRatio
    ))
}
```

**关键函数**:
```go
// model/model-meta.go
func GetModelMetaByModel(model string) (*ModelMeta, error) {
    var modelMeta ModelMeta
    // 从数据库查询！
    err := DB.Where("model = ?", model).First(&modelMeta).Error
    return &modelMeta, err
}
```

---

## 🔧 如何修改费率

### 方法 1: 通过前端界面（推荐✅）

1. **登录管理后台**
   - 访问: `https://your-domain/panel/models`

2. **找到目标模型**
   - 搜索: `gpt-4o`

3. **点击编辑按钮**

4. **修改费率**
   ```
   模型倍率 (Model Ratio): 1.25 → 1.25 (不变)
   补全倍率 (Completion Ratio): 3.75 → 5.0 (修改)
   ```

5. **保存**
   - 点击"确定"
   - 修改立即生效，无需重启

---

### 方法 2: 通过 API

```bash
# 更新单个模型费率
curl -X PUT https://your-domain/api/model-meta/ \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "id": 1,
    "status": 1,
    "model_ratio": 1.25,
    "completion_ratio": 5.0
  }'
```

---

### 方法 3: 直接修改数据库（不推荐）

```sql
-- 查看当前值
SELECT * FROM model_meta WHERE model = 'gpt-4o';

-- 修改费率
UPDATE model_meta 
SET completion_ratio = 5.0, 
    update_time = UNIX_TIMESTAMP()
WHERE model = 'gpt-4o';
```

**注意**: 修改后无需重启服务，下次计费时会自动读取新值。

---

### 方法 4: 修改代码后重启（仅限初始化）

```go
// model/models.go
var ModelMetaMap = map[string]ModelMeta{
    "gpt-4o": {
        Model: "gpt-4o",
        ChannelType: 1,
        ModelRatio: 1.25,
        CompletionRatio: 5.0,  // 修改这里
    },
}
```

**注意**: 
- ❌ 只在首次启动或新增模型时生效
- ❌ 如果数据库中已存在该模型，`CreateOrUpdateModelMeta` 会**更新**数据库
- ✅ 适用于批量初始化新模型

---

## 🧮 验证费率是否正确

### 查询数据库

```sql
SELECT 
    model,
    model_ratio,
    completion_ratio,
    model_ratio * 0.002 as input_price_per_1k,
    completion_ratio * 0.002 as output_price_per_1k
FROM model_meta
WHERE model LIKE 'gpt-4o%';
```

**期望输出**:
```
| model              | model_ratio | completion_ratio | input_price  | output_price |
|--------------------|-------------|------------------|--------------|--------------|
| gpt-4o             | 1.25        | 5.0              | $0.0025      | $0.01        |
| gpt-4o-2024-08-06  | 1.25        | 5.0              | $0.0025      | $0.01        |
| gpt-4o-mini        | 0.075       | 0.3              | $0.00015     | $0.0006      |
```

---

### 通过日志验证

查看实际扣费日志：

```sql
SELECT 
    model_name,
    prompt_tokens,
    completion_tokens,
    quota,
    quota * 0.002 as actual_usd,
    -- 手动计算
    (prompt_tokens / 1000.0 * 0.0025 + completion_tokens / 1000.0 * 0.01) as expected_usd
FROM logs
WHERE model_name = 'gpt-4o'
LIMIT 10;
```

**如果 `actual_usd` ≈ `expected_usd`，说明费率正确！**

---

## 🔍 问题排查

### 为什么修改代码不生效？

**原因**: 代码中的 `ModelMetaMap` 只在以下情况写入数据库：
1. 首次启动（表为空）
2. 调用 `CreateOrUpdateModelMeta` 且模型不存在

**解决**:
- ✅ 方案 1: 通过前端修改（推荐）
- ✅ 方案 2: 删除数据库记录后重启
  ```sql
  DELETE FROM model_meta WHERE model = 'gpt-4o';
  -- 然后重启服务
  ```

---

### 当前 gpt-4o 费率是否正确？

**检查方法**:

```bash
# 通过 API 查询
curl https://your-domain/api/model-meta/?keyword=gpt-4o \
  -H "Authorization: Bearer YOUR_TOKEN"
```

**期望响应**:
```json
{
  "success": true,
  "data": [
    {
      "id": 1,
      "model": "gpt-4o",
      "channel_type": 1,
      "model_ratio": 1.25,
      "completion_ratio": 3.75  // ← 如果是这个值，说明不正确！
    }
  ]
}
```

**正确值应该是**:
- `model_ratio`: 1.25 ✅
- `completion_ratio`: 5.0 ✅

---

## 📈 OpenAI 官方价格对照表

| 模型 | Input (官方) | Output (官方) | ModelRatio | CompletionRatio |
|------|-------------|--------------|------------|-----------------|
| gpt-4o | $0.0025 / 1K | $0.01 / 1K | 1.25 | 5.0 |
| gpt-4o-mini | $0.00015 / 1K | $0.0006 / 1K | 0.075 | 0.3 |
| gpt-4 | $0.03 / 1K | $0.06 / 1K | 15.0 | 30.0 |
| gpt-3.5-turbo | $0.0005 / 1K | $0.0015 / 1K | 0.25 | 0.75 |

**计算公式**:
```
ModelRatio = OpenAI Input Price / $0.002
CompletionRatio = OpenAI Output Price / $0.002
```

**验证 gpt-4o**:
```
ModelRatio = $0.0025 / $0.002 = 1.25 ✅
CompletionRatio = $0.01 / $0.002 = 5.0 ✅
```

---

## 🎯 总结

### ✅ 正确做法

1. **通过前端界面修改** - 访问 `/panel/models`，编辑对应模型
2. **修改立即生效** - 无需重启服务
3. **数据库是真理** - 计费逻辑只读取数据库

### ❌ 错误理解

1. ~~修改代码就能改费率~~ - 代码只用于初始化
2. ~~需要重启服务生效~~ - 从数据库读取，实时生效
3. ~~费率写死在代码中~~ - 实际存储在数据库

### 🔧 建议修复流程

1. **登录管理后台**
2. **进入模型管理页面** (`/panel/models`)
3. **搜索 `gpt-4o`**
4. **点击编辑**
5. **修改 `completion_ratio` 从 `3.75` 改为 `5.0`**
6. **保存** - 立即生效！

---

**创建日期**: 2025-11-10  
**推荐做法**: ✅ 通过前端管理界面修改  
**影响**: 修改后立即对新请求生效



