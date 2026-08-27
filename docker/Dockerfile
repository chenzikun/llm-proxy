# 前端需预先在本地构建好，运行 scripts/build-and-deploy.sh 一键完成。
# 本 Dockerfile 只负责编译 Go 二进制，不在容器内跑 npm。

FROM golang:alpine AS builder

RUN apk add --no-cache g++

ENV GO111MODULE=on \
    CGO_ENABLED=1 \
    GOOS=linux

WORKDIR /build
ADD go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -trimpath \
    -ldflags "-s -w -X 'github.com/zicorn/llm-proxy/pkg/common.Version=$(cat VERSION)' -extldflags '-static'" \
    -o llm-proxy ./cmd/server

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
