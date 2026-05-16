import QtQuick
import QtQuick.Layouts
import QtQuick.Controls
import "../.."

ColumnLayout {
    spacing: 16

    Text {
        text: "Auxiliary Model"
        font.pixelSize: 20
        font.weight: Font.Bold
        color: Theme.textPrimary
    }

    RowLayout {
        spacing: 12
        Button {
            text: "Fetch Models"
            onClicked: appState.fetchAuxiliaryModels()
        }
        Button {
            text: "Test Connection"
            onClicked: appState.testAuxiliary()
        }
    }

    ConfigSection {
        section: appState.configSections.find(s => s.key === "auxiliary")
        value: appState.config.auxiliary || {}
        originalValue: appState.originalConfig.auxiliary || {}
        config: appState.config
        onFieldChanged: (name, v) => appState.setConfigField("auxiliary", name, v)
    }
}
