# Agent Runtime (Go) 设计 —— 接入 pantheon/conversation + pantheon/agent + WebSocket

**Date**: 2026-05-26
**Status**: Draft
**Author**: brainstorming session
**Scope**: 复刻 Node `server/endpoints/agentWebsocket.js` + `server/utils/agents/{index.js, aibitat/*}`，目标是让 backend 能跑 `@agent` 多步工具调用，覆盖 RAG / Web / SQL / 文档总结 / MCP / AgentFlow / 文件系统等技能；底层以 `github.com/odysseythink/pantheon` 的 `conversation`、`agent`、`tool` 三个子包替代 aibitat 框架本身

---

## 1. 目标与边界

### 1.1 目标

- 提供与 Node 行为对齐的 `WS /api/agent-invocation/:uuid` 端点
- 同时复刻 Node "HTTP → 写 invocation → 返回 `agentInitWebsocketConnection` → 前端拨号 WS" 的协议交接，**不破坏现有前端**
- 以 `pantheon/conversation` 替换 aibitat 的 channel/participant 编排，`pantheon/agent` 替换 aibitat 的 step-loop + tool 调度
- 工具注册中心同时容纳：本地默认 skill（RAG / docSummarizer / webScrape / rechart / sqlAgent / fs / createFiles）、imported plugins、agent-flows、MCP 已发现工具
- 暴露 `ActiveAgentRuntime`，让 `chat.go` / `api_chat.go` / `api_openai.go` 在判定为 agent 触发时调用统一入口
- 支持 graceful shutdown：进程退出时关闭所有活跃 WS、`Conversation.Terminate`、释放 MCP 信号量
- 完整可测：mock LLM provider + mock tool registry + httptest WS client，e2e 跑通 step-finish 链路

### 1.2 非目标（v1）

- 不复刻 Node "工具批准（toolApprovalRequest）+ skill whitelist" 全套交互；v1 只支持**自动批准**和**全部拒绝**两个开关
- 不复刻 gmail/google-calendar/outlook 等 OAuth 第三方插件（这些走 PR-N，独立产物）
- 不实现"importedPlugin"动态加载（manifest-driven Node 插件），仅做扩展点预留
- 不接入 `EphemeralAgentHandler`（API key 路径下的 stateless agent，Phase 2）
- 不支持 multi-channel（Node 也几乎只用 single-channel，YAGNI）

---

## 2. Node 现状速览

### 2.1 触发与连接生命周期

```
前端发 POST /workspace/:slug/stream-chat
   ↓
server/utils/chats/stream.js → grepAgents(message)
   ↓ (message starts with @agent OR workspace.chatMode=automatic & supportsNativeToolCalling)
WorkspaceAgentInvocation.new() → 写 DB，得 uuid
   ↓
SSE 返回 { type: "agentInitWebsocketConnection", websocketUUID: uuid, close: false }
SSE 返回 { type: "statusResponse", textResponse: "@agent: Swapping over...", close: true }
   ↓ 前端拨号 WS /agent-invocation/:uuid
agentWebsocket.js 接收 → AgentHandler({uuid}).init() → createAIbitat({socket}) → startAgentCluster()
   ↓ aibitat 跑 step loop，通过 socket.send 推送 { type, content } 消息
完成 → aibitat.onTerminate → socket.close
```

### 2.2 WS 上行消息（Frontend → Server）

| `type` | 触发时机 | 处理函数 |
|---|---|---|
| `awaitingFeedback` | aibitat 进入 `onInterrupt` 时，前端按提示给 user feedback | `socket.handleFeedback` 解开 promise |
| `toolApprovalResponse` | aibitat 请求 skill 批准时 | `socket.handleToolApproval` 解开 promise |
| 命令字（裸字符串 `exit`/`stop`/`/exit`/`/stop`/`halt`/`/halt`/`/reset`） | 用户主动中止 | `socket.checkBailCommand` → `aibitat.abort()` |

### 2.3 WS 下行消息（Server → Frontend）

