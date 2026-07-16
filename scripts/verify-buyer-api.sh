#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
source "${SCRIPT_DIR}/lib/api-test.sh"

: "${BASE_URL:=http://127.0.0.1:8080/api/v1}"
: "${BUYER_USERNAME:?set BUYER_USERNAME}"
: "${BUYER_PASSWORD:?set BUYER_PASSWORD}"
: "${TEST_ADDRESS_ID:?set TEST_ADDRESS_ID}"
: "${TEST_SKU_ID:?set TEST_SKU_ID}"
: "${TEST_QUANTITY:=1}"

require_command curl
require_command jq
require_test_mutations
BASE_URL="${BASE_URL%/}"

TOKEN=$(buyer_login)
ORDER_ID=""

cleanup() {
  set +e
  if [[ -n "$ORDER_ID" ]]; then
    api_request POST "/orders/${ORDER_ID}/cancel" "$TOKEN" >/dev/null
  fi
  api_request DELETE "/cart/items/${TEST_SKU_ID}" "$TOKEN" >/dev/null
}
trap cleanup EXIT

cart_response=$(api_request POST "/cart/items" "$TOKEN" "$(jq -n --argjson sku_id "$TEST_SKU_ID" --argjson quantity "$TEST_QUANTITY" '{sku_id:$sku_id,quantity:$quantity}')")
assert_success "$cart_response" "add cart item"
cart_list=$(api_request GET "/cart/items" "$TOKEN")
jq -e --argjson sku_id "$TEST_SKU_ID" --argjson quantity "$TEST_QUANTITY" \
  '.code == 0 and any(.data[]; .sku_id == $sku_id and .quantity >= $quantity and .available == true)' >/dev/null <<<"$cart_list"

items=$(jq -n --argjson sku_id "$TEST_SKU_ID" --argjson quantity "$TEST_QUANTITY" '[{sku_id:$sku_id,quantity:$quantity}]')
preview_body=$(jq -n --argjson address_id "$TEST_ADDRESS_ID" --argjson items "$items" '{address_id:$address_id,items:$items}')
preview=$(api_request POST "/orders/preview" "$TOKEN" "$preview_body")
assert_success "$preview" "preview order"
idempotency_token=$(jq -er '.data.idempotency_token' <<<"$preview")

run_id="API-ACCEPTANCE-$(date +%s)"
create_body=$(jq -n \
  --argjson address_id "$TEST_ADDRESS_ID" \
  --argjson items "$items" \
  --arg token "$idempotency_token" \
  --arg remark "$run_id" \
  '{address_id:$address_id,items:$items,idempotency_token:$token,remark:$remark}')
created=$(api_request POST "/orders" "$TOKEN" "$create_body")
assert_success "$created" "create order"
ORDER_ID=$(jq -er '.data.id' <<<"$created")
ORDER_NO=$(jq -er '.data.order_no' <<<"$created")

duplicate=$(api_request POST "/orders" "$TOKEN" "$create_body")
jq -e --argjson order_id "$ORDER_ID" '.code == 0 and .data.id == $order_id' >/dev/null <<<"$duplicate"

detail=$(api_request GET "/orders/${ORDER_ID}" "$TOKEN")
jq -e --arg order_no "$ORDER_NO" --argjson sku_id "$TEST_SKU_ID" \
  '.code == 0 and .data.order_no == $order_no and any(.data.items[]; .sku_id == $sku_id)' >/dev/null <<<"$detail"
list=$(api_request GET "/orders?page=1&page_size=20" "$TOKEN")
jq -e --argjson order_id "$ORDER_ID" '.code == 0 and any(.data.list[]; .id == $order_id)' >/dev/null <<<"$list"

echo "buyer API verification passed: order=${ORDER_NO}"
