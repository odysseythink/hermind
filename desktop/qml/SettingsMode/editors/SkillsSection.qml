import QtQuick
import QtQuick.Layouts
import QtQuick.Controls
import Hermind

ColumnLayout {
    spacing: 16

    Text {
        text: "Skills"
        font.pixelSize: 20
        font.weight: Font.Bold
        color: Theme.textPrimary
    }

    ConfigSection {
        section: appState.configSections.find(s => s.key === "skills")
        value: appState.config.skills || {}
        originalValue: appState.originalConfig.skills || {}
        config: appState.config
        onFieldChanged: (name, v) => appState.setConfigField("skills", name, v)
    }
}
