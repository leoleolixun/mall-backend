#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
source "${SCRIPT_DIR}/lib/api-test.sh"

: "${BASE_URL:=http://127.0.0.1:8080/api/v1}"
: "${BUYER_USERNAME:?set BUYER_USERNAME}"
: "${BUYER_PASSWORD:?set BUYER_PASSWORD}"
: "${TEST_PAID_ORDER_ID:?set TEST_PAID_ORDER_ID}"
: "${TEST_PAID_ORDER_ITEM_ID:?set TEST_PAID_ORDER_ITEM_ID}"

require_command curl
require_command jq
require_test_mutations
BASE_URL="${BASE_URL%/}"
TOKEN=$(buyer_login)
AFTER_SALE_ID=""

cleanup() {
  set +e
  if [[ -n "$AFTER_SALE_ID" ]]; then
    api_request POST "/after-sales/${AFTER_SALE_ID}/cancel" "$TOKEN" >/dev/null
  fi
}
trap cleanup EXIT

body=$(jq -n \
  --argjson order_id "$TEST_PAID_ORDER_ID" \
  --argjson order_item_id "$TEST_PAID_ORDER_ITEM_ID" \
  '{order_id:$order_id,order_item_id:$order_item_id,type:"refund_only",reason:"API 自动化验收",description:"测试完成后自动取消",images:[]}')
created=$(api_request POST "/after-sales" "$TOKEN" "$body")
assert_success "$created" "create after-sale"
AFTER_SALE_ID=$(jq -er '.data.id' <<<"$created")
AFTER_SALE_NO=$(jq -er '.data.after_sale_no' <<<"$created")

detail=$(api_request GET "/after-sales/${AFTER_SALE_ID}" "$TOKEN")
jq -e --arg after_sale_no "$AFTER_SALE_NO" '.code == 0 and .data.after_sale_no == $after_sale_no and .data.status == 1' >/dev/null <<<"$detail"
list=$(api_request GET "/after-sales?page=1&page_size=20&status=1" "$TOKEN")
jq -e --argjson id "$AFTER_SALE_ID" '.code == 0 and any(.data.list[]; .id == $id)' >/dev/null <<<"$list"
cancelled=$(api_request POST "/after-sales/${AFTER_SALE_ID}/cancel" "$TOKEN")
assert_success "$cancelled" "cancel after-sale"
AFTER_SALE_ID=""

echo "after-sale API verification passed: after_sale=${AFTER_SALE_NO}"
