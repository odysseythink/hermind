import QtQuick
import QtQuick.Layouts
import QtQuick.Controls
import Hermind

SpinBox {
    property var field
    property string fieldValue
    signal changed(string value)
    Layout.fillWidth: true

    value: parseInt(fieldValue) || 0
    from: field.min !== undefined ? field.min : -999999
    to: field.max !== undefined ? field.max : 999999
    onValueModified: changed(String(value))

    contentItem: TextField {
        text: parent.textFromValue(parent.value, parent.locale)
        font.pixelSize: 13
        color: Theme.textPrimary
        horizontalAlignment: Qt.AlignHCenter
        verticalAlignment: Qt.AlignVCenter
        readOnly: !parent.editable
        validator: parent.validator
        inputMethodHints: Qt.ImhDigitsOnly
        background: Rectangle {
            color: Theme.glassInput
            border.color: parent.activeFocus ? Theme.accent : Theme.glassBorder
            border.width: parent.activeFocus ? 2 : 1
            radius: 8
        }
    }

    up.indicator: Rectangle {
        x: parent.width - width
        height: parent.height
        implicitWidth: 28
        radius: 8
        color: parent.up.pressed ? Theme.glassSurfaceHover : Theme.glassSurface
        border.color: Theme.glassBorder
        Text {
            text: "+"
            font.pixelSize: 16
            color: Theme.textSecondary
            anchors.centerIn: parent
        }
    }

    down.indicator: Rectangle {
        x: 0
        height: parent.height
        implicitWidth: 28
        radius: 8
        color: parent.down.pressed ? Theme.glassSurfaceHover : Theme.glassSurface
        border.color: Theme.glassBorder
        Text {
            text: "-"
            font.pixelSize: 16
            color: Theme.textSecondary
            anchors.centerIn: parent
        }
    }

    background: Rectangle {
        color: "transparent"
    }
}
