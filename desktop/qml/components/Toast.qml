import QtQuick
import QtQuick.Controls
import Hermind

GlassPanel {
    property string message: ""

    visible: message.length > 0
    baseColor: Theme.glassCard
    highlightStrength: 0.1
    radius: 8
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
