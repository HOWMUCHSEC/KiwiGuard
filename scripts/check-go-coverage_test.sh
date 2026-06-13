#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
script="$repo_root/scripts/check-go-coverage.sh"

tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT
export GOCACHE="$tmpdir/gocache"

cat >"$tmpdir/go.mod" <<'EOF'
module coverage-fixture

go 1.25.11
EOF

cat >"$tmpdir/math.go" <<'EOF'
package coveragefixture

func Covered() int {
	return 1
}

func Uncovered() int {
	return 2
}
EOF

cat >"$tmpdir/math_test.go" <<'EOF'
package coveragefixture

import "testing"

func TestCovered(t *testing.T) {
	if Covered() != 1 {
		t.Fatal("Covered returned wrong value")
	}
}
EOF

(cd "$tmpdir" && go test ./... -coverprofile=coverage.out >/dev/null)

(cd "$tmpdir" && "$script" coverage.out 49.9 >/dev/null)

if (cd "$tmpdir" && "$script" coverage.out 100.1 >stdout 2>stderr); then
	echo "expected coverage gate to fail when threshold is above total coverage" >&2
	exit 1
fi

if ! grep -q "coverage threshold not met" "$tmpdir/stderr"; then
	echo "expected failure output to explain that coverage is below threshold" >&2
	cat "$tmpdir/stderr" >&2
	exit 1
fi

if ! "$script" "$tmpdir/missing.out" 75.0 >"$tmpdir/stdout" 2>"$tmpdir/stderr"; then
	if ! grep -q "coverage profile not found" "$tmpdir/stderr"; then
		echo "expected missing profile output to identify missing coverage file" >&2
		cat "$tmpdir/stderr" >&2
		exit 1
	fi
else
	echo "expected coverage gate to fail when coverage profile is missing" >&2
	exit 1
fi
