# Server + Collector vs backend 功能覆盖度对比报告

> 生成时间: 2026-05-28
> 对比基准: `/Users/ranwei/Downloads/anything-llm-1.13.0/server/` (Node.js Express) + `/Users/ranwei/Downloads/anything-llm-1.13.0/collector/` (Node.js Express) vs `backend/` (Go/Gin)

---

## 总体结论

**backend 目前无法 100% 替换 server + collector。**

核心聊天、RAG、文档处理、Agent、API、认证等主干功能已基本对齐，但仍有 **约 15+ 个功能模块缺失或仅为 stub**。其中若干是 Hermind 的差异化卖点（Community Hub、Scheduled Jobs、Model Router、Telegram Bot、Native Embedding 等）。

| 维度 | 覆盖率估计 |
|------|-----------|
| 核心 API 端点 (REST) | ~85% |
| 核心功能模块 | ~80% |
| Collector 文档处理 | ~95% |
| Agent 技能/插件 | ~70% |
| 外围/增值功能 | ~40% |

---

## 一、功能模块逐一对照

### 1. 认证与用户管理 ✅ 基本对齐

| 功能 | server (Node) | backend | 状态 |
|------|--------------|-----------|------|
| JWT Session | ✅ | ✅ | 对齐 |
| 单用户/多用户模式 | ✅ | ✅ | 对齐 |
| 用户角色 (admin/manager/default) | ✅ | ✅ | 对齐 |
| 邀请码系统 | ✅ | ✅ | 对齐 |
| 密码重置/恢复码 | ✅ | ✅ | 对齐 |
| Simple SSO | ✅ | ✅ | 对齐 |
| 临时认证令牌 | ✅ | ✅ | 对齐 |
| 用户头像 (PFP) | ✅ | ✅ | 对齐 |
| 个人简介 (Bio) | ✅ | ✅ | 对齐 |
| **每日消息限额** | ✅ enforced | ⚠️ DB字段存在，未确认是否强制执行 | 部分 |
| 浏览器扩展 API Key | ✅ | ❌ stub (返回空列表+假key) | **缺失** |

### 2. Workspace 管理 ✅ 基本对齐

| 功能 | server (Node) | backend | 状态 |
|------|--------------|-----------|------|
| CRUD | ✅ | ✅ | 对齐 |
| 用户成员关系 | ✅ | ✅ | 对齐 |
| 按 workspace 的 LLM 配置 | ✅ | ✅ | 对齐 |
| 文档 Pin/Unpin | ✅ | ✅ | 对齐 |
| 相似度阈值/Top-N | ✅ | ✅ | 对齐 |
| 向量库重置 | ✅ | ✅ | 对齐 |
| 建议消息 | ✅ | ✅ | 对齐 |
| Workspace 头像 | ✅ | ✅ | 对齐 |
| **聊天记录 Fork** | ✅ | ❌ 无实现 | **缺失** |
| **Workspace 变更追踪** | ✅ | ❌ 无实现 | **缺失** |
| **自动重命名 Thread** | ✅ | ❌ 无实现 | **缺失** |

### 3. 聊天/LLM ✅ 高度对齐

| 功能 | server (Node) | backend | 状态 |
|------|--------------|-----------|------|
| SSE 流式聊天 | ✅ | ✅ | 对齐 |
| Thread 聊天 | ✅ | ✅ | 对齐 |
| Chat/Query/Automatic 模式 | ✅ | ✅ | 对齐 |
| RAG + 向量检索 | ✅ | ✅ | 对齐 |
| Reranking | ✅ | ✅ | 对齐 |
| 聊天编辑/更新 | ✅ | ✅ | 对齐 |
| 反馈评分 | ✅ | ✅ | 对齐 |
| TTS (ElevenLabs/OpenAI/Native) | ✅ | ✅ | 对齐 |
| OpenAI 兼容 API | ✅ | ✅ | 对齐 |
| API 向量存储端点 | ✅ | ✅ | 对齐 |
| 解析文件上下文注入 | ✅ | ✅ | 对齐 |

### 4. 文档管理 ✅ 高度对齐

| 功能 | server (Node) | backend | 状态 |
|------|--------------|-----------|------|
| 文件上传 | ✅ | ✅ | 对齐 |
| 链接抓取 | ✅ | ✅ | 对齐 |
| 原始文本 | ✅ | ✅ | 对齐 |
| 文件夹管理 | ✅ | ✅ | 对齐 |
| 文档移动/删除 | ✅ | ✅ | 对齐 |
| 元数据 | ✅ | ✅ | 对齐 |
| 支持的 MIME 查询 | ✅ | ✅ | 对齐 |
| 直接上传 (direct-uploads) | ✅ | ✅ | 对齐 |

### 5. Agent 运行时 ⚠️ 部分对齐，有显著差异

| 功能 | server (Node) | backend | 状态 |
|------|--------------|-----------|------|
| WebSocket 驱动 Agent | ✅ (AIbitat) | ✅ | 对齐 |
| 工具审批门 (Approval Gate) | ✅ | ✅ | 对齐 |
| 自动审批白名单 | ✅ | ✅ | 对齐 |
| Agent 会话 Telemetry | ✅ | ✅ | 对齐 |
| Bail 命令 | ✅ | ✅ | 对齐 |
| No-code Flow Builder | ✅ | ✅ | 对齐 |
| **多 Agent 编排 (AIbitat multi-agent)** | ✅ | ❌ 单 Agent only | **缺失** |
| **导入社区插件 (Hub plugins)** | ✅ | ❌ 无支持 | **缺失** |
| **Filesystem Agent** | ✅ 可用 | ⚠️ stub (handler 返回 `available: false`) | **缺失** |
| **Create Files Agent** | ✅ 可用 | ⚠️ stub (handler 返回 `available: false`) | **缺失** |

**Built-in Agent Skills 对比:**

