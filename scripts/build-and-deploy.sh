#!/bin/bash
# build-and-deploy.sh
#
# 同步源码到远程服务器，服务器端 Docker 完成前端 + 后端全量构建并重启
#
# 用法：
#   bash scripts/build-and-deploy.sh
#
# 可选环境变量：
#   SSH_HOST   远程主机（默认 ubuntu）
#   REMOTE_DIR 远程目录（默认 ~/llm-proxy）

set -e

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
SSH_HOST="${SSH_HOST:-ubuntu}"
REMOTE_DIR="${REMOTE_DIR:-~/llm-proxy}"

cd "$ROOT_DIR"

# ── 1. 同步源码到服务器（排除构建产物、运行时数据）────────────────────────────
echo "🔄 同步代码到 $SSH_HOST:$REMOTE_DIR ..."
rsync -az --progress \
  --exclude='.git' \
  --exclude='data' \
  --exclude='logs' \
  --exclude='upload' \
  --exclude='node_modules' \
  --exclude='web/node_modules' \
  --exclude='internal/webstatic/build' \
  --exclude='*.db' \
  --exclude='*.db-journal' \
  --exclude='.env' \
  "$ROOT_DIR/" "$SSH_HOST:$REMOTE_DIR/"

# ── 2. 服务器：Docker 全量构建（含前端）并重启 ──────────────────────────────
echo "🐳 服务器重建镜像（前端 + 后端）并重启..."
ssh "$SSH_HOST" "cd $REMOTE_DIR && \
  docker compose -p llm-proxy -f docker/docker-compose.prod.yml build llm-proxy && \
  docker compose -p llm-proxy -f docker/docker-compose.prod.yml up -d"

echo ""
echo "✅ 部署完成！服务地址：http://10.229.20.93:3000"
