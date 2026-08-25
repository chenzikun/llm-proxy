# Session ID 支持的接口列表

## ✅ 已支持 `X-Session-ID` 的接口

以下所有接口都**完全支持** `X-Session-ID` header，无需额外修改：

### 1️⃣ Chat Completions（聊天对话）
- **路由**: `POST /v1/chat/completions`
- **Handler**: `controller.RelayTextHelper`
- **支持状态**: ✅ 完全支持
- **UUID 验证**: ✅ 强制 UUID v4 格式
- **日志记录**: ✅ 记录到数据库

**调用链**:
```
Request → Relay() → relayHelper() → RelayTextHelper() 
→ GetRequestMeta() 读取 X-Session-ID
→ 验证 UUID v4
→ PostConsumeQuota() 
→ PostCost()
→ RecordConsumeLog(sessionId)
```

---

### 2️⃣ Completions（文本补全）
- **路由**: `POST /v1/completions`
- **Handler**: `controller.RelayTextHelper`
- **支持状态**: ✅ 完全支持
- **UUID 验证**: ✅ 强制 UUID v4 格式
- **日志记录**: ✅ 记录到数据库

---

### 3️⃣ **Embeddings（文本嵌入）** 🆕
- **路由**: 
  - `POST /v1/embeddings`
  - `POST /v1/engines/:model/embeddings`
- **Handler**: `controller.RelayTextHelper`（通过 default case）
- **支持状态**: ✅ **完全支持（无需修改）**
- **UUID 验证**: ✅ 强制 UUID v4 格式
- **日志记录**: ✅ 记录到数据库

**调用链**:
```go
// controller/relay.go
func relayHelper(c *gin.Context, relayMode int) *objects.ErrorWithStatusCode {
    switch relayMode {
    case relaymode.ImagesGenerations:
        err = controller.RelayImageHelper(c, relayMode)
    case relaymode.AudioSpeech:
        err = controller.RelayAudioSpeechHelper(c, relayMode)
    // ...
    default:
        err = controller.RelayTextHelper(c, relayMode)  // ← Embeddings 走这里！
    }
    return err
}
```

**验证逻辑** (`relay/controller/text.go`):
```go
func RelayTextHelper(c *gin.Context, relayMode int) *objects.ErrorWithStatusCode {
    ctx := c.Request.Context()
    meta := objects.GetRequestMeta(c)  // ← 读取 X-Session-ID
    
    // 验证 X-Session-ID 格式（必须是 UUID v4）
    if meta.SessionId != "" && !common.IsValidUUIDv4(meta.SessionId) {
        return objects.ErrorWrapper(fmt.Errorf("invalid sessionId: %s, must be UUID v4 format", meta.SessionId), "invalid_session_id", http.StatusBadRequest)
    }
    
    // ... 处理请求 ...
    
    // 记录日志时会传递 meta.SessionId
    go objects.PostConsumeQuota(ctx, usage, meta, preConsumedQuota)
}
```

**日志记录** (`objects/billing.go`):
```go
func PostCost(ctx context.Context, meta *Meta, preConsumedQuota int64, actuallyConsumedQuota int64, promptTokens, completionTokens int, logContent string) error {
    // ...
    model.RecordConsumeLog(ctx, meta.UserId, meta.ChannelId, promptTokens, completionTokens, meta.ActualModelName, meta.TokenName, actuallyConsumedQuota, logContent, meta.SessionId)  // ← 传递 SessionId
    return nil
}
```

---

### 4️⃣ Moderations（内容审核）
- **路由**: `POST /v1/moderations`
- **Handler**: `controller.RelayTextHelper`
- **支持状态**: ✅ 完全支持
- **UUID 验证**: ✅ 强制 UUID v4 格式
- **日志记录**: ✅ 记录到数据库

---

