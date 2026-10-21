#!/bin/sh
# Fails when a package's statement coverage is below its floor in
# scripts/coverage-floors.txt. Usage: scripts/check-coverage.sh [floors-file]
floors=${1:-scripts/coverage-floors.txt}
out=$(go test ./... -cover 2>&1) || { echo "$out" | grep -E -- "^(FAIL|--- FAIL|panic)|(_test|testing)\.go:[0-9]+|DEBUG" | head -40; echo "tests failed"; exit 1; }
status=0
while read -r pkg min; do
	case "$pkg" in ''|'#'*) continue;; esac
	got=$(echo "$out" | awk -v p="meno/$pkg" '$2 == p { for (i = 1; i <= NF; i++) if ($i == "coverage:") { sub("%", "", $(i+1)); print $(i+1) } }')
	if [ -z "$got" ]; then
		echo "$pkg: no coverage reported"
		status=1
	elif ! awk -v g="$got" -v m="$min" 'BEGIN { exit !(g + 0 >= m + 0) }'; then
		echo "$pkg: coverage $got% is below the floor of $min%"
		status=1
	fi
done < "$floors"
[ "$status" -eq 0 ] && echo "coverage floors hold"
exit $status
