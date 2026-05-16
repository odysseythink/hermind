import QtQuick
import QtQuick.Controls
import Hermind

Button {
    text: "▼"
    flat: true
    background: Rectangle {
        color: Theme.surface
        border.color: Theme.border
        radius: 16
    }
    contentItem: Text {
        text: parent.text
        color: Theme.textPrimary
        horizontalAlignment: Text.AlignHCenter
        verticalAlignment: Text.AlignVCenter
    }
}
