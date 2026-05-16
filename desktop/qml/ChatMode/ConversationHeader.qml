import QtQuick
import QtQuick.Controls
import Hermind

Rectangle {
    color: Theme.bg

    Text {
        anchors.centerIn: parent
        text: qsTr("New Conversation")
        color: Theme.textSecondary
        font.pixelSize: 13
    }
}
