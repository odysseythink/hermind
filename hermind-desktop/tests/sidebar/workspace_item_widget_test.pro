QT += testlib widgets
CONFIG += qt warn_on c++17 console
CONFIG -= app_bundle
TEMPLATE = app
TARGET = tst_workspace_item_widget

INCLUDEPATH += ../../sidebar ../../models ../../widgets ../..

SOURCES += \
    tst_workspace_item_widget.cpp \
    ../../sidebar/workspace_item_widget.cpp \
    ../../sidebar/workspace_list_model.cpp \
    ../../models/hermind_workspace.cpp \
    ../../widgets/theme_colors.cpp \
    ../../widgets/theme_style_helper.cpp \
    ../../theme_manager.cpp \
    ../../settings_store.cpp

HEADERS += \
    ../../sidebar/workspace_item_widget.h \
    ../../sidebar/workspace_list_model.h \
    ../../models/hermind_workspace.h \
    ../../widgets/theme_colors.h \
    ../../widgets/theme_style_helper.h \
    ../../theme_manager.h \
    ../../settings_store.h
