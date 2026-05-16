import QtQuick
import QtQuick.Controls
import ".."

Button {
    text: Theme.isDark ? "🌙" : "☀️"
    flat: true
    onClicked: Theme.isDark = !Theme.isDark
}
