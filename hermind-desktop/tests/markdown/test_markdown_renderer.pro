QT += widgets testlib gui
CONFIG += qt warn_on depend_includepath testcase c++17

TARGET = tst_markdown_renderer

INCLUDEPATH += $$PWD/../../markdown $$PWD/../..

SOURCES += \
    tst_markdown_renderer.cpp \
    ../../markdown/markdown_renderer.cpp \
    ../../markdown/qt_builtin_parser.cpp \
    ../../markdown/html_generator.cpp \
    ../../markdown/html_sanitizer.cpp \
    ../../markdown/syntax_highlighter.cpp

HEADERS += \
    ../../markdown/markdown_renderer.h \
    ../../markdown/qt_builtin_parser.h \
    ../../markdown/html_generator.h \
    ../../markdown/html_sanitizer.h \
    ../../markdown/i_markdown_parser.h \
    ../../markdown/syntax_highlighter.h

RESOURCES += test_resources.qrc
