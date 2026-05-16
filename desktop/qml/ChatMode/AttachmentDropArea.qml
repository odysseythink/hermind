import QtQuick
import Hermind

Rectangle {
    id: root
    visible: false
    color: "#80000000"
    anchors.fill: parent
    z: 100

    property var acceptedFiles: []

    Text {
        anchors.centerIn: parent
        text: "Drop files here"
        color: "white"
        font.pixelSize: 20
    }

    DropArea {
        anchors.fill: parent

        onEntered: (drag) => {
            if (drag.hasUrls) {
                root.visible = true
                drag.accepted = true
            } else {
                drag.accepted = false
            }
        }

        onDropped: (drop) => {
            root.visible = false
            if (drop.hasUrls) {
                const urls = drop.urls
                const files = []
                for (let i = 0; i < urls.length; i++) {
                    const url = urls[i]
                    if (url.startsWith("file:///")) {
                        files.push(url.substring(8))
                    } else if (url.startsWith("file://")) {
                        files.push(url.substring(7))
                    } else {
                        files.push(url)
                    }
                }
                root.acceptedFiles = files
                appState.toast("Attached " + files.length + " file(s)")
                // TODO: wire to appState.attachFiles(files) when backend supports it
            }
        }

        onExited: {
            root.visible = false
        }
    }
}
