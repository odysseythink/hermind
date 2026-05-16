import QtQuick
import QtQuick.Controls
import Hermind

ComboBox {
    model: ListModel {
        ListElement { text: "EN"; code: "en" }
        ListElement { text: "中"; code: "zh_CN" }
    }
    textRole: "text"
    currentIndex: 0
    onActivated: {
        const code = model.get(currentIndex).code
        appState.setLanguage(code)
    }
}
