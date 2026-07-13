QT += testlib
CONFIG += testcase console c++17

TARGET = tst_formula_renderer

INCLUDEPATH += ../../markdown

SOURCES += \
    tst_formula_renderer.cpp \
    ../../markdown/formula_renderer.cpp \
    ../../markdown/html_generator.cpp

RESOURCES += test_resources.qrc
