#!/usr/bin/env bash
set -euo pipefail

required_files=(
  "LICENSE"
  "CONTRIBUTING.md"
  "SECURITY.md"
  "CODE_OF_CONDUCT.md"
  "CHANGELOG.md"
  ".github/PULL_REQUEST_TEMPLATE.md"
  ".github/ISSUE_TEMPLATE/bug_report.yml"
  ".github/ISSUE_TEMPLATE/feature_request.yml"
  ".github/dependabot.yml"
)

missing=0
for path in "${required_files[@]}"; do
  if [[ ! -s "$path" ]]; then
    echo "missing required repository standards file: $path" >&2
    missing=1
  fi
done

required_readme_terms=(
  "CONTRIBUTING.md"
  "SECURITY.md"
  "CHANGELOG.md"
  "make verify"
)

for term in "${required_readme_terms[@]}"; do
  if ! grep -q "$term" README.md; then
    echo "README.md must mention $term" >&2
    missing=1
  fi
done

required_make_targets=(
  "verify:"
  "lint-go:"
  "vuln-go:"
  "tidy-check:"
  "test-go-race:"
  "build-web:"
)

for target in "${required_make_targets[@]}"; do
  if ! grep -q "^$target" Makefile; then
    echo "Makefile must define $target" >&2
    missing=1
  fi
done

if [[ "$missing" -ne 0 ]]; then
  exit 1
fi

echo "repository standards check passed"
