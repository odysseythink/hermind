QT += testlib
CONFIG += qt warn_on c++17 console
CONFIG -= app_bundle
TEMPLATE = app
TARGET = tst_chat_message

INCLUDEPATH += ../../api ../../models

SOURCES += \
    tst_chat_message.cpp \
    ../../api/api_response.cpp \
    ../../models/hermind_chat_message.cpp

HEADERS += \
    ../../api/api_response.h \
    ../../models/hermind_chat_message.h
