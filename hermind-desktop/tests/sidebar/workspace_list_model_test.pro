QT += testlib
CONFIG += qt warn_on c++17 console
CONFIG -= app_bundle
TEMPLATE = app
TARGET = tst_workspace_list_model

INCLUDEPATH += ../../sidebar ../../models

SOURCES += \
    tst_workspace_list_model.cpp \
    ../../sidebar/workspace_list_model.cpp \
    ../../models/hermind_workspace.cpp

HEADERS += \
    ../../sidebar/workspace_list_model.h \
    ../../models/hermind_workspace.h
