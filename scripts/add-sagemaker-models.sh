#!/bin/bash
# add-sagemaker-models.sh
#
# 用途：通过管理 API 批量添加 SageMaker endpoint 模型到 model_meta 表。
# SageMaker endpoint 名称是部署相关的，不应写死在代码中，通过此脚本按需注入。
#
# 使用方法：
#   export API_BASE=http://localhost:3000       # 服务地址
#   export ADMIN_TOKEN=<your_admin_api_key>     # 管理员 API Key
#   bash scripts/add-sagemaker-models.sh
#
# 或者直接指定 endpoint 列表：
#   ENDPOINTS="my-endpoint-001,my-endpoint-002" bash scripts/add-sagemaker-models.sh

set -e

API_BASE="${API_BASE:-http://localhost:3000}"
ADMIN_TOKEN="${ADMIN_TOKEN:?请设置 ADMIN_TOKEN 环境变量}"

# SageMaker endpoint 对应的 channelType = 46
CHANNEL_TYPE=46

# 待添加的 endpoint 列表，逗号分隔；也可通过环境变量传入
ENDPOINTS="${ENDPOINTS:-}"

if [ -z "$ENDPOINTS" ]; then
  echo "❌ 请设置 ENDPOINTS 环境变量，例如："
  echo "   ENDPOINTS=\"app-csms-xxx,app-csms-yyy\" bash $0"
  exit 1
fi

IFS=',' read -ra MODEL_NAMES <<< "$ENDPOINTS"

for MODEL_NAME in "${MODEL_NAMES[@]}"; do
  MODEL_NAME=$(echo "$MODEL_NAME" | xargs)  # trim whitespace
  [ -z "$MODEL_NAME" ] && continue

  echo "➕ 添加模型: $MODEL_NAME (channelType=$CHANNEL_TYPE)"

  HTTP_STATUS=$(curl -s -o /tmp/add_model_resp.json -w "%{http_code}" \
    -X POST "$API_BASE/api/model-meta" \
    -H "Authorization: Bearer $ADMIN_TOKEN" \
    -H "Content-Type: application/json" \
    -d "{
      \"model\": \"$MODEL_NAME\",
      \"channel_type\": $CHANNEL_TYPE,
      \"model_ratio\": 0,
      \"completion_ratio\": 0,
      \"status\": 1
    }")

  RESP=$(cat /tmp/add_model_resp.json)
  if [ "$HTTP_STATUS" = "200" ]; then
    echo "   ✅ 成功: $RESP"
  else
    echo "   ⚠️  HTTP $HTTP_STATUS: $RESP"
  fi
done

echo ""
echo "完成。可通过以下命令验证："
echo "  curl -s '$API_BASE/v1/models' -H 'Authorization: Bearer $ADMIN_TOKEN' | jq '.data[].id' | grep endpoint"
