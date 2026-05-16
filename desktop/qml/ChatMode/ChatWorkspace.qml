import QtQuick
import QtQuick.Layouts
import QtQuick.Controls
import ".."

Rectangle {
    color: Theme.bg

    ColumnLayout {
        anchors.fill: parent
        spacing: 0

        ConversationHeader {
            Layout.fillWidth: true
            Layout.preferredHeight: 40
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
            Layout.preferredHeight: 80
        }
    }
}
