QT += widgets testlib
CONFIG += qt warn_on depend_includepath testcase c++17

TEMPLATE = app
TARGET = tst_chat_message_item

INCLUDEPATH += $$PWD/../../widgets $$PWD/../../models $$PWD/../..

SOURCES += \
    tst_chat_message_item.cpp \
    ../../widgets/chat_message_item.cpp \
    ../../widgets/theme_colors.cpp \
    ../../theme_manager.cpp \
    ../../settings_store.cpp \
    ../../models/hermind_chat_message.cpp

HEADERS += \
    ../../widgets/chat_message_item.h \
    ../../widgets/theme_colors.h \
    ../../theme_manager.h \
    ../../settings_store.h \
    ../../models/hermind_chat_message.h
