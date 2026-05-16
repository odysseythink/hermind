import QtQuick

QtObject {
    property bool isDark: true

    property color bg: isDark ? "#0a0b0d" : "#ffffff"
    property color surface: isDark ? "#14161a" : "#f5f5f5"
    property color surfaceHover: isDark ? "#1a1c20" : "#eeeeee"
    property color border: isDark ? "#2a2e36" : "#d0d0d0"
    property color textPrimary: isDark ? "#e8e6e3" : "#1a1a1a"
    property color textSecondary: isDark ? "#8a8680" : "#666666"
    property color accent: "#FFB800"
    property color accentHover: "#FF8A00"
    property color success: "#6a9955"
    property color error: "#ce9178"
    property color codeBg: "#1e1e1e"
}
