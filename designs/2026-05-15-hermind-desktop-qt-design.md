# hermind 桌面版设计文档

**日期**: 2026-05-15  
**状态**: 已确认，待实现  
**作者**: AI-assisted design session  

---

## 1. 背景与目标

hermind 目前提供两种使用方式：
- `hermind web`：浏览器内 Web UI（React 18 + Vite）
- `hermind run`：CLI 单轮对话

本设计为 hermind 增加**原生桌面应用**，覆盖 macOS 和 Windows 平台。Linux 及服务器环境继续使用现有的 `hermind web` 浏览器模式。

### 1.1 设计约束

- **Wails 已排除**：前期尝试引入 Wails 时发生严重事故（删除用户文件），任何基于 WebView 的框架（包括 Qt WebView）均不再考虑。
- **React Web UI 保留**：现有 `web/` 目录的 React 前端继续维护，用于 Web/Linux 模式。
- **Go 后端复用**：桌面版不重复实现 LLM 调用、配置管理、对话状态等核心逻辑。
- **安全优先**：桌面应用与文件系统交互必须经过显式用户授权，禁止静默删除/覆盖用户文件。

### 1.2 目标平台

| 平台 | 交付形态 | 说明 |
|------|---------|------|
| macOS | `hermind-desktop.app` | 原生 .app bundle，双击启动 |
| Windows | `hermind-desktop.exe` | 单文件 portable 或安装包，双击启动（无控制台窗口）|
| Linux | `hermind web` | 不变，继续使用浏览器 |

---

## 2. 架构概述

```
┌─────────────────────────────────────────────────────────────┐
│                        hermind                               │
├─────────────────────────┬───────────────────────────────────┤
│    Desktop (mac/win)    │      Web / Server (all platforms) │
│                         │                                   │
│  ┌─────────────────┐    │   ┌──────────────────────────┐    │
│  │   Qt 6 Widgets   │    │   │   Browser (React/Vite)   │    │
│  │  C++ Native UI   │    │   │   fetch/SSE → Go HTTP    │    │
│  │                 │    │   └──────────────────────────┘    │
│  │  QNetworkAccess │◄───┼───►                               │
│  │     Manager      │    │                                   │
│  └────────┬────────┘    │   ┌──────────────────────────┐    │
│           │ HTTP 127.0.0.1   │   Go Backend (hermind web)│   │
│  ┌────────▼────────┐    │   └──────────────────────────┘    │
│  │   Go Backend     │◄───┼───┤                               │
│  │ (复用现有 api/)  │    │                                   │
│  └─────────────────┘    │                                   │
└─────────────────────────┴───────────────────────────────────┘
```

**核心原则**：
- Go 后端几乎不变，新增 `hermind desktop` 子命令。
- Qt 桌面应用是一个新的构建产物，前端 UI 完全用 C++ Qt Widgets 重写。
- 通信通过本地 HTTP，复用现有 REST API 和 SSE 端点。

---

## 3. 目录结构

```
hermind/
├── api/                    # 现有 Go HTTP API（不变）
├── web/                    # 现有 React Web UI（不变）
├── cmd/hermind/            # 现有 Go 入口（新增 desktop 子命令）
├── desktop/                # 新增：Qt 桌面应用
│   ├── CMakeLists.txt
│   ├── src/
│   │   ├── main.cpp
│   │   ├── appwindow.h / .cpp          # 主窗口
│   │   ├── chatwidget.h / .cpp         # 聊天界面
│   │   ├── messagebubble.h / .cpp      # 消息气泡组件
│   │   ├── codeblock.h / .cpp          # 代码块高亮组件
│   │   ├── settingsdialog.h / .cpp     # 设置面板
│   │   ├── httplib.h / .cpp            # Go 后端 HTTP 客户端封装
│   │   ├── sseparser.h / .cpp          # SSE 流解析器
│   │   ├── shortcutmanager.h / .cpp    # 全局快捷键
│   │   ├── trayicon.h / .cpp           # 系统托盘 + 通知
│   │   ├── dragdrop.h / .cpp           # 文件拖放处理
│   │   ├── hermindprocess.h / .cpp     # Go 子进程生命周期管理
│   │   └── configstore.h / .cpp        # 桌面端本地配置缓存
│   ├── resources/
│   │   ├── icons/
│   │   └── styles.qss                  # Qt 样式表（仿现有 Web UI 主题）
│   └── tests/                          # Qt Test 单元测试
├── ...
```

