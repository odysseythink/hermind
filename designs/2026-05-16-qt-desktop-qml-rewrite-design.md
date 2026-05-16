# Qt Desktop QML 重写设计文档

## 背景

现有 Qt Desktop 使用 Qt Widgets (C++) 实现，虽然核心聊天流已跑通，但：
- Settings 功能基本缺失（ConfigFormEngine 只支持基础字段类型）
- 与 Web UI 差距大（ProviderEditor、FallbackEditor、MCP/Cron/Gateway 编辑器均未实现）
- Qt Widgets 的声明式动态表单开发效率低

## 目标

用 QML (Qt Quick) 重写全部 Desktop UI，实现与 Web UI 100% 功能对等。

## 架构

### 分层

```
+-------------------------------------------------+
|  QML UI Layer (纯 QML + JS)                      |
|  - Shell: TopBar, Footer, Theme/Language toggle |
|  - ChatMode: MessageList, MessageBubble, ...    |
|  - SettingsMode: Sidebar, Panel, Editors, Fields |
+-------------------------------------------------+
|  C++ Glue Layer (精简到 5-7 个类)                |
|  - HermindClient (QML singleton)                |
|  - AppState (central state QObject)             |
|  - HermindProcess (backend process)             |
|  - TrayIcon / ShortcutManager                   |
+-------------------------------------------------+
|  Go Backend (不变)                               |
+-------------------------------------------------+
```

### C++ 胶水层设计

#### HermindClient

注册为 QML singleton (`qmldir` + `qmlRegisterSingletonType`)：

```cpp
class HermindClient : public QObject {
    Q_OBJECT
public:
    Q_INVOKABLE void get(const QString &path, QJSValue callback);
    Q_INVOKABLE void post(const QString &path, QJsonObject body, QJSValue callback);
    Q_INVOKABLE void put(const QString &path, QJsonObject body, QJSValue callback);
    Q_INVOKABLE void delete_(const QString &path, QJSValue callback);
    Q_INVOKABLE void upload(const QString &path, const QByteArray &data,
                            const QString &fileName, const QString &mimeType,
                            QJSValue callback);
    Q_INVOKABLE QNetworkReply* getStream(const QString &path);
};
```

回调使用 `QJSValue`（可调用 JavaScript 函数）。内部仍用 `QNetworkAccessManager`。

#### AppState

中央状态对象，注册为 QML 上下文属性：

```cpp
class AppState : public QObject {
    Q_OBJECT
    Q_PROPERTY(QJsonObject config READ config NOTIFY configChanged)
    Q_PROPERTY(QJsonObject originalConfig READ originalConfig)
    Q_PROPERTY(QString activeGroup READ activeGroup WRITE setActiveGroup NOTIFY activeGroupChanged)
    Q_PROPERTY(QString activeSubKey READ activeSubKey WRITE setActiveSubKey NOTIFY activeSubKeyChanged)
    Q_PROPERTY(int dirtyCount READ dirtyCount NOTIFY dirtyCountChanged)
    Q_PROPERTY(bool isStreaming READ isStreaming NOTIFY isStreamingChanged)
    Q_PROPERTY(QJsonArray messages READ messages NOTIFY messagesChanged)
    Q_PROPERTY(QStringList providerModels READ providerModels NOTIFY providerModelsChanged)

public slots:
    void boot();
    void setConfigField(const QString &section, const QString &field, const QJsonValue &value);
    void setConfigScalar(const QString &section, const QJsonValue &value);
    void setKeyedField(const QString &section, const QString &subkey,
                       const QString &instanceKey, const QString &field, const QJsonValue &value);
    void createKeyedInstance(const QString &section, const QString &subkey,
                             const QString &instanceKey, const QJsonObject &initial);
    void deleteKeyedInstance(const QString &section, const QString &subkey, const QString &instanceKey);
    void setListField(const QString &section, const QString &subkey,
                      int index, const QString &field, const QJsonValue &value);
    void createListInstance(const QString &section, const QString &subkey, const QJsonObject &initial);
    void deleteListInstance(const QString &section, const QString &subkey, int index);
    void moveListInstance(const QString &section, const QString &subkey, int index, const QString &direction);
    void saveConfig();
    void sendMessage(const QString &text);
    void cancelGeneration();
    void loadSession(const QString &sessionId);
    void startNewSession();
    void fetchProviderModels(const QString &instanceKey);
    void testProvider(const QString &instanceKey);

signals:
    void configChanged();
    void activeGroupChanged();
    void activeSubKeyChanged();
    void dirtyCountChanged();
    void isStreamingChanged();
    void messagesChanged();
    void providerModelsChanged();
    void streamChunk(const QString &text);
    void streamDone();
    void streamError(const QString &error);
    void toast(const QString &message);
};
```

