QT += widgets testlib network websockets
CONFIG += qt warn_on depend_includepath testcase c++17

TEMPLATE = app
TARGET = tst_chat_container_widget

INCLUDEPATH += $$PWD/../../widgets $$PWD/../../api $$PWD/../../models $$PWD/../../chat $$PWD/../../streaming $$PWD/../../markdown $$PWD/../..

SOURCES += \
    tst_chat_container_widget.cpp \
    ../../widgets/chat_container_widget.cpp \
    ../../widgets/prompt_input.cpp \
    ../../widgets/agent_menu.cpp \
    ../../widgets/tools_menu.cpp \
    ../../widgets/slash_commands_tab.cpp \
    ../../widgets/attach_item.cpp \
    ../../widgets/attachment_manager.cpp \
    ../../widgets/tool_approval_dialog.cpp \
    ../../widgets/agent_status_banner.cpp \
    ../../widgets/download_card.cpp \
    ../../widgets/sources_sidebar.cpp \
    ../../widgets/source_item.cpp \
    ../../widgets/citation_detail_modal.cpp \
    ../../widgets/memories_sidebar.cpp \
    ../../widgets/memory_card.cpp \
    ../../widgets/memory_modal.cpp \
    ../../widgets/default_chat_widget.cpp \
    ../../widgets/quick_actions.cpp \
    ../../widgets/suggested_messages.cpp \
    ../../widgets/chat_history_widget.cpp \
    ../../widgets/plain_message_item.cpp \
    ../../widgets/markdown_message_item.cpp \
    ../../widgets/theme_colors.cpp \
    ../../widgets/theme_style_helper.cpp \
    ../../theme_manager.cpp \
    ../../settings_store.cpp \
    ../../chat/chat_stream_handler.cpp \
    ../../chat/agent_event_handler.cpp \
    ../../auth/auth_manager.cpp \
    ../../api/hermind_api_client.cpp \
    ../../models/hermind_memory.cpp \
    ../../api/api_response.cpp \
    ../../models/hermind_user.cpp \
    ../../models/hermind_workspace.cpp \
    ../../models/hermind_workspace_thread.cpp \
    ../../models/hermind_stream_chat_response.cpp \
    ../../models/hermind_agent_event.cpp \
    ../../models/hermind_chat_message.cpp \
    ../../streaming/hermind_sse_client.cpp \
    ../../streaming/hermind_websocket_client.cpp \
    ../../markdown/markdown_renderer.cpp \
    ../../markdown/qt_builtin_parser.cpp \
    ../../markdown/html_generator.cpp \
    ../../markdown/html_sanitizer.cpp \
    ../../markdown/syntax_highlighter.cpp \
    ../../markdown/formula_renderer.cpp

HEADERS += \
    ../../widgets/chat_container_widget.h \
    ../../widgets/prompt_input.h \
    ../../widgets/agent_menu.h \
    ../../widgets/tools_menu.h \
    ../../widgets/slash_commands_tab.h \
    ../../widgets/attach_item.h \
    ../../widgets/attachment_manager.h \
    ../../widgets/tool_approval_dialog.h \
    ../../widgets/agent_status_banner.h \
    ../../widgets/download_card.h \
    ../../widgets/sources_sidebar.h \
    ../../widgets/source_item.h \
    ../../widgets/citation_detail_modal.h \
    ../../widgets/memories_sidebar.h \
    ../../widgets/memory_card.h \
    ../../widgets/memory_modal.h \
    ../../widgets/default_chat_widget.h \
    ../../widgets/quick_actions.h \
    ../../widgets/suggested_messages.h \
    ../../widgets/chat_history_widget.h \
    ../../widgets/plain_message_item.h \
    ../../widgets/markdown_message_item.h \
    ../../widgets/theme_colors.h \
    ../../widgets/theme_style_helper.h \
    ../../theme_manager.h \
    ../../settings_store.h \
    ../../chat/chat_stream_handler.h \
    ../../chat/agent_event_handler.h \
    ../../auth/auth_manager.h \
    ../../api/hermind_api_client.h \
    ../../api/api_response.h \
    ../../models/hermind_user.h \
    ../../models/hermind_workspace.h \
    ../../models/hermind_workspace_thread.h \
    ../../models/hermind_stream_chat_response.h \
    ../../models/hermind_agent_event.h \
    ../../models/hermind_chat_message.h \
    ../../streaming/hermind_sse_client.h \
    ../../streaming/hermind_websocket_client.h \
    ../../markdown/markdown_renderer.h \
    ../../markdown/i_markdown_parser.h \
    ../../markdown/qt_builtin_parser.h \
    ../../markdown/html_generator.h \
    ../../markdown/html_sanitizer.h \
    ../../markdown/syntax_highlighter.h \
    ../../markdown/formula_renderer.h

RESOURCES += ../markdown/test_resources.qrc
