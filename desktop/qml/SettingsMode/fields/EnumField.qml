import QtQuick
import QtQuick.Controls
import "../.."

ComboBox {
    property var field
    property string value
    signal changed(string value)

    model: field.enum || []
    currentIndex: model.indexOf(value)
    onActivated: changed(model[index])
}
