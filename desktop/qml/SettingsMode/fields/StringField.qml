import QtQuick
import QtQuick.Controls
import Hermind

TextField {
    property var field
    property string value
    signal changed(string value)

    text: value
    placeholderText: field.help || ""
    color: Theme.textPrimary
    background: Rectangle { color: Theme.bg; border.color: Theme.border; radius: 4 }
    onTextChanged: changed(text)
}
