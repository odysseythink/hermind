import QtQuick
import QtQuick.Controls
import Hermind

TextField {
    property var field
    property string value
    signal changed(string value)

    text: value
    validator: DoubleValidator { bottom: field.min ?? -Infinity; top: field.max ?? Infinity }
    onTextChanged: if (acceptableInput) changed(text)
}
