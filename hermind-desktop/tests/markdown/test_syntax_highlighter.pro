QT += testlib gui
CONFIG += testcase console c++17

TARGET = tst_syntax_highlighter

INCLUDEPATH += ../../markdown

SOURCES += \
    tst_syntax_highlighter.cpp \
    ../../markdown/syntax_highlighter.cpp \
    ../../markdown/html_sanitizer.cpp \
    ../../markdown/qt_builtin_parser.cpp \
    ../../markdown/html_generator.cpp

RESOURCES += test_resources.qrc
