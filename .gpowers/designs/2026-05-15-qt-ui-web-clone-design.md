# Qt Desktop UI 完全仿照 Web UI 设计文档

**日期**: 2026-05-15  
**主题**: Qt 桌面端视觉与布局对齐 Web 暗色主题  
**策略**: 方案 B — 全量重写（一次性对齐，无中间态）  
**状态**: 待确认  

---

## 1. 目标

将现有 Qt 桌面端 UI 的视觉风格、颜色系统、布局结构、组件样式完全对齐 Web UI 的默认暗色主题。

**不对齐的内容**（明确排除）：
- 亮/自动主题切换（仅暗色）
- CSS transition 动画（Qt Widgets 不支持，用静态样式替代）
- 自定义字体文件（使用系统字体 fallback）
- Web 特有的 CSS 高级特性（如 `box-shadow`、`backdrop-filter`）

---

## 2. 颜色系统

重写 `desktop/resources/styles.qss`，从 VS Code 风格迁移到 Web 暗色 token。

### 2.1 Token 映射表

| Token | Web CSS | Qt QSS | 用途 |
|-------|---------|--------|------|
| Background | `#0a0b0d` | `QMainWindow`, `QWidget` background | 页面背景 |
| Surface | `#14161a` | 侧边栏、TopBar、Composer、Footer | 表面层背景 |
| Surface-2 | `#1d2027` | 输入框默认背景、hover 背景 |  elevated 表面 |
| Border | `#2a2e36` | 所有边框、分隔线 | 边框/分割线 |
| Text | `#e8e6e3` | 主文字色 | 标题、正文 |
| Muted | `#8a8680` | 次要文字、placeholder、状态文字 | 辅助信息 |
| Accent | `#FFB800` | 按钮、激活边框、焦点环 | 品牌强调色 |
| Accent-2 | `#FF8A00` | hover 状态 | 悬停强调色 |
| Accent-bg | `rgba(255,184,0,0.08)` | 激活项背景 | 选中态背景 |
| Focus | `rgba(255,184,0,0.35)` | focus 边框+阴影替代 | 焦点指示 |
| Success | `#7ee787` | 状态点（就绪） | 成功状态 |
| Error | `#ff6b6b` | 错误提示 | 错误状态 |

### 2.2 QSS 结构

```qss
/* ===== 基础 ===== */
QMainWindow, QWidget { background: #0a0b0d; color: #e8e6e3; }

/* ===== 输入控件 ===== */
QTextEdit, QLineEdit, QComboBox {
    background: #0a0b0d;
    color: #e8e6e3;
    border: 1px solid #2a2e36;
    border-radius: 4px;
    padding: 8px 12px;
    font-size: 13px;
}
QTextEdit:focus, QLineEdit:focus, QComboBox:focus {
    border-color: #FFB800;
}

/* ===== 按钮 ===== */
QPushButton {
    background: #FFB800;
    color: #0a0b0d;
    border: 1px solid #FFB800;
    border-radius: 4px;
    padding: 6px 16px;
    font-weight: 600;
    font-size: 12px;
}
QPushButton:hover { background: #FF8A00; border-color: #FF8A00; }
QPushButton:disabled {
    background: transparent; color: #8a8680; border-color: #2a2e36; opacity: 0.35;
}

/* ===== 列表 ===== */
QListWidget {
    background: #14161a;
    color: #e8e6e3;
    border: none;
    outline: none;
}
QListWidget::item {
    padding: 8px 16px;
    border-left: 2px solid transparent;
    color: #8a8680;
}
QListWidget::item:hover { background: rgba(255,255,255,0.04); }
QListWidget::item:selected {
    background: rgba(255,184,0,0.08);
    color: #e8e6e3;
    border-left: 2px solid #FFB800;
}

/* ===== 滚动条 ===== */
QScrollBar:vertical {
    width: 10px; background: transparent; border: none;
}
QScrollBar::handle:vertical {
    background: #2a2e36; border-radius: 2px;
    border: 2px solid #0a0b0d;
    min-height: 20px;
}
QScrollBar::handle:vertical:hover { background: #8a8680; }
QScrollBar::add-line:vertical, QScrollBar::sub-line:vertical { height: 0; }

/* ===== 菜单 ===== */
QMenuBar { background: #14161a; color: #e8e6e3; border-bottom: 1px solid #2a2e36; }
QMenu { background: #14161a; color: #e8e6e3; border: 1px solid #2a2e36; }
QMenu::item:selected { background: rgba(255,255,255,0.04); }

/* ===== 对话框 ===== */
QDialog { background: #0a0b0d; }
QLabel { color: #e8e6e3; }
```

