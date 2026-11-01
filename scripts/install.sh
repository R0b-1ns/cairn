#!/bin/sh
# Install the latest cairn release: download, verify the SHA-256, install on PATH.
# Usage: curl -fsSL https://raw.githubusercontent.com/R0b-1ns/cairn/main/scripts/install.sh | sh
# Environment:
#   CAIRN_VERSION      release tag to install, for example v0.3.0 (default: latest)
#   CAIRN_INSTALL_DIR  directory to install into (default: /usr/local/bin if writable, else ~/.local/bin)
set -eu

repo=R0b-1ns/cairn

err() { echo "install: $*" >&2; exit 1; }

case "$(uname -s)" in
Darwin) os=darwin ;;
Linux) os=linux ;;
*) err "unsupported OS: $(uname -s) (macOS and Linux only)" ;;
esac
case "$(uname -m)" in
x86_64 | amd64) arch=amd64 ;;
aarch64 | arm64) arch=arm64 ;;
*) err "unsupported architecture: $(uname -m)" ;;
esac
asset="cairn-$os-$arch"

if [ -n "${CAIRN_VERSION:-}" ]; then
	base="https://github.com/$repo/releases/download/$CAIRN_VERSION"
else
	base="https://github.com/$repo/releases/latest/download"
fi

if command -v sha256sum >/dev/null 2>&1; then
	sha256() { sha256sum "$1" | cut -d ' ' -f 1; }
elif command -v shasum >/dev/null 2>&1; then
	sha256() { shasum -a 256 "$1" | cut -d ' ' -f 1; }
else
	err "neither sha256sum nor shasum found"
fi

if [ -n "${CAIRN_INSTALL_DIR:-}" ]; then
	dir=$CAIRN_INSTALL_DIR
elif [ -d /usr/local/bin ] && [ -w /usr/local/bin ]; then
	dir=/usr/local/bin
else
	dir=$HOME/.local/bin
fi

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT INT TERM

echo "Downloading $asset ..."
curl -fsSL -o "$tmp/$asset" "$base/$asset" || err "cannot download $base/$asset"
curl -fsSL -o "$tmp/SHA256SUMS" "$base/SHA256SUMS" || err "cannot download $base/SHA256SUMS"

want=$(awk -v f="$asset" '$2 == f { print $1 }' "$tmp/SHA256SUMS")
[ -n "$want" ] || err "$asset is not listed in SHA256SUMS"
got=$(sha256 "$tmp/$asset")
[ "$got" = "$want" ] || err "checksum mismatch for $asset (expected $want, got $got)"
echo "Checksum OK."

chmod +x "$tmp/$asset"
mkdir -p "$dir"
mv "$tmp/$asset" "$dir/cairn"
echo "Installed $dir/cairn"

case ":$PATH:" in
*":$dir:"*) ;;
*) echo "Note: $dir is not on your PATH. Add it, for example: export PATH=\"$dir:\$PATH\"" ;;
esac
