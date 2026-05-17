import QtQuick
import QtQuick.Controls
import Hermind

Rectangle {
    color: Theme.glassSurface

    Text {
        anchors.left: parent.left
        anchors.leftMargin: 16
        anchors.verticalCenter: parent.verticalCenter
        text: appState.activeGroup === "" ? qsTr("New Conversation") : ""
        color: Theme.textSecondary
        font.pixelSize: 12
    }

    // Bottom border
    Rectangle {
        anchors.bottom: parent.bottom
        anchors.left: parent.left
        anchors.right: parent.right
        height: 1
        color: Theme.glassBorder
    }
}