| `type` | 内容 | 触发 |
|---|---|---|
| `statusResponse` | `{ content, animate }` | `aibitat.introspect(text)`（思考状态、工具结果摘要） |
| `wssFailure` | `{ content }` | 任一 error |
| `WAITING_ON_INPUT` | `{ question }` | `socket.awaitResponse` |
| `toolApprovalRequest` | `{ requestId, skillName, payload, description, timeoutMs }` | 请求批准 |
| `[message]` 原生 chat | `{ from, to, content, state, ... }` | `aibitat.onMessage` 中转（USER 消息默认 muted） |
| `__unhandled` | 任意 | 通用 escape hatch |

### 2.4 AgentHandler 内部

- 读 `Workspace + User + Thread` 拼 AIbitat 配置
- 装载 plugins：`websocket`（必选）+ `chat-history` + `memory(rag-memory)` + `docSummarizer` + `webScraping` + ... 受 SystemSettings `disabled_agent_skills` 控制
- 装载 imported plugins、AgentFlows、MCP `activeMCPServers()`
- 通过 `Provider.systemPrompt({provider, workspace, user})` 拼 system prompt
- `aibitat.start({content})` 把用户消息送入 channel

---

## 3. Pantheon 上游能力映射

### 3.1 `pantheon/conversation`（aibitat 框架本身的替代）

| Aibitat 概念 | Pantheon 类型 | 行为映射 |
|---|---|---|
| `aibitat.agent` 注册 | `conversation.Participant{ Name, Role, Model, Agent }` | 一个 participant = 一个 agent（USER 用 Participant + `Interrupt: ALWAYS` 表达） |
| `aibitat.channel` | `conversation.Channel{ Name, Members[], Role, MaxRounds, Model }` | 1:1 chat 也用 Channel 表达（成员 = [USER, @agent]） |
| `aibitat.onMessage` | `Conversation.OnMessage(MessageHandler)` | 1:1 映射 |
| `aibitat.onError` | `Conversation.OnError(ErrorHandler)` | 1:1 |
| `aibitat.onTerminate` | `Conversation.OnTerminate(TerminateHandler)` | 1:1 |
| `aibitat.onInterrupt` | `Conversation.OnInterrupt(InterruptHandler)` | 1:1 |
| `aibitat.start({content})` | `Conversation.Start(ctx, route, content)` | 入参对齐 |
| `aibitat.continue(feedback, attachments)` | `Conversation.Continue(ctx, ...)` | 待验证（pantheon API 可能要扩展） |
| `aibitat.terminate()` | `Conversation.Terminate(node)` | 1:1 |
| `aibitat.abort()` | 没直接对应 | **gap：用 `context.CancelFunc` + `Terminate` 组合表达** |

> 已读源码确认：`Conversation` 提供 `OnStart/OnMessage/OnError/OnTerminate/OnInterrupt` 5 个事件钩子，`RegisterParticipant/RegisterChannel`，以及内部 `newMessage/newError/terminate/interrupt`。`maxRounds` 默认 100。继续/取消的具体 API 在 v1 实现时按需对 pantheon 提 PR 或 fork。

### 3.2 `pantheon/agent`（step-loop + tool 调度）

- `agent.New(model, ...Option)` → 内部维护 `maxSteps`、`toolRegistry`、`compressor`
- `Agent.Run(ctx, *core.Request) (*Result, error)` —— 同步多步循环；超步抛 `agent reached max steps`
- `Agent.RunStream(ctx, *core.Request) StreamResponse`（`iter.Seq2[*StreamEvent, error]`）
- `StreamEventType` ∈ `text_delta` / `reasoning_delta` / `tool_call` / `tool_result` / `step_start` / `step_finish` / `usage` / `error`
- `WithRegistry(*tool.Registry)` —— **关键**，把工具注册中心交给 agent；executor 自动 30s 超时 + panic recover

> `pantheon/agent` 的 step loop + 工具调度等价于 aibitat 的 `_chat` step；我们**不再重写循环**，只负责构造 Agent + 喂 Request。

### 3.3 `pantheon/tool.Registry`（工具注册中心）

```go
type Entry struct {
    Name           string
    Toolset        string
    Schema         core.ToolDefinition
    Handler        Handler         // func(ctx, args json.RawMessage) (string, error)
    CheckFn        CheckFunc       // 运行时可用性闸门
    RequiresEnv    []string
    IsInteractive  bool            // 串行执行标记
    MaxResultChars int             // 截断
    Description    string
    Emoji          string
}
```

