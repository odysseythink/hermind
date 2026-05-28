# AnythingLLM Go 后端重写设计文档

> 日期: 2026-05-22  
> 主题: 将 Node.js/Express 后端完全重写为 Go/Gin  
> 状态: 待实现  
> 关联: 前端零改动，API 完全兼容

---

## 1. 概述

将 AnythingLLM v1.12.1 的 Node.js/Express 后端（415 个 JS 文件、约 8.7 万行代码）完全重写为 Go。前端保持零改动，API 契约 100% 兼容。

### 1.1 当前后端规模

- **代码量**: 415 个 JS 文件，~87,000 行
- **数据模型**: Prisma schema 定义 28 个模型
- **端点模块**: 20+ 个 endpoint 文件
- **向量数据库**: 11 种（LanceDB, PGVector, Pinecone, Chroma, Weaviate, Qdrant, Milvus, Zilliz, Astra）
- **LLM 提供商**: 30+ 种
- **Agent 运行时**: 自定义 Agent + MCP + Flow Builder

---

## 2. 设计目标与约束

### 2.1 硬性约束

| 约束 | 说明 |
|------|------|
| API 完全兼容 | 所有 REST 端点、请求/响应 JSON 格式、HTTP 状态码、SSE 流式协议必须和 Node 后端完全一致 |
| 前端零改动 | `frontend/` 目录不做任何修改，构建产物通过 `//go:embed` 嵌入 Go 二进制 |
| 数据库兼容 | 支持 SQLite（默认）和 PostgreSQL，表结构通过 GORM 复刻 Prisma schema |
| 环境变量兼容 | `.env` 变量名与 Node 后端完全一致，用户切换时只需替换二进制 |

### 2.2 技术选型（已确认）

| 组件 | 选择 | 理由 |
|------|------|------|
| Web 框架 | **Gin** | 最流行，中间件机制最接近 Express |
| ORM | **GORM** | 模型定义最接近 Prisma，迁移工具成熟 |
| AI SDK | **Pantheon** (`github.com/odysseythink/pantheon`) | 统一 LLM + 嵌入 + Agent + 工具调用 |
| 日志 | **mlog** (`github.com/odysseythink/mlog`) | 结构化日志，零分配热路径，glog 兼容 |
| 数据库 | SQLite / PostgreSQL | GORM AutoMigrate |
| WebSocket | Gorilla WebSocket | 第二阶段 Agent 使用 |
| 配置 | 自定义 struct + `env` tag | 复刻所有 Node `.env` 变量 |

---

## 3. 项目结构