---

## 3. 布局重构

### 3.1 当前布局 → 目标布局

**当前**（原生 MenuBar + 简单 Splitter）：
```
┌─────────────────────────────────────────┐
│ MenuBar (File > Settings)               │
├──────────┬──────────────────────────────┤
│ Session  │  ChatWidget                  │
│ List     │  (ScrollArea + Input)        │
│ (250px)  │                              │
└──────────┴──────────────────────────────┘
```

**目标**（自定义 TopBar + StatusFooter）：
```
┌─────────────────────────────────────────┐
│ TopBar (48px)                           │
│ [◈ HERMIND]    [Chat|Set] [● Ready] [Save]│
├─────────────────────────────────────────┤
│ Sidebar (240px) │ ChatWorkspace         │
│  [+ New Chat]   │ ┌─────────────────┐   │
│  [📄 Active]    │ │ ConversationHeader│   │
│  [📄 Item]      │ ├─────────────────┤   │
│                 │ │ MessageList     │   │
│                 │ │  • Bubble       │   │
│                 │ │  • Bubble       │   │
│                 │ ├─────────────────┤   │
│                 │ │ Composer        │   │
│                 │ └─────────────────┘   │
├─────────────────────────────────────────┤
│ StatusFooter (32px)                     │
│ ◈ hermind v0.x · Qt6 Desktop · Ready   │
└─────────────────────────────────────────┘
```

### 3.2 AppWindow 重构

`AppWindow` 从 `QMainWindow`（含 MenuBar）改为纯 `QWidget` 容器：

```cpp
class AppWindow : public QWidget {
    Q_OBJECT
public:
    explicit AppWindow(QWidget *parent = nullptr);
    void setClient(HermindClient *client);

protected:
    void closeEvent(QCloseEvent *event) override;

private:
    void setupUI();
    void setupTopBar();
    void setupSidebar();
    void setupChatArea();
    void setupFooter();

    TopBar *m_topBar;
    SessionListWidget *m_sessionList;
    ChatWidget *m_chatWidget;
    StatusFooter *m_footer;
    QSplitter *m_splitter;
    SettingsDialog *m_settingsDialog;
};
```

---

## 4. 组件详细设计

### 4.1 TopBar（新增）

替换原生 MenuBar，自定义 `QWidget` + `QHBoxLayout`：

| 元素 | 类型 | 样式 |
|------|------|------|
| 品牌标识 | `QLabel` | mono font, 14px, uppercase, letter-spacing 0.05em, `◈ HERMIND` |
| Spacer | `QSpacerItem` | `Expanding` |
| 模式切换 | `QButtonGroup` + `QPushButton` | 无间隙按钮组，active 项 amber 背景 |
| 状态指示 | `QLabel` + 自定义 dot widget | mono 12px, uppercase, 8px 圆点 |
| Save 按钮 | `QPushButton` | amber 背景, mono 12px, uppercase |

高度固定 48px，背景 `#14161a`，底部边框 `#2a2e36`。

### 4.2 StatusFooter（新增）

底部状态栏，高度 32px：

```cpp
class StatusFooter : public QWidget {
    Q_OBJECT
public:
    explicit StatusFooter(QWidget *parent = nullptr);
    void setVersion(const QString &version);
    void setModel(const QString &model);
    void setStatus(const QString &status);
private:
    QLabel *m_label;
};
```

样式：背景 `#14161a`，顶部边框 `#2a2e36`，mono 11px，`#8a8680` 文字。

### 4.3 EmptyStateWidget（新增）

空会话时的占位界面：

```cpp
class EmptyStateWidget : public QWidget {
    Q_OBJECT
public:
    explicit EmptyStateWidget(QWidget *parent = nullptr);
signals:
    void suggestionClicked(const QString &text);
};
```

布局：垂直居中，`QVBoxLayout`：
- 标题 `QLabel`：28px, weight 600, `#e8e6e3`, "How can I help you today?"
- 建议按钮网格：`QHBoxLayout` + `QPushButton`（surface 背景，8px 圆角，hover 边框变 amber）

### 4.4 MessageBubble（改造）

**去掉 avatar**，改为 role tag 模式：

```cpp
class MessageBubble : public QWidget {
    Q_OBJECT
public:
    explicit MessageBubble(bool isUser, QWidget *parent = nullptr);
    void setHtml(const QString &html);
    void setPlainText(const QString &text);
private:
    void setupUI();
    QLabel *m_roleTag;      // "YOU" / "HERMIND"
    QTextEdit *m_content;   // read-only, frameless
    bool m_isUser;
};
```

