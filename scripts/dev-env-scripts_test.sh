#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

required_targets=(
  "dev-env"
  "dev-env-infra"
  "dev-env-seed"
  "dev-env-stop"
  "dev-mock-llm"
  "dev-client-smoke"
  "dev-client-limits-smoke"
  "dev-clickhouse-outage-smoke"
  "beta-openai-smoke"
)

for target in "${required_targets[@]}"; do
  if ! grep -q "^${target}:" "$repo_root/Makefile"; then
    echo "Makefile must define ${target}" >&2
    exit 1
  fi
done

required_scripts=(
  "scripts/dev-mock-llm-api.sh"
  "scripts/dev-llm-client.sh"
  "scripts/dev-client-limits-smoke.sh"
  "scripts/dev-clickhouse-outage-smoke.sh"
  "scripts/beta-openai-smoke.sh"
)

for script in "${required_scripts[@]}"; do
  path="$repo_root/$script"
  if [[ ! -x "$path" ]]; then
    echo "$script must exist and be executable" >&2
    exit 1
  fi
  bash -n "$path"
  "$path" --help >/dev/null
done

for sql_fragment in "route_limit_policies" "requests_per_window" "max_concurrent_requests"; do
  if ! grep -q "$sql_fragment" "$repo_root/scripts/dev-seed-config.sql"; then
    echo "scripts/dev-seed-config.sql must seed gateway route limits with $sql_fragment" >&2
    exit 1
  fi
done

echo "dev environment script checks passed"
