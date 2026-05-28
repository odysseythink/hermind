# backend Embed Widget 复刻设计

> 目标：在 backend 中完整复刻主 server（Node.js/Express）的 Embed Widget 后端功能，实现功能对等。

---

## 1. 背景与范围

### 1.1 现状

backend 目前缺少完整的 Embed Widget 后端支持：

- 无 `EmbedConfig` / `EmbedChat` 数据模型
- 无公共嵌入端点（stream-chat、history、session delete）
- 无管理后台的嵌入配置 CRUD
- 无开发者 API（Bearer API Key 认证的 embed 端点）

### 1.2 目标

实现与主 server 功能对等的 Embed Widget 后端，包括：

1. **公共嵌入端点** — iframe 嵌入小部件的聊天、历史、会话删除
2. **管理后台** — 嵌入配置的创建、更新、删除、列表、聊天记录管理
3. **开发者 API** — 通过 Bearer API Key 访问的 REST 端点

### 1.3 非目标

- 前端嵌入小部件 UI（已有，位于 `embed/` Git submodule）
- 浏览器扩展支持（独立模块）

---

## 2. 数据模型

### 2.1 EmbedConfig

```go
package models

type EmbedConfig struct {
    ID                       int       `gorm:"primaryKey;autoIncrement" json:"id"`
    UUID                     string    `gorm:"uniqueIndex;not null" json:"uuid"`
    Enabled                  bool      `gorm:"default:false" json:"enabled"`
    ChatMode                 string    `gorm:"default:query" json:"chatMode"`
    AllowlistDomains         *string   `json:"allowlistDomains"`        // JSON array string
    AllowModelOverride       bool      `gorm:"default:false" json:"allowModelOverride"`
    AllowTemperatureOverride bool      `gorm:"default:false" json:"allowTemperatureOverride"`
    AllowPromptOverride      bool      `gorm:"default:false" json:"allowPromptOverride"`
    MaxChatsPerDay           *int      `json:"maxChatsPerDay"`
    MaxChatsPerSession       *int      `json:"maxChatsPerSession"`
    MessageLimit             *int      `gorm:"default:20" json:"messageLimit"`
    WorkspaceID              int       `json:"workspaceId"`
    CreatedBy                *int      `json:"createdBy"`
    CreatedAt                time.Time `json:"createdAt"`
    LastUpdatedAt            time.Time `json:"lastUpdatedAt"`
}
```

**字段说明：**

| 字段 | 类型 | 说明 |
|------|------|------|
| `UUID` | `string` | 对外唯一标识符，用于嵌入 URL |
| `Enabled` | `bool` | 是否启用该嵌入配置 |
| `ChatMode` | `string` | `"chat"` 或 `"query"`，默认 `"query"` |
| `AllowlistDomains` | `*string` | JSON 数组字符串，`null` 表示不限制，`"[]"` 表示拒绝全部 |
| `Allow*Override` | `bool` | 是否允许前端覆盖对应参数 |
| `MaxChatsPerDay` | `*int` | 每日聊天次数上限，`null` 表示不限 |
| `MaxChatsPerSession` | `*int` | 每会话聊天次数上限，`null` 表示不限 |
| `MessageLimit` | `*int` | 加载历史消息条数上限，默认 20 |
| `WorkspaceID` | `int` | 关联工作区 |
| `CreatedBy` | `*int` | 创建者用户 ID |

### 2.2 EmbedChat

```go
package models

type EmbedChat struct {
    ID                    int       `gorm:"primaryKey;autoIncrement" json:"id"`
    Prompt                string    `json:"prompt"`
    Response              string    `json:"response"`              // JSON string
    SessionID             string    `json:"sessionId"`
    Include               bool      `gorm:"default:true" json:"include"`
    ConnectionInformation *string   `json:"connectionInformation"` // JSON string
    EmbedID               int       `json:"embedId"`
    UserID                *int      `json:"userId"`
    CreatedAt             time.Time `json:"createdAt"`
}
```

**字段说明：**

| 字段 | 类型 | 说明 |
|------|------|------|
| `Prompt` | `string` | 用户输入的消息 |
| `Response` | `string` | JSON 序列化的完整响应（含 `text`, `type`, `sources`, `metrics`） |
| `SessionID` | `string` | 客户端生成的 UUID v4，用于会话隔离 |
| `Include` | `bool` | `false` 表示会话历史已软删除 |
| `ConnectionInformation` | `*string` | JSON 序列化的连接元数据 `{host, ip, username}` |
| `EmbedID` | `int` | 关联的 EmbedConfig ID |

