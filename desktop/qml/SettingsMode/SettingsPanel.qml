import QtQuick
import QtQuick.Layouts
import QtQuick.Controls
import Hermind

Rectangle {
    color: "transparent"

    Loader {
        anchors.fill: parent
        anchors.margins: !appState.activeGroup ? 0 : 24
        sourceComponent: {
            if (!appState.activeGroup) return emptyStateGrid

            // Route to specific editor when a sub-item is selected
            if (appState.activeSubKey) {
                if (appState.activeGroup === "providers")
                    return providerEditor
                if (appState.activeGroup === "fallback_providers")
                    return fallbackEditor
                if (appState.activeGroup === "mcp" || appState.activeGroup === "gateway")
                    return keyedEditor
                if (appState.activeGroup === "cron")
                    return listEditor
            }

            // No subkey — route by section shape
            const section = appState.configSections.find(s => s.key === appState.activeGroup)
            if (!section) return emptyStateGrid
            if (section.shape === "scalar") return scalarEditor
            return configSectionComp
        }
    }

    Component {
        id: emptyStateGrid
        ScrollView {
            id: scrollView
            clip: true
            ScrollBar.vertical: ScrollBar {}
            contentWidth: availableWidth

            ColumnLayout {
                width: scrollView.availableWidth
                spacing: 0

                // Title
                Text {
                    Layout.alignment: Qt.AlignHCenter
                    Layout.topMargin: 48
                    Layout.bottomMargin: 32
                    text: qsTr("请选择要配置的分区")
                    font.pixelSize: 18
                    font.weight: Font.Bold
                    color: Theme.textPrimary
                }

                // Card grid
                GridLayout {
                    Layout.alignment: Qt.AlignHCenter
                    columns: 3
                    columnSpacing: 16
                    rowSpacing: 16

                    Repeater {
                        model: cardData()
                        delegate: GlassPanel {
                            Layout.preferredWidth: 240
                            Layout.preferredHeight: 100
                            baseColor: Theme.glassCard
                            highlightStrength: 0.1

                            ColumnLayout {
                                anchors.fill: parent
                                anchors.margins: 16
                                spacing: 8

                                Text {
                                    text: modelData.title
                                    font.pixelSize: 15
                                    font.weight: Font.Bold
                                    color: Theme.textPrimary
                                }

                                Text {
                                    Layout.fillWidth: true
                                    text: modelData.desc
                                    font.pixelSize: 12
                                    color: Theme.textSecondary
                                    wrapMode: Text.Wrap
                                    elide: Text.ElideRight
                                    maximumLineCount: 2
                                }

                                Item { Layout.fillHeight: true }

                                RowLayout {
                                    Layout.fillWidth: true
                                    spacing: 8

                                    Item { Layout.fillWidth: true }

                                    Rectangle {
                                        Layout.preferredWidth: statusText.implicitWidth + 16
                                        Layout.preferredHeight: 22
                                        radius: 4
                                        color: Theme.glassSurface
                                        border.color: Theme.glassBorder
                                        border.width: 1

                                        Text {
                                            id: statusText
                                            anchors.centerIn: parent
                                            text: qsTr("可用")
                                            font.pixelSize: 11
                                            color: Theme.textSecondary
                                        }
                                    }
                                }
                            }

                            MouseArea {
                                anchors.fill: parent
                                cursorShape: Qt.PointingHandCursor
                                hoverEnabled: true
                                onEntered: parent.baseColor = Theme.glassCardHover
                                onExited: parent.baseColor = Theme.glassCard
                                onClicked: {
                                    appState.activeGroup = modelData.firstSectionKey
                                    appState.activeSubKey = ""
                                }
                            }
                        }
                    }
                }

                Item { Layout.fillHeight: true }
            }
        }
    }

    property var _cardDataCache: buildCardData()

    Connections {
        target: appState
        function onConfigSectionsChanged() {
            _cardDataCache = buildCardData()
        }
    }

    function cardData() {
        return _cardDataCache
    }

    function buildCardData() {
        const groups = {}
        const sections = appState.configSections
        for (let i = 0; i < sections.length; i++) {
            const sec = sections[i]
            const gid = sec.group_id || "other"
            if (!groups[gid]) {
                groups[gid] = { sections: [] }
            }
            groups[gid].sections.push(sec)
        }

        const meta = {
            models: { title: qsTr("模型"), desc: qsTr("默认模型及 Provider 配置。"), order: 0 },
            memory: { title: qsTr("记忆"), desc: qsTr("长期记忆后端配置。"), order: 1 },
            skills: { title: qsTr("技能"), desc: qsTr("技能启停以及按平台覆盖。"), order: 2 },
            runtime: { title: qsTr("运行时"), desc: qsTr("Agent 提示词、辅助模型、终端、存储。"), order: 3 },
            advanced: { title: qsTr("高级"), desc: qsTr("MCP 服务器、浏览器自动化、定时任务。"), order: 4 },
            gateway: { title: qsTr("IM 频道"), desc: qsTr("多平台 IM 适配器配置。"), order: 5 },
            observability: { title: qsTr("可观测性"), desc: qsTr("日志级别、指标、追踪。"), order: 6 }
        }

        const cards = []
        for (const gid in groups) {
            const m = meta[gid]
            if (!m) continue
            const secs = groups[gid].sections
            secs.sort((a, b) => (a.key || "").localeCompare(b.key || ""))
            cards.push({
                title: m.title,
                desc: m.desc,
                firstSectionKey: secs[0].key,
                order: m.order
            })
        }
        cards.sort((a, b) => a.order - b.order)
        return cards
    }

    Component {
        id: configSectionComp
        ConfigSection {
            section: appState.configSections.find(s => s.key === appState.activeGroup)
            value: appState.config[appState.activeGroup] || {}
            originalValue: appState.originalConfig[appState.activeGroup] || {}
            config: appState.config
            onFieldChanged: (name, v) => appState.setConfigField(appState.activeGroup, name, v)
        }
    }

    Component {
        id: scalarEditor
        ConfigSection {
            section: appState.configSections.find(s => s.key === appState.activeGroup)
            value: { var o = {}; o[section.fields[0].name] = appState.config[appState.activeGroup]; return o; }
            originalValue: { var o = {}; o[section.fields[0].name] = appState.originalConfig[appState.activeGroup]; return o; }
            onFieldChanged: (name, v) => appState.setConfigScalar(appState.activeGroup, v)
        }
    }

    Component {
        id: providerEditor
        ProviderEditor {
            instanceKey: appState.activeSubKey
        }
    }

    Component {
        id: fallbackEditor
        FallbackProviderEditor {
            index: parseInt(appState.activeSubKey)
        }
    }

    Component {
        id: keyedEditor
        KeyedInstanceEditor {
            subKey: appState.activeSubKey
        }
    }

    Component {
        id: listEditor
        ListElementEditor {
            index: parseInt(appState.activeSubKey)
        }
    }
}
