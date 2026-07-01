QT += widgets testlib
CONFIG += qt warn_on depend_includepath testcase c++17

TEMPLATE = app

INCLUDEPATH += $$PWD/../../widgets $$PWD/../..

SOURCES += \
    tst_buttons.cpp \
    ../../widgets/icon_button.cpp \
    ../../widgets/sidebar_menu_button.cpp \
    ../../widgets/theme_colors.cpp \
    ../../widgets/theme_style_helper.cpp \
    ../../theme_manager.cpp \
    ../../settings_store.cpp

HEADERS += \
    ../../widgets/icon_button.h \
    ../../widgets/sidebar_menu_button.h \
    ../../widgets/theme_colors.h \
    ../../widgets/theme_style_helper.h \
    ../../theme_manager.h \
    ../../settings_store.h
