# ── Stage 1: 构建前端 ────────────────────────────────────────────────────────
FROM node:20-alpine AS frontend

WORKDIR /web
COPY web/package*.json ./
RUN npm install
COPY web/ ./
# 直接 build 到目标目录，不走 mv 逻辑
RUN npx react-scripts build

# ── Stage 2: 编译 Go 二进制 ──────────────────────────────────────────────────
FROM golang:alpine AS builder

RUN apk add --no-cache g++

ENV GO111MODULE=on \
    CGO_ENABLED=1 \
    GOOS=linux

WORKDIR /build
ADD go.mod go.sum ./
RUN go mod download

COPY . .
# 用 Stage 1 构建好的前端替换本地产物（不依赖本地 build/ 是否存在）
COPY --from=frontend /web/build ./internal/webstatic/build

RUN go build -trimpath \
    -ldflags "-s -w -X 'github.com/zicorn/llm-proxy/pkg/common.Version=$(cat VERSION)' -extldflags '-static'" \
    -o llm-proxy ./cmd/server

# ── Stage 3: 最小运行时镜像 ──────────────────────────────────────────────────
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    tzdata \
    curl \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /build/llm-proxy /
EXPOSE 3000
WORKDIR /data
CMD ["/llm-proxy"]
