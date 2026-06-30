QT += testlib network
CONFIG += qt warn_on c++17 console
CONFIG -= app_bundle
TEMPLATE = app
TARGET = tst_sse_client

INCLUDEPATH += ../../api ../../models ../../streaming

SOURCES += \
    tst_sse_client.cpp \
    ../../api/api_response.cpp \
    ../../models/hermind_stream_chat_response.cpp \
    ../../streaming/hermind_sse_client.cpp

HEADERS += \
    ../../api/api_response.h \
    ../../models/hermind_stream_chat_response.h \
    ../../streaming/hermind_sse_client.h
