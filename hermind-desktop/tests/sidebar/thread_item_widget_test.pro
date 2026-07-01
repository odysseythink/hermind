QT += testlib widgets
CONFIG += qt warn_on c++17 console
CONFIG -= app_bundle
TEMPLATE = app
TARGET = tst_thread_item_widget

INCLUDEPATH += ../../sidebar ../../models ../../widgets ../..

SOURCES += \
    tst_thread_item_widget.cpp \
    ../../sidebar/thread_item_widget.cpp \
    ../../models/hermind_workspace_thread.cpp \
    ../../widgets/theme_colors.cpp \
    ../../widgets/theme_style_helper.cpp \
    ../../theme_manager.cpp \
    ../../settings_store.cpp

HEADERS += \
    ../../sidebar/thread_item_widget.h \
    ../../models/hermind_workspace_thread.h \
    ../../widgets/theme_colors.h \
    ../../widgets/theme_style_helper.h \
    ../../theme_manager.h \
    ../../settings_store.h