| Skill | server | backend |
|-------|--------|-----------|
| RAG Memory | ✅ | ✅ |
| Doc Summarizer | ✅ | ✅ |
| Web Scraping | ✅ | ✅ |
| SQL Agent (MySQL/Postgres/SQLite/MSSQL) | ✅ | ✅ |
| Rechart | ✅ | ✅ |
| Gmail | ✅ | ✅ |
| Google Calendar | ✅ | ✅ |
| Outlook (15 actions) | ✅ | ✅ (V3C 新增) |
| Filesystem Agent | ✅ | ❌ stub |
| Create Files (docx/pdf/pptx/xlsx/txt) | ✅ | ❌ stub |
| HTTP Socket | ✅ | ❌ |
| Router Classifier | ✅ | ❌ |
| Request User Input | ✅ | ❌ |
| CLI | ✅ | ❌ |
| Websocket | ✅ | ❌ |
| MCP Tools (动态加载) | ✅ | ✅ |

### 6. 系统/管理 ✅ 基本对齐

| 功能 | server (Node) | backend | 状态 |
|------|--------------|-----------|------|
| 系统设置 KV | ✅ | ✅ | 对齐 |
| 事件日志 | ✅ | ✅ | 对齐 |
| Logo/品牌 | ✅ | ✅ | 对齐 |
| API Key 管理 | ✅ | ✅ | 对齐 |
| 用户管理 (admin) | ✅ | ✅ | 对齐 |
| Onboarding | ✅ | ✅ | 对齐 |
| 自定义模型 | ✅ | ✅ | 对齐 |
| Prompt Presets | ✅ | ✅ | 对齐 |
| Prompt Variables | ✅ | ✅ | 对齐 |
| Metrics 端点 | ✅ | ✅ | 对齐 |
| **Prompt History 审计** | ✅ | ❌ 无模型 | **缺失** |
| **Slash Command Presets** | ✅ | ❌ 无模型 | **缺失** |

### 7. Embed 小部件 ✅ 高度对齐

| 功能 | server (Node) | backend | 状态 |
|------|--------------|-----------|------|
| 配置管理 | ✅ | ✅ | 对齐 |
| 会话隔离 | ✅ | ✅ | 对齐 |
| 域名白名单 | ✅ | ✅ | 对齐 |
| 速率限制 | ✅ | ✅ | 对齐 |
| 聊天模式覆盖 | ✅ | ✅ | 对齐 |
| 消息限额 | ✅ | ✅ | 对齐 |

### 8. Vector DB ✅ 高度对齐

| Provider | server (Node) | backend | 状态 |
|----------|--------------|-----------|------|
| LanceDB | ✅ | ✅ (CGO, macOS 为 stub) | 对齐* |
| PGVector | ✅ | ✅ | 对齐 |
| Pinecone | ✅ | ✅ | 对齐 |
| Chroma | ✅ | ✅ | 对齐 |
| ChromaCloud | ✅ | ✅ | 对齐 |
| Weaviate | ✅ | ✅ | 对齐 |
| Qdrant | ✅ | ✅ | 对齐 |
| Milvus | ✅ | ✅ | 对齐 |
| Astra DB | ✅ | ✅ | 对齐 |
| Zilliz | ✅ | ✅ | 对齐 |

> *LanceDB 在 macOS/Windows 构建标签下是 stub，Linux 完整。

### 9. Embedding ⚠️ 部分对齐

| Provider | server (Node) | backend | 状态 |
|----------|--------------|-----------|------|
| OpenAI | ✅ | ✅ | 对齐 |
| Azure | ✅ | ✅ (openai-compat) | 对齐 |
| Ollama | ✅ | ✅ (openai-compat) | 对齐 |
| LM Studio | ✅ | ✅ (openai-compat) | 对齐 |
| LocalAI | ✅ | ✅ (openai-compat) | 对齐 |
| Cohere | ✅ | ✅ | 对齐 |
| VoyageAI | ✅ | ✅ | 对齐 |
| LiteLLM | ✅ | ✅ (openai-compat) | 对齐 |
| Mistral | ✅ | ✅ (openai-compat) | 对齐 |
| Gemini | ✅ | ✅ (openai-compat) | 对齐 |
| OpenRouter | ✅ | ✅ (openai-compat) | 对齐 |
| Lemonade | ✅ | ✅ (openai-compat) | 对齐 |
| Generic OpenAI | ✅ | ✅ (openai-compat) | 对齐 |
| **Native (Xenova/transformers ONNX)** | ✅ **本地 CPU 离线** | ❌ **完全没有** | **重大缺失** |

> Native embedding 是 server 的核心差异化功能：在纯 CPU 环境下本地运行 embedding，无需外部 API，隐私性强。backend 完全依赖 Pantheon（OpenAI 兼容协议），没有 ONNX 本地嵌入能力。

### 10. 后台任务 ⚠️ 部分对齐

| 任务 | server (Node, Bree) | backend (cron) | 状态 |
|------|---------------------|------------------|------|
| 孤儿文档清理 | ✅ | ✅ | 对齐 |
| 生成文件清理 | ✅ | ✅ | 对齐 |
| 监控文档同步 | ✅ | ✅ | 对齐 |
| Embedding Worker | ✅ | ✅ | 对齐 |
| **Memory Extraction** | ✅ | ❌ | **缺失** |
| **Scheduled Agent Jobs** | ✅ (cron 驱动) | ❌ (worker 框架存在，无 scheduled job 模型/逻辑) | **重大缺失** |

> backend 的 `workers` 包有 Manager + cron 调度框架，但只注册了 4 个固定任务。server 有完整的 `scheduled_jobs` / `scheduled_job_runs` 表和 UI，支持用户配置 cron 表达式来定时运行 Agent。

### 11. MCP ✅ 高度对齐

| 功能 | server (Node) | backend | 状态 |
|------|--------------|-----------|------|
| MCP 服务器管理 | ✅ | ✅ | 对齐 |
| Stdio/SSE/HTTP 传输 | ✅ | ✅ | 对齐 |
| 动态工具→插件转换 | ✅ | ✅ | 对齐 |
| 工具启用/禁用 | ✅ | ✅ | 对齐 |
| 并发调用限制 | ✅ | ✅ | 对齐 |
| 强制重载 | ✅ | ✅ | 对齐 |

