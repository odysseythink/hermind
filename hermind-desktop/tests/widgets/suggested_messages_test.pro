QT += widgets testlib
CONFIG += qt warn_on depend_includepath testcase c++17

TEMPLATE = app
TARGET = tst_suggested_messages

INCLUDEPATH += $$PWD/../../widgets $$PWD/../../models $$PWD/../..

SOURCES += \
    tst_suggested_messages.cpp \
    ../../widgets/suggested_messages.cpp \
    ../../widgets/theme_colors.cpp \
    ../../settings_store.cpp \
    ../../theme_manager.cpp

HEADERS += \
    ../../widgets/suggested_messages.h \
    ../../widgets/theme_colors.h \
    ../../settings_store.h \
    ../../theme_manager.h
