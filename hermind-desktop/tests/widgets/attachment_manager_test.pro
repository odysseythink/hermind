QT += widgets testlib
CONFIG += qt warn_on depend_includepath testcase c++17

TEMPLATE = app
TARGET = tst_attachment_manager

INCLUDEPATH += $$PWD/../../widgets $$PWD/../../models $$PWD/../..

SOURCES += \
    tst_attachment_manager.cpp \
    ../../widgets/attachment_manager.cpp \
    ../../widgets/theme_colors.cpp \
    ../../settings_store.cpp \
    ../../theme_manager.cpp

HEADERS += \
    ../../widgets/attachment_manager.h \
    ../../widgets/theme_colors.h \
    ../../settings_store.h \
    ../../theme_manager.h
