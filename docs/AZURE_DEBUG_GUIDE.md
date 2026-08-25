# Azure OpenAI 调试指南

## 验证结果

✅ **URL 构建逻辑正确** - 项目代码能够正确构建 Azure OpenAI 的请求 URL

## 可能的差异点

### 1. **API Version 配置问题** ⚠️

**您的脚本:**
```python
api_version="2024-12-01-preview"
```

**项目配置要求:**
- API Version 必须在渠道的 `Config` 字段中配置
- 字段名必须是 `api_version`（小写，下划线）

**解决方案:**
在创建或编辑 Azure 渠道时，Config 字段填写:
```json
{
  "api_version": "2024-12-01-preview"
}
```

### 2. **Base URL 格式问题** ⚠️

**您的脚本:**
```python
azure_endpoint="https://autel-ai.cognitiveservices.azure.com/"
```

**项目配置建议:**
- Base URL: `https://autel-ai.cognitiveservices.azure.com`（不要末尾斜杠）
- 项目会自动处理 URL 拼接，但建议保持一致

### 3. **请求头差异** ✅

项目正确使用了 `api-key` header（Azure 专用），这个是正确的。

### 4. **模型名称中的点号处理** ⚠️

项目代码中有这一行：
```go
model_ = strings.Replace(model_, ".", "", -1)
```

这会移除模型名中的所有点号。例如：
- `gpt-3.5-turbo` → `gpt-35-turbo`
- `gpt-4o` → `gpt-4o`（无变化）

如果您的 Azure deployment 名称包含点号，需要注意这个转换。

### 5. **常见配置错误**

#### a) API Version 未配置
**症状:** 返回 404 或 400 错误
**解决:** 确保 Config 中配置了 `api_version`

#### b) Deployment 名称不匹配
**症状:** 返回 404 "Deployment not found"
**解决:** 
- 确认 Azure 上的 deployment 名称
- 如果使用模型映射，确保映射到正确的 deployment 名称
- 注意点号会被移除

#### c) Base URL 错误
**症状:** 连接超时或 DNS 错误
**解决:** 检查 Base URL 格式是否正确

#### d) API Key 错误
**症状:** 返回 401 Unauthorized
**解决:** 确认 API Key 是否正确复制（注意空格）

## 完整的渠道配置示例

### 基础配置
```
渠道类型: Azure
名称: Azure-GPT4o
Base URL: https://autel-ai.cognitiveservices.azure.com
密钥: [您的 Azure API Key]
模型列表: gpt-4o,gpt-4,gpt-35-turbo
状态: 已启用
```

### Config 配置
```json
{
  "api_version": "2024-12-01-preview"
}
```

### 模型映射（如果需要）
```json
{
  "gpt-4o": "gpt-4o",
  "gpt-4": "gpt-4",
  "gpt-3.5-turbo": "gpt-35-turbo"
}
```

## 调试步骤

### 第一步：使用诊断脚本
运行 `debug_azure.py` 脚本：
```bash
python3 debug_azure.py
```

修改脚本中的配置项：
- `AZURE_API_KEY`: 您的实际 API Key
- `ONE_API_BASE_URL`: 项目地址（如 http://localhost:3000）
- `ONE_API_TOKEN`: 项目中的 token

### 第二步：检查项目日志
项目会记录实际发送的 URL，查看日志输出：
```
fullRequestURL: https://autel-ai.cognitiveservices.azure.com/openai/deployments/gpt-4o/chat/completions?api-version=2024-12-01-preview
```

确认：
- URL 是否正确
- Deployment 名称是否匹配
- API Version 是否正确

### 第三步：对比差异
如果直接访问 Azure 成功，但通过项目失败：

1. **检查请求 URL**
   - 项目日志中的 URL
   - 您脚本中的 URL
   - 是否完全一致？

2. **检查请求头**
   - 项目使用 `api-key` header
   - 您的脚本也使用 `api-key`
   - 值是否相同？

3. **检查请求体**
   - 项目可能会修改请求体
   - 确认 model 参数的值

## 常见错误及解决方案

### 错误 1: "Deployment not found"
```
{
  "error": {
    "code": "DeploymentNotFound",
    "message": "The API deployment for this resource does not exist."
  }
}
```

**原因:** Deployment 名称不匹配

**解决方案:**
1. 在 Azure Portal 确认 deployment 的实际名称
2. 检查项目中的模型映射配置
3. 注意模型名中的点号会被转换为连字符（`gpt-3.5-turbo` → `gpt-35-turbo`）

### 错误 2: "API version not supported"
```
{
  "error": {
    "code": "InvalidApiVersion",
    "message": "The API version is not supported."
  }
}
```

**原因:** API Version 配置错误或未配置

**解决方案:**
- 确保 Config 中配置了 `api_version`
- 使用 Azure 支持的版本号
- 常用版本: `2024-12-01-preview`, `2024-08-01-preview`, `2024-02-01`

### 错误 3: "Unauthorized"
```
{
  "error": {
    "code": "401",
    "message": "Unauthorized"
  }
}
```

**原因:** API Key 错误

**解决方案:**
- 在 Azure Portal 重新复制 API Key
- 检查是否有多余的空格或换行符
- 确认 Key 没有过期或被重新生成

### 错误 4: 连接超时
```
Error: do_request_failed: timeout
```

**原因:** Base URL 错误或网络问题

**解决方案:**
- 检查 Base URL 格式
- 确认网络能访问 Azure
- 检查是否有代理设置干扰

## 获取详细日志

如果需要查看详细的请求日志，可以修改项目日志级别或查看：
- 控制台输出
- 日志文件
- 数据库中的请求记录

关键日志位置：
- `relay/adaptor/common.go:28` - 记录完整的请求 URL
- `relay/controller/text.go` - 记录请求处理流程

## 测试建议

1. **先测试简单请求**
   使用最简单的配置和请求测试，确认基础连接正常

2. **逐步添加复杂功能**
   - 先测试基本的 chat completion
   - 再测试流式响应
   - 最后测试其他功能

3. **对比请求细节**
   使用抓包工具（如 Wireshark, Charles）对比：
   - 直接访问 Azure 的请求
   - 通过项目代理的请求
   - 找出具体差异

## 需要提供的调试信息

如果问题仍未解决，请提供：

1. **错误信息**
   - HTTP 状态码
   - 错误响应内容
   - 错误发生的时间

2. **配置信息**
   - 渠道配置（隐藏敏感信息）
   - API Version
   - Base URL

3. **日志输出**
   - 项目日志中的 `fullRequestURL`
   - 其他相关错误日志

4. **测试脚本结果**
   - `debug_azure.py` 的完整输出
   - 三种方式的测试结果

## 参考链接

- [Azure OpenAI 官方文档](https://learn.microsoft.com/en-us/azure/ai-services/openai/)
- [Azure OpenAI API 参考](https://learn.microsoft.com/en-us/azure/ai-services/openai/reference)
- [One-API 项目文档](https://github.com/songquanpeng/one-api)


