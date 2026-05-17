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
                text: "Skills"
                font.pixelSize: 20
                font.weight: Font.Bold
                color: Theme.textPrimary
            }
        }
    }

    ConfigSection {
        Layout.fillWidth: true
        section: appState.configSections.find(s => s.key === "skills")
        value: appState.config.skills || {}
        originalValue: appState.originalConfig.skills || {}
        config: appState.config
        onFieldChanged: (name, v) => appState.setConfigField("skills", name, v)
    }
}
