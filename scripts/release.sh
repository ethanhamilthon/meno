#!/bin/sh
# Builds the release archives for macOS and Linux (arm64, amd64) and SHA256SUMS.
# Each archive meno_<version>_<os>_<arch>.tar.gz contains one file: meno.
# Usage: scripts/release.sh [version] [outdir]  (version defaults to internal/version/VERSION)
set -eu
root=$(cd "$(dirname "$0")/.." && pwd)
version=${1:-$(tr -d ' \n' < "$root/internal/version/VERSION")}
out=${2:-dist}
rm -rf "$out"
mkdir -p "$out"
for target in darwin/arm64 darwin/amd64 linux/arm64 linux/amd64; do
	os=${target%/*}
	arch=${target#*/}
	stage=$(mktemp -d)
	CGO_ENABLED=0 GOOS=$os GOARCH=$arch go build -trimpath -ldflags "-s -w" -o "$stage/meno" ./cmd/meno
	tar -C "$stage" -czf "$out/meno_${version}_${os}_${arch}.tar.gz" meno
	rm -rf "$stage"
done
cp install.sh uninstall.sh "$out/"
(cd "$out" && if command -v sha256sum >/dev/null; then sha256sum *.tar.gz; else shasum -a 256 *.tar.gz; fi > SHA256SUMS)
echo "built $version in $out"
