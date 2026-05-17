import QtQuick
import QtQuick.Layouts
import QtQuick.Controls
import Hermind

Rectangle {
    property string name: ""
    property string status: ""

    color: Theme.glassCard
    border.color: Theme.glassBorder
    border.width: 1
    radius: 8
    height: 40

    // Top highlight
    Rectangle {
        anchors.top: parent.top
        anchors.left: parent.left
        anchors.right: parent.right
        height: 5
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
