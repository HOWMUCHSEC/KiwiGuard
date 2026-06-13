#!/usr/bin/env bash
set -euo pipefail

gateway_url="http://127.0.0.1:18080"
control_url="http://127.0.0.1:18081"
clickhouse_addr="localhost:9000"
clickhouse_database="kiwiguard"
clickhouse_user="kiwiguard"
clickhouse_password="kiwiguard"
model="kiwiguard-dev"
request_id="kg-outage-$(date +%s)"
prompt="ClickHouse outage smoke request marker kg-safe-clickhouse-outage."

usage() {
  cat <<'EOF'
Usage: scripts/dev-clickhouse-outage-smoke.sh [options]

Sends one gateway request while ClickHouse is healthy, immediately stops the
development ClickHouse service before the async event flush, verifies that the
event enters the durable spool, restarts ClickHouse, and verifies replay.

Run this while `make dev-env` is active in another terminal.

Options:
  --gateway-url URL             Gateway URL. Default: http://127.0.0.1:18080
  --control-url URL             Control API URL. Default: http://127.0.0.1:18081
  --clickhouse-addr HOST:PORT   ClickHouse native address. Default: localhost:9000
  --clickhouse-database NAME    ClickHouse database. Default: kiwiguard
  --clickhouse-user USER        ClickHouse username. Default: kiwiguard
  --clickhouse-password VALUE   ClickHouse password. Default: kiwiguard
  --model NAME                  Requested model. Default: kiwiguard-dev
  --request-id VALUE            Request id. Default: kg-outage-<unix time>
  --prompt TEXT                 Prompt text sent to the gateway.
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
clickhouse_stopped=0
trap 'if [[ "$clickhouse_stopped" == "1" ]]; then docker compose -f deployments/docker-compose.yml up -d --wait clickhouse >/dev/null 2>&1 || true; fi; rm -rf "$tmpdir"' EXIT

request_json="$tmpdir/request.json"
response_json="$tmpdir/response.json"
spool_json="$tmpdir/spool.json"

python3 - "$request_json" "$model" "$prompt" <<'PY'
import json
import sys

path, model, prompt = sys.argv[1], sys.argv[2], sys.argv[3]
with open(path, "w", encoding="utf-8") as f:
    json.dump({
        "model": model,
        "messages": [
            {"role": "system", "content": "You are a local KiwiGuard outage smoke test."},
            {"role": "user", "content": prompt},
        ],
        "stream": False,
    }, f)
PY

require_control() {
  curl -sS -o "$spool_json" "$control_url/api/traffic/spool" >/dev/null
}

spool_depth() {
  python3 - "$spool_json" <<'PY'
import json
import sys
with open(sys.argv[1], encoding="utf-8") as f:
    print(int(json.load(f).get("depth", 0)))
PY
}

wait_for_spool_depth() {
  local minimum="$1"
  for _ in {1..20}; do
    require_control
    if (( "$(spool_depth)" >= minimum )); then
      return 0
    fi
    sleep 1
  done
  echo "spool depth did not reach $minimum" >&2
  cat "$spool_json" >&2
  return 1
}

query_replayed_rows() {
  local query="
select count()
from ${clickhouse_database}.kiwiguard_traffic_events
where request_id = '${request_id}'
  and spool_status = 'replayed'
"
  docker compose -f deployments/docker-compose.yml exec -T clickhouse \
    clickhouse-client \
    --host "${clickhouse_addr%:*}" \
    --port "${clickhouse_addr##*:}" \
    --user "$clickhouse_user" \
    --password "$clickhouse_password" \
    --query "$query" 2>/dev/null || true
}

wait_for_replay() {
  local result=""
  for _ in {1..30}; do
    result="$(query_replayed_rows)"
    if [[ "$result" =~ ^[0-9]+$ ]] && (( result >= 1 )); then
      echo "$result"
      return 0
    fi
    sleep 1
  done
  echo "ClickHouse did not receive replayed rows for request_id=$request_id" >&2
  echo "last replay count: ${result:-<empty>}" >&2
  return 1
}

require_control
initial_depth="$(spool_depth)"

http_code="$(
  curl -sS \
    -o "$response_json" \
    -w "%{http_code}" \
    -X POST "$gateway_url/v1/chat/completions" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer dev-token" \
    -H "X-Request-ID: $request_id" \
    --data-binary "@$request_json"
)"

if [[ "$http_code" != "200" ]]; then
  echo "gateway returned HTTP $http_code during ClickHouse outage" >&2
  cat "$response_json" >&2
  exit 1
fi

if ! grep -q "mock llm captured input" "$response_json"; then
  echo "gateway response did not come from the mock LLM API" >&2
  cat "$response_json" >&2
  exit 1
fi

docker compose -f deployments/docker-compose.yml stop clickhouse >/dev/null
clickhouse_stopped=1

wait_for_spool_depth "$((initial_depth + 1))"

docker compose -f deployments/docker-compose.yml up -d --wait clickhouse >/dev/null
clickhouse_stopped=0

replayed_rows="$(wait_for_replay)"

echo "request_id=$request_id"
echo "spool_depth_before=$initial_depth"
echo "spool_depth_during=$(spool_depth)"
echo "replayed_rows=$replayed_rows"
