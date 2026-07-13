QT += testlib network
CONFIG += qt warn_on c++17 console
CONFIG -= app_bundle
TEMPLATE = app
TARGET = tst_chat_stream_handler

INCLUDEPATH += ../../api ../../models ../../chat

SOURCES += \
    tst_chat_stream_handler.cpp \
    ../../chat/chat_stream_handler.cpp \
    ../../api/api_response.cpp \
    ../../models/hermind_chat_message.cpp \
    ../../models/hermind_stream_chat_response.cpp

HEADERS += \
    ../../chat/chat_stream_handler.h \
    ../../api/api_response.h \
    ../../models/hermind_chat_message.h \
    ../../models/hermind_stream_chat_response.h
