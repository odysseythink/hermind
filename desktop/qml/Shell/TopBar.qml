import QtQuick
import QtQuick.Layouts
import QtQuick.Controls
import ".."

Rectangle {
    color: Theme.surface

    RowLayout {
        anchors.fill: parent
        anchors.leftMargin: 16
        anchors.rightMargin: 16
        spacing: 12

        Text {
            text: qsTr("◈ HERMIND")
            font.family: "monospace"
            font.pixelSize: 14
            font.weight: Font.Bold
            color: Theme.textPrimary
            Layout.alignment: Qt.AlignVCenter
        }

        Item { Layout.fillWidth: true }

        LanguageToggle {
            Layout.preferredWidth: 60
            Layout.preferredHeight: 28
        }

        Button {
            text: Theme.isDark ? "🌙" : "☀️"
            Layout.preferredWidth: 28
            Layout.preferredHeight: 28
            flat: true
            onClicked: Theme.isDark = !Theme.isDark
        }

        ButtonGroup {
            id: modeGroup
        }

        Button {
            text: qsTr("Chat")
            checkable: true
            checked: appState.activeGroup === ""
            ButtonGroup.group: modeGroup
            onClicked: {
                appState.activeGroup = ""
                appState.activeSubKey = ""
            }
        }

        Button {
            text: qsTr("Set")
            checkable: true
            checked: appState.activeGroup !== ""
            ButtonGroup.group: modeGroup
            onClicked: {
                appState.activeGroup = "models"
            }
        }

        Rectangle {
            width: 8; height: 8
            radius: 4
            color: appState.status === "ready" ? "#7ee787" : "#ce9178"
            Layout.alignment: Qt.AlignVCenter
        }

        Text {
            text: appState.status === "ready" ? "READY" : appState.status.toUpperCase()
            font.family: "monospace"
            font.pixelSize: 12
            color: Theme.textSecondary
            Layout.alignment: Qt.AlignVCenter
        }

        Button {
            text: qsTr("Save")
            enabled: appState.dirtyCount > 0
            onClicked: appState.saveConfig()
        }
    }
}
