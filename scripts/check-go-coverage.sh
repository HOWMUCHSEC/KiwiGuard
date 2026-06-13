#!/usr/bin/env bash
set -euo pipefail

usage() {
	echo "usage: $0 [coverage-profile] [minimum-percent]" >&2
	echo "  coverage-profile defaults to coverage.out" >&2
	echo "  minimum-percent defaults to GO_COVERAGE_MIN or 75.0" >&2
}

coverage_profile="${1:-coverage.out}"
minimum="${2:-${GO_COVERAGE_MIN:-75.0}}"

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
	usage
	exit 0
fi

if [[ ! -f "$coverage_profile" ]]; then
	echo "coverage profile not found: $coverage_profile" >&2
	exit 1
fi

if ! awk -v value="$minimum" 'BEGIN { exit(value ~ /^[0-9]+([.][0-9]+)?$/ ? 0 : 1) }'; then
	echo "invalid coverage threshold: $minimum" >&2
	usage
	exit 2
fi

summary="$(go tool cover -func="$coverage_profile")"
total="$(awk '/^total:/ { sub(/%$/, "", $3); print $3 }' <<<"$summary")"

if [[ -z "$total" ]]; then
	echo "unable to read total coverage from: $coverage_profile" >&2
	exit 1
fi

awk -v total="$total" -v minimum="$minimum" '
	BEGIN {
		if (total + 0 < minimum + 0) {
			printf "coverage threshold not met: total %.1f%% is below required %.1f%%\n", total, minimum > "/dev/stderr"
			exit 1
		}
		printf "coverage threshold met: total %.1f%% >= required %.1f%%\n", total, minimum
	}
'
