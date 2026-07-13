QT += widgets testlib
CONFIG += qt warn_on depend_includepath testcase c++17

TEMPLATE = app
TARGET = tst_tools_menu

INCLUDEPATH += $$PWD/../../widgets $$PWD/../../models $$PWD/../..

SOURCES += \
    tst_tools_menu.cpp \
    ../../widgets/tools_menu.cpp \
    ../../widgets/slash_commands_tab.cpp \
    ../../widgets/theme_colors.cpp \
    ../../settings_store.cpp \
    ../../theme_manager.cpp

HEADERS += \
    ../../widgets/tools_menu.h \
    ../../widgets/slash_commands_tab.h \
    ../../widgets/theme_colors.h \
    ../../settings_store.h \
    ../../theme_manager.h
