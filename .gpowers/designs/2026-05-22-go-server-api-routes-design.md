# backend `/api/` 路由补齐设计方案（v2）

> 目标：backend 完整接管 Node `server/` 进程的 HTTP/WS 入口；最终运行环境仅剩 **backend + collector**（collector 因生态依赖保留 sidecar）。前端、桌面客户端、Agent WS、`/v1/*` 集成功能全部不降级。
>
> 策略：纵切（按"可发布的用户能力"分 Phase 交付），高风险子系统单独成期。

---

## 1. 范围与口径

### 1.1 "100% 替代 server/" 的精确定义

server/ 中所有 HTTP/WS 入口（除明确排除项外）由 backend 接管；server/utils/agents 的 Agent kernel 也全部 Go 化。最终部署形态：
- **backend**（单 Go 二进制）：所有路由、Agent runtime、cron、嵌入流水线、SSE、WS
- **collector**（保留 Node sidecar）：文档解析、抓取、OCR 等重型依赖

### 1.2 IN（必须实现）

| 类别 | 数量 | 说明 |
|---|---|---|
| 已实现路由保持 | 99 method×path | 不动 |
| system.js 缺失 | 29 | PFP、偏好、Prompt 预设/变量、本地文件、SSO simple、文档移除等 |
| workspaces.js 缺失 | 17 | 上传、嵌入、SSE 进度、TTS、PFP、search/vector-search、prompt-history 等 |
| admin.js 缺失 | 8 | API Key 管理、用户/工作区 CRUD 完整 |
| document.js 缺失 | 2 | create-folder / move-files |
| workspacesParsedFiles.js 缺失 | 3 | delete-parsed-files / embed-parsed-file / parse |
| `/v1/*` 开发者 API（去 OpenAI 兼容） | ~48 | admin / auth / document / embed / system / users / workspace / workspaceThread |
| ext/* 数据连接器 | 8 | Confluence / Drupal / Obsidian / Paperless / GitHub / GitLab / website-depth / YouTube |
| experimental/* | 5 | Live Sync + agent-plugins |
| admin/agent-skills OAuth + 回调 | 8 | Outlook / Gmail / GCal 授权链路 |
| agent-skills/generated-files | 1 | Agent 产物文件下载 |
| **Agent WebSocket `/agent-invocation/:uuid`** | 1 (WS) | + 完整 Agent kernel 全 Go 化 |
| **合计新增** | **~130 method×path + 1 WS + Agent 内核** | |

### 1.3 OUT（明确不做）

- `/v1/openai/*` OpenAI 兼容层（4 条）
- `/api/community-hub/*`（7 条）
- `/api/mobile/*`（7 条）
- `/api/web-push/*`（2 条）

### 1.4 向量 DB 提供方覆盖

**全 9 种与 Node 对齐**：LanceDB、PGVector、Chroma、Qdrant、Pinecone、Weaviate、Milvus、Astra、Zilliz。

---

## 2. 总体架构

```
┌──────────────────────────────────────────────────────────────────────┐
│                          backend (single binary)                    │
│  ┌────────────────────────────────────────────────────────────────┐  │
│  │ HTTP Layer (Gin)                                                │  │
│  │  /api/* (前端) │ /api/v1/* (API Key) │ /api/agent-invocation │  │
│  │                                          (WebSocket, gorilla)  │  │
│  ├────────────────────────────────────────────────────────────────┤  │
│  │ Middleware: auth(JWT) / api-key / rbac / recovery / cors        │  │
│  ├────────────────────────────────────────────────────────────────┤  │
│  │ Handler Layer                                                   │  │
│  │  admin │ system │ workspace │ chat │ document │ thread │ embed  │  │
│  │  agent-flow │ agent-skill │ mcp │ telegram │ browser-ext │ ext  │  │
│  │  experimental │ apikey-v1 │ ws-agent                            │  │
│  ├────────────────────────────────────────────────────────────────┤  │
│  │ Service Layer                                                   │  │
│  │  AdminSvc / SystemSvc / WorkspaceSvc / ChatSvc / DocSvc /       │  │
│  │  ThreadSvc / EmbedSvc / AgentFlowSvc / APIKeySvc / PromptSvc /  │  │
│  │  TTSSvc / SSOSvc / SQLValidatorSvc / ExtConnectorSvc /          │  │
│  │  LiveSyncSvc / AgentSkillSvc / AgentKernel(Aibitat-Go)          │  │
│  ├────────────────────────────────────────────────────────────────┤  │
│  │ Infrastructure                                                  │  │
│  │  VectorDB(9) │ Embedder │ LLM Provider │ Reranker │ FileStore │ │  │
│  │  SSE Hub │ WS Hub │ Embed Queue │ Cron(robfig) │ Encryption │   │  │
│  │  CollectorClient(HTTP→sidecar) │ MCP Host │ OAuth2 Clients      │  │
│  ├────────────────────────────────────────────────────────────────┤  │
│  │ Storage: GORM(SQLite/Postgres) │ Local FS (storage/) │ VectorDB │  │
│  └────────────────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼ HTTP
                       ┌──────────────────────────┐
                       │  collector (Node sidecar) │
                       │  PDF/OCR/Office/URL 抓取  │
                       └──────────────────────────┘
```

### 2.1 关键架构约定

1. **单 Go 二进制**：路由层、Agent kernel、WS、cron、嵌入器、向量 DB 适配器统统跑在同一个进程。
2. **协议选型固化**：HTTP 用 `gin-gonic/gin`，WS 用 `gorilla/websocket`，cron 用 `robfig/cron/v3`。
3. **存储路径完全复用** Node 的 `storage/`（同一磁盘目录）；数据库表沿用 Prisma 已有 schema —— Go 端用 GORM 但**不做 schema 变更**，避免 Node→Go 切换期数据双轨。
4. **API Key 校验链路**：前端 JWT cookie 走 `auth middleware`；`/v1/*` 走 `api-key middleware`；两者复用同一 RBAC。
5. **单实例假设**：SSE Hub / WS Hub / Embed Queue 都是进程内状态。多实例水平扩展明确**列为非目标**。
6. **依赖外部资源**：collector（必选）、OAuth2 provider（按需）、外部向量 DB（按用户配置）。

---

## 3. Phase 划分

| Phase | 名称 | 主要交付 | 基础设施 | 路由增量 |
|---|---|---|---|---|
| **P4** | 多用户后台 + 系统设置补齐 | Admin 流程、用户/工作区 CRUD、API Key 管理、PFP、Prompt 预设/变量、SSO simple、本地文件浏览、文档/文件夹移除 | FileSystemSvc、SystemConfigSvc、APIKeySvc（补完）、PromptPresetSvc、PromptVariableSvc、SSOSvc、SQLValidatorSvc | 37 |
| **P5** | 工作区文档管理 + 嵌入流水线 | 上传/链接抓取、嵌入/取消嵌入、SSE 进度、reset-vector-db、search / vector-search、prompt-history、TTS、PFP、thread fork | EmbedQueue（进程内 worker pool）、SSE Hub、Reranker、TTSSvc、Collector 集成深化 | 22 |
| **P6** | 向量 DB 多提供方扩展 | 7 个新增 provider 适配器 + 配置切换 + 兼容性测试 | VectorDB interface 完善 | 0（仅扩 infra） |
| **P7** | 开发者 `/v1/*` API | admin / auth / document / embed / system / users / workspace / workspaceThread —— 复用 P4-P5 service 层 | API Key 鉴权 middleware 复用、Swagger 生成 | ~48 |
| **P8** | ext 数据连接器 + 生成文件下载 | Confluence / Drupal / Obsidian / Paperless / GitHub / GitLab / website-depth / YouTube + agent-skills 生成文件 | ExtConnectorSvc（薄包装 collector） | 9 |
| **P9** | experimental 模块 | Live Sync 队列 + 文件监听、社区 agent 插件加载（manifest 级，调用待 P10） | fsnotify Watcher、AgentPluginLoader、LiveSyncQueue | 5 |
| **P10** | Agent runtime 全 Go 化 + WS + OAuth 集成 | Aibitat-Go 内核、provider 适配层、plugins 全套、`/agent-invocation/:uuid` WS、admin/agent-skills OAuth 链路 | WS Hub、OAuth2 client、Plugin SPI、MCP Host | 9 |

### 3.1 分阶段策略要点

- **P4 → P7 走得快**：每片够用就上，不一次性造完所有 infra。
- **P6 是横向扩展**：纯 infra 层，路由 0 增量，解锁已上线功能给更多用户。
- **P7 高度复用**：`/v1/*` handler 是薄壳，service 层完全复用 P4-P5。
- **P10 是最大变量**：Agent runtime ≈ 30K LOC JS，Go 化需要再切 P10a-P10e（见 §5.7）。

---

## 4. 横切基础设施

### 4.1 鉴权层（中间件矩阵）

| 中间件 | 入口 | 用途 |
|---|---|---|
| `auth` | `/api/*` | JWT cookie 校验 → `c.Set("user", *User)` |
| `apiKey` | `/api/v1/*` | `Authorization: Bearer <key>` → `c.Set("apiKey", *APIKey)` |
| `rbac` | 选用 | 校验 `c.Get("user").Role` ∈ allowed |
| `embedSession` | `/api/embed/*` | sessionId 验证 |
| `multiUser` | 大量后台路由 | 单/多用户模式分流 |

### 4.2 FileSystemSvc

```go
ListDocuments(folder string) ([]DocEntry, error)
ReadDocument(folder, name string) ([]byte, error)
RemoveDocument(folder, name string) error
RemoveFolder(folder string) error
CreateFolder(name string) error
MoveFiles(moves []FileMove) error
SavePFP(userId int, content []byte, mime string) (path string, error)
ReadPFP(userId int) ([]byte, string, error)
SaveLogo(content []byte) error
ReadLogo() ([]byte, string, error)
IsDefaultLogo() (bool, error)
StoragePath(parts ...string) string
```

**路径硬约束**：所有路径以 `cfg.StorageDir` 为根；上层永远不传绝对路径；目录穿越校验在 service 层一处做。

### 4.3 VectorDB 接口（P6 关键）

```go
type VectorDatabase interface {
    Connect(ctx context.Context) error
    AddDocuments(namespace string, docs []Document, opts AddOptions) error
    RemoveDocuments(namespace string, docNames []string) error
    SimilaritySearch(namespace string, queryEmb []float32, topK int, filters Filter) ([]Match, error)
    ResetNamespace(namespace string) error
    DeleteNamespace(namespace string) error
    CountVectors(namespace string) (int, error)
    NamespaceExists(namespace string) (bool, error)
    ListNamespaces() ([]string, error)
    Heartbeat() error
}
```

每个 provider 在 `internal/vectordb/<name>.go` 实现，主程序按 `cfg.VectorDB` 选择。

### 4.4 EmbedQueue

- **进程内 worker pool**：goroutine pool + buffered chan，`EMBEDDING_CONCURRENCY` 配置并发度，默认 1
- 每个工作区一条逻辑队列，按 workspaceSlug 串行化（避免向量 DB 并发写冲突）
- 事件总线（与 Node IPC 协议同名）：`embed.batch_starting` / `doc_starting` / `chunk_progress` / `doc_complete` / `doc_failed` / `all_complete`
- **事件转发**：所有事件 `Publish` 到 SSE Hub topic `embed-progress:<workspaceSlug>`，由 `/workspace/:slug/embed-progress` 的 SSE handler 推给前端
- **进度持久化**：状态写 `WorkspaceParsedFiles` 表 + 进程内事件流；重启后不重做已完成文件

### 4.5 SSE Hub

```go
type SSEHub interface {
    Subscribe(topic string) (ch <-chan SSEEvent, unsubscribe func())
    Publish(topic string, event SSEEvent)
}
```

- topic 命名：`embed-progress:<workspaceSlug>` 等
- 实现：`map[topic]map[id]chan` + `sync.RWMutex`
- handler 用 `c.Stream()` 推给前端；keepalive 心跳 + `c.Writer.Flush()`

### 4.6 WS Hub

- `gorilla/websocket` 升级器
- 每个 `agent-invocation/:uuid` 一个 `AgentSession`，持有 Aibitat 实例与上下游 channel
- 错误统一发 `{type:"wssFailure", content:"..."}`（与 Node 协议一致）
- ping/pong 30s；断线后保留 session 60s 等重连

### 4.7 Cron Scheduler

`robfig/cron/v3`，进程内单实例。注册作业（对照 Node `server/jobs/`）：
- `cleanup-generated-files` —— 每天
- `cleanup-orphan-documents` —— 每天
- `sync-watched-documents` —— 每 10 分钟（受 LiveSync 开关控制，P9 接入）
- `handle-telegram-chat` —— 事件触发（非 cron）

**不做**：embedding-worker 子进程级 OOM 隔离 —— Go 端用 worker pool，明确接受"OOM 整进程挂"的代价。

### 4.8 OAuth2 Client（P10）

- 通用包装 `golang.org/x/oauth2`
- 三个 provider 配置：Outlook（Microsoft Graph）、Gmail、Google Calendar
- Token 存储：复用 Prisma 已有表，Go 端 GORM model 映射；token 字段经 EncryptionManager 加密落盘
- 访问 token 60s 内将过期时主动刷新

### 4.9 Reranker

- 接口：`Rerank(query string, candidates []Match) ([]Match, error)`
- 实现：Cohere / 本地 BGE-Reranker
- 是否启用由 workspace `vectorSearchMode` 决定

### 4.10 明确不引入

- 分布式锁 / leader election（单实例假设）
- 消息队列（Redis / Kafka）—— SSE/WS Hub 进程内足够
- 指标采集后端（仅暴露 `/utils/metrics` 文本接口）
- 多租户硬安全（与 Node 现状一致，软隔离）

---

## 5. 各 Phase 内部分解

### 5.1 P4 — 多用户后台 + 系统设置补齐（37 路由）

**Admin 8 条**

| Method | Path | Handler → Service |
|---|---|---|
| POST | `/admin/users/new` | AdminH → `AdminSvc.CreateUser` |
| POST | `/admin/user/:id` | AdminH → `AdminSvc.UpdateUser` |
| GET | `/admin/workspaces/:workspaceId/users` | AdminH → `WorkspaceSvc.ListWorkspaceUsers` |
| POST | `/admin/workspaces/new` | AdminH → `WorkspaceSvc.Create` |
| POST | `/admin/workspaces/:workspaceId/update-users` | AdminH → `WorkspaceSvc.UpdateUsers` |
| DELETE | `/admin/workspaces/:id` | AdminH → `WorkspaceSvc.Delete` + `VectorDBSvc.DeleteNamespace` |
| GET | `/admin/api-keys` | AdminH → `APIKeySvc.List` |
| POST | `/admin/generate-api-key` | AdminH → `APIKeySvc.Create` |
| DELETE | `/admin/delete-api-key/:id` | AdminH → `APIKeySvc.Delete` |

**System 29 条**（按类聚合）：
- **用户态**（5）：check-token、refresh-user、update-password、enable-multi-user、user (POST)
- **文件系统**（5）：local-files、accepted-document-types、remove-document、remove-documents、remove-folder
- **PFP**（3）：pfp/:id、upload-pfp、remove-pfp
- **Logo**（3）：upload-logo、remove-logo（GET 已有 logo / is-default-logo）
- **Prompt 预设**（4）：slash-command-presets（GET / POST / POST :id / DELETE :id）
- **Prompt 变量**（3）：prompt-variables（POST）、:id（PUT / DELETE）
- **杂项**（6）：system-vectors、custom-app-name、export-chats、workspace-chats（POST / DELETE :id）、default-system-prompt、validate-sql-connection
- **入口/SSO**：env-dump、migrate、request-token/sso/simple

**P4 关键决策**

1. `validCanModify` 权限规则放 **service 层**（admin 改自己 vs 改他人 需业务上下文）。
2. PFP / Logo 命名沿用 Node：`pfp-<userId>.<ext>`、`logo.<ext>`，否则前端缓存键会撞。
3. `system/migrate` 返回 noop 200（前端启动探活用，不能丢）。
4. `system/validate-sql-connection` 首版只支持 Postgres + SQLite，MySQL 列为已知限制。

### 5.2 P5 — 工作区文档管理 + 嵌入流水线（22 路由）

**Workspace 17 条**：上传/嵌入 9 条、聊天历史 4 条、TTS/PFP 3 条、其他 1 条。**核心是嵌入流水线**。
**parsedFiles 3 条** + **document 2 条**：纯 FileSystemSvc + 嵌入触发，无新难点。

**P5 关键决策**

1. **嵌入流水线时序**（与 Node 对齐）：
   ```
   POST /workspace/:slug/upload
     → FileSystemSvc.Save(uploaded file → storage/documents/<workspaceFolder>/)
     → CollectorClient.ParseDocument(filePath)        // sidecar
     → WorkspaceParsedFiles.Insert(status="pending")
     → EmbedQueue.Enqueue(file, workspaceSlug)        // 异步
     → 返回 201 + parsedFileId
   ```
   嵌入异步走 worker，进度通过 SSE `/workspace/:slug/embed-progress` 推送。

2. **upload-and-embed vs upload + update-embeddings**：保留两条独立路由，与 Node 一致。

3. **TTS 提供方**：先支持 OpenAI TTS + ElevenLabs；本地 Piper 与 Agent 一起做（P10）。

4. **search vs vector-search**：
   - `/workspace/search` 是文档名/元数据搜索（DB 查询）
   - `/workspace/:slug/vector-search` 是向量召回
   - 分两个 service 方法。

5. **remove-and-unembed 顺序**：VectorDB.Remove → 删盘 → 软删 ParsedFile；**三步全成功返回 200；任何一步失败返回 207 Multi-Status 并在 body 中列出各步骤状态（不返回 500）**，与 Node 行为对齐。

6. **prompt-history**：是 chat 子集（role=user 过滤），不需要新表。

### 5.3 P6 — VectorDB 多提供方扩展（0 路由）

**Provider 工作量评估**：

| Provider | Go SDK | 工作量 | 难点 |
|---|---|---|---|
| Chroma | 官方 chroma-go | S | API 稳定 |
| Qdrant | 官方 qdrant-go-client | S | gRPC + REST 双 |
| Pinecone | 官方 pinecone-go | S | namespace 概念差异 |
| Weaviate | 官方 weaviate-go-client | M | GraphQL filter 语法转换 |
| Milvus | 官方 milvus-sdk-go | M | collection schema 偏离最重 |
| Astra | DataStax REST | M | 相对小众 |
| Zilliz | 复用 Milvus 适配器 | S | Milvus 云版 |

**P6 关键决策**

1. **统一 `Document` 表示**：`{id, text, embedding, metadata{}}`。
2. **filter 表达式抽象**：`{field, op, value}` minimal 子集；复杂查询不支持，文档化说明。
3. **测试矩阵**：每个 provider 跑 add / search / remove / reset / count 五条用例。本地（Chroma / Qdrant / Weaviate / Milvus）走 Docker compose CI；云（Pinecone / Astra / Zilliz）走录制金样本 + 季度手测。

### 5.4 P7 — `/v1/*` 开发者 API（~48 路由）

**核心策略**：handler 是薄壳，service 层完全复用 P4-P5；只有 auth 中间件不同，响应封装稍有差异。

**P7 关键决策**

1. **响应差异**：`/v1/*` 返回 `{success, error, <data>}` 风格，`/api/*` 直接返回数据。统一在 handler 层用 wrapper 处理。
2. **OpenAPI 文档**：用 `swaggo/swag` 从注释生成。
3. **`/v1/users/:id/issue-auth-token`**：颁发短期 token 给前端跳转登录，特殊 API Key 权限校验（仅 admin scope）。
4. **`/v1/admin/workspaces/:workspaceSlug/manage-users`**：与 `/api/admin/workspaces/:workspaceId/update-users` 业务等价但参数键名不同；handler 转参后调同 service。

### 5.5 P8 — ext 数据连接器 + 生成文件下载（9 路由）

全部薄包装到 collector：

```go
ExtConnectorSvc.Confluence(req) → collector.POST /v1/connectors/confluence
ExtConnectorSvc.YouTubeTranscript(req) → collector.POST /v1/util/youtube-transcript
...
```

**P8 关键决策**

1. **响应原样透传**：collector 已是 Node 端最终响应格式；handler 直接 forward，不重新结构化。
2. **超时**：HTTP 客户端超时 5 分钟；handler 用 `context.WithTimeout` 显式管理。
3. **`agent-skills/generated-files/:filename`**：从 `storage/plugins/agent-skills/<filename>` 读盘；filename 仅允许 `[a-z]+-<uuid>(\.ext)?`，防穿越。

### 5.6 P9 — experimental 模块（5 路由）

**Live Sync**（3）+ **agent-plugins**（3）—— 工作量小但概念新。

**P9 关键决策**

1. **Live Sync 文件监听**：`fsnotify` 监听 `storage/documents/`，变化时触发 DocumentSyncQueue 入队。
2. **`/workspace/:slug/update-watch-status`**：单条文档"是否监听"开关，对应 `DocumentSyncQueue` 表增删。
3. **agent-plugins 加载**：从 `storage/plugins/agent-skills/<hubId>/plugin.json` 读 manifest；P9 仅做 manifest 注册 + 列表展示，真正调用由 P10 接入。

### 5.7 P10 — Agent runtime 全 Go 化（9 路由 + 内核）

最大变量，拆 5 个子阶段：

#### P10a · Aibitat 内核

- 多 Agent 循环：`function-calling` 模式（Anthropic tool use / OpenAI function calling）
- 消息历史维护、token 计数、context window 管理、agent 间消息路由
- 退出条件：max iterations / explicit terminate / error
- **不直接照搬 JS 设计**：Go 用 channel + state machine 重写更自然

#### P10b · Provider 适配层

| Provider | Go SDK | 备注 |
|---|---|---|
| Anthropic | `anthropics/anthropic-sdk-go` | 官方 |
| OpenAI | `sashabaranov/go-openai` | 社区主流 |
| Cohere | `cohere-ai/cohere-go-v2` | 官方 |
| Google AI (Gemini) | `google.golang.org/genai` | 官方 |
| Mistral | REST 直调 | 无官方 Go SDK |
| Groq | OpenAI 兼容（复用 SDK + base URL） | — |
| Ollama / LM Studio / LocalAI | OpenAI 兼容（同上） | — |
| Azure OpenAI | OpenAI SDK + Azure 认证 | — |

Provider 接口统一 4 个方法：`StreamChat / ChatCompletion / Embed / CountTokens`。

#### P10c · Plugins 全套（15 个）

| Plugin | 工作量 | 备注 |
|---|---|---|
| chat-history / file-history / memory | S | 纯 DB 读写 |
| summarize / web-scraping | S | 复用 collector |
| web-browsing | M | 无头浏览（chromedp） |
| filesystem / create-files | S | FileSystemSvc 已有 |
| rechart | M | 服务端 chart 生成（go-echarts 输出 PNG） |
| sql-agent | M | 复用 SQLValidatorSvc + 查询执行 |
| gmail / google-calendar / outlook | M | OAuth + Microsoft Graph / Google API |
| http-socket / websocket | S | WS Hub 已有 |
| cli | S | 不打算移植（仅本地调试用） |

#### P10d · `/agent-invocation/:uuid` WS 路由

- 升级 WS → 创建 AgentSession → 实例化 Aibitat → 接收 user message → 流式回写
- 错误恢复：plugin panic 不中断会话（goroutine recover）
- 与 Node `http-socket.js` 协议完全对齐：`{type, content}` JSON 帧

#### P10e · admin/agent-skills OAuth 链路（9 条路由）

| 路由 | 说明 |
|---|---|
| `GET /admin/agent-skills/outlook/auth-url` | 生成 OAuth 授权 URL |
| `GET /agent-skills/outlook/auth-callback` | OAuth 回调 → 存 token |
| `GET /admin/agent-skills/outlook/status` | 是否已授权 |
| `POST /admin/agent-skills/outlook/revoke` | 撤销 token |
| `GET /admin/agent-skills/gmail/status` | Gmail 授权状态 |
| `GET /admin/agent-skills/google-calendar/status` | GCal 授权状态 |

Gmail / GCal 暂只实现 status 查询（Node 端就是 status-only，真正授权另有入口）。

**P10 关键决策**

1. **Anthropic Tool Use 直接对齐 SDK 原生 tool 类型**，避免重新包装。
2. **WS 错误处理**：会话级 panic recover、plugin 级 timeout（默认 60s）、连接级 ping/pong。
3. **MCP Host 是 Aibitat 的 plugin 之一**，不是独立组件；与现有 `/mcp-servers/*` CRUD 串联。

---

## 6. 测试策略

### 6.1 单元测试

- 每个 service 一份 `_test.go`，外部依赖（DB / collector / VectorDB / LLM provider）用接口 + mock。
- **覆盖率门槛**：service 层 ≥ 70%，infra（VectorDB adapter / SSE Hub / WS Hub / EmbedQueue）≥ 80%。
- handler 层不强求单测（薄壳，归集成测试覆盖）。
- 关键 service 必有用例：边界（空输入、超长、特殊字符）、错误传播、并发安全。

### 6.2 集成测试（Gin handler 端到端）

- 测试基座：`httptest.Server` + 内存 SQLite + 临时目录 + LanceDB 内存模式 + mock collector
- 每条路由至少 3 个用例：正常路径、未授权、参数错误
- `/v1/*` 复用 `/api/*` 同类 fixture，仅替换 auth 头
- WS 测试：`gorilla/websocket` 测试客户端，验证消息协议帧、错误帧、连接清理

### 6.3 API 兼容性测试（**核心护栏**）

**金样本回放**：
- 准备 `golden/<route>.req.json` + `golden/<route>.resp.json` 数据集（先用 Node 录制 ≈100 个代表性请求）
- CI 跑 `go test -tags=compat`，对每条 golden 同时打 Node + Go 后端比对响应、状态码、关键字段
- **允许差异**（白名单）：时间戳、自增 ID、UUID、文件路径前缀
- **不允许差异**：业务字段、错误消息文案（前端可能断言）

**执行频率**：
- 本地：手动按需
- CI：每个 Phase 收尾跑全量；常规 PR 跑变更涉及子集

**VectorDB provider 兼容性**：每个 provider 五条标准用例（add / search / remove / reset / count）；本地 docker，云走金样本 + 季度手测。

### 6.4 Agent runtime 测试（P10 专项）

- **provider 层**：VCR 风格录制真实 LLM 响应；replay 模式跑 unit test
- **plugin 层**：每个 plugin 单独 unit test + 至少 1 个 e2e 用例
- **协议层**：录制 Node 端 WS 帧序列作为 golden，Go 端逐帧对齐 type / content

### 6.5 性能 / 压测

- 不做正式 benchmark suite
- **底线**：每个 Phase 收尾时手动 ab / wrk 几条热路径（chat stream、embed-progress、vector search），不慢于 Node 50%

---

## 7. 风险登记

| 风险 | 触发场景 | 影响 | 缓解 |
|---|---|---|---|
| Agent runtime 翻写工作量爆表 | P10 实际 LOC / 复杂度超预估 | 整个目标交付推迟数月 | P10 拆 a-e 各独立可发；若 P10b 之后受阻，临时退回 Agent sidecar 方案继续上线其他模块 |
| Provider Go SDK 能力差异（tool-use、流式） | 现实中 Go SDK 比 JS 慢半个版本 | Aibitat 某些能力缺失 | P10b 先做 spike：每个 provider 跑 `streamChat + tool-use + embed` 三件套，提前暴露差异并文档化 |
| Prisma↔GORM schema 偏移 | Go 端无意中改动表结构 | Node→Go 切换期数据双轨损坏 | GORM `DisableMigration: true`；schema 变更只走 Prisma；CI 加 schema-diff check |
| VectorDB 云服务无法完整自动化测试 | Pinecone / Astra / Zilliz 需付费账号 | 适配器质量靠手测 | 录制金样本 + 季度手测；文档化"云 provider 支持等级 ≠ 本地" |
| collector 协议改动 | Node 升级 collector 后接口变动 | Go 端 ext / 上传链路断裂 | CollectorClient 加一层版本协商；CI 加端到端 smoke test |
| 金样本与 Node 实际响应漂移 | Node 持续迭代修 bug | golden 对不上但实际是 Node 在变 | golden 录制时打 Node 版本戳；不一致时人工裁决 |
| 嵌入流水线在 worker pool 下 OOM | 大文档 + 高并发上传 | 整个 backend 挂 | EmbedQueue 默认并发 1；环境变量可调；监控内存告警；接受"OOM 整进程挂"代价 |
| WS 长连接资源泄漏 | Agent 会话异常断开 | fd / 内存累积 | 60s 会话清理；ping/pong 30s；进程级 fd 限制告警 |
| OAuth Token 加密落盘 | 数据库泄露导致 third-party 凭据外泄 | 用户外部账号受损 | 复用 EncryptionManager（AES-GCM）；token 字段全加密 |
| `/v1/*` API Key 权限边界粗 | Key 拥有者越权访问其他工作区 | 数据泄露 | API Key 关联 user_id 后复用 RBAC；P7 专项越权测试 |

---

## 8. 验收标准

设计完成 = 全部满足：

1. **路由覆盖**：§1.2 "IN 列表" 所有路由 100% 实现且通过集成测试。
2. **响应兼容**：所有路由在金样本回放下与 Node 端响应一致（允许 §6.3 白名单差异）。
3. **VectorDB 矩阵**：9 个 provider 全部通过五条标准用例（本地 6 个跑 CI，云 3 个手测金样本）。
4. **Agent runtime 等价**：P10 完成后，**前端不切 backend URL 即可直接使用**；Agent 会话可执行的工具集与 Node 端一致。
5. **单进程部署**：`backend` + `collector` 两个进程即可完整运行；server/ 下线后 staging 跑满 1 周无 P0/P1。
6. **测试与代码量门槛**：service 层 ≥ 70% / infra ≥ 80%；新增代码无 `go vet` / `staticcheck` warning。
7. **文档**：每个 Phase 收尾更新 `backend/README.md` 与 `AGENTS.md`；OpenAPI `/v1/*` 全覆盖。
8. **回滚预案**：每 Phase 上线前在反向代理层留一层"按路径分流"开关，30 秒内可回滚到 Node server。

---

## 9. 明确不做（范围外）

- `/v1/openai/*` OpenAI 兼容层
- `/api/community-hub/*`
- `/api/mobile/*`
- `/api/web-push/*`
- 多实例水平扩展、leader election、分布式锁
- 嵌入子进程级 OOM 隔离
- 性能 benchmark suite
- collector 进程 Go 化

---

*设计文档版本：v2.0*
*日期：2026-05-25*
*前版（v1.0）参见 git 历史 `8356d67`*
