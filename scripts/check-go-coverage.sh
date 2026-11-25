#!/usr/bin/env bash
# Runs the Go tests with cross-package coverage and fails when the total drops below the floor.
# Cross-package (-coverpkg) because some packages are exercised from others: the HTTP clients
# by the fixture tests, the whole app by the integration test.
set -euo pipefail

minimum_coverage_percent="${1:-80}"
repository_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
coverage_profile="$(mktemp)"
trap 'rm -f "${coverage_profile}"' EXIT

cd "${repository_root}"
go test -race -count=1 -coverpkg=./apps/api/... -coverprofile="${coverage_profile}" ./apps/api/...
# The icon generator is not part of the backend coverage, but its tests guard the committed icons.
go test -count=1 ./tools/icon-generator/...

total_coverage_percent="$(go tool cover -func="${coverage_profile}" | awk '/^total:/ { sub(/%/, "", $NF); print $NF }')"
echo "Go coverage: ${total_coverage_percent}% (floor ${minimum_coverage_percent}%)"
if awk -v measured="${total_coverage_percent}" -v floor="${minimum_coverage_percent}" 'BEGIN { exit !(measured < floor) }'; then
  echo "Go coverage dropped below ${minimum_coverage_percent}%"
  exit 1
fi