```
backend/
├── cmd/server/
│   └── main.go                    # 入口：加载配置、初始化 DB、启动 Gin、注册路由
├── internal/
│   ├── config/
│   │   └── config.go              # 环境变量配置结构体
│   ├── handlers/                  # HTTP handlers（对应 Node 的 endpoints/）
│   │   ├── system.go              # /api/ping, /api/system/*, /api/onboarding
│   │   ├── auth.go                # /api/login, /api/register, /api/request-token, /api/password-reset
│   │   ├── workspace.go           # /api/workspace/*
│   │   ├── chat.go                # /api/workspace/:slug/stream-chat (SSE)
│   │   ├── document.go            # /api/document/*
│   │   └── admin.go               # /api/admin/*
│   ├── middleware/
│   │   ├── auth.go                # validatedRequest（JWT/Session 解析）
│   │   ├── rbac.go                # flexUserRoleValid（角色权限检查）
│   │   └── workspace.go           # validWorkspaceSlug（工作区存在性校验）
│   ├── models/                    # GORM 模型（复刻 Prisma schema）
│   │   ├── user.go
│   │   ├── workspace.go
│   │   ├── workspace_chat.go
│   │   ├── workspace_document.go
│   │   ├── document_vector.go
│   │   ├── system_setting.go
│   │   ├── workspace_user.go
│   │   ├── workspace_thread.go
│   │   ├── invite.go
│   │   ├── api_key.go
│   │   ├── password_reset_token.go
│   │   └── recovery_code.go
│   ├── services/                  # 业务逻辑层
│   │   ├── auth_service.go        # 认证逻辑（单用户/多用户双模式）
│   │   ├── chat_service.go        # 聊天核心：上下文构建、Pantheon 调用、SSE 流式
│   │   ├── workspace_service.go   # 工作区 CRUD
│   │   ├── document_service.go    # 文档上传、解析状态跟踪
│   │   ├── vector_service.go      # 向量数据库抽象接口 + 嵌入调用
│   │   ├── system_service.go      # 系统设置读写 + 缓存
│   │   └── admin_service.go       # 用户管理、工作区管理
│   ├── providers/
│   │   └── llm.go                 # Pantheon Provider 初始化 + 模型获取
│   ├── vectordb/
│   │   ├── interface.go           # VectorDatabase 接口定义
│   │   ├── lancedb.go             # LanceDB 实现（第一阶段）
│   │   └── pgvector.go            # PGVector 实现（第一阶段）
│   ├── dto/                       # 请求/响应结构体（JSON tag 严格复刻现有格式）
│   │   ├── auth.go
│   │   ├── workspace.go
│   │   ├── chat.go
│   │   ├── document.go
│   │   └── system.go
│   └── static/
│       └── frontend.go            # //go:embed frontend/dist
├── pkg/
│   ├── mlog/                      # 直接使用 github.com/odysseythink/mlog
│   └── utils/
│       ├── jwt.go                 # JWT 生成/解析
│       ├── bcrypt.go              # 密码哈希
│       ├── encryption.go          # PEM 加密管理器（复刻 Node EncryptionManager）
│       └── files.go               # 文件路径、安全检查
├── storage/                       # 运行时存储（SQLite DB、上传文档、向量缓存）
├── tests/
│   ├── integration/               # API 端点集成测试
│   └── unit/                      # 服务层单元测试
├── frontend/dist/                 # 构建产物（yarn build 后嵌入二进制）
├── go.mod
├── go.sum
└── Makefile
```

---

## 4. 数据模型映射

### 4.1 映射原则

1. **表名兼容**: Prisma 默认小写复数（`users`, `workspaces`），GORM 默认也是小写复数，基本兼容。
2. **字段命名**: GORM 模型使用大写开头，JSON tag 显式指定小写开头驼峰（`json:"createdAt"`），匹配前端期望。
3. **关系映射**: Prisma 的 `@relation` 映射为 GORM 的 `foreignKey` + `references` + `constraint:OnDelete:CASCADE`。
4. **默认值**: GORM 的 `gorm:"default:value"` 复刻 Prisma 的 `@default(...)`。

### 4.2 核心模型示例

```go
// models/user.go
type User struct {
    ID                    int       `gorm:"primaryKey;autoIncrement" json:"id"`
    Username              *string   `gorm:"unique" json:"username"`
    Password              string    `json:"-"`  // 不序列化到 JSON
    PfpFilename           *string   `json:"pfpFilename"`
    Role                  string    `gorm:"default:default" json:"role"`
    Suspended             int       `gorm:"default:0" json:"suspended"`
    SeenRecoveryCodes     *bool     `gorm:"default:false" json:"seen_recovery_codes"`
    DailyMessageLimit     *int      `json:"dailyMessageLimit"`
    Bio                   *string   `gorm:"default:''" json:"bio"`
    WebPushSubscriptionConfig *string `json:"web_push_subscription_config"`
    CreatedAt             time.Time `json:"createdAt"`
    LastUpdatedAt         time.Time `json:"lastUpdatedAt"`

    // Relations
    WorkspaceChats        []WorkspaceChat      `gorm:"foreignKey:UserID" json:"workspace_chats,omitempty"`
    WorkspaceUsers        []WorkspaceUser      `gorm:"foreignKey:UserID" json:"workspace_users,omitempty"`
    Threads               []WorkspaceThread    `gorm:"foreignKey:UserID" json:"threads,omitempty"`
    RecoveryCodes         []RecoveryCode       `gorm:"foreignKey:UserID" json:"recovery_codes,omitempty"`
    PasswordResetTokens   []PasswordResetToken `gorm:"foreignKey:UserID" json:"password_reset_tokens,omitempty"`
}
```

