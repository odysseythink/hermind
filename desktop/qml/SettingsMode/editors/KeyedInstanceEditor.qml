import QtQuick
import QtQuick.Layouts
import QtQuick.Controls
import Hermind

ColumnLayout {
    property string subKey

    spacing: 14

    GlassPanel {
        Layout.fillWidth: true
        baseColor: Theme.glassSurface
        highlightStrength: 0.08

        ColumnLayout {
            anchors.fill: parent
            anchors.margins: 16
            spacing: 12

            Text {
                text: subKey
                font.pixelSize: 20
                font.weight: Font.Bold
                color: Theme.textPrimary
            }

            RowLayout {
                spacing: 12
                Button {
                    text: "Delete"
                    onClicked: {
                        const section = appState.activeGroup
                        const sub = appState.configSections.find(s => s.key === section)?.subkey || ""
                        appState.deleteKeyedInstance(section, sub, subKey)
                    }
                }
            }
        }
    }

    ConfigSection {
        Layout.fillWidth: true
        section: appState.configSections.find(s => s.key === appState.activeGroup)
        value: {
            const section = appState.activeGroup
            const sub = appState.configSections.find(s => s.key === section)?.subkey || ""
            return appState.config[section]?.[sub]?.[subKey] || {}
        }
        originalValue: {
            const section = appState.activeGroup
            const sub = appState.configSections.find(s => s.key === section)?.subkey || ""
            return appState.originalConfig[section]?.[sub]?.[subKey] || {}
        }
        config: appState.config
        onFieldChanged: (name, v) => {
            const section = appState.activeGroup
            const sub = appState.configSections.find(s => s.key === section)?.subkey || ""
            appState.setKeyedField(section, sub, subKey, name, v)
        }
    }
}
