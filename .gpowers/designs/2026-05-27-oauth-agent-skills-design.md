# OAuth Agent Skills Design — gmail / google-calendar / outlook

**Date**: 2026-05-27
**Status**: Draft
**Author**: brainstorming session
**Scope**: backend 实现 3 个 Node parity 的 agent skill: `gmail-agent` / `google-calendar-agent` / `outlook-agent`。所有三个都在单用户模式下开放、走 PR-AR-5 的 approval gate、按 PR-AR-6 同款单 tool + action enum 模式打包。**整体一个 PR-AR-7 交付**，约 22-27h。

**先决条件**: PR-AR-1 ~ PR-AR-6 全部已合(@agent runtime + tool registry + 4+3 默认 skill + safeJoin + approval gate)。

---

## 1. 关键事实(读源码后的修正)

> Node 团队把这三个 skill 起名"OAuth 三件套"是**误导**: 实际只有 1 个真 OAuth + 2 个 Apps Script 桥接。

| Skill | Node 真实架构 | Hermind 持有的凭据 | Go 复刻成本 |
|---|---|---|---|
| **gmail** | Google Apps Script 桥接 | `deploymentId + apiKey` (HTTP POST → script.google.com) | ~4h |
| **google-calendar** | 同款 Apps Script 桥接 | `deploymentId + apiKey` | ~3h |
| **outlook** | Microsoft OAuth 2.0 | `clientId + clientSecret + refreshToken + tenant` (自管 authorize/callback/refresh) | ~15-20h |

**Apps Script 桥接的 wire format** (`server/utils/agents/aibitat/plugins/gmail/lib.js:339`):

```http
POST https://script.google.com/macros/s/<deploymentId>/exec
Content-Type: application/json
X-Hermind-UA: Hermind-Gmail-Agent/1.0

{"key": "<apiKey>", "action": "search", "query": "is:inbox", "limit": 10}
```

响应:

```json
{"status": "ok"|"error", "data": {...}, "error": "..." }
```

> Apps Script 那一侧是一段 Google 内运行的 JavaScript,持有部署者的 Google 身份(GmailApp.* / CalendarApp.*),Hermind 服务器**不持有任何 Google access_token / refresh_token**。这就是为什么 Gmail/GCal 实现成本只有 outlook 的 1/4。

---

## 2. 目标与边界

### 2.1 目标

- 三个 skill 在单用户模式下注册到 default-skills(PR-AR-3 Source 1),CheckFn 受配置就绪性 + multi-user-mode 双重 gate
- 所有"会写/会发"动作(send/createDraft/createEvent 等)走 PR-AR-5 的 approval gate
- 共享 `BridgeClient`(Apps Script HTTP 调度)和 `TokenStore`(Outlook OAuth)两个基础设施
- OAuth callback 走 `GET /api/oauth/outlook/callback` + session token 鉴权(已选)
- token 加密落库,复用 `pkg/utils/EncryptionManager.Encrypt/Decrypt` (AES-GCM)
- 自带 Apps Script 模板 `assets/apps-script/{gmail,gcal}/Code.gs` + 部署 README

### 2.2 非目标(v1)

