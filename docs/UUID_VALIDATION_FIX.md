# UUID v4 验证修复

## 🐛 问题描述

### 原问题
UUID 验证函数 `IsValidUUIDv4()` **只支持 32 字符**的紧凑格式，不支持标准的 36 字符格式：

```
❌ 被拒绝: 880cf795-d73e-4319-a9cc-65f15e14b040  (36 字符，标准格式)
✅ 被接受: 8948e44366524e6ab09a024e7fd13862  (32 字符，紧凑格式)
```

### 用户反馈
> '880cf795-d73e-4319-a9cc-65f15e14b040' 这种格式不支持，只能是 8948e44366524e6ab09a024e7fd13862 这种格式的，正常吗

**答案**: 不正常！标准的 UUID v4 格式应该是带连字符的。

---

## ✅ 解决方案

### 修改的文件

#### 1. `common/utils.go`

**修改前**:
```go
func IsValidUUIDv4(str string) bool {
	// 1. 检查长度是否为32
	if len(str) != 32 {
		return false  // ❌ 拒绝标准格式
	}

	// 2. 添加中划线转换为标准 UUID 格式
	uuidStr := str[0:8] + "-" + str[8:12] + "-" + str[12:16] + "-" + str[16:20] + "-" + str[20:]

	// 3. 解析并验证
	u, err := uuid.Parse(uuidStr)
	if err != nil {
		return false
	}

	// 4. 验证是否为 v4
	return u.Version() == 4
}
```

**修改后**:
```go
func IsValidUUIDv4(str string) bool {
	// 1. 移除连字符（支持两种格式）
	// 标准格式: 880cf795-d73e-4319-a9cc-65f15e14b040 (36字符)
	// 紧凑格式: 8948e44366524e6ab09a024e7fd13862 (32字符)
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

**还需要添加 import**:
```go
import (
	// ... 其他 imports
	"strings"  // ← 添加这个
)
```

---

#### 2. `relay/controller/text.go`

**修改**:
```go
func RelayTextHelper(c *gin.Context, relayMode int) *objects.ErrorWithStatusCode {
	ctx := c.Request.Context()
	meta := objects.GetRequestMeta(c)
	// 验证 X-Session-ID 格式（必须是 UUID v4）
	if meta.SessionId != "" && !common.IsValidUUIDv4(meta.SessionId) {
		return objects.ErrorWrapper(fmt.Errorf("invalid sessionId: %s, must be UUID v4 format", meta.SessionId), "invalid_session_id", http.StatusBadRequest)
	}
	// get & validate textRequest
	textRequest, err := getAndValidateTextRequest(c, meta.Mode)
	// ...
}
```

**说明**:
- 使用 `meta.SessionId` 而不是重新读取 header（避免重复）
- 更清晰的错误提示信息

---

## ✅ 修复后支持的格式

### 有效的 UUID v4 格式

| 格式 | 示例 | 状态 |
|------|------|------|
| 标准格式（带连字符） | `880cf795-d73e-4319-a9cc-65f15e14b040` | ✅ 支持 |
| 紧凑格式（不带连字符） | `8948e44366524e6ab09a024e7fd13862` | ✅ 支持 |
| 标准格式（大写） | `550E8400-E29B-41D4-A716-446655440000` | ✅ 支持 |
| 紧凑格式（大写） | `550E8400E29B41D4A716446655440000` | ✅ 支持 |

### 无效的格式

| 格式 | 示例 | 原因 |
|------|------|------|
| UUID v1 | `6ba7b810-9dad-11d1-80b4-00c04fd430c8` | ❌ 不是 v4 |
| UUID v5 | `74738ff5-5367-5958-9aee-98fffdcd1876` | ❌ 不是 v4 |
| 长度不足 | `880cf795-d73e-4319` | ❌ 长度错误 |
| 随机字符串 | `test123` | ❌ 不是 UUID |
| 非十六进制 | `880cf795-d73e-4319-a9cc-65f15e14b04g` | ❌ 包含 'g' |

---

## 🧪 测试用例

已创建测试文件 `common/utils_test.go` 包含以下测试：

```go
func TestIsValidUUIDv4(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		// ✅ 有效格式
		{"标准格式 UUID v4", "880cf795-d73e-4319-a9cc-65f15e14b040", true},
		{"紧凑格式 UUID v4", "8948e44366524e6ab09a024e7fd13862", true},
		{"另一个标准格式", "550e8400-e29b-41d4-a716-446655440000", true},
		{"另一个紧凑格式", "550e8400e29b41d4a716446655440000", true},
		
		// ❌ 无效格式
		{"UUID v1", "6ba7b810-9dad-11d1-80b4-00c04fd430c8", false},
		{"长度不足", "880cf795-d73e-4319-a9cc", false},
		{"非十六进制", "880cf795-d73e-4319-a9cc-65f15e14b04g", false},
		{"空字符串", "", false},
		{"随机字符串", "test123", false},
	}
	// ... 测试代码
}
```

**运行测试**:
```bash
go test -v ./common -run TestIsValidUUIDv4
```

---

## 📝 使用示例

### Python SDK

```python
from openai import OpenAI
import uuid

# 方式 1: 标准格式（推荐）
session_id = str(uuid.uuid4())  # 自动生成带连字符的格式
# 例如: '880cf795-d73e-4319-a9cc-65f15e14b040'

client = OpenAI(
    api_key="your-key",
    base_url="http://your-proxy",
    default_headers={"X-Session-ID": session_id}
)

# 方式 2: 紧凑格式（也支持）
session_id = uuid.uuid4().hex  # 不带连字符
# 例如: '8948e44366524e6ab09a024e7fd13862'

