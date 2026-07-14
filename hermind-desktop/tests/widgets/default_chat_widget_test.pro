QT += widgets testlib network websockets
CONFIG += qt warn_on depend_includepath testcase c++17

TEMPLATE = app
TARGET = tst_default_chat_widget

INCLUDEPATH += $$PWD/../../widgets $$PWD/../../api $$PWD/../../models $$PWD/../../streaming $$PWD/../..

SOURCES += \
    tst_default_chat_widget.cpp \
    ../../widgets/default_chat_widget.cpp \
    ../../widgets/quick_actions.cpp \
    ../../widgets/suggested_messages.cpp \
    ../../widgets/prompt_input.cpp \
    ../../widgets/agent_menu.cpp \
    ../../widgets/tools_menu.cpp \
    ../../widgets/slash_commands_tab.cpp \
    ../../widgets/attach_item.cpp \
    ../../widgets/attachment_manager.cpp \
    ../../widgets/theme_colors.cpp \
    ../../theme_manager.cpp \
    ../../settings_store.cpp \
    ../../api/hermind_api_client.cpp \
    ../../api/api_response.cpp \
    ../../models/hermind_memory.cpp \
    ../../models/hermind_user.cpp \
    ../../models/hermind_workspace.cpp \
    ../../models/hermind_workspace_user.cpp \
    ../../models/hermind_workspace_thread.cpp \
    ../../models/hermind_chat_message.cpp \
    ../../models/hermind_stream_chat_response.cpp \
    ../../models/hermind_agent_event.cpp \
    ../../streaming/hermind_sse_client.cpp \
    ../../streaming/hermind_websocket_client.cpp

HEADERS += \
    ../../widgets/default_chat_widget.h \
    ../../widgets/quick_actions.h \
    ../../widgets/suggested_messages.h \
    ../../widgets/prompt_input.h \
    ../../widgets/agent_menu.h \
    ../../widgets/tools_menu.h \
    ../../widgets/slash_commands_tab.h \
    ../../widgets/attach_item.h \
    ../../widgets/attachment_manager.h \
    ../../widgets/theme_colors.h \
    ../../theme_manager.h \
    ../../settings_store.h \
    ../../api/hermind_api_client.h \
    ../../api/api_response.h \
    ../../models/hermind_memory.h \
    ../../models/hermind_user.h \
    ../../models/hermind_workspace.h \
    ../../models/hermind_workspace_thread.h \
    ../../models/hermind_chat_message.h \
    ../../models/hermind_stream_chat_response.h \
    ../../models/hermind_agent_event.h \
    ../../streaming/hermind_sse_client.h \
    ../../streaming/hermind_websocket_client.h