- 每个本地 skill 实现一个 `Entry`：RAG/docSummarizer/webScrape/rechart/sqlAgent/fs/createFiles
- MCP 工具通过 `internal/mcp.Hypervisor.ToolsAsPlugins(serverName)` 一次性投影成 Entry
- AgentFlow 通过 `agent_flow_service` 投影
- Registry 是 per-session 构造的（避免 stale CheckFn + 用户级权限），不是全局单例

---

## 4. Go 架构

### 4.1 包布局

```
internal/agent/
├── runtime.go          // type Runtime；唯一对外入口
├── session.go          // type Session = 一次 invocation 的生命周期承载
├── transport_ws.go     // WebSocket 帧编解码 + 上下行路由 + bail/timeout
├── handler.go          // Gin 升级 + auth + uuid 校验
├── invocation.go       // workspace_agent_invocations 表 CRUD
├── system_prompt.go    // Provider.systemPrompt 等价物
├── tools/
│   ├── registry.go     // 把本地 + MCP + AgentFlow + imported 拼成 *tool.Registry
│   ├── rag_memory.go   // memory(rag-memory) 等价 —— search/store 两 action
│   ├── doc_summarizer.go
│   ├── web_browsing.go
│   ├── web_scraping.go
│   ├── rechart.go
│   ├── sql_agent.go    // 跨多 DB（mysql/postgres/sqlite/mssql）查询
│   ├── filesystem.go
│   ├── create_files.go
│   └── disabled.go     // 读 SystemSettings disabled_agent_skills 名单
├── plugins_imported.go // 接口预留（v1 stub）
└── doc.go

internal/services/
└── agent_service.go    // 给 chat_service / api_chat 用的 facade：grepAgents 等价
```

### 4.2 类型签名

```go
// internal/agent/runtime.go

type Runtime struct {
    cfg       *config.Config
    db        *gorm.DB
    llmFactory providers.Factory      // 复用 providers/llm.go 的工厂
    embedder  embedder.Embedder
    vectorSvc *services.VectorSearchService
    docSvc    *services.DocumentService
    mcp       *mcp.Hypervisor
    flowSvc   *services.AgentFlowService
    sysSvc    *services.SystemService
    eventLog  *services.EventLogService

    sessions sync.Map  // uuid → *Session
    upgrader websocket.Upgrader
}

func NewRuntime(deps Deps) *Runtime
func (r *Runtime) HandleWS(c *gin.Context)
func (r *Runtime) CreateInvocation(ctx context.Context, ws *models.Workspace, user *models.User, thread *models.WorkspaceThread, prompt string) (uuid string, err error)
func (r *Runtime) IsAgentInvocation(ctx context.Context, ws *models.Workspace, message string) (bool, error)
func (r *Runtime) Shutdown(ctx context.Context) error

// internal/agent/session.go

type Session struct {
    UUID         string
    WorkspaceID  int
    UserID       *int
    ThreadID     *int
    Conv         *conversation.Conversation
    Agent        *agent.Agent
    Tools        *tool.Registry
    Socket       *wsConn
    StartedAt    time.Time
    cancel       context.CancelFunc
}

func (s *Session) Run(ctx context.Context, prompt string) error
func (s *Session) Abort()
```

### 4.3 WebSocket 库选型

| 候选 | 优点 | 缺点 | 决定 |
|---|---|---|---|
| `github.com/gorilla/websocket` | 生态标准、和 Gin 集成大量示例 | 单包社区维护，但 v1.5 稳定 | **选用** |
| `nhooyr.io/websocket` (`github.com/coder/websocket`) | 现代 API、context 一等公民 | 与 Gin 集成示例少 | 备选 |
| `github.com/gobwas/ws` | 零分配性能极致 | API 偏低层，开发量大 | 否决 |

固定到 `gorilla/websocket v1.5.x`，配置：

- `ReadBufferSize: 4096`, `WriteBufferSize: 4096`
- `CheckOrigin`：从 `cfg.CORSOrigin` 推导
- `HandshakeTimeout: 10s`
- 每 conn 启动 reader goroutine + writer goroutine + 心跳 ticker
- 读：`SetReadDeadline(now + 5min)`，每收到一帧重置（Node 用 300s 超时）
- 写：通过 buffered chan `outbound <-chan []byte` 串行化，避免多 goroutine 直写

