#!/usr/bin/env bash

require_command() {
  local command_name="$1"
  if ! command -v "$command_name" >/dev/null 2>&1; then
    echo "missing required command: ${command_name}" >&2
    exit 1
  fi
}

require_test_mutations() {
  if [[ "${ALLOW_TEST_MUTATIONS:-}" != "1" ]]; then
    echo "refusing to mutate data: set ALLOW_TEST_MUTATIONS=1 only in an isolated test environment" >&2
    exit 1
  fi
}

api_request() {
  local method="$1"
  local path="$2"
  local token="${3:-}"
  local body="${4:-}"
  local args=(-sS -X "$method" "${BASE_URL}${path}")
  if [[ -n "$token" ]]; then
    args+=(-H "Authorization: Bearer ${token}")
  fi
  if [[ -n "$body" ]]; then
    args+=(-H "Content-Type: application/json" --data "$body")
  fi
  curl "${args[@]}"
}

assert_success() {
  local payload="$1"
  local action="$2"
  if ! jq -e '.code == 0' >/dev/null <<<"$payload"; then
    echo "${action} failed: ${payload}" >&2
    exit 1
  fi
}

buyer_login() {
  local body response
  body=$(jq -n --arg username "$BUYER_USERNAME" --arg password "$BUYER_PASSWORD" '{username:$username,password:$password}')
  response=$(api_request POST "/auth/login/password" "" "$body")
  assert_success "$response" "buyer login"
  jq -er '.data.access_token' <<<"$response"
}

merchant_login() {
  local username="$1"
  local password="$2"
  local body response
  body=$(jq -n --arg username "$username" --arg password "$password" '{username:$username,password:$password}')
  response=$(api_request POST "/merchant/auth/login" "" "$body")
  assert_success "$response" "merchant login"
  jq -er '.data.access_token' <<<"$response"
}