### 2.3 数据库关系

- `EmbedConfig` → `Workspace`（N:1，`workspace_id` 外键）
- `EmbedConfig` → `EmbedChat`（1:N，`embed_id` 外键，级联删除）
- `EmbedConfig` → `User`（N:1，`created_by` 外键，可选）

---

## 3. DTO 定义

### 3.1 请求 DTO

```go
package dto

// CreateEmbedConfigRequest 创建嵌入配置
type CreateEmbedConfigRequest struct {
    WorkspaceSlug            string   `json:"workspace_slug" binding:"required"`
    ChatMode                 string   `json:"chat_mode,omitempty"`
    AllowlistDomains         []string `json:"allowlist_domains,omitempty"`
    AllowModelOverride       bool     `json:"allow_model_override"`
    AllowTemperatureOverride bool     `json:"allow_temperature_override"`
    AllowPromptOverride      bool     `json:"allow_prompt_override"`
    MaxChatsPerDay           *int     `json:"max_chats_per_day,omitempty"`
    MaxChatsPerSession       *int     `json:"max_chats_per_session,omitempty"`
    MessageLimit             *int     `json:"message_limit,omitempty"`
}

// UpdateEmbedConfigRequest 更新嵌入配置（仅白名单字段）
type UpdateEmbedConfigRequest struct {
    Enabled                  *bool    `json:"enabled,omitempty"`
    ChatMode                 *string  `json:"chat_mode,omitempty"`
    AllowlistDomains         []string `json:"allowlist_domains,omitempty"`
    AllowModelOverride       *bool    `json:"allow_model_override,omitempty"`
    AllowTemperatureOverride *bool    `json:"allow_temperature_override,omitempty"`
    AllowPromptOverride      *bool    `json:"allow_prompt_override,omitempty"`
    MaxChatsPerDay           *int     `json:"max_chats_per_day,omitempty"`
    MaxChatsPerSession       *int     `json:"max_chats_per_session,omitempty"`
    MessageLimit             *int     `json:"message_limit,omitempty"`
    WorkspaceID              *int     `json:"workspace_id,omitempty"`
}

// EmbedStreamChatRequest 嵌入聊天流请求
type EmbedStreamChatRequest struct {
    SessionID   string   `json:"sessionId" binding:"required"`
    Message     string   `json:"message" binding:"required"`
    Prompt      *string  `json:"prompt,omitempty"`
    Model       *string  `json:"model,omitempty"`
    Temperature *float64 `json:"temperature,omitempty"`
    Username    *string  `json:"username,omitempty"`
}

// ListEmbedChatsRequest 聊天记录列表（管理后台）
type ListEmbedChatsRequest struct {
    Offset int `json:"offset"`
    Limit  int `json:"limit"`
}
```

### 3.2 响应 DTO

```go
package dto

// EmbedConfigResponse 嵌入配置响应
type EmbedConfigResponse struct {
    ID        int              `json:"id"`
    UUID      string           `json:"uuid"`
    Enabled   bool             `json:"enabled"`
    ChatMode  string           `json:"chatMode"`
    Workspace WorkspaceSummary `json:"workspace"`
    ChatCount int64            `json:"chatCount"`
    CreatedAt time.Time        `json:"createdAt"`
}

// WorkspaceSummary 工作区摘要
type WorkspaceSummary struct {
    ID   int    `json:"id"`
    Name string `json:"name"`
}

// EmbedConfigListResponse 嵌入配置列表
type EmbedConfigListResponse struct {
    Embeds []EmbedConfigResponse `json:"embeds"`
}

// EmbedChatHistoryItem 聊天历史项
type EmbedChatHistoryItem struct {
    Role    string    `json:"role"`
    Content string    `json:"content"`
    SentAt  time.Time `json:"sentAt"`
    Sources []any     `json:"sources,omitempty"`
}

// EmbedHistoryResponse 嵌入聊天历史响应
type EmbedHistoryResponse struct {
    History []EmbedChatHistoryItem `json:"history"`
}

// EmbedChatListResponse 聊天记录列表（管理后台）
type EmbedChatListResponse struct {
    Chats     []EmbedChatAdminItem `json:"chats"`
    HasPages  bool                 `json:"hasPages"`
    TotalChats int64               `json:"totalChats"`
}

// EmbedChatAdminItem 管理后台聊天记录项
type EmbedChatAdminItem struct {
    ID          int              `json:"id"`
    Prompt      string           `json:"prompt"`
    Response    string           `json:"response"`
    SessionID   string           `json:"sessionId"`
    EmbedConfig EmbedConfigShort `json:"embed_config"`
    Workspace   WorkspaceSummary `json:"workspace"`
    CreatedAt   time.Time        `json:"createdAt"`
}

// EmbedConfigShort 嵌入配置简短信息
type EmbedConfigShort struct {
    ID   int    `json:"id"`
    UUID string `json:"uuid"`
}
```