样式：
- **User**: role tag 颜色 `#FFB800`；气泡透明背景 + `1px solid #FFB800` 边框，圆角 4px
- **Assistant**: role tag 颜色 `#8a8680`；气泡 `#14161a` 背景 + `1px solid #2a2e36` 边框，圆角 4px
- 最大宽度 85%，用户右对齐，助手左对齐

### 4.5 PromptInput（改造）

```cpp
class PromptInput : public QWidget {
    Q_OBJECT
public:
    explicit PromptInput(QWidget *parent = nullptr);
    QString text() const;
    void clear();
    void setEnabled(bool enabled);
signals:
    void sendClicked();
    void attachClicked();
private:
    QTextEdit *m_textEdit;
    QPushButton *m_sendBtn;
    QPushButton *m_attachBtn;
};
```

样式变化：
- Textarea：背景 `#0a0b0d`，边框 `#2a2e36`，圆角 4px，focus 时边框 `#FFB800`
- Send 按钮：amber 背景 `#FFB800`，深色文字，hover `#FF8A00`
- Attach 按钮：透明背景，边框样式，icon 或文字 "Attach"

### 4.6 SessionListWidget（改造）

保持 `QListWidget`，但调整样式：
- 宽度 240px（从 250px 调整）
- Item：hover 背景 `rgba(255,255,255,0.04)`，选中背景 `rgba(255,184,0,0.08)` + amber 左边框
- "New Chat" 按钮：透明背景，边框 `#2a2e36`，mono 11px uppercase，hover 边框变 amber

### 4.7 ConversationHeader（新增）

聊天区域顶部的会话标题栏：

```cpp
class ConversationHeader : public QWidget {
    Q_OBJECT
public:
    explicit ConversationHeader(QWidget *parent = nullptr);
    void setTitle(const QString &title);
private:
    QLabel *m_title;
};
```

高度 44px，mono 12px uppercase，`#8a8680`，底部边框 `#2a2e36`。

### 4.8 ChatWidget（改造）

整合新组件：

```cpp
class ChatWidget : public QWidget {
    Q_OBJECT
public:
    explicit ChatWidget(QWidget *parent = nullptr);
    void setClient(HermindClient *client);
    void addMessage(bool isUser, const QString &html);
    void setEmptyState(bool empty);
    void appendToCurrentBubble(const QString &text);
    void finalizeCurrentBubble();

protected:
    void dragEnterEvent(QDragEnterEvent *event) override;
    void dropEvent(QDropEvent *event) override;

private:
    void setupUI();
    void sendMessage(const QString &text);
    void onRenderResponse(const QJsonObject &response);
    void onStreamEvent(const QString &eventType, const QString &data);

    ConversationHeader *m_header;
    QWidget *m_messageContainer;
    QVBoxLayout *m_messageLayout;
    QScrollArea *m_scrollArea;
    EmptyStateWidget *m_emptyState;
    PromptInput *m_promptInput;
    HermindClient *m_client;
    MessageBubble *m_currentBubble;
    QString m_currentMarkdown;
    QTimer *m_renderTimer;
};
```

布局：`QVBoxLayout`（header + message area + composer），message area 内部 `QStackedWidget` 切换 EmptyState/MessageList。

---

## 5. 文件变更清单

### 5.1 重写文件（保留文件名，内容全量替换）

| 文件 | 变更类型 | 说明 |
|------|---------|------|
| `desktop/resources/styles.qss` | 重写 | 新颜色系统 |
| `desktop/src/appwindow.h` | 重写 | 去掉 MenuBar，添加 TopBar/Footer |
| `desktop/src/appwindow.cpp` | 重写 | 新布局逻辑 |
| `desktop/src/messagebubble.h` | 重写 | 去掉 avatar，添加 role tag |
| `desktop/src/messagebubble.cpp` | 重写 | 新样式 |
| `desktop/src/promptinput.h` | 重写 | 添加 attach 按钮 |
| `desktop/src/promptinput.cpp` | 重写 | 新样式 |
| `desktop/src/sessionlistwidget.h` | 重写 | 保持接口 |
| `desktop/src/sessionlistwidget.cpp` | 重写 | 新样式、宽度调整 |
| `desktop/src/chatwidget.h` | 重写 | 添加 EmptyState、Header |
| `desktop/src/chatwidget.cpp` | 重写 | 新布局 |

### 5.2 新增文件