### 12. OAuth / 外部集成 ⚠️ 部分对齐

| 功能 | server (Node) | backend | 状态 |
|------|--------------|-----------|------|
| Outlook OAuth2 | ✅ | ✅ (V3C 完善) | 对齐 |
| Gmail (API Key) | ✅ | ✅ | 对齐 |
| Google Calendar | ✅ | ✅ | 对齐 |

### 13. API v1 (开发者 API) ✅ 高度对齐

| 功能 | server (Node) | backend | 状态 |
|------|--------------|-----------|------|
| API Key 认证 | ✅ | ✅ | 对齐 |
| Workspace CRUD | ✅ | ✅ | 对齐 |
| Document CRUD | ✅ | ✅ | 对齐 |
| Thread CRUD | ✅ | ✅ | 对齐 |
| Chat (sync + stream) | ✅ | ✅ | 对齐 |
| OpenAI 兼容端点 | ✅ | ✅ | 对齐 |
| Embeddings | ✅ | ✅ | 对齐 |
| Vector Search | ✅ | ✅ | 对齐 |
| System 设置导出 | ✅ | ✅ | 对齐 |

### 14. Telegram Bot ❌ 完全缺失

| 功能 | server (Node) | backend | 状态 |
|------|--------------|-----------|------|
| Webhook 接收 | ✅ 完整实现 | ❌ stub handler，所有端点返回空/假数据 | **完全缺失** |
| Bot 命令处理 | ✅ | ❌ | **缺失** |
| 用户审批/菜单 | ✅ | ❌ | **缺失** |
| 消息队列 | ✅ | ❌ | **缺失** |

### 15. Browser Extension ❌ 完全缺失

| 功能 | server (Node) | backend | 状态 |
|------|--------------|-----------|------|
| API Key 管理 | ✅ | ❌ stub | **缺失** |
| 通过扩展聊天 | ✅ | ❌ (无路由) | **缺失** |
| 扩展上下文聊天历史 | ✅ | ❌ (无路由) | **缺失** |

### 16. 其他外围功能 ❌ 大量缺失

| 功能 | server (Node) | backend | 状态 |
|------|--------------|-----------|------|
| **Model Router** (条件 LLM 路由) | ✅ | ❌ | **重大缺失** |
| **Community Hub** (插件下载/导入) | ✅ | ❌ | **重大缺失** |
| **Web Push** (VAPID) | ✅ | ❌ (DB字段存在，无实现) | **缺失** |
| **Memories** (全局/Workspace 长期记忆) | ✅ 独立 CRUD | ⚠️ 仅有 `rag_memory` tool，无独立系统 | **部分缺失** |
| **Mobile Device Pairing** | ✅ | ❌ | **缺失** |
| **Live Sync** (实验性) | ✅ | ❌ | **缺失** |
| **Prompt History** | ✅ | ❌ | **缺失** |
| **Slash Command Presets** | ✅ | ❌ | **缺失** |
| **Document Sync Executions** | ✅ | ⚠️ 仅有 queue 模型，无 execution 追踪 | **部分缺失** |
| **Telemetry (PostHog)** | ✅ | ⚠️ agent 有 telemetry，系统级不完整 | **部分缺失** |
| **Swagger/OpenAPI 自动生成** | ✅ (swagger-autogen) | ❌ | **缺失** |
| **EncryptionManager** (字段级加密) | ✅ | ✅ | 对齐 |

### 17. LLM Provider 数量

| | server (Node) | backend |
|--|--------------|-----------|
| 总数 | ~35 | ~40+ |
| 独有 | Hermind Router | Qwen, Wenxin, Zhipu |

> backend 的 provider 列表实际上**更多**，包含了 server 没有的国产模型（通义千问、文心一言、智谱）。缺失的是 `hermind-router`（Model Router 的配套 provider）。

---

## 二、Collector 文档处理对比

### backend Collector 已实现的

| 能力 | 状态 |
|------|------|
| 文本文件 (txt/md/org/adoc/rst/csv/json/html) | ✅ |
| PDF (数字提取 + Tesseract OCR fallback) | ✅ |
| DOCX | ✅ |
| XLSX | ✅ |
| PPTX | ✅ |
| ODT/ODP | ✅ |
| EPUB | ✅ |
| MBOX | ✅ |
| 图片 OCR (png/jpg/jpeg/webp, Tesseract) | ✅ |
| 音频转录 (mp3/wav/mp4/mpeg/ogg/oga/opus/m4a/webm, Whisper local + OpenAI) | ✅ |
| URL 抓取 (chromedp/Puppeteer 替代) | ✅ |
| YouTube 字幕 | ✅ |
| GitHub/GitLab Repo | ✅ |
| Confluence | ✅ |
| DrupalWiki | ✅ |
| Paperless-ngx | ✅ |
| Obsidian Vault | ✅ |
| Website Depth Crawling | ✅ |
| Resync (所有扩展类型) | ✅ |
| Token 估算 | ✅ |
| 临时文件安全写入 | ✅ |

### Collector 差异点

| 差异 | server (Node) | backend |
|------|--------------|-----------|
| 架构 | 独立进程 (port 8888)，HTTP API | 库内嵌 (in-process)，直接调用 |
| 安全签名 | RSA CommunicationKey + X-Integrity | ✅ 相同 RSA 实现保留 |
| 加密凭证 (resync) | AES-256-CBC | 需要确认 |
| 路径遍历防御 | ✅ normalizePath + isWithin | 需要确认 |
| 浏览器启动参数 | ✅ 自定义 Puppeteer args | ✅ 通过 chromedp |
| IP 限制绕过 | ✅ COLLECTOR_ALLOW_ANY_IP | ✅ 相同环境变量 |

**Collector 层面，backend 基本可 1:1 替换。** 唯一架构差异是从独立微服务变为库内嵌，这对单进程部署反而是优势。

