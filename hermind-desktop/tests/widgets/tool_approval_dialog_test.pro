QT += widgets testlib
CONFIG += qt warn_on depend_includepath testcase c++17

TEMPLATE = app
TARGET = tst_tool_approval_dialog

INCLUDEPATH += $$PWD/../../widgets $$PWD/../../models $$PWD/../..

SOURCES += \
    tst_tool_approval_dialog.cpp \
    ../../widgets/tool_approval_dialog.cpp

HEADERS += \
    ../../widgets/tool_approval_dialog.h
