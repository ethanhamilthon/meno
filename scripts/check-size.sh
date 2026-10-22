#!/bin/sh
# Fails when a hand-written Go file is longer than the limit in AGENTS.md.
limit=${1:-700}
roots=${SIZE_ROOTS:-"cmd internal"}
bad=$(find $roots -name '*.go' | while read -r f; do
	n=$(wc -l < "$f")
	[ "$n" -gt "$limit" ] && echo "$f: $n lines"
done)
if [ -n "$bad" ]; then
	echo "files over $limit lines:"
	echo "$bad"
	exit 1
fi