### 5️⃣ Edits（文本编辑）
- **路由**: `POST /v1/edits`
- **Handler**: `controller.RelayTextHelper`
- **支持状态**: ✅ 完全支持
- **UUID 验证**: ✅ 强制 UUID v4 格式
- **日志记录**: ✅ 记录到数据库

---

## 🎯 使用方法

### Chat/Completions/Embeddings 接口

**OpenAI SDK (Python)**:
```python
from openai import OpenAI

client = OpenAI(
    api_key="sk-xxx",
    base_url="https://your-proxy.com/v1",
    default_headers={
        "X-Session-ID": "880cf795-d73e-4319-a9cc-65f15e14b040"  # 标准 UUID v4
    }
)

# Chat Completions
response = client.chat.completions.create(
    model="gpt-4",
    messages=[{"role": "user", "content": "Hello"}]
)

# Embeddings - 同样支持！
embedding = client.embeddings.create(
    model="text-embedding-ada-002",
    input="The food was delicious and the waiter..."
)
```

**cURL**:
```bash
# Chat Completions
curl https://your-proxy.com/v1/chat/completions \
  -H "Authorization: Bearer sk-xxx" \
  -H "X-Session-ID: 880cf795-d73e-4319-a9cc-65f15e14b040" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "Hello"}]
  }'

# Embeddings - 完全支持！
curl https://your-proxy.com/v1/embeddings \
  -H "Authorization: Bearer sk-xxx" \
  -H "X-Session-ID: 880cf795-d73e-4319-a9cc-65f15e14b040" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "text-embedding-ada-002",
    "input": "The food was delicious"
  }'
```

---

## 📋 UUID v4 格式要求

### ✅ 支持的格式

| 格式 | 示例 | 说明 |
|------|------|------|
| **标准格式（带连字符）** | `880cf795-d73e-4319-a9cc-65f15e14b040` | 推荐使用 |
| **紧凑格式（无连字符）** | `880cf795d73e4319a9cc65f15e14b040` | 也支持 |

### ❌ 错误示例

```bash
# 错误 1: 非 UUID v4（版本号不是 4）
X-Session-ID: 880cf795-d73e-3319-a9cc-65f15e14b040  # 第3段第一个字符是3，不是4

# 错误 2: 长度不对
X-Session-ID: 880cf795-d73e-4319-a9cc  # 太短

# 错误 3: 随意字符串
X-Session-ID: my-session-123  # 不是UUID格式
```

**错误响应**:
```json
{
  "error": {
    "message": "invalid sessionId: my-session-123, must be UUID v4 format",
    "type": "invalid_session_id",
    "code": "invalid_session_id"
  }
}
```

---

## 🔍 验证逻辑

### UUID v4 验证函数 (`common/utils.go`)

```go
func IsValidUUIDv4(str string) bool {
    // 1. 移除连字符（支持两种格式）
    cleanStr := strings.ReplaceAll(str, "-", "")

    // 2. 检查长度是否为32
    if len(cleanStr) != 32 {
        return false
    }

    // 3. 添加中划线转换为标准 UUID 格式
    uuidStr := cleanStr[0:8] + "-" + cleanStr[8:12] + "-" + cleanStr[12:16] + "-" + cleanStr[16:20] + "-" + cleanStr[20:]

    // 4. 解析并验证
    u, err := uuid.Parse(uuidStr)
    if err != nil {
        return false
    }

    // 5. 验证是否为 v4
    return u.Version() == 4
}
```

### 验证时机

- **位置**: `relay/controller/text.go` 第 29-32 行
- **时机**: 请求进入后，在实际处理之前
- **影响**: 
  - ✅ 如果 `X-Session-ID` 为空 → 不验证，允许通过
  - ✅ 如果 `X-Session-ID` 是有效 UUID v4 → 验证通过
  - ❌ 如果 `X-Session-ID` 是无效格式 → 返回 400 错误

---

## 📊 数据库记录

### 日志表结构 (`logs` 表)

