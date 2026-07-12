# Hermind Desktop 阶段 1.3 — 聊天容器与消息历史实现计划

**Goal:** 在 `hermind-desktop` 中实现 `frontend` 的 `WorkspaceChat/ChatContainer` 与 `ChatHistory` 核心能力：支持空状态欢迎页、工作区/线程消息历史加载、用户发送消息、SSE 流式回复、停止生成、基础消息列表渲染与自动滚动。

**Architecture:** 新增 `HermindChatMessage` 消息模型与 `ChatStreamHandler`/`AgentEventHandler` 两个纯逻辑处理器，把 SSE 事件和 Agent WebSocket 事件转换为消息列表变更；新增 `ChatContainerWidget`、`ChatHistoryWidget`、`ChatMessageItem` 三层 Qt Widget 负责展示与交互；`MainChatWidget` 把现有的占位聊天区替换为 `ChatContainerWidget`，复用已有的 `HermindApiClient`、`NavigationManager` 与主题系统。

**Tech Stack:** Qt 6 Widgets + qmake，C++17，QNetworkAccessManager/QWebSocket，Qt Test。

> For executing workers: implement this plan task-by-task (prefer a fresh subagent/Task per task — a clean context per task avoids single-session degradation). Steps use - [ ] checkboxes for tracking.

## File Structure

```
hermind-desktop/
├── models/
│   ├── hermind_chat_message.h          # 聊天消息数据模型
│   └── hermind_chat_message.cpp
├── chat/
│   ├── chat_stream_handler.h           # SSE 流式响应 -> 消息列表
│   ├── chat_stream_handler.cpp
│   ├── agent_event_handler.h           # Agent WebSocket 事件 -> 消息列表
│   └── agent_event_handler.cpp
├── widgets/
│   ├── chat_message_item.h             # 单条消息气泡
│   ├── chat_message_item.cpp
│   ├── chat_history_widget.h           # 可滚动消息列表
│   ├── chat_history_widget.cpp
│   ├── chat_container_widget.h         # 聊天容器（状态 + API 调用）
│   └── chat_container_widget.cpp
├── main_chat_widget.h/.cpp/.ui         # 替换占位区域为 ChatContainerWidget
├── hermind-desktop.pro                 # 新增源文件
└── tests/
    ├── models/tst_chat_message.cpp + chat_message_test.pro
    ├── api/tst_api_client_chat_history.cpp + chat_history_test.pro
    ├── chat/tst_chat_stream_handler.cpp + chat_stream_handler_test.pro
    ├── chat/tst_agent_event_handler.cpp + agent_event_handler_test.pro
    ├── widgets/tst_chat_message_item.cpp + chat_message_item_test.pro
    ├── widgets/tst_chat_history_widget.cpp + chat_history_widget_test.pro
    └── widgets/tst_chat_container_widget.cpp + chat_container_widget_test.pro
```

## Dependency Overview

```
Phase A: 数据模型 + API
  Task 1: HermindChatMessage 模型
      |
      v
  Task 2: API 客户端 chatHistory 方法

Phase B: 流式 / Agent 逻辑处理器
  Task 3: ChatStreamHandler  (依赖 Task 1)
      |
      v
  Task 4: AgentEventHandler  (依赖 Task 1)

Phase C: UI 控件
  Task 5: ChatMessageItem    (依赖 Task 1)
      |
      v
  Task 6: ChatHistoryWidget  (依赖 Task 1, Task 5)
      |
      v
  Task 7: ChatContainerWidget (依赖 Task 1, Task 2, Task 3, Task 4, Task 6)
      |
      v
  Task 8: 主题样式集成       (依赖 Task 5, Task 6, Task 7)

Phase D: 集成与构建
  Task 9: MainChatWidget 替换占位区 (依赖 Task 7, Task 8)
      |
      v
  Task 10: 更新 hermind-desktop.pro + 构建验证
      |
      v
  Task 11: 全量测试与手动验证
```

**可并行：**
- Phase A 内部 Task 1/2 串行（Task 2 依赖 Task 1）。
- Phase B 的 Task 3 与 Task 4 可并行（均依赖 Task 1）。
- Phase C 的 Task 5 可与 Phase B 并行（依赖 Task 1）；Task 6 依赖 Task 5；Task 7 依赖 Task 2/3/4/6；Task 8 依赖 Task 5/6/7。
- Phase D 必须等 Phase A/B/C 全部完成。

## 边界与范围

