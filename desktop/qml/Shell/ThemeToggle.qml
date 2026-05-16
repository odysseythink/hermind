import QtQuick
import QtQuick.Controls
import Hermind

Button {
    text: Theme.isDark ? "🌙" : "☀️"
    flat: true
    onClicked: Theme.isDark = !Theme.isDark
}
