import QtQuick
import QtQuick.Controls
import Hermind

Button {
    text: "⏹"
    flat: true
    ToolTip.text: "Stop generation"
    ToolTip.visible: hovered
    onClicked: appState.cancelGeneration()
}
