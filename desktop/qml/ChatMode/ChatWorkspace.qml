import QtQuick
import QtQuick.Layouts
import QtQuick.Controls
import Hermind

Rectangle {
    color: "transparent"

    ColumnLayout {
        anchors.fill: parent
        spacing: 0

        ConversationHeader {
            Layout.fillWidth: true
            Layout.preferredHeight: 32
        }

        StackLayout {
            Layout.fillWidth: true
            Layout.fillHeight: true
            currentIndex: appState.messages.length === 0 ? 0 : 1

            EmptyState {
                Layout.fillWidth: true
                Layout.fillHeight: true
            }

            MessageList {
                Layout.fillWidth: true
                Layout.fillHeight: true
            }
        }

        PromptInput {
            Layout.fillWidth: true
            Layout.preferredHeight: 64
        }
    }
}