```go
// models/workspace.go
type Workspace struct {
    ID                    int     `gorm:"primaryKey;autoIncrement" json:"id"`
    Name                  string  `json:"name"`
    Slug                  string  `gorm:"unique" json:"slug"`
    VectorTag             *string `json:"vectorTag"`
    CreatedAt             time.Time `json:"createdAt"`
    LastUpdatedAt         time.Time `json:"lastUpdatedAt"`
    OpenAiTemp            *float64 `json:"openAiTemp"`
    OpenAiHistory         int     `gorm:"default:20" json:"openAiHistory"`
    OpenAiPrompt          *string `json:"openAiPrompt"`
    SimilarityThreshold   *float64 `gorm:"default:0.25" json:"similarityThreshold"`
    ChatProvider          *string `json:"chatProvider"`
    ChatModel             *string `json:"chatModel"`
    TopN                  *int    `gorm:"default:4" json:"topN"`
    ChatMode              *string `gorm:"default:chat" json:"chatMode"`
    PfpFilename           *string `json:"pfpFilename"`
    AgentProvider         *string `json:"agentProvider"`
    AgentModel            *string `json:"agentModel"`
    QueryRefusalResponse  *string `json:"queryRefusalResponse"`
    VectorSearchMode      *string `gorm:"default:default" json:"vectorSearchMode"`

    // Relations
    WorkspaceUsers        []WorkspaceUser      `gorm:"foreignKey:WorkspaceID" json:"workspace_users,omitempty"`
    Documents             []WorkspaceDocument  `gorm:"foreignKey:WorkspaceID" json:"documents,omitempty"`
    Threads               []WorkspaceThread    `gorm:"foreignKey:WorkspaceID" json:"threads,omitempty"`
    WorkspaceChats        []WorkspaceChat      `gorm:"foreignKey:WorkspaceID" json:"workspace_chats,omitempty"`
}
```

### 4.3 迁移策略

- **从零开始**: 由于是全新重写，无需处理旧数据迁移。
- **AutoMigrate**: 使用 `db.AutoMigrate(&models.User{}, &models.Workspace{}, ...)` 自动建表。
- **Seed**: 启动时检查 `system_settings` 表，若为空则写入默认键值对（复刻 Node 的 Prisma seed）。

---

## 5. API 路由映射

### 5.1 路由注册

复刻 Node 后端的模块化路由注册模式：

```go
// internal/router/router.go
func RegisterRoutes(r *gin.Engine, db *gorm.DB, cfg *config.Config) {
    // 初始化共享 services
    authSvc := services.NewAuthService(db, cfg)
    sysSvc := services.NewSystemService(db)
    wsSvc := services.NewWorkspaceService(db, cfg)
    chatSvc := services.NewChatService(db, cfg, sysSvc)
    docSvc := services.NewDocumentService(db, cfg)
    vectorSvc := services.NewVectorService(cfg)
    adminSvc := services.NewAdminService(db)

    // API 路由组
    api := r.Group("/api")
    {
        handlers.RegisterSystemRoutes(api, sysSvc, cfg)
        handlers.RegisterAuthRoutes(api, authSvc)
        handlers.RegisterWorkspaceRoutes(api, wsSvc, authSvc)
        handlers.RegisterChatRoutes(api, chatSvc, authSvc)
        handlers.RegisterDocumentRoutes(api, docSvc, cfg)
        handlers.RegisterAdminRoutes(api, adminSvc, authSvc)
    }

    // 静态文件 + SPA fallback
    registerStaticRoutes(r)
}
```

### 5.2 中间件链

复刻 Node 后端的中间件链：

```go
// 典型端点：POST /api/workspace/:slug/stream-chat
r.POST("/workspace/:slug/stream-chat",
    middleware.ValidatedRequest(authSvc),              // 认证
    middleware.FlexUserRoleValid([]string{"all"}),     // 角色
    middleware.ValidWorkspaceSlug(),                   // 工作区校验
    handler.StreamChat)
```

