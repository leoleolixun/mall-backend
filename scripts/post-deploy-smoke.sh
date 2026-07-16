#!/usr/bin/env bash

set -euo pipefail

: "${BASE_URL:=http://127.0.0.1:8080}"
: "${CHECK_METRICS:=true}"
BASE_URL="${BASE_URL%/}"

for command_name in curl grep mktemp; do
  if ! command -v "$command_name" >/dev/null 2>&1; then
    echo "missing required command: ${command_name}" >&2
    exit 1
  fi
done

headers=$(mktemp)
body=$(mktemp)
trap 'rm -f "$headers" "$body"' EXIT

curl -fsS -D "$headers" -o "$body" "${BASE_URL}/health"
grep -Eiq '^X-Request-ID: [0-9a-f-]+\r?$' "$headers"
grep -q '"code":0' "$body"
curl -fsS "${BASE_URL}/docs/openapi.yaml" | grep -q '^openapi:'
curl -fsS "${BASE_URL}/swagger/index.html" | grep -qi 'swagger'

status=$(curl -sS -o "$body" -w '%{http_code}' "${BASE_URL}/api/v1/me")
if [[ "$status" != "401" ]] || ! grep -q '"code":401' "$body"; then
  echo "unauthorized smoke check failed: status=${status} body=$(cat "$body")" >&2
  exit 1
fi

if [[ "$CHECK_METRICS" == "true" ]]; then
  curl -fsS "${BASE_URL}/metrics" | grep -q '^go_mall_http_requests_total'
fi

echo "post-deploy smoke verification passed"
