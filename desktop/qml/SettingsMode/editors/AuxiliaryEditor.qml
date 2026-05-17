import QtQuick
import QtQuick.Layouts
import QtQuick.Controls
import Hermind

ColumnLayout {
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
        }
    }

    ConfigSection {
        Layout.fillWidth: true
        section: appState.configSections.find(s => s.key === "auxiliary")
        value: appState.config.auxiliary || {}
        originalValue: appState.originalConfig.auxiliary || {}
        config: appState.config
        onFieldChanged: (name, v) => appState.setConfigField("auxiliary", name, v)
    }
}
