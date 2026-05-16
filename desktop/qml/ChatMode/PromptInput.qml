import QtQuick
import QtQuick.Layouts
import QtQuick.Controls
import ".."

Rectangle {
    color: Theme.bg
    border.color: Theme.border
    border.width: 1

    RowLayout {
        anchors.fill: parent
        anchors.margins: 12
        spacing: 12

        TextArea {
            id: inputArea
            Layout.fillWidth: true
            Layout.fillHeight: true
            placeholderText: qsTr("Type a message...")
            wrapMode: TextEdit.Wrap
            color: Theme.textPrimary
            background: Rectangle { color: "transparent" }

            Keys.onReturnPressed: (event) => {
                if (event.modifiers & Qt.ShiftModifier) {
                    event.accepted = false
                } else {
                    sendButton.clicked()
                    event.accepted = true
                }
            }
        }

        ColumnLayout {
            spacing: 8
            Button {
                id: sendButton
                text: "Send"
                visible: !appState.isStreaming
                enabled: inputArea.text.trim().length > 0
                onClicked: {
                    appState.sendMessage(inputArea.text.trim())
                    inputArea.clear()
                }
            }
            StopButton {
                visible: appState.isStreaming
            }
            Button {
                text: "Attach"
                enabled: !appState.isStreaming
            }
        }
    }
}
