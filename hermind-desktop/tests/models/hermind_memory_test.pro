QT += testlib
CONFIG += qt warn_on c++17 console
CONFIG -= app_bundle
TEMPLATE = app
TARGET = tst_hermind_memory

INCLUDEPATH += ../../models

SOURCES += \
    tst_hermind_memory.cpp \
    ../../models/hermind_memory.cpp

HEADERS += \
    ../../models/hermind_memory.h
