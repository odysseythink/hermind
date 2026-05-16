import QtQuick
import ".."

Rectangle {
    visible: false
    color: "#80000000"
    anchors.fill: parent

    Text {
        anchors.centerIn: parent
        text: "Drop files here"
        color: "white"
        font.pixelSize: 20
    }
}
