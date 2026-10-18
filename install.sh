#!/bin/sh
# Installs meno from a GitHub release.
#
#   curl -fsSL https://github.com/ethanhamilthon/meno/releases/latest/download/install.sh | sh
#   curl -fsSL .../install.sh | sh -s -- v0.1.0        # a specific version
#
# Environment: MENO_INSTALL_DIR (default ~/.local/bin, or /usr/local/bin when run as root),
# MENO_REPO (default ethanhamilthon/meno), MENO_BASE_URL (override the download location).
set -eu

repo=${MENO_REPO:-ethanhamilthon/meno}
version=${1:-latest}

say() { printf 'meno: %s\n' "$*"; }
die() { printf 'meno: error: %s\n' "$*" >&2; exit 1; }

case "$(uname -s)" in
	Darwin) os=darwin ;;
	Linux) os=linux ;;
	*) die "unsupported OS $(uname -s): meno supports macOS and Linux" ;;
esac
case "$(uname -m)" in
	arm64|aarch64) arch=arm64 ;;
	x86_64|amd64) arch=amd64 ;;
	*) die "unsupported architecture $(uname -m)" ;;
esac

if [ -n "${MENO_INSTALL_DIR:-}" ]; then
	dir=$MENO_INSTALL_DIR
elif [ "$(id -u)" = 0 ]; then
	dir=/usr/local/bin
else
	dir=$HOME/.local/bin
fi

if [ -n "${MENO_BASE_URL:-}" ]; then
	base=$MENO_BASE_URL
elif [ "$version" = latest ]; then
	base=https://github.com/$repo/releases/latest/download
else
	base=https://github.com/$repo/releases/download/$version
fi

command -v curl >/dev/null || die "curl is required"
command -v tar >/dev/null || die "tar is required"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT INT TERM

curl -fsSL "$base/SHA256SUMS" -o "$tmp/SHA256SUMS" || die "cannot download $base/SHA256SUMS (is there a release for $version?)"
archive=$(grep "_${os}_${arch}\.tar\.gz\$" "$tmp/SHA256SUMS" | awk '{print $2}' | head -n 1)
[ -n "$archive" ] || die "no build for $os/$arch in this release"
say "downloading $archive"
curl -fsSL "$base/$archive" -o "$tmp/$archive" || die "download failed"

want=$(grep " $archive\$" "$tmp/SHA256SUMS" | awk '{print $1}' | head -n 1)
if command -v sha256sum >/dev/null; then
	got=$(sha256sum "$tmp/$archive" | awk '{print $1}')
elif command -v shasum >/dev/null; then
	got=$(shasum -a 256 "$tmp/$archive" | awk '{print $1}')
else
	die "sha256sum or shasum is required to verify the download"
fi
[ "$want" = "$got" ] || die "checksum mismatch for $archive (expected $want, got $got)"

tar -C "$tmp" -xzf "$tmp/$archive" meno || die "bad archive"
mkdir -p "$dir"
mv "$tmp/meno" "$dir/meno"
chmod 755 "$dir/meno"
say "installed $("$dir/meno" --version 2>/dev/null || echo meno) to $dir/meno"

case ":$PATH:" in
	*":$dir:"*) ;;
	*) say "add $dir to your PATH:  export PATH=\"$dir:\$PATH\"" ;;
esac