---

## 三、关键缺失项影响评估

| 缺失项 | 严重程度 | 用户影响 | 实现复杂度 |
|--------|---------|---------|-----------|
| **Model Router** | 🔴 高 | 高级用户无法做条件路由 | 中 (需新表+规则引擎) |
| **Scheduled Jobs** | 🔴 高 | 无法定时运行 Agent | 中 (已有 worker 框架，缺表+UI) |
| **Native Embedding** | 🔴 高 | 无法离线 CPU embedding，必须依赖外部 API | 高 (需 ONNX Runtime + Go 绑定) |
| **Community Hub** | 🟡 中 | 无法下载社区插件 | 高 (需 Hub 协议+下载+导入) |
| **Telegram Bot** | 🟡 中 | Telegram 集成完全不可用 | 中-高 |
| **Browser Extension** | 🟡 中 | Chrome 扩展不可用 | 中 |
| **Memories 独立系统** | 🟡 中 | 无长期记忆管理 UI | 低-中 |
| **Web Push** | 🟢 低 | 无浏览器推送 | 低 |
| **Prompt History** | 🟢 低 | 无 prompt 变更审计 | 低 |
| **Chat Fork** | 🟢 低 | 无法从某点分叉对话 | 低 |
| **Swagger 自动生成** | 🟢 低 | 无在线 API 文档 | 低 |
| **Hermind Router** | 🟡 中 | 缺少自有路由 provider | 中 |
| **Filesystem Agent** | 🟡 中 | 文件系统 Agent 技能不可用 | 中 |
| **Create Files Agent** | 🟡 中 | 生成文件技能不可用 | 中 (代码存在但 handler 标记为 stub) |
| **多 Agent 编排** | 🟡 中 | 只能单 Agent 运行 | 高 |

---

## 四、结论

### 能否 100% 替换？

**否。** 在以下场景下，backend **不能**直接替换：

1. 用户依赖 **Model Router** 做条件 LLM 路由
2. 用户需要 **Scheduled Jobs** 定时运行 Agent
3. 用户需要 **离线本地 Embedding** (Xenova/ONNX)
4. 用户需要 **Telegram Bot** 或 **Browser Extension**
5. 用户活跃使用 **Community Hub** 插件
6. 用户需要 **多 Agent 同时协作** (AIbitat multi-agent)
7. 需要 **Swagger/OpenAPI 自动生成文档**

### 在哪些场景下可以替换？

如果用户只使用以下功能，backend **基本可以替换**：

- 标准聊天 + RAG (workspace 文档)
- Thread 对话
- 文档上传/链接抓取
- 基础 Agent 技能 (Gmail, Calendar, Outlook, Web Scraping, SQL, MCP)
- API v1 开发者接口
- Embed 小部件
- 多用户管理
- 标准 Vector DB (LanceDB/PGVector 等)

### 建议的迁移路径

1. **Phase 1 (已就绪)**: 核心聊天、文档、Agent、API — 可直接切换
2. **Phase 2 (中优先级)**: Scheduled Jobs、Memories 系统、Filesystem/Create Files Agent、Prompt History
3. **Phase 3 (低优先级)**: Telegram、Browser Extension、Web Push、Live Sync
4. **Phase 4 (高难度)**: Model Router、Community Hub、Native Embedding (ONNX)、多 Agent 编排

---

## 五、Phase 2 源码级详细分析

以下对 Phase 2 四个模块进行源码级别的拆解，包含完整的接口定义、依赖注入链、关键函数签名、数据流分析，以及每个缺口在代码中的精确位置。

---

### 5.1 Scheduled Jobs（定时任务）

#### 5.1.1 源码结构总览

**Job 接口**（`backend/internal/workers/job.go`，21 行）——所有后台任务的统一抽象：

```go
type Job interface {
    Name() string
    Schedule() string          // cron 表达式或 "@every 12h"，空字符串 = 不自动调度
    Enabled(ctx context.Context) bool
    Run(ctx context.Context) error
}
```

**Manager 实现**（`backend/internal/workers/manager.go`，175 行）：

```go
type Manager struct {
    cron  *cron.Cron
    jobs  []Job
    db    *gorm.DB
    cfg   *config.Config
    wg    sync.WaitGroup
    mu    sync.RWMutex
    state State          // Booting / Running / Stopping / Stopped
}

func NewManager(db *gorm.DB, cfg *config.Config) *Manager
func (m *Manager) Register(jobs ...Job)          // 追加到 m.jobs
func (m *Manager) Start() error                   // 遍历 jobs，非空 schedule → cron.AddFunc；空 → on-demand
func (m *Manager) Stop(ctx context.Context) error // 停止 cron，等待 wg
func (m *Manager) Trigger(name string) error      // 按 name 查找并手动触发（go m.wrapJob(job)()）
```

`wrapJob()` 提供了完整的执行包装：
- `Enabled()` 前置检查
- `sync.WaitGroup` 追踪
- `panic` recovery
- 按 job name 硬编码的 timeout 解析（仅支持 3 个系统任务 + 默认 5m）
- `mlog` 结构化日志记录 start/finish/error

**现有 4 个 Job 实现**：

| Job | 文件 | Schedule | Enabled 条件 | 依赖 |
|-----|------|----------|-------------|------|
| `cleanup-orphan-documents` | `cleanup_orphan.go` | `cfg.WorkerCleanupOrphanInterval` (默认 `"0 */12 * * *"`) | `cfg.WorkerCleanupOrphanEnabled` | `db`, `cfg` |
| `cleanup-generated-files` | `cleanup_generated.go` | `cfg.WorkerCleanupGeneratedInterval` (默认 `"0 */8 * * *"`) | `cfg.WorkerCleanupGeneratedEnabled` | `db`, `cfg` |
| `sync-watched-documents` | `sync_watched.go` | `cfg.WorkerSyncWatchedInterval` (默认 `"0 * * * *"`) | `cfg.WorkerSyncWatchedEnabled` **&&** `DocumentSyncQueue` 有待同步项 | `db`, `cfg`, `collector.Client` |
| `embed-worker` | `embed_worker.go` | `""` (on-demand only) | `emb != nil && vectorDB != nil` | `db`, `cfg`, `embedder.Embedder`, `vectordb.VectorDatabase` |