---

## 5. WebSocket 线协议（前端零改动）

完全沿用 Node 现有 JSON 格式，**字段名、type 取值、嵌套结构必须 1:1 相同**，确保 `frontend/src/utils/chat/agent.js` 不需要任何 if-go-else 分支。

### 5.1 上行帧（Frontend → Go）

```ts
type ClientFrame =
  | string                                                // bail commands: "exit"|"/exit"|"stop"|"/stop"|"halt"|"/halt"|"/reset"
  | { type: "awaitingFeedback"; feedback: string; attachments?: any[] }
  | { type: "toolApprovalResponse"; requestId: string; approved: boolean }
```

Go 端：

```go
type clientFrame struct {
    Type        string `json:"type,omitempty"`
    Feedback    string `json:"feedback,omitempty"`
    Attachments []any  `json:"attachments,omitempty"`
    RequestID   string `json:"requestId,omitempty"`
    Approved    bool   `json:"approved,omitempty"`
}
```

读循环：

1. 先按 string 解码尝试 bail 命令（Node 也支持裸字符串）
2. 否则按 `clientFrame` JSON 解码
3. 按 `type` 路由到当前等待 promise（用 Go `chan` + `select`）

### 5.2 下行帧（Go → Frontend）

| `type` | 字段 | 触发 |
|---|---|---|
| `statusResponse` | `content`, `animate=true` | introspect |
| `wssFailure` | `content` | 任一 error → 然后 close socket |
| `WAITING_ON_INPUT` | `question` | 等待用户 feedback |
| `toolApprovalRequest` | `requestId`, `skillName`, `payload`, `description`, `timeoutMs` | 需要批准 |
| chat message | `from`, `to`, `content`, `state` | `Conversation.OnMessage` |
| `__unhandled` | `content` | 兜底 |

每帧写之前 `json.Marshal`，按 `TextMessage` 发送。

### 5.3 序列示意

```
WS握手 ✓
S→C : { type:"statusResponse", content:"@agent: invoking...", animate:true }
   (Agent.RunStream 开始，进入 step 1)
S→C : { type:"statusResponse", content:"thinking...", animate:true }
S→C : { from:"@agent", to:"USER", content:"我会先查询文档", state:"success" }
S→C : { type:"statusResponse", content:"calling tool: rag-memory", animate:true }
   (tool 执行)
S→C : { type:"statusResponse", content:"tool result truncated to 4000 chars", animate:true }
   (step 2 ... 直到 finish)
S→C : { from:"@agent", to:"USER", content:"<最终答案>", state:"success" }
   (Conversation.Terminate)
WS close (1000)
```

---

## 6. 触发与协议交接（HTTP → WS）

### 6.1 在 `chat_service.Stream` 中插入触发判定

```go
// internal/services/chat_service.go (新增分支)
if invoked, err := s.agentRuntime.IsAgentInvocation(ctx, ws, req.Message); err != nil {
    return nil, err
} else if invoked {
    uuid, err := s.agentRuntime.CreateInvocation(ctx, ws, user, thread, req.Message)
    if err != nil { return nil, err }

    // 像 Node 一样在 SSE 流里返回两个特殊 chunk，然后关闭
    out := make(chan dto.StreamChatResponse, 2)
    out <- dto.StreamChatResponse{
        Type: "agentInitWebsocketConnection",
        WebsocketUUID: &uuid,
        Close: false,
    }
    out <- dto.StreamChatResponse{
        Type: "statusResponse",
        TextResponse: utils.Ptr("@agent: Swapping over to agent chat. Type /exit to exit agent execution loop early."),
        Close: true,
        Animate: true,
    }
    close(out)
    return out, nil
}
```

> `dto.StreamChatResponse` 已经有 `Type` 字段，新增 `WebsocketUUID *string` 即可，对前端透明（前端读到该 chunk 就触发拨号）。

### 6.2 `IsAgentInvocation` 判定逻辑

