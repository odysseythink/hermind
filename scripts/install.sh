#!/usr/bin/env bash
# install.sh — fetch the latest hermind release from GitHub and install
# it into /usr/local/bin. Override PREFIX or VERSION via env vars.
#
# On macOS and Linux with Homebrew, the preferred path is:
#     brew tap odysseythink/tap
#     brew install hermind
# This script is for systems without Homebrew.
set -euo pipefail

REPO=${REPO:-odysseythink/hermind}
PREFIX=${PREFIX:-/usr/local/bin}
VERSION=${VERSION:-$(curl -sSfL https://api.github.com/repos/"$REPO"/releases/latest | grep -oE '"tag_name": *"[^"]+"' | head -n1 | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')}

if [[ -z "${VERSION:-}" ]]; then
  echo "error: could not determine latest release tag" >&2
  exit 1
fi

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64) ARCH="x86_64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "unsupported arch: $ARCH" >&2; exit 1 ;;
esac

# Strip leading 'v' from tag for archive name.
VER_NO_V="${VERSION#v}"
ARCHIVE="hermind_${VER_NO_V}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/${VERSION}/${ARCHIVE}"

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

echo "Downloading hermind ${VERSION} (${OS}_${ARCH})..."
curl -sSfL "$URL" -o "$TMP/hermind.tar.gz"
tar -xzf "$TMP/hermind.tar.gz" -C "$TMP"

if [[ ! -x "$TMP/hermind" ]]; then
  echo "error: archive did not contain hermind binary" >&2
  exit 1
fi

install -m 0755 "$TMP/hermind" "$PREFIX/hermind"
echo "Installed $PREFIX/hermind"
"$PREFIX/hermind" version
