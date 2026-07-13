QT += widgets testlib gui
CONFIG += qt warn_on depend_includepath testcase c++17

TEMPLATE = app
TARGET = tst_markdown_message_item

INCLUDEPATH += $$PWD/../../markdown $$PWD/../../widgets $$PWD/../../models $$PWD/../..

SOURCES += \
    tst_markdown_message_item.cpp \
    ../../widgets/plain_message_item.cpp \
    ../../widgets/markdown_message_item.cpp \
    ../../widgets/theme_colors.cpp \
    ../../theme_manager.cpp \
    ../../settings_store.cpp \
    ../../models/hermind_chat_message.cpp \
    ../../markdown/markdown_renderer.cpp \
    ../../markdown/qt_builtin_parser.cpp \
    ../../markdown/html_generator.cpp \
    ../../markdown/html_sanitizer.cpp

HEADERS += \
    ../../widgets/plain_message_item.h \
    ../../widgets/markdown_message_item.h \
    ../../widgets/theme_colors.h \
    ../../theme_manager.h \
    ../../settings_store.h \
    ../../models/hermind_chat_message.h \
    ../../markdown/markdown_renderer.h \
    ../../markdown/qt_builtin_parser.h \
    ../../markdown/html_generator.h \
    ../../markdown/html_sanitizer.h \
    ../../markdown/i_markdown_parser.h

RESOURCES += ../markdown/test_resources.qrc
