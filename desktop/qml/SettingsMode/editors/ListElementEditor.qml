import QtQuick
import QtQuick.Layouts
import QtQuick.Controls
import Hermind

ColumnLayout {
    property int index

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
                text: "Item #" + (index + 1)
                font.pixelSize: 20
                font.weight: Font.Bold
                color: Theme.textPrimary
            }

            RowLayout {
                spacing: 12
                Button {
                    text: "↑"
                    enabled: index > 0
                    onClicked: appState.moveListInstance(appState.activeGroup, "jobs", index, "up")
                }
                Button {
                    text: "↓"
                    onClicked: appState.moveListInstance(appState.activeGroup, "jobs", index, "down")
                }
                Button {
                    text: "Delete"
                    onClicked: appState.deleteListInstance(appState.activeGroup, "jobs", index)
                }
            }
        }
    }

    ConfigSection {
        Layout.fillWidth: true
        section: appState.configSections.find(s => s.key === appState.activeGroup)
        value: appState.config[appState.activeGroup]?.jobs?.[index] || {}
        originalValue: appState.originalConfig[appState.activeGroup]?.jobs?.[index] || {}
        config: appState.config
        onFieldChanged: (name, v) => appState.setListField(appState.activeGroup, "jobs", index, name, v)
    }
}