### 3.3 SSE Chunk DTO（复用现有）

复用 `dto.StreamChatResponse`，Embed 流使用相同的 SSE chunk 格式：

```json
{"uuid": "...", "type": "textResponseChunk", "textResponse": "token", "sources": [], "close": false, "error": null}
```

错误 abort chunk：

```json
{"id": "...", "type": "abort", "textResponse": null, "sources": [], "close": true, "error": "error message"}
```

---

## 4. 中间件设计

### 4.1 中间件清单

| 中间件 | 用途 | 应用路由 |
|--------|------|----------|
| `ValidEmbedConfig` | 按 UUID 校验并加载 EmbedConfig + Workspace | 所有 `/:embedId/` 公开路由 |
| `ValidEmbedConfigId` | 按数字 ID 校验 EmbedConfig | 管理后台 update/delete 路由 |
| `SetConnectionMeta` | 捕获 Origin 和 IP | stream-chat 路由 |
| `CanRespond` | 综合门前校验（启用/域名/会话/消息/限流） | stream-chat 路由 |

### 4.2 ValidEmbedConfig

```go
func ValidEmbedConfig(db *gorm.DB) gin.HandlerFunc {
    return func(c *gin.Context) {
        uuid := c.Param("embedId")
        var cfg models.EmbedConfig
        if err := db.Where("uuid = ?", uuid).Preload("Workspace").First(&cfg).Error; err != nil {
            c.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "Embed config not found"})
            c.Abort()
            return
        }
        c.Set("embedConfig", &cfg)
        c.Next()
    }
}
```

### 4.3 ValidEmbedConfigId

```go
func ValidEmbedConfigId(db *gorm.DB) gin.HandlerFunc {
    return func(c *gin.Context) {
        id, err := strconv.Atoi(c.Param("embedId"))
        if err != nil {
            c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "Invalid embed ID"})
            c.Abort()
            return
        }
        var cfg models.EmbedConfig
        if err := db.First(&cfg, id).Error; err != nil {
            c.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "Embed config not found"})
            c.Abort()
            return
        }
        c.Set("embedConfig", &cfg)
        c.Next()
    }
}
```

### 4.4 SetConnectionMeta

```go
func SetConnectionMeta() gin.HandlerFunc {
    return func(c *gin.Context) {
        host := c.GetHeader("Origin")
        ip := c.ClientIP()
        c.Set("connection", &dto.ConnectionMeta{Host: host, IP: ip})
        c.Next()
    }
}
```

### 4.5 CanRespond

```go
func CanRespond(db *gorm.DB) gin.HandlerFunc {
    return func(c *gin.Context) {
        embed := c.MustGet("embedConfig").(*models.EmbedConfig)
        conn := c.MustGet("connection").(*dto.ConnectionMeta)
        
        var req dto.EmbedStreamChatRequest
        if err := c.ShouldBindJSON(&req); err != nil {
            writeSSEAbort(c, "Invalid request body")
            c.Abort()
            return
        }
        
        // 1. 启用检查
        if !embed.Enabled {
            writeSSEAbort(c, "Embed is not enabled")
            c.Abort()
            return
        }
        
        // 2. 域名白名单检查
        if embed.AllowlistDomains != nil {
            allowed, err := parseAllowlistDomains(*embed.AllowlistDomains)
            if err != nil || len(allowed) == 0 {
                writeSSEAbort(c, "Domain not allowed")
                c.Abort()
                return
            }
            if !isOriginAllowed(conn.Host, allowed) {
                writeSSEAbort(c, "Domain not allowed")
                c.Abort()
                return
            }
        }
        
        // 3. Session ID 校验（UUID v4）
        if !isValidUUIDv4(req.SessionID) {
            writeSSEAbort(c, "Invalid session ID")
            c.Abort()
            return
        }
        
        // 4. 消息校验
        if strings.TrimSpace(req.Message) == "" {
            writeSSEAbort(c, "Message is required")
            c.Abort()
            return
        }
        if embed.ChatMode != "chat" && embed.ChatMode != "query" {
            writeSSEAbort(c, "Invalid chat mode")
            c.Abort()
            return
        }
        
        // 5. 日速率限制（24h 滑动窗口）
        since := time.Now().Add(-24 * time.Hour)
        if embed.MaxChatsPerDay != nil && *embed.MaxChatsPerDay > 0 {
            count := countRecentChats(db, embed.ID, since)
            if count >= int64(*embed.MaxChatsPerDay) {
                writeSSEAbort(c, "Daily chat limit exceeded")
                c.Abort()
                return
            }
        }
        
        // 6. 会话速率限制（24h 滑动窗口）
        if embed.MaxChatsPerSession != nil && *embed.MaxChatsPerSession > 0 {
            count := countRecentSessionChats(db, embed.ID, req.SessionID, since)
            if count >= int64(*embed.MaxChatsPerSession) {
                writeSSEAbort(c, "Session chat limit exceeded")
                c.Abort()
                return
            }
        }
        
        c.Set("embedRequest", &req)
        c.Next()
    }
}
```

