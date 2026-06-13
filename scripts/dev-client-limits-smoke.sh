#!/usr/bin/env bash
set -euo pipefail

gateway_url="http://127.0.0.1:18080"
control_url="http://127.0.0.1:18081"
route_key="chat-completions"
model="kiwiguard-dev"
request_id_prefix="kg-client-limits-$(date +%s)"
prompt="Client limits smoke request marker kg-safe-client-limits."

usage() {
  cat <<'EOF'
Usage: scripts/dev-client-limits-smoke.sh [options]

Creates a gateway client, configures route and client-specific limits, verifies
authorized traffic, rate limiting, override behavior, and revoke enforcement.

Run this while `make dev-env` is active in another terminal.

Options:
  --gateway-url URL             Gateway URL. Default: http://127.0.0.1:18080
  --control-url URL             Control API URL. Default: http://127.0.0.1:18081
  --route-key KEY               Route key to limit. Default: chat-completions
  --model NAME                  Requested model. Default: kiwiguard-dev
  --request-id-prefix VALUE     Request id prefix. Default: kg-client-limits-<unix time>
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --gateway-url)
      gateway_url="${2:-}"
      shift 2
      ;;
    --control-url)
      control_url="${2:-}"
      shift 2
      ;;
    --route-key)
      route_key="${2:-}"
      shift 2
      ;;
    --model)
      model="${2:-}"
      shift 2
      ;;
    --request-id-prefix)
      request_id_prefix="${2:-}"
      shift 2
      ;;
    --help|-h)
      usage
      exit 0
      ;;
    *)
      echo "unknown argument: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

if [[ -z "$gateway_url" || -z "$control_url" || -z "$route_key" || -z "$model" || -z "$request_id_prefix" ]]; then
  echo "gateway URL, control URL, route key, model, and request id prefix are required" >&2
  exit 2
fi

tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT

create_client_json="$tmpdir/create-client.json"
route_limit_json="$tmpdir/route-limit.json"
client_limit_json="$tmpdir/client-limit.json"
active_config_json="$tmpdir/active-config.json"
activate_json="$tmpdir/activate.json"
control_response_json="$tmpdir/control-response.json"
request_json="$tmpdir/request.json"
response_json="$tmpdir/response.json"

fail_with_body() {
  local message="$1"
  local body_path="${2:-}"
  echo "$message" >&2
  if [[ -n "$body_path" && -s "$body_path" ]]; then
    cat "$body_path" >&2
  fi
  exit 1
}

write_limit_json() {
  local path="$1"
  local requests_per_window="$2"
  python3 - "$path" "$requests_per_window" <<'PY'
import json
import sys

path, requests_per_window = sys.argv[1], int(sys.argv[2])
with open(path, "w", encoding="utf-8") as f:
    json.dump({
        "requests_per_window": requests_per_window,
        "window_seconds": 60,
        "max_concurrent_requests": 1,
        "max_body_bytes": 4096,
        "enabled": True,
    }, f)
PY
}

write_gateway_request_json() {
  local path="$1"
  python3 - "$path" "$model" "$prompt" <<'PY'
import json
import sys

path, model, prompt = sys.argv[1], sys.argv[2], sys.argv[3]
with open(path, "w", encoding="utf-8") as f:
    json.dump({
        "model": model,
        "messages": [
            {"role": "system", "content": "You are a local KiwiGuard client limits smoke test."},
            {"role": "user", "content": prompt},
        ],
        "stream": False,
    }, f)
PY
}

control_request() {
  local method="$1"
  local url="$2"
  local output="$3"
  local data_path="${4:-}"

  if [[ -n "$data_path" ]]; then
    curl -sS \
      -o "$output" \
      -w "%{http_code}" \
      -X "$method" "$url" \
      -H "Content-Type: application/json" \
      --data-binary "@$data_path"
    return
  fi

  curl -sS \
    -o "$output" \
    -w "%{http_code}" \
    -X "$method" "$url"
}

require_control_status() {
  local description="$1"
  local expected="$2"
  local method="$3"
  local url="$4"
  local data_path="${5:-}"
  local http_code

  http_code="$(control_request "$method" "$url" "$control_response_json" "$data_path")"
  if [[ "$http_code" != "$expected" ]]; then
    fail_with_body "$description returned HTTP $http_code, want $expected" "$control_response_json"
  fi
}

wait_for_control_health() {
  local body="$tmpdir/control-health.json"
  local http_code=""
  for _ in {1..20}; do
    http_code="$(curl -sS -o "$body" -w "%{http_code}" "$control_url/api/healthz" || true)"
    if [[ "$http_code" == "200" ]]; then
      return 0
    fi
    sleep 1
  done
  fail_with_body "control API health did not become ready; last HTTP ${http_code:-<none>}" "$body"
}

activate_current_policy_bundles() {
  local http_code
  http_code="$(control_request "GET" "$control_url/api/config/active" "$active_config_json")"
  if [[ "$http_code" != "200" ]]; then
    fail_with_body "active config returned HTTP $http_code, want 200" "$active_config_json"
  fi

  python3 - "$active_config_json" "$activate_json" <<'PY'
import json
import sys

active_path, activate_path = sys.argv[1], sys.argv[2]
with open(active_path, encoding="utf-8") as f:
    keys = json.load(f).get("active_policy_bundle_keys", [])
if not isinstance(keys, list):
    raise SystemExit("active_policy_bundle_keys is not a list")
with open(activate_path, "w", encoding="utf-8") as f:
    json.dump({"keys": keys, "reason": "dev client limits smoke"}, f)
PY

  require_control_status "activate policy bundles" "200" "POST" "$control_url/api/policy-bundles/activate" "$activate_json"
  wait_for_control_health
  sleep 1
}

