QT += testlib widgets network websockets
CONFIG += qt warn_on c++17 console
CONFIG -= app_bundle
TEMPLATE = app
TARGET = tst_sidebar_widget

INCLUDEPATH += ../../sidebar ../../models ../../api ../../streaming ../../navigation ../../widgets ../../auth ../..

SOURCES += \
    tst_sidebar_widget.cpp \
    ../../sidebar_widget.cpp \
    ../../sidebar/active_workspaces_widget.cpp \
    ../../sidebar/sidebar_footer_widget.cpp \
    ../../sidebar/workspace_item_widget.cpp \
    ../../sidebar/workspace_list_model.cpp \
    ../../api/hermind_api_client.cpp \
    ../../api/api_response.cpp \
    ../../models/hermind_workspace.cpp \
    ../../models/hermind_user.cpp \
    ../../models/hermind_stream_chat_response.cpp \
    ../../models/hermind_agent_event.cpp \
    ../../streaming/hermind_sse_client.cpp \
    ../../streaming/hermind_websocket_client.cpp \
    ../../navigation/navigation_manager.cpp \
    ../../widgets/search_input.cpp \
    ../../widgets/icon_button.cpp \
    ../../widgets/theme_colors.cpp \
    ../../widgets/theme_style_helper.cpp \
    ../../theme_manager.cpp \
    ../../settings_store.cpp \
    ../../auth/auth_manager.cpp

HEADERS += \
    ../../sidebar_widget.h \
    ../../sidebar/active_workspaces_widget.h \
    ../../sidebar/sidebar_footer_widget.h \
    ../../sidebar/workspace_item_widget.h \
    ../../sidebar/workspace_list_model.h \
    ../../api/hermind_api_client.h \
    ../../api/api_response.h \
    ../../models/hermind_workspace.h \
    ../../models/hermind_user.h \
    ../../models/hermind_stream_chat_response.h \
    ../../models/hermind_agent_event.h \
    ../../streaming/hermind_sse_client.h \
    ../../streaming/hermind_websocket_client.h \
    ../../navigation/navigation_manager.h \
    ../../navigation/navigation_route.h \
    ../../widgets/search_input.h \
    ../../widgets/icon_button.h \
    ../../widgets/theme_colors.h \
    ../../widgets/theme_style_helper.h \
    ../../theme_manager.h \
    ../../settings_store.h \
    ../../auth/auth_manager.h \
    ../../auth/auth_state.h

FORMS += \
    ../../sidebar_widget.ui
