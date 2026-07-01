QT += testlib widgets
CONFIG += qt warn_on depend_includepath testcase c++17

TEMPLATE = app

INCLUDEPATH += $$PWD/../.. $$PWD/../../auth

SOURCES += \
    tst_theme_manager.cpp \
    ../../theme_manager.cpp \
    ../../settings_store.cpp

HEADERS += \
    ../../theme_manager.h \
    ../../settings_store.h
