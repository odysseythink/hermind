import QtQuick
import QtQuick.Controls
import ".."

ComboBox {
    model: ListModel {
        ListElement { text: "EN"; code: "en" }
        ListElement { text: "中"; code: "zh_CN" }
    }
    textRole: "text"
    currentIndex: 0
    onActivated: {
        // TODO: switch Qt translator
    }
}
