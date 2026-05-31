# Browser Extension 设计文档

> 日期: 2026-05-28
> 范围: Hermind Go 后端 + Chrome Manifest V3 扩展客户端
> 基准: anything-llm 1.13.0 服务器端源码分析

---

## 1. 架构总览

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           Hermind Browser Extension                         │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│   ┌─────────────┐     ┌─────────────┐     ┌─────────────┐                  │
│   │   Popup     │◄───►│ Background  │◄───►│   Content   │                  │
│   │  (React)    │     │  Service    │     │   Script    │                  │
│   │  Vite build │     │   Worker    │     │             │                  │
│   └─────────────┘     └──────┬──────┘     └─────────────┘                  │
│                              │                                              │
│                              │ chrome.runtime.sendMessage                   │
│                              ▼                                              │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │                    chrome.storage.sync (apiBase + apiKey)           │   │
│   └─────────────────────────────────────────────────────────────────────┘   │
│                              │                                              │
│                              │ HTTP /api/browser-extension/*                │
│                              ▼                                              │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │                      Hermind Go Backend (Gin)                       │   │
│   │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌───────────┐  │   │
│   │  │  HTTP       │  │  validBrx   │  │  BrowserExt │  │  Document │  │   │
│   │  │  Handlers   │──►│ Middleware  │──►│  Service    │──►│  Service  │  │   │
│   │  │  (8 routes) │  │  (Bearer)   │  │  (biz logic)│  │  (embed)  │  │   │
│   │  └─────────────┘  └─────────────┘  └─────────────┘  └───────────┘  │   │
│   │         │                                               │           │   │
│   │         ▼                                               ▼           │   │
│   │  ┌─────────────┐                              ┌─────────────┐       │   │
│   │  │  Auth       │                              │  Collector  │       │   │
│   │  │  (cookie)   │                              │  (raw text) │       │   │
│   │  └─────────────┘                              └─────────────┘       │   │
│   └─────────────────────────────────────────────────────────────────────┘   │
│                              │                                              │
│                              ▼                                              │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │  SQLite/PostgreSQL  │  browser_extension_api_keys, workspaces, ...   │   │
│   └─────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

**核心原则**:
- 后端完全复用现有基础设施（Gin、GORM、Collector、DocumentService）
- 扩展客户端独立构建，输出 `browser-extension/dist/` 供用户手动加载或打包
- 认证与 Telegram 保持对称：专用 API key 体系，独立中间件

---

## 2. 后端组件

### 2.1 数据模型

```go
// backend/internal/models/browser_extension_api_key.go
type BrowserExtensionApiKey struct {
    ID            int       `gorm:"primaryKey;autoIncrement" json:"id"`
    Key           string    `gorm:"unique" json:"key"`
    UserID        *int      `json:"userId"`        // nil in single-user mode
    CreatedAt     time.Time `json:"createdAt"`
    LastUpdatedAt time.Time `json:"lastUpdatedAt"`
}
```

与 anything-llm 的 Prisma schema 对齐：`id`, `key` (unique), `user_id` (nullable), `createdAt`, `lastUpdatedAt`。GORM `AutoMigrate` 自动建表。

### 2.2 专用中间件

```go
// backend/internal/middleware/browser_extension.go
func ValidBrowserExtensionApiKey(
    extSvc *services.BrowserExtensionService,
    authSvc *services.AuthService,
) gin.HandlerFunc
```

行为：
1. 读取 `Authorization` header，提取 `Bearer` token
2. 检查 token 以 `brx-` 开头，调用 `extSvc.Validate(key)`
3. **单用户模式**：直接通过，无需 user 检查
4. **多用户模式**：加载 key 关联的 user，检查 user 存在且未 suspend；设置 `user` + `apiKey` 到 context
5. 任一失败返回 `403`

### 2.3 服务层

```go
// backend/internal/services/browser_extension.go
type BrowserExtensionService struct {
    db *gorm.DB
}

func (s *BrowserExtensionService) CreateKey(userID *int) (*models.BrowserExtensionApiKey, error)
func (s *BrowserExtensionService) ListKeys(userID *int, isAdmin bool) ([]models.BrowserExtensionApiKey, error)
func (s *BrowserExtensionService) DeleteKey(id int, userID *int, isAdmin bool) error
func (s *BrowserExtensionService) Validate(key string) (*models.BrowserExtensionApiKey, error)
func (s *BrowserExtensionService) DeleteAllForUser(userID int) error
```

Key 生成：`brx-` + `uuid.NewString()`。

### 2.4 HTTP 端点（8 个）

| 方法 | 路由 | 中间件 | 功能 |
|------|------|--------|------|
| `GET` | `/api/browser-extension/check` | `ValidBrowserExtensionApiKey` | 验证 key，返回 `{connected:true, workspaces:[], apiKeyId:int}` |
| `DELETE` | `/api/browser-extension/disconnect` | `ValidBrowserExtensionApiKey` | 撤销并删除当前 key |
| `GET` | `/api/browser-extension/workspaces` | `ValidBrowserExtensionApiKey` | 返回用户可见 workspace 列表 |
| `POST` | `/api/browser-extension/embed-content` | `ValidBrowserExtensionApiKey` | `{workspaceId, textContent, metadata}` → 保存 + 嵌入 |
| `POST` | `/api/browser-extension/upload-content` | `ValidBrowserExtensionApiKey` | `{textContent, metadata}` → 仅保存到文档池 |
| `GET` | `/api/browser-extension/api-keys` | `ValidatedRequest` + `FlexUserRoleValid([admin,manager])` | 列出所有 keys |
| `POST` | `/api/browser-extension/api-keys/new` | `ValidatedRequest` + `FlexUserRoleValid([admin,manager])` | 生成新 key |
| `DELETE` | `/api/browser-extension/api-keys/:id` | `ValidatedRequest` + `FlexUserRoleValid([admin,manager])` | 撤销指定 key |

已有端点复用：`/api/ping` 和 `/api/system/logo` 已存在，扩展客户端直接调用。

### 2.5 Admin 级联清理

在 `AdminService.DeleteUser` 中追加：`extSvc.DeleteAllForUser(userID)`，删除用户时一并清理其所有扩展 API keys。

---

## 3. 扩展客户端组件

### 3.1 项目结构

```
browser-extension/
├── public/
│   ├── manifest.json          # Chrome Manifest V3
│   ├── background.js          # Service Worker
│   ├── contentScript.js       # Content Script
│   └── icons/
│       ├── icon16.png
│       ├── icon32.png
│       ├── icon48.png
│       └── icon128.png
├── src/
│   ├── App.jsx                # Popup 根组件
│   ├── components/
│   │   └── Config.jsx         # 连接配置面板
│   ├── hooks/
│   │   └── useApiConnection.js
│   ├── models/
│   │   └── browserExtension.js # API 客户端
│   ├── utils/
│   │   └── constants.js
│   ├── index.css              # Tailwind
│   └── main.jsx               # React root
├── index.html                 # Popup HTML
├── vite.config.js             # 输出到 dist/
├── package.json               # React 18, Vite, Tailwind
└── README.md
```

构建输出：`yarn build` → `browser-extension/dist/`，包含 `index.html` + 打包 JS/CSS、`background.js`、`contentScript.js`、`manifest.json` + icons。

### 3.2 Manifest V3

```json
{
  "manifest_version": 3,
  "name": "Hermind Browser Companion",
  "version": "1.0.0",
  "description": "Save web content to your Hermind knowledge base",
  "permissions": ["contextMenus", "storage", "alarms", "activeTab"],
  "host_permissions": ["<all_urls>"],
  "background": { "service_worker": "background.js" },
  "content_scripts": [
    {
      "matches": ["<all_urls>"],
      "js": ["contentScript.js"],
      "run_at": "document_idle"
    }
  ],
  "action": {
    "default_popup": "index.html",
    "default_icon": {
      "16": "icons/icon16.png",
      "32": "icons/icon32.png",
      "48": "icons/icon48.png",
      "128": "icons/icon128.png"
    }
  }
}
```

### 3.3 Background Service Worker

核心职责（纯 JS，不经过 Vite 打包）：

1. **安装时初始化右键菜单**：基础菜单项（Save selected、Save page、Embed selected、Embed page）
2. **点击处理**：调用对应 API（upload-content / embed-content）
3. **整页保存**：通过 `chrome.scripting.executeScript` 获取 `document.body.innerText`
4. **定时同步**：`chrome.alarms` 每分钟调用 `/browser-extension/check`，重建 Workspace 子菜单
5. **Popup 通信**：监听 `checkConnection`、`disconnect`、`getPageContent` 消息

### 3.4 Content Script

两个职责：
1. 监听 `window.postMessage` 自动注入连接（来自 Web UI Settings 页面）
2. 响应 background 请求提取页面内容（`document.body.innerText`）

### 3.5 React Popup

极简 UI：
- 连接状态（✅ 已连接 / ❌ 未连接 / ⏳ 验证中）
- 连接字符串输入框（`<origin>/api|<brx-key>`）
- 断开按钮
- Logo 显示（从 `/api/system/logo` 获取）
- Workspace 数量预览

### 3.6 前端 Web UI 自动注入

Hermind 前端 `BrowserExtensionApiKey` 设置页面生成 key 后触发：

```javascript
window.postMessage({
  type: "NEW_BROWSER_EXTENSION_CONNECTION",
  apiKey: `${window.location.origin}/api|${apiKey}`
}, "*");
```

Content script 监听并保存到 `chrome.storage.sync`。

---

## 4. 数据流与内容处理

### 4.1 连接建立

```
Web UI Settings Page
  ├─ POST /api/browser-extension/api-keys/new → 生成 brx-<uuid>
  ├─ 拼接连接字符串: `${origin}/api|${key}`
  └─ window.postMessage → Content Script → chrome.storage.sync
       └─ Background SW 验证 → GET /browser-extension/check
            └─ 更新 badge ✅ + 重建右键菜单
```

### 4.2 保存文本（upload-content）

```
User selects text → Right-click → "Save selected to Hermind"
  └─ Background SW → POST /api/browser-extension/upload-content
       { textContent, metadata: { title, url } }
```

服务端调用 `DocumentService.SaveRawText(text, title, metadata, nil)`：**不传入 workspace slugs** → 只保存 JSON 文件到 `custom-documents/`，不创建 `workspace_documents` 记录。文件在"文档管理"中可见，用户可后续手动嵌入。

### 4.3 嵌入文本（embed-content）

```
User selects text → Right-click → "Embed to workspace" → <Workspace>
  └─ Background SW → POST /api/browser-extension/embed-content
       { workspaceId, textContent, metadata: { title, url } }
```

服务端：
1. 验证 workspace 存在且用户有权限
2. 调用 `SaveRawText(text, title, metadata, [wsSlug])` → 创建 `WorkspaceDocument` 记录
3. 调用 `EmbedDocument(doc)` → 分块、向量化、存入向量数据库

### 4.4 自动同步 Workspace

`chrome.alarms` 每分钟触发 `updateWorkspaces()`：
- GET `/browser-extension/check` 获取最新 workspace 列表
- 重建右键菜单动态子菜单（Embed selected to workspace → [ws1, ws2, ...]）

### 4.5 与 anything-llm 的关键差异

| 差异 | anything-llm | Hermind | 处理方案 |
|------|-------------|---------|----------|
| Collector | 独立 Node.js 进程，HTTP 转发 | Go 包 `internal/collector`，直接调用 | Handler 直接调用 `DocumentService.SaveRawText()` + `EmbedDocument()` |
| 上传文档池 | Collector 处理后存为 `raw-text` 文档 | `SaveRawText` 写 JSON 到 `custom-documents/` | 语义等价，`custom-documents/` 即通用文档池 |
| 嵌入流程 | Collector → `Document.addDocuments()` → vectorize | `SaveRawText` + `EmbedDocument` | 两步合并为 handler 内直接调用 |
| Telemetry | `Telemetry.sendTelemetry("browser_extension_*")` | `mlog` 结构化日志 | 记录日志，不阻塞用户操作 |

---

## 5. 错误处理与测试

### 5.1 错误码

| 场景 | HTTP Status | Response |
|------|-------------|----------|
| brx key 格式错误 / 不存在 | `403` | `{ error: "Invalid browser extension API key" }` |
| brx key 关联用户被 suspend | `403` | `{ error: "User account suspended" }` |
| workspace 不存在 / 无权访问 | `404` | `{ error: "Workspace not found" }` |
| 文本内容为空 | `400` | `{ error: "textContent cannot be empty" }` |
| 内部错误 | `500` | `{ error: "Failed to process content" }` |

### 5.2 扩展客户端错误恢复

- **403 错误**：Key 失效 → 清除 `chrome.storage.sync`，badge 显示 ❌
- **网络错误**：静默重试一次，延迟 1 秒

### 5.3 测试策略

**后端测试**（Go）：

| 测试文件 | 覆盖范围 |
|----------|----------|
| `services/browser_extension_test.go` | `CreateKey`, `Validate`, `DeleteKey`, `ListKeys`, `DeleteAllForUser` |
| `middleware/browser_extension_test.go` | 有效 key、无效 key、suspend 用户、单用户模式 |
| `handlers/browser_extension_test.go` | 8 个端点全路径：正常、权限不足、空内容、无效 workspace、disconnect |

**扩展客户端测试**：

不引入独立测试框架。验证依赖：
1. 手动加载测试：`chrome://extensions/` → Load unpacked
2. 端到端脚本：`scripts/test-extension.sh`
3. 构建检查：`make build-extension` 必须成功

### 5.4 构建集成

新增 Makefile 目标：

```makefile
.PHONY: build-extension
build-extension:
	@cd ../browser-extension && yarn install && yarn build

.PHONY: build-all
build-all: build-extension build
```

扩展构建产物 **不嵌入 Go 二进制**，用户手动从 Chrome Web Store 加载或 side-load。

---

## 6. 交付计划

4-PR 渐进式交付：

| PR | 内容 |
|----|------|
| **PR1** | 后端基础设施：模型、中间件、8 个端点、Admin 级联清理、AutoMigrate |
| **PR2** | 扩展客户端项目：React + Vite + Manifest V3、popup UI、Logo 显示、构建脚本 |
| **PR3** | 扩展客户端功能：Background SW（右键菜单、API、alarms）、Content script（自动注入、页面提取）、动态 Workspace 子菜单 |
| **PR4** | 集成测试 + 构建集成：后端 handler 测试、Makefile 目标、AGENTS.md 更新 |
