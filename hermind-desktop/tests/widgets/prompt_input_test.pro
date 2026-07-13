QT += widgets testlib
CONFIG += qt warn_on depend_includepath testcase c++17

TEMPLATE = app
TARGET = tst_prompt_input

INCLUDEPATH += $$PWD/../../widgets $$PWD/../../models $$PWD/../..

SOURCES += \
    tst_prompt_input.cpp \
    ../../widgets/prompt_input.cpp \
    ../../widgets/theme_colors.cpp \
    ../../settings_store.cpp \
    ../../theme_manager.cpp

HEADERS += \
    ../../widgets/prompt_input.h \
    ../../widgets/theme_colors.h \
    ../../settings_store.h \
    ../../theme_manager.h
