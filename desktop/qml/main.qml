import QtQuick
import QtQuick.Window
import Hermind

Window {
    id: root
    visible: true
    width: 1200
    height: 800
    title: "hermind"
    flags: Qt.Window | Qt.WindowMinimizeButtonHint | Qt.WindowCloseButtonHint

    onClosing: (close) => {
        close.accepted = false
        root.hide()
    }

    Loader {
        anchors.fill: parent
        sourceComponent: appState.status === "ready" ? appWindow : bootScreen
    }

    Component {
        id: bootScreen
        Rectangle {
            color: "#0a0b0d"
            Text {
                anchors.centerIn: parent
                text: appState.status === "error" ? ("Boot failed: " + appState.flashMessage) : "Loading..."
                color: "#e8e6e3"
                font.pixelSize: 16
            }
        }
    }

    Component {
        id: appWindow
        AppWindow {}
    }
}