| 中间件 | Node 对应 | 职责 |
|--------|-----------|------|
| `ValidatedRequest` | `validatedRequest` | 解析 JWT/Session，设置 `c.Set("user", user)` 和 `c.Set("multiUserMode", bool)` |
| `FlexUserRoleValid` | `flexUserRoleValid` | 检查 `user.Role` 是否在允许列表中 |
| `ValidWorkspaceSlug` | `validWorkspaceSlug` | 从 `:slug` 加载 workspace，设置 `c.Set("workspace", ws)` |
| `ChatHistoryViewable` | `chatHistoryViewable` | 检查当前用户是否有权查看聊天历史 |

### 5.3 DTO 设计

所有请求/响应结构体显式定义，JSON tag 严格匹配现有 API：

```go
// dto/chat.go
type StreamChatRequest struct {
    Message     string   `json:"message"`
    Attachments []string `json:"attachments"`
}

type StreamChatResponse struct {
    ID           string  `json:"id"`
    Type         string  `json:"type"`         // "textResponseChunk", "abort", etc.
    TextResponse *string `json:"textResponse"`
    Sources      []any   `json:"sources"`
    Close        bool    `json:"close"`
    Error        *string `json:"error,omitempty"`
}

type WorkspaceResponse struct {
    Workspace *models.Workspace `json:"workspace"`
    Message   string            `json:"message,omitempty"`
}
```

---

## 6. 核心服务层设计

### 6.1 认证服务

支持双模式认证，完全复刻 Node 逻辑：

**单用户模式**:
1. 读取 `AUTH_TOKEN` 环境变量
2. JWT payload `p` 经过 EncryptionManager 解密
3. bcrypt 比对 `decrypt(p)` 和 `AUTH_TOKEN`

**多用户模式**:
1. bcrypt 比对密码哈希
2. 生成 JWT Session
3. 支持每日消息配额检查

```go
type AuthService struct {
    db  *gorm.DB
    cfg *config.Config
    enc *utils.EncryptionManager
}

func (s *AuthService) ValidateToken(ctx context.Context, tokenStr string) (*models.User, error) {
    if !s.cfg.MultiUserMode {
        return s.validateSingleUserToken(tokenStr)
    }
    return s.validateMultiUserSession(tokenStr)
}

func (s *AuthService) Login(ctx context.Context, username, password string) (string, error) {
    // 多用户模式：查询 users 表，bcrypt 比对密码
    // 生成 JWT
}
```

### 6.2 聊天服务

核心流程复刻 Node 的 `streamChatWithWorkspace`：

```
1. 接收用户消息 + attachments
2. 根据 workspace.ChatProvider/ChatModel 获取 Pantheon LanguageModel
3. 从 workspace_chats 表读取最近 OpenAiHistory 条消息作为上下文
4. 调用 VectorService.SimilaritySearch 获取相关文档片段（RAG）
5. 组装 System Prompt（workspace.OpenAiPrompt + 搜索结果上下文）
6. 调用 Pantheon model.Stream(ctx, request)
7. 将 Pantheon 的 iter.Seq2 转换为 SSE chunks
8. 保存 assistant 完整回复到 workspace_chats
9. 发送 Telemetry 事件
```

