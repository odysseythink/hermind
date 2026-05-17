import QtQuick
import QtQuick.Controls
import Hermind

TextArea {
    property var field
    property string value
    signal changed(string value)

    text: value
    placeholderText: field.help || ""
    color: Theme.textPrimary
    font.pixelSize: 13
    wrapMode: TextEdit.Wrap
    background: Rectangle {
        color: Theme.glassInput
        border.color: parent.activeFocus ? Theme.accent : Theme.glassBorder
        border.width: parent.activeFocus ? 2 : 1
        radius: 8
    }
    onTextChanged: changed(text)
}
