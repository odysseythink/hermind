QT += testlib
CONFIG += qt warn_on depend_includepath testcase c++17

TEMPLATE = app
TARGET = tst_html_sanitizer

INCLUDEPATH += $$PWD/../../markdown $$PWD/../..

SOURCES += \
    tst_html_sanitizer.cpp \
    ../../markdown/html_sanitizer.cpp

HEADERS += \
    ../../markdown/html_sanitizer.h
