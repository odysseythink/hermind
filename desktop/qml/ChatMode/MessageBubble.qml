import QtQuick
import QtQuick.Layouts
import QtQuick.Controls
import ".."

Rectangle {
    property bool isUser: false
    property string markdownText: ""
    property var toolCalls: []
    property bool isStreaming: false

    color: isUser ? "transparent" : Theme.surface
    border.color: isUser ? Theme.accent : Theme.border
    border.width: 1
    radius: 4

    ColumnLayout {
        anchors.fill: parent
        anchors.margins: 12
        spacing: 8

        Text {
            text: isUser ? qsTr("YOU") : qsTr("HERMIND")
            font.family: "monospace"
            font.pixelSize: 10
            font.weight: Font.Bold
            color: isUser ? Theme.accent : Theme.textSecondary
        }

        RowLayout {
            Layout.fillWidth: true
            spacing: 4

            TextEdit {
                id: contentEdit
                text: markdownText
                readOnly: true
                selectByMouse: true
                textFormat: TextEdit.MarkdownText
                wrapMode: TextEdit.Wrap
                color: Theme.textPrimary
                font.pixelSize: 13
                Layout.fillWidth: true
            }

            StreamingCursor {
                visible: isStreaming
                Layout.alignment: Qt.AlignBottom
            }
        }

        ColumnLayout {
            visible: toolCalls.length > 0
            spacing: 4
            Repeater {
                model: toolCalls
                delegate: ToolCallCard {
                    name: modelData.name || ""
                    status: modelData.status || ""
                }
            }
        }

        RowLayout {
            visible: !isUser
            spacing: 8
            Layout.alignment: Qt.AlignRight

            Button {
                text: qsTr("Copy")
                flat: true
                onClicked: {
                    const text = contentEdit.selectedText.length > 0 ? contentEdit.selectedText : contentEdit.text
                    Qt.application.clipboard.setText(text)
                    appState.toast("Copied to clipboard")
                }
            }
            Button {
                text: qsTr("Regenerate")
                flat: true
            }
            Button {
                text: qsTr("Delete")
                flat: true
            }
        }
    }
}
