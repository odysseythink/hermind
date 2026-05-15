cmake_minimum_required(VERSION 3.16)
project(hermind-desktop VERSION 0.3.0 LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_AUTOMOC ON)
set(CMAKE_AUTORCC ON)

enable_testing()
find_package(Qt6 REQUIRED COMPONENTS Core Gui Widgets Network Test)

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
    src/shortcutmanager.cpp
    src/trayicon.cpp
    src/settingsdialog.cpp
    src/topbar.cpp
    src/statusfooter.cpp
    src/emptystatewidget.cpp
    src/conversationheader.cpp
    resources/resources.qrc
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
    src/shortcutmanager.h
    src/trayicon.h
    src/settingsdialog.h
    src/topbar.h
    src/statusfooter.h
    src/emptystatewidget.h
    src/conversationheader.h
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
    MACOSX_BUNDLE_INFO_PLIST ${CMAKE_SOURCE_DIR}/packaging/macos/Info.plist.in
)

add_executable(test_httplib tests/test_httplib.cpp src/httplib.cpp)
target_link_libraries(test_httplib PRIVATE Qt6::Core Qt6::Network Qt6::Test)
add_test(NAME test_httplib COMMAND test_httplib)

add_executable(test_sseparser tests/test_sseparser.cpp src/sseparser.cpp)
target_link_libraries(test_sseparser PRIVATE Qt6::Core Qt6::Test)
add_test(NAME test_sseparser COMMAND test_sseparser)
