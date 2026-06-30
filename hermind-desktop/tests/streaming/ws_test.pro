QT += testlib websockets
CONFIG += qt warn_on c++17 console
CONFIG -= app_bundle
TEMPLATE = app
TARGET = tst_websocket_client

INCLUDEPATH += ../../api ../../models ../../streaming

SOURCES += \
    tst_websocket_client.cpp \
    ../../streaming/hermind_websocket_client.cpp

HEADERS += \
    ../../streaming/hermind_websocket_client.h