```go
func (r *Runtime) IsAgentInvocation(ctx context.Context, ws *models.Workspace, message string) (bool, error) {
    // 1. @agent 前缀
    if strings.HasPrefix(strings.TrimSpace(message), "@agent") { return true, nil }
    // 2. workspace.chat_mode = "automatic" 且 provider 支持 native tool calling
    if ws.ChatMode == "automatic" && supportsNativeToolCalling(ws, r.cfg) { return true, nil }
    return false, nil
}
```

`supportsNativeToolCalling` 是个静态白名单（参考 Node `Workspace.supportsNativeToolCalling`），首批包含 `openai/anthropic/google/groq/mistral/ollama` 等。

### 6.3 DB schema：`workspace_agent_invocations`

复刻 Node prisma 表：

```go
// internal/models/workspace_agent_invocation.go

type WorkspaceAgentInvocation struct {
    ID          int       `gorm:"primaryKey"`
    UUID        string    `gorm:"uniqueIndex;not null"`
    WorkspaceID int       `gorm:"index;not null"`
    UserID      *int      `gorm:"index"`
    ThreadID    *int      `gorm:"index"`
    Prompt      string    `gorm:"type:text;not null"`
    Closed      bool      `gorm:"default:false"`
    CreatedAt   time.Time
    UpdatedAt   time.Time
}
```

由 `services/db.go` 自动迁移。WS 接入时校验 `Closed=false`，否则关 socket。

---

## 7. Tool registry 装载顺序与去重

在 `Runtime.HandleWS` → `buildSessionRegistry(workspaceID, userID)` 中按顺序装载：

1. **本地默认 skill**（按 `SystemSettings.disabled_agent_skills` 过滤）
   - `rag-memory`（默认开）
   - `document-summarizer`（默认开）
   - `web-scraping`（默认开）
   - `web-browsing`（仅在 `AgentSerperProvider` 等配置就绪时开）
   - `rechart`
   - `sql-agent`（仅当至少一个 DB 凭据配置）
   - `filesystem-agent` / `create-files-agent`（受 isToolAvailable 闸门）
2. **MCP 工具**：`for _, name := range hv.ActiveServers() { for _, p := range hv.ToolsAsPlugins(name) { register(toolFromMCP(p)) } }`
   - 命名约定：`<server>-<tool>`（与 Node 一致）
3. **AgentFlows**：`for _, f := range flowSvc.ActivePlugins() { register(toolFromFlow(f)) }`
4. **Imported plugins**：v1 stub（注释 `TODO: PR-N`）

**去重策略**：后注册者覆盖前注册者，并往 `eventLog` 记 `agent.tool.override`。

---

## 8. 默认 skill 单点说明

只列**v1 必须落地**的 4 个，其余 6 个进 v2（PR-后续）。

### 8.1 `rag-memory`

- 两个 action：`search` / `store`
- `search`：调 `VectorSearchService.SimilaritySearch(workspaceID, query, topK)`，把 chunk 拼成上下文返给 LLM
- `store`：用 `embedder` 嵌入后，写入工作区专用 namespace（`memory-<workspaceID>`）；首次写需建 namespace
- 与 chat.go 的 RAG 共用 `vectorSvc`，但 search/store 都不污染聊天历史

### 8.2 `document-summarizer`

- 两个 action：`list` / `summarize`
- `list`：调 `DocumentService.ListWorkspaceDocuments(workspaceID)`，输出 filename + first 200 char
- `summarize`：从向量库取该文档全文 chunks → 调 LLM 走 map-reduce summarize（沿用 `summarizeContent`，Go 化）

### 8.3 `web-scraping`

- 单一参数 `url`
- Go 用 `net/http` + `golang.org/x/net/html` 拉 HTML，跳 robots.txt
- 提取 `<article>` 优先，否则 `<main>` 否则 `<body>`，去 script/style/nav
- 限制 max 100KB 文本，超时 30s
- 失败返回 `Error("...")`（pantheon tool 约定）

### 8.4 `web-browsing`（可选）

- 走第三方 SERP（Serper / Brave / SearXNG），凭据来自 SystemSettings
- 任一未配置则 `CheckFn` 返回 false，不会出现在 registry

> `sql-agent`、`filesystem`、`create-files`、`rechart` 进 v2。`gmail/outlook/google-calendar` 进 v3（OAuth）。

