#!/usr/bin/env bash
set -euo pipefail

gateway_url="http://127.0.0.1:18080"
control_url="http://127.0.0.1:18081"
postgres_dsn="postgres://kiwiguard:kiwiguard@localhost:5432/kiwiguard?sslmode=disable"
compose_file="deployments/docker-compose.yml"
route_key="chat-completions"
provider_key="dev-openai"
base_url="${KIWIGUARD_BETA_OPENAI_BASE_URL:-}"
model="${KIWIGUARD_BETA_OPENAI_MODEL:-}"
credential_ref="${KIWIGUARD_BETA_OPENAI_CREDENTIAL_REF:-env:KIWIGUARD_BETA_OPENAI_API_KEY}"
control_token="${KIWIGUARD_CONTROL_AUTH_TOKEN:-}"
request_id_prefix="kg-beta-openai-$(date +%s)"

usage() {
  cat <<'EOF'
Usage: scripts/beta-openai-smoke.sh [options]

Configures the active dev provider to use a real OpenAI-compatible upstream via
credential_ref, creates a temporary gateway client, sends one authenticated chat
completion request through KiwiGuard, then revokes the temporary client.

Required environment:
  KIWIGUARD_BETA_OPENAI_BASE_URL       Upstream base URL, for example https://api.openai.com
  KIWIGUARD_BETA_OPENAI_MODEL          Upstream chat model name

Credential environment:
  KIWIGUARD_BETA_OPENAI_CREDENTIAL_REF Credential ref. Default: env:KIWIGUARD_BETA_OPENAI_API_KEY
  KIWIGUARD_BETA_OPENAI_API_KEY        Required when using the default env: ref

The KiwiGuard server process must be started with the same credential source
available to its environment or filesystem.

Options:
  --gateway-url URL             Gateway URL. Default: http://127.0.0.1:18080
  --control-url URL             Control API URL. Default: http://127.0.0.1:18081
  --postgres-dsn DSN            PostgreSQL DSN. Default: local dev DSN
  --compose-file PATH           Docker Compose file for postgres psql. Default: deployments/docker-compose.yml
  --route-key KEY               Route key. Default: chat-completions
  --provider-key KEY            Provider key. Default: dev-openai
  --base-url URL                Overrides KIWIGUARD_BETA_OPENAI_BASE_URL
  --model NAME                  Overrides KIWIGUARD_BETA_OPENAI_MODEL
  --credential-ref REF          Overrides KIWIGUARD_BETA_OPENAI_CREDENTIAL_REF
  --control-token TOKEN         Bearer token for protected control APIs. Default: KIWIGUARD_CONTROL_AUTH_TOKEN
  --request-id-prefix VALUE     Request id prefix. Default: kg-beta-openai-<unix time>
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
    --postgres-dsn)
      postgres_dsn="${2:-}"
      shift 2
      ;;
    --compose-file)
      compose_file="${2:-}"
      shift 2
      ;;
    --route-key)
      route_key="${2:-}"
      shift 2
      ;;
    --provider-key)
      provider_key="${2:-}"
      shift 2
      ;;
    --base-url)
      base_url="${2:-}"
      shift 2
      ;;
    --model)
      model="${2:-}"
      shift 2
      ;;
    --credential-ref)
      credential_ref="${2:-}"
      shift 2
      ;;
    --control-token)
      control_token="${2:-}"
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

if [[ -z "$gateway_url" || -z "$control_url" || -z "$postgres_dsn" || -z "$compose_file" || -z "$route_key" || -z "$provider_key" || -z "$base_url" || -z "$model" || -z "$credential_ref" || -z "$request_id_prefix" ]]; then
  echo "gateway URL, control URL, PostgreSQL DSN, compose file, route key, provider key, base URL, model, credential ref, and request id prefix are required" >&2
  exit 2
fi

if [[ "$credential_ref" == env:* ]]; then
  credential_env_name="${credential_ref#env:}"
  if [[ -z "$credential_env_name" ]]; then
    echo "credential ref env: must include an environment variable name" >&2
    exit 2
  fi
  if [[ -z "${!credential_env_name:-}" ]]; then
    echo "$credential_env_name must be set for credential ref $credential_ref" >&2
    exit 2
  fi
fi

tmpdir="$(mktemp -d)"
client_id=""
client_key=""

create_client_json="$tmpdir/create-client.json"
route_limit_json="$tmpdir/route-limit.json"
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
    json.dump({"keys": keys, "reason": "beta openai smoke"}, f)
PY

  require_control_status "activate policy bundles" "200" "POST" "$control_url/api/policy-bundles/activate" "$activate_json"
  wait_for_control_health
  sleep 1
}

write_route_limit_json() {
  python3 - "$route_limit_json" <<'PY'
import json
import sys

with open(sys.argv[1], "w", encoding="utf-8") as f:
    json.dump({
        "requests_per_window": 10,
        "window_seconds": 60,
        "max_concurrent_requests": 2,
        "max_body_bytes": 8192,
        "enabled": True,
    }, f)
PY
}

write_gateway_request_json() {
  python3 - "$request_json" "$model" <<'PY'
import json
import sys

path, model = sys.argv[1], sys.argv[2]
with open(path, "w", encoding="utf-8") as f:
    json.dump({
        "model": model,
        "messages": [
            {"role": "system", "content": "You are a KiwiGuard beta smoke test."},
            {"role": "user", "content": "Reply with exactly: KiwiGuard beta smoke ok"},
        ],
        "stream": False,
        "temperature": 0,
    }, f)
PY
}