| 字段 | 类型 | 说明 |
|------|------|------|
| `session_id` | VARCHAR(255) | Session ID（可选） |
| `user_id` | INT | 用户 ID |
| `channel_id` | INT | 渠道 ID |
| `model_name` | VARCHAR(255) | 模型名称 |
| `prompt_tokens` | INT | Prompt tokens |
| `completion_tokens` | INT | Completion tokens |
| `quota` | INT | 消费额度 |
| `created_at` | TIMESTAMP | 创建时间 |

### 查询示例

```sql
-- 按 Session ID 查询所有日志
SELECT * FROM logs WHERE session_id = '880cf795-d73e-4319-a9cc-65f15e14b040';

-- 统计某个 Session 的总消费
SELECT SUM(quota) as total_quota FROM logs WHERE session_id = '880cf795-d73e-4319-a9cc-65f15e14b040';

-- 查看某个 Session 使用的模型分布
SELECT model_name, COUNT(*) as count, SUM(quota) as total_quota 
FROM logs 
WHERE session_id = '880cf795-d73e-4319-a9cc-65f15e14b040' 
GROUP BY model_name;
```

---

## ⚠️ 特殊接口（暂不支持）

以下接口**不经过** `RelayTextHelper`，因此**暂不支持** `X-Session-ID`：

| 接口 | 路由 | Handler | 支持状态 |
|------|------|---------|---------|
| Images Generations | `/v1/images/generations` | `RelayImageHelper` | ✅ 已支持 |
| Audio Speech (TTS) | `/v1/audio/speech` | `RelayAudioSpeechHelper` | ✅ 已支持 |
| Audio Transcription (STT) | `/v1/audio/transcriptions` | `RelayAudioHelper` | ✅ 已支持 |
| Audio Translation | `/v1/audio/translations` | `RelayAudioHelper` | ✅ 已支持 |
| Rerank | `/v1/rerank` | `RelayProxyHelper` | ❌ 暂不支持 |
| Proxy | 自定义代理 | `RelayProxyHelper` | ❌ 暂不支持 |

**注**: Image 和 Audio 接口已在之前的修改中添加了 `X-Session-ID` 支持。

---

## 🎉 总结

### ✅ 已支持的接口（无需修改）

1. **Chat Completions** - 聊天对话
2. **Completions** - 文本补全
3. **Embeddings** - 文本嵌入 ✨ **自动支持！**
4. **Moderations** - 内容审核
5. **Edits** - 文本编辑
6. **Images Generations** - 图像生成（已单独实现）
7. **Audio Speech** - 语音合成（已单独实现）
8. **Audio Transcription/Translation** - 语音转录/翻译（已单独实现）

### 🔧 实现原理

所有使用 `RelayTextHelper` 的接口都**自动继承** `X-Session-ID` 支持，因为：

1. ✅ `GetRequestMeta()` 统一读取 `X-Session-ID` header
2. ✅ `RelayTextHelper()` 统一验证 UUID v4 格式
3. ✅ `PostConsumeQuota()` → `PostCost()` → `RecordConsumeLog()` 统一传递 `sessionId`

### 💡 关键优势

- **零修改**: Embedding 接口无需任何代码修改即可支持
- **统一验证**: 所有接口共享同一套 UUID v4 验证逻辑
- **统一记录**: 所有接口日志都包含 `session_id` 字段
- **易于扩展**: 未来新增的 text 类接口自动获得支持

---

## 📚 相关文档

- `SESSION_ID_USAGE.md` - Session ID 使用指南
- `FRONTEND_SESSION_ID_CHANGES.md` - 前端修改说明
- `SESSION_ID_FIX.md` - Header 透传问题修复
- `UUID_VALIDATION_FIX.md` - UUID 验证修复
- `FRONTEND_REMOVE_DETAILS_COLUMN.md` - 前端"详情"列移除

---

**修改日期**: 2025-11-10  
**状态**: ✅ Embeddings 接口已自动支持（无需修改）  
**影响**: 无（现有功能）



