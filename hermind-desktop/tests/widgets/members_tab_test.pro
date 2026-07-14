QT += widgets testlib network websockets
CONFIG += qt warn_on depend_includepath testcase c++17

TEMPLATE = app
TARGET = tst_members_tab

INCLUDEPATH += $$PWD/../../widgets $$PWD/../../api $$PWD/../../models $$PWD/../../streaming $$PWD/../..

SOURCES += \
    tst_members_tab.cpp \
    ../../widgets/members_tab.cpp \
    ../../widgets/theme_colors.cpp \
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
    ../../widgets/members_tab.h \
    ../../widgets/theme_colors.h \
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
