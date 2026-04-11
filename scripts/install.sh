#!/usr/bin/env bash
# install.sh — fetch the latest hermes release from GitHub and install
# it into /usr/local/bin. Override PREFIX or VERSION via env vars.
set -euo pipefail

REPO=${REPO:-nousresearch/hermes-agent-go}
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
ARCHIVE="hermes_${VER_NO_V}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/${VERSION}/${ARCHIVE}"

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

echo "Downloading hermes ${VERSION} (${OS}_${ARCH})..."
curl -sSfL "$URL" -o "$TMP/hermes.tar.gz"
tar -xzf "$TMP/hermes.tar.gz" -C "$TMP"

if [[ ! -x "$TMP/hermes" ]]; then
  echo "error: archive did not contain hermes binary" >&2
  exit 1
fi

install -m 0755 "$TMP/hermes" "$PREFIX/hermes"
echo "Installed $PREFIX/hermes"
"$PREFIX/hermes" version
