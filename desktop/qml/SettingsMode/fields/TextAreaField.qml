import QtQuick
import QtQuick.Controls
import "../.."

TextArea {
    property var field
    property string value
    signal changed(string value)

    text: value
    placeholderText: field.help || ""
    wrapMode: TextEdit.Wrap
    color: Theme.textPrimary
    background: Rectangle { color: Theme.bg; border.color: Theme.border; radius: 4 }
    onTextChanged: changed(text)
}
