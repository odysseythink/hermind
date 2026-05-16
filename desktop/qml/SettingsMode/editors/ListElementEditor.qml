import QtQuick
import QtQuick.Layouts
import QtQuick.Controls
import "../.."

ColumnLayout {
    property string subKey

    spacing: 16

    Text {
        text: subKey
        font.pixelSize: 20
        font.weight: Font.Bold
        color: Theme.textPrimary
    }

    RowLayout {
        spacing: 12
        Button {
            text: "↑"
            onClicked: {
                const section = appState.activeGroup
                const idx = parseInt(subKey.slice(subKey.indexOf(":") + 1))
                appState.moveListInstance(section, "jobs", idx, "up")
            }
        }
        Button {
            text: "↓"
            onClicked: {
                const section = appState.activeGroup
                const idx = parseInt(subKey.slice(subKey.indexOf(":") + 1))
                appState.moveListInstance(section, "jobs", idx, "down")
            }
        }
        Button {
            text: "Delete"
            onClicked: {
                const section = appState.activeGroup
                const idx = parseInt(subKey.slice(subKey.indexOf(":") + 1))
                appState.deleteListInstance(section, "jobs", idx)
            }
        }
    }

    ConfigSection {
        section: appState.configSections.find(s => s.key === appState.activeGroup)
        value: {
            const section = appState.activeGroup
            const idx = parseInt(subKey.slice(subKey.indexOf(":") + 1))
            return appState.config[section]?.jobs?.[idx] || {}
        }
        originalValue: {
            const section = appState.activeGroup
            const idx = parseInt(subKey.slice(subKey.indexOf(":") + 1))
            return appState.originalConfig[section]?.jobs?.[idx] || {}
        }
        config: appState.config
        onFieldChanged: (name, v) => {
            const section = appState.activeGroup
            const idx = parseInt(subKey.slice(subKey.indexOf(":") + 1))
            appState.setListField(section, "jobs", idx, name, v)
        }
    }
}
