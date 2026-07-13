QT += testlib network widgets websockets
CONFIG += qt warn_on c++17 console
CONFIG -= app_bundle
TEMPLATE = app
TARGET = tst_backend_live

INCLUDEPATH += ../../api ../../models ../../streaming

SOURCES += \
    tst_backend_live.cpp \
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
    ../../api/api_response.h \
    ../../api/hermind_api_client.h \
    ../../models/hermind_user.h \
    ../../models/hermind_workspace.h \
    ../../models/hermind_stream_chat_response.h \
    ../../models/hermind_agent_event.h \
    ../../streaming/hermind_sse_client.h \
    ../../streaming/hermind_websocket_client.h