**依赖注入链**（`backend/cmd/server/main.go` 第 185–194 行）：

```go
workerMgr := workers.NewManager(db, cfg)
workerMgr.Register(
    workers.NewCleanupOrphanJob(db, cfg),
    workers.NewCleanupGeneratedJob(db, cfg),
    workers.NewSyncWatchedJob(db, cfg, coll),
    workers.NewEmbedWorkerJob(db, cfg, emb, vectorDB),
)
if err := workerMgr.Start(); err != nil {
    mlog.Fatal("failed to start worker manager", mlog.Err(err))
}
```

**Agent Runtime 可用性**（同文件第 168–195 行）：

```go
agentRuntime := agent.NewRuntime(agent.Deps{DB: db, Cfg: cfg, ...})
// ...
chatSvc := services.NewChatService(db, cfg, vectorSvc, llmProv, emb, agentRuntime, rerankerSvc)
```

`agentRuntime` 在 `main()` 中早于 `workerMgr` 初始化，因此 **Scheduled Job 可直接复用 `agentRuntime` 启动 Agent 会话**，无需新增运行时实例。

#### 5.1.2 Gap 分析（精确到代码位置）

| 缺口 | 位置 | 说明 |
|------|------|------|
| 无 `ScheduledJob` 模型 | `backend/internal/models/` — 不存在 | 需要 GORM 模型：`id`, `name`, `workspace_id`, `cron_expr`, `prompt_preset_id`, `system_prompt`, `enabled`, `created_at`, `updated_at` |
| 无 `ScheduledJobRun` 模型 | `backend/internal/models/` — 不存在 | 需要 GORM 模型：`id`, `scheduled_job_id`, `status`, `started_at`, `finished_at`, `output`, `error_msg` |
| AutoMigrate 未注册 | `backend/internal/services/db.go` 第 30–54 行 | 当前仅注册 18 个模型，无 `ScheduledJob` / `ScheduledJobRun` |
| 无服务层 | `backend/internal/services/` — 不存在 | 无 `scheduled_job_service.go` |
| 无 REST handler | `backend/internal/handlers/` — 不存在 | 无 `scheduled_job_handler.go` |
| Manager 不支持 DB 加载 | `manager.go` 第 51–76 行 | `Start()` 仅遍历硬编码的 `m.jobs`，没有从 DB 读取并动态注册的逻辑 |
| Manager 无热重载 | `manager.go` 第 45–76 行 | `Register()` 仅在 boot 阶段调用，运行中无法增删改 job schedule |
| `wrapJob` timeout 硬编码 | `manager.go` 第 149–164 行 | switch-case 只识别 3 个系统任务名，新增 job 会回退到默认 5m |

#### 5.1.3 实现路径建议

1. **模型**：新增 `backend/internal/models/scheduled_job.go` 和 `scheduled_job_run.go`，字段对齐 Node 的 `scheduled_jobs` / `scheduled_job_runs` 表
2. **AutoMigrate**：在 `db.go` 第 31 行数组中追加 `&models.ScheduledJob{}`、`&models.ScheduledJobRun{}`
3. **服务层**：`backend/internal/services/scheduled_job_service.go` — CRUD + `Trigger(jobID)` + `ListRuns(jobID)`
4. **Manager 扩展**：
   - 新增 `LoadFromDB(ctx)` 方法：查询 `ScheduledJob` 表中 `enabled=true` 的记录，转换为匿名 `Job` 实现并 `Register()`
   - 新增 `Reload(ctx)` 方法：Stop cron → 清空 jobs → 重新 LoadFromDB + 硬编码系统 jobs → Start()
   - 修改 `wrapJob()` timeout 逻辑：从 `switch-case` 改为读取 `ScheduledJob.Timeout` 字段
5. **REST handler**：`GET/POST/PUT/DELETE /api/scheduled-jobs`，`POST /api/scheduled-jobs/:id/trigger`，`GET /api/scheduled-jobs/:id/runs`
6. **Agent 桥接**：Job 的 `Run()` 方法中调用 `agentRuntime.StartSession(...)` 启动一次完整的 Agent 会话

---

### 5.2 Memories（长期记忆系统）

#### 5.2.1 源码结构总览

**RAG Memory 工具**（`backend/internal/agent/tools/rag_memory.go`，94 行）：

```go
func NewRAGMemorySkill(tc *ToolContext) *tool.Entry {
    return &tool.Entry{
        Name:        "rag-memory",
        Toolset:     "memory",
        Description: "Search local documents or store information to long-term memory...",
        Emoji:       "🧠",
        MaxResultChars: 8 * 1024,
        CheckFn:     func() bool { return tc.VectorSearchSvc != nil },
        Schema:      core.ToolDefinition{...},
        Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
            switch args.Action {
            case "search": return ragMemorySearch(ctx, tc, args.Content)
            case "store":  return ragMemoryStore(ctx, tc, args.Content)
            }
        },
    }
}
```

**`ragMemorySearch` 实现**（第 46–76 行）：

```go
func ragMemorySearch(ctx context.Context, tc *ToolContext, content string) (string, error) {
    results, err := tc.VectorSearchSvc.Search(ctx, tc.Workspace, dto.VectorSearchRequest{
        Query: content,
        TopN:  intPtr(4),
    })
    // 构造 JSON 结果：text, score, source
}
```

**`ragMemoryStore` 实现**（第 78–81 行）——**纯 stub**：

```go
func ragMemoryStore(ctx context.Context, tc *ToolContext, content string) (string, error) {
    tc.Emit("Memory store request acknowledged (deferred)")
    return tool.Result(map[string]any{
        "status": "deferred",
        "note":   "store action is not yet implemented",
    }), nil
}
```

