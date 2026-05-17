import QtQuick
import QtQuick.Layouts
import QtQuick.Controls
import Hermind

Flow {
    property var field
    property var value
    signal changed(var value)

    spacing: 8
    Repeater {
        model: field.enum || []
        delegate: Rectangle {
            Layout.preferredWidth: chipText.implicitWidth + 20
            Layout.preferredHeight: 32
            radius: 16
            color: checked ? Qt.rgba(255, 184, 0, 0.25) : Theme.glassCard
            border.color: checked ? Theme.accent : Theme.glassBorder
            border.width: 1

            property bool checked: value.includes(modelData)

            Text {
                id: chipText
                anchors.centerIn: parent
                text: modelData
                font.pixelSize: 12
                font.weight: parent.checked ? Font.Bold : Font.Normal
                color: parent.checked ? "#1a1817" : Theme.textSecondary
            }

            MouseArea {
                anchors.fill: parent
                cursorShape: Qt.PointingHandCursor
                onClicked: {
                    const arr = [...value]
                    const idx = arr.indexOf(modelData)
                    if (idx >= 0) arr.splice(idx, 1)
                    else arr.push(modelData)
                    changed(arr)
                }
            }
        }
    }
}
