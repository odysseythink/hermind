import QtQuick
import QtQuick.Layouts
import QtQuick.Controls
import Hermind

Rectangle {
    property bool isUser: false
    property string markdownText: ""
    property var toolCalls: []
    property bool isStreaming: false

    color: isUser ? Qt.rgba(255, 184, 0, 0.25) : Theme.glassCard
    border.color: isUser ? Qt.rgba(255, 184, 0, 0.4) : Theme.glassBorder
    border.width: 1
    radius: 12

    // Top highlight for glass effect
    Rectangle {
        visible: !isUser
        anchors.top: parent.top
        anchors.left: parent.left
        anchors.right: parent.right
        height: 7
        radius: parent.radius
        color: Theme.glassSurfaceHover
        opacity: 0.15
        Rectangle {
            anchors.bottom: parent.bottom
            anchors.left: parent.left
            anchors.right: parent.right
            height: parent.height * 0.5
            color: parent.parent.color
            opacity: 1.0
        }
    }

    // User message top highlight (accent tinted)
    Rectangle {
        visible: isUser
        anchors.top: parent.top
        anchors.left: parent.left
        anchors.right: parent.right
        height: 7
        radius: parent.radius
        color: "#FFE066"
        opacity: 0.3
        Rectangle {
            anchors.bottom: parent.bottom
            anchors.left: parent.left
            anchors.right: parent.right
            height: parent.height * 0.5
            color: parent.parent.color
            opacity: 1.0
        }
    }

    ColumnLayout {
        anchors.fill: parent
        anchors.margins: 16
        spacing: 12

        // Header
        RowLayout {
            Layout.fillWidth: true
            spacing: 8

            Rectangle {
                Layout.preferredWidth: roleLabel.implicitWidth + 12
                Layout.preferredHeight: 20
                radius: 4
                color: isUser ? "#1a1817" : Theme.glassSurface

                Text {
                    id: roleLabel
                    anchors.centerIn: parent
                    text: isUser ? qsTr("YOU") : qsTr("HERMIND")
                    font.family: "monospace"
                    font.pixelSize: 10
                    font.weight: Font.Bold
                    color: isUser ? Theme.accent : Theme.textSecondary
                }
            }

            Item { Layout.fillWidth: true }
        }

        // Content
        RowLayout {
            Layout.fillWidth: true
            spacing: 4

            TextEdit {
                id: contentEdit
                text: markdownText
                readOnly: true
                selectByMouse: true
                textFormat: TextEdit.MarkdownText
                wrapMode: TextEdit.Wrap
                color: isUser ? "#1a1817" : Theme.textPrimary
                font.pixelSize: 14
                Layout.fillWidth: true
            }

            StreamingCursor {
                visible: isStreaming
                Layout.alignment: Qt.AlignBottom
            }
        }

        // Tool calls
        ColumnLayout {
            visible: toolCalls.length > 0
            spacing: 8
            Repeater {
                model: toolCalls
                delegate: ToolCallCard {
                    name: modelData.name || ""
                    status: modelData.status || ""
                }
            }
        }

        // Action buttons (AI messages only)
        RowLayout {
            visible: !isUser
            spacing: 4
            Layout.alignment: Qt.AlignRight

            Repeater {
                model: [
                    { text: qsTr("Copy"), action: () => {
                        const text = contentEdit.selectedText.length > 0 ? contentEdit.selectedText : contentEdit.text
                        Qt.application.clipboard.setText(text)
                        appState.toast("Copied to clipboard")
                    }},
                    { text: qsTr("Regenerate"), action: () => {} },
                    { text: qsTr("Delete"), action: () => {} }
                ]
                delegate: Rectangle {
                    Layout.preferredWidth: actionText.implicitWidth + 16
                    Layout.preferredHeight: 28
                    radius: 6
                    color: actionMouse.containsMouse ? Theme.glassCardHover : "transparent"

                    Text {
                        id: actionText
                        anchors.centerIn: parent
                        text: modelData.text
                        font.pixelSize: 12
                        color: Theme.textSecondary
                    }

                    MouseArea {
                        id: actionMouse
                        anchors.fill: parent
                        cursorShape: Qt.PointingHandCursor
                        hoverEnabled: true
                        onClicked: modelData.action()
                    }
                }
            }
        }
    }
}
