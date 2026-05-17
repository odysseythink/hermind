import QtQuick
import QtQuick.Layouts
import QtQuick.Controls
import Hermind

Rectangle {
    color: Theme.bg

    RowLayout {
        anchors.fill: parent
        anchors.margins: 12
        anchors.topMargin: 8
        anchors.bottomMargin: 12
        spacing: 12

        // Input container with border
        Rectangle {
            id: inputContainer
            Layout.fillWidth: true
            Layout.fillHeight: true
            radius: 24
            color: Theme.glassInput

            // Top highlight
            Rectangle {
                anchors.top: parent.top
                anchors.left: parent.left
                anchors.right: parent.right
                height: 10
                radius: parent.radius
                color: Theme.glassSurfaceHover
                opacity: 0.12
                Rectangle {
                    anchors.bottom: parent.bottom
                    anchors.left: parent.left
                    anchors.right: parent.right
                    height: parent.height * 0.5
                    color: parent.parent.color
                    opacity: 1.0
                }
            }

            // Border
            Rectangle {
                anchors.fill: parent
                color: "transparent"
                radius: parent.radius
                border.color: Theme.glassBorder
                border.width: 1
            }

            // Focus glow
            Rectangle {
                anchors.fill: parent
                anchors.margins: -2
                radius: parent.radius + 2
                color: Theme.glassGlow
                opacity: inputArea.activeFocus ? 0.6 : 0
                visible: opacity > 0
                Behavior on opacity { NumberAnimation { duration: 200 } }
                z: -1
            }

            RowLayout {
                anchors.fill: parent
                anchors.leftMargin: 16
                anchors.rightMargin: 8
                spacing: 8

                // Attach button (paperclip icon)
                Text {
                    text: "\uD83D\uDCCE"
                    font.pixelSize: 18
                    color: Theme.textSecondary
                    Layout.alignment: Qt.AlignVCenter

                    MouseArea {
                        anchors.fill: parent
                        cursorShape: Qt.PointingHandCursor
                        enabled: !appState.isStreaming
                        onClicked: {
                            // Placeholder for attach functionality
                        }
                    }
                }

                TextArea {
                    id: inputArea
                    Layout.fillWidth: true
                    Layout.fillHeight: true
                    placeholderText: qsTr("Type a message...")
                    wrapMode: TextEdit.Wrap
                    color: Theme.textPrimary
                    font.pixelSize: 14
                    background: Rectangle { color: "transparent" }
                    verticalAlignment: TextEdit.AlignVCenter

                    Keys.onReturnPressed: (event) => {
                        if (event.modifiers & Qt.ShiftModifier) {
                            event.accepted = false
                        } else {
                            sendBtnArea.clicked()
                            event.accepted = true
                        }
                    }
                }

                // Send button (paper plane icon in yellow circle)
                Rectangle {
                    Layout.preferredWidth: 36
                    Layout.preferredHeight: 36
                    radius: 18
                    color: inputArea.text.trim().length > 0 && !appState.isStreaming ? Theme.accent : Theme.glassSurface

                    Text {
                        anchors.centerIn: parent
                        text: "\u2708"
                        font.pixelSize: 16
                        color: inputArea.text.trim().length > 0 && !appState.isStreaming ? "#1a1817" : Theme.textSecondary
                        rotation: -45
                    }

                    MouseArea {
                        id: sendBtnArea
                        anchors.fill: parent
                        cursorShape: Qt.PointingHandCursor
                        enabled: inputArea.text.trim().length > 0 && !appState.isStreaming
                        onClicked: {
                            appState.sendMessage(inputArea.text.trim())
                            inputArea.clear()
                        }
                    }
                }
            }
        }
    }
}
