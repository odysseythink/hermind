# Qt Desktop 100% 功能对等重写设计文档

**日期**: 2026-05-15  
**作者**: Kimi Code CLI  
**状态**: 待实现  
**方案**: 方案 A（基础设施保留 + UI 组件全面扩展）

---

## 1. 概述

本项目是一个 Go 后端 + React Web 前端的 AI 助手应用（Hermind）。目前 Web UI 功能丰富，但已有一个基于 Qt6 Widgets 的桌面应用（`desktop/` 目录），其功能尚未与 Web UI 对齐。

**本次设计的目标**：让 Qt Desktop 应用达到与 Web UI 100% 功能对等，同时保留 Web UI 作为 Linux/无图形环境的前端备选。

**关键约束**：
- 必须使用纯 Qt6 Widgets，**不可用 QWebEngineView**（当前构建环境为 LLVM MinGW，QWebEngineView 依赖 MSVC）
- 在现有 `desktop/` 代码基础上扩展，而非彻底重构
- 一次性完整交付，所有功能到位

---

## 2. 目标与平台策略

| 平台 | 前端选择 | 理由 |
|---|---|---|
| Windows | Qt Desktop | 桌面程序体验优于 Web |
| macOS | Qt Desktop | 桌面程序体验优于 Web |
| Linux / 无图形环境 | Web UI | 无 Qt 显示环境时可用 |

Web UI（`web/` 目录和嵌入的 `api/webroot/`）**保留不变**，Go 后端继续同时服务两个前端。

---

## 3. 架构总览

### 3.1 现有架构（保留部分）

现有 `desktop/` 已经具备稳定的基础设施：

- `HermindProcess`：启动并管理 Go 后端子进程
- `HermindClient`（`httplib.h/cpp`）：HTTP 客户端，封装 GET/POST/流式请求
- `SSEParser`：SSE（Server-Sent Events）流解析器
- `ShortcutManager`：全局快捷键（如 Ctrl+Shift+H 切换窗口）
- `TrayIcon`：系统托盘图标（隐藏/显示/退出）
- `AppWindow`：主窗口框架（TopBar + QSplitter + SessionList + ChatWidget + StatusFooter）
- `ChatWidget`：聊天核心（消息列表、SSE 流、PromptInput、EmptyState、拖拽上传）

### 3.2 扩展后的模块结构

```
┌─────────────────────────────────────────────────────────────┐
│                      AppWindow (QWidget)                     │
│  ┌─────────┐  ┌──────────────────────────────────────────┐  │
│  │ TopBar  │  │              QSplitter                    │  │
│  │(mode,   │  │  ┌──────────────┐  ┌──────────────────┐  │  │
│  │ i18n,   │  │  │SessionList   │  │   ChatWidget      │  │  │
│  │ theme)  │  │  │Widget        │  │  ┌──────────────┐ │  │  │
│  └─────────┘  │  │              │  │  │Conversation  │ │  │  │
│               │  │ • loadSession│  │  │Header        │ │  │  │
│               │  │ • newSession │  │  ├──────────────┤ │  │  │
│               │  │              │  │  │ MessageArea  │ │  │  │
│               │  └──────────────┘  │  │ (scrollable) │ │  │  │
│               │                    │  ├──────────────┤ │  │  │
│               │                    │  │ PromptInput  │ │  │  │
│               │                    │  └──────────────┘ │  │  │
│               │                    └──────────────────┘  │  │
│               └──────────────────────────────────────────┘  │
│  ┌─────────────┐                                            │
│  │ StatusFooter│                                            │
│  └─────────────┘                                            │
└─────────────────────────────────────────────────────────────┘
                    ↕ HTTP / SSE
┌─────────────────────────────────────────────────────────────┐
│  HermindClient  ←──→  Go Backend (localhost:random_port)    │
│  • GET /api/conversation                                    │
│  • POST /api/conversation/messages                          │
│  • GET /api/sse (streaming)                                 │
│  • GET/PUT /api/config                                      │
│  • etc.                                                     │
└─────────────────────────────────────────────────────────────┘
```

### 3.3 新增/扩展的组件

