import QtQuick
import QtQuick.Controls
import Hermind

ApplicationWindow {
    id: root
    visible: true
    width: 1200
    height: 800
    title: "hermind"

    Loader {
        anchors.fill: parent
        sourceComponent: appState.status === "ready" ? appWindow : bootScreen
    }

    Component {
        id: bootScreen
        Rectangle {
            color: Theme.bg
            Text {
                anchors.centerIn: parent
                text: appState.status === "error" ? ("Boot failed: " + appState.flashMessage) : "Loading..."
                color: Theme.textPrimary
                font.pixelSize: 16
            }
        }
    }

    Component {
        id: appWindow
        AppWindow {}
    }
}
