# Hermind Desktop Qt 版逐步实现 Frontend 全部界面 Roadmap

> 生成日期：2026-06-30  
> 基于源码：hermind-desktop/（Qt 桌面程序）与 frontend/src/（React SPA）  
> 原则：深入探索源码后再做决策，所有页面清单、路径、组件均来自实际代码，不凭空想象。
> 最后更新：2026-06-30（已应用 Execution Rubric）

---

## Execution Rubric（执行规则 — 已审批）

本 Roadmap 覆盖 Hermind 桌面端从 3 个基础 Qt Widget 逐步复刻整个 React frontend 的 22 周规划。为避免每个阶段重复判断“拆多细 / 用 design 还是 plan”，先一次性约定执行规则（已审批）。

### A. 子阶段拆分原则

一个阶段必须拆分为子阶段（编号如 4.4.0、4.4.1…）的触发条件：

1. **触及 >8 个文件/模块**：或同时改动共享基础设施（API 客户端、导航框架、主题状态）与具体页面。
2. **混合独立可交付的工作包**：例如一个阶段既包含聊天核心，又包含设置页，应拆开。
3. **存在前后真实依赖**：后项必须等待前项的接口/数据结构/构建产物落地才能开工。
4. **混合多种技术栈/风险等级**：例如 Qt 原生页与 WebView 兜底页、高复杂度视觉组件与简单表单页应分离。

每个子阶段必须**独立可验证**（编译通过、至少有一个验收动作、不依赖后续子阶段才能交付）。

### B. 执行模式判定标准

| 模式 | 适用场景 | 原因 |
|------|---------|------|
| **normal** | 机械、低风险、答案唯一、改动隔离；无共享签名或架构决策。例如：实现单个设置表单页、给已有导航增加一个页面槽位、样式调整。 | normal 可直接编辑代码，无需额外的计划开销。 |
| **plan** | 多步骤实现、存在真实依赖、有共享签名/调用方扇出、或适合先写 TDD 任务清单再动手。例如：新建 `HermindApiClient` 并替换所有页面的占位 `qDebug`、重构 `MainWindow` 导航框架。 | plan 强制先写计划文件和依赖图，禁止直接改代码。 |
| **design** | 存在真正的架构/数据模型/公共接口/迁移语义未知 — 猜错代价高。例如：WebView 与 Qt 原生页之间的统一状态桥接协议、消息 Markdown 渲染方案选型、跨平台打包策略。 | design 必须经用户审批 spec 后才能进入实现，防止未经验证的假设浪费大量工作。 |

**Tie-breaker**：若一个子阶段可落入两种模式，只有当存在真实未知时才选更谨慎的模式；否则优先选更便宜的模式。禁止把常规叶子工作标为 `design`“以防万一”。

### C. 依赖声明规则

- 每个子阶段的 `Depends on:` 必须指向**编号更小的前面子阶段**，并说明依赖的**真实符号/文件/契约**（例如 `hermind-desktop/api_client.h` 的 `HermindApiClient::getWorkspaces()`）。
- 必须显式说明哪些子阶段可**并行执行**；默认不留下“能否同时做？”的疑问。
- 依赖图基于实际源码：`hermind-desktop/` 当前仅 `mainwindow.cpp/.h/.ui`、`main_chat_widget.*`、`main_setting_widget.*`、`main.cpp`、`hermind-desktop.pro`；`frontend/src/main.jsx` 已确认 44 条路由；所有新模块将新建文件，不修改 `frontend/src/`。

---

## 1. 目标与范围

### 1.1 目标
在现有 `hermind-desktop`（Qt Widgets 桌面程序）基础上，逐步实现 `frontend` 的全部界面，使其成为功能完整的 Hermind 桌面客户端。

### 1.2 现状说明
- `hermind-desktop` 当前仅包含 3 个核心文件：`mainwindow`、`main_chat_widget`、`main_setting_widget`。
- 已实现：主窗口 + `QStackedWidget` 双页切换（聊天页 / 设置页），但所有按钮仅打印 `qDebug`，无实际业务逻辑。
- `frontend` 是完整的 React 18 SPA，包含数十个页面、组件、弹窗、数据连接器、Agent Builder 等。
- 后端 `backend` 已提供与前端 100% 兼容的 REST/WebSocket API（见 `AGENTS.md`）。

### 1.3 关键约束
- **API 契约不变**：Qt 桌面端应复用同一套 `/api/*` 接口，不新增后端逻辑。
- **前端零改动**：`frontend/src/` 保持原样，Qt 端自行实现等价界面。
- **构建工具不变**：继续使用 Qt 6 + qmake（`hermind-desktop.pro`）。
- **多平台**：Windows / macOS / Linux 三端均需可编译运行。

---

## 2. 源码盘点

### 2.1 hermind-desktop 当前结构
```
hermind-desktop/
├── main.cpp                  # QApplication 入口
├── mainwindow.h/.cpp/.ui     # 主窗口 + QStackedWidget
├── main_chat_widget.h/.cpp/.ui   # 聊天主界面（左侧边栏 + 右侧会话区）
├── main_setting_widget.h/.cpp/.ui # 设置界面（仅外观设置）
├── hermind-desktop.pro       # Qt 工程文件
└── resources.qrc             # 图标资源
```

当前 `MainChatWidget` 已模拟的控件：
- 左侧：Logo、搜索、工作区标题、会话列表、`+ New Thread`、`Assistant Chats`。
- 右侧：工作区名、版本、欢迎语、消息输入框、工具按钮、麦克风、发送、`创建代理 / 编辑工作区 / 上传文件`。

当前 `MainSettingWidget` 仅实现：
- 左侧 9 个菜单按钮（AI 提供商、管理员、代理技能、会议助理、桌面助手、社区中心、外观、频道、工具）。
- 右侧仅 `外观` 页有内容：默认窗口、主题、显示语言。

### 2.2 frontend 路由与页面全景

路由定义在 `frontend/src/main.jsx`，共 30+ 条路由，可分为 7 大模块：

| 模块 | 路由前缀 | 页面数量 | 说明 |
|------|----------|----------|------|
| 入口 / 认证 | `/`, `/login`, `/sso/simple`, `/onboarding/*`, `/accept-invite/:code` | 5 | 登录、引导、邀请 |
| 主页 / 聊天 | `/`, `/workspace/:slug`, `/workspace/:slug/t/:threadSlug` | 3 | DefaultChat、WorkspaceChat |
| 工作区设置 | `/workspace/:slug/settings/:tab` | 5 Tab | General/Chat/Vector/Members/Agent |
| AI 提供商设置 | `/settings/*` | 7 | LLM、Embedding、Text Splitter、VectorDB、Audio、Transcription、ModelRouters |
| 管理设置 | `/settings/*` | 7 | Users、Workspaces、Invites、Chats、Event Logs、ApiKeys、System Prompt Variables |
| Agent 设置 | `/settings/*` | 4 | Agent Skills、Agent-Created Skills、Agent Builder、Default System Prompt |
| 自定义 / 工具 / 频道 | `/settings/*` | 8 | Interface、Branding、Chat、Security、Privacy、Embed Widgets、Browser Extension、Telegram、Scheduled Jobs |

