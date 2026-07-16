#!/usr/bin/env bash

set -euo pipefail

: "${BASE_URL:=http://127.0.0.1:8080/api/v1}"
: "${TEST_PRODUCT_ID:?set TEST_PRODUCT_ID}"
: "${READ_REQUESTS:=200}"
: "${READ_CONCURRENCY:=50}"
: "${READ_WARMUP_REQUESTS:=$READ_CONCURRENCY}"

BASE_URL="${BASE_URL%/}"
ROOT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
cd "$ROOT_DIR"

go run ./cmd/bench-http \
  -url "${BASE_URL}/products?page=1&page_size=20" \
  -requests "$READ_REQUESTS" \
  -concurrency "$READ_CONCURRENCY" \
  -warmup-requests "$READ_WARMUP_REQUESTS" \
  -p95-limit 500ms

go run ./cmd/bench-http \
  -url "${BASE_URL}/products/${TEST_PRODUCT_ID}" \
  -requests "$READ_REQUESTS" \
  -concurrency "$READ_CONCURRENCY" \
  -warmup-requests "$READ_WARMUP_REQUESTS" \
  -p95-limit 500ms

if [[ -n "${ORDER_CREATE_BODY_DIR:-}" ]]; then
  : "${BENCH_HTTP_AUTHORIZATION:?set BENCH_HTTP_AUTHORIZATION for order benchmarks}"
  go run ./cmd/bench-http \
    -url "${BASE_URL}/orders" \
    -method POST \
    -body-dir "$ORDER_CREATE_BODY_DIR" \
    -concurrency 10 \
    -p95-limit 1500ms
fi
