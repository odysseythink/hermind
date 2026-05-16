cmake_minimum_required(VERSION 3.16)
project(hermind-desktop VERSION 0.3.0 LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_AUTOMOC ON)
set(CMAKE_AUTORCC ON)

enable_testing()
find_package(Qt6 REQUIRED COMPONENTS Core Quick QuickControls2 Network Widgets Test)

set(CPP_SOURCES
    src/main.cpp
    src/HermindProcess.cpp
    src/HermindClient.cpp
    src/AppState.cpp
    src/SSEParser.cpp
    src/TrayIcon.cpp
    src/ShortcutManager.cpp
)

set(CPP_HEADERS
    src/HermindProcess.h
    src/HermindClient.h
    src/AppState.h
    src/SSEParser.h
    src/TrayIcon.h
    src/ShortcutManager.h
)

qt_add_executable(hermind-desktop ${CPP_SOURCES} ${CPP_HEADERS} translations/translations.qrc qml/theme.qrc)

qt_add_qml_module(hermind-desktop
    URI Hermind
    VERSION 1.0
    RESOURCE_PREFIX "/"
    QML_FILES
        qml/main.qml
        qml/AppWindow.qml
        qml/ChatMode/ChatWorkspace.qml
        qml/ChatMode/MessageList.qml
        qml/ChatMode/MessageBubble.qml
        qml/ChatMode/PromptInput.qml
        qml/ChatMode/ConversationHeader.qml
        qml/ChatMode/EmptyState.qml
        qml/ChatMode/ToolCallCard.qml
        qml/ChatMode/StreamingCursor.qml
        qml/ChatMode/StopButton.qml
        qml/ChatMode/AttachmentDropArea.qml
        qml/SettingsMode/SettingsSidebar.qml
        qml/SettingsMode/SettingsPanel.qml
        qml/SettingsMode/ConfigSection.qml
        qml/SettingsMode/editors/ProviderEditor.qml
        qml/SettingsMode/editors/FallbackProviderEditor.qml
        qml/SettingsMode/editors/KeyedInstanceEditor.qml
        qml/SettingsMode/editors/ListElementEditor.qml
        qml/SettingsMode/editors/DefaultModelEditor.qml
        qml/SettingsMode/editors/AuxiliaryEditor.qml
        qml/SettingsMode/editors/SkillsSection.qml
        qml/SettingsMode/fields/StringField.qml
        qml/SettingsMode/fields/NumberField.qml
        qml/SettingsMode/fields/FloatField.qml
        qml/SettingsMode/fields/BoolField.qml
        qml/SettingsMode/fields/EnumField.qml
        qml/SettingsMode/fields/SecretField.qml
        qml/SettingsMode/fields/TextAreaField.qml
        qml/SettingsMode/fields/MultiSelectField.qml
        qml/Shell/TopBar.qml
        qml/Shell/Footer.qml
        qml/Shell/ThemeToggle.qml
        qml/Shell/LanguageToggle.qml
        qml/components/ScrollToBottomButton.qml
        qml/components/Toast.qml
)

target_link_libraries(hermind-desktop PRIVATE
    Qt6::Core
    Qt6::Quick
    Qt6::QuickControls2
    Qt6::Network
    Qt6::Widgets
)

set_target_properties(hermind-desktop PROPERTIES
    WIN32_EXECUTABLE TRUE
    MACOSX_BUNDLE TRUE
    MACOSX_BUNDLE_INFO_PLIST ${CMAKE_SOURCE_DIR}/packaging/macos/Info.plist.in
)

add_executable(test_httplib tests/test_httplib.cpp src/HermindClient.cpp)
target_link_libraries(test_httplib PRIVATE Qt6::Core Qt6::Network Qt6::Test Qt6::Qml)
add_test(NAME test_httplib COMMAND test_httplib)

add_executable(test_sseparser tests/test_sseparser.cpp src/SSEParser.cpp)
target_link_libraries(test_sseparser PRIVATE Qt6::Core Qt6::Test)
add_test(NAME test_sseparser COMMAND test_sseparser)
