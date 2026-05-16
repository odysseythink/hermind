import QtQuick
import QtQuick.Controls
import Hermind

SpinBox {
    property var field
    property string value
    signal changed(string value)

    from: field.min ?? -2147483648
    to: field.max ?? 2147483647
    value: parseInt(value) || 0
    onValueModified: changed(String(value))
}
