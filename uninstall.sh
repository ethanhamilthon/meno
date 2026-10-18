#!/bin/sh
# Removes the meno binary. Your data in ~/.meno is kept unless you pass --purge.
set -eu
for d in "${MENO_INSTALL_DIR:-}" "$HOME/.local/bin" /usr/local/bin; do
	if [ -n "$d" ] && [ -f "$d/meno" ]; then rm -f "$d/meno" && echo "meno: removed $d/meno"; fi
done
if [ "${1:-}" = "--purge" ]; then
	rm -rf "${MENO_HOME:-$HOME/.meno}"
	echo "meno: removed data directory"
fi