| 组件 | 类型 | 说明 |
|---|---|---|
| `MessageBubble` | 扩展 | 增强 Markdown 渲染，增加操作按钮（编辑/删除/重生成/复制） |
| `SettingsEditor` | **重写** | 替代现有 `SettingsDialog`，扩展为分组侧边栏式完整配置编辑器 |
| `TopBar` | 扩展 | 增加语言切换按钮、主题切换按钮 |
| `EmptyStateWidget` | 扩展 | 连接 `/api/suggestions` 动态加载建议卡片 |
| `ChatWidget` | 扩展 | 增加消息编辑/删除/重生成、工具调用可视化、停止生成按钮 |
| `HermindClient` | 扩展 | 新增 `put()`、`deleteResource()`、`upload()` 等方法 |
| `MarkdownRenderer` | **新增** | 封装 Markdown → QTextDocument 的渲染逻辑 |
| `ConfigFormEngine` | **新增** | 根据 `/api/config/schema` 动态生成 Qt 表单控件 |
| `ToolCallWidget` | **新增** | 工具调用过程的可视化展示卡片 |
| `I18n` | **新增** | 国际化管理，加载 `.qm` 翻译文件 |
| `ThemeManager` | **新增** | 主题切换，加载/切换 QSS 样式表 |
| `SettingsSidebar` | **新增** | 设置编辑器左侧分组导航 |
| `SettingsPanel` | **新增** | 设置编辑器右侧内容区域容器 |
| `ProviderEditor` | **新增** | Provider 管理子面板 |
| `FallbackEditor` | **新增** | 可拖拽排序的 Fallback provider 列表 |
| `McpServerEditor` | **新增** | MCP 服务器管理 |
| `CronEditor` | **新增** | Cron 任务调度编辑器 |
| `ScalarEditor` | **新增** | 标量配置编辑器（用于 Runtime/Memory/Skills 等） |
| `CodeHighlighter` | **新增** | 基于 `QSyntaxHighlighter` 的代码高亮器 |

---

## 4. 聊天模块设计

### 4.1 MessageBubble 扩展

现有 `MessageBubble` 使用 `QTextEdit`（只读模式）显示内容。扩展后结构：

```cpp
class MessageBubble : public QWidget
{
    Q_OBJECT
public:
    explicit MessageBubble(bool isUser, QWidget *parent = nullptr);
    
    void appendMarkdown(const QString &text);
    QString markdownBuffer() const;
    void setHtmlContent(const QString &html);
    
    // 新增
    void setMessageId(const QString &id);
    void setStreaming(bool streaming);
    void addToolCallWidget(ToolCallWidget *widget);
    
private:
    bool m_isUser;
    QString m_messageId;
    bool m_isStreaming;
    QString m_markdownBuffer;
    
    QLabel *m_roleTag;
    QTextEdit *m_content;
    MarkdownRenderer *m_renderer;
    
    // 操作按钮区域（仅助手消息）
    QWidget *m_actionBar;
    QPushButton *m_editBtn;
    QPushButton *m_deleteBtn;
    QPushButton *m_regenerateBtn;
    QPushButton *m_copyBtn;
    
    // 工具调用展示区域
    QWidget *m_toolCallsArea;
};
```

**操作按钮交互**：
- **编辑**：消息变为可编辑文本框 → 修改后 `PUT /api/conversation/messages/{id}`
- **删除**：确认对话框 → `DELETE /api/conversation/messages/{id}` → 从列表移除
- **重生成**：删除当前助手消息 → 重新发送前一条用户消息
- **复制**：复制原始 Markdown 文本到剪贴板

### 4.2 流式输出与防抖渲染

保留并优化现有的 `QTimer` 防抖渲染机制：

```cpp
void ChatWidget::appendToCurrentBubble(const QString &text)
{
    m_pendingMarkdown += text;
    m_renderGeneration++;
    m_renderTimer->start(50); // 50ms 防抖
}

void ChatWidget::onRenderTimer()
{
    int gen = m_renderGeneration;
    m_currentBubble->setMarkdownBuffer(m_pendingMarkdown);
    
    m_markdownRenderer->renderAsync(m_pendingMarkdown, [this, gen](const QString &html) {
        if (gen != m_renderGeneration) return; // 丢弃过期渲染
        m_currentBubble->setHtmlContent(html);
    });
}
```

