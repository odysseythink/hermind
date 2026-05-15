cmake_minimum_required(VERSION 3.16)
project(hermind-desktop VERSION 0.3.0 LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_AUTOMOC ON)
set(CMAKE_AUTORCC ON)

find_package(Qt6 REQUIRED COMPONENTS Core Gui Widgets Network)

set(SOURCES
    src/main.cpp
    src/hermindprocess.cpp
    src/httplib.cpp
    src/sseparser.cpp
    src/appwindow.cpp
    src/sessionlistwidget.cpp
    src/chatwidget.cpp
    src/messagebubble.cpp
    src/promptinput.cpp
    src/settingsdialog.cpp
    src/trayicon.cpp
    src/shortcutmanager.cpp
    src/dragdrophandler.cpp
    src/configstore.cpp
)

set(HEADERS
    src/hermindprocess.h
    src/httplib.h
    src/sseparser.h
    src/appwindow.h
    src/sessionlistwidget.h
    src/chatwidget.h
    src/messagebubble.h
    src/promptinput.h
    src/settingsdialog.h
    src/trayicon.h
    src/shortcutmanager.h
    src/dragdrophandler.h
    src/configstore.h
)

add_executable(hermind-desktop ${SOURCES} ${HEADERS})

target_link_libraries(hermind-desktop PRIVATE
    Qt6::Core
    Qt6::Gui
    Qt6::Widgets
    Qt6::Network
)

set_target_properties(hermind-desktop PROPERTIES
    WIN32_EXECUTABLE TRUE
    MACOSX_BUNDLE TRUE
)
