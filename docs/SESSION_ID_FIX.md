# X-Session-ID 问题修复

## 🐛 问题描述

用户报告了两个关于 `X-Session-ID` 的问题：

### 问题 1: 过于严格的验证
```
[ERR] relay error happen, status code is 400, won't retry in this case 
[ERR] relay error (channel id 4, user id: 1): invalid sessionId: test123
```

系统要求 `X-Session-ID` 必须是 UUID v4 格式，这太严格了。用户应该能使用任意字符串作为 session ID。

### 问题 2: Header 被透传
`X-Session-ID` 是代理系统内部用于统计会话的 header，不应该被透传到下游的 OpenAI API。

---

## ✅ 修复方案

### 1. 移除 UUID 验证

**文件**: `relay/controller/text.go`

**修改前**:
```go
func RelayTextHelper(c *gin.Context, relayMode int) *objects.ErrorWithStatusCode {
	ctx := c.Request.Context()
	meta := objects.GetRequestMeta(c)
	sessionId := c.GetHeader("x-session-id")
	if sessionId != "" && !common.IsValidUUIDv4(sessionId) {
		return objects.ErrorWrapper(fmt.Errorf("invalid sessionId: %s", sessionId), "invalid_session_id", http.StatusBadRequest)
	}
	// get & validate textRequest
	textRequest, err := getAndValidateTextRequest(c, meta.Mode)
```

**修改后**:
```go
func RelayTextHelper(c *gin.Context, relayMode int) *objects.ErrorWithStatusCode {
	ctx := c.Request.Context()
	meta := objects.GetRequestMeta(c)
	// get & validate textRequest
	textRequest, err := getAndValidateTextRequest(c, meta.Mode)
```

**说明**:
- ✅ 移除了 UUID v4 验证
- ✅ 移除了重复的 sessionId 读取（已在 `GetRequestMeta` 中读取）
- ✅ 现在支持任意字符串作为 session ID

---

### 2. 防止 Header 透传

**文件**: `relay/adaptor/proxy/adaptor.go`

**修改前**:
```go
func (a *ProxyAdaptor) SetupRequestHeader(c *gin.Context, req *http.Request, meta *objects.Meta) error {
	for k, v := range c.Request.Header {
		req.Header.Set(k, v[0])
	}

	// remove unnecessary headers
	req.Header.Del("Host")
	req.Header.Del("Content-Length")
	req.Header.Del("Accept-Encoding")
	req.Header.Del("Connection")

	// set authorization header
	req.Header.Set("Authorization", meta.APIKey)

	return nil
}
```

**修改后**:
```go
func (a *ProxyAdaptor) SetupRequestHeader(c *gin.Context, req *http.Request, meta *objects.Meta) error {
	for k, v := range c.Request.Header {
		req.Header.Set(k, v[0])
	}

	// remove unnecessary headers
	req.Header.Del("Host")
	req.Header.Del("Content-Length")
	req.Header.Del("Accept-Encoding")
	req.Header.Del("Connection")
	req.Header.Del("X-Session-ID") // 内部使用，不透传给下游

	// set authorization header
	req.Header.Set("Authorization", meta.APIKey)

	return nil
}
```

**说明**:
- ✅ 在转发请求到下游 API 前删除 `X-Session-ID` header
- ✅ 防止内部 header 泄露到外部 API

---

## 🔍 为什么只需要修改 proxy adaptor？

系统中有多个 adaptor，但只有 `proxy` adaptor 会复制所有 headers：

### Proxy Adaptor（需要修复）
```go
// 会复制所有 headers
for k, v := range c.Request.Header {
    req.Header.Set(k, v[0])
}
```

### 其他 Adaptors（无需修改）
```go
// 只复制特定的 headers
func SetupCommonRequestHeader(c *gin.Context, req *http.Request, meta *objects.Meta) {
    req.Header.Set("Content-Type", c.Request.Header.Get("Content-Type"))
    req.Header.Set("Accept", c.Request.Header.Get("Accept"))
    // 不会复制 X-Session-ID
}
```