---

## 4. Go 后端适配

### 4.1 新增 `hermind desktop` 子命令

在 `cli/app.go` 中新增子命令：

```go
{
    Name:  "desktop",
    Usage: "启动桌面版后端服务（无浏览器、仅文件日志）",
    Action: func(c *cli.Context) error {
        cfg := config.Load()
        return api.RunDesktopServer(cfg)
    },
}
```

### 4.2 桌面模式专属行为

| 行为 | Web 模式 (`hermind web`) | 桌面模式 (`hermind desktop`) |
|------|------------------------|---------------------------|
| 端口绑定 | 随机 [30000,40000) | 随机 [40000,50000) 或 `0`（由 OS 分配） |
| 自动打开浏览器 | ✅ 是 | ❌ 否 |
| 日志输出 | stdout + 文件 | **仅文件** |
| 进程信号处理 | Ctrl+C 退出 | **SIGTERM / taskkill 优雅退出** |
| 健康检查 | 无 | **`GET /health`** 返回 `{"status":"ok"}` |
| 就绪通知 | 无 | 启动后打印 `HERMIND_READY <port>` 到 stdout |
| Markdown 渲染端点 | 无 | **`POST /api/render`**（见 5.1） |

### 4.3 Windows 无控制台编译

桌面版 Go 后端在 Windows 上使用 GUI 子系统编译：

```bash
GOOS=windows GOARCH=amd64 go build \
  -ldflags "-H=windowsgui" \
  -o desktop/resources/hermind-desktop-backend.exe \
  ./cmd/hermind
```

`-H=windowsgui` 确保双击启动时**不弹出控制台窗口**。

### 4.4 日志目录

| 平台 | 路径 |
|------|------|
| macOS | `~/Library/Logs/hermind/` |
| Windows | `%LOCALAPPDATA%\hermind\logs\` |

Go 后端在桌面模式下将日志只写入上述目录，不输出 stdout/stderr。

---

## 5. Markdown 与代码高亮方案

Web UI 使用 `react-markdown` + `shiki` 渲染富文本消息。纯 Qt Widgets 方案采用**后端预渲染**策略：

### 5.1 新增 `POST /api/render` 端点

Go 后端新增端点，将 Markdown 文本渲染为 Qt 兼容的 HTML：

```go
type RenderRequest struct {
    Content string `json:"content"`
}

type RenderResponse struct {
    HTML string `json:"html"`
}
```

使用 `github.com/yuin/goldmark` 解析 Markdown（支持 GFM、表格、任务列表），扩展支持：
- `goldmark-highlighting`（基于 `chroma`）做代码语法高亮，生成内联 style 的 HTML
- KaTeX 预渲染数学公式为 HTML 片段（复用现有 `api/webroot` 内的 KaTeX 资源）

Qt 前端收到 HTML 后直接通过 `QTextEdit::setHtml()` 显示。

**限制说明**：`QTextEdit` 不支持全部 CSS，但基础排版（段落、列表、代码块背景、表格）均可正常渲染。对于 `QTextEdit` 不支持的复杂样式，后端降级为纯文本输出。

### 5.2 消息渲染流程

```
[收到 SSE 消息增量]
    │
    ▼
[Qt 累积原始 Markdown 文本]
    │
    ▼
[POST /api/render → Go 后端]
    │
    ▼
[Go goldmark + chroma → HTML]
    │
    ▼
[Qt setHtml(html) 更新消息气泡]
```

---

## 6. Qt ↔ Go 进程生命周期

### 6.1 启动流程

```cpp
class HermindProcess : public QObject {
    Q_OBJECT
    QProcess *goProcess;
    int backendPort = 0;

public:
    void start() {
        QString goBinary = QCoreApplication::applicationDirPath()
                         + "/hermind-desktop-backend";
#ifdef Q_OS_WIN
        goBinary += ".exe";
#endif
        goProcess = new QProcess(this);

#ifdef Q_OS_WIN
        goProcess->setCreateProcessArgumentsModifier(
            [](QProcess::CreateProcessArguments *args) {
                args->flags |= CREATE_NO_WINDOW;
            });
#endif

        goProcess->start(goBinary, QStringList() << "desktop");

        connect(goProcess, &QProcess::readyReadStandardOutput,
                this, &HermindProcess::onStdout);
    }

