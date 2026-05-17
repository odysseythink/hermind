import QtQuick
import QtQuick.Layouts
import QtQuick.Controls
import Hermind

Rectangle {
    property string name: ""
    property string status: ""

    color: Theme.surfaceHover
    border.color: Theme.border
    border.width: 1
    radius: 4
    height: 40

    RowLayout {
        anchors.fill: parent
        anchors.margins: 8
        spacing: 8

        Text {
            text: "🔧"
            font.pixelSize: 14
        }

        Text {
            text: name
            color: Theme.textPrimary
            font.pixelSize: 12
            font.weight: Font.Medium
            Layout.fillWidth: true
            elide: Text.ElideRight
        }

        Text {
            text: status
            color: status === "done" ? Theme.success
                  : status === "error" ? Theme.error
                  : Theme.accent
            font.pixelSize: 11
            font.family: "monospace"
        }
    }
}
