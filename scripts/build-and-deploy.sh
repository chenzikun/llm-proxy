#!/bin/bash
# build-and-deploy.sh
#
# 一键本地构建前端 + 同步 + 服务器重建 Go 镜像 + 重启
#
# 用法：
#   bash scripts/build-and-deploy.sh
#
# 可选环境变量：
#   SSH_HOST   远程主机（默认 ubuntu）
#   REMOTE_DIR 远程目录（默认 ~/llm-proxy）
#   SKIP_FRONTEND  设为 1 时跳过前端构建（只部署 Go 变更）

set -e

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
SSH_HOST="${SSH_HOST:-ubuntu}"
REMOTE_DIR="${REMOTE_DIR:-~/llm-proxy}"

cd "$ROOT_DIR"

# ── 1. 构建前端 ──────────────────────────────────────────────────────────────
if [ "${SKIP_FRONTEND}" != "1" ]; then
  echo "📦 构建前端..."
  cd web
  [ ! -d node_modules ] && npm install
  npm run build
  cd "$ROOT_DIR"
else
  echo "⏭️  跳过前端构建（SKIP_FRONTEND=1）"
fi

# ── 2. 同步到服务器（显式包含 web/build，忽略 node_modules / .git 等）────────
echo "🔄 同步代码到 $SSH_HOST:$REMOTE_DIR ..."
rsync -az --progress \
  --exclude='.git' \
  --exclude='data' \
  --exclude='logs' \
  --exclude='upload' \
  --exclude='node_modules' \
  --exclude='web/node_modules' \
  --exclude='*.db' \
  --exclude='*.db-journal' \
  --exclude='.env' \
  "$ROOT_DIR/" "$SSH_HOST:$REMOTE_DIR/"

# ── 3. 服务器：重建 Go 镜像并重启 ────────────────────────────────────────────
echo "🐳 服务器重建镜像并重启..."
ssh "$SSH_HOST" "cd $REMOTE_DIR && \
  docker compose -p llm-proxy -f docker/docker-compose.prod.yml build llm-proxy && \
  docker compose -p llm-proxy -f docker/docker-compose.prod.yml up -d"

echo ""
echo "✅ 部署完成！服务地址：http://10.229.20.93:3000"
