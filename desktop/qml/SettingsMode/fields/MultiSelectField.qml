import QtQuick
import QtQuick.Layouts
import QtQuick.Controls
import "../.."

Flow {
    property var field
    property var value
    signal changed(var value)

    spacing: 8
    Repeater {
        model: field.enum || []
        delegate: Button {
            text: modelData
            checkable: true
            checked: value.includes(modelData)
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
