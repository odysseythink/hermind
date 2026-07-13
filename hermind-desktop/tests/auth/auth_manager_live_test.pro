QT += testlib network websockets widgets
CONFIG += qt warn_on depend_includepath testcase c++17

TEMPLATE = app

INCLUDEPATH += $$PWD/../.. $$PWD/../../api $$PWD/../../models $$PWD/../../streaming $$PWD/../../auth

SOURCES += \
    tst_auth_manager_live.cpp \
    ../../auth/auth_manager.cpp \
    ../../settings_store.cpp \
    ../../api/api_response.cpp \
    ../../api/hermind_api_client.cpp \
    ../../models/hermind_memory.cpp \
    ../../models/hermind_user.cpp \
    ../../models/hermind_workspace.cpp \
    ../../models/hermind_stream_chat_response.cpp \
    ../../models/hermind_agent_event.cpp \
    ../../streaming/hermind_sse_client.cpp \
    ../../streaming/hermind_websocket_client.cpp

HEADERS += \
    ../../auth/auth_manager.h \
    ../../auth/auth_state.h \
    ../../settings_store.h \
    ../../api/api_response.h \
    ../../api/hermind_api_client.h \
    ../../models/hermind_user.h \
    ../../models/hermind_workspace.h \
    ../../models/hermind_stream_chat_response.h \
    ../../models/hermind_agent_event.h \
    ../../streaming/hermind_sse_client.h \
    ../../streaming/hermind_websocket_client.h
