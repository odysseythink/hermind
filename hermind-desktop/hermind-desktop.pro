QT += widgets network websockets

CONFIG += c++17

INCLUDEPATH += $$PWD $$PWD/api $$PWD/models $$PWD/streaming $$PWD/auth $$PWD/navigation $$PWD/widgets $$PWD/sidebar $$PWD/chat

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
    widgets/chat_message_item.cpp \
    widgets/chat_history_widget.cpp \
    widgets/chat_container_widget.cpp

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
    widgets/chat_message_item.h \
    widgets/chat_history_widget.h \
    widgets/chat_container_widget.h

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
