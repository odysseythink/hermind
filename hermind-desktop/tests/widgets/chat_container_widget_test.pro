QT += widgets testlib network websockets
CONFIG += qt warn_on depend_includepath testcase c++17

TEMPLATE = app
TARGET = tst_chat_container_widget

INCLUDEPATH += $$PWD/../../widgets $$PWD/../../api $$PWD/../../models $$PWD/../../chat $$PWD/../../streaming $$PWD/../..

SOURCES += \
    tst_chat_container_widget.cpp \
    ../../widgets/chat_container_widget.cpp \
    ../../widgets/chat_history_widget.cpp \
    ../../widgets/chat_message_item.cpp \
    ../../widgets/theme_colors.cpp \
    ../../theme_manager.cpp \
    ../../settings_store.cpp \
    ../../chat/chat_stream_handler.cpp \
    ../../chat/agent_event_handler.cpp \
    ../../api/hermind_api_client.cpp \
    ../../api/api_response.cpp \
    ../../models/hermind_user.cpp \
    ../../models/hermind_workspace.cpp \
    ../../models/hermind_workspace_thread.cpp \
    ../../models/hermind_stream_chat_response.cpp \
    ../../models/hermind_agent_event.cpp \
    ../../models/hermind_chat_message.cpp \
    ../../streaming/hermind_sse_client.cpp \
    ../../streaming/hermind_websocket_client.cpp

HEADERS += \
    ../../widgets/chat_container_widget.h \
    ../../widgets/chat_history_widget.h \
    ../../widgets/chat_message_item.h \
    ../../widgets/theme_colors.h \
    ../../theme_manager.h \
    ../../settings_store.h \
    ../../chat/chat_stream_handler.h \
    ../../chat/agent_event_handler.h \
    ../../api/hermind_api_client.h \
    ../../api/api_response.h \
    ../../models/hermind_user.h \
    ../../models/hermind_workspace.h \
    ../../models/hermind_workspace_thread.h \
    ../../models/hermind_stream_chat_response.h \
    ../../models/hermind_agent_event.h \
    ../../models/hermind_chat_message.h \
    ../../streaming/hermind_sse_client.h \
    ../../streaming/hermind_websocket_client.h