### 4.3 工具调用可视化

新增 `ToolCallWidget`，在 SSE 流中收到 `tool_calls` 事件时动态展示：

```cpp
class ToolCallWidget : public QWidget
{
    Q_OBJECT
public:
    void setToolName(const QString &name);
    void setStatus(const QString &status); // "running" / "success" / "error"
    void setArguments(const QJsonObject &args);
    void setResult(const QJsonValue &result);
    
private:
    QLabel *m_statusIcon;    // ⏳ / ✅ / ❌
    QLabel *m_toolNameLabel;
    QPushButton *m_expandBtn;
    QWidget *m_detailsPanel; // 展开后显示参数和结果
};
```

### 4.4 停止生成

流式进行中时，`PromptInput` 区域显示停止按钮替代发送按钮：

```
┌────────────────────────────────────┐
│  输入框内容...                      │
│                                    │
├────────────────────────────────────┤
│ [📎] [⏹ 停止生成]                   │
└────────────────────────────────────┘
```

点击后调用 `POST /api/conversation/cancel` 并中断 `QNetworkReply`。

### 4.5 附件上传

扩展 `ChatWidget` 的拖拽支持：
- 拖拽文件到输入区域 → `POST /api/upload` → 文件 URL 插入消息体
- `PromptInput` 增加附件按钮选择文件

---

## 5. Markdown 渲染策略

由于无法使用 QWebEngineView，采用**分层渲染**策略：基础元素内联渲染，复杂元素弹窗查看。

### 5.1 渲染层级表

| 元素 | 渲染方式 | 说明 |
|---|---|---|
| 标题、段落、列表、引用、分割线 | `QTextDocument::setMarkdown()` | Qt6 内置，完美支持 |
| 表格 | `QTextDocument::setMarkdown()` | Qt6.5+ 支持 GFM 表格 |
| 行内代码 `` `code` `` | `QTextDocument::setMarkdown()` + 自定义样式 | 灰色背景、等宽字体 |
| 代码块 | `CodeHighlighter` + `QTextEdit` | 基于 `QSyntaxHighlighter` |
| 链接 | `QTextDocument::setMarkdown()` + 信号 | 点击打开系统浏览器 |
| 图片 | `QTextDocument::addResource()` + `QPixmap` | 异步下载后嵌入 |
| **数学公式** `$...$` / `$$...$$` | 文本占位符 + 点击弹窗 | 调用后端 `/api/render/math` |
| **Mermaid 图表** | 代码块 + 查看按钮 | 调用后端 `/api/render/mermaid` |

### 5.2 代码高亮

使用自定义 `CodeHighlighter : public QSyntaxHighlighter`，基于正则表达式实现：

```cpp
class CodeHighlighter : public QSyntaxHighlighter
{
    Q_OBJECT
public:
    explicit CodeHighlighter(QTextDocument *parent = nullptr);
    void setLanguage(const QString &language); // "cpp", "go", "python", etc.
    
protected:
    void highlightBlock(const QString &text) override;
    
private:
    struct Rule {
        QRegularExpression pattern;
        QTextCharFormat format;
    };
    QVector<Rule> m_rules;
    
    // 预定义语言规则
    void loadCppRules();
    void loadGoRules();
    void loadPythonRules();
    // ... 等常见语言
};
```

**支持语言**（覆盖 90% 使用场景）：C/C++, Go, Python, JavaScript, TypeScript, Rust, Java, Bash, JSON, YAML, Markdown, SQL。

**样式来源**：从 `ThemeManager` 获取当前主题的配色配置。

### 5.3 数学公式渲染

流式输出阶段显示原始 LaTeX 文本。流结束后，如果检测到完整公式，替换为占位符：

