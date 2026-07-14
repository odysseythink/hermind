QT += widgets testlib network websockets
CONFIG += qt warn_on depend_includepath testcase c++17

TEMPLATE = app

INCLUDEPATH += $$PWD/../.. $$PWD/../../api $$PWD/../../models $$PWD/../../streaming \
    $$PWD/../../auth $$PWD/../../navigation $$PWD/../../widgets $$PWD/../../sidebar \
    $$PWD/../../chat $$PWD/../../markdown

SOURCES += \
    tst_widgets.cpp \
    tst_llm_provider_info.cpp \
    tst_llm_model_selector.cpp \
    tst_agent_config_state.cpp \
    tst_agent_config_tab.cpp \
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
    ../../widgets/agent_config_state.cpp \
    ../../widgets/agent_config_tab.cpp \
    ../../models/hermind_workspace.cpp \
    ../../widgets/ai_provider_settings_tab.cpp \
    ../../api/api_response.cpp \
    ../../api/hermind_api_client.cpp \
    ../../models/hermind_user.cpp \
    ../../models/hermind_workspace_thread.cpp \
    ../../models/hermind_stream_chat_response.cpp \
    ../../models/hermind_chat_message.cpp \
    ../../models/hermind_agent_event.cpp \
    ../../models/hermind_memory.cpp \
    ../../models/hermind_workspace_user.cpp \
    ../../streaming/hermind_sse_client.cpp \
    ../../streaming/hermind_websocket_client.cpp \
    ../../chat/chat_stream_handler.cpp \
    ../../chat/agent_event_handler.cpp \
    ../../auth/auth_manager.cpp \
    ../../theme_manager.cpp \
    ../../settings_store.cpp

HEADERS += \
    tst_llm_provider_info.h \
    tst_llm_model_selector.h \
    tst_agent_config_state.h \
    tst_agent_config_tab.h \
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
    ../../widgets/agent_config_state.h \
    ../../widgets/agent_config_tab.h \
    ../../models/hermind_workspace.h \
    ../../widgets/ai_provider_settings_tab.h \
    ../../api/api_response.h \
    ../../api/hermind_api_client.h \
    ../../models/hermind_user.h \
    ../../models/hermind_workspace_thread.h \
    ../../models/hermind_stream_chat_response.h \
    ../../models/hermind_chat_message.h \
    ../../models/hermind_agent_event.h \
    ../../models/hermind_memory.h \
    ../../models/hermind_workspace_user.h \
    ../../streaming/hermind_sse_client.h \
    ../../streaming/hermind_websocket_client.h \
    ../../chat/chat_stream_handler.h \
    ../../chat/agent_event_handler.h \
    ../../auth/auth_manager.h \
    ../../theme_manager.h \
    ../../settings_store.h