---

## 9. 认证

```go
// internal/handlers/agent_ws.go
api.GET("/agent-invocation/:uuid",
    middleware.WSValidatedRequest(authSvc),  // 新中间件
    runtime.HandleWS,
)
```

`WSValidatedRequest`：

- Gin 升级前从 query `?token=` 或 `Sec-WebSocket-Protocol` 子协议读 JWT
- 失败：`c.AbortWithStatus(http.StatusUnauthorized)`（升级前关 socket）
- 成功：把 `*models.User` 塞 context

> **不能用 `Authorization` header**：浏览器 WebSocket API 不支持自定义 header。统一用 query token（短期 token，3 分钟 TTL），由前端 `agentInitWebsocketConnection` 触发时由 HTTP API 颁发一次性 token。已有 `temporary_auth_token_service` 可直接复用。

---

## 10. 中止 / 超时 / Graceful shutdown

| 场景 | 处理 |
|---|---|
| 用户在 WS 发 `/exit` | `session.Abort()` → `context.CancelFunc()` → `Conversation.Terminate("USER")` → socket close 1000 |
| 5min 空闲 | reader read deadline 触发 → 同上 |
| LLM 报错 / tool panic | `Conversation.OnError` 收到 → 发 `wssFailure` → close |
| 进程 SIGTERM | `Runtime.Shutdown(ctx)`：遍历 `sessions sync.Map`，每个 `Abort()`；最多等 30s；超时硬关 |
| MCP 工具调用 | 沿用 PR-D 的 per-server 信号量 + 30s timeout；agent 看到的是字符串错误结果 |

---

## 11. 测试策略

### 11.1 单元

- `internal/agent/transport_ws_test.go`：用 `net.Pipe` 模拟 conn，验证帧编解码、bail 命令、JSON 解析
- `internal/agent/tools/rag_memory_test.go`：mock vectorSvc，验证 search/store 两 action
- `internal/agent/tools/doc_summarizer_test.go`：mock docSvc + LLM provider

### 11.2 集成

- `internal/agent/runtime_e2e_test.go`：
  1. 启 `httptest.NewServer` 套 Gin runtime
  2. mock LLM provider：第一步返回 tool_call，第二步返回 text_delta 完成
  3. 客户端用 `gorilla/websocket` 拨号 `/agent-invocation/:uuid`
  4. 断言收到 `step_start` → `statusResponse` → `tool_call`/`tool_result` → 终末 message → close

### 11.3 mock 工具集

`internal/agent/internal/mockprov/`：

- `MockLanguageModel` 实现 `core.LanguageModel`，按预设脚本 yield `StreamPart`
- `MockTool` 注册到 `tool.Registry`，记录调用次数 + 参数
- 默认 8 个标准脚本（zero-tool / single-tool / multi-step / panic / timeout / max-steps / abort / interrupt）

### 11.4 测试隔离

- 不依赖 Node `server/` 跑测试
- 不依赖网络：webScraping/webBrowsing 用 `httptest.NewServer` 桩
- WS test：客户端用 `nhooyr` 或 `gorilla` 都可，挑前者更简洁

---

## 12. 风险与权衡

| 风险 | 缓解 |
|---|---|
| `pantheon/conversation` v0.0.9 没暴露 `Continue/Abort` API | 先用 `context.CancelFunc + Terminate` 组合；如需 feedback 续跑，向 pantheon 提 PR 或本地小 fork |
| pantheon 升级破坏 API | 设计文档绑定 v0.0.9；升级前在 `agent_e2e_test.go` 跑回归 |
| Tool result 截断（4000 字符）丢信息 | 默认 4000，sql-agent 等设 16000；通过 `Entry.MaxResultChars` 配置 |
| WebSocket goroutine 泄漏 | reader/writer/ticker 三个 goroutine 用 `errgroup.WithContext`；session.cancel() 必关 |
| 多用户跨 session 数据混淆 | Registry per-session 构造，CheckFn 必读 userID 闸门；user_id 写入 sessions Map 作为 owner，断线后清理 |
| `disabled_agent_skills` SystemSettings 与 Node 不同步 | 双进程部署时，约定 Node 写、Go 读；并在 `system_setting.go` 加 watcher 5min 拉一次 |
| 前端不识别新 frame 字段 | 严格使用 Node 已有 type/字段命名；新增字段只能加在 Go 自有 type 下且默认值不破坏现有 UI 渲染 |