```
┌─────────────────────────────────────────┐
│  E = mc²  [🔍 查看公式]                  │
└─────────────────────────────────────────┘
        ↓ 点击
┌─────────────────────────────────────────┐
│           公式渲染弹窗                   │
│  ┌─────────────────────────────────┐    │
│  │        (KaTeX 渲染的 SVG)        │    │
│  │                                 │    │
│  └─────────────────────────────────┘    │
│       [复制 LaTeX] [关闭]                │
└─────────────────────────────────────────┘
```

**实现流程**：
1. `MarkdownRenderer` 解析阶段检测 `$...$` 或 `$$...$$`
2. 在 `QTextDocument` 中用占位符替代（如 `〖formula:0〗`）
3. 用户点击 → 调用 `POST /api/render/math` → 后端用 KaTeX CLI 生成 SVG
4. Qt 弹窗用 `QLabel` + `QPixmap` 显示

### 5.4 Mermaid 图表渲染

```
┌─────────────────────────────────────────┐
│  ```mermaid                             │
│  graph TD;                               │
│    A-->B;                                │
│  ```                                     │
│  [📊 查看图表]                            │
└─────────────────────────────────────────┘
        ↓ 点击
┌─────────────────────────────────────────┐
│           图表渲染弹窗                   │
│  ┌─────────────────────────────────┐    │
│  │        (Mermaid 渲染的 SVG)      │    │
│  │                                 │    │
│  └─────────────────────────────────┘    │
│       [复制源码] [关闭]                  │
└─────────────────────────────────────────┘
```

类似数学公式，调用 `POST /api/render/mermaid`。

### 5.5 MarkdownRenderer 类设计

```cpp
class MarkdownRenderer : public QObject
{
    Q_OBJECT
public:
    explicit MarkdownRenderer(QObject *parent = nullptr);
    
    QString renderSync(const QString &markdown);
    void renderAsync(const QString &markdown, 
                     std::function<void(const QString&)> callback);
    
private:
    QString preprocessMath(const QString &markdown);
    QString preprocessMermaid(const QString &markdown);
    QString postprocessCodeBlocks(const QString &html);
};
```

---

## 6. 设置编辑器设计

### 6.1 整体布局

现有 `SettingsDialog`（简单 QDialog）被 `SettingsEditor`（分组侧边栏式）替代：

```
┌─────────────────────────────────────────────────────────────────┐
│  设置                                        [× 关闭] [💾 保存]   │
├──────────────────┬──────────────────────────────────────────────┤
│  📁 Models       │                                              │
│     Providers    │  ┌────────────────────────────────────────┐  │
│     Fallbacks    │  │ Provider 管理                          │  │
│     Default Model│  │ ┌─────────────┐  ┌─────────────────┐   │  │
│  ─────────────── │  │ │ OpenAI   [×]│  │ API Key: *****  │   │  │
│  ⚙️ Advanced     │  │ │ Anthropic[×]│  │ Base URL: ...   │   │  │
│     MCP Servers  │  │ │ + Add New   │  │ Model: [下拉框 ▼]│   │  │
│     Cron         │  │ └─────────────┘  └─────────────────┘   │  │
│     Auxiliary    │  │                                        │  │
│  ─────────────── │  │ Fallback Providers (可拖拽排序)         │  │
│  🌐 Gateway      │  │ ┌──────────────────────────────────┐   │  │
│     Telegram     │  │ │ 1. OpenRouter  [↑] [↓] [×]      │   │  │
│  ─────────────── │  │ │ 2. DeepSeek    [↑] [↓] [×]      │   │  │
│  🔧 Runtime      │  │ │ + Add Fallback                   │   │  │
│  🧠 Memory       │  │ └──────────────────────────────────┘   │  │
│  🛠️ Skills       │  │                                        │  │
│                  │  │ Default Model: [gpt-4o          ▼]    │  │
│                  │  └────────────────────────────────────────┘  │
│  [分组有 ● 脏标记] │                                              │
└──────────────────┴──────────────────────────────────────────────┘
```

### 6.2 ConfigFormEngine（动态表单渲染引擎）

Web UI 通过 `GET /api/config/schema` 获取配置 schema，然后动态渲染表单。Qt 版本需要同样的能力。

