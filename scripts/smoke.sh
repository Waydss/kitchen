#!/bin/sh
set -eu

API_URL="${API_URL:-http://localhost:8080}"
SWAGGER_URL="${SWAGGER_URL:-http://localhost:8082}"
RESTAURANT_ID="10000000-0000-0000-0000-000000000001"

curl -fsS "$API_URL/health/ready" >/dev/null
curl -fsS "$API_URL/api/v1/restaurants" >/dev/null
curl -fsS "$API_URL/api/v1/restaurants/$RESTAURANT_ID" >/dev/null
curl -fsS "$API_URL/api/v1/restaurants/$RESTAURANT_ID/menu" >/dev/null
curl -fsS -X POST "$API_URL/api/v1/orders" \
    -H "Content-Type: application/json" \
    -H "Idempotency-Key: smoke-order-0001" \
    -d '{"customer_id":"50000000-0000-0000-0000-000000000001","restaurant_id":"10000000-0000-0000-0000-000000000001","items":[{"menu_item_id":"30000000-0000-0000-0000-000000000001","quantity":1}]}' >/dev/null
curl -fsS "$SWAGGER_URL/" >/dev/null
curl -fsS "$SWAGGER_URL/kitchen-openapi.yaml" >/dev/null

echo "Smoke test passed"