```go
func (s *ChatService) Stream(ctx context.Context, ws *models.Workspace, user *models.User, req dto.StreamChatRequest) (<-chan dto.StreamChatResponse, error) {
    // 获取 Pantheon 模型
    model, err := s.llmProv.GetModel(ctx, ws.ChatProvider, ws.ChatModel)
    if err != nil {
        return nil, err
    }

    // 构建历史上下文
    history, err := s.buildChatHistory(ctx, ws.ID, ws.OpenAiHistory)
    if err != nil {
        return nil, err
    }

    // 向量搜索（RAG）
    sources, err := s.vectorSvc.SimilaritySearch(ctx, ws, req.Message, ws.TopN)
    if err != nil {
        return nil, err
    }

    // 组装 System Prompt
    systemPrompt := s.buildSystemPrompt(ws, sources)

    // 组装 Pantheon Request
    pantheonReq := &core.Request{
        System: systemPrompt,
        Messages: append(history, core.Message{
            Role: core.MESSAGE_ROLE_USER,
            Content: []core.ContentParter{core.TextPart{Text: req.Message}},
        }),
    }

    // 流式调用
    stream, err := model.Stream(ctx, pantheonReq)
    if err != nil {
        return nil, err
    }

    // 转换 channel
    out := make(chan dto.StreamChatResponse, 16)
    go func() {
        defer close(out)
        var fullText strings.Builder
        msgID := uuid.New().String()

        for part, err := range stream {
            if err != nil {
                out <- dto.StreamChatResponse{
                    ID: msgID, Type: "abort",
                    Close: true, Error: utils.Ptr(err.Error()),
                }
                return
            }

            switch part.Type {
            case core.StreamPartTypeTextDelta:
                fullText.WriteString(part.TextDelta)
                out <- dto.StreamChatResponse{
                    ID: msgID, Type: "textResponseChunk",
                    TextResponse: utils.Ptr(part.TextDelta),
                }
            case core.StreamPartTypeFinish:
                out <- dto.StreamChatResponse{
                    ID: msgID, Type: "textResponseChunk",
                    TextResponse: utils.Ptr(""), Close: true,
                }
            }
        }

        // 保存到数据库
        s.saveChatResponse(ctx, ws, user, req.Message, fullText.String())

        // Telemetry
        s.telemetry.Send("sent_chat", map[string]any{...})
    }()

    return out, nil
}
```

### 6.3 SSE 流式实现

Gin 原生支持流式响应：

```go
func (h *ChatHandler) StreamChat(c *gin.Context) {
    c.Header("Content-Type", "text/event-stream")
    c.Header("Cache-Control", "no-cache")
    c.Header("Connection", "keep-alive")
    c.Header("Access-Control-Allow-Origin", "*")

    ws := c.MustGet("workspace").(*models.Workspace)
    user := c.MustGet("user").(*models.User)

    var req dto.StreamChatRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, dto.StreamChatResponse{Type: "abort", Error: utils.Ptr(err.Error()), Close: true})
        return
    }

    stream, err := h.chatSvc.Stream(c.Request.Context(), ws, user, req)
    if err != nil {
        c.JSON(500, dto.StreamChatResponse{Type: "abort", Error: utils.Ptr(err.Error()), Close: true})
        return
    }

    c.Stream(func(w io.Writer) bool {
        for chunk := range stream {
            json.NewEncoder(w).Encode(chunk)
            if f, ok := w.(http.Flusher); ok {
                f.Flush()
            }
        }
        return false
    })
}
```

---

## 7. 向量数据库抽象

### 7.1 接口设计

```go
package vectordb

type VectorDatabase interface {
    Name() string
    Connect(ctx context.Context) error
    Heartbeat(ctx context.Context) (map[string]any, error)
    Tables(ctx context.Context) ([]string, error)
    TotalVectors(ctx context.Context) (int64, error)
    AddVectors(ctx context.Context, namespace string, chunks []VectorChunk) error
    DeleteVectors(ctx context.Context, namespace string, docIds []string) error
    SimilaritySearch(ctx context.Context, namespace, query string, topN int) ([]SearchResult, error)
}

type VectorChunk struct {
    ID       string
    Vector   []float32
    Metadata map[string]any
}

type SearchResult struct {
    DocId    string
    Text     string
    Score    float64
    Metadata map[string]any
}
```

### 7.2 LanceDB 实现（第一阶段）

使用 LanceDB 的 Go SDK（`github.com/lancedb/lancedb`）或 REST API。

