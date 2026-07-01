QT += testlib widgets
CONFIG += qt warn_on c++17 console
CONFIG -= app_bundle
TEMPLATE = app
TARGET = tst_sidebar_footer_widget

INCLUDEPATH += ../../sidebar ../../widgets ../../models ../..

SOURCES += \
    tst_sidebar_footer_widget.cpp \
    ../../sidebar/sidebar_footer_widget.cpp \
    ../../widgets/icon_button.cpp \
    ../../widgets/theme_colors.cpp \
    ../../widgets/theme_style_helper.cpp \
    ../../theme_manager.cpp \
    ../../settings_store.cpp \
    ../../models/hermind_workspace.cpp

HEADERS += \
    ../../sidebar/sidebar_footer_widget.h \
    ../../widgets/icon_button.h \
    ../../widgets/theme_colors.h \
    ../../widgets/theme_style_helper.h \
    ../../theme_manager.h \
    ../../settings_store.h \
    ../../models/hermind_workspace.h
