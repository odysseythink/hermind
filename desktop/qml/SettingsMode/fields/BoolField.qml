import QtQuick
import QtQuick.Controls
import Hermind

Switch {
    property var field
    property string value
    signal changed(string value)

    checked: value === "true"
    onCheckedChanged: changed(checked ? "true" : "false")

    indicator: Rectangle {
        implicitWidth: 44
        implicitHeight: 24
        x: parent.leftPadding
        y: parent.height / 2 - height / 2
        radius: 12
        color: parent.checked ? Theme.accent : Theme.border
        border.color: parent.checked ? Theme.accent : Theme.border

        Rectangle {
            x: parent.parent.checked ? parent.width - width - 2 : 2
            y: 2
            width: 20
            height: 20
            radius: 10
            color: "#ffffff"
            Behavior on x {
                NumberAnimation { duration: 120; easing.type: Easing.InOutQuad }
            }
        }
    }

    contentItem: Text {
        text: parent.field ? (parent.field.label || parent.field.name) : ""
        font.pixelSize: 13
        color: Theme.textPrimary
        verticalAlignment: Text.AlignVCenter
        leftPadding: parent.indicator.width + 8
    }
}
