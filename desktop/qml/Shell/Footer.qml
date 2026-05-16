import QtQuick
import QtQuick.Controls
import ".."

Rectangle {
    color: Theme.bg
    height: 28

    Text {
        anchors.centerIn: parent
        text: appState.flashMessage
        color: appState.status === "error" ? Theme.error : Theme.textSecondary
        font.pixelSize: 11
    }
}
