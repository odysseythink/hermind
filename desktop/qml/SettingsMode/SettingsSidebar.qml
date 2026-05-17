import QtQuick
import QtQuick.Layouts
import QtQuick.Controls
import Hermind

Rectangle {
    color: Theme.glassSurface

    // Right border
    Rectangle {
        anchors.right: parent.right
        anchors.top: parent.top
        anchors.bottom: parent.bottom
        width: 1
        color: Theme.glassBorder
    }

    // Group configuration: key → { label, order }
    property var groupMeta: ({
        models: { label: qsTr("模型"), order: 0 },
        memory: { label: qsTr("记忆"), order: 1 },
        skills: { label: qsTr("技能"), order: 2 },
        runtime: { label: qsTr("运行时"), order: 3 },
        advanced: { label: qsTr("高级"), order: 4 },
        gateway: { label: qsTr("IM 频道"), order: 5 },
        observability: { label: qsTr("可观测性"), order: 6 }
    })

    // Cached grouped sections
    property var groupedSections: buildGroups()
    property var groupKeys: sortedGroupKeys(groupedSections)

    onGroupedSectionsChanged: {
        groupKeys = sortedGroupKeys(groupedSections)
    }

    Connections {
        target: appState
        function onConfigSectionsChanged() {
            groupedSections = buildGroups()
        }
    }

    // Build grouped sections from flat configSections
    function buildGroups() {
        const sections = appState.configSections
        const groups = {}
        for (let i = 0; i < sections.length; i++) {
            const sec = sections[i]
            const gid = sec.group_id || "other"
            if (!groups[gid]) {
                groups[gid] = []
            }
            groups[gid].push(sec)
        }
        // Sort sections within each group by key
        for (const gid in groups) {
            groups[gid].sort((a, b) => (a.key || "").localeCompare(b.key || ""))
        }
        return groups
    }

    // Get sorted group keys
    function sortedGroupKeys(groups) {
        return Object.keys(groups).sort((a, b) => {
            const oa = groupMeta[a] ? groupMeta[a].order : 99
            const ob = groupMeta[b] ? groupMeta[b].order : 99
            return oa - ob
        })
    }

    // Check if a group is active (any section in it is selected)
    function isGroupActive(gid, sections) {
        for (let i = 0; i < sections.length; i++) {
            if (appState.activeGroup === sections[i].key) return true
        }
        return false
    }

    ListView {
        id: sidebarList
        anchors.fill: parent
        anchors.topMargin: 12
        anchors.bottomMargin: 12
        spacing: 0
        clip: true

        model: groupKeys

        delegate: Column {
            width: ListView.view.width

            property string groupId: modelData
            property var groupSections: groupedSections[modelData] || []
            property bool active: isGroupActive(groupId, groupSections)
            property bool expanded: groupSections.length > 1 && active

            // Group header
            Rectangle {
                width: parent.width
                height: groupHeaderText.implicitHeight + 16
                color: active ? Theme.glassSurfaceHover : "transparent"

                RowLayout {
                    anchors.fill: parent
                    anchors.leftMargin: 16
                    anchors.rightMargin: 12
                    spacing: 8

                    Text {
                        id: groupHeaderText
                        Layout.fillWidth: true
                        text: groupMeta[groupId] ? groupMeta[groupId].label : groupId
                        font.pixelSize: 14
                        font.weight: active ? Font.Bold : Font.Normal
                        color: Theme.textPrimary
                    }

                    Text {
                        text: expanded ? "\u25BC" : "\u25B6"
                        font.pixelSize: 10
                        color: Theme.textSecondary
                        visible: groupSections.length > 1
                    }
                }

                MouseArea {
                    anchors.fill: parent
                    cursorShape: Qt.PointingHandCursor
                    onClicked: {
                        if (groupSections.length === 1) {
                            appState.activeGroup = groupSections[0].key
                            appState.activeSubKey = ""
                        } else {
                            appState.activeGroup = groupSections[0].key
                            appState.activeSubKey = ""
                        }
                    }
                }
            }

            // Sub-items
            Column {
                width: parent.width
                visible: expanded

                Repeater {
                    model: groupSections
                    delegate: Rectangle {
                        width: parent.width
                        height: subItemText.implicitHeight + 12
                        color: appState.activeGroup === modelData.key ? Theme.glassCard : "transparent"
                        border.color: appState.activeGroup === modelData.key ? Theme.glassBorder : "transparent"
                        border.width: appState.activeGroup === modelData.key ? 1 : 0

                        Text {
                            id: subItemText
                            anchors.left: parent.left
                            anchors.leftMargin: 32
                            anchors.verticalCenter: parent.verticalCenter
                            text: modelData.label || modelData.key
                            font.pixelSize: 13
                            font.weight: appState.activeGroup === modelData.key ? Font.Bold : Font.Normal
                            color: appState.activeGroup === modelData.key ? Theme.textPrimary : Theme.textSecondary
                        }

                        MouseArea {
                            anchors.fill: parent
                            cursorShape: Qt.PointingHandCursor
                            onClicked: {
                                appState.activeGroup = modelData.key
                                appState.activeSubKey = ""
                            }
                        }
                    }
                }
            }
        }
    }
}
