QT += widgets testlib
CONFIG += qt warn_on depend_includepath testcase c++17

TEMPLATE = app
TARGET = tst_chat_history_widget

INCLUDEPATH += $$PWD/../../widgets $$PWD/../../models $$PWD/../../markdown $$PWD/../..

SOURCES += \
    tst_chat_history_widget.cpp \
    ../../widgets/chat_history_widget.cpp \
    ../../widgets/plain_message_item.cpp \
    ../../widgets/markdown_message_item.cpp \
    ../../widgets/theme_colors.cpp \
    ../../theme_manager.cpp \
    ../../settings_store.cpp \
    ../../models/hermind_chat_message.cpp \
    ../../markdown/markdown_renderer.cpp \
    ../../markdown/qt_builtin_parser.cpp \
    ../../markdown/html_generator.cpp \
    ../../markdown/html_sanitizer.cpp \
    ../../markdown/syntax_highlighter.cpp \
    ../../markdown/formula_renderer.cpp

HEADERS += \
    ../../widgets/chat_history_widget.h \
    ../../widgets/plain_message_item.h \
    ../../widgets/markdown_message_item.h \
    ../../widgets/theme_colors.h \
    ../../theme_manager.h \
    ../../settings_store.h \
    ../../models/hermind_chat_message.h \
    ../../markdown/markdown_renderer.h \
    ../../markdown/i_markdown_parser.h \
    ../../markdown/qt_builtin_parser.h \
    ../../markdown/html_generator.h \
    ../../markdown/html_sanitizer.h \
    ../../markdown/syntax_highlighter.h \
    ../../markdown/formula_renderer.h

RESOURCES += ../markdown/test_resources.qrc
