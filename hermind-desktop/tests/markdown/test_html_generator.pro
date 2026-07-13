QT += testlib gui
CONFIG += testcase console c++17

TARGET = tst_html_generator

INCLUDEPATH += ../../markdown

SOURCES += \
    tst_html_generator.cpp \
    ../../markdown/html_sanitizer.cpp \
    ../../markdown/qt_builtin_parser.cpp \
    ../../markdown/html_generator.cpp

RESOURCES += test_resources.qrc
