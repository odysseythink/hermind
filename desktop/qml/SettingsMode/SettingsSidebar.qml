import QtQuick
import QtQuick.Layouts
import QtQuick.Controls
import Hermind

Rectangle {
    color: "#0f1012"

    ListView {
        anchors.fill: parent
        anchors.margins: 8
        model: appState.configSections
        spacing: 4

        delegate: Column {
            width: ListView.view.width

            Button {
                width: parent.width
                text: modelData.label || modelData.key
                flat: true
                highlighted: appState.activeGroup === modelData.key
                onClicked: {
                    appState.activeGroup = modelData.key
                    appState.activeSubKey = ""
                }
            }

            // Sub-items for keyed_map/list sections
            ListView {
                visible: modelData.shape === "keyed_map" || modelData.shape === "list"
                width: parent.width
                model: {
                    if (modelData.shape === "keyed_map") {
                        const subkey = modelData.subkey
                        const container = appState.config[modelData.key]?.[subkey] || {}
                        return Object.keys(container).sort()
                    }
                    if (modelData.shape === "list") {
                        const subkey = modelData.subkey || modelData.key
                        const arr = appState.config[modelData.key]?.[subkey] || []
                        return arr.map((_, i) => String(i))
                    }
                    return []
                }
                delegate: Button {
                    width: parent.width
                    text: modelData
                    flat: true
                    leftPadding: 24
                    highlighted: appState.activeSubKey === modelData
                    onClicked: appState.activeSubKey = modelData
                }
            }
        }
    }
}