configure_beta_provider() {
  docker compose -f "$compose_file" exec -T postgres psql "$postgres_dsn" \
    -v ON_ERROR_STOP=1 \
    -v provider_key="$provider_key" \
    -v route_key="$route_key" \
    -v base_url="$base_url" \
    -v credential_ref="$credential_ref" \
    -v model="$model" <<'SQL'
begin;

create temporary table beta_openai_smoke_vars on commit drop as
select
    :'provider_key'::text as provider_key,
    :'route_key'::text as route_key,
    :'base_url'::text as base_url,
    :'credential_ref'::text as credential_ref,
    :'model'::text as model;

update providers
set base_url = vars.base_url,
    credential_ref = vars.credential_ref
from beta_openai_smoke_vars vars
where providers.name = vars.provider_key
  and providers.revision_id in (
      select id from config_revisions where status in ('active', 'draft')
  );

update model_mappings
set source_model = vars.model,
    target_model = vars.model,
    parameters = jsonb_set(
        jsonb_set(coalesce(model_mappings.parameters, '{}'::jsonb), '{route_key}', to_jsonb(vars.route_key), true),
        '{provider}',
        to_jsonb(vars.provider_key),
        true
    )
from beta_openai_smoke_vars vars
where model_mappings.name = 'dev-chat'
  and model_mappings.revision_id in (
      select id from config_revisions where status in ('active', 'draft')
  );

update routes
set upstream_provider = vars.provider_key,
    upstream_model = vars.model
from beta_openai_smoke_vars vars
where routes.name = vars.route_key
  and routes.revision_id in (
      select id from config_revisions where status in ('active', 'draft')
  );

do $$
declare
    active_revision bigint;
    provider_count int;
    mapping_count int;
    route_count int;
begin
    select revision_number into active_revision
    from config_revisions
    where status = 'active'
    order by revision_number desc
    limit 1;

    if active_revision is null then
        raise exception 'no active config revision found';
    end if;

    select count(*) into provider_count
    from providers
    join beta_openai_smoke_vars vars on providers.name = vars.provider_key
    where providers.revision_id in (select id from config_revisions where status in ('active', 'draft'));

    select count(*) into mapping_count
    from model_mappings
    where name = 'dev-chat'
      and revision_id in (select id from config_revisions where status in ('active', 'draft'));

    select count(*) into route_count
    from routes
    join beta_openai_smoke_vars vars on routes.name = vars.route_key
    where routes.revision_id in (select id from config_revisions where status in ('active', 'draft'));

    if provider_count = 0 then
        raise exception 'provider for beta OpenAI smoke was not found';
    end if;
    if mapping_count = 0 then
        raise exception 'model mapping dev-chat for beta OpenAI smoke was not found';
    end if;
    if route_count = 0 then
        raise exception 'route for beta OpenAI smoke was not found';
    end if;

    perform pg_notify('kiwiguard_config_activated', active_revision::text);
end $$;

commit;
SQL
}

gateway_request() {
  local output="$1"
  curl -sS \
    -o "$output" \
    -w "%{http_code}" \
    -X POST "$gateway_url/v1/chat/completions" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer $client_key" \
    -H "X-Request-ID: ${request_id_prefix}-real-upstream" \
    --data-binary "@$request_json"
}

validate_openai_response() {
  python3 - "$response_json" <<'PY'
import json
import sys

with open(sys.argv[1], encoding="utf-8") as f:
    body = json.load(f)
if "error" in body:
    raise SystemExit(f"upstream returned error payload: {body['error']}")
choices = body.get("choices")
if not isinstance(choices, list) or not choices:
    raise SystemExit("response does not look like a chat completions payload: missing choices")
message = choices[0].get("message", {}) if isinstance(choices[0], dict) else {}
content = message.get("content", "")
if not isinstance(content, str) or not content:
    raise SystemExit("response choices[0].message.content is empty")
PY
}

cleanup() {
  if [[ -n "$client_id" ]]; then
    control_request "POST" "$control_url/api/gateway-clients/$client_id/revoke" "$control_response_json" >/dev/null || true
  fi
  rm -rf "$tmpdir"
}
trap cleanup EXIT

python3 - "$create_client_json" <<'PY'
import json
import sys

with open(sys.argv[1], "w", encoding="utf-8") as f:
    json.dump({
        "name": "Beta OpenAI Smoke",
        "notes": "Created by scripts/beta-openai-smoke.sh",
    }, f)
PY

write_route_limit_json
write_gateway_request_json

wait_for_control_health
configure_beta_provider
sleep 2

http_code="$(control_request "POST" "$control_url/api/gateway-clients" "$control_response_json" "$create_client_json")"
if [[ "$http_code" != "201" ]]; then
  fail_with_body "create gateway client returned HTTP $http_code, want 201" "$control_response_json"
fi

read -r client_id client_key < <(
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

require_control_status "configure route limit" "200" "PUT" "$control_url/api/gateway-limits/routes/$route_key" "$route_limit_json"
activate_current_policy_bundles

http_code="$(gateway_request "$response_json")"
if [[ "$http_code" != "200" ]]; then
  fail_with_body "real upstream request returned HTTP $http_code, want 200" "$response_json"
fi
validate_openai_response

require_control_status "revoke gateway client" "200" "POST" "$control_url/api/gateway-clients/$client_id/revoke" ""
client_id=""
activate_current_policy_bundles

echo "route_key=$route_key"
echo "provider_key=$provider_key"
echo "model=$model"
echo "request_id_prefix=$request_id_prefix"
echo "beta_openai_smoke=ok"
