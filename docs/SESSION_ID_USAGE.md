# Session ID 费用记录功能使用指南

## 📖 概述

本系统已支持通过 **OpenAI 标准的 `default_headers`** 传递 Session ID，用于费用记录、搜索和统计。

**核心优势**：
- ✅ **无需修改 SDK**：完全兼容 OpenAI 标准 SDK
- ✅ **一次配置**：在客户端初始化时设置，所有请求自动带上
- ✅ **支持搜索**：可按 Session ID 搜索和统计费用
- ✅ **索引优化**：数据库已建立索引，查询高效

---

## 🚀 客户端使用方法

### Python SDK

```python
from openai import OpenAI

# 初始化时设置 Session ID
client = OpenAI(
    api_key="your-api-key",
    base_url="http://your-proxy-server/v1",
    default_headers={
        "X-Session-ID": "user_session_12345"  # ← 自定义 Session ID
    }
)

# 之后所有请求都会自动带上 Session ID
response = client.chat.completions.create(
    model="gpt-4",
    messages=[{"role": "user", "content": "Hello"}]
)
```

### Node.js SDK

```javascript
import OpenAI from 'openai';

const client = new OpenAI({
  apiKey: 'your-api-key',
  baseURL: 'http://your-proxy-server/v1',
  defaultHeaders: {
    'X-Session-ID': 'user_session_12345'  // ← 自定义 Session ID
  }
});

// 之后所有请求都会自动带上 Session ID
const response = await client.chat.completions.create({
  model: 'gpt-4',
  messages: [{ role: 'user', content: 'Hello' }]
});
```

### Java SDK

```java
import com.openai.client.OpenAIClient;
import com.openai.client.okhttp.OpenAIOkHttpClient;

Map<String, String> headers = new HashMap<>();
headers.put("X-Session-ID", "user_session_12345");

OpenAIClient client = OpenAIOkHttpClient.builder()
    .apiKey("your-api-key")
    .baseUrl("http://your-proxy-server/v1")
    .defaultHeaders(headers)  // ← 自定义 Session ID
    .build();
```

### cURL 直接调用

```bash
curl -X POST http://your-proxy-server/v1/chat/completions \
  -H "Authorization: Bearer your-api-key" \
  -H "X-Session-ID: user_session_12345" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "Hello"}]
  }'
```

---

## 🔍 查询和统计

### 1. 查询日志（按 Session ID）

**API**: `GET /api/log`

```bash
curl "http://your-proxy-server/api/log?session_id=user_session_12345&p=0" \
  -H "Authorization: Bearer your-admin-token"
```

**查询参数**：
- `session_id`: Session ID（新增）
- `type`: 日志类型（1=充值, 2=消费, 3=管理, 4=系统）
- `start_timestamp`: 开始时间戳
- `end_timestamp`: 结束时间戳
- `model_name`: 模型名称
- `username`: 用户名
- `token_name`: Token 名称
- `channel`: 渠道 ID
- `p`: 页码（从0开始）

**响应示例**：
```json
{
  "success": true,
  "message": "",
  "data": [
    {
      "id": 12345,
      "user_id": 1,
      "created_at": 1699999999,
      "type": 2,
      "content": "模型倍率 1.00，补全倍率 1.00",
      "username": "user1",
      "token_name": "my-token",
      "model_name": "gpt-4",
      "quota": 1000,
      "prompt_tokens": 50,
      "completion_tokens": 100,
      "channel": 1,
      "session_id": "user_session_12345"  // ← Session ID
    }
  ]
}
```

### 2. 统计费用（按 Session ID）

**API**: `GET /api/log/stat`

```bash
curl "http://your-proxy-server/api/log/stat?session_id=user_session_12345&start_timestamp=1699000000&end_timestamp=1700000000" \
  -H "Authorization: Bearer your-admin-token"
```

**响应示例**：
```json
{
  "success": true,
  "message": "",
  "data": {
    "quota": 125000,      // 该 Session 总消费额度
    "token": 5000,        // 总 Token 数
    "logCount": 42        // 请求次数
  }
}
```

### 3. 用户查询自己的日志

**API**: `GET /api/log/self`

```bash
curl "http://your-proxy-server/api/log/self?session_id=user_session_12345&p=0" \
  -H "Authorization: Bearer your-user-token"
```

### 4. 用户统计自己的费用

**API**: `GET /api/log/self/stat`

