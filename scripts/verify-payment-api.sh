#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
source "${SCRIPT_DIR}/lib/api-test.sh"

: "${BASE_URL:=http://127.0.0.1:8080/api/v1}"
: "${BUYER_USERNAME:?set BUYER_USERNAME}"
: "${BUYER_PASSWORD:?set BUYER_PASSWORD}"
: "${TEST_PENDING_ORDER_ID:?set TEST_PENDING_ORDER_ID}"

require_command curl
require_command jq
require_test_mutations
if [[ "${ALLOW_MOCK_PAYMENT_TESTS:-}" != "1" ]]; then
  echo "set ALLOW_MOCK_PAYMENT_TESTS=1 only when payment.mock_enabled is enabled in a test environment" >&2
  exit 1
fi
BASE_URL="${BASE_URL%/}"
TOKEN=$(buyer_login)

created=$(api_request POST "/payments" "$TOKEN" "$(jq -n --argjson order_id "$TEST_PENDING_ORDER_ID" '{order_id:$order_id,pay_channel:"mock",pay_scene:"mock"}')")
assert_success "$created" "create mock payment"
PAYMENT_NO=$(jq -er '.data.payment_no' <<<"$created")
jq -e '.data.status == 1' >/dev/null <<<"$created"

completed=$(api_request POST "/payments/${PAYMENT_NO}/mock-complete" "$TOKEN")
jq -e --arg payment_no "$PAYMENT_NO" '.code == 0 and .data.payment_no == $payment_no and .data.status == 2' >/dev/null <<<"$completed"
completed_again=$(api_request POST "/payments/${PAYMENT_NO}/mock-complete" "$TOKEN")
jq -e --arg payment_no "$PAYMENT_NO" '.code == 0 and .data.payment_no == $payment_no and .data.status == 2' >/dev/null <<<"$completed_again"
detail=$(api_request GET "/payments/${PAYMENT_NO}" "$TOKEN")
jq -e '.code == 0 and .data.status == 2 and (.data.transaction_id | length) > 0' >/dev/null <<<"$detail"

echo "mock payment API verification passed: payment=${PAYMENT_NO}"
