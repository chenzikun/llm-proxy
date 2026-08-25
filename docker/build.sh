#!/usr/bin/env bash
# docker rmi llm-proxy-frontend:1.0
docker build -f ./docker/Dockerfile-frontend -t llm-proxy-frontend:1.0 .
docker build -f ./docker/Dockerfile-backend -t harbor-energy.auteltech.cn/dev/llm-proxy:1.0 .
