#!/usr/bin/env bash
set -e

echo "Building go-desktop-interface static library..."

export CGO_ENABLED=1

# Auto-detect C compiler
if [ -z "$CC" ]; then
    if command -v clang &> /dev/null; then
        export CC=clang
    elif command -v gcc &> /dev/null; then
        export CC=gcc
    else
        echo "ERROR: Cannot find C compiler for CGO. Please set CC environment variable."
        exit 1
    fi
fi

# Ensure output directory exists
mkdir -p "$(dirname "$0")/build"

# Build the static library
cd "$(dirname "$0")/.."
go build -buildmode=c-archive -o "$(dirname "$0")/build/libgo-desktop-interface.a" ./cmd/go-desktop-interface

echo ""
echo "Success: libgo-desktop-interface.a and libgo-desktop-interface.h"
echo "generated in desktop/build/"
