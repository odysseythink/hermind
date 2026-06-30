QT += testlib
QT -= gui
CONFIG += qt warn_on depend_includepath testcase c++17

TEMPLATE = app

INCLUDEPATH += $$PWD/../../models $$PWD/../../auth

SOURCES += \
    tst_auth_state.cpp \
    ../../models/hermind_user.cpp

HEADERS += \
    ../../auth/auth_state.h \
    ../../models/hermind_user.h
