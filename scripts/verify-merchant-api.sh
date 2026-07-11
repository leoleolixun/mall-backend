#!/usr/bin/env bash

set -euo pipefail

: "${BASE_URL:=http://127.0.0.1:8080/api/v1}"
: "${MERCHANT_USERNAME:?set MERCHANT_USERNAME}"
: "${MERCHANT_PASSWORD:?set MERCHANT_PASSWORD}"

for command in curl jq; do
  if ! command -v "$command" >/dev/null 2>&1; then
    echo "missing required command: $command" >&2
    exit 1
  fi
done

BASE_URL="${BASE_URL%/}"
TOKEN=""
CATEGORY_ID=""
PRODUCT_ID=""
SKU_ID=""

request() {
  local method="$1"
  local path="$2"
  local body="${3:-}"
  local args=(-sS -X "$method" "${BASE_URL}${path}" -H "Authorization: Bearer ${TOKEN}")
  if [[ -n "$body" ]]; then
    args+=(-H "Content-Type: application/json" --data "$body")
  fi
  curl "${args[@]}"
}

assert_success() {
  local response="$1"
  local action="$2"
  if ! jq -e '.code == 0' >/dev/null <<<"$response"; then
    echo "$action failed: $response" >&2
    exit 1
  fi
}

cleanup() {
  set +e
  if [[ -n "$TOKEN" && -n "$PRODUCT_ID" ]]; then
    request POST "/merchant/products/${PRODUCT_ID}/off-sale" >/dev/null
  fi
  if [[ -n "$TOKEN" && -n "$SKU_ID" && -n "$PRODUCT_ID" ]]; then
    request DELETE "/merchant/products/${PRODUCT_ID}/skus/${SKU_ID}" >/dev/null
  fi
  if [[ -n "$TOKEN" && -n "$PRODUCT_ID" ]]; then
    request DELETE "/merchant/products/${PRODUCT_ID}" >/dev/null
  fi
  if [[ -n "$TOKEN" && -n "$CATEGORY_ID" ]]; then
    request DELETE "/merchant/categories/${CATEGORY_ID}" >/dev/null
  fi
}
trap cleanup EXIT

login_body=$(jq -n --arg username "$MERCHANT_USERNAME" --arg password "$MERCHANT_PASSWORD" '{username:$username,password:$password}')
login_response=$(curl -sS -X POST "${BASE_URL}/merchant/auth/login" -H "Content-Type: application/json" --data "$login_body")
TOKEN=$(jq -er '.data.access_token' <<<"$login_response")
jq -e '.code == 0 and (.data.user.permissions | index("catalog:write")) != null and (.data.user.permissions | index("inventory:write")) != null' \
  >/dev/null <<<"$login_response"

suffix=$(date +%s)
category_response=$(request POST "/merchant/categories" "$(jq -n --arg name "验收分类-${suffix}" '{name:$name,sort:999}')")
assert_success "$category_response" "create category"
CATEGORY_ID=$(jq -er '.data.id' <<<"$category_response")

product_response=$(request POST "/merchant/products" "$(jq -n --argjson category_id "$CATEGORY_ID" --arg name "验收商品-${suffix}" '{category_id:$category_id,name:$name,description:"merchant api verification"}')")
assert_success "$product_response" "create product"
PRODUCT_ID=$(jq -er '.data.id' <<<"$product_response")

on_sale_without_sku=$(request POST "/merchant/products/${PRODUCT_ID}/on-sale")
jq -e '.code == 40000 and (.message | contains("SKU"))' >/dev/null <<<"$on_sale_without_sku"

sku_response=$(request POST "/merchant/products/${PRODUCT_ID}/skus" '{"name":"默认规格","price":19900,"stock":20}')
assert_success "$sku_response" "create SKU"
SKU_ID=$(jq -er '.data.id' <<<"$sku_response")

