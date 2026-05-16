import QtQuick
import QtQuick.Layouts
import QtQuick.Controls
import ".."

Rectangle {
    property bool isUser: false
    property string markdownText: ""
    property var toolCalls: []

    color: isUser ? "transparent" : Theme.surface
    border.color: isUser ? Theme.accent : Theme.border
    border.width: 1
    radius: 4

    ColumnLayout {
        anchors.fill: parent
        anchors.margins: 12
        spacing: 8

        Text {
            text: isUser ? "YOU" : "HERMIND"
            font.family: "monospace"
            font.pixelSize: 10
            font.weight: Font.Bold
            color: isUser ? Theme.accent : Theme.textSecondary
        }

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
                text: "Copy"
                flat: true
                onClicked: {
                    // clipboard access in QML
                }
            }
            Button {
                text: "Regenerate"
                flat: true
            }
            Button {
                text: "Delete"
                flat: true
            }
        }
    }
}
