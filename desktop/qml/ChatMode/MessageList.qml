import QtQuick
import QtQuick.Controls
import Hermind

ListView {
    id: messageListView
    clip: true
    model: appState.messages
    spacing: 16
    anchors.margins: 16

    delegate: Item {
        width: ListView.view.width - 32
        height: messageBubble.height

        MessageBubble {
            id: messageBubble
            width: modelData.role === "user" ? Math.min(parent.width * 0.8, implicitWidth) : parent.width
            anchors.right: modelData.role === "user" ? parent.right : undefined
            anchors.left: modelData.role === "user" ? undefined : parent.left
            isUser: modelData.role === "user"
            markdownText: modelData.content || ""
            toolCalls: modelData.toolCalls || []
            isStreaming: !isUser && appState.isStreaming && index === messageListView.count - 1
        }
    }

    ScrollBar.vertical: ScrollBar {}

    ScrollToBottomButton {
        anchors.right: parent.right
        anchors.bottom: parent.bottom
        anchors.margins: 16
        visible: messageListView.contentHeight > messageListView.height &&
                 messageListView.contentY < messageListView.contentHeight - messageListView.height - 50
        onClicked: messageListView.positionViewAtEnd()
    }
}
