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
run_test models hermind_memory_test.pro tst_hermind_memory

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
echo "=== sidebar tests ==="
run_test sidebar workspace_list_model_test.pro tst_workspace_list_model
run_test sidebar workspace_item_widget_test.pro tst_workspace_item_widget
run_test sidebar thread_item_widget_test.pro tst_thread_item_widget
run_test sidebar thread_container_widget_test.pro tst_thread_container_widget
run_test sidebar active_workspaces_widget_test.pro tst_active_workspaces_widget
run_test sidebar sidebar_footer_widget_test.pro tst_sidebar_footer_widget
run_test sidebar sidebar_widget_test.pro tst_sidebar_widget

echo ""
echo "=== dialogs tests ==="
run_test dialogs new_workspace_dialog_test.pro tst_new_workspace_dialog

echo ""
echo "=== widgets tests ==="
run_test widgets widgets_test.pro widgets_test
run_test widgets prompt_input_test.pro tst_prompt_input
run_test widgets agent_menu_test.pro tst_agent_menu
run_test widgets tools_menu_test.pro tst_tools_menu
run_test widgets attachment_manager_test.pro tst_attachment_manager
run_test widgets tool_approval_dialog_test.pro tst_tool_approval_dialog
run_test widgets sources_sidebar_test.pro tst_sources_sidebar
run_test widgets memories_sidebar_test.pro tst_memories_sidebar
run_test widgets suggested_messages_test.pro tst_suggested_messages
run_test widgets default_chat_widget_test.pro tst_default_chat_widget
run_test widgets chat_container_widget_test.pro tst_chat_container_widget

echo ""
echo "=== all desktop unit tests passed ==="