```bash
curl "http://your-proxy-server/api/log/self/stat?session_id=user_session_12345" \
  -H "Authorization: Bearer your-user-token"
```

---

## 💾 数据库部署

### 执行数据库迁移

```bash
# 根据你的数据库类型执行相应的 SQL

# MySQL
mysql -u your_user -p your_database < bin/migration_add_session_id.sql

# PostgreSQL（需要修改 SQL 文件中的注释）
psql -U your_user -d your_database -f bin/migration_add_session_id.sql

# SQLite（需要修改 SQL 文件中的注释）
sqlite3 your_database.db < bin/migration_add_session_id.sql
```

### 数据库变更内容

- **新增字段**: `logs.session_id` (VARCHAR/TEXT, 默认空字符串)
- **新增索引**: `idx_session_id` (用于快速查询)

---

## 📊 使用场景示例

### 场景1: 按用户会话统计费用

```python
# 为每个用户会话分配唯一 ID
user_id = "user_001"
conversation_id = "conv_20241108_001"
session_id = f"{user_id}_{conversation_id}"

client = OpenAI(
    api_key="key",
    base_url="http://proxy",
    default_headers={"X-Session-ID": session_id}
)

# 整个会话的所有请求都会记录相同的 session_id
# 后续可以按 session_id 统计该会话的总费用
```

### 场景2: 按项目或部门统计

```python
# 为不同项目设置不同的 Session ID
project_id = "project_A"
session_id = f"proj_{project_id}_{timestamp}"

client = OpenAI(
    api_key="key",
    base_url="http://proxy",
    default_headers={"X-Session-ID": session_id}
)
```

### 场景3: 按租户隔离

```python
# SaaS 平台为每个租户设置独立 Session ID
tenant_id = "tenant_12345"
session_id = f"tenant_{tenant_id}"

client = OpenAI(
    api_key="key",
    base_url="http://proxy",
    default_headers={"X-Session-ID": session_id}
)
```

---

## 🎯 最佳实践

1. **Session ID 命名规范**：
   - 使用有意义的前缀：`user_`, `proj_`, `tenant_`
   - 包含时间戳或会话标识
   - 总长度建议不超过 255 字符

2. **安全性**：
   - Session ID 不应包含敏感信息
   - 建议使用 UUID 或类似随机标识符

3. **性能优化**：
   - 已建立数据库索引，查询高效
   - 建议定期清理旧日志

4. **兼容性**：
   - Session ID 是可选的，不传也不影响正常使用
   - 已有代码无需改动，可渐进式升级

---

## 🔧 技术实现

### 服务端处理流程

1. **接收请求**: 从 HTTP Header `X-Session-ID` 读取
2. **存入 Meta**: 通过 `GetRequestMeta()` 自动提取并存储
3. **记录日志**: `RecordConsumeLog()` 将 Session ID 保存到数据库
4. **查询统计**: 所有日志查询和统计接口都支持按 Session ID 过滤

### 核心修改文件

- `objects/relay_meta.go`: Meta 结构体增加 SessionId 字段
- `model/log.go`: Log 模型增加 SessionId 字段和查询支持
- `controller/log.go`: API 接口增加 session_id 查询参数
- `bin/migration_add_session_id.sql`: 数据库迁移脚本

---

## ❓ 常见问题

**Q: 不传 Session ID 会怎样？**
A: 完全不影响，Session ID 字段为空，其他功能正常工作。

**Q: Session ID 有长度限制吗？**
A: 数据库字段为 VARCHAR(255)，建议不超过 255 字符。

**Q: 可以中途修改 Session ID 吗？**
A: 可以，每次创建新的 OpenAI client 时设置新的 Session ID。

**Q: 是否支持批量查询多个 Session ID？**
A: 目前支持单个 Session ID 查询，批量查询可以通过多次调用实现。

---

## 📝 总结

通过使用 OpenAI SDK 标准的 `default_headers` 功能，你可以：
- ✅ **无需修改**任何现有 SDK 代码
- ✅ **自动记录**所有请求的 Session ID
- ✅ **灵活查询**和统计特定会话/项目/租户的费用
- ✅ **完全兼容**现有系统，渐进式升级

部署步骤：
1. 执行数据库迁移 SQL
2. 重启服务
3. 客户端在初始化时添加 `default_headers`
4. 使用 API 查询和统计

**开始使用吧！🎉**

