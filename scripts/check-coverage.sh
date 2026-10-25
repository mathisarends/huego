#!/usr/bin/env bash
set -euo pipefail

threshold=${1:-90}

# -coverpkg counts code that tests in other packages exercise, such as the
# hue client driven by the cmd tests; covdata merges the per-package runs
# into one profile without duplicate blocks.
covdir=$(mktemp -d)
trap 'rm -rf "$covdir"' EXIT

go test ./... -cover -coverpkg=./cmd/...,./internal/... -args -test.gocoverdir="$covdir"
go tool covdata textfmt -i="$covdir" -o coverage.out

total=$(go tool cover -func=coverage.out | tail -1 | awk '{print substr($3, 1, length($3)-1)}')

printf "Total coverage: %s%%\n" "$total"

awk -v t="$threshold" -v v="$total" 'BEGIN { if (v+0 < t+0) { exit 1 } }'
