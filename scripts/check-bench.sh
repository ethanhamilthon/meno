#!/bin/sh
# Compares the benchmarks of two revisions on the same machine and toolchain.
#
#   scripts/check-bench.sh BASE_REV [TOLERANCE_PERCENT] [COUNT]
#
# It runs the benchmarks of BASE_REV and of the working tree COUNT times each,
# interleaved so drift in machine load hits both equally, and compares the
# median ns/op. A benchmark that got slower by more than TOLERANCE_PERCENT
# (default 25) is reported. There are no absolute time limits: only the change
# against the base counts. With BENCH_REPORT_ONLY=1 the result is printed but
# never fails, which is how a new gate is introduced.
set -eu
base=${1:?usage: check-bench.sh BASE_REV [TOLERANCE_PERCENT] [COUNT]}
tol=${2:-25}
count=${3:-5}
pkgs=${BENCH_PKGS:-./internal/ui ./internal/app}
tmp=$(mktemp -d)
trap 'git worktree remove --force "$tmp/base" >/dev/null 2>&1 || true; rm -rf "$tmp"' EXIT
git worktree add --detach "$tmp/base" "$base" >/dev/null
: > "$tmp/base.txt"; : > "$tmp/head.txt"
i=0
while [ "$i" -lt "$count" ]; do
	(cd "$tmp/base" && go test -run '^$' -bench . -benchtime 200ms -count 1 $pkgs) >> "$tmp/base.txt" 2>&1 || true
	go test -run '^$' -bench . -benchtime 200ms -count 1 $pkgs >> "$tmp/head.txt" 2>&1 || true
	i=$((i + 1))
done
median() { # file name -> median ns/op
	awk -v n="$2" '$1 == n && $4 == "ns/op" { print $3 }' "$1" | sort -n | awk '{ a[NR] = $1 } END { if (NR) print a[int((NR + 1) / 2)] }'
}
names=$(awk '/^Benchmark/ { print $1 }' "$tmp/head.txt" | sort -u)
status=0
printf '%-48s %12s %12s %8s\n' benchmark base head change
for n in $names; do
	b=$(median "$tmp/base.txt" "$n"); h=$(median "$tmp/head.txt" "$n")
	[ -n "$b" ] && [ -n "$h" ] || { printf '%-48s %12s %12s %8s\n' "$n" "${b:--}" "${h:--}" new; continue; }
	pct=$(awk -v b="$b" -v h="$h" 'BEGIN { printf "%.1f", (h - b) * 100 / b }')
	flag=""
	if awk -v p="$pct" -v t="$tol" 'BEGIN { exit !(p + 0 > t + 0) }'; then flag=" SLOWER"; status=1; fi
	printf '%-48s %12s %12s %7s%%%s\n' "$n" "$b" "$h" "$pct" "$flag"
done
if [ "$status" -ne 0 ] && [ "${BENCH_REPORT_ONLY:-0}" = 1 ]; then
	echo "slower than base by more than $tol% (report only)"; exit 0
fi
[ "$status" -eq 0 ] || echo "slower than base by more than $tol%"
exit $status