**域名白名单解析规则：**

- 输入为 JSON 数组字符串，如 `"["https://example.com", "https://app.example.com"]"`
- `null`（数据库 NULL）→ 开放，不限制任何域名
- `"[]"` 或解析失败 → 拒绝所有来源
- 存储时自动补全 `https://` 前缀并校验 URL 格式

---

## 5. 服务层设计

### 5.1 EmbedService 结构

```go
package services

type EmbedService struct {
    db        *gorm.DB
    cfg       *config.Config
    vectorSvc *VectorService
    llmProv   providers.LLMProvider
    embedder  embedder.Embedder
}

func NewEmbedService(
    db *gorm.DB,
    cfg *config.Config,
    vectorSvc *VectorService,
    llmProv providers.LLMProvider,
    embedder embedder.Embedder,
) *EmbedService {
    return &EmbedService{db, cfg, vectorSvc, llmProv, embedder}
}
```

### 5.2 CRUD 方法

```go
func (s *EmbedService) Create(ctx context.Context, req dto.CreateEmbedConfigRequest, creatorID *int) (*models.EmbedConfig, error)
func (s *EmbedService) Update(ctx context.Context, embedID int, req dto.UpdateEmbedConfigRequest) error
func (s *EmbedService) Delete(ctx context.Context, embedID int) error
func (s *EmbedService) GetByUUID(ctx context.Context, uuid string) (*models.EmbedConfig, error)
func (s *EmbedService) GetByID(ctx context.Context, id int) (*models.EmbedConfig, error)
func (s *EmbedService) List(ctx context.Context) ([]dto.EmbedConfigResponse, error)
```

**Create 字段校验逻辑：**

- `ChatMode`：仅允许 `"chat"` 或 `"query"`，默认 `"query"`
- `AllowlistDomains`：字符串数组 → 自动补全 `https://` 前缀 → URL 校验 → JSON 序列化存储
- `MessageLimit`：无效值（`<= 0` 或 `NaN`）→ 设为默认值 `20`
- `MaxChatsPerDay` / `MaxChatsPerSession`：无效值 → `nil`
- `WorkspaceSlug`：必须存在对应工作区

### 5.3 聊天历史方法

```go
func (s *EmbedService) ListChats(ctx context.Context, embedID int, sessionID *string, limit, offset int) ([]models.EmbedChat, error)
func (s *EmbedService) MarkHistoryInvalid(ctx context.Context, embedID int, sessionID string) error
func (s *EmbedService) CountRecentChats(ctx context.Context, embedID int, since time.Time) int64
func (s *EmbedService) CountRecentSessionChats(ctx context.Context, embedID int, sessionID string, since time.Time) int64
```

### 5.4 StreamChat 核心方法

```go
func (s *EmbedService) StreamChat(
    ctx context.Context,
    embed *models.EmbedConfig,
    req *dto.EmbedStreamChatRequest,
    conn *dto.ConnectionMeta,
) (<-chan dto.StreamChatResponse, error)
```

**执行流程：**

1. **解析模式**：`chatMode = embed.ChatMode`，若为 `"automatic"` 则强制设为 `"chat"`
2. **应用覆盖**（仅在配置允许时）：
   - `allow_prompt_override` → 覆盖系统提示词
   - `allow_model_override` → 覆盖聊天模型
   - `allow_temperature_override` → 覆盖温度参数
