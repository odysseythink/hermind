QT += testlib
QT -= gui
CONFIG += qt warn_on depend_includepath testcase c++17

TEMPLATE = app

INCLUDEPATH += $$PWD/../../navigation

SOURCES += \
    tst_navigation_manager.cpp \
    ../../navigation/navigation_manager.cpp

HEADERS += \
    ../../navigation/navigation_manager.h \
    ../../navigation/navigation_route.h
