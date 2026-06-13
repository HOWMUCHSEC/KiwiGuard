#!/usr/bin/env bash
set -euo pipefail

gateway_url="http://127.0.0.1:18080"
control_url="http://127.0.0.1:18081"
clickhouse_addr="localhost:9000"
clickhouse_database="kiwiguard"
clickhouse_user="kiwiguard"
clickhouse_password="kiwiguard"
model="kiwiguard-dev"
request_id="kg-dev-$(date +%s)"
prompt="Please summarize this KiwiGuard smoke request marker kg-safe-smoke-input."
control_token="${KIWIGUARD_CONTROL_AUTH_TOKEN:-}"
gateway_client_key="${KIWIGUARD_DEV_GATEWAY_CLIENT_KEY:-}"

usage() {
  cat <<'EOF'
Usage: scripts/dev-llm-client.sh [options]

Sends a mock OpenAI-compatible request through KiwiGuard and verifies that
ClickHouse received request and response capture metadata.

Options:
  --gateway-url URL             Gateway URL. Default: http://127.0.0.1:18080
  --control-url URL             Control API URL. Default: http://127.0.0.1:18081
  --clickhouse-addr HOST:PORT   ClickHouse native address. Default: localhost:9000
  --clickhouse-database NAME    ClickHouse database. Default: kiwiguard
  --clickhouse-user USER        ClickHouse username. Default: kiwiguard
  --clickhouse-password VALUE   ClickHouse password. Default: kiwiguard
  --model NAME                  Requested model. Default: kiwiguard-dev
  --request-id VALUE            Request id. Default: kg-dev-<unix time>
  --prompt TEXT                 Prompt text sent to the gateway.
  --control-token TOKEN         Bearer token for protected control APIs. Default: KIWIGUARD_CONTROL_AUTH_TOKEN
  --gateway-client-key KEY      Reuse a gateway client key instead of creating a temporary client.
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
    --clickhouse-addr)
      clickhouse_addr="${2:-}"
      shift 2
      ;;
    --clickhouse-database)
      clickhouse_database="${2:-}"
      shift 2
      ;;
    --clickhouse-user)
      clickhouse_user="${2:-}"
      shift 2
      ;;
    --clickhouse-password)
      clickhouse_password="${2:-}"
      shift 2
      ;;
    --model)
      model="${2:-}"
      shift 2
      ;;
    --request-id)
      request_id="${2:-}"
      shift 2
      ;;
    --prompt)
      prompt="${2:-}"
      shift 2
      ;;
    --control-token)
      control_token="${2:-}"
      shift 2
      ;;
    --gateway-client-key)
      gateway_client_key="${2:-}"
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

if [[ -z "$gateway_url" || -z "$control_url" || -z "$request_id" || -z "$model" ]]; then
  echo "gateway URL, control URL, request id, and model are required" >&2
  exit 2
fi

tmpdir="$(mktemp -d)"
client_id=""
created_client=0
trap 'if [[ "$created_client" == "1" && -n "$client_id" ]]; then control_request "POST" "$control_url/api/gateway-clients/$client_id/revoke" "$tmpdir/revoke-response.json" >/dev/null 2>&1 || true; fi; rm -rf "$tmpdir"' EXIT

create_client_json="$tmpdir/create-client.json"
control_response_json="$tmpdir/control-response.json"
request_json="$tmpdir/request.json"
response_json="$tmpdir/response.json"

