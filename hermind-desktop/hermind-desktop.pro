QT += widgets

CONFIG += c++17

# You can make your code fail to compile if it uses deprecated APIs.
# In order to do so, uncomment the following line.
#DEFINES += QT_DISABLE_DEPRECATED_BEFORE=0x060000    # disables all the APIs deprecated before Qt 6.0.0

SOURCES += \
    main.cpp \
    main_chat_widget.cpp \
    main_setting_widget.cpp \
    mainwindow.cpp

HEADERS += \
    main_chat_widget.h \
    main_setting_widget.h \
    mainwindow.h

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
