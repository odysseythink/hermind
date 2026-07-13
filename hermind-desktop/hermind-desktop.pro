QT += widgets network websockets

CONFIG += c++17

INCLUDEPATH += $$PWD $$PWD/api $$PWD/models $$PWD/streaming $$PWD/auth $$PWD/navigation $$PWD/widgets $$PWD/sidebar $$PWD/chat $$PWD/markdown

# You can make your code fail to compile if it uses deprecated APIs.
# In order to do so, uncomment the following line.
#DEFINES += QT_DISABLE_DEPRECATED_BEFORE=0x060000    # disables all the APIs deprecated before Qt 6.0.0

SOURCES += \
    main.cpp \
    main_chat_widget.cpp \
    main_setting_widget.cpp \
    mainwindow.cpp \
    new_workspace_dialog.cpp \
    settings_store.cpp \
    theme_manager.cpp \
    api/api_response.cpp \
    models/hermind_user.cpp \
    models/hermind_workspace.cpp \
    models/hermind_workspace_thread.cpp \
    api/hermind_api_client.cpp \
    models/hermind_stream_chat_response.cpp \
    models/hermind_chat_message.cpp \
    models/hermind_agent_event.cpp \
    models/hermind_memory.cpp \
    chat/chat_stream_handler.cpp \
    chat/agent_event_handler.cpp \
    streaming/hermind_sse_client.cpp \
    streaming/hermind_websocket_client.cpp \
    auth/auth_manager.cpp \
    navigation/navigation_manager.cpp \
    sidebar_widget.cpp \
    sidebar/workspace_list_model.cpp \
    sidebar/workspace_item_widget.cpp \
    sidebar/thread_item_widget.cpp \
    sidebar/thread_container_widget.cpp \
    sidebar/active_workspaces_widget.cpp \
    sidebar/sidebar_footer_widget.cpp \
    widgets/theme_colors.cpp \
    widgets/theme_style_helper.cpp \
    widgets/icon_button.cpp \
    widgets/sidebar_menu_button.cpp \
    widgets/search_input.cpp \
    widgets/styled_separator.cpp \
    widgets/rounded_frame.cpp \
    widgets/setting_row.cpp \
    widgets/chat_history_widget.cpp \
    widgets/chat_container_widget.cpp \
    widgets/prompt_input.cpp \
    widgets/agent_menu.cpp \
    widgets/tools_menu.cpp \
    widgets/slash_commands_tab.cpp \
    widgets/attach_item.cpp \
    widgets/attachment_manager.cpp \
    widgets/tool_approval_dialog.cpp \
    widgets/agent_status_banner.cpp \
    widgets/download_card.cpp \
    widgets/sources_sidebar.cpp \
    widgets/source_item.cpp \
    widgets/citation_detail_modal.cpp \
    widgets/memories_sidebar.cpp \
    widgets/memory_card.cpp \
    widgets/memory_modal.cpp \
    widgets/default_chat_widget.cpp \
    widgets/quick_actions.cpp \
    widgets/suggested_messages.cpp \
    widgets/plain_message_item.cpp \
    widgets/markdown_message_item.cpp \
    markdown/html_sanitizer.cpp \
    markdown/qt_builtin_parser.cpp \
    markdown/html_generator.cpp \
    markdown/syntax_highlighter.cpp \
    markdown/formula_renderer.cpp \
    markdown/markdown_renderer.cpp

HEADERS += \
    main_chat_widget.h \
    main_setting_widget.h \
    mainwindow.h \
    new_workspace_dialog.h \
    settings_store.h \
    theme_manager.h \
    api/api_response.h \
    models/hermind_user.h \
    models/hermind_workspace.h \
    models/hermind_workspace_thread.h \
    api/hermind_api_client.h \
    models/hermind_stream_chat_response.h \
    models/hermind_agent_event.h \
    models/hermind_chat_message.h \
    models/hermind_memory.h \
    chat/chat_stream_handler.h \
    chat/agent_event_handler.h \
    streaming/hermind_sse_client.h \
    streaming/hermind_websocket_client.h \
    auth/auth_state.h \
    auth/auth_manager.h \
    navigation/navigation_manager.h \
    navigation/navigation_route.h \
    sidebar_widget.h \
    sidebar/workspace_list_model.h \
    sidebar/workspace_item_widget.h \
    sidebar/thread_item_widget.h \
    sidebar/thread_container_widget.h \
    sidebar/active_workspaces_widget.h \
    sidebar/sidebar_footer_widget.h \
    widgets/theme_colors.h \
    widgets/theme_style_helper.h \
    widgets/icon_button.h \
    widgets/sidebar_menu_button.h \
    widgets/search_input.h \
    widgets/styled_separator.h \
    widgets/rounded_frame.h \
    widgets/setting_row.h \
    widgets/chat_history_widget.h \
    widgets/chat_container_widget.h \
    widgets/prompt_input.h \
    widgets/agent_menu.h \
    widgets/tools_menu.h \
    widgets/slash_commands_tab.h \
    widgets/attach_item.h \
    widgets/attachment_manager.h \
    widgets/tool_approval_dialog.h \
    widgets/agent_status_banner.h \
    widgets/download_card.h \
    widgets/sources_sidebar.h \
    widgets/source_item.h \
    widgets/citation_detail_modal.h \
    widgets/memories_sidebar.h \
    widgets/memory_card.h \
    widgets/memory_modal.h \
    widgets/default_chat_widget.h \
    widgets/quick_actions.h \
    widgets/suggested_messages.h \
    widgets/plain_message_item.h \
    widgets/markdown_message_item.h \
    markdown/html_sanitizer.h \
    markdown/i_markdown_parser.h \
    markdown/qt_builtin_parser.h \
    markdown/html_generator.h \
    markdown/syntax_highlighter.h \
    markdown/formula_renderer.h \
    markdown/markdown_renderer.h

FORMS += \
    main_chat_widget.ui \
    main_setting_widget.ui \
    mainwindow.ui \
    sidebar_widget.ui

RESOURCES += \
    resources.qrc

# Default rules for deployment.
qnx: target.path = /tmp/$${TARGET}/bin
else: unix:!android: target.path = /opt/$${TARGET}/bin
!isEmpty(target.path): INSTALLS += target