```go
type LanceDB struct {
    uri string
}

func (l *LanceDB) Connect(ctx context.Context) error {
    // lancedb.Connect(l.uri)
}

func (l *LanceDB) SimilaritySearch(ctx context.Context, namespace, query string, topN int) ([]SearchResult, error) {
    // 1. 调用 Pantheon 嵌入接口将 query 转为 vector
    // 2. table.Search(vector).Limit(topN).Execute()
    // 3. 转换结果为 SearchResult
}
```

### 7.3 PGVector 实现（第一阶段）

通过 `pgx` + `pgvector` 扩展 SQL：

```go
type PGVector struct {
    conn *pgx.Conn
}

func (p *PGVector) SimilaritySearch(ctx context.Context, namespace, query string, topN int) ([]SearchResult, error) {
    // 1. 嵌入 query
    // 2. SELECT id, metadata, embedding <=> $1 AS score FROM table ORDER BY score LIMIT $2
}
```

> ⚠️ **关键依赖**: 向量搜索需要先将文本 query 转为 embedding vector。Pantheon 提供嵌入接口，`VectorService` 需要持有 `EmbeddingProvider` 引用。

---

## 8. 配置管理

### 8.1 配置结构体

复刻所有 Node `.env` 变量：

```go
type Config struct {
    ServerPort      string `env:"SERVER_PORT" envDefault:"3001"`
    JWTSecret       string `env:"JWT_SECRET"`
    SigKey          string `env:"SIG_KEY"`
    SigSalt         string `env:"SIG_SALT"`
    StorageDir      string `env:"STORAGE_DIR"`
    AuthToken       string `env:"AUTH_TOKEN"`
    VectorDB        string `env:"VECTOR_DB" envDefault:"lancedb"`
    LLMProvider     string `env:"LLM_PROVIDER"`
    EmbeddingEngine string `env:"EMBEDDING_ENGINE"`
    TTSProvider     string `env:"TTS_PROVIDER" envDefault:"native"`
    EnableHTTPS     bool   `env:"ENABLE_HTTPS"`
    NodeEnv         string `env:"NODE_ENV" envDefault:"production"`
    // ... 其他所有变量
}
```

### 8.2 加载方式

使用 `github.com/caarlos0/env/v11` 或自定义解析：

```go
func Load() (*Config, error) {
    // 1. 加载 .env 文件（dotenv 风格）
    // 2. 从 os.Getenv 填充结构体
    // 3. 验证必填字段
}
```

---

## 9. 前端嵌入（`//go:embed`）

### 9.1 实现

```go
// internal/static/frontend.go
package static

import "embed"

//go:embed all:frontend/dist
var FrontendFS embed.FS
```

### 9.2 静态路由注册

```go
func registerStaticRoutes(r *gin.Engine) {
    // 服务静态文件（JS/CSS/图片）
    staticServer := http.FileServer(http.FS(static.FrontendFS))
    r.GET("/assets/*filepath", gin.WrapH(staticServer))

    // SPA fallback: 所有非 /api 路径返回 index.html
    r.NoRoute(func(c *gin.Context) {
        if strings.HasPrefix(c.Request.URL.Path, "/api/") {
            c.JSON(404, gin.H{"error": "Not found"})
            return
        }

        // Vite 构建产物中的 index.html
        index, err := static.FrontendFS.ReadFile("frontend/dist/index.html")
        if err != nil {
            c.JSON(500, gin.H{"error": "index.html not found"})
            return
        }
        c.Data(200, "text/html; charset=utf-8", index)
    })
}
```

### 9.3 构建流程

```makefile
# Makefile
.PHONY: build build-frontend build-server dev

build-frontend:
	cd frontend && yarn build

build-server: build-frontend
	cd backend && go build -o ../anything-llm ./cmd/server/

dev-frontend:
	cd frontend && yarn dev

dev-server:
	cd backend && go run ./cmd/server/ -logtostderr

build: build-server
```

---

## 10. 错误处理

统一错误响应格式，复刻 Node 后端的 `response.status().json({error: ...})`：