完整路由清单见 [附录 A：Frontend 路由表](#附录-a-frontend-路由表)。

### 2.3 frontend 核心组件依赖

| 组件域 | 关键文件路径 | Qt 实现复杂度 |
|--------|--------------|---------------|
| 布局 | `components/Sidebar/index.jsx`、`components/SettingsSidebar/index.jsx` | 中 |
| 聊天容器 | `components/WorkspaceChat/ChatContainer/index.jsx` | 高 |
| 消息历史 | `ChatHistory/HistoricalMessage`、`PromptReply` | 高 |
| 输入框 | `PromptInput/index.jsx`（含附件、@Agent、工具菜单、语音） | 高 |
| 工作区管理弹窗 | `Modals/ManageWorkspace/`（Documents / Data Connectors） | 高 |
| 数据连接器 | `DataConnectors/Connectors/*`（GitHub、GitLab、YouTube 等 8 个） | 中 |
| LLM 选择器 | `components/LLMSelection/*`（40+ 提供商配置组件） | 极高 |
| Embedding 选择器 | `components/EmbeddingSelection/*` | 高 |
| VectorDB 选择器 | `components/VectorDBSelection/*`（10 个后端） | 高 |
| Agent Builder | `pages/Admin/AgentBuilder/`（可视化流程块） | 极高 |
| Agent Skills | `pages/Admin/Agents/skills.jsx` + 多个 Skill Panel | 高 |

---

## 3. 技术方案选择

### 方案 A：纯 Qt Widgets 重写（推荐长期目标）
- **做法**：用 `QWidget/QML` 逐一复刻所有页面。
- **优点**：原生体验、离线可用、与系统深度集成、不依赖 Web 运行时。
- **缺点**：开发周期长；Markdown/Agent Builder/LLM 选择器等组件实现成本高。
- **适用**：核心高频页面（聊天、侧边栏、基础设置）。

### 方案 B：QWebEngineView 嵌入 frontend/dist
- **做法**：桌面端直接加载已构建的 `frontend/dist/index.html`，仅保留一个浏览器窗口壳。
- **优点**：一天内即可拥有完整界面；与前端同步零成本。
- **缺点**：失去原生桌面优势；内存占用大；与后端/本地文件系统集成需额外桥接。
- **结论**：可作为快速 MVP，但不符"Qt 版本桌面程序逐步实现 frontend 界面"的要求。

### 方案 C：混合架构（推荐实施策略）
- **做法**：
  1. **核心高频界面用 Qt Widgets 实现**：聊天页、侧边栏、基础设置。
  2. **复杂低频界面用内嵌 WebView 兜底**：Agent Builder、LLM/Embedding/VectorDB 选择器、Data Connectors、Embed Widgets 等。
  3. **统一 API 层**：所有界面共享一个 `HermindApiClient` 与后端通信。
- **优点**：平衡开发成本与原生体验；优先交付核心价值。
- **缺点**：需要维护两套 UI 技术栈的交互边界。

### 推荐决策
**采用方案 C（混合架构），按阶段逐步替换 WebView 兜底页为原生 Qt 页面。**

理由：
1. `frontend` 界面数量巨大（>50 页面/组件），纯 Qt 重写周期不可接受。
2. 聊天页是桌面用户最高频场景，必须原生实现以提供优质体验。
3. Agent Builder、LLM 选择器等在 Web 端已有成熟实现，内嵌复用可降低风险。
4. 后端 API 已稳定，所有界面无论 Qt/Web 均可调用同一接口。

---

## 4. 阶段规划

### 4.x 子阶段总览

| 阶段 | 子阶段 | 任务 | 模式 | 关键输出文件 |
|------|--------|------|------|--------------|
| 0 | 0.0 | `HermindApiClient` 统一 API 客户端 | [plan] | `api_client.{h,cpp}` |
| 0 | 0.1 | `SettingsStore` 本地设置存储 | [normal] | `settings_store.{h,cpp}` |
| 0 | 0.2 | `AuthManager` 认证状态管理 | [plan] | `auth_manager.{h,cpp}` |
| 0 | 0.3 | `ThemeManager` 主题/语言状态 | [normal] | `theme_manager.{h,cpp}` |
| 0 | 0.4 | `NavigationManager` 全局导航框架 | [plan] | `navigation_manager.{h,cpp}` |
| 0 | 0.5 | 通用 UI 组件库 | [normal] | `widgets/*` |
| 1 | 1.0 | Sidebar 原生实现 | [plan] | `sidebar_widget.{h,cpp,ui}` |
| 1 | 1.1 | Workspace / Thread 树 | [normal] | `workspace_thread_model.*`, `thread_item_widget.*` |
| 1 | 1.2 | 搜索框 + 新建工作区弹窗 | [normal] | `search_box_widget.*`, `new_workspace_dialog.*` |
| 1 | 1.3 | 聊天容器与消息历史 | [plan] | `chat_container.{h,cpp,ui}`, `chat_history_widget.*` |
| 1 | 1.4 | Markdown 渲染组件 | [design] | `markdown_text_browser.*` 或 `markdown_document.*` |
| 1 | 1.5 | Prompt 输入框（含附件/DnD） | [plan] | `prompt_input_widget.*` |
| 1 | 1.6 | Agent 调用与工具菜单（@Agent） | [plan] | `agent_tool_menu.*`, `agent_session.*` |
| 1 | 1.7 | 欢迎页 / 快捷操作 / Sources 侧边栏 | [normal] | `default_chat_widget.*`, `sources_sidebar.*` |
| 2 | 2.0 | Workspace Settings 框架 | [plan] | `workspace_settings_widget.{h,cpp,ui}` |
| 2 | 2.1 | General Appearance Tab | [normal] | `general_appearance_tab.*` |
| 2 | 2.2 | Chat Settings Tab | [normal] | `chat_settings_tab.*` |
| 2 | 2.3 | Vector Database Tab | [normal] | `vector_database_tab.*` |
| 2 | 2.4 | Members Tab | [normal] | `members_tab.*` |
| 2 | 2.5 | Agent Config Tab | [normal] | `agent_config_tab.*` |
| 3 | 3.0 | AI Provider Settings 框架 | [plan] | `ai_provider_settings_widget.*` |
| 3 | 3.1 | LLM Preference（WebView 兜底） | [design] | `web_view_llm_preference.*` |
| 3 | 3.2 | Embedding Preference（WebView 兜底） | [plan] | `web_view_embedding_preference.*` |
| 3 | 3.3 | Text Splitter Preference（Qt 原生） | [normal] | `text_splitter_preference_tab.*` |
| 3 | 3.4 | Vector Database（WebView 兜底） | [plan] | `web_view_vector_database.*` |
| 3 | 3.5 | Audio Preference（Qt 原生） | [normal] | `audio_preference_tab.*` |
| 3 | 3.6 | Transcription Preference（Qt 原生） | [normal] | `transcription_preference_tab.*` |
| 3 | 3.7 | Model Routers + Rules（WebView 兜底） | [plan] | `web_view_model_routers.*` |
| 4 | 4.0 | Admin Settings 框架 | [plan] | `admin_settings_widget.*` |
| 4 | 4.1 | Users | [normal] | `users_tab.*` |
| 4 | 4.2 | Workspaces | [normal] | `workspaces_tab.*` |
| 4 | 4.3 | Invitations | [normal] | `invitations_tab.*` |
| 4 | 4.4 | Agent Skills（WebView 兜底） | [plan] | `web_view_agent_skills.*` |
| 4 | 4.5 | Agent-Created Skills（Qt 原生） | [normal] | `agent_created_skills_tab.*` |
| 4 | 4.6 | Agent Builder（WebView 兜底） | [plan] | `web_view_agent_builder.*` |
| 4 | 4.7 | Default System Prompt | [normal] | `default_system_prompt_tab.*` |
| 4 | 4.8 | System Prompt Variables | [normal] | `system_prompt_variables_tab.*` |
| 4 | 4.9 | Event Logs | [normal] | `event_logs_tab.*` |
| 5 | 5.0 | Customization Settings 框架 | [plan] | `customization_settings_widget.*` |
| 5 | 5.1 | Interface | [normal] | `interface_settings_tab.*` |
| 5 | 5.2 | Branding | [normal] | `branding_settings_tab.*` |
| 5 | 5.3 | Chat | [normal] | `chat_settings_tab.*` |
| 5 | 5.4 | Security | [normal] | `security_settings_tab.*` |
| 5 | 5.5 | Privacy & Data | [normal] | `privacy_settings_tab.*` |
| 5 | 5.6 | Chat Embed Widgets（WebView 兜底） | [plan] | `web_view_chat_embed_widgets.*` |
| 5 | 5.7 | Browser Extension API Key | [normal] | `browser_extension_api_key_tab.*` |
| 5 | 5.8 | Telegram Bot（WebView 兜底） | [plan] | `web_view_telegram_bot.*` |
| 5 | 5.9 | Scheduled Jobs | [normal] | `scheduled_jobs_tab.*` |
| 6 | 6.0 | Toast / Modal 包装系统 | [plan] | `toast_manager.*`, `modal_wrapper.*` |
| 6 | 6.1 | Login 页 | [normal] | `login_widget.*` |
| 6 | 6.2 | Onboarding 引导 | [normal] | `onboarding_widget.*` |
| 6 | 6.3 | Invite 接受页 | [normal] | `invite_widget.*` |
| 6 | 6.4 | 新建 / 编辑工作区弹窗 | [normal] | `new_workspace_dialog.*`, `manage_workspace_dialog.*` |
| 6 | 6.5 | 文件 / 数据连接器弹窗（WebView 兜底） | [plan] | `web_view_data_connector.*` |
| 6 | 6.6 | 404 页 | [normal] | `not_found_widget.*` |
| 7 | 7.0 | 长消息列表虚拟滚动 / 分页 | [plan] | `virtual_chat_history_view.*` |
| 7 | 7.1 | 图片 / 文件缓存与懒加载 | [normal] | `file_cache_manager.*` |
| 7 | 7.2 | 错误边界与崩溃恢复 | [normal] | `crash_handler.*` |
| 7 | 7.3 | 键盘快捷键 | [normal] | `shortcut_manager.*` |
| 7 | 7.4 | 多语言 i18n 框架对接 | [plan] | `i18n_manager.*` |
| 7 | 7.5 | 跨平台打包验证 | [normal] | `hermind-desktop.pro` / CI 脚本 |

### 阶段 0：基础设施（2 周）

**目标**：建立 Qt 端与后端的通信、状态、导航基础，支持后续页面快速开发。

| 子阶段 | 任务 | 输出文件 | 模式 | 依赖 |
|--------|------|----------|------|------|
| 0.0 | `HermindApiClient` 统一 API 客户端 | `api_client.{h,cpp}` | [plan] | 无 |
| 0.1 | `SettingsStore` 本地设置存储 | `settings_store.{h,cpp}` | [normal] | 无 |
| 0.2 | `AuthManager` 认证状态管理 | `auth_manager.{h,cpp}` | [plan] | 0.0, 0.1 |
| 0.3 | `ThemeManager` 主题/语言状态 | `theme_manager.{h,cpp}` | [normal] | 0.1 |
| 0.4 | `NavigationManager` 全局导航框架 | `navigation_manager.{h,cpp}` | [plan] | 0.0 |
| 0.5 | 通用 UI 组件库 | `widgets/*` | [normal] | 0.3 |

**依赖图**（阶段 0 内部）：
```
0.0 HermindApiClient       0.1 SettingsStore
   |                            |
   |----> 0.2 AuthManager <-----|
   |                            |
   |----> 0.4 NavigationManager |
   |                            |
   +----> (all later stages)    +----> 0.3 ThemeManager
                                         |
                                         +----> 0.5 WidgetLibrary
```

**可并行**：0.0 与 0.1 可并行启动；0.3 与 0.4 可并行（均依赖 0.1/0.0 但互不依赖）。

**模式说明**：
- 0.0 `[plan]`：需要一次性定义 REST/SSE/WebSocket 的统一接口、错误处理、认证头注入；后续所有页面共享此契约。
- 0.1 `[normal]`：单一的 `QSettings` 包装，接口明确，无架构未知。
- 0.2 `[plan]`：依赖 0.0/0.1，需定义 JWT/密码弹窗/多用户模式的公共状态接口，被导航和聊天页共同调用。
- 0.3 `[normal]`：读取/写入 `QSettings` 的主题与语言键，接口简单。
- 0.4 `[plan]`：重构现有 `MainWindow` 的 `QStackedWidget`（当前仅 2 页切换）为多页面栈和历史返回；影响所有页面注册方式。
- 0.5 `[normal]`：基于 0.3 主题状态实现原子控件，无共享架构决策。

**验收标准**：
- Qt 客户端能登录并拉取当前用户与工作区列表（验证 0.0/0.2）。
- 主题/语言切换即时生效（验证 0.1/0.3/0.5）。
- 能在聊天页与设置页之间切换，并在内存中保持状态（验证 0.4）。

### 阶段 1：聊天核心界面（4 周）

**目标**：实现 `frontend` 中 `/workspace/:slug` 与 `/` 的核心聊天体验。

| 子阶段 | 任务 | 对应 frontend 文件 | 模式 | 依赖 |
|--------|------|--------------------|------|------|
| 1.0 | Sidebar 原生实现 | `components/Sidebar/index.jsx` | [plan] | 0.0, 0.4, 0.5 |
| 1.1 | Workspace / Thread 树 | `ActiveWorkspaces/ThreadContainer/ThreadItem` | [normal] | 1.0 |
| 1.2 | 搜索框 + 新建工作区弹窗 | `SearchBox/index.jsx`, `Modals/NewWorkspace.jsx` | [normal] | 1.0, 0.0 |
| 1.3 | 聊天容器与消息历史 | `WorkspaceChat/ChatContainer/index.jsx`, `ChatHistory/*` | [plan] | 0.0, 0.4, 0.5 |
| 1.4 | Markdown 渲染组件 | 引入 `QTextDocument`/Markdown 库 | [design] | 1.3 |
| 1.5 | Prompt 输入框（含附件/DnD） | `PromptInput/index.jsx`, `PromptInput/Attachments`, `DnDWrapper` | [plan] | 1.3, 0.5 |
| 1.6 | Agent 调用与工具菜单（@Agent） | `PromptInput/ToolsMenu` | [plan] | 1.3, 0.0 |
| 1.7 | 欢迎页 / 快捷操作 / Sources 侧边栏 | `DefaultChat/index.jsx`, `SourcesSidebar`, `MemoriesSidebar` | [normal] | 1.0, 1.3 |

**依赖图**（阶段 1 内部）：
```
1.0 Sidebar
   |
   +----> 1.1 Thread Tree
   +----> 1.2 Search + New Workspace
   +----> 1.7 Welcome / Sources
   |
1.3 Chat Container
   |
   +----> 1.4 Markdown Rendering
   +----> 1.5 Prompt Input (Attachments/DnD)
   +----> 1.6 Agent Tools / @Agent
   +----> 1.7 Welcome / Sources
```

**可并行**：1.1/1.2 与 1.3 主干可并行开发（均需 1.0 与阶段 0 基础）；1.4/1.5/1.6/1.7 均依赖 1.3，可在 1.3 接口确定后并行。

**跨阶段依赖**：整阶段依赖阶段 0（0.0 API、0.4 导航、0.5 控件）；1.6 额外依赖后端 Agent WebSocket 契约（已存在，无需新增后端代码）。

**范围调整（2026-07-13）**：`SpeechToText`（语音输入）从 1.6 移出，标记为**后续阶段**；阶段 1 验收不再要求语音输入。

**模式说明**：
- 1.0 `[plan]`：Sidebar 是聊天页与工作区导航的共享入口，需定义 `WorkspaceModel` 数据类与 `NavigationManager` 的集成契约。
- 1.1 `[normal]`：Thread 列表项是 Sidebar 内部的叶子控件，复用 1.0 的数据模型。
- 1.2 `[normal]`：搜索过滤与新建工作区弹窗调用已有 `HermindApiClient::createWorkspace()`，隔离改动。
- 1.3 `[plan]`：聊天容器是消息历史、输入框、停止生成等多组件的父容器，需先定义消息数据结构与滚动接口。
- 1.4 `[design]`：存在真实架构选择（`md4qt` vs `cmark` vs 自绘 `QTextDocument`），渲染一致性和代码块高亮方案需先定型。
- 1.5 `[plan]`：输入框同时涉及文本编辑、附件上传、跨平台 DnD，需要统一的事件与文件 API 接口。
- 1.6 `[plan]`：@Agent / 工具菜单依赖 Agent WebSocket 协议解析（`frontend/src/utils/chat/agent.js` 逻辑），需要先把协议映射到 Qt。
- 1.7 `[normal]`：欢迎页与 Sources 侧边栏是聊天容器的叶子补充，无新增架构决策。

**验收标准**：
- 用户可选择一个工作区/线程进行聊天。
- 支持普通流式回复与 Agent WebSocket 会话。
- 支持文件上传、引用来源展示、重新生成、复制消息。
- 消息 Markdown（代码块、列表、链接）可正常渲染。

### 阶段 2：工作区设置（2 周）

**目标**：实现 `/workspace/:slug/settings/:tab` 的 5 个 Tab。

| 子阶段 | Tab | 对应文件 | 模式 | 依赖 |
|--------|-----|----------|------|------|
| 2.0 | Workspace Settings 框架 | `pages/WorkspaceSettings/index.jsx` | [plan] | 0.0, 0.4, 0.5 |
| 2.1 | General Appearance | `WorkspaceSettings/GeneralAppearance/*` | [normal] | 2.0 |
| 2.2 | Chat Settings | `WorkspaceSettings/ChatSettings/*` | [normal] | 2.0 |
| 2.3 | Vector Database | `WorkspaceSettings/VectorDatabase/*` | [normal] | 2.0 |
| 2.4 | Members | `WorkspaceSettings/Members/*` | [normal] | 2.0 |
| 2.5 | Agent Config | `WorkspaceSettings/AgentConfig/*` | [normal] | 2.0 |

**依赖图**（阶段 2 内部）：
```
2.0 Workspace Settings Frame
   |
   +----> 2.1 General Appearance
   +----> 2.2 Chat Settings
   +----> 2.3 Vector Database
   +----> 2.4 Members
   +----> 2.5 Agent Config
```

**可并行**：2.1–2.5 在 2.0 的 Tab 接口与路由注册完成后可完全并行开发。

**跨阶段依赖**：依赖阶段 0 的 `NavigationManager`（从聊天页进入设置页）和 `HermindApiClient`；不依赖阶段 1 的具体实现。

**模式说明**：
- 2.0 `[plan]`：需要把 `/workspace/:slug/settings/:tab` 路由映射到二级 `QStackedWidget`，并定义 Tab 注册接口供 2.1–2.5 使用。
- 2.1–2.5 `[normal]`：均为表单/列表页，使用 2.0 框架与阶段 0 的 API 客户端，无共享架构决策。

**验收标准**：
- 5 个 Tab 可切换并保存配置。
- 工作区名、建议消息、删除工作区可用。
- 成员管理（添加/移除）可用。

### 阶段 3：全局设置 - AI 提供商（3 周）

**目标**：实现 LLM、Embedding、VectorDB、Text Splitter、Audio、Transcription、Model Routers。

| 子阶段 | 页面 | 对应文件 | 策略 | 模式 | 依赖 |
|--------|------|----------|------|------|------|
| 3.0 | AI Provider Settings 框架 | `pages/GeneralSettings/*` 公共布局 | 原生框架 | [plan] | 0.0, 0.4, 0.5 |
| 3.1 | LLM Preference | `GeneralSettings/LLMPreference/index.jsx` | WebView 兜底 | [design] | 3.0 |
| 3.2 | Embedding Preference | `GeneralSettings/EmbeddingPreference/index.jsx` | WebView 兜底 | [plan] | 3.0 |
| 3.3 | Text Splitter Preference | `GeneralSettings/EmbeddingTextSplitterPreference/index.jsx` | Qt 原生 | [normal] | 3.0 |
| 3.4 | Vector Database | `GeneralSettings/VectorDatabase/index.jsx` | WebView 兜底 | [plan] | 3.0 |
| 3.5 | Audio Preference | `GeneralSettings/AudioPreference/*` | Qt 原生 | [normal] | 3.0 |
| 3.6 | Transcription Preference | `GeneralSettings/TranscriptionPreference/index.jsx` | Qt 原生 | [normal] | 3.0 |
| 3.7 | Model Routers + Rules | `GeneralSettings/ModelRouters/*` | WebView 兜底 | [plan] | 3.0 |

**依赖图**（阶段 3 内部）：
```
3.0 AI Provider Settings Frame
   |
   +----> 3.1 LLM Preference (WebView)
   +----> 3.2 Embedding Preference (WebView)
   +----> 3.3 Text Splitter (Native)
   +----> 3.4 Vector Database (WebView)
   +----> 3.5 Audio Preference (Native)
   +----> 3.6 Transcription Preference (Native)
   +----> 3.7 Model Routers (WebView)
```

**可并行**：3.1–3.7 在 3.0 的框架与 WebView 桥接基类完成后可并行；原生页与 WebView 页之间互不阻塞。

**跨阶段依赖**：依赖阶段 0 的 `NavigationManager`（从设置侧边栏切换）和 `HermindApiClient`；3.1/3.2/3.4/3.7 还依赖 WebView 状态桥接（由 3.0 统一提供）。

**模式说明**：
- 3.0 `[plan]`：需为设置页左侧菜单与右侧内容定义二级导航框架，并封装 WebView 页与原生页的统一加载接口。
- 3.1 `[design]`：首个 WebView 兜底页，需确定 `QWebChannel` 桥接协议、`window.hermindQt` 对象、本地文件选择等集成方案；方案确定后 3.2/3.4/3.7 可复用。
- 3.2/3.4/3.7 `[plan]`：复用 3.1 的 WebView 基类，但每个页面有独立的 URL 路由与后端保存逻辑，需单独计划。
- 3.3/3.5/3.6 `[normal]`：纯表单页，使用 3.0 框架与 `HermindApiClient`。

**验收标准**：
- 原生页面可独立配置并保存。
- WebView 兜底页能正确加载对应 `/settings/*` URL 并与后端交互。
- 切换 VectorDB/Embedding 时的变更警告弹窗正常。

### 阶段 4：全局设置 - 管理与 Agent（4 周）

**目标**：实现用户、工作区、邀请、Agent Skills、Agent Builder、System Prompt 等。

| 子阶段 | 页面 | 对应文件 | 策略 | 模式 | 依赖 |
|--------|------|----------|------|------|------|
| 4.0 | Admin Settings 框架 | `pages/Admin/*` 公共布局 | 原生框架 | [plan] | 0.0, 0.4, 0.5 |
| 4.1 | Users | `Admin/Users/*` | Qt 原生 | [normal] | 4.0 |
| 4.2 | Workspaces | `Admin/Workspaces/*` | Qt 原生 | [normal] | 4.0 |
| 4.3 | Invitations | `Admin/Invitations/*` | Qt 原生 | [normal] | 4.0 |
| 4.4 | Agent Skills | `Admin/Agents/*` | WebView 兜底 | [plan] | 4.0, 3.1 |
| 4.5 | Agent-Created Skills | `Admin/AgentCreatedSkillsPage/*` | Qt 原生 | [normal] | 4.0 |
| 4.6 | Agent Builder | `Admin/AgentBuilder/*` | WebView 兜底 | [plan] | 4.0, 3.1 |
| 4.7 | Default System Prompt | `Admin/DefaultSystemPrompt/*` | Qt 原生 | [normal] | 4.0 |
| 4.8 | System Prompt Variables | `Admin/SystemPromptVariables/*` | Qt 原生 | [normal] | 4.0 |
| 4.9 | Event Logs | `Admin/Logging/*` | Qt 原生 | [normal] | 4.0 |

**依赖图**（阶段 4 内部）：
```
4.0 Admin Settings Frame
   |
   +----> 4.1 Users
   +----> 4.2 Workspaces
   +----> 4.3 Invitations
   +----> 4.4 Agent Skills (WebView)
   +----> 4.5 Agent-Created Skills (Native)
   +----> 4.6 Agent Builder (WebView)
   +----> 4.7 Default System Prompt
   +----> 4.8 System Prompt Variables
   +----> 4.9 Event Logs
```

**可并行**：4.1–4.9 在 4.0 框架完成后可完全并行；4.4/4.6 还需等待 3.1 的 WebView 桥接基类落地。

**跨阶段依赖**：依赖阶段 0；4.4/4.6 额外依赖阶段 3.1 的 WebView 桥接方案。

**模式说明**：
- 4.0 `[plan]`：与 3.0 类似的二级设置框架，但面向 Admin 菜单；需复用/扩展 `NavigationManager`。
- 4.1/4.2/4.3/4.5/4.7/4.8/4.9 `[normal]`：表格/表单 CRUD 页，逻辑直接。
- 4.4 `[plan]`：复用 3.1 WebView 桥接，但 Skill Panel 与后端保存逻辑较复杂，需计划。
- 4.6 `[plan]`：Agent Builder 是可视化流程图，WebView 加载与流程 ID 路由处理需要独立验证。

**验收标准**：
- 用户/工作区/邀请的 CRUD 可用。
- Agent Skills / Agent Builder 通过 WebView 正常运作。
- System Prompt Variables 可增删改查。

### 阶段 5：全局设置 - 自定义、工具、频道（3 周）

**目标**：实现 Interface、Branding、Chat、Security、Privacy、Embed Widgets、Browser Extension、Telegram、Scheduled Jobs。

| 子阶段 | 页面 | 对应文件 | 策略 | 模式 | 依赖 |
|--------|------|----------|------|------|------|
| 5.0 | Customization Settings 框架 | `pages/GeneralSettings/Settings/*` 公共布局 | 原生框架 | [plan] | 0.0, 0.4, 0.5 |
| 5.1 | Interface | `GeneralSettings/Settings/Interface/*` | Qt 原生 | [normal] | 5.0 |
| 5.2 | Branding | `GeneralSettings/Settings/Branding/*` | Qt 原生 | [normal] | 5.0 |
| 5.3 | Chat | `GeneralSettings/Settings/Chat/*` | Qt 原生 | [normal] | 5.0 |
| 5.4 | Security | `GeneralSettings/Security/*` | Qt 原生 | [normal] | 5.0 |
| 5.5 | Privacy & Data | `GeneralSettings/PrivacyAndData/*` | Qt 原生 | [normal] | 5.0 |
| 5.6 | Chat Embed Widgets | `GeneralSettings/ChatEmbedWidgets/*` | WebView 兜底 | [plan] | 5.0, 3.1 |
| 5.7 | Browser Extension API Key | `GeneralSettings/BrowserExtensionApiKey/*` | Qt 原生 | [normal] | 5.0 |
| 5.8 | Telegram Bot | `GeneralSettings/Connections/TelegramBot/*` | WebView 兜底 | [plan] | 5.0, 3.1 |
| 5.9 | Scheduled Jobs | `GeneralSettings/ScheduledJobs/*` | Qt 原生 | [normal] | 5.0 |

**依赖图**（阶段 5 内部）：
```
5.0 Customization Settings Frame
   |
   +----> 5.1 Interface
   +----> 5.2 Branding
   +----> 5.3 Chat
   +----> 5.4 Security
   +----> 5.5 Privacy & Data
   +----> 5.6 Chat Embed Widgets (WebView)
   +----> 5.7 Browser Extension API Key
   +----> 5.8 Telegram Bot (WebView)
   +----> 5.9 Scheduled Jobs
```

**可并行**：5.1–5.9 在 5.0 框架完成后可并行；5.6/5.8 还需 3.1 WebView 桥接基类。

**跨阶段依赖**：依赖阶段 0；5.6/5.8 依赖阶段 3.1。

**模式说明**：
- 5.0 `[plan]`：设置页左侧菜单分组与右侧内容框架，需与 3.0/4.0 共享路由注册机制。
- 5.1–5.5/5.7/5.9 `[normal]`：常规设置表单页。
- 5.6/5.8 `[plan]`：复用 3.1 WebView 桥接，但每个页面有独立 URL 与后端交互。

**验收标准**：
- 原生设置页可保存并即时生效。
- WebView 页能正确加载并调用后端。

### 阶段 6：弹窗与辅助界面（2 周）

**目标**：补全全局弹窗、登录、引导、404 等。

| 子阶段 | 任务 | 对应文件 | 策略 | 模式 | 依赖 |
|--------|------|----------|------|------|------|
| 6.0 | Toast / Modal 包装系统 | `utils/toast`, `ModalWrapper` | Qt 原生 | [plan] | 0.5 |
| 6.1 | Login 页 | `pages/Login/index.jsx`, `components/Modals/Password` | Qt 原生 | [normal] | 0.2, 6.0 |
| 6.2 | Onboarding 引导 | `pages/OnboardingFlow/*` | Qt 原生或 WebView | [normal] | 0.2, 6.0 |
| 6.3 | Invite 接受页 | `pages/Invite/index.jsx` | Qt 原生 | [normal] | 0.0, 6.0 |
| 6.4 | 新建 / 编辑工作区弹窗 | `Modals/NewWorkspace.jsx`, `ManageWorkspace/index.jsx` | Qt 原生 | [normal] | 1.0, 6.0 |
| 6.5 | 文件 / 数据连接器弹窗 | `ManageWorkspace/Documents/*`, `DataConnectors/*` | WebView 兜底 | [plan] | 3.1, 6.0 |
| 6.6 | 404 页 | `pages/404.jsx` | Qt 原生 | [normal] | 0.4 |

**依赖图**（阶段 6 内部）：
```
6.0 Toast / Modal System
   |
   +----> 6.1 Login
   +----> 6.2 Onboarding
   +----> 6.3 Invite
   +----> 6.4 New/Edit Workspace Modal
   +----> 6.5 Data Connector Modal (WebView)
   |
6.6 404 Page
   |
   (depends on NavigationManager 0.4)
```

**可并行**：6.1–6.5 在 6.0 完成后可并行；6.6 仅依赖 0.4 导航框架，可与 6.0 并行。

**跨阶段依赖**：6.1/6.2 依赖 0.2 `AuthManager`；6.3 依赖 0.0 API；6.4 依赖 1.0 Sidebar 的数据模型；6.5 依赖 3.1 WebView 桥接。

**模式说明**：
- 6.0 `[plan]`：全局 Toast / Modal 是跨所有页面的共享基础设施，需先定义统一调用接口（类似 `frontend/src/utils/toast`）。
- 6.1/6.2/6.3/6.4/6.6 `[normal]`：独立页面/弹窗，复用已有管理器。
- 6.5 `[plan]`：数据连接器弹窗依赖 WebView 加载 `ManageWorkspace` 与 `DataConnectors`，需复用 3.1 桥接并处理文件选择。

### 阶段 7：性能优化与收尾（2 周）

**目标**：长消息虚拟滚动、缓存、错误恢复、快捷键、i18n、跨平台打包。

| 子阶段 | 任务 | 输出文件 | 模式 | 依赖 |
|--------|------|----------|------|------|
| 7.0 | 长消息列表虚拟滚动 / 分页 | `virtual_chat_history_view.*` | [plan] | 1.3 |
| 7.1 | 图片 / 文件缓存与懒加载 | `file_cache_manager.*` | [normal] | 1.3, 1.5 |
| 7.2 | 错误边界与崩溃恢复 | `crash_handler.*` | [normal] | 0.4 |
| 7.3 | 键盘快捷键 | `shortcut_manager.*` | [normal] | 0.4, 1.3 |
| 7.4 | 多语言 i18n 框架对接 | `i18n_manager.*` | [plan] | 0.3, 0.5 |
| 7.5 | 跨平台打包验证 | `hermind-desktop.pro` / CI 脚本 | [normal] | 全部 |

**依赖图**（阶段 7 内部）：
```
7.0 Virtual Scrolling      7.1 File Cache        7.2 Crash Handler
   |                            |                       |
   +----> 7.5 Packaging <-------+-----------------------+
   |                            |                       |
7.3 Shortcuts                7.4 i18n
   |                            |
   +----> 7.5 Packaging <-------+
```

**可并行**：7.0–7.4 可并行启动（依赖均已在前序阶段落地）；7.5 必须等待全部功能合入后执行。

**模式说明**：
- 7.0 `[plan]`：虚拟滚动需要替换 1.3 的 `chat_history_widget` 列表实现，涉及数据模型与视图的解耦。
- 7.1 `[normal]`：在 1.3/1.5 现有文件加载路径上增加缓存层。
- 7.2 `[normal]`：在 `NavigationManager` 顶部增加异常捕获与恢复。
- 7.3 `[normal]`：基于 0.4/1.3 注册全局快捷键表。
- 7.4 `[plan]`：需统一所有 Qt 控件的翻译加载方式，并决定运行时语言切换策略。
- 7.5 `[normal]`：纯验证任务，依赖全部代码就位。

---

## 5. 界面实现优先级矩阵

| 优先级 | 界面 | 原因 |
|--------|------|------|
| P0 | 聊天页、Sidebar、登录/密码弹窗 | 用户核心路径，必须原生 |
| P1 | 工作区设置 5 Tab、Users/Workspaces/Invitations | 管理高频功能 |
| P2 | Interface/Security/Chat/Privacy/Branding | 全局设置基础 |
| P3 | Agent Skills / Agent Builder / LLM / Embedding / VectorDB | 复杂但低频，可 WebView 兜底 |
| P4 | Data Connectors / Embed Widgets / Telegram / Scheduled Jobs | 长尾功能 |

---

## 6. 关键实现建议

### 6.1 页面导航模型
- 用 `QStackedWidget` 作为主容器，每个页面对应一个索引。
- 对设置类页面引入二级 `QStackedWidget`（左侧菜单 + 右侧内容）。
- 维护一个页面历史栈，支持返回上一级（模拟浏览器 history）。

### 6.2 网络层
- 封装 `HermindApiClient`：
  - REST：`GET/POST/DELETE` 到 `http://localhost:3001/api/*`。
  - SSE：用于流式聊天响应。
  - WebSocket：用于 Agent 会话（`/api/agent-invocation/${socketId}`）。
- 所有请求自动附加 `Authorization` Cookie / Header。

### 6.3 Markdown 渲染
- 方案 1：使用 `QTextDocument` + `QTextBrowser` 配合自定义 `QSyntaxHighlighter`。
- 方案 2：引入第三方库 `md4qt` 或 `cmark` 渲染为 HTML 后通过 `QTextBrowser::setHtml` 展示。
- 代码块建议集成 `QScintilla` 或自绘行号背景。

### 6.4 WebView 兜底策略
- 使用 `QWebEngineView` 加载本地构建的 `frontend/dist/index.html`。
- 通过 `QWebChannel` 注入 `window.hermindQt` 桥接对象，用于：
  - 通知 Qt 端导航切换（如从 WebView 回到原生聊天页）。
  - 共享登录态、主题、语言。
  - 本地文件选择（WebView 中 `<input type=file>` 受限时）。

### 6.5 数据模型映射
Qt 端应维护与 `frontend/src/models/*` 等价的数据类：
- `WorkspaceModel`（对应 `models/workspace.js`）
- `WorkspaceThreadModel`
- `SystemModel`
- `AdminModel`
- `AgentFlowsModel`
- `AgentSkillsModel`

---

## 7. 风险与应对

| 风险 | 影响 | 应对 |
|------|------|------|
| 纯 Qt 重写周期过长 | 高 | 采用混合架构，复杂页 WebView 兜底 |
| Markdown / 代码块渲染不一致 | 中 | 使用成熟 Markdown 库，增加测试覆盖 |
| WebView 与原生页面状态不同步 | 中 | 通过 QWebChannel 统一状态管理 |
| Agent WebSocket 消息复杂 | 高 | 复用 `frontend/src/utils/chat/agent.js` 协议解析逻辑 |
| 文件拖拽/上传跨平台差异 | 中 | 使用 Qt 原生 QFileDialog + DnD，按 MIME 过滤 |
| 后端 API 未来变更 | 中 | 所有 API 调用集中在 `HermindApiClient`，便于统一适配 |
| Qt 6 WebEngine 打包体积大 | 中 | 仅在 WebView 兜底方案中使用；长期逐步替换为原生 |

---

## 8. 里程碑与交付物

| 里程碑 | 预计时间 | 对应子阶段 | 交付物 |
|--------|----------|------------|--------|
| M0 | 2 周后 | 0.0–0.5 | API/认证/导航/主题基础可用 |
| M1 | 6 周后 | 1.0–1.7 | 聊天核心可用（消息、上传、流式、Agent） |
| M2 | 8 周后 | 2.0–2.5 | 工作区设置完成 |
| M3 | 11 周后 | 3.0–3.7 | AI 提供商设置完成（含 WebView 兜底） |
| M4 | 15 周后 | 4.0–4.9 | 管理与 Agent 设置完成 |
| M5 | 18 周后 | 5.0–5.9 | 自定义/工具/频道设置完成 |
| M6 | 20 周后 | 6.0–6.6 | 弹窗/登录/引导/404 补全 |
| M7 | 22 周后 | 7.0–7.5 | 性能优化、多平台打包、RC 发布 |

---

## 9. 下一步行动

1. **启动阶段 0.0**：以 `HermindApiClient` 为首个 [plan] 任务，输出 `api_client.{h,cpp}`。
2. **并行启动 0.1**：`SettingsStore` 作为 [normal] 任务可与 0.0 并行。
3. **进入阶段 1.4 前先做设计决策**：Markdown 渲染方案（`md4qt`/`cmark`/`QTextDocument`）必须在 1.3 接口确定前定型。
4. **进入阶段 3.1 前先做设计决策**：WebView 桥接协议（`QWebChannel` + `window.hermindQt`）必须在首个 WebView 兜底页实现前经用户审批。
5. **按子阶段索引表跟踪进度**：每个子阶段完成后勾选对应行，确保无静默漏项。

---

## 附录 A：Frontend 路由表

来源：`frontend/src/main.jsx` + `frontend/src/utils/paths.js`

| # | 路由 | 页面组件 | 权限 | 策略建议 |
|---|------|----------|------|----------|
| 1 | `/` | `Main` / `DefaultChat` | PrivateRoute | Qt 原生 |
| 2 | `/login` | `Login` | 公开 | Qt 原生 |
| 3 | `/sso/simple` | `SimpleSSOPassthrough` | 公开 | Qt 原生 |
| 4 | `/workspace/:slug` | `WorkspaceChat` | PrivateRoute | Qt 原生 |
| 5 | `/workspace/:slug/t/:threadSlug` | `WorkspaceChat` | PrivateRoute | Qt 原生 |
| 6 | `/workspace/:slug/settings/general-appearance` | `WorkspaceSettings` + `GeneralAppearance` | ManagerRoute | Qt 原生 |
| 7 | `/workspace/:slug/settings/chat-settings` | `WorkspaceSettings` + `ChatSettings` | ManagerRoute | Qt 原生 |
| 8 | `/workspace/:slug/settings/vector-database` | `WorkspaceSettings` + `VectorDatabase` | ManagerRoute | Qt 原生 |
| 9 | `/workspace/:slug/settings/members` | `WorkspaceSettings` + `Members` | ManagerRoute | Qt 原生 |
| 10 | `/workspace/:slug/settings/agent-config` | `WorkspaceSettings` + `AgentConfig` | ManagerRoute | Qt 原生 |
| 11 | `/accept-invite/:code` | `InvitePage` | 公开 | Qt 原生 |
| 12 | `/onboarding` | `OnboardingFlow` | 公开 | Qt 原生 / WebView |
| 13 | `/onboarding/:step` | `OnboardingFlow` | 公开 | Qt 原生 / WebView |
| 14 | `/settings/llm-preference` | `GeneralLLMPreference` | AdminRoute | WebView 兜底 |
| 15 | `/settings/transcription-preference` | `GeneralTranscriptionPreference` | AdminRoute | Qt 原生 |
| 16 | `/settings/audio-preference` | `GeneralAudioPreference` | AdminRoute | Qt 原生 |
| 17 | `/settings/embedding-preference` | `GeneralEmbeddingPreference` | AdminRoute | WebView 兜底 |
| 18 | `/settings/text-splitter-preference` | `EmbeddingTextSplitterPreference` | AdminRoute | Qt 原生 |
| 19 | `/settings/vector-database` | `GeneralVectorDatabase` | AdminRoute | WebView 兜底 |
| 20 | `/settings/agents` | `AdminAgents` | AdminRoute | WebView 兜底 |
| 21 | `/settings/agent-created-skills` | `AgentCreatedSkillsPage` | AdminRoute | Qt 原生 |
| 22 | `/settings/agents/builder` | `AgentBuilder` | AdminRoute | WebView 兜底 |
| 23 | `/settings/agents/builder/:flowId` | `AgentBuilder` | AdminRoute | WebView 兜底 |
| 24 | `/settings/event-logs` | `AdminLogs` | AdminRoute | Qt 原生 |
| 25 | `/settings/embed-chat-widgets` | `ChatEmbedWidgets` | AdminRoute | WebView 兜底 |
| 26 | `/settings/security` | `GeneralSecurity` | ManagerRoute | Qt 原生 |
| 27 | `/settings/privacy` | `PrivacyAndData` | AdminRoute | Qt 原生 |
| 28 | `/settings/interface` | `InterfaceSettings` | ManagerRoute | Qt 原生 |
| 29 | `/settings/branding` | `BrandingSettings` | ManagerRoute | Qt 原生 |
| 30 | `/settings/default-system-prompt` | `DefaultSystemPrompt` | AdminRoute | Qt 原生 |
| 31 | `/settings/chat` | `ChatSettings` | ManagerRoute | Qt 原生 |
| 32 | `/settings/api-keys` | `GeneralApiKeys` | AdminRoute | Qt 原生 |
| 33 | `/settings/model-routers` | `ModelRouters` | AdminRoute | WebView 兜底 |
| 34 | `/settings/model-routers/:id` | `RouterRulesPage` | AdminRoute | WebView 兜底 |
| 35 | `/settings/system-prompt-variables` | `SystemPromptVariables` | AdminRoute | Qt 原生 |
| 36 | `/settings/browser-extension` | `GeneralBrowserExtension` | ManagerRoute | Qt 原生 |
| 37 | `/settings/workspace-chats` | `GeneralChats` | ManagerRoute | Qt 原生 |
| 38 | `/settings/invites` | `AdminInvites` | ManagerRoute | Qt 原生 |
| 39 | `/settings/users` | `AdminUsers` | ManagerRoute | Qt 原生 |
| 40 | `/settings/workspaces` | `AdminWorkspaces` | ManagerRoute | Qt 原生 |
| 41 | `/settings/external-connections/telegram` | `TelegramBotSettings` | AdminRoute | WebView 兜底 |
| 42 | `/settings/scheduled-jobs` | `ScheduledJobs` | SingleUserRoute | Qt 原生 |
| 43 | `/settings/scheduled-jobs/:id/runs` | `ScheduledJobRuns` | SingleUserRoute | Qt 原生 |
| 44 | `/settings/scheduled-jobs/:id/runs/:runId` | `ScheduledJobRunDetail` | SingleUserRoute | Qt 原生 |
| 45 | `*` | `NotFound` | 公开 | Qt 原生 |

---

## 附录 B：关键源码文件索引

### hermind-desktop
- `hermind-desktop/mainwindow.h`, `mainwindow.cpp`, `mainwindow.ui`
- `hermind-desktop/main_chat_widget.h`, `main_chat_widget.cpp`, `main_chat_widget.ui`
- `hermind-desktop/main_setting_widget.h`, `main_setting_widget.cpp`, `main_setting_widget.ui`
- `hermind-desktop/hermind-desktop.pro`

### frontend 路由与入口
- `frontend/src/main.jsx`
- `frontend/src/App.jsx`
- `frontend/src/utils/paths.js`

### frontend 聊天核心
- `frontend/src/pages/Main/index.jsx`
- `frontend/src/pages/WorkspaceChat/index.jsx`
- `frontend/src/components/DefaultChat/index.jsx`
- `frontend/src/components/Sidebar/index.jsx`
- `frontend/src/components/WorkspaceChat/ChatContainer/index.jsx`
- `frontend/src/components/WorkspaceChat/ChatContainer/PromptInput/index.jsx`
- `frontend/src/components/WorkspaceChat/ChatContainer/ChatHistory/index.jsx`
- `frontend/src/components/Modals/ManageWorkspace/index.jsx`

### frontend 设置
- `frontend/src/components/SettingsSidebar/index.jsx`
- `frontend/src/pages/WorkspaceSettings/index.jsx`
- `frontend/src/pages/GeneralSettings/LLMPreference/index.jsx`
- `frontend/src/pages/GeneralSettings/VectorDatabase/index.jsx`
- `frontend/src/pages/GeneralSettings/Settings/Interface/index.jsx`
- `frontend/src/pages/Admin/Agents/index.jsx`
- `frontend/src/pages/Admin/AgentBuilder/index.jsx`
- `frontend/src/pages/Admin/Workspaces/index.jsx`
- `frontend/src/pages/Admin/Users/index.jsx`
- `frontend/src/pages/Admin/Invitations/index.jsx`

---

*本 Roadmap 基于 2026-06-30 的源码快照生成，后续如 frontend 或 hermind-desktop 结构发生重大变化，应同步更新。*
