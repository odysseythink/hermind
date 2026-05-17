import QtQuick

Rectangle {
    id: root

    property color baseColor: Theme.glassCard
    property color highlightColor: Theme.glassSurfaceHover
    property color borderColor: Theme.glassBorder
    property real highlightStrength: 0.12
    property real borderOpacity: 1.0

    color: baseColor
    radius: 8

    Rectangle {
        anchors.top: parent.top
        anchors.left: parent.left
        anchors.right: parent.right
        height: Math.max(1, parent.radius * 0.6)
        radius: parent.radius
        color: highlightColor
        opacity: highlightStrength

        Rectangle {
            anchors.bottom: parent.bottom
            anchors.left: parent.left
            anchors.right: parent.right
            height: parent.height * 0.5
            color: root.color
            opacity: 1.0
        }
    }

    Rectangle {
        anchors.fill: parent
        color: "transparent"
        radius: parent.radius
        border.color: borderColor
        border.width: 1
        opacity: borderOpacity
    }
}
