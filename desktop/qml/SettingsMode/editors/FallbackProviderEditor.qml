import QtQuick
import QtQuick.Layouts
import QtQuick.Controls
import Hermind

ColumnLayout {
    property int index

    spacing: 16

    Text {
        text: "Fallback Provider #" + (index + 1)
        font.pixelSize: 20
        font.weight: Font.Bold
        color: Theme.textPrimary
    }

    RowLayout {
        spacing: 12
        Button {
            text: "↑"
            enabled: index > 0
            onClicked: appState.moveListInstance("fallback_providers", "", index, "up")
        }
        Button {
            text: "↓"
            onClicked: appState.moveListInstance("fallback_providers", "", index, "down")
        }
        Button {
            text: "Fetch Models"
            onClicked: appState.fetchFallbackModels(index)
        }
        Button {
            text: "Delete"
            onClicked: appState.deleteListInstance("fallback_providers", "", index)
        }
    }

    ConfigSection {
        section: appState.configSections.find(s => s.key === "fallback_providers")
        value: appState.config.fallback_providers?.[index] || {}
        originalValue: appState.originalConfig.fallback_providers?.[index] || {}
        config: appState.config
        onFieldChanged: (name, v) => appState.setListField("fallback_providers", "", index, name, v)
    }
}
