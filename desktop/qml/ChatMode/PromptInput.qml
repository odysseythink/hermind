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
            placeholderText: "Type a message..."
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
                enabled: inputArea.text.trim().length > 0 && !appState.isStreaming
                onClicked: {
                    appState.sendMessage(inputArea.text.trim())
                    inputArea.clear()
                }
            }
            Button {
                text: "Attach"
                enabled: !appState.isStreaming
            }
        }
    }
}