control_request() {
  local method="$1"
  local url="$2"
  local output="$3"
  local data_path="${4:-}"

  if [[ -n "$control_token" && -n "$data_path" ]]; then
    curl -sS \
      -o "$output" \
      -w "%{http_code}" \
      -X "$method" "$url" \
      -H "Authorization: Bearer $control_token" \
      -H "Content-Type: application/json" \
      --data-binary "@$data_path"
    return
  fi
  if [[ -n "$control_token" ]]; then
    curl -sS \
      -o "$output" \
      -w "%{http_code}" \
      -X "$method" "$url" \
      -H "Authorization: Bearer $control_token"
    return
  fi
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

if [[ -z "$gateway_client_key" ]]; then
  python3 - "$create_client_json" <<'PY'
import json
import sys

with open(sys.argv[1], "w", encoding="utf-8") as f:
    json.dump({
        "name": "Dev Client Smoke",
        "notes": "Created by scripts/dev-llm-client.sh",
    }, f)
PY

  http_code="$(control_request "POST" "$control_url/api/gateway-clients" "$control_response_json" "$create_client_json")"
  if [[ "$http_code" != "201" ]]; then
    echo "create gateway client returned HTTP $http_code, want 201" >&2
    cat "$control_response_json" >&2
    exit 1
  fi
  read -r client_id gateway_client_key < <(
    python3 - "$control_response_json" <<'PY'
import json
import sys

with open(sys.argv[1], encoding="utf-8") as f:
    body = json.load(f)
client_id = body.get("client", {}).get("id", "")
key = body.get("key", "")
if not client_id or not key:
    raise SystemExit("create gateway client response missing client.id or key")
print(client_id, key)
PY
  )
  created_client=1
fi

python3 - "$request_json" "$model" "$prompt" <<'PY'
import json
import sys

path, model, prompt = sys.argv[1], sys.argv[2], sys.argv[3]
with open(path, "w", encoding="utf-8") as f:
    json.dump({
        "model": model,
        "messages": [
            {"role": "system", "content": "You are a local KiwiGuard smoke test."},
            {"role": "user", "content": prompt},
        ],
        "stream": False,
    }, f)
PY

http_code="$(
  curl -sS \
    -o "$response_json" \
    -w "%{http_code}" \
    -X POST "$gateway_url/v1/chat/completions" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer $gateway_client_key" \
    -H "X-Request-ID: $request_id" \
    --data-binary "@$request_json"
)"

if [[ "$http_code" != "200" ]]; then
  echo "gateway returned HTTP $http_code" >&2
  cat "$response_json" >&2
  exit 1
fi

if ! grep -q "mock llm captured input" "$response_json"; then
  echo "gateway response did not come from the mock LLM API" >&2
  cat "$response_json" >&2
  exit 1
fi

query="
select
    count(),
    countIf(request_hash != ''),
    countIf(response_hash != ''),
    countIf(upstream_status = 200),
    countIf(gateway_status = 200),
    countIf(direction = 'input'),
    countIf(direction = 'output'),
    countIf(position(request_payload, 'kg-safe-smoke-input') > 0),
    countIf(position(response_payload, 'mock llm captured input') > 0)
from ${clickhouse_database}.kiwiguard_traffic_events
where request_id = '${request_id}'
"

for _ in {1..20}; do
  result="$(
    docker compose -f deployments/docker-compose.yml exec -T clickhouse \
      clickhouse-client \
      --host "${clickhouse_addr%:*}" \
      --port "${clickhouse_addr##*:}" \
      --user "$clickhouse_user" \
      --password "$clickhouse_password" \
      --query "$query" 2>/dev/null || true
  )"
  if [[ "$result" =~ ^([0-9]+)[[:space:]]+([0-9]+)[[:space:]]+([0-9]+)[[:space:]]+([0-9]+)[[:space:]]+([0-9]+)[[:space:]]+([0-9]+)[[:space:]]+([0-9]+)[[:space:]]+([0-9]+)[[:space:]]+([0-9]+)$ ]]; then
    total="${BASH_REMATCH[1]}"
    request_hashes="${BASH_REMATCH[2]}"
    response_hashes="${BASH_REMATCH[3]}"
    upstream_successes="${BASH_REMATCH[4]}"
    gateway_successes="${BASH_REMATCH[5]}"
    input_events="${BASH_REMATCH[6]}"
    output_events="${BASH_REMATCH[7]}"
    request_payloads="${BASH_REMATCH[8]}"
    response_payloads="${BASH_REMATCH[9]}"
    if (( total >= 2 && request_hashes >= 1 && response_hashes >= 1 && upstream_successes >= 1 && gateway_successes >= 1 && input_events >= 1 && output_events >= 1 && request_payloads >= 1 && response_payloads >= 1 )); then
      echo "request_id=$request_id"
      echo "gateway_response=$(cat "$response_json")"
      echo "clickhouse_events=$result"
      exit 0
    fi
  fi
  sleep 1
done

echo "ClickHouse did not receive request and response capture metadata for request_id=$request_id" >&2
echo "last query result: ${result:-<empty>}" >&2
exit 1