    void onStdout() {
        QString output = goProcess->readAllStandardOutput();
        QRegularExpression re("HERMIND_READY (\\d+)");
        auto match = re.match(output);
        if (match.hasMatch()) {
            backendPort = match.captured(1).toInt();
            emit backendReady(QHostAddress::LocalHost, backendPort);
        }
    }

    void shutdown() {
        goProcess->terminate();
        if (!goProcess->waitForFinished(5000)) {
            goProcess->kill();
        }
    }
};
```

### 6.2 关闭流程

1. 用户关闭主窗口 → `AppWindow::closeEvent()`
2. 调用 `HermindProcess::shutdown()` 发送 SIGTERM
3. Go 后端收到信号后优雅关闭 HTTP server、刷新日志、退出
4. Qt 等待最多 5 秒，超时强制 `kill()`

---

## 7. 系统集成设计

### 7.1 全局快捷键

引入第三方库 `QHotkey`（跨平台 `QGlobalShortcut` 实现，基于 `CGEventTap` on macOS 和 `RegisterHotKey` on Windows）：

```cpp
QHotkey *hotkey = new QHotkey(QKeySequence("Ctrl+Shift+H"), true, this);
connect(hotkey, &QHotkey::activated, this, &AppWindow::toggleVisible);
```

默认快捷键：`Ctrl+Shift+H`（Windows）/ `Cmd+Shift+H`（macOS），可自定义。

### 7.2 系统通知

使用 `QSystemTrayIcon::showMessage()`：

```cpp
trayIcon->showMessage(
    "hermind",
    "New message from assistant",
    QSystemTrayIcon::Information,
    3000
);
```

通知触发逻辑：SSE 流检测到新消息完成时，若窗口不在前台，则弹出托盘通知。

### 7.3 文件拖放

`ChatWidget` 实现拖放事件：

```cpp
void ChatWidget::dragEnterEvent(QDragEnterEvent *event) {
    if (event->mimeData()->hasUrls()) {
        event->acceptProposedAction();
    }
}

void ChatWidget::dropEvent(QDropEvent *event) {
    const QMimeData *mime = event->mimeData();
    if (mime->hasUrls()) {
        for (const QUrl &url : mime->urls()) {
            uploadFile(url.toLocalFile());
        }
    }
}
```

支持拖入文本文件、图片等，通过现有 HTTP API 上传处理。

---

## 8. 聊天界面布局

Qt 端采用 `QScrollArea` + 动态 `QWidget` 方案：

```
┌─────────────────────────────────────────┐
│  [≡]  Hermind                [_] [□] [X]│  ← 标题栏
├────────┬────────────────────────────────┤
│        │                                │
│ 会话列表 │      消息区域                   │
│ (List) │      (ScrollArea)              │
│        │                                │
│ ────── │      ┌──────────────────────┐  │
│ 设置    │      │  [附件] 输入框... [▶]│  │  ← 输入栏
│ ────── │      └──────────────────────┘  │
└────────┴────────────────────────────────┘
```

**组件说明**：
- `SessionListWidget`：左侧会话列表，`QListWidget` + 自定义 item delegate
- `MessageBubble`：每条消息一个自定义 `QWidget`，内部：
  - 头像/角色标识（`QLabel` + 图标）
  - 内容区：`QTextEdit`（只读，`setHtml()` 显示后端渲染的 HTML）
  - 操作按钮：复制、重新生成（小图标按钮，悬浮显示）
- `PromptInput`：底部输入栏，`QTextEdit`（多行）+ `QPushButton`

**流式输出**：SSE 收到增量文本后，追加到当前 `MessageBubble` 的原始 Markdown 缓存，每隔 100ms 或收到完整 token 后调用 `/api/render` 刷新 HTML。

---

## 9. 构建系统

`desktop/` 使用 **CMake** + **Qt6**：

```cmake
cmake_minimum_required(VERSION 3.16)
project(hermind-desktop VERSION 0.3.0 LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_AUTOMOC ON)

