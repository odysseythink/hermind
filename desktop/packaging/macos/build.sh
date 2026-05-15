#!/bin/bash
set -e
cd "$(dirname "$0")/../.."
mkdir -p build && cd build
cmake .. -DCMAKE_BUILD_TYPE=Release -DCMAKE_PREFIX_PATH=/opt/homebrew
cmake --build . --config Release
macdeployqt hermind-desktop.app -qmldir=../src 2>/dev/null || true
# Copy Go backend into bundle
cp ../../bin/hermind-desktop-backend hermind-desktop.app/Contents/MacOS/ 2>/dev/null || true
echo "Built: $(pwd)/hermind-desktop.app"