3. **Query 模式空文档保护**：
   - 若工作区无向量化文档（`embeddingsCount == 0`）且 `chatMode == "query"`
   - 立即返回单条 SSE：`"I do not have enough information to answer that. Try another question."`
4. **加载历史**：
   - 从 `EmbedChat` 查询 `embed_id = ? AND session_id = ? AND include = true`
   - 按 `id DESC` 取 `messageLimit` 条，再反转为时间序
   - 转换为 OpenAI 格式的 `{role, content}` 数组
5. **加载固定文档**：调用 `DocumentManager` 获取工作区 pinned documents，加入 `contextTexts`
6. **向量搜索**：调用 `VectorService.SimilaritySearch()`，使用工作区的 `similarityThreshold`、`topN`、`vectorSearchMode`
7. **Query 模式无结果保护**：若 `chatMode == "query"` 且 `contextTexts` 为空，返回 `workspace.QueryRefusalResponse` 或默认拒绝文本
8. **LLM 调用**：
   - 构建消息列表（system prompt + context + history + user message）
   - 调用 `llmProv.Stream()` 获取 token 流
   - 逐 token 转发为 SSE chunk
9. **持久化**：流结束后将完整响应写入 `EmbedChat`
   - `Response` JSON：`{text, type, sources, metrics}`
   - `ConnectionInformation` JSON：`{host, ip, username}`

---

## 6. 处理器与路由

### 6.1 公共嵌入端点

文件：`internal/handlers/embed.go`

```go
func RegisterEmbedRoutes(r *gin.RouterGroup, svc *services.EmbedService, db *gorm.DB) {
    h := NewEmbedHandler(svc)
    
    // 公开路由（无认证）
    r.POST("/embed/:embedId/stream-chat",
        middleware.ValidEmbedConfig(db),
        middleware.SetConnectionMeta(),
        middleware.CanRespond(db),
        h.StreamChat)
    
    r.GET("/embed/:embedId/:sessionId",
        middleware.ValidEmbedConfig(db),
        h.GetSessionHistory)
    
    r.DELETE("/embed/:embedId/:sessionId",
        middleware.ValidEmbedConfig(db),
        h.DeleteSession)
}
```

### 6.2 管理后台端点

```go
func RegisterEmbedManagementRoutes(r *gin.RouterGroup, svc *services.EmbedService, authSvc *services.AuthService, db *gorm.DB) {
    h := NewEmbedHandler(svc)
    
    r.GET("/embeds",
        middleware.ValidatedRequest(authSvc),
        middleware.FlexUserRoleValid([]string{"admin"}),
        h.ListEmbedConfigs)
    
    r.POST("/embeds/new",
        middleware.ValidatedRequest(authSvc),
        middleware.FlexUserRoleValid([]string{"admin"}),
        h.CreateEmbedConfig)
    
    r.POST("/embed/update/:embedId",
        middleware.ValidatedRequest(authSvc),
        middleware.FlexUserRoleValid([]string{"admin"}),
        middleware.ValidEmbedConfigId(db),
        h.UpdateEmbedConfig)
    
    r.DELETE("/embed/:embedId",
        middleware.ValidatedRequest(authSvc),
        middleware.FlexUserRoleValid([]string{"admin"}),
        middleware.ValidEmbedConfigId(db),
        h.DeleteEmbedConfig)
    
    r.POST("/embed/chats",
        middleware.ValidatedRequest(authSvc),
        middleware.FlexUserRoleValid([]string{"admin"}),
        h.ListAllChats)
    
    r.DELETE("/embed/chats/:chatId",
        middleware.ValidatedRequest(authSvc),
        middleware.FlexUserRoleValid([]string{"admin"}),
        h.DeleteChat)
}
```

### 6.3 开发者 API 端点

文件：`internal/handlers/api_embed.go`

```go
func RegisterAPIEmbedRoutes(r *gin.RouterGroup, svc *services.EmbedService, apiKeySvc *services.APIKeyService, db *gorm.DB) {
    h := NewAPIEmbedHandler(svc)
    
    // 需要 ValidAPIKey 中间件（若不存在则新增）
    r.GET("/v1/embed",
        middleware.ValidAPIKey(apiKeySvc),
        h.ListEmbedConfigs)
    
    r.GET("/v1/embed/:embedUuid/chats",
        middleware.ValidAPIKey(apiKeySvc),
        h.ListEmbedChats)
    
    r.GET("/v1/embed/:embedUuid/chats/:sessionUuid",
        middleware.ValidAPIKey(apiKeySvc),
        h.ListSessionChats)
    
    r.POST("/v1/embed/new",
        middleware.ValidAPIKey(apiKeySvc),
        h.CreateEmbedConfig)
    
    r.POST("/v1/embed/:embedUuid",
        middleware.ValidAPIKey(apiKeySvc),
        h.UpdateEmbedConfig)
    
    r.DELETE("/v1/embed/:embedUuid",
        middleware.ValidAPIKey(apiKeySvc),
        h.DeleteEmbedConfig)
}
```

