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
                text: "Default Model"
                font.pixelSize: 20
                font.weight: Font.Bold
                color: Theme.textPrimary
            }
        }
    }

    ConfigSection {
        Layout.fillWidth: true
        section: appState.configSections.find(s => s.key === "model")
        value: { var o = {}; o[section.fields[0].name] = appState.config.model || ""; return o; }
        originalValue: { var o = {}; o[section.fields[0].name] = appState.originalConfig.model || ""; return o; }
        onFieldChanged: (name, v) => appState.setConfigScalar("model", v)
    }
}