update_sku_response=$(request PUT "/merchant/products/${PRODUCT_ID}/skus/${SKU_ID}" '{"name":"默认规格","price":19900,"stock":15,"low_stock_threshold":20}')
assert_success "$update_sku_response" "adjust SKU stock"

inventory_response=$(request GET "/merchant/inventory-logs?sku_id=${SKU_ID}&page=1&page_size=10")
jq -e \
  '.code == 0 and any(.data.list[]; .change_type == "merchant_init" and .quantity == 20) and any(.data.list[]; .change_type == "merchant_adjustment" and .quantity == -5 and .before_stock == 20 and .after_stock == 15)' \
  >/dev/null <<<"$inventory_response"

stock_response=$(request PUT "/merchant/inventory/skus/${SKU_ID}/stock" '{"stock":14,"low_stock_threshold":20,"remark":"merchant api verification"}')
jq -e --argjson sku_id "$SKU_ID" \
  '.code == 0 and .data.sku_id == $sku_id and .data.stock == 14 and .data.low_stock_threshold == 20' \
  >/dev/null <<<"$stock_response"

alert_response=$(request GET "/merchant/inventory-alerts?sku_id=${SKU_ID}&page=1&page_size=10")
jq -e --argjson sku_id "$SKU_ID" \
  '.code == 0 and .data.total == 1 and any(.data.list[]; .sku_id == $sku_id and .stock == 14 and .low_stock_threshold == 20 and .severity == "low_stock")' \
  >/dev/null <<<"$alert_response"

dashboard_response=$(request GET "/merchant/dashboard/overview")
jq -e \
  '.code == 0 and .data.total_products >= 1 and .data.low_stock_skus >= 1 and (.data.pending_shipment_orders | type == "number") and (.data.today_paid_amount | type == "number")' \
  >/dev/null <<<"$dashboard_response"

analytics_response=$(request GET "/merchant/dashboard/analytics?days=7&top_limit=10")
jq -e \
  '.code == 0 and (.data.sales_trend | length) == 7 and (.data.top_products | type == "array") and (.data.start_date | type == "string") and (.data.end_date | type == "string")' \
  >/dev/null <<<"$analytics_response"

on_sale_response=$(request POST "/merchant/products/${PRODUCT_ID}/on-sale")
assert_success "$on_sale_response" "put product on sale"

detail_response=$(request GET "/merchant/products/${PRODUCT_ID}")
jq -e --argjson product_id "$PRODUCT_ID" --argjson sku_id "$SKU_ID" \
  '.code == 0 and .data.id == $product_id and .data.status == 1 and any(.data.skus[]; .id == $sku_id)' \
  >/dev/null <<<"$detail_response"

list_response=$(request GET "/merchant/products?page=1&page_size=10&status=1&keyword=${suffix}")
jq -e --argjson product_id "$PRODUCT_ID" '.code == 0 and any(.data.list[]; .id == $product_id)' \
  >/dev/null <<<"$list_response"

delete_last_sku=$(request DELETE "/merchant/products/${PRODUCT_ID}/skus/${SKU_ID}")
jq -e '.code == 40000 and (.message | contains("先下架商品"))' >/dev/null <<<"$delete_last_sku"

off_sale_response=$(request POST "/merchant/products/${PRODUCT_ID}/off-sale")
assert_success "$off_sale_response" "take product off sale"

delete_sku_response=$(request DELETE "/merchant/products/${PRODUCT_ID}/skus/${SKU_ID}")
assert_success "$delete_sku_response" "delete SKU"
SKU_ID=""

delete_product_response=$(request DELETE "/merchant/products/${PRODUCT_ID}")
assert_success "$delete_product_response" "delete product"
PRODUCT_ID=""

delete_category_response=$(request DELETE "/merchant/categories/${CATEGORY_ID}")
assert_success "$delete_category_response" "delete category"
CATEGORY_ID=""

echo "merchant API verification passed"
