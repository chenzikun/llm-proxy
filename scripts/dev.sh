#!/bin/bash
# dev.sh — 本地开发构建 + 运行（不走 Docker、不连远程服务器）
#
# 用法：
#   bash scripts/dev.sh                    # 前端 + 后端全量构建，然后前台运行
#   SKIP_FRONTEND=1 bash scripts/dev.sh    # 只重编后端，复用已有前端产物
#
# 可选环境变量：
#   PORT=3000           监听端口
#   ENV_FILE=.env       环境变量文件（godotenv 自动加载，缺失时从 .env.example 复制）
#   SQLITE_PATH=...     SQLite 文件位置（默认 data/one-api.db）
#   BIN=build/llm-proxy 二进制产物路径
#   SKIP_FRONTEND=1     跳过前端构建
#   SKIP_INSTALL=1      跳过 npm install
#
# 前端需要热更新时：先跑本脚本启动后端，再另开终端 `cd web && npm start`
# （web/package.json 已配置 proxy → http://127.0.0.1:3000）

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT_DIR"

PORT="${PORT:-3000}"
ENV_FILE="${ENV_FILE:-.env}"
FRONTEND_BUILD_DIR="internal/webstatic/build"
BIN="${BIN:-build/llm-proxy}"

# ── 0. 依赖检查 ──────────────────────────────────────────────────────────────
command -v go >/dev/null || { echo "❌ 未找到 go，请先安装 Go"; exit 1; }
if [ "${SKIP_FRONTEND:-0}" != "1" ]; then
  command -v npm >/dev/null || { echo "❌ 未找到 npm，请先安装 Node.js"; exit 1; }
fi

# ── 1. 环境变量文件 ──────────────────────────────────────────────────────────
if [ ! -f "$ENV_FILE" ] && [ -f .env.example ]; then
  cp .env.example "$ENV_FILE"
  echo "📝 已从 .env.example 生成 $ENV_FILE"
fi

# ── 2. 前端构建（产物 embed 进 Go 二进制）────────────────────────────────────
if [ "${SKIP_FRONTEND:-0}" = "1" ]; then
  echo "⏭️  跳过前端构建（SKIP_FRONTEND=1）"
else
  echo "🎨 构建前端 web/ → $FRONTEND_BUILD_DIR ..."
  cd web
  if [ ! -d node_modules ] && [ "${SKIP_INSTALL:-0}" != "1" ]; then
    echo "📦 安装前端依赖 ..."
    npm install
  fi
  # package.json 的 build 脚本已自带 rm -rf ../internal/webstatic/build && mv build
  npm run build
  cd "$ROOT_DIR"
fi

# go:embed all:build 要求目录必须存在，缺失时补占位页避免编译失败
if [ ! -d "$FRONTEND_BUILD_DIR" ]; then
  echo "⚠️  $FRONTEND_BUILD_DIR 不存在，写入占位页以免 go:embed 报错"
  mkdir -p "$FRONTEND_BUILD_DIR"
  printf '<!doctype html><meta charset="utf-8"><title>llm-proxy</title><p>前端未构建，请在 web/ 下执行 npm run build</p>\n' \
    > "$FRONTEND_BUILD_DIR/index.html"
fi

# ── 3. 后端编译（CGO 供 sqlite 驱动使用）────────────────────────────────────
echo "🔨 编译后端 → $BIN ..."
mkdir -p "$(dirname "$BIN")"
CGO_ENABLED=1 go build -trimpath \
  -ldflags "-X 'github.com/zicorn/llm-proxy/pkg/common.Version=$(cat VERSION)'" \
  -o "$BIN" ./cmd/server

# ── 4. 运行 ─────────────────────────────────────────────────────────────────
mkdir -p data logs
export PORT
export SQLITE_PATH="${SQLITE_PATH:-$ROOT_DIR/data/one-api.db}"

if lsof -ti :"$PORT" >/dev/null 2>&1; then
  echo "⚠️  端口 $PORT 已被占用（PID: $(lsof -ti :"$PORT" | tr '\n' ' ')），请先停掉再启动"
  exit 1
fi

echo "🚀 启动 http://localhost:$PORT"
echo "   SQLite: $SQLITE_PATH"
echo "   日志:   $ROOT_DIR/logs"
echo ""
exec "$BIN" --log-dir "$ROOT_DIR/logs"
