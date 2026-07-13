QT += widgets testlib
CONFIG += qt warn_on depend_includepath testcase c++17

TEMPLATE = app

INCLUDEPATH += $$PWD/../../widgets $$PWD/../..

SOURCES += \
    tst_widgets.cpp \
    tst_llm_provider_info.cpp \
    tst_llm_model_selector.cpp \
    ../../widgets/workspace_settings_tab.cpp \
    ../../widgets/suggested_messages_editor.cpp \
    ../../widgets/icon_button.cpp \
    ../../widgets/sidebar_menu_button.cpp \
    ../../widgets/search_input.cpp \
    ../../widgets/styled_separator.cpp \
    ../../widgets/rounded_frame.cpp \
    ../../widgets/setting_row.cpp \
    ../../widgets/theme_colors.cpp \
    ../../widgets/theme_style_helper.cpp \
    ../../widgets/llm_provider_info.cpp \
    ../../widgets/llm_model_selector.cpp \
    ../../theme_manager.cpp \
    ../../settings_store.cpp

HEADERS += \
    tst_llm_provider_info.h \
    tst_llm_model_selector.h \
    ../../widgets/workspace_settings_tab.h \
    ../../widgets/suggested_messages_editor.h \
    ../../widgets/icon_button.h \
    ../../widgets/sidebar_menu_button.h \
    ../../widgets/search_input.h \
    ../../widgets/styled_separator.h \
    ../../widgets/rounded_frame.h \
    ../../widgets/setting_row.h \
    ../../widgets/theme_colors.h \
    ../../widgets/theme_style_helper.h \
    ../../widgets/llm_provider_info.h \
    ../../widgets/llm_model_selector.h \
    ../../theme_manager.h \
    ../../settings_store.h
