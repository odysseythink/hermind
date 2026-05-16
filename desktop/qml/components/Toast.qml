import QtQuick
import QtQuick.Controls
import Hermind

Rectangle {
    property string message: ""

    visible: message.length > 0
    color: Theme.surface
    border.color: Theme.border
    radius: 4
    width: toastText.width + 24
    height: toastText.height + 16

    Text {
        id: toastText
        anchors.centerIn: parent
        text: message
        color: Theme.textPrimary
        font.pixelSize: 13
    }
}
