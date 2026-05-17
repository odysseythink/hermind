import QtQuick
import QtQuick.Layouts
import QtQuick.Controls
import Hermind

Rectangle {
    color: "transparent"

    ColumnLayout {
        anchors.centerIn: parent
        spacing: 32

        Text {
            text: qsTr("How can I help you today?")
            font.pixelSize: 28
            font.weight: Font.Bold
            color: Theme.textPrimary
            Layout.alignment: Qt.AlignHCenter
        }

        ColumnLayout {
            spacing: 12
            Layout.alignment: Qt.AlignHCenter

            // First row: 3 buttons
            RowLayout {
                spacing: 12
                Layout.alignment: Qt.AlignHCenter

                Repeater {
                    model: [
                        qsTr("What can you help me with?"),
                        qsTr("Explain this codebase"),
                        qsTr("Write a test for the current function")
                    ]
                    delegate: GlassPanel {
                        Layout.preferredWidth: promptBtnText.implicitWidth + 24
                        Layout.preferredHeight: 36
                        baseColor: Theme.glassCard
                        highlightStrength: 0.1
                        radius: 18

                        Text {
                            id: promptBtnText
                            anchors.centerIn: parent
                            text: modelData
                            font.pixelSize: 13
                            color: Theme.textSecondary
                        }

                        MouseArea {
                            anchors.fill: parent
                            cursorShape: Qt.PointingHandCursor
                            hoverEnabled: true
                            onEntered: parent.baseColor = Theme.glassCardHover
                            onExited: parent.baseColor = Theme.glassCard
                            onClicked: appState.sendMessage(modelData)
                        }
                    }
                }
            }

            // Second row: 1 button centered
            GlassPanel {
                Layout.preferredWidth: promptBtnText2.implicitWidth + 24
                Layout.preferredHeight: 36
                radius: 18
                baseColor: Theme.glassCard
                highlightStrength: 0.1
                Layout.alignment: Qt.AlignHCenter

                Text {
                    id: promptBtnText2
                    anchors.centerIn: parent
                    text: qsTr("Summarize the recent changes")
                    font.pixelSize: 13
                    color: Theme.textSecondary
                }

                MouseArea {
                    anchors.fill: parent
                    cursorShape: Qt.PointingHandCursor
                    hoverEnabled: true
                    onEntered: parent.baseColor = Theme.glassCardHover
                    onExited: parent.baseColor = Theme.glassCard
                    onClicked: appState.sendMessage(qsTr("Summarize the recent changes"))
                }
            }
        }
    }
}