```cpp
class ConfigFormEngine : public QObject
{
    Q_OBJECT
public:
    explicit ConfigFormEngine(QObject *parent = nullptr);
    
    // 根据 schema 节点生成对应的 QWidget
    QWidget* buildForm(const QJsonObject &schema, 
                       const QJsonObject &currentValues);
    
    // 收集表单当前值
    QJsonObject collectValues(QWidget *formRoot);
    
    // 检查是否有变更
    bool isDirty() const;
    
signals:
    void valueChanged(const QString &key, const QJsonValue &value);
    void dirtyStateChanged(bool dirty);
    
private:
    QWidget* createStringEditor(const QJsonObject &fieldSchema);
    QWidget* createNumberEditor(const QJsonObject &fieldSchema);
    QWidget* createBoolEditor(const QJsonObject &fieldSchema);
    QWidget* createSelectEditor(const QJsonObject &fieldSchema);
    QWidget* createMapEditor(const QJsonObject &fieldSchema);
    QWidget* createListEditor(const QJsonObject &fieldSchema);
    QWidget* createSecretEditor(const QJsonObject &fieldSchema);
};
```

### 6.3 脏状态跟踪

- 每个 `SettingsPanel` 子面板监听内部控件变更信号
- 变更时向 `SettingsEditor` 报告，侧边栏对应分组显示 `●` 标记
- `TopBar` 的保存按钮同时高亮
- 保存时调用 `PUT /api/config`，后端自动处理 secret 保留逻辑（空值 = 保留原值）

### 6.4 Provider 管理详细交互

1. **添加 Provider**：点击 "+ Add" → 选择 provider 类型 → 列表新增一行
2. **测试连接**：每个 provider 卡片有 "Test" 按钮 → `POST /api/providers/{name}/test` → 显示状态
3. **获取模型列表**：点击 "Fetch Models" → `POST /api/providers/{name}/models` → 下拉框更新
4. **删除 Provider**：确认对话框后从列表移除

---

## 7. 国际化与主题

### 7.1 国际化（i18n）

使用 Qt 原生 **Qt Linguist** 方案：

```
desktop/resources/
├── i18n/
│   ├── hermind_en.ts      # 英文翻译源文件
│   ├── hermind_zh_CN.ts   # 简体中文翻译源文件
│   ├── hermind_en.qm      # 编译后的二进制翻译文件
│   └── hermind_zh_CN.qm
```

**翻译流程**：
1. 代码中使用 `tr("Original English")` 标记所有用户可见字符串
2. 使用 `lupdate` 提取待翻译字符串到 `.ts` 文件
3. 用 Qt Linguist 或手动编辑填写翻译
4. 用 `lrelease` 编译 `.ts` → `.qm`
5. `.qm` 文件通过 `resources.qrc` 嵌入可执行文件

**运行时切换**：

```cpp
class I18n : public QObject
{
    Q_OBJECT
public:
    void loadLanguage(const QString &language); // "en" 或 "zh_CN"
    QStringList availableLanguages() const;
    
signals:
    void languageChanged(const QString &language);
    
private:
    QTranslator *m_translator;
    QString m_currentLanguage;
};
```

切换语言时移除旧 `QTranslator`，加载新 `.qm`，发送信号，所有 UI 组件重新 `RetranslateUi()`。

### 7.2 主题系统

双主题 QSS 系统：

```
desktop/resources/
├── themes/
│   ├── dark.qss         # 暗色主题
│   └── light.qss        # 亮色主题
```

**ThemeManager**：

```cpp
class ThemeManager : public QObject
{
    Q_OBJECT
public:
    void loadTheme(const QString &themeName); // "dark" 或 "light"
    QString currentTheme() const;
    QColor accentColor() const;
    QColor backgroundColor() const;
    QColor textColor() const;
    
signals:
    void themeChanged(const QString &themeName);
};
```

**暗色主题色值**（与现有 `styles.qss` 保持一致）：