所有其他 adaptors（OpenAI、Anthropic、AWS 等）都使用 `SetupCommonRequestHeader`，只复制必要的 headers，因此不会有透传问题。

---

## ✅ 修复后的行为

### Session ID 验证
- ✅ 支持任意字符串（不限于 UUID）
- ✅ 支持空值（可选参数）
- ✅ 示例有效值：
  - `"test123"` ✅
  - `"user_session_abc"` ✅
  - `"conv-2024-11-10"` ✅
  - `"550e8400-e29b-41d4-a716-446655440000"` ✅（UUID 也可以）

### Header 处理
- ✅ 在代理内部读取并记录到日志
- ✅ 不会被转发到下游 OpenAI API
- ✅ 对下游 API 完全透明

---

## 📝 使用示例

### 客户端使用（任意字符串都可以）

```python
from openai import OpenAI

# 方式 1: 使用自定义字符串
client = OpenAI(
    api_key="your-key",
    base_url="http://your-proxy",
    default_headers={"X-Session-ID": "my-session-123"}  # ✅ 任意字符串
)

# 方式 2: 使用 UUID
client = OpenAI(
    api_key="your-key",
    base_url="http://your-proxy",
    default_headers={"X-Session-ID": "550e8400-e29b-41d4-a716-446655440000"}  # ✅ UUID 也可以
)

# 方式 3: 使用业务 ID
client = OpenAI(
    api_key="your-key",
    base_url="http://your-proxy",
    default_headers={"X-Session-ID": f"user_{user_id}_conv_{conv_id}"}  # ✅ 组合 ID
)

response = client.chat.completions.create(
    model="gpt-4",
    messages=[{"role": "user", "content": "Hello"}]
)
```

### cURL 使用

```bash
# 任意字符串都可以
curl -X POST http://your-proxy/v1/chat/completions \
  -H "Authorization: Bearer your-api-key" \
  -H "X-Session-ID: test123" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "Hello"}]
  }'
```

---

## 🔒 安全性

### 内部使用
- ✅ `X-Session-ID` 只在代理系统内部使用
- ✅ 记录到数据库日志中
- ✅ 用于会话统计和费用追踪

### 不会泄露
- ✅ 不会被转发到 OpenAI API
- ✅ 不会被下游服务看到
- ✅ 完全在代理层面处理

---

## 📊 修改文件清单

| 文件 | 修改类型 | 说明 |
|------|---------|------|
| `relay/controller/text.go` | 删除代码 | 移除 UUID 验证 |
| `relay/adaptor/proxy/adaptor.go` | 添加代码 | 删除 header 防止透传 |

**总计**: 2个文件修改

---

## 🧪 测试验证

### 测试 1: 非 UUID 格式的 session ID
```bash
curl -H "X-Session-ID: test123" ...
# 预期: ✅ 成功，记录到日志
```

### 测试 2: UUID 格式的 session ID
```bash
curl -H "X-Session-ID: 550e8400-e29b-41d4-a716-446655440000" ...
# 预期: ✅ 成功，记录到日志
```

### 测试 3: 不带 session ID
```bash
curl ...  # 不带 X-Session-ID header
# 预期: ✅ 成功，session_id 字段为空
```

### 测试 4: Header 不会透传
- 查看发往下游 API 的请求
- 预期: ✅ 不包含 `X-Session-ID` header

---

## 🎯 总结

修复完成后：

1. ✅ **灵活性提升**: 支持任意字符串作为 session ID
2. ✅ **安全性增强**: Header 不会泄露到下游 API
3. ✅ **向下兼容**: 不影响现有功能
4. ✅ **代码简化**: 移除不必要的验证逻辑

用户现在可以自由使用任何有意义的字符串作为 session ID，用于内部统计和追踪，而不会影响到下游 API 的调用。

---

**修复日期**: 2025-11-10  
**状态**: ✅ 已修复  
**影响**: 低（仅改进验证和安全性）




