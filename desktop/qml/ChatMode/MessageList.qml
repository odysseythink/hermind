import QtQuick
import QtQuick.Controls
import ".."

ListView {
    clip: true
    model: appState.messages
    spacing: 16
    anchors.margins: 16

    delegate: MessageBubble {
        width: ListView.view.width - 32
        isUser: modelData.role === "user"
        markdownText: modelData.content || ""
        toolCalls: modelData.toolCalls || []
    }

    ScrollBar.vertical: ScrollBar {}
}
