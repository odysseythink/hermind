QT += widgets testlib network websockets
CONFIG += qt warn_on depend_includepath testcase c++17

TEMPLATE = app
TARGET = tst_general_appearance_tab

INCLUDEPATH += $$PWD/../../widgets $$PWD/../../api $$PWD/../../models $$PWD/../../streaming $$PWD/../..

SOURCES += \
    tst_general_appearance_tab.cpp \
    ../../widgets/general_appearance_tab.cpp \
    ../../widgets/suggested_messages_editor.cpp \
    ../../widgets/setting_row.cpp \
    ../../widgets/icon_button.cpp \
    ../../widgets/theme_colors.cpp \
    ../../widgets/theme_style_helper.cpp \
    ../../theme_manager.cpp \
    ../../settings_store.cpp \
    ../../api/hermind_api_client.cpp \
    ../../api/api_response.cpp \
    ../../models/hermind_user.cpp \
    ../../models/hermind_memory.cpp \
    ../../models/hermind_workspace.cpp \
    ../../models/hermind_workspace_user.cpp \
    ../../models/hermind_workspace_thread.cpp \
    ../../models/hermind_chat_message.cpp \
    ../../models/hermind_stream_chat_response.cpp \
    ../../models/hermind_agent_event.cpp \
    ../../streaming/hermind_sse_client.cpp \
    ../../streaming/hermind_websocket_client.cpp

HEADERS += \
    ../../widgets/general_appearance_tab.h \
    ../../widgets/suggested_messages_editor.h \
    ../../widgets/setting_row.h \
    ../../widgets/icon_button.h \
    ../../widgets/theme_colors.h \
    ../../widgets/theme_style_helper.h \
    ../../theme_manager.h \
    ../../settings_store.h \
    ../../api/hermind_api_client.h \
    ../../api/api_response.h \
    ../../models/hermind_user.h \
    ../../models/hermind_memory.h \
    ../../models/hermind_workspace.h \
    ../../models/hermind_workspace_user.h \
    ../../models/hermind_workspace_thread.h \
    ../../models/hermind_chat_message.h \
    ../../models/hermind_stream_chat_response.h \
    ../../models/hermind_agent_event.h \
    ../../streaming/hermind_sse_client.h \
    ../../streaming/hermind_websocket_client.h
