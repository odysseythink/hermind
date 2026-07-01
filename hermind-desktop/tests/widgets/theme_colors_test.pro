QT += testlib
CONFIG += qt warn_on depend_includepath testcase c++17

TEMPLATE = app

INCLUDEPATH += $$PWD/../../widgets

SOURCES += \
    tst_theme_colors.cpp \
    ../../widgets/theme_colors.cpp

HEADERS += \
    ../../widgets/theme_colors.h