```go
type ErrorResponse struct {
    Error   string `json:"error"`
    Message string `json:"message,omitempty"`
}

func JSONError(c *gin.Context, status int, err error) {
    mlog.Error("API error",
        mlog.String("path", c.Request.URL.Path),
        mlog.Int("status", status),
        mlog.Err(err),
    )
    c.JSON(status, ErrorResponse{Error: err.Error()})
}
```

Gin Recovery 中间件：

```go
r.Use(gin.Recovery())
r.Use(func(c *gin.Context) {
    defer func() {
        if r := recover(); r != nil {
            mlog.Error("panic recovered", mlog.Any("recover", r))
            c.JSON(500, ErrorResponse{Error: "Internal server error"})
        }
    }()
    c.Next()
})
```

---

## 11. 日志（mlog）

直接使用 `github.com/odysseythink/mlog`：

```go
import "github.com/odysseythink/mlog"

func init() {
    defer mlog.Flush()
    mlog.SetEncoder(mlog.NewJSONEncoder())
    mlog.SetLogDir("./storage/logs")
}

// 在 handler 中
mlog.Info("请求处理完成",
    mlog.String("method", c.Request.Method),
    mlog.String("path", c.Request.URL.Path),
    mlog.Int("status", c.Writer.Status()),
    mlog.Duration("elapsed", time.Since(start)),
)
```

---

## 12. 测试策略

### 12.1 测试结构

```
tests/
├── integration/
│   ├── auth_test.go       # 登录/注册/密码重置
│   ├── workspace_test.go  # 工作区 CRUD
│   ├── chat_test.go       # 聊天 + SSE 流式
│   └── document_test.go   # 文档上传
└── unit/
    ├── auth_service_test.go
    ├── chat_service_test.go
    └── vector_service_test.go
```

### 12.2 测试工具

- **集成测试**: `gin.TestEngine()` + SQLite in-memory (`:memory:`)
- **单元测试**: `sqlmock` mock GORM 查询 + mock Pantheon `LanguageModel` 接口
- **向量 DB 测试**: LanceDB 临时目录 + Testcontainers（PGVector）

### 12.3 示例

```go
func TestStreamChat(t *testing.T) {
    db, mock := setupTestDB(t)
    cfg := &config.Config{VectorDB: "lancedb"}
    
    chatSvc := services.NewChatService(db, cfg, mockLLMProvider)
    handler := handlers.NewChatHandler(chatSvc, authSvc)
    
    w := httptest.NewRecorder()
    req, _ := http.NewRequest("POST", "/api/workspace/test-ws/stream-chat",
        strings.NewReader(`{"message":"hello"}`))
    req.Header.Set("Authorization", "Bearer "+validToken)
    
    r := gin.Default()
    r.POST("/api/workspace/:slug/stream-chat", handler.StreamChat)
    r.ServeHTTP(w, req)
    
    assert.Equal(t, 200, w.Code)
    assert.Contains(t, w.Header().Get("Content-Type"), "text/event-stream")
}
```

---

## 13. 分阶段交付范围

### 13.1 第一阶段（MVP）— 核心 Chat-with-Documents

| 模块 | 端点 | 说明 |
|------|------|------|
| **system** | `GET /api/ping` | 健康检查 |
| | `GET /api/setup-complete` | 初始化状态 |
| | `GET /api/onboarding` | 引导状态 |
| | `POST /api/onboarding` | 完成引导 |
| | `GET /api/system/*` | 系统设置读取 |
| | `POST /api/system/*` | 系统设置更新 |
| | `GET /api/env-dump` | 环境变量导出（dev only） |
| **auth** | `POST /api/request-token` | 获取 auth token |
| | `POST /api/login` | 多用户登录 |
| | `POST /api/register` | 多用户注册 |
| | `POST /api/password-reset` | 密码重置 |
| | `POST /api/logout` | 登出 |
| **workspace** | `POST /api/workspace/new` | 创建工作区 |
| | `GET /api/workspace` | 列出工作区 |
| | `GET /api/workspace/:slug` | 获取工作区详情 |
| | `DELETE /api/workspace/:slug` | 删除工作区 |
| | `POST /api/workspace/:slug/update` | 更新工作区 |
| | `POST /api/workspace/:slug/upload` | 上传文档到工作区 |
| | `GET /api/workspace/:slug/chats` | 聊天历史 |
| | `GET /api/workspace/:slug/suggested-messages` | 建议消息 |
| **chat** | `POST /api/workspace/:slug/stream-chat` | **核心 SSE 流式聊天** |
| | `POST /api/workspace/:slug/slack` | 简单聊天 |
| **document** | `POST /api/document/upload` | 文档上传 |
| | `GET /api/document/:docId` | 文档详情 |
| | `DELETE /api/document/:docId` | 删除文档 |
| | `GET /api/document/accepted-extensions` | 支持的文件类型 |
| **admin** | `GET /api/admin/workspaces` | 管理工作区 |
| | `GET /api/admin/users` | 管理用户 |
| | `POST /api/admin/users/new` | 新建用户 |
| | `DELETE /api/admin/users/:id` | 删除用户 |

