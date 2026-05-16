import QtQuick
import QtQuick.Layouts
import QtQuick.Controls
import "../.."

ColumnLayout {
    property string instanceKey

    spacing: 16

    Text {
        text: instanceKey
        font.pixelSize: 20
        font.weight: Font.Bold
        color: Theme.textPrimary
    }

    RowLayout {
        spacing: 12
        Button {
            text: "Fetch Models"
            onClicked: appState.fetchProviderModels(instanceKey)
        }
        Button {
            text: "Test Connection"
            onClicked: appState.testProvider(instanceKey)
        }
        Button {
            text: "Delete"
            onClicked: appState.deleteKeyedInstance("providers", "", instanceKey)
        }
    }

    ConfigSection {
        section: appState.configSections.find(s => s.key === "providers")
        value: appState.config.providers?.[instanceKey] || {}
        originalValue: appState.originalConfig.providers?.[instanceKey] || {}
        config: appState.config
        onFieldChanged: (name, v) => appState.setKeyedField("providers", "", instanceKey, name, v)
    }

    Text {
        text: "Models"
        font.pixelSize: 14
        font.weight: Font.Bold
        color: Theme.textPrimary
        visible: appState.providerModels[instanceKey]?.length > 0
    }

    Flow {
        spacing: 8
        Repeater {
            model: appState.providerModels[instanceKey] || []
            delegate: Text {
                text: modelData
                color: Theme.textSecondary
                font.pixelSize: 12
            }
        }
    }
}
