import QtQuick
import QtQuick.Layouts
import QtQuick.Controls
import ".."

Rectangle {
    color: Theme.bg

    Loader {
        anchors.fill: parent
        anchors.margins: 24
        sourceComponent: {
            if (!appState.activeGroup) return emptyState

            // Route to specific editor when a sub-item is selected
            if (appState.activeSubKey) {
                if (appState.activeGroup === "providers")
                    return providerEditor
                if (appState.activeGroup === "fallback_providers")
                    return fallbackEditor
                if (appState.activeGroup === "mcp" || appState.activeGroup === "gateway")
                    return keyedEditor
                if (appState.activeGroup === "cron")
                    return listEditor
            }

            // No subkey — route by section shape
            const section = appState.configSections.find(s => s.key === appState.activeGroup)
            if (!section) return emptyState
            if (section.shape === "scalar") return scalarEditor
            return configSectionComp
        }
    }

    Component {
        id: emptyState
        Text {
            text: "Select an item from the sidebar"
            color: Theme.textSecondary
            font.pixelSize: 14
            anchors.centerIn: parent
        }
    }

    Component {
        id: configSectionComp
        ConfigSection {
            section: appState.configSections.find(s => s.key === appState.activeGroup)
            value: appState.config[appState.activeGroup] || {}
            originalValue: appState.originalConfig[appState.activeGroup] || {}
            config: appState.config
            onFieldChanged: (name, v) => appState.setConfigField(appState.activeGroup, name, v)
        }
    }

    Component {
        id: scalarEditor
        ConfigSection {
            section: appState.configSections.find(s => s.key === appState.activeGroup)
            value: { var o = {}; o[section.fields[0].name] = appState.config[appState.activeGroup]; return o; }
            originalValue: { var o = {}; o[section.fields[0].name] = appState.originalConfig[appState.activeGroup]; return o; }
            onFieldChanged: (name, v) => appState.setConfigScalar(appState.activeGroup, v)
        }
    }

    Component {
        id: providerEditor
        ProviderEditor {
            instanceKey: appState.activeSubKey
        }
    }

    Component {
        id: fallbackEditor
        FallbackProviderEditor {
            index: parseInt(appState.activeSubKey)
        }
    }

    Component {
        id: keyedEditor
        KeyedInstanceEditor {
            subKey: appState.activeSubKey
        }
    }

    Component {
        id: listEditor
        ListElementEditor {
            index: parseInt(appState.activeSubKey)
        }
    }
}