---

## 13. 与现有模块的接合点

| 现有模块 | 接合点 | 改动 |
|---|---|---|
| `internal/handlers/chat.go` `StreamChat` | 调 `agentRuntime.IsAgentInvocation` 早返 | +10 行 |
| `internal/services/chat_service.go` `Stream` | 命中 agent 时返回特殊 SSE 序列 | +15 行 |
| `internal/dto/chat.go` `StreamChatResponse` | 新增 `WebsocketUUID *string` 字段 | +1 字段 |
| `cmd/server/main.go` | 构造 `Runtime`，注入 `chat_service`，注册 WS 路由，graceful shutdown 链 | +20 行 |
| `internal/mcp/plugins.go` | 已存在，0 改动 |
| `internal/services/agent_flow_service.go` | 新增 `ActivePlugins() []FlowPlugin` 方法 | +25 行 |
| `internal/services/temporary_auth_token_service.go` | 新增 `IssueAgentWSToken(userID)`（3min TTL） | +15 行 |
| `internal/middleware/auth.go` | 新增 `WSValidatedRequest`（query token + 抬升 user） | +30 行 |

---

## 14. 分期交付

| PR | 范围 | 工时 |
|---|---|---|
| **PR-AR-1** | 包骨架 + `Runtime` 类型 + `gorilla/websocket` 升级 + WS auth + invocation DB 模型 + 健康路由（`/agent-invocation/:uuid` 返 echo 用于联调） | 5-7h |
| **PR-AR-2** | `pantheon/conversation` + `pantheon/agent` wiring：构造 Participant/Channel/Agent，OnMessage/OnError 桥接到 WS，单 LLM tool-less 走通 | 8-10h |
| **PR-AR-3** | `tool.Registry` 装载链路 + MCP/AgentFlow 投影 + `disabled_agent_skills` 过滤 + 4 个默认 skill（rag-memory/docSummarizer/web-scraping/rechart） | 10-12h |
| **PR-AR-4** | HTTP → WS 协议交接：`chat_service.Stream` 改造 + `agentInitWebsocketConnection` SSE chunk + tempToken 颁发 + e2e (httptest WS) | 8-10h |
| **PR-AR-5** | 中止/超时/Graceful shutdown + `wssFailure`/`WAITING_ON_INPUT` 完整链路 + `toolApprovalRequest`（auto-approve 开关，无 whitelist） | 6-8h |
| **PR-AR-6** | sql-agent + filesystem + create-files 三个补足 skill（CheckFn 闸门 + 自带集成测试） | 10-12h | ✅ |
| **PR-AR-7** | gmail-agent + google-calendar-agent（Apps Script 桥）+ outlook-agent（Microsoft Graph OAuth）| 22-27h | ✅ |

**v1 主链路 = PR-AR-1 至 PR-AR-5，约 37-47h（5-6 工作日）**；PR-AR-6 与 PR-AR-7 是补完，已落地。

---

## 15. 后续（不在 v1 范围）

- toolApprovalRequest + AgentSkillWhitelist（与 Node 联动持久化）
- imported plugin manifest 加载（Node 的 `imported.js` 等价）
- `EphemeralAgentHandler`（API key 路径 stateless agent）
- multi-channel / 多 agent 协作（pantheon `Channel` 支持，但前端无 UI）
- agent 调用观测面板（基于 pantheon `observability` + `eventLog`）

---

## 16. 实施计划文件

- `.gpowers/plans/2026-05-27-agent-runtime-pr1-skeleton.md`（待写）
- `.gpowers/plans/2026-05-27-agent-runtime-pr2-conversation-wiring.md`（待写）
- `.gpowers/plans/2026-05-27-agent-runtime-pr3-tool-registry.md`（待写）
- `.gpowers/plans/2026-05-27-agent-runtime-pr4-http-ws-handoff.md`（待写）
- `.gpowers/plans/2026-05-27-agent-runtime-pr5-lifecycle.md`（待写）

—— end of design
