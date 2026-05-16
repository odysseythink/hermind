import QtQuick
import QtQuick.Layouts
import QtQuick.Controls
import ".."

Rectangle {
    color: "transparent"

    ColumnLayout {
        anchors.centerIn: parent
        spacing: 20

        Text {
            text: "How can I help you today?"
            font.pixelSize: 28
            font.weight: Font.Bold
            color: Theme.textPrimary
            Layout.alignment: Qt.AlignHCenter
        }

        RowLayout {
            spacing: 12
            Layout.alignment: Qt.AlignHCenter

            Repeater {
                model: ["Explain a concept", "Write some code", "Debug an error"]
                delegate: Button {
                    text: modelData
                    onClicked: appState.sendMessage(text)
                }
            }
        }
    }
}