### 13.2 第二阶段 — 扩展功能

| 模块 | 说明 |
|------|------|
| Agent WebSocket | `/api/agent-websocket`（实时 Agent 调用） |
| Agent Flows | `/api/agent-flows/*`（Flow Builder） |
| MCP Servers | `/api/mcp-servers/*` |
| Embed Widget | `/api/embed/*`, `/api/embed-management/*` |
| Browser Extension | `/api/browser-extension/*` |
| Telegram | `/api/telegram/*` |
| Web Push | `/api/web-push/*` |
| Mobile API | `/api/mobile/*` |
| Community Hub | `/api/community-hub/*` |
| Developer API | `/api/v1/*` |
| Experimental | `/api/experimental/*` |

---

## 14. 构建和部署

### 14.1 开发模式

```bash
# Terminal 1: 前端开发服务器
cd frontend && yarn dev

# Terminal 2: Go 后端开发
cd backend && go run ./cmd/server/ -logtostderr
```

### 14.2 生产构建

```bash
# 一键构建
make build

# 输出: ./anything-llm（单二进制文件，包含前端）
# 运行: ./anything-llm
```

### 14.3 Docker

```dockerfile
# 多阶段构建
FROM node:18 AS frontend
WORKDIR /app/frontend
COPY frontend/ .
RUN yarn install && yarn build

FROM golang:1.23 AS builder
WORKDIR /app/backend
COPY backend/ .
COPY --from=frontend /app/frontend/dist ./frontend/dist
RUN go mod download
RUN CGO_ENABLED=1 go build -o /app/anything-llm ./cmd/server/

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y ca-certificates
COPY --from=builder /app/anything-llm /usr/local/bin/anything-llm
EXPOSE 3001
CMD ["anything-llm"]
```

---

## 15. 风险和对策

| 风险 | 影响 | 对策 |
|------|------|------|
| LanceDB Go SDK 不成熟 | 向量搜索功能受阻 | 备选方案：通过 HTTP API 调用 LanceDB；或第一阶段只用 PGVector |
| Pantheon 嵌入接口未覆盖所有提供商 | 嵌入功能受限 | Pantheon 已支持主流嵌入，缺失的可以通过 OpenAI-Compatible provider 桥接 |
| 28 个 Prisma 模型映射复杂 | 数据模型错误导致 API 不兼容 | 逐模型对照验证，写单元测试确保 JSON 序列化一致 |
| SSE 流式与前端期望不一致 | 聊天流式显示异常 | 逐 chunk 类型对照 Node 的 `writeResponseChunk` 实现 |
| 文件上传大小限制 | Node 有 3GB limit | Gin 的 `c.Request.ParseMultipartForm(3 << 30)` |

---

## 16. 自检清单

- [x] 无 TBD/TODO 占位符
- [x] 所有端点都映射到具体 handler
- [x] 技术选型已确认（Gin, GORM, Pantheon, mlog）
- [x] 数据模型 JSON tag 与前端期望一致
- [x] 错误响应格式复刻 Node 后端
- [x] 分阶段范围明确
- [x] 构建流程完整
- [x] 风险已识别并有对策