**`VectorSearcher` 接口**（`backend/internal/agent/tools/context.go` 第 31–33 行）：

```go
type VectorSearcher interface {
    Search(ctx context.Context, ws *models.Workspace, req dto.VectorSearchRequest) ([]dto.VectorSearchResult, error)
}
```

> 注意：接口**只有 `Search` 方法，没有 `Store`/`Index` 方法**。这意味着即使新增 Memory 模型，工具层也无法直接调用向量写入——需要扩展接口或走 DocumentService 的已有路径。

**`ToolContext` 结构**（`context.go` 第 41–55 行）：

```go
type ToolContext struct {
    Ctx             context.Context
    Workspace       *models.Workspace
    User            *models.User
    Settings        map[string]string
    LM              core.LanguageModel
    VectorSearchSvc VectorSearcher   // ← 仅 search，无 store 能力
    DocSvc          DocumentLister
    MCPHv           MCPHypervisor
    FlowSvc         *services.AgentFlowService
    EventLog        EventLogger
    Emit            StatusEmitter
    Approval        ApprovalFn
    Cfg             *config.Config
}
```

**Builder 注册链**（`builder.go` 第 86–103 行）：

```go
for _, e := range []*tool.Entry{
    NewRAGMemorySkill(tc),       // 第 88 行
    NewDocSummarizerSkill(tc),
    NewWebScrapingSkill(tc),
    NewRechartSkill(tc),
    NewSQLAgentSkill(tc),
    NewFilesystemAgentSkill(tc),
    NewCreateFilesAgentSkill(tc),
    // ...
} {
    b.addWithApproval(reg, seen, e, "default", false, globalAutoApprove, whitelist)
}
```

`rag-memory` 作为 default skill 注册，**不需要 approval**（`requiresApproval=false`），且不受 whitelist 限制。

#### 5.2.2 Gap 分析（精确到代码位置）

| 缺口 | 位置 | 说明 |
|------|------|------|
| `ragMemoryStore` 为 stub | `rag_memory.go` 第 78–81 行 | 不写入任何持久化存储，仅返回 deferred 提示 |
| 无 `Memory` GORM 模型 | `backend/internal/models/` — 不存在 | 需要独立表存储记忆文本内容和元数据 |
| 无向量写入接口 | `context.go` 第 31–33 行 | `VectorSearcher` 只有 `Search`，没有 `Store`/`Index` |
| AutoMigrate 未注册 | `db.go` 第 30–54 行 | 无 `Memory` 模型 |
| 无服务层 | `backend/internal/services/` — 不存在 | 无 `memory_service.go` |
| 无 REST handler | `backend/internal/handlers/` — 不存在 | 无 `memory_handler.go` |

#### 5.2.3 实现路径建议

1. **模型**：新增 `backend/internal/models/memory.go`：
   ```go
   type Memory struct {
       ID          int `gorm:"primaryKey"`
       WorkspaceID int
       Content     string
       Metadata    *string  // JSON
       CreatedAt   time.Time
       UpdatedAt   time.Time
   }
   ```
2. **AutoMigrate**：在 `db.go` 中追加 `&models.Memory{}`
3. **服务层**：`backend/internal/services/memory_service.go` — CRUD + `Search(workspaceID, query)`（复用 `VectorSearchSvc`）
4. **向量写入路径选择**：
   - 方案 A：扩展 `VectorSearcher` 接口增加 `Store(...)` 方法，需要修改 `VectorSearchService` 实现
   - 方案 B：复用现有 `DocumentService` 的路径——将 memory content 作为虚拟文档写入 `WorkspaceDocument` + `DocumentVector` 表，自动进入 embedding pipeline
   - **推荐方案 B**：零接口改动，复用现有向量化基础设施
5. **实现 `ragMemoryStore`**：
   - 写入 `memories` 表
   - 同步创建 `WorkspaceDocument` 记录（标记 `source="memory"`），触发 `EmbedWorkerJob.Enqueue()`
6. **REST handler**：`GET/POST/PUT/DELETE /api/workspaces/:id/memories`

---

### 5.3 Filesystem Agent & Create Files Agent（文件系统与文件生成 Agent 技能）

#### 5.3.1 源码结构总览

**Filesystem Agent 工具**（`backend/internal/agent/tools/filesystem_agent.go`，332 行）：

```go
func NewFilesystemAgentSkill(tc *ToolContext) *tool.Entry {
    return &tool.Entry{
        Name:        "filesystem-agent",
        Toolset:     "filesystem",
        CheckFn:     func() bool { return tc.Cfg != nil && tc.Cfg.AgentFilesystemEnabled },
        MaxResultChars: 16 * 1024,
        Schema:      core.ToolDefinition{...},
        Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
            root := tc.Cfg.AgentFilesystemRoot
            // Approval gate for destructive actions
            switch args.Action {
            case "list_dir":     return fsListDir(tc, root, args.Path)
            case "read_file":    return fsReadFile(tc, root, args.Path)
            case "get_info":     return fsGetInfo(tc, root, args.Path)
            case "search_files": return fsSearchFiles(tc, root, args.Path, args.Pattern)
            case "write_file":   return fsWriteFile(tc, root, args.Path, args.Content)
            case "edit_file":    return fsEditFile(tc, root, args.Path, args.OldString, args.NewString)
            case "move_file":    return fsMoveFile(tc, root, args.Source, args.Destination)
            case "copy_file":    return fsCopyFile(tc, root, args.Source, args.Destination)
            case "create_dir":   return fsCreateDir(tc, root, args.Path)
            }
        },
    }
}
```

**9 个 action 全部实现**，关键特性：
- **安全沙箱**：`safeJoin()`（`filesystem_safejoin.go`，47 行）防御 `../`、绝对路径、symlink escape
- **Approval gate**：destructive action（write/edit/move/copy/create）通过 `tc.Approval` 请求用户确认
- **读取限制**：`fsMaxReadBytes = 1 << 20`（1 MiB），超大文件自动截断并标记 `truncated: true`
- **编辑安全**：`fsEditFile` 检测 old_string 出现次数——0 次报错，1 次替换，>1 次报错（避免歧义替换）

