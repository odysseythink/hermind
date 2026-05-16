import QtQuick
import QtQuick.Layouts
import QtQuick.Controls
import Hermind

ColumnLayout {
    spacing: 16

    Text {
        text: "Default Model"
        font.pixelSize: 20
        font.weight: Font.Bold
        color: Theme.textPrimary
    }

    ConfigSection {
        section: appState.configSections.find(s => s.key === "model")
        value: { var o = {}; o[section.fields[0].name] = appState.config.model || ""; return o; }
        originalValue: { var o = {}; o[section.fields[0].name] = appState.originalConfig.model || ""; return o; }
        onFieldChanged: (name, v) => appState.setConfigScalar("model", v)
    }
}