**阶段 1.3 内必须实现：**
1. 聊天消息数据模型（`HermindChatMessage`）。
2. 加载工作区默认聊天历史 `/workspace/:slug/chats` 与线程历史 `/workspace/:slug/thread/:threadSlug/chats`。
3. 用户发送消息并触发 SSE 流式响应。
4. 处理 SSE 事件类型：`textResponse`、`textResponseChunk`、`finalizeResponseStream`、`abort`、`statusResponse`、`stopGeneration`、`modelRouteNotification`、`agentInitWebsocketConnection`、action（`reset_chat`、`rename_thread`）。
5. 处理 Agent WebSocket 事件：`reportStreamEvent`、`statusResponse`、`fileDownloadCard`、`rechartVisualize`、`wssFailure`、`toolApprovalRequest`、`clarificationRequest`、线程重命名事件。
6. 空状态欢迎页与有消息时的聊天视图切换。
7. 停止生成按钮与 `abortStream()` 调用。
8. 消息列表自动滚动到底部。

**阶段 1.3 内显式不实现（留给后续子阶段）：**
- Markdown / 代码块 / 引用高亮渲染（阶段 1.4）。
- 附件拖拽上传、图片预览、PromptInput 高级 UI（阶段 1.5）。
- @Agent 工具菜单、Agent 技能面板、澄清问题 UI、工具审批 UI（阶段 1.6）。
- Sources 侧边栏、Memories 侧边栏、快捷操作、建议消息、WorkspaceModelPicker、ChatSettingsMenu（阶段 1.7）。
- 重新生成、编辑消息、Fork 线程、TTS、反馈评分、Metrics 弹窗。

## Risks & Open Questions

| 风险 | 说明 | 缓解 |
|------|------|------|
| SSE 事件顺序与前端不一致 | 后端 `textResponseChunk` / `finalizeResponseStream` 的顺序依赖 UUID 匹配 | 处理器严格按 UUID 查找并追加，找不到时新建占位消息 |
| Agent WebSocket 在 Qt 中的重连/生命周期 | WebSocket 在 `ChatContainerWidget` 析构或切换工作区时必须关闭 | `ChatContainerWidget` 在 `setWorkspace`/`setThreadSlug` 变化或析构时调用 `closeAgentWebSocket()` |
| 消息列表长内容性能 | 本阶段不做虚拟滚动，先实现基础列表 | 限制首次加载历史条数由后端决定；若明显卡顿再提前进入阶段 7.0 |
| 主题颜色同步 | 需要复用 `ThemeColors` 和 `ThemeManager` | 在 Task 8 统一通过 `ThemeManager::themeChanged` 刷新样式表 |

## Parts

| # | File | Scope | Status |
|---|---|---|---|
| 1 | 2026-07-13-hermind-desktop-1-3-chat-container/models.md | 数据模型 + API | done |
| 2 | 2026-07-13-hermind-desktop-1-3-chat-container/stream.md | SSE / Agent 流处理器 | done |
| 3 | 2026-07-13-hermind-desktop-1-3-chat-container/ui.md | UI 控件与主题 | done |
| 4 | 2026-07-13-hermind-desktop-1-3-chat-container/integration.md | 集成、构建、测试、验证 | done |

## Spec-Coverage Table

| Spec 项 | 覆盖任务 | 状态 |
|---------|---------|------|
| 聊天消息数据模型 | Task 1 | covered |
| 工作区/线程历史加载 API | Task 2 | covered |
| SSE 流式响应处理 | Task 3 | covered |
| Agent WebSocket 事件处理 | Task 4 | covered |
| 单条消息 UI 渲染 | Task 5 | covered |
| 消息列表滚动与自动底部 | Task 6 | covered |
| 聊天容器状态管理与发送 | Task 7 | covered |
| 主题/暗色模式样式 | Task 8 | covered |
| 替换 MainChatWidget 占位区 | Task 9 | covered |
| 工程文件更新与编译 | Task 10 | covered |
| 单元测试 + 手动端到端验证 | Task 11 | covered |
| Markdown 渲染 | — | no-op (1.4) |
| 附件/DnD 上传 | — | no-op (1.5) |
| Agent 工具菜单/@Agent | — | no-op (1.6) |
| Sources/Memories 侧边栏 | — | no-op (1.7) |
| 重新生成/编辑/Fork/TTS | — | no-op (后续阶段) |

## Self-Review

- [ ] 1. Spec-coverage table: 上表已映射所有 1.3 功能到任务，无 GAP。
- [ ] 2. Placeholder scan: 各 Part 文件中无 TODO/TBD/延迟实现占位。
- [ ] 3. No phantom tasks: 每个任务均产生可验证变更（代码/测试/构建/手动验证）。
- [ ] 4. Dependency soundness: 每个任务的 `Depends on:` 均指向前序任务，无向后引用。
- [ ] 5. Caller & build soundness: 共享签名变更（如 `HermindApiClient` 新增方法）在同任务内更新所有调用方并全量编译。
- [ ] 6. Test-the-risk: 状态变更逻辑（流处理器、事件处理器）均有行为断言，而非仅编译检查。
- [ ] 7. Type consistency: 模型字段、回调签名在各任务间保持一致。