| 文件 | 说明 |
|------|------|
| `desktop/src/topbar.h` / `.cpp` | 自定义顶部栏 |
| `desktop/src/statusfooter.h` / `.cpp` | 底部状态栏 |
| `desktop/src/emptystatewidget.h` / `.cpp` | 空状态占位 |
| `desktop/src/conversationheader.h` / `.cpp` | 会话标题栏 |

### 5.3 不变文件

| 文件 | 原因 |
|------|------|
| `desktop/src/main.cpp` | 仅入口，不受 UI 样式影响 |
| `desktop/src/hermindprocess.h/.cpp` | 进程管理，与 UI 无关 |
| `desktop/src/httplib.h/.cpp` | HTTP 客户端，与 UI 无关 |
| `desktop/src/sseparser.h/.cpp` | SSE 解析，与 UI 无关 |
| `desktop/src/shortcutmanager.h/.cpp` | 快捷键，与 UI 无关 |
| `desktop/src/trayicon.h/.cpp` | 托盘图标，与 UI 无关 |
| `desktop/src/settingsdialog.h/.cpp` | 设置对话框，本次不涉及 |
| `desktop/CMakeLists.txt` | 仅需添加新源文件到 target |
| `desktop/resources/resources.qrc` | 仅需确认 styles.qss 路径 |

### 5.4 CMakeLists.txt 变更

在 `add_executable(hermind-desktop ...)` 中添加新源文件：

```cmake
add_executable(hermind-desktop
    src/main.cpp
    src/appwindow.cpp
    src/chatwidget.cpp
    src/messagebubble.cpp
    src/promptinput.cpp
    src/sessionlistwidget.cpp
    src/httplib.cpp
    src/sseparser.cpp
    src/hermindprocess.cpp
    src/shortcutmanager.cpp
    src/trayicon.cpp
    src/settingsdialog.cpp
    # === 新增 ===
    src/topbar.cpp
    src/statusfooter.cpp
    src/emptystatewidget.cpp
    src/conversationheader.cpp
)
```

---

## 6. 字体策略

使用系统字体 fallback，不嵌入自定义字体：

```cpp
// 在 main.cpp 中设置全局字体
QFont sansFont("-apple-system");  // macOS
#ifdef Q_OS_WIN
    sansFont = QFont("Segoe UI");
#endif
sansFont.setPointSize(10);  // ~13px
QApplication::setFont(sansFont);

// Mono 字体用于标签和代码
// JetBrains Mono → SF Mono → Menlo → Consolas → monospace
```

QSS 中：
```qss
/* 不需要显式设置字体族，依赖应用级默认 */
/* 特殊 mono 场景用内联 style */
```

---

## 7. 编译验证策略

方案 B（全量重写）的风险控制：

1. **本地编译验证**：`cd desktop/build && cmake .. && cmake --build .`
2. **Qt 测试运行**：`ctest`（test_httplib、test_sseparser 应不受影响）
3. **运行时验证**：启动应用，检查：
   - 颜色是否正确（无蓝色残留）
   - TopBar 是否显示
   - 空状态是否出现
   - 发送消息后气泡样式是否正确
   - 侧边栏选中态是否正确
   - 滚动条样式是否正确

---

## 8. 风险与缓解

| 风险 | 影响 | 缓解 |
|------|------|------|
| 全量重写编译错误多 | 高 | 按依赖顺序逐个文件验证编译；先改 QSS，再改组件 |
| Qt QSS 不支持某些 CSS 特性 | 中 | 用 border 替代 box-shadow；用静态样式替代 transition |
| QListWidget 样式受限 | 中 | 如需更精细控制，可改用 QListView + 自定义 delegate |
| 布局计算差异 | 低 | Qt Layout 与 CSS flexbox 行为类似，测试验证即可 |

---

## 9. 验收标准

- [ ] 应用启动后背景色为 `#0a0b0d`，无任何蓝色元素
- [ ] TopBar 显示品牌、模式切换、状态点、Save 按钮（amber 色）
- [ ] 空会话时显示 "How can I help you today?" + 建议按钮
- [ ] 用户消息气泡：透明背景 + amber 边框，右上角显示 "YOU"
- [ ] 助手消息气泡：`#14161a` 背景，左上角显示 "HERMIND"
- [ ] 输入框 focus 时边框变为 amber
- [ ] Send 按钮为 amber 背景
- [ ] 侧边栏选中项有 amber 左边框 + 浅 amber 背景
- [ ] 底部有状态栏显示版本信息
- [ ] 滚动条为细滚动条样式
