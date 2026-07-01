#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")" && pwd)"
MAKE="${MAKE:-mingw32-make}"

is_windows() {
    [[ "$OSTYPE" == "msys" || "$OSTYPE" == "cygwin" || "${OS:-}" == "Windows_NT" ]]
}

run_test() {
    local dir="$1"
    local pro="$2"
    local target="$3"

    cd "$ROOT/$dir"
    qmake "$pro"
    $MAKE
    if is_windows; then
        "./release/${target}.exe"
    else
        "./${target}"
    fi
}

echo "=== models tests ==="
run_test models models_test.pro tst_models

echo ""
echo "=== api tests ==="
run_test api api_client_test.pro tst_api_client

echo ""
echo "=== navigation tests ==="
run_test navigation navigation_route_test.pro navigation_route_test
run_test navigation navigation_manager_test.pro navigation_manager_test

echo ""
echo "=== streaming sse tests ==="
run_test streaming sse_test.pro tst_sse_client

echo ""
echo "=== streaming websocket tests ==="
run_test streaming ws_test.pro tst_websocket_client

echo ""
echo "=== widgets tests ==="
run_test widgets widgets_test.pro widgets_test

echo ""
echo "=== all desktop unit tests passed ==="