---

## 7. 与现有系统集成

### 7.1 数据库迁移

修改 `internal/services/db.go` 的 `AutoMigrate`：

```go
func AutoMigrate(db *gorm.DB) error {
    return db.AutoMigrate(
        // ... existing models ...
        &models.EmbedConfig{},
        &models.EmbedChat{},
    )
}
```

### 7.2 主函数注册

修改 `cmd/server/main.go`：

```go
// 新增服务实例化
embedSvc := services.NewEmbedService(db, cfg, vectorSvc, llmProv, embedder)

// 在 /api 路由组下注册
handlers.RegisterEmbedRoutes(api, embedSvc, db)
handlers.RegisterEmbedManagementRoutes(api, embedSvc, authSvc, db)
handlers.RegisterAPIEmbedRoutes(api, embedSvc, apiKeySvc, db)
```

### 7.3 API Key 中间件

若 `middleware.ValidAPIKey` 不存在，需新增：

```go
func ValidAPIKey(apiKeySvc *services.APIKeyService) gin.HandlerFunc {
    return func(c *gin.Context) {
        authHeader := c.GetHeader("Authorization")
        tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
        if tokenStr == "" {
            c.JSON(http.StatusUnauthorized, dto.ErrorResponse{Error: "API key required"})
            c.Abort()
            return
        }
        key, err := apiKeySvc.ValidateKey(c.Request.Context(), tokenStr)
        if err != nil {
            c.JSON(http.StatusForbidden, dto.ErrorResponse{Error: "Invalid API key"})
            c.Abort()
            return
        }
        c.Set("apiKey", key)
        c.Next()
    }
}
```

---

## 8. 错误处理

### 8.1 SSE 错误响应

所有 Embed stream-chat 的错误均通过 SSE abort chunk 返回：

```go
func writeSSEAbort(c *gin.Context, msg string) {
    c.Header("Content-Type", "text/event-stream")
    c.Header("Cache-Control", "no-cache")
    c.Header("Connection", "keep-alive")
    c.Header("Access-Control-Allow-Origin", "*")
    
    chunk := dto.StreamChatResponse{
        ID:           generateUUID(),
        Type:         "abort",
        TextResponse: nil,
        Sources:      []any{},
        Close:        true,
        Error:        &msg,
    }
    // write to response writer
}
```

### 8.2 HTTP 状态码映射

| 场景 | 状态码 |
|------|--------|
| Embed UUID 不存在 | `404` |
| 域名未授权 | `401`（SSE abort） |
| Session ID 无效 | `404`（SSE abort） |
| 消息为空 | `400`（SSE abort） |
| 日/会话限流 | `429`（SSE abort） |
| Embed 未启用 | `503`（SSE abort） |
| 内部错误 | `500` / SSE abort |

---

## 9. 测试策略

### 9.1 单元测试

- `embed_service_test.go`：字段校验逻辑（`Create` 的边界条件）、域名白名单解析
- `embed_middleware_test.go`：`CanRespond` 的各校验分支

### 9.2 集成测试

- 创建 EmbedConfig → 查询 → 更新 → 删除 完整流程
- StreamChat 的 SSE 响应格式验证
- 历史记录隔离（不同 session_id 互不可见）
- 会话软删除后历史为空
- 速率限制触发

---

## 10. 实现顺序建议

1. **模型 + 迁移** — `EmbedConfig`, `EmbedChat`, `AutoMigrate` 更新
2. **DTO** — 所有请求/响应结构体
3. **服务层骨架** — `EmbedService` + CRUD 方法
4. **中间件** — `ValidEmbedConfig`, `SetConnectionMeta`, `CanRespond`
5. **公共端点** — `stream-chat`, `history`, `delete session`
6. **管理后台端点** — CRUD + 聊天记录管理
7. **开发者 API** — `ValidAPIKey` 中间件 + API 路由
8. **StreamChat 核心逻辑** — RAG 流、覆盖校验、持久化
9. **集成测试**

---

*设计完成，等待转入 implementation planning。*