- 不支持多用户 OAuth(每 user 一份凭据): 按 `MULTI_USER_MODE=true` 直接禁用,与 Node 一致 (decision: [2026-05-27-oauth-single-user-only](#))
- 不复用 Node 已部署的 Apps Script: Go 提供新模板,管理员重新部署 (decision: [2026-05-27-apps-script-go-template](#))
- 不实现 attachment 上传/下载(Node 通过 `prepareAttachment` 走 collector,Go 暂不接 collector → Phase 2)
- 不做"自动换 access_token 拿 OneDrive 文件"等扩展能力(只做 mail send/read/draft 主链路)
- 不做 OAuth client_id/secret 的 admin UI(前端走"填表 → POST SystemSetting"模式)
- 不做并发 refresh_token 锁(单用户 + 单 session 串行,加 sync.Mutex 即可,不上 DB lock)

---

## 3. Go 架构

### 3.1 包布局

```
backend/internal/agent/tools/
├── oauth/
│   ├── doc.go
│   ├── bridge_client.go             # 共享 Apps Script HTTP client
│   ├── bridge_client_test.go
│   ├── outlook_oauth.go             # AuthorizeURL/ExchangeCode/RefreshToken
│   ├── outlook_oauth_test.go        # 用 httptest 桩 login.microsoftonline.com
│   ├── outlook_token_store.go       # 加密 token CRUD
│   ├── outlook_token_store_test.go
│   ├── state.go                     # HMAC(userID|nonce|return_to|exp) 编/解
│   └── state_test.go
├── gmail_agent.go                   # one fat tool, action enum
├── gmail_agent_test.go
├── gcal_agent.go
├── gcal_agent_test.go
├── outlook_agent.go
├── outlook_agent_test.go
└── builder.go                       # MODIFY — 注册 3 个新 skill

backend/internal/handlers/
├── oauth.go                         # NEW — GET authorize + GET callback
└── oauth_test.go

backend/internal/models/
└── outlook_oauth_token.go           # NEW — gorm 模型

backend/internal/config/config.go  # MODIFY — Outlook redirect_uri base

backend/assets/apps-script/
├── gmail/Code.gs                    # NEW — Apps Script 模板
├── gmail/README.md
├── gcal/Code.gs
└── gcal/README.md
```

### 3.2 类型签名

```go
// internal/agent/tools/oauth/bridge_client.go

type BridgeClient struct {
    httpClient *http.Client
    timeout    time.Duration
}

func NewBridgeClient() *BridgeClient
func (b *BridgeClient) Call(ctx context.Context, deploymentID, apiKey, action string, params map[string]any) (json.RawMessage, error)

// internal/agent/tools/oauth/outlook_oauth.go

type OutlookOAuth struct {
    enc           *utils.EncryptionManager
    store         *TokenStore
    redirectURI   string                  // 启动时算好 (cfg.PublicBaseURL + /api/oauth/outlook/callback)
    authority     string                  // "common" | "consumers" | tenant ID
    httpClient    *http.Client
    refreshMu     sync.Mutex              // single-user 模式下保护并发 refresh
}

func NewOutlookOAuth(enc *utils.EncryptionManager, store *TokenStore, cfg *config.Config) *OutlookOAuth
func (o *OutlookOAuth) AuthorizeURL(state string, clientID string) string
func (o *OutlookOAuth) ExchangeCode(ctx context.Context, code, clientID, clientSecret string) (*TokenSet, error)
func (o *OutlookOAuth) ValidAccessToken(ctx context.Context, userID int) (string, error)  // 自动 refresh

// internal/agent/tools/oauth/outlook_token_store.go

type TokenSet struct {
    AccessToken  string
    RefreshToken string
    ExpiresAt    time.Time
    Tenant       string
}

type TokenStore struct {
    db  *gorm.DB
    enc *utils.EncryptionManager
}

func NewTokenStore(db *gorm.DB, enc *utils.EncryptionManager) *TokenStore
func (s *TokenStore) Get(ctx context.Context, userID int) (*TokenSet, error)
func (s *TokenStore) Save(ctx context.Context, userID int, ts *TokenSet) error
func (s *TokenStore) Delete(ctx context.Context, userID int) error

// internal/agent/tools/oauth/state.go

type StatePayload struct {
    UserID   int
    Nonce    string
    ReturnTo string
    ExpiresAt int64  // unix seconds
}

func EncodeState(secret []byte, p StatePayload) string             // base64url(json + HMAC)
func DecodeState(secret []byte, encoded string) (*StatePayload, error)  // verify HMAC + exp
```

### 3.3 Skill 注册形状(与 PR-AR-6 同款)

每个 skill 是**一个 `tool.Entry`**,带 `action` enum,在 Handler 内分发到 BridgeClient 或 OutlookOAuth。

```go
// gmail_agent.go (摘要)
func NewGmailAgentSkill(tc *ToolContext, deps BuilderDeps) *tool.Entry {
    return &tool.Entry{
        Name: "gmail-agent",
        Toolset: "gmail",
        CheckFn: func() bool {
            return !deps.Cfg.MultiUserMode &&
                hasGmailConfig(tc.Settings["gmail_agent_config"])
        },
        MaxResultChars: 16 * 1024,
        Schema: gmailSchema(),
        Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
            var args struct { Action string `json:"action"`; /* ... */ }
            json.Unmarshal(raw, &args)
            // approval gate on destructive actions
            if isDestructiveGmailAction(args.Action) && tc.Approval != nil {
                if ok, reason := tc.Approval(ctx, "gmail-agent:"+args.Action, args, "..."); !ok {
                    return tool.Error("rejected: " + reason), nil
                }
            }
            cfg := parseGmailConfig(tc.Settings["gmail_agent_config"])
            res, err := deps.Bridge.Call(ctx, cfg.DeploymentID, cfg.APIKey, args.Action, mapFromArgs(args))
            if err != nil { return tool.Error(err.Error()), nil }
            return string(res), nil
        },
    }
}
```

---

## 4. Apps Script Bridge 协议

### 4.1 Wire format (Node parity, 用同名 SystemSetting)

| 维度 | 值 |
|---|---|
| Endpoint | `https://script.google.com/macros/s/<deploymentId>/exec` |
| Method | POST |
| Content-Type | `application/json` |
| UA Header | `X-Hermind-UA: Hermind-<Skill>-Agent-Go/1.0` |
| Body | `{"key": "<apiKey>", "action": "<name>", ...params}` |
| Response | `{"status":"ok","data":{...}}` 或 `{"status":"error","error":"..."}` |
| Timeout | 30s |
| SystemSetting key | `gmail_agent_config` / `google_calendar_agent_config` (与 Node 同名) |
| Setting value | JSON: `{"deploymentId":"...", "apiKey":"..."}` |

### 4.2 Action surface (Go skill 暴露给 LLM 的 action 集合)

**gmail-agent** (12 actions,删减 Node 的偏冷门部分):

| Action | LLM 描述 | 需要 approval |
|---|---|---|
| `search` | Search Gmail using query syntax (e.g. `from:alice is:unread`) | ❌ |
| `read_thread` | Read a full thread by ID | ❌ |
| `list_drafts` | List draft emails | ❌ |
| `get_draft` | Get a draft by ID | ❌ |
| `mailbox_stats` | Get mailbox statistics | ❌ |
| `create_draft` | Create a new draft | ✅ |
| `update_draft` | Update existing draft | ✅ |
| `send_draft` | Send an existing draft | ✅ |
| `send_email` | Compose + send directly | ✅ |
| `reply_to_thread` | Reply to a thread | ✅ |
| `delete_draft` | Delete a draft | ✅ |
| `move_to_trash` | Move thread to trash | ✅ |

**google-calendar-agent** (8 actions):

| Action | 描述 | approval |
|---|---|---|
| `list_calendars` | List all calendars | ❌ |
| `get_calendar` | Get calendar metadata | ❌ |
| `get_event` | Get event details | ❌ |
| `get_events_for_day` | List events on a specific date | ❌ |
| `get_events` | List events in a date range | ❌ |
| `quick_add` | Create event from natural language | ✅ |
| `create_event` | Structured event creation | ✅ |
| `update_event` | Modify existing event | ✅ |

> Node 还有 `set_my_status` / 删事件等,v1 不暴露给 LLM,后续按需补。

### 4.3 Bridge 实现要点

```go
// bridge_client.go
func (b *BridgeClient) Call(ctx context.Context, deploymentID, apiKey, action string, params map[string]any) (json.RawMessage, error) {
    url := "https://script.google.com/macros/s/" + deploymentID + "/exec"
    body := map[string]any{"key": apiKey, "action": action}
    for k, v := range params { body[k] = v }
    payload, _ := json.Marshal(body)

    req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(payload))
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("X-Hermind-UA", "Hermind-Agent-Go/1.0")

    resp, err := b.httpClient.Do(req)
    if err != nil { return nil, fmt.Errorf("bridge call: %w", err) }
    defer resp.Body.Close()
    if resp.StatusCode != 200 {
        return nil, fmt.Errorf("bridge status %d", resp.StatusCode)
    }

    raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))  // 4 MiB cap
    var env struct {
        Status string          `json:"status"`
        Data   json.RawMessage `json:"data"`
        Error  string          `json:"error"`
    }
    if err := json.Unmarshal(raw, &env); err != nil {
        return nil, fmt.Errorf("bridge response not JSON: %w", err)
    }
    if env.Status == "error" {
        return nil, fmt.Errorf("apps script error: %s", env.Error)
    }
    return env.Data, nil
}
```

**关键**:

- 30s timeout (Apps Script 自身有 6 分钟上限,我们留 30s 给典型 mail search)
- 4 MiB response cap(防止超大 mailbox dump)
- HTTP 200 + `status:"error"` **必须**视为错误(Apps Script 自带错误 envelope)

---

## 5. Outlook OAuth 流程

### 5.1 整体序列图

```
┌─────────┐         ┌─────────┐         ┌──────────────────┐         ┌─────────┐
│ Browser │         │  Go API │         │ login.micro...   │         │ Outlook │
└────┬────┘         └────┬────┘         └────────┬─────────┘         └────┬────┘
     │ 1. user clicks    │                       │                        │
     │ "Connect Outlook" │                       │                        │
     ├──GET /authorize──>│                       │                        │
     │                   │ 2. enc state          │                        │
     │                   │ 3. build URL          │                        │
     │<─302 to login─────│                       │                        │
     │                                           │                        │
     │ 4. user enters credentials                │                        │
     ├───────────────────────────────────────────┤                        │
     │                                           │                        │
     │ 5. consents to SCOPES                     │                        │
     ├───────────────────────────────────────────┤                        │
     │                                           │                        │
     │<─302 to callback?code=...&state=... ──────│                        │
     │                                                                    │
     ├─GET /callback ───>│                       │                        │
     │                   │ 6. verify state HMAC  │                        │
     │                   │ 7. POST token endpoint w/ code                 │
     │                   ├──────────────────────>│                        │
     │                   │<──token response ─────│                        │
     │                   │ 8. encrypt+save token │                        │
     │<─302 to return_to─│                       │                        │
     │                                                                    │
     │  ... later, agent invokes outlook-agent ...                        │
     │                   │ 9. ValidAccessToken                            │
     │                   │    if expired: refresh                         │
     │                   ├──POST refresh_token──>│                        │
     │                   │<─new access_token ────│                        │
     │                   │ 10. call Graph API    │                        │
     │                   ├──GET /me/messages──── ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ >│
     │                   │<─response ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ │
```

### 5.2 State 编码 (CSRF + replay 防护)

```go
type StatePayload struct {
    UserID    int    // 当前登录的 admin user (单用户模式下通常是 ID=1 或 0)
    Nonce     string // 16-byte random hex
    ReturnTo  string // 完整 URL,必须以 cfg.PublicBaseURL 开头(防开放重定向)
    ExpiresAt int64  // unix seconds, +10min
}

// EncodeState 返回 base64url(json_payload) + "." + base64url(HMAC-SHA256(secret, json_payload))
// secret 用 cfg.JWTSecret 派生(避免引入新密钥)
func EncodeState(secret []byte, p StatePayload) string {
    raw, _ := json.Marshal(p)
    mac := hmac.New(sha256.New, secret)
    mac.Write(raw)
    sig := mac.Sum(nil)
    return base64.RawURLEncoding.EncodeToString(raw) + "." +
           base64.RawURLEncoding.EncodeToString(sig)
}
```

### 5.3 Authorize URL 构造

```
https://login.microsoftonline.com/<authority>/oauth2/v2.0/authorize?
  client_id=<cfg-or-setting>&
  response_type=code&
  redirect_uri=<cfg.PublicBaseURL>/api/oauth/outlook/callback&
  response_mode=query&
  scope=offline_access Mail.Read Mail.ReadWrite Mail.Send User.Read&
  state=<encoded>&
  prompt=select_account
```

`authority` 来源 (按优先级):
1. SystemSetting `outlook_agent_config.tenant` (admin 在 UI 里手填)
2. fallback `common` (允许个人 + 企业账户混合)

`client_id` / `client_secret`: 来自 `outlook_agent_config` SystemSetting,**不放 env**(env 不易热改),与 Node 一致。

### 5.4 ExchangeCode

POST `https://login.microsoftonline.com/<authority>/oauth2/v2.0/token`
body (`application/x-www-form-urlencoded`):

```
client_id=<>&
client_secret=<>&
code=<from callback>&
redirect_uri=<must match exactly>&
grant_type=authorization_code&
scope=<same SCOPES>
```

响应:

```json
{
  "access_token": "...",
  "refresh_token": "...",
  "expires_in": 3599,
  "token_type": "Bearer"
}
```

写入 token store:

```go
ts := &TokenSet{
    AccessToken:  data.AccessToken,
    RefreshToken: data.RefreshToken,
    ExpiresAt:    time.Now().Add(time.Duration(data.ExpiresIn-60) * time.Second),  // 60s leeway
    Tenant:       authority,
}
store.Save(ctx, userID, ts)
```

### 5.5 ValidAccessToken (自动 refresh)

```go
func (o *OutlookOAuth) ValidAccessToken(ctx context.Context, userID int) (string, error) {
    o.refreshMu.Lock()
    defer o.refreshMu.Unlock()

    ts, err := o.store.Get(ctx, userID)
    if err != nil { return "", err }
    if time.Now().Before(ts.ExpiresAt) {
        return ts.AccessToken, nil
    }
    // Refresh
    newTS, err := o.refresh(ctx, ts.RefreshToken, ts.Tenant)
    if err != nil { return "", fmt.Errorf("refresh failed: %w", err) }
    // Microsoft 偶尔返回新的 refresh_token,偶尔不返回 -- 保留旧的
    if newTS.RefreshToken == "" { newTS.RefreshToken = ts.RefreshToken }
    o.store.Save(ctx, userID, newTS)
    return newTS.AccessToken, nil
}
```

**为什么用 mutex 而不是 DB row lock**: 单用户模式下并发上限就是同时跑的 agent session 数,通常 < 5;mutex 串行 refresh 简单可靠;refresh 本身 200-500ms,影响微乎其微。

### 5.6 OutlookToken 表 schema

```go
// internal/models/outlook_oauth_token.go
type OutlookOAuthToken struct {
    ID                     int       `gorm:"primaryKey"`
    UserID                 int       `gorm:"uniqueIndex;not null"`  // 0 = admin
    Tenant                 string    `gorm:"not null"`              // "common" / "consumers" / tenant-id
    EncryptedAccessToken   string    `gorm:"type:text;not null"`    // AES-GCM via EncryptionManager
    EncryptedRefreshToken  string    `gorm:"type:text;not null"`
    ExpiresAt              time.Time `gorm:"not null"`
    CreatedAt              time.Time
    UpdatedAt              time.Time
}

func (OutlookOAuthToken) TableName() string { return "outlook_oauth_tokens" }
```

`AutoMigrate` 进 `internal/models/db.go`。

---

## 6. HTTP 路由设计

### 6.1 端点列表

| Method | Path | Middleware | 用途 |
|---|---|---|---|
| GET | `/api/oauth/outlook/authorize` | `ValidatedRequest(authSvc)` | 生成 state + 重定向到 Microsoft |
| GET | `/api/oauth/outlook/callback` | 无 (state 自带鉴权) | 接收 code + 交换 token + 落库 |
| POST | `/api/oauth/outlook/disconnect` | `ValidatedRequest(authSvc)` | 删除当前 user 的 token |
| GET | `/api/oauth/outlook/status` | `ValidatedRequest(authSvc)` | 返回 `{connected: bool, expiresAt}` |

### 6.2 Callback handler 行为

```go
func (h *OAuthHandler) OutlookCallback(c *gin.Context) {
    code := c.Query("code")
    encState := c.Query("state")
    if code == "" || encState == "" {
        c.HTML(400, "oauth_error.html", gin.H{"message": "missing code/state"})
        return
    }
    state, err := oauth.DecodeState(h.stateSecret, encState)
    if err != nil { c.HTML(400, "oauth_error.html", gin.H{"message": err.Error()}); return }
    if time.Now().Unix() > state.ExpiresAt {
        c.HTML(400, "oauth_error.html", gin.H{"message": "state expired"}); return
    }

    cfg := loadOutlookConfig(h.sysSvc, c.Request.Context())  // clientId/clientSecret
    ts, err := h.outlook.ExchangeCode(c.Request.Context(), code, cfg.ClientID, cfg.ClientSecret)
    if err != nil { c.HTML(500, "oauth_error.html", gin.H{"message": err.Error()}); return }
    if err := h.tokenStore.Save(c.Request.Context(), state.UserID, ts); err != nil { /* ... */ }

    c.Redirect(302, state.ReturnTo)
}
```

> Open-redirect 防御: `DecodeState` 内验证 `state.ReturnTo` 必须以 `cfg.PublicBaseURL` 开头。

### 6.3 错误页面

`/api/oauth/outlook/callback` 失败时渲染最小 HTML (不能 302 回前端,因为可能是 state 篡改):

```html
<!doctype html>
<html><body>
<h1>OAuth Error</h1>
<p>{{.message}}</p>
<p>Please close this window and try again.</p>
</body></html>
```

不引入模板引擎,用 `c.String(status, html)` 直出。

---

## 7. Apps Script 模板

### 7.1 设计原则

- Go 项目自带模板,**不复用 Node 部署**,因为协议表面属于 Go 项目自治
- 模板与 Node 的 Apps Script 协议保持兼容(同 action 名 + 同响应 envelope),理论上 admin 可以 fallback 用 Node 部署
- 模板包含: Code.gs (主代码) + appsscript.json (manifest) + README.md (一步步部署指南 + 截图)

### 7.2 gmail/Code.gs 关键片段

```javascript
const VALID_API_KEY = PropertiesService.getScriptProperties().getProperty('API_KEY');

function doPost(e) {
  try {
    const req = JSON.parse(e.postData.contents);
    if (req.key !== VALID_API_KEY) {
      return jsonResponse({status: 'error', error: 'invalid api key'});
    }
    const dispatchTable = {
      search: handleSearch,
      read_thread: handleReadThread,
      send_email: handleSendEmail,
      // ... 12 action 全集
    };
    const handler = dispatchTable[req.action];
    if (!handler) {
      return jsonResponse({status: 'error', error: 'unknown action: ' + req.action});
    }
    const data = handler(req);
    return jsonResponse({status: 'ok', data: data});
  } catch (err) {
    return jsonResponse({status: 'error', error: err.toString()});
  }
}

function handleSearch(req) {
  const threads = GmailApp.search(req.query, req.start || 0, req.limit || 10);
  return threads.map(t => ({
    id: t.getId(),
    subject: t.getFirstMessageSubject(),
    snippet: t.getMessages()[0].getPlainBody().substring(0, 200),
    lastMessageDate: t.getLastMessageDate().toISOString(),
  }));
}

function jsonResponse(obj) {
  return ContentService.createTextOutput(JSON.stringify(obj))
    .setMimeType(ContentService.MimeType.JSON);
}
```

### 7.3 README 部署步骤(摘要)

```markdown
1. 打开 https://script.google.com → New Project
2. 粘贴 Code.gs 全部内容
3. Project Settings → Script Properties → 添加 `API_KEY` = <自己生成的随机字符串>
4. Deploy → New Deployment → Web app
   - Description: Hermind Gmail Bridge
   - Execute as: Me
   - Who has access: Anyone with the link
5. Authorize when prompted (会弹一次 Google OAuth 让脚本拿到你的 Gmail 权限)
6. 复制 deployment ID(URL 形如 .../macros/s/<deploymentID>/exec)
7. 回到 Hermind → Agent Settings → Gmail Agent
   - Deployment ID: <粘贴>
   - API Key: <粘贴第 3 步的随机字符串>
8. Test → 在 @agent 对话里说 "@agent search my inbox for emails from boss"
```

GCal README 同款,只替换 GmailApp → CalendarApp、scope 注释。

---

## 8. SystemSetting schema

### 8.1 Gmail / GCal 凭据

| Key | Value (JSON) | 写入方式 |
|---|---|---|
| `gmail_agent_config` | `{"deploymentId": "...", "apiKey": "..."}` | 现有 `POST /api/system/setting` (Node + Go 同 schema,**双进程部署期 Node 写 Go 读零冲突**) |
| `google_calendar_agent_config` | 同上 | 同上 |

### 8.2 Outlook 配置

| Key | Value | 备注 |
|---|---|---|
| `outlook_agent_config` | `{"clientId":"...","clientSecret":"...","tenant":"common"}` | clientSecret 也走 EncryptionManager 加密(因为 SystemSetting 表通常不加密) |

**SystemSetting 加密的 marker**: 在 value 前缀加 `enc:` 标识 (例如 `{"clientId":"x","clientSecret":"enc:AESGCM(...)"}`),`loadOutlookConfig` 时检测前缀决定是否 Decrypt。这是 Node 已有的模式,与现有 system_setting handler 兼容。

---

## 9. CheckFn 与单用户 gate

每个 skill 的 `CheckFn` 必须同时通过两道闸:

```go
func gmailCheckFn(deps BuilderDeps, settings map[string]string) func() bool {
    return func() bool {
        if deps.Cfg.MultiUserMode { return false }   // 1. 单用户模式
        raw := settings["gmail_agent_config"]
        var cfg struct { DeploymentID, APIKey string }
        if err := json.Unmarshal([]byte(raw), &cfg); err != nil { return false }
        return cfg.DeploymentID != "" && cfg.APIKey != ""  // 2. 配置完整
    }
}
```

Outlook 多一道:

```go
// outlook CheckFn 还要确认 token 已存在(否则 LLM 看到工具调不通会困惑)
ts, err := deps.OutlookStore.Get(ctx, currentUserID)
return err == nil && ts != nil
```

---

## 10. 测试策略

### 10.1 单元

| 文件 | 关键 case |
|---|---|
| `bridge_client_test.go` | `httptest.NewServer` mock Apps Script,验证: 200 ok / 200 error envelope / 5xx / 超时 / 4 MiB cap |
| `outlook_oauth_test.go` | mock `login.microsoftonline.com`: code exchange 成功 / refresh 成功 / refresh 失败 / response 含/不含新 refresh_token |
| `outlook_token_store_test.go` | encrypt+save+get 往返,token 旧版本读出来,delete idempotent |
| `state_test.go` | encode/decode 往返 / 篡改 nonce 检测 / 过期检测 / return_to 不以 PublicBaseURL 开头被拒 |

### 10.2 集成

`oauth_e2e_test.go`:

1. seed 一个 user
2. mock login.microsoftonline.com 起 httptest,把 cfg.PublicBaseURL 指过去(测试用)
3. 走 `GET /api/oauth/outlook/authorize` → 校验 302 目标含正确 state + scope
4. 模拟用户同意,httptest 服务器返回 302 → `GET /api/oauth/outlook/callback?code=...&state=...`
5. 校验 token 已落库
6. 调 outlook-agent skill → ValidAccessToken → 无需 refresh 直接返回
7. 把 expiresAt 改成过去 → 再调 → refresh 触发 → 新 token 落库

### 10.3 Skill e2e

每个 skill 至少 1 个 happy path + 1 个 approval-rejected 测试,通过 `tool.Registry.Dispatch`,使用 mock BridgeClient/OutlookOAuth。

---

## 11. 风险与权衡

| 风险 | 缓解 |
|---|---|
| Apps Script 部署文档不清晰 → admin 卡在 OAuth 同意页 | README 配截图,常见问题区写"Execute as / Who has access"两个选项的正确组合 |
| Apps Script 部署被 Google 限流 (script.google.com 每天 quota) | 每个 skill 自己有 quota,文档提示;Bridge response cap 也间接保护 |
| OAuth state 用 cfg.JWTSecret 作 HMAC key,JWT 旋转会让正在进行的 OAuth 流失败 | state TTL 10 分钟,JWT 旋转触发的影响窗口极短;文档提示"先停用 OAuth 再旋转 JWT" |
| `refresh_token` 加密密钥泄漏 | EncryptionManager 用 cfg.SigKey + cfg.SigSalt 派生,与现有 auth_token / api_key 同等级保护 |
| 多并发 agent session 同时触发 refresh → 重复消耗 refresh_token | `sync.Mutex` 串行化;Microsoft 通常允许同 refresh_token 多次使用,但不保证;mutex 是稳妥做法 |
| Microsoft 偶尔返回新 refresh_token(rotating) | `ValidAccessToken` 内: `if newTS.RefreshToken == "" { newTS.RefreshToken = oldTS.RefreshToken }`,兼容两种情况 |
| OAuth callback 错误页 XSS | 用 `c.String(status, html)` 配 `template.HTMLEscapeString(message)`,不直接拼字符串 |
| 单用户模式下,user.ID = 0 (admin bypass) vs user.ID = 1 (有 password) 混在一起 | TokenStore.UserID 用 `int` 同步存 0 或正整数;Get 时按调用方 user 取即可 |
| return_to 开放重定向 | DecodeState 内强制 `strings.HasPrefix(state.ReturnTo, cfg.PublicBaseURL)`,否则用 cfg.PublicBaseURL+"/" 兜底 |

---

## 12. 分期内容(此文档对应一个 PR)

PR-AR-7 共 7 个 Task,详见 `.gpowers/plans/2026-05-27-agent-runtime-pr7-oauth-skills.md` (待写):

| Task | 内容 | 工时 |
|---|---|---|
| 0 | Decision artefacts + go.mod + config knobs + EncryptionManager 复用验证 | 2h |
| 1 | BridgeClient + 单元测试 | 2h |
| 2 | gmail-agent skill + 12 action + e2e | 4h |
| 3 | google-calendar-agent skill + 8 action + e2e | 3h |
| 4 | Outlook OAuth 实现(state + AuthorizeURL + ExchangeCode + Refresh + TokenStore + AutoMigrate) | 6h |
| 5 | OAuth handler 路由(authorize/callback/disconnect/status)+ e2e | 4h |
| 6 | outlook-agent skill + 5 action + Builder 注册 + 单用户 gate + 最终验收 | 4h |
| Apps Script 模板 + README + 截图 | 跨 Task 2/3 | (含在内) |

**总计 25h**(范围 22-27h,中段)。

---

## 13. 后续(不在 PR-AR-7 范围)

- Attachment 支持(通过 collector 上传 + Apps Script 接收 base64)
- 多用户 OAuth(每 user 独立 client_id 或共享 client_id + tokenStore.UserID 多行)
- 移除 Apps Script 桥接,改 Google OAuth 2.0 真实接 Gmail / Calendar API(需要 Google Cloud Console 项目 + audit + 隐私政策)
- Microsoft Graph API 扩展能力 (Calendar / OneDrive / Teams)
- OAuth client_id/secret 的前端管理 UI

—— end of design
