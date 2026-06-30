QT += widgets network websockets

CONFIG += c++17

INCLUDEPATH += $$PWD/api $$PWD/models $$PWD/streaming $$PWD/auth

# You can make your code fail to compile if it uses deprecated APIs.
# In order to do so, uncomment the following line.
#DEFINES += QT_DISABLE_DEPRECATED_BEFORE=0x060000    # disables all the APIs deprecated before Qt 6.0.0

SOURCES += \
    main.cpp \
    main_chat_widget.cpp \
    main_setting_widget.cpp \
    mainwindow.cpp \
    settings_store.cpp \
    api/api_response.cpp \
    models/hermind_user.cpp \
    models/hermind_workspace.cpp \
    api/hermind_api_client.cpp \
    models/hermind_stream_chat_response.cpp \
    models/hermind_agent_event.cpp \
    streaming/hermind_sse_client.cpp \
    streaming/hermind_websocket_client.cpp \
    auth/auth_manager.cpp

HEADERS += \
    main_chat_widget.h \
    main_setting_widget.h \
    mainwindow.h \
    settings_store.h \
    api/api_response.h \
    models/hermind_user.h \
    models/hermind_workspace.h \
    api/hermind_api_client.h \
    models/hermind_stream_chat_response.h \
    models/hermind_agent_event.h \
    streaming/hermind_sse_client.h \
    streaming/hermind_websocket_client.h \
    auth/auth_state.h \
    auth/auth_manager.h

FORMS += \
    main_chat_widget.ui \
    main_setting_widget.ui \
    mainwindow.ui

RESOURCES += \
    resources.qrc

# Default rules for deployment.
qnx: target.path = /tmp/$${TARGET}/bin
else: unix:!android: target.path = /opt/$${TARGET}/bin
!isEmpty(target.path): INSTALLS += target
