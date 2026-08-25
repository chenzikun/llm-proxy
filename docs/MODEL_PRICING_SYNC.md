# 模型价格同步 - 快速启动指南

## 📁 相关文件

- **sync_model_pricing.py** - 基础版同步脚本（一键同步）
- **sync_model_pricing_advanced.py** - 增强版同步脚本（支持命令行参数）
- **test_environments.py** - 环境连接测试脚本
- **SYNC_PRICING_README.md** - 详细使用文档

## ⚡ 快速开始

### 1️⃣ 安装依赖

```bash
pip install requests urllib3
```

### 2️⃣ 测试环境（推荐首次运行）

```bash
python test_environments.py
```

### 3️⃣ 查看差异（干运行模式）

```bash
python sync_model_pricing_advanced.py --dry-run --report pricing_diff.csv
```

### 4️⃣ 执行同步

```bash
# 一键同步所有环境
python sync_model_pricing.py

# 或使用增强版脚本
python sync_model_pricing_advanced.py
```

## 🎯 常用命令

```bash
# 列出所有可用环境
python sync_model_pricing_advanced.py --list-envs

# 只同步到开发环境
python sync_model_pricing_advanced.py --env dev

# 只同步特定模型
python sync_model_pricing_advanced.py --models gpt-4o,gpt-4o-mini

# 查看帮助
python sync_model_pricing_advanced.py --help
```

## 📖 详细文档

查看 **SYNC_PRICING_README.md** 了解：
- 详细的功能说明
- 所有命令行参数
- 使用场景示例
- 故障排查指南
- 最佳实践

## 🔑 同步规则

✅ **会同步**：
- 同名模型的 `model_ratio`（输入价格）
- 同名模型的 `completion_ratio`（输出价格）

❌ **不会同步**：
- 目标环境不存在的模型（会跳过）
- 价格已经一致的模型（会跳过）
- 模型的其他属性（状态、渠道类型等）

## 🌍 环境配置

**源环境**：美国生产 (eneprodus)

**目标环境**：
- 中国开发 (dev)
- 中国测试 (enetest)
- 中国生产 (eneprod)
- 美西测试 (enetestus)
- 加拿大OCI (eneprodca)
- 欧洲生产 (eneprodeu)
- 新能源亚太 (eneprodap)

## ⚠️ 重要提示

- 脚本需要管理员权限（root账号）
- 建议先运行干运行模式查看差异
- 生产环境更新前做好备份
- 脚本支持幂等运行（可重复执行）

---

如有问题，请查看详细文档或联系开发团队。


