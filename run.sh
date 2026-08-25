#!/bin/bash
HOST=`ifconfig | grep 'inet ' | grep -v -E '127.*|172.*' | awk '{print $2}'`

# 请先build镜像：bash docker/build.sh
docker stop llm-proxy
docker rm llm-proxy
docker run -d --name=llm-proxy  -p 3000:3000 \
    -e "SQL_DSN=root:root@tcp(10.240.3.251:13306)/llm_proxy"  \
    -e "SERVER_ADDRESS=http://10.240.1.177:3000"     \
    -e "SESSION_SECRET=Autel@123"  \
    -e "TZ=Asia/Shanghai" \
    -e "REDIS_MODE=single" \
    -e "REDIS_SERVER=10.240.1.171:26379"   \
    harbor-energy.auteltech.cn/dev/llm-proxy:1.0