**safeJoin 实现**（`filesystem_safejoin.go` 第 19–47 行）：

```go
func safeJoin(root, userPath string) (string, error) {
    if filepath.IsAbs(userPath) { return "", fmt.Errorf("%w: absolute paths not allowed", ErrPathEscape) }
    absRoot, _ := filepath.Abs(root)
    resolvedRoot, _ := filepath.EvalSymlinks(absRoot)
    joined := filepath.Join(resolvedRoot, userPath)
    cleaned := filepath.Clean(joined)
    if !strings.HasPrefix(cleaned, resolvedRoot+string(filepath.Separator)) && cleaned != resolvedRoot {
        return "", fmt.Errorf("%w: %s outside %s", ErrPathEscape, cleaned, resolvedRoot)
    }
    // 再检查 symlink 解析后的目标
    if final, err := filepath.EvalSymlinks(cleaned); err == nil {
        if !strings.HasPrefix(final, resolvedRoot+string(filepath.Separator)) && final != resolvedRoot {
            return "", fmt.Errorf("%w: symlink target %s outside %s", ErrPathEscape, final, resolvedRoot)
        }
        return final, nil
    }
    return cleaned, nil
}
```

**safeJoin 测试覆盖**（`filesystem_safejoin_test.go`，69 行，5 个用例）：
- 正常路径 → 返回绝对路径 ✅
- `../etc/passwd` → `ErrPathEscape` ✅
- `/etc/passwd` → `ErrPathEscape` ✅
- symlink escape → `ErrPathEscape` ✅
- 嵌套路径 → 允许 ✅
- 空路径 → 视为 root ✅

**Create Files Agent 工具**（`backend/internal/agent/tools/create_files_agent.go`，153 行）：

```go
func NewCreateFilesAgentSkill(tc *ToolContext) *tool.Entry {
    return &tool.Entry{
        Name:        "create-files-agent",
        Toolset:     "create-files",
        CheckFn:     func() bool { return tc.Cfg != nil && tc.Cfg.AgentCreateFilesEnabled },
        MaxResultChars: 2 * 1024,
        Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
            switch args.Format {
            case "txt", "md":   os.WriteFile(dst, []byte(contentStr), 0o644)
            case "docx":        writeDocxFile(ctx, dst, contentStr, args.Filename)
            case "pdf":         writePDFFile(ctx, dst, contentStr, args.Filename)
            case "xlsx":        writeXLSXFile(ctx, dst, args.Content)
            case "pptx":        return tool.Error("pptx format not supported...")
            }
        },
    }
}
```

**文件名安全处理**：`sanitiseFilename()`（第 139–153 行）只允许 `a-zA-Z0-9-_`，截断至 64 字符，最终文件名格式：`<sanitised>-<timestamp>-<uuid8>.<format>`。

**Config 初始化**（`backend/internal/config/config.go` 第 289–334 行）：

```go
type Config struct {
    // ...
    AgentFilesystemEnabled  bool   `env:"AGENT_FILESYSTEM_ENABLED"  envDefault:"true"`
    AgentFilesystemRoot     string `env:"AGENT_FILESYSTEM_ROOT"`      // 空 → <StorageDir>/hermind-fs
    AgentCreateFilesEnabled bool   `env:"AGENT_CREATE_FILES_ENABLED" envDefault:"true"`
    AgentCreateFilesDir     string `env:"AGENT_CREATE_FILES_DIR"`     // 空 → <StorageDir>/generated-files
}
```

初始化时自动创建目录（第 333–334 行）：
```go
_ = os.MkdirAll(cfg.AgentFilesystemRoot, 0755)
_ = os.MkdirAll(cfg.AgentCreateFilesDir, 0755)
```

**Builder 注册**（`builder.go` 第 93–94 行）：
```go
NewFilesystemAgentSkill(tc),   // 第 93 行
NewCreateFilesAgentSkill(tc),  // 第 94 行
```

两者均作为 default skill 注册，`requiresApproval=false`。

**单元测试**：
- `filesystem_agent_test.go` — 覆盖所有 9 个 action
- `create_files_agent_test.go` — 覆盖 txt/md/docx/pdf/xlsx
- `builder_test.go` — 验证 Builder 正确注册
- `filesystem_safejoin_test.go` — 5 个安全用例

#### 5.3.2 Availability Handler 缺口（唯一阻塞点）

`backend/internal/handlers/agent_skill.go`：

```go
type AgentSkillHandler struct {
    sysSvc *services.SystemService   // ← 仅有 SystemService，无 cfg
}

func NewAgentSkillHandler(sysSvc *services.SystemService) *AgentSkillHandler {
    return &AgentSkillHandler{sysSvc: sysSvc}
}

func (h *AgentSkillHandler) FileSystemAgentAvailable(c *gin.Context) {
    c.JSON(http.StatusOK, gin.H{"available": false})   // 第 24 行：硬编码 false
}
func (h *AgentSkillHandler) CreateFilesAgentAvailable(c *gin.Context) {
    c.JSON(http.StatusOK, gin.H{"available": false})   // 第 29 行：硬编码 false
}
```

**依赖注入链**（`backend/cmd/server/main.go` 第 248 行）：
```go
handlers.RegisterAgentSkillRoutes(api, sysSvc, authSvc)
//                              ↑ 只传了 sysSvc 和 authSvc，cfg 未传入
```

```go
func RegisterAgentSkillRoutes(r *gin.RouterGroup, sysSvc *services.SystemService, authSvc *services.AuthService) {
    h := NewAgentSkillHandler(sysSvc)   // ← 这里也没有 cfg
    r.GET("/agent-skills/filesystem-agent/is-available", middleware.ValidatedRequest(authSvc), h.FileSystemAgentAvailable)
    r.GET("/agent-skills/create-files-agent/is-available", middleware.ValidatedRequest(authSvc), h.CreateFilesAgentAvailable)
    r.POST("/agent-skills/whitelist/add", middleware.ValidatedRequest(authSvc), middleware.FlexUserRoleValid([]string{"all"}), h.AddToWhitelist)
}
```

