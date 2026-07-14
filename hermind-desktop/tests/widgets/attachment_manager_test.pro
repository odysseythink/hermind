QT += widgets testlib network websockets
CONFIG += qt warn_on depend_includepath testcase c++17

TEMPLATE = app
TARGET = tst_attachment_manager

INCLUDEPATH += $$PWD/../../widgets $$PWD/../../models $$PWD/../../api $$PWD/../../streaming $$PWD/../..

SOURCES += \
    tst_attachment_manager.cpp \
    ../../widgets/attachment_manager.cpp \
    ../../widgets/theme_colors.cpp \
    ../../settings_store.cpp \
    ../../theme_manager.cpp \
    ../../api/hermind_api_client.cpp \
    ../../api/api_response.cpp \
    ../../streaming/hermind_sse_client.cpp \
    ../../streaming/hermind_websocket_client.cpp \
    ../../models/hermind_user.cpp \
    ../../models/hermind_workspace.cpp \
    ../../models/hermind_workspace_user.cpp \
    ../../models/hermind_workspace_thread.cpp \
    ../../models/hermind_chat_message.cpp \
    ../../models/hermind_stream_chat_response.cpp \
    ../../models/hermind_agent_event.cpp \
    ../../models/hermind_memory.cpp

HEADERS += \
    ../../widgets/attachment_manager.h \
    ../../widgets/theme_colors.h \
    ../../settings_store.h \
    ../../theme_manager.h \
    ../../api/hermind_api_client.h \
    ../../api/api_response.h \
    ../../streaming/hermind_sse_client.h \
    ../../streaming/hermind_websocket_client.h \
    ../../models/hermind_user.h \
    ../../models/hermind_workspace.h \
    ../../models/hermind_workspace_thread.h \
    ../../models/hermind_chat_message.h \
    ../../models/hermind_stream_chat_response.h \
    ../../models/hermind_agent_event.h \
    ../../models/hermind_memory.h
