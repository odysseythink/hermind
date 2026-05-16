import QtQuick
import QtQuick.Controls
import Hermind

Switch {
    property var field
    property string value
    signal changed(string value)

    checked: value === "true"
    onCheckedChanged: changed(checked ? "true" : "false")
}