内部持有 `HermindClient*` 指针，所有网络请求通过它发出。状态变更 emit signal，QML property binding 自动更新 UI。

### QML 文件结构

```
desktop/
  CMakeLists.txt
  src/
    main.cpp
    HermindClient.h/cpp
    HermindProcess.h/cpp
    AppState.h/cpp
    TrayIcon.h/cpp
    ShortcutManager.h/cpp
  qml/
    main.qml
    AppWindow.qml
    ChatMode/
      ChatWorkspace.qml
      MessageList.qml
      MessageBubble.qml
      PromptInput.qml
      AttachmentDropArea.qml
      ConversationHeader.qml
      EmptyState.qml
      ToolCallCard.qml
      StreamingCursor.qml
      StopButton.qml
      Toast.qml
    SettingsMode/
      SettingsSidebar.qml
      SettingsPanel.qml
      ConfigSection.qml
      editors/
        ProviderEditor.qml
        FallbackProviderEditor.qml
        KeyedInstanceEditor.qml
        ListElementEditor.qml
        DefaultModelEditor.qml
        AuxiliaryEditor.qml
        SkillsSection.qml
      fields/
        StringField.qml
        NumberField.qml
        FloatField.qml
        BoolField.qml
        EnumField.qml
        SecretField.qml
        TextAreaField.qml
        MultiSelectField.qml
    Shell/
      TopBar.qml
      Footer.qml
      ThemeToggle.qml
      LanguageToggle.qml
    components/
      ScrollToBottomButton.qml
      SourcesSidebar.qml
      SlashMenu.qml
      ChartVisualization.qml
```

### 关键 QML 组件设计

#### ConfigSection.qml（动态表单引擎）

```qml
Column {
    property var section
    property var value
    property var originalValue
    property var config
    signal fieldChanged(string name, var value)

    Repeater {
        model: section.fields
        delegate: Loader {
            active: isVisible(modelData, value)
            sourceComponent: {
                switch (modelData.kind) {
                    case "string": return stringField;
                    case "int":    return numberField;
                    case "float":  return floatField;
                    case "bool":   return boolField;
                    case "enum":   return enumField;
                    case "secret": return secretField;
                    case "text":   return textAreaField;
                    case "multiselect": return multiSelectField;
                }
            }
        }
    }
}
```

Web UI 的 `visible_when` 条件在 QML 中作为 `Loader.active` 的绑定表达式实现。

#### SettingsPanel.qml（路由）

```qml
Loader {
    sourceComponent: {
        if (!activeSubKey) return emptyState;
        if (activeSubKey.startsWith("fallback:")) return fallbackEditor;
        if (activeSubKey.startsWith("cron:"))     return listElementEditor;
        if (activeSubKey.startsWith("mcp:"))      return keyedInstanceEditor;
        if (activeSubKey.startsWith("gateway:"))  return keyedInstanceEditor;
        // Lookup section by activeSubKey
        const section = findSection(activeSubKey);
        if (section.shape === "scalar") return scalarEditor;
        if (section.shape === "keyed_map") return keyedInstanceEditor;
        if (section.shape === "list") return listEditor;
        // Default: provider instance
        return providerEditor;
    }
}
```

#### SettingsSidebar.qml

左侧导航，分组展开/折叠。Web UI 中 `SettingsSidebar.tsx` 的 props 全部映射为 QML property：

```qml
ListView {
    property string activeGroup
    property string activeSubKey
    property var configSections
    property var providerInstances
    property var dirtyProviderKeys
    // ...
    model: configSections
    delegate: GroupSection {}
}
```

#### MessageBubble.qml

```qml
Rectangle {
    property bool isUser
    property string markdownBuffer
    property string htmlContent
    property var toolCalls

    Column {
        Label { text: isUser ? "YOU" : "HERMIND"; font.family: "monospace" }
        TextEdit { text: htmlContent; readOnly: true; textFormat: TextEdit.RichText }
        Column {
            Repeater {
                model: toolCalls
                delegate: ToolCallCard { name: modelData.name; status: modelData.status }
            }
        }
        Row {
            Button { text: "Copy"; onClicked: clipboard.setText(markdownBuffer) }
            Button { text: "Regenerate"; onClicked: appState.regenerateMessage(...) }
            Button { text: "Delete"; onClicked: appState.deleteMessage(...) }
        }
    }
}
```