client = OpenAI(
    api_key="your-key",
    base_url="http://your-proxy",
    default_headers={"X-Session-ID": session_id}
)

response = client.chat.completions.create(
    model="gpt-4",
    messages=[{"role": "user", "content": "Hello"}]
)
```

### cURL

```bash
# 标准格式（带连字符）
curl -X POST http://your-proxy/v1/chat/completions \
  -H "Authorization: Bearer your-api-key" \
  -H "X-Session-ID: 880cf795-d73e-4319-a9cc-65f15e14b040" \
  -H "Content-Type: application/json" \
  -d '{"model": "gpt-4", "messages": [{"role": "user", "content": "Hello"}]}'

# 紧凑格式（不带连字符）
curl -X POST http://your-proxy/v1/chat/completions \
  -H "Authorization: Bearer your-api-key" \
  -H "X-Session-ID: 8948e44366524e6ab09a024e7fd13862" \
  -H "Content-Type: application/json" \
  -d '{"model": "gpt-4", "messages": [{"role": "user", "content": "Hello"}]}'
```

### JavaScript

```javascript
import OpenAI from 'openai';
import { v4 as uuidv4 } from 'uuid';

// 标准格式
const sessionId = uuidv4();  // 自动生成带连字符
// 例如: '880cf795-d73e-4319-a9cc-65f15e14b040'

const openai = new OpenAI({
  apiKey: 'your-key',
  baseURL: 'http://your-proxy',
  defaultHeaders: {
    'X-Session-ID': sessionId
  }
});

const response = await openai.chat.completions.create({
  model: 'gpt-4',
  messages: [{ role: 'user', content: 'Hello' }]
});
```

---

## 🔍 错误处理

### 有效请求
```
X-Session-ID: 880cf795-d73e-4319-a9cc-65f15e14b040
→ ✅ 验证通过，记录到日志
```

### 无效请求
```
X-Session-ID: test123
→ ❌ HTTP 400 Bad Request
{
  "error": {
    "message": "invalid sessionId: test123, must be UUID v4 format",
    "type": "invalid_session_id"
  }
}
```

---

## 📊 修改摘要

| 修改项 | 文件 | 说明 |
|-------|------|------|
| UUID 验证逻辑 | `common/utils.go` | 支持带/不带连字符两种格式 |
| 添加 import | `common/utils.go` | 导入 `strings` 包 |
| 验证调用 | `relay/controller/text.go` | 使用 `meta.SessionId` |
| 单元测试 | `common/utils_test.go` | 覆盖各种场景 |

**总计**: 3个文件修改，1个文件新增

---

## ⚠️ 注意事项

### 1. UUID 版本要求
只接受 **UUID v4**（随机生成的 UUID），不接受其他版本：
- ❌ UUID v1 (基于时间戳)
- ❌ UUID v3 (基于 MD5 哈希)
- ✅ UUID v4 (完全随机)
- ❌ UUID v5 (基于 SHA1 哈希)

### 2. 大小写不敏感
UUID 的十六进制字符不区分大小写：
- ✅ `880cf795-d73e-4319-a9cc-65f15e14b040` (小写)
- ✅ `880CF795-D73E-4319-A9CC-65F15E14B040` (大写)
- ✅ `880Cf795-D73e-4319-A9cC-65f15E14b040` (混合)

### 3. 空值处理
`X-Session-ID` header 是**可选的**：
- ✅ 不提供 header → 通过验证，session_id 为空
- ✅ 提供有效 UUID → 通过验证，记录到日志
- ❌ 提供无效格式 → 返回 400 错误

---

## 🎯 验证步骤

### 1. 检查代码修改
```bash
git diff common/utils.go
git diff relay/controller/text.go
```

### 2. 运行测试
```bash
go test -v ./common -run TestIsValidUUIDv4
```

### 3. 编译检查
```bash
go build -o /tmp/test-build
```

### 4. 实际测试
```bash
# 测试标准格式
curl -X POST http://localhost:3000/v1/chat/completions \
  -H "Authorization: Bearer sk-xxx" \
  -H "X-Session-ID: 880cf795-d73e-4319-a9cc-65f15e14b040" \
  -H "Content-Type: application/json" \
  -d '{"model": "gpt-4", "messages": [{"role": "user", "content": "test"}]}'

# 测试紧凑格式
curl -X POST http://localhost:3000/v1/chat/completions \
  -H "Authorization: Bearer sk-xxx" \
  -H "X-Session-ID: 8948e44366524e6ab09a024e7fd13862" \
  -H "Content-Type: application/json" \
  -d '{"model": "gpt-4", "messages": [{"role": "user", "content": "test"}]}'

# 测试无效格式（应该返回 400）
curl -X POST http://localhost:3000/v1/chat/completions \
  -H "Authorization: Bearer sk-xxx" \
  -H "X-Session-ID: test123" \
  -H "Content-Type: application/json" \
  -d '{"model": "gpt-4", "messages": [{"role": "user", "content": "test"}]}'
```

---

## 🎉 总结

修复完成后：
1. ✅ **标准兼容**: 支持带连字符的标准 UUID v4 格式
2. ✅ **向后兼容**: 仍然支持紧凑格式（不带连字符）
3. ✅ **格式严格**: 只接受 UUID v4，确保数据质量
4. ✅ **错误清晰**: 提供明确的错误信息
5. ✅ **测试完整**: 包含全面的单元测试

**修复日期**: 2025-11-10  
**状态**: ✅ 已修复  
**影响**: 中（改进 UUID 格式兼容性）




