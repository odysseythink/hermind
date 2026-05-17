import QtQuick
import QtQuick.Layouts
import QtQuick.Controls
import Hermind

Rectangle {
    color: Theme.bg

    ColumnLayout {
        anchors.fill: parent
        spacing: 0

        TopBar {
            Layout.fillWidth: true
            Layout.preferredHeight: 48
        }

        StackLayout {
            id: modeStack
            Layout.fillWidth: true
            Layout.fillHeight: true
            currentIndex: appState.activeGroup === "" ? 0 : 1

            ChatWorkspace {
                Layout.fillWidth: true
                Layout.fillHeight: true
            }

            RowLayout {
                Layout.fillWidth: true
                Layout.fillHeight: true
                spacing: 0

                SettingsSidebar {
                    Layout.preferredWidth: 260
                    Layout.fillHeight: true
                }

                SettingsPanel {
                    Layout.fillWidth: true
                    Layout.fillHeight: true
                }
            }
        }

        Footer {
            Layout.fillWidth: true
            Layout.preferredHeight: 28
        }
    }
}
