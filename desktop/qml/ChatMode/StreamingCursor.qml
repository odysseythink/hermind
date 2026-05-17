import QtQuick
import Hermind

Rectangle {
    width: 8
    height: 18
    color: Theme.accent
    radius: 1

    SequentialAnimation on opacity {
        loops: Animation.Infinite
        NumberAnimation { to: 0; duration: 500 }
        NumberAnimation { to: 1; duration: 500 }
    }
}