#### 5.3.3 实现路径建议

这是 Phase 2 中**最接近可用**的功能。工具逻辑、安全沙箱、测试、配置开关全部就绪，**仅 availability handler 硬编码 `false`**。

**修改范围（约 4 处，10 行以内）：**

1. **`agent_skill.go`**：为 `AgentSkillHandler` 注入 `cfg`：
   ```go
   type AgentSkillHandler struct {
       sysSvc *services.SystemService
       cfg    *config.Config   // 新增
   }
   func NewAgentSkillHandler(sysSvc *services.SystemService, cfg *config.Config) *AgentSkillHandler {
       return &AgentSkillHandler{sysSvc: sysSvc, cfg: cfg}
   }
   ```
2. **`agent_skill.go`**：修改两个 handler：
   ```go
   func (h *AgentSkillHandler) FileSystemAgentAvailable(c *gin.Context) {
       c.JSON(http.StatusOK, gin.H{"available": h.cfg != nil && h.cfg.AgentFilesystemEnabled})
   }
   func (h *AgentSkillHandler) CreateFilesAgentAvailable(c *gin.Context) {
       c.JSON(http.StatusOK, gin.H{"available": h.cfg != nil && h.cfg.AgentCreateFilesEnabled})
   }
   ```
3. **`agent_skill.go`**：更新 `RegisterAgentSkillRoutes` 签名：
   ```go
   func RegisterAgentSkillRoutes(r *gin.RouterGroup, sysSvc *services.SystemService, authSvc *services.AuthService, cfg *config.Config)
   ```
4. **`main.go` 第 248 行**：传入 `cfg`：
   ```go
   handlers.RegisterAgentSkillRoutes(api, sysSvc, authSvc, cfg)
   ```

5. **验证**：`go test ./internal/handlers/... ./internal/agent/tools/...`

---

### 5.4 Prompt History（Prompt 变更审计）

#### 5.4.1 源码结构总览

**完全空白**。`backend/` 目录下无任何 `prompt_history`、`promptHistory`、`PromptHistory` 引用。

通过全局搜索确认：
- `backend/internal/models/` — 无相关结构体
- `backend/internal/services/` — 无相关服务
- `backend/internal/handlers/` — 无相关 handler
- `backend/internal/services/db.go` — `AutoMigrate` 未注册

现有的相关代码仅存在于 Prompt Preset 模块：

**PromptPreset 模型**（`backend/internal/models/prompt_preset.go`）：
```go
type PromptPreset struct {
    ID          int    `gorm:"primaryKey"`
    WorkspaceID *int
    Name        string
    SystemPrompt string
    Temperature  *float64
    TopP         *float64
    Model        *string
    // ... 无 history 关联
}
```

**PromptPresetService**（`backend/internal/services/prompt_preset_service.go`）——标准的 CRUD 服务，无审计 hook。

**PromptPreset Handler**（`backend/internal/handlers/prompt_preset.go`）——`POST /api/prompt-presets` 创建、`PUT /api/prompt-presets/:id` 更新，**无变更记录**。

#### 5.4.2 Gap 分析

| 缺口 | 说明 |
|------|------|
| 无 GORM 模型 | 需要 `PromptHistory` 表 |
| 无服务层 | 需要 `PromptHistoryService` |
| 无 REST API | 需要 `GET /api/prompt-history` |
| 无审计 hook | 需要在 `PromptPresetService.Update`、SystemSettings 修改等位置插入记录逻辑 |
| AutoMigrate 未注册 | 需在 `db.go` 中追加 |

#### 5.4.3 实现路径建议

1. **模型**：`backend/internal/models/prompt_history.go`：
   ```go
   type PromptHistory struct {
       ID         int    `gorm:"primaryKey"`
       EntityType string // "prompt_preset" | "system_setting" | "slash_command"
       EntityID   *int   // nullable for system-wide settings
       FieldName  string // "system_prompt" | "temperature" | "model" ...
       OldValue   *string
       NewValue   *string
       ChangedBy  *int   // user ID
       ChangeReason *string
       CreatedAt  time.Time
   }
   ```
2. **AutoMigrate**：`db.go` 中追加 `&models.PromptHistory{}`
3. **服务层**：`backend/internal/services/prompt_history_service.go`：
   - `Record(ctx, entityType, entityID, fieldName, oldValue, newValue, changedBy, reason)` — hook 形式
   - `List(ctx, filters, pagination)` — 查询
4. **审计 hook 插入点**：
   - `PromptPresetService.Update()` 调用前后
   - `SystemService.SetSetting()` 调用前后（针对 `system_prompt` 等关键 key）
5. **REST handler**：`GET /api/prompt-history?entity_type=&entity_id=&page=&page_size=`
6. **前端**：Prompt Preset 编辑页面增加 "History" Tab，展示变更时间线

---

### Phase 2 实施优先级建议

| 子项 | 实现复杂度 | 用户影响 | 建议顺序 | 关键工作量 |
|------|-----------|---------|---------|-----------|
| **Filesystem/Create Files Agent** | 🔵 极低（~10 行） | 高 | **#1** | 改 4 个位置：handler struct + 2 个方法 + Register 签名 + main.go 调用 |
| **Prompt History** | 🟢 低（~200 行） | 中 | **#2** | 纯新增：1 模型 + 1 服务 + 1 handler + 2 处 hook |
| **Memories 系统** | 🟡 低-中（~400 行） | 高 | **#3** | 1 模型 + 1 服务 + 1 handler + `ragMemoryStore` 重写 + 向量写入路径选择 |
| **Scheduled Jobs** | 🟡 中（~600 行） | 高 | **#4** | 2 模型 + 服务 + handler + Manager DB 加载/热重载 + Agent Runtime 桥接 + wrapJob timeout 泛化 |

---

*报告结束。*
