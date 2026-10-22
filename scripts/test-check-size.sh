#!/bin/sh
# Tests scripts/check-size.sh against temporary fixture roots.
set -e
here=$(cd "$(dirname "$0")" && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
mkdir -p "$tmp/dir with space"
gen() { i=0; : > "$1"; while [ $i -lt "$2" ]; do echo "// l" >> "$1"; i=$((i+1)); done; }
gen "$tmp/dir with space/a.go" 500
gen "$tmp/dir with space/b.go" 700
SIZE_ROOTS="$tmp" "$here/check-size.sh" >/dev/null || { echo "500/700 should pass"; exit 1; }
gen "$tmp/dir with space/c.go" 701
if SIZE_ROOTS="$tmp" "$here/check-size.sh" >/dev/null; then echo "701 should fail"; exit 1; fi
echo ok
