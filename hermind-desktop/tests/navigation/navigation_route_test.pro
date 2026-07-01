QT += testlib
QT -= gui
CONFIG += qt warn_on depend_includepath testcase c++17

TEMPLATE = app

INCLUDEPATH += $$PWD/../../navigation

SOURCES += \
    tst_navigation_route.cpp

HEADERS += \
    ../../navigation/navigation_route.h