| 用途 | 色值 |
|---|---|
| 背景主色 | `#0a0b0d` |
| 背景次色 | `#141619` |
| 卡片/气泡背景 | `#1e2028` |
| 边框 | `#2a2d35` |
| 文字主色 | `#e8e6e3` |
| 文字次色 | `#8b8f98` |
| 强调色 | `#FFB800` |
| 用户气泡背景 | `#2a3f5f` |
| 助手气泡背景 | `#1e2028` |
| 成功 | `#4ade80` |
| 错误 | `#f87171` |

**亮色主题**：新增一套与 Web UI 亮主题对齐的配色。

---

## 8. 新增后端 API

Qt Desktop 需要后端提供少量新增 API：

### 8.1 `POST /api/render/math`

KaTeX 渲染数学公式为 SVG/PNG。

**请求**：
```json
{
  "latex": "E = mc^2",
  "displayMode": false,
  "format": "svg"
}
```

**响应**：
```json
{
  "data": "<svg>...</svg>",
  "format": "svg"
}
```

**后端实现**：调用 `npx katex` 子进程。

### 8.2 `POST /api/render/mermaid`

Mermaid 渲染图表为 SVG/PNG。

**请求**：
```json
{
  "diagram": "graph TD; A-->B;",
  "format": "svg"
}
```

**响应**：
```json
{
  "data": "<svg>...</svg>",
  "format": "svg"
}
```

**后端实现**：调用 `npx @mermaid-js/mermaid-cli` 子进程。

### 8.3 依赖与降级

这些渲染 API 需要 Node.js 工具链。桌面应用打包时可选择 bundled 一个精简的 Node.js 运行时 + 必要 npm 包。

**降级策略**：如果用户系统没有这些工具，数学公式和 Mermaid 图表显示为原始源码（不弹出渲染失败错误）。

---

## 9. 测试策略

### 9.1 单元测试（Qt Test）

| 测试文件 | 测试内容 |
|---|---|
| `test_httplib.cpp` | 已有：HermindClient GET/POST 请求 |
| `test_sseparser.cpp` | 已有：SSE 事件解析 |
| `test_markdown_renderer.cpp` | **新增**：Markdown → HTML 渲染正确性 |
| `test_config_form_engine.cpp` | **新增**：Schema → Qt 控件映射、值收集、脏状态检测 |
| `test_i18n.cpp` | **新增**：语言切换、翻译加载 |

### 9.2 集成测试

- 启动 `hermind-desktop` → 验证 `HermindProcess` 正确启动后端 → 验证能连接 `GET /api/status`
- 发送消息 → 验证 SSE 流式回复正常显示
- 修改设置 → 验证 `PUT /api/config` 成功保存

### 9.3 手动测试清单

| 功能 | 检查点 |
|---|---|
| 聊天 | 发送消息、流式显示、停止生成、编辑/删除/重生成消息 |
| Markdown | 标题、列表、表格、代码块高亮、链接点击、图片显示 |
| 数学公式 | 显示占位符、点击弹窗、渲染结果正确 |
| Mermaid | 显示代码块+按钮、点击弹窗、渲染结果正确 |
| 设置 | 所有分组可切换、表单控件正常、脏状态标记、保存成功 |
| Provider | 添加/删除/测试连接/获取模型列表 |
| 主题 | 暗/亮切换、颜色正确应用 |
| 语言 | 中/英切换、所有字符串更新 |
| 系统托盘 | 隐藏/显示、全局快捷键、退出 |

---

## 10. 实施注意事项

1. **构建系统**：继续使用 CMake，确保新增源文件加入 `CMakeLists.txt` 的 `SOURCES` 和 `HEADERS` 列表。
2. **Qt6 版本要求**：需要 Qt 6.5+ 以获得 `QTextDocument::setMarkdown()` 对 GFM 表格的支持。
3. **C++ 标准**：保持 C++17（与现有代码一致）。
4. **文件组织**：新增组件统一放入 `desktop/src/`，头文件 `.h` 和实现 `.cpp` 配对。
5. **信号槽命名**：遵循现有代码风格（`m_` 前缀成员变量，驼峰命名）。
6. **向后兼容**：Web UI 不做任何修改，Go 后端新增 API 不应影响现有 Web UI 功能。
