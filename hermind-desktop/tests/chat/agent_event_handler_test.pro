QT += testlib network
CONFIG += qt warn_on c++17 console
CONFIG -= app_bundle
TEMPLATE = app
TARGET = tst_agent_event_handler

INCLUDEPATH += ../../api ../../models ../../chat

SOURCES += \
    tst_agent_event_handler.cpp \
    ../../chat/agent_event_handler.cpp \
    ../../api/api_response.cpp \
    ../../models/hermind_chat_message.cpp \
    ../../models/hermind_agent_event.cpp

HEADERS += \
    ../../chat/agent_event_handler.h \
    ../../api/api_response.h \
    ../../models/hermind_chat_message.h \
    ../../models/hermind_agent_event.h
