import QtQuick
import QtQuick.Layouts
import QtQuick.Controls
import Hermind

TextField {
    property var field
    property string value
    signal changed(string value)

    Layout.fillWidth: true
    Layout.preferredHeight: 36
    topPadding: 10
    bottomPadding: 10
    text: value
    placeholderText: field.help || ""
    color: Theme.textPrimary
    font.pixelSize: 13
    background: Rectangle {
        implicitHeight: 36
        color: Theme.glassInput
        border.color: parent.activeFocus ? Theme.accent : Theme.glassBorder
        border.width: parent.activeFocus ? 2 : 1
        radius: 8
    }
    onTextChanged: changed(text)
}
