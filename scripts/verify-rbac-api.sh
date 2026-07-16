#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
source "${SCRIPT_DIR}/lib/api-test.sh"

: "${BASE_URL:=http://127.0.0.1:8080/api/v1}"
: "${SALES_USERNAME:?set SALES_USERNAME}"
: "${SALES_PASSWORD:?set SALES_PASSWORD}"
: "${WAREHOUSE_USERNAME:?set WAREHOUSE_USERNAME}"
: "${WAREHOUSE_PASSWORD:?set WAREHOUSE_PASSWORD}"
: "${TEST_PRODUCT_ID:?set TEST_PRODUCT_ID}"
: "${TEST_SKU_ID:?set TEST_SKU_ID}"

require_command curl
require_command jq
BASE_URL="${BASE_URL%/}"

SALES_TOKEN=$(merchant_login "$SALES_USERNAME" "$SALES_PASSWORD")
WAREHOUSE_TOKEN=$(merchant_login "$WAREHOUSE_USERNAME" "$WAREHOUSE_PASSWORD")

sales_orders=$(api_request GET "/merchant/orders?page=1&page_size=1" "$SALES_TOKEN")
assert_success "$sales_orders" "sales order read"
sales_stock=$(api_request PUT "/merchant/inventory/skus/${TEST_SKU_ID}/stock" "$SALES_TOKEN" '{"stock":1,"low_stock_threshold":1,"remark":"must be rejected"}')
jq -e '.code == 40300' >/dev/null <<<"$sales_stock"

warehouse_inventory=$(api_request GET "/merchant/inventory-alerts?page=1&page_size=1" "$WAREHOUSE_TOKEN")
assert_success "$warehouse_inventory" "warehouse inventory read"
warehouse_product_write=$(api_request PUT "/merchant/products/${TEST_PRODUCT_ID}" "$WAREHOUSE_TOKEN" '{}')
jq -e '.code == 40300' >/dev/null <<<"$warehouse_product_write"
warehouse_accounts=$(api_request GET "/merchant/accounts?page=1&page_size=1" "$WAREHOUSE_TOKEN")
jq -e '.code == 40300' >/dev/null <<<"$warehouse_accounts"

echo "merchant RBAC verification passed"