### 状态管理数据流

```
QML 组件 (SettingsPanel/ProviderEditor)
    ↓ 用户交互
AppState.setConfigField(section, field, value)
    ↓ 修改 m_config
emit configChanged()
    ↓ property binding
QML 组件自动刷新
    ↓ dirtyCount 变化
TopBar Save 按钮 enabled/disabled 自动切换
```

聊天流数据流：

```
PromptInput.onSendClicked
    ↓
AppState.sendMessage(text)
    ↓ POST /api/conversation/messages
    ↓ startStream()
SSEParser.feed(data)
    ↓ parse event
emit streamChunk(text)
    ↓
MessageBubble.appendText(text)
    ↓ renderTimer
MarkdownRenderer.render(markdown)
    ↓
MessageBubble.htmlContent = html
```

### 主题系统

QML 中不使用 QSS，改用 `Qt.labs.settings` + 动态颜色 property：

```qml
// Theme.qml (singleton)
pragma Singleton
import QtQuick
QtObject {
    property bool isDark: true
    property color bg: isDark ? "#0a0b0d" : "#ffffff"
    property color surface: isDark ? "#14161a" : "#f5f5f5"
    property color border: isDark ? "#2a2e36" : "#d0d0d0"
    property color textPrimary: isDark ? "#e8e6e3" : "#1a1a1a"
    property color textSecondary: isDark ? "#8a8680" : "#666666"
    property color accent: "#FFB800"
    property color accentHover: "#FF8A00"
    property color success: "#6a9955"
    property color error: "#ce9178"
}
```

所有 QML 组件通过 `Theme.bg`、`Theme.textPrimary` 等引用颜色。切换主题时只改 `Theme.isDark`，所有组件自动刷新。

### i18n

使用 Qt Linguist (`.ts` → `.qm`)，QML 中通过 `qsTr()` 标记字符串。

```qml
Label { text: qsTr("How can I help you today?") }
```

运行时切换：

```cpp
appState->setLanguage("zh_CN");  // 加载 QTranslator + 强制刷新所有 qsTr
```

### Markdown 渲染

保持 C++ `MarkdownRenderer` 类，暴露给 QML：

```cpp
class MarkdownRenderer : public QObject {
    Q_OBJECT
public:
    Q_INVOKABLE QString render(const QString &markdown);
};
```

QML 中：

```qml
TextEdit {
    text: markdownRenderer.render(messageBubble.markdownBuffer)
    textFormat: TextEdit.RichText
}
```

### CMake 构建

```cmake
find_package(Qt6 REQUIRED COMPONENTS Core Quick QuickControls2 Network LinguistTools)

qt_add_executable(hermind-desktop src/main.cpp ...)

qt_add_qml_module(hermind-desktop
    URI Hermind
    VERSION 1.0
    QML_FILES
        qml/main.qml
        qml/AppWindow.qml
        qml/ChatMode/ChatWorkspace.qml
        ...
    SOURCES
        src/HermindClient.cpp
        src/AppState.cpp
        ...
)

target_link_libraries(hermind-desktop PRIVATE
    Qt6::Core Qt6::Quick Qt6::QuickControls2 Qt6::Network
)
```

### 测试策略

1. **C++ 单元测试**：保留 test_httplib、test_sseparser
2. **QML 单元测试**：新增 `tests/qml/` 目录，用 `TestCase` 测试关键组件逻辑
3. **集成测试**：启动后端 + Desktop，验证端到端流程

### 迁移计划

1. **Phase 1: C++ 胶水层** — HermindClient (QML singleton)、AppState、CMake 重构
2. **Phase 2: Shell + Chat** — TopBar、MessageBubble、MessageList、PromptInput
3. **Phase 3: Settings 框架** — SettingsSidebar、SettingsPanel、ConfigSection、所有 Field 组件
4. **Phase 4: Settings 编辑器** — ProviderEditor、FallbackEditor、MCP/Gateway/Cron 编辑器
5. **Phase 5: 主题 + i18n + 打磨** — Theme singleton、qsTr、动画、边缘 case

### 风险

1. **QML 学习曲线**：需要熟悉 property binding、Loader、Repeater 等概念
2. **C++/QML 边界调试**：信号/槽跨边界问题可能难以定位
3. **性能**：大量消息列表用 `ListView` 替代 `Column + Repeater` 避免内存爆炸
4. **时间**：完整重写约 15-17 天，比补全 Widgets 慢 2 倍

---

*设计完成，待进入 writing-plans 阶段。*