find_package(Qt6 REQUIRED COMPONENTS Core Gui Widgets Network)

# 可选：QHotkey 作为子模块
add_subdirectory(third_party/QHotkey)

add_executable(hermind-desktop
    src/main.cpp
    src/appwindow.cpp
    src/chatwidget.cpp
    src/messagebubble.cpp
    src/codeblock.cpp
    src/httplib.cpp
    src/sseparser.cpp
    src/shortcutmanager.cpp
    src/trayicon.cpp
    src/dragdrop.cpp
    src/hermindprocess.cpp
    src/configstore.cpp
)

target_link_libraries(hermind-desktop PRIVATE
    Qt6::Core
    Qt6::Gui
    Qt6::Widgets
    Qt6::Network
    qhotkey
)
```

---

## 10. 打包与分发

### 10.1 macOS

使用 `macdeployqt`：

```bash
macdeployqt hermind-desktop.app -qmldir=src
# 将 Go 后端二进制复制到 .app bundle 内
cp hermind-desktop-backend hermind-desktop.app/Contents/MacOS/
```

产物：`hermind-desktop.app`（可拖拽安装到 `/Applications`）。

### 10.2 Windows

使用 `windeployqt` + Inno Setup / NSIS：

```bash
windeployqt hermind-desktop.exe
# 将 Go 后端二进制复制到同级目录
cp hermind-desktop-backend.exe .
# 打包为 installer 或 zip
```

产物：`hermind-desktop.exe`（安装包）或 `.zip`（便携版）。

### 10.3 CI 构建

建议在现有 GitHub Actions 工作流中新增矩阵任务：
- **macOS runner**: 安装 Qt6（`jurplel/install-qt-action`），CMake 构建，打包 .app
- **Windows runner**: 安装 Qt6 + MinGW/MSVC，CMake 构建，`windeployqt` 打包

---

## 11. 测试策略

| 层级 | 方案 |
|------|------|
| Go 后端单元测试 | 复用现有 Go test suite，新增 `desktop` 模式启动测试（验证端口绑定、日志写入） |
| Qt 单元测试 | `Qt Test` 框架，测试 HTTP 客户端、SSE 解析器、配置序列化 |
| 集成测试 | 启动完整桌面应用 + Go 后端，通过自动化发送消息验证端到端流程 |
| 安全测试 | 验证文件拖放路径校验、禁止越界访问用户目录 |

---

## 12. 风险与缓解

| 风险 | 影响 | 缓解措施 |
|------|------|---------|
| Qt Widgets 渲染 Markdown 效果不如 Web | 中 | 后端预渲染 HTML，明确支持/不支持的标签清单，逐步迭代 |
| Qt 6 与 Go HTTP 通信延迟感知 | 低 | 本地 HTTP 延迟 < 1ms，SSE 流式输出保持实时 |
| Windows 上 Qt 部署复杂（DLL 地狱）| 中 | `windeployqt` 自动收集依赖，CI 验证 clean VM 启动 |
| 维护两套 UI（React + Qt）成本高 | 高 | 保持 UI 功能最小化，核心功能对齐；长期可考虑 React 端收敛到管理后台 |
| 再次引发布局/文件安全 bug | 高 | 代码审查聚焦文件路径处理，所有文件操作必须经过用户显式确认 |

---

## 13. 里程碑

| 阶段 | 目标 |
|------|------|
| M1 | Go 后端新增 `hermind desktop` 模式 + `/api/render` 端点 |
| M2 | Qt 桌面应用基础框架：窗口、进程管理、HTTP 客户端、SSE 解析 |
| M3 | 聊天界面：消息气泡、输入栏、会话列表 |
| M4 | Markdown 渲染集成 + 代码高亮 |
| M5 | 系统集成：全局快捷键、托盘通知、文件拖放 |
| M6 | 设置面板、配置同步 |
| M7 | 打包脚本 + CI + 发布 |

---

## 附录：相关文件

- 现有 Go API: `api/server.go`, `api/handlers_*.go`
- 现有前端: `web/src/`, `web/index.html`
- 现有配置: `config/config.go`
