import QtQuick
import QtQuick.Layouts
import QtQuick.Controls
import Hermind

Rectangle {
    color: Theme.glassSurface

    // Top subtle highlight
    Rectangle {
        anchors.top: parent.top
        anchors.left: parent.left
        anchors.right: parent.right
        height: 1
        color: Theme.glassSurfaceHover
        opacity: 0.3
    }

    RowLayout {
        anchors.fill: parent
        anchors.leftMargin: 16
        anchors.rightMargin: 16
        spacing: 8

        // Logo
        Text {
            text: qsTr("◈ HERMIND")
            font.family: "monospace"
            font.pixelSize: 14
            font.weight: Font.Bold
            color: Theme.textPrimary
            Layout.alignment: Qt.AlignVCenter
        }

        Item { Layout.preferredWidth: 24 }

        // Mode tabs
        RowLayout {
            spacing: 4
            Layout.alignment: Qt.AlignVCenter

            Rectangle {
                Layout.preferredWidth: chatTab.implicitWidth + 20
                Layout.preferredHeight: 28
                radius: 6
                color: appState.activeGroup === "" ? Theme.accent : "transparent"

                Text {
                    id: chatTab
                    anchors.centerIn: parent
                    text: qsTr("对话")
                    font.pixelSize: 13
                    font.weight: appState.activeGroup === "" ? Font.Bold : Font.Normal
                    color: appState.activeGroup === "" ? "#1a1817" : Theme.textSecondary
                }

                MouseArea {
                    anchors.fill: parent
                    cursorShape: Qt.PointingHandCursor
                    onClicked: {
                        appState.activeGroup = ""
                        appState.activeSubKey = ""
                    }
                }
            }

            Rectangle {
                Layout.preferredWidth: settingsTab.implicitWidth + 20
                Layout.preferredHeight: 28
                radius: 6
                color: appState.activeGroup !== "" ? Theme.accent : "transparent"

                Text {
                    id: settingsTab
                    anchors.centerIn: parent
                    text: qsTr("设置")
                    font.pixelSize: 13
                    font.weight: appState.activeGroup !== "" ? Font.Bold : Font.Normal
                    color: appState.activeGroup !== "" ? "#1a1817" : Theme.textSecondary
                }

                MouseArea {
                    anchors.fill: parent
                    cursorShape: Qt.PointingHandCursor
                    onClicked: {
                        appState.activeGroup = "models"
                        appState.activeSubKey = ""
                    }
                }
            }
        }

        Item { Layout.fillWidth: true }

        // Save status
        RowLayout {
            spacing: 6
            Layout.alignment: Qt.AlignVCenter
            visible: appState.activeGroup !== ""

            Rectangle {
                width: 8
                height: 8
                radius: 4
                color: appState.dirtyCount > 0 ? Theme.warning : Theme.success
            }

            Text {
                text: appState.dirtyCount > 0 ? qsTr("有未保存的更改") : qsTr("已全部保存")
                font.pixelSize: 12
                color: Theme.textSecondary
            }
        }

        // Theme toggle
        Rectangle {
            Layout.preferredWidth: 28
            Layout.preferredHeight: 24
            radius: 4
            color: Theme.cardBg
            border.color: Theme.border
            border.width: 1

            Text {
                anchors.centerIn: parent
                text: Theme.isDark ? "\uD83C\uDF19" : "\u2600"
                font.pixelSize: 13
                color: Theme.textSecondary
            }

            MouseArea {
                anchors.fill: parent
                cursorShape: Qt.PointingHandCursor
                onClicked: Theme.isDark = !Theme.isDark
            }
        }

        // Language toggle
        Rectangle {
            id: langToggle
            Layout.preferredWidth: 36
            Layout.preferredHeight: 24
            radius: 4
            color: Theme.cardBg
            border.color: Theme.border
            border.width: 1

            property int langIndex: 0
            property var langs: [
                { text: "EN", code: "en" },
                { text: "\u4E2D", code: "zh_CN" }
            ]

            Text {
                anchors.centerIn: parent
                text: langToggle.langs[langToggle.langIndex].text
                font.pixelSize: 11
                color: Theme.textSecondary
            }

            MouseArea {
                anchors.fill: parent
                cursorShape: Qt.PointingHandCursor
                onClicked: {
                    langToggle.langIndex = (langToggle.langIndex + 1) % langToggle.langs.length
                    appState.setLanguage(langToggle.langs[langToggle.langIndex].code)
                }
            }
        }

        // Font size toggle
        Rectangle {
            Layout.preferredWidth: 28
            Layout.preferredHeight: 24
            radius: 4
            color: Theme.cardBg
            border.color: Theme.border
            border.width: 1

            Text {
                anchors.centerIn: parent
                text: qsTr("A")
                font.pixelSize: 13
                font.weight: Font.Bold
                color: Theme.textSecondary
            }

            MouseArea {
                anchors.fill: parent
                cursorShape: Qt.PointingHandCursor
                onClicked: {
                    // Placeholder for font size toggle
                }
            }
        }

        // Save button
        Rectangle {
            Layout.preferredWidth: saveText.implicitWidth + 20
            Layout.preferredHeight: 28
            radius: 6
            color: appState.dirtyCount > 0 ? Theme.accent : Theme.cardBg
            border.color: appState.dirtyCount > 0 ? Theme.accent : Theme.border
            border.width: 1

            Text {
                id: saveText
                anchors.centerIn: parent
                text: qsTr("保存")
                font.pixelSize: 13
                font.weight: appState.dirtyCount > 0 ? Font.Bold : Font.Normal
                color: appState.dirtyCount > 0 ? "#1a1817" : Theme.textSecondary
            }

            MouseArea {
                anchors.fill: parent
                cursorShape: Qt.PointingHandCursor
                enabled: appState.dirtyCount > 0
                onClicked: appState.saveConfig()
            }
        }
    }

    // Bottom border
    Rectangle {
        anchors.bottom: parent.bottom
        anchors.left: parent.left
        anchors.right: parent.right
        height: 1
        color: Theme.glassBorder
    }
}