gateway_request() {
  local request_id="$1"
  local output="$2"
  curl -sS \
    -o "$output" \
    -w "%{http_code}" \
    -X POST "$gateway_url/v1/chat/completions" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer $client_key" \
    -H "X-Request-ID: $request_id" \
    --data-binary "@$request_json"
}

gateway_request_without_auth() {
  local request_id="$1"
  local output="$2"
  curl -sS \
    -o "$output" \
    -w "%{http_code}" \
    -X POST "$gateway_url/v1/chat/completions" \
    -H "Content-Type: application/json" \
    -H "X-Request-ID: $request_id" \
    --data-binary "@$request_json"
}

expect_gateway_status() {
  local description="$1"
  local expected="$2"
  local request_id="$3"
  local http_code

  http_code="$(gateway_request "$request_id" "$response_json")"
  if [[ "$http_code" != "$expected" ]]; then
    fail_with_body "$description returned HTTP $http_code, want $expected" "$response_json"
  fi
}

expect_gateway_success() {
  local description="$1"
  local request_id="$2"
  expect_gateway_status "$description" "200" "$request_id"
  if ! grep -q "mock llm captured input" "$response_json"; then
    fail_with_body "$description did not return the mock LLM response" "$response_json"
  fi
}

wait_for_protected_route() {
  local request_id_base="$1"
  local http_code=""

  for attempt in {1..20}; do
    http_code="$(gateway_request_without_auth "${request_id_base}-${attempt}" "$response_json")"
    if [[ "$http_code" == "401" ]]; then
      return 0
    fi
    sleep 1
  done
  fail_with_body "route did not require client authentication; last HTTP ${http_code:-<none>}" "$response_json"
}

wait_for_gateway_status() {
  local description="$1"
  local expected="$2"
  local request_id_base="$3"
  local http_code=""

  for attempt in {1..20}; do
    http_code="$(gateway_request "${request_id_base}-${attempt}" "$response_json")"
    if [[ "$http_code" == "$expected" ]]; then
      return 0
    fi
    sleep 1
  done
  fail_with_body "$description did not return HTTP $expected; last HTTP ${http_code:-<none>}" "$response_json"
}

wait_for_gateway_success() {
  local description="$1"
  local request_id_base="$2"
  wait_for_gateway_status "$description" "200" "$request_id_base"
  if ! grep -q "mock llm captured input" "$response_json"; then
    fail_with_body "$description did not return the mock LLM response" "$response_json"
  fi
}

python3 - "$create_client_json" <<'PY'
import json
import sys

with open(sys.argv[1], "w", encoding="utf-8") as f:
    json.dump({
        "name": "Dev Client Limits Smoke",
        "notes": "Created by scripts/dev-client-limits-smoke.sh",
    }, f)
PY

write_limit_json "$route_limit_json" 1
write_limit_json "$client_limit_json" 3
write_gateway_request_json "$request_json"

wait_for_control_health

create_client_output="$(
  curl -sS \
    -w $'\n%{http_code}' \
    -X POST "$control_url/api/gateway-clients" \
    -H "Content-Type: application/json" \
    --data-binary "@$create_client_json"
)"
http_code="${create_client_output##*$'\n'}"
create_client_body="${create_client_output%$'\n'*}"
if [[ "$http_code" != "201" ]]; then
  echo "create gateway client returned HTTP $http_code, want 201" >&2
  if [[ -n "$create_client_body" ]]; then
    printf '%s\n' "$create_client_body" >&2
  fi
  exit 1
fi

read -r client_id client_key < <(
  printf '%s' "$create_client_body" | python3 -c 'import json, sys; body = json.load(sys.stdin); client_id = body.get("client", {}).get("id", ""); key = body.get("key", "");
if not client_id or not key:
    raise SystemExit("create gateway client response missing client.id or key")
print(client_id, key)'
)
if [[ -z "$client_id" || -z "$client_key" ]]; then
  echo "create gateway client response missing client id or one-time key" >&2
  exit 1
fi

require_control_status "configure route limit" "200" "PUT" "$control_url/api/gateway-limits/routes/$route_key" "$route_limit_json"
activate_current_policy_bundles

wait_for_protected_route "${request_id_prefix}-protected"
expect_gateway_success "first authorized request" "${request_id_prefix}-allowed-1"
expect_gateway_status "second authorized request" "429" "${request_id_prefix}-limited"

require_control_status "configure client route limit override" "200" "PUT" "$control_url/api/gateway-limits/clients/$client_id/routes/$route_key" "$client_limit_json"
activate_current_policy_bundles

wait_for_protected_route "${request_id_prefix}-override-protected"
wait_for_gateway_success "authorized request after client override" "${request_id_prefix}-override"

require_control_status "revoke gateway client" "200" "POST" "$control_url/api/gateway-clients/$client_id/revoke" ""
require_control_status "refresh route limit after revoke" "200" "PUT" "$control_url/api/gateway-limits/routes/$route_key" "$route_limit_json"
activate_current_policy_bundles
wait_for_gateway_status "request after revoke" "403" "${request_id_prefix}-revoked"

echo "client_id=$client_id"
echo "route_key=$route_key"
echo "request_id_prefix=$request_id_prefix"
echo "client_limits_smoke=ok"
