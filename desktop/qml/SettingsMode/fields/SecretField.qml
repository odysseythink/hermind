import QtQuick
import QtQuick.Layouts
import QtQuick.Controls
import Hermind

RowLayout {
    property var field
    property string value
    signal changed(string value)

    spacing: 8

    TextField {
        id: secretInput
        Layout.fillWidth: true
        Layout.preferredHeight: 36
        topPadding: 10
        bottomPadding: 10
        text: value
        background: Rectangle {
            implicitHeight: 36
            color: Theme.glassInput
            border.color: secretInput.activeFocus ? Theme.accent : Theme.glassBorder
            border.width: secretInput.activeFocus ? 2 : 1
            radius: 8
        }
        placeholderText: field.help || ""
        color: Theme.textPrimary
        font.pixelSize: 13
        echoMode: showSecret.checked ? TextInput.Normal : TextInput.Password
        onTextChanged: changed(text)
    }

    CheckBox {
        id: showSecret
        text: qsTr("Show")
        checked: false
    }
}
