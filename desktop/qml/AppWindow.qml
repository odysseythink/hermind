import QtQuick
import QtQuick.Layouts
import QtQuick.Controls
import Hermind

Rectangle {
    gradient: Gradient {
        GradientStop { position: 0.0; color: Theme.bgGradientCenter }
        GradientStop { position: 1.0; color: Theme.bgGradientEdge }
    }

    ColumnLayout {
        anchors.fill: parent
        spacing: 0

        TopBar {
            Layout.fillWidth: true
            Layout.preferredHeight: 44
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
                    Layout.preferredWidth: 220
                    Layout.fillHeight: true
                }

                SettingsPanel {
                    Layout.fillWidth: true
                    Layout.fillHeight: true
                }
            }
        }
    }

    // Toast notification
    Toast {
        id: toast
        anchors.horizontalCenter: parent.horizontalCenter
        anchors.bottom: parent.bottom
        anchors.bottomMargin: 24
        message: appState.flashMessage
    }
}
