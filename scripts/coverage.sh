#!/usr/bin/env bash
# Statement coverage for the whole module, including subprocess coverage
# of main() (the os.Exit adapter): the e2e test builds the binary with
# -cover and the test binary runs with GOCOVERDIR, so the in-process and
# child counters both land in one directory.
#
# `go test -cover` alone cannot capture main(): cmd/go overrides
# GOCOVERDIR for instrumented test binaries, so the child's counters
# never reach a directory we control. The documented recipe (go help
# cover / integration-test coverage) is to build the test binary with
# `go test -c -cover` and run it with GOCOVERDIR set, which is what
# this script does for the root package.
set -euo pipefail

cd "$(dirname "$0")/.."
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

# In-process coverage for every package.
go test ./... -coverprofile="$work/inproc.out" >/dev/null

# Subprocess coverage for main(): the compiled test binary plus the
# e2e child binary write counters into the same directory. GOCOVERDIR
# must already exist or the instrumented binaries refuse to run.
mkdir -p "$work/counters"
go test -c -cover -o "$work/root.test" .
GOCOVERDIR="$work/counters" "$work/root.test" -test.run TestMainFunctionE2E >/dev/null
go tool covdata textfmt -i="$work/counters" -o "$work/child.out"

# Merge: for blocks present in both profiles keep the higher count
# (each profile independently observed execution), then report.
awk '
  NR == FNR { if (FNR > 1) c[$1] = $3; next }
  FNR == 1 { print; next }
  { if ($1 in c && c[$1] > $3) $3 = c[$1]; print }
' "$work/child.out" "$work/inproc.out" > "$work/merged.out"

echo "total:"
go tool cover -func="$work/merged.out" | tail -1
echo "statements below 100%:"
go tool cover -func="$work/merged.out" | grep -v '100.0%' || echo "  none"
