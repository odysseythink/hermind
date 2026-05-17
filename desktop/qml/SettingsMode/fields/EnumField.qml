import QtQuick
import QtQuick.Layouts
import QtQuick.Controls
import Hermind

ComboBox {
    id: control
    property var field
    property string value
    signal changed(string value)

    Layout.fillWidth: true
    implicitHeight: 36
    model: field.enum || []
    currentIndex: model.indexOf(value)
    onActivated: (index) => changed(model[index])

    contentItem: Text {
        text: parent.displayText
        font.pixelSize: 13
        color: Theme.textPrimary
        verticalAlignment: Text.AlignVCenter
        elide: Text.ElideRight
    }

    background: Rectangle {
        color: Theme.glassInput
        border.color: parent.down ? Theme.accent : Theme.glassBorder
        border.width: parent.down ? 2 : 1
        radius: 8
    }

    popup: Popup {
        y: parent.height + 4
        width: parent.width
        implicitHeight: contentItem.implicitHeight + 16
        padding: 8

        contentItem: ListView {
            clip: true
            implicitHeight: contentHeight
            model: control.delegateModel
            currentIndex: control.highlightedIndex
            ScrollIndicator.vertical: ScrollIndicator {}
        }

        background: Rectangle {
            color: Theme.glassCard
            border.color: Theme.glassBorder
            radius: 8
        }
    }
}
