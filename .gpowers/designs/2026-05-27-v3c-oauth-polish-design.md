# v3-C — OAuth Polish Pack Design

**Date**: 2026-05-27
**Status**: Draft
**Author**: brainstorming session
**Scope**: 一次性吃掉 PR-AR-5 / PR-AR-7 / 多个 plan 显式 punt 的"OAuth 完善尾巴",共 6 块:
1. **Multi-user OAuth**(per-user Outlook token,移除单用户限制)
2. **Attachment 上传**(gmail/outlook 通过 collector 解析)
3. **Outlook 15 个延后 action**(reply/getInbox/markRead/markUnread/deleteDraft/sendDraft/listDrafts/updateDraft/getDraft/replyToMessage 等)
4. **AgentSkillWhitelist**(per-user tool approval whitelist,替换/补充 PR-AR-5 的"全局 auto-approve 开关")
5. **OAuth `clientSecret` 落库加密**(PR-AR-7 punt 的 `enc:` 前缀模式)
6. **OAuth refresh DB advisory lock**(PR-AR-7 punt 的多进程并发保护)
7. **WS CORS 多 origin 白名单细化**(PR-AR-5 punt)

**Total estimate**: 15-18h(一个 PR 全包,因为这些 item 都是"小但碎",分多 PR 不划算)

**先决条件**: PR-AR-5 + PR-AR-7 已合(本设计基于他们留的 hook 点)。

---

## 1. 现状盘点(实测 5/27)

### 1.1 Multi-user OAuth gate

| 现状位置 | 当前行为 |
|---|---|
| `internal/agent/tools/oauth/outlook_oauth.go` | `TokenStore` schema 已有 `UserID` 字段并加 `uniqueIndex`,**底座已支持多用户** |
| `internal/agent/tools/outlook_agent.go` `CheckFn` | `if deps.Cfg.MultiUserMode { return false }` 显式拒绝多用户 |
| `internal/agent/tools/gmail_agent.go` / `gcal_agent.go` `CheckFn` | 同款拒绝 |
| `internal/handlers/oauth.go` `OutlookAuthorize` | state 携带 user.ID,callback 也按 user.ID 落库 — **已正确实现 per-user**,只是没启用 |

**结论**:基建已经做完,只需移除 3 个 `MultiUserMode` 短路 + 前端 per-user 设置 UI(本 PR 文档化前端工作但不实施)。

### 1.2 Attachment 上传

| 现状 | 情况 |
|---|---|
| `internal/collector` 包 | 已存在(用于文档解析) |
| Node `prepareAttachment` + `parseAttachment` | 通过 collector parse → 写入 chat citations |
| Go gmail/outlook agent | 无 attachment 支持 |

### 1.3 Outlook 15 action

PR-AR-7 实装 5 个:`search` / `read_thread` / `read_message` / `create_draft` / `send_email`。

Node `OutlookBridge` 全部 callable(实测 20 个):
- 已实:5
- 待补:`getInbox` / `createDraftReply` / `getDraft` / `listDrafts` / `updateDraft` / `deleteDraft` / `sendDraft` / `replyToMessage` / `markRead` / `markUnread`(以及 OAuth-helper 类的 5 个不暴露给 LLM)

→ 补 **10 个 LLM-facing action**(`getAuthUrl` 等 OAuth-helper 不算)。

### 1.4 AgentSkillWhitelist

Node `models/agentSkillWhitelist.js` 完整逻辑:

```
- _getLabel(userId): 
    multi-user: "user_<id>_whitelisted_agent_skills"  
    single-user: "whitelisted_agent_skills"
- get(userId): SystemSetting → JSON 数组
- add/remove/isWhitelisted: 标准 CRUD
- clearSingleUserWhitelist: 切多用户时清空
```

Go 现状:**完全无实装**;PR-AR-5 用 `agent_tool_auto_approve` 单一全局开关代替。

### 1.5 OAuth clientSecret 加密

PR-AR-7 design §8.2 写要在 SystemSetting value 用 `enc:` 前缀标识加密字段;**plan Task 5 标 `enc:` 模式 punt 到 follow-up**。Go 当前把 `outlook_agent_config.clientSecret` 明文存 SystemSetting 表。

### 1.6 OAuth refresh DB advisory lock

PR-AR-7 design §5.5 用 `sync.Mutex` 串行 refresh — 单进程有效。多进程部署时无效。PR-AR-7 risk 表显式 punt。

### 1.7 WS CORS 多 origin

PR-AR-5 实装 `buildCheckOrigin(cfg)`:支持 `""`(same-host) / `"*"`(allow-any+warn) / CSV exact match。**但缺一种模式**:`*.example.com` 通配域名。design §5.1 punt。

---

## 2. 目标与边界

### 2.1 目标

- Multi-user OAuth:`MultiUserMode==true` 不再使 3 个 OAuth skill 隐藏;each user 走自己的 `/api/oauth/outlook/authorize?user_id=X` 流程;Token 表已支持多行(uniqueIndex on user_id),只需调整 CheckFn
- Attachment(gmail/outlook send):用户可在 args 里传 `attachments: [{filename, data: base64}]`,server 端走 collector parse → 嵌入 mail body 或作为附件元数据
- Outlook 10 action 补齐,与 Node 1:1 对齐
- AgentSkillWhitelist:实现 Go 端 `services.AgentSkillWhitelistService`,接入 PR-AR-5 的 `Builder.addWithApproval` — whitelisted skill 不走 approval gate;CRUD HTTP routes 暴露给前端管理 UI
- OAuth `clientSecret` 加密:`enc:` 前缀 marker;读 SystemSetting 时检测前缀决定是否 Decrypt
- DB advisory lock:对 `outlook_oauth_tokens` 表用 `SELECT ... FOR UPDATE`(postgres/mysql)或 GORM `Transaction` + `Locking` 实现 row-level lock;sqlite 走 immediate-mode transaction
- WS CORS:`buildCheckOrigin` 加 `*.example.com` 通配模式

### 2.2 非目标(本 PR)

- 前端 per-user OAuth UI — 文档化要做的事,不实施(独立 FE PR)
- gmail/outlook attachment 的 binary 发送(只支持文本上下文形式)
- AgentSkillWhitelist 的 admin 全局 UI — 后端 API 提供完毕,FE 单独
- 旧的 `agent_tool_auto_approve` 全局开关移除 — 保留向后兼容,与 whitelist 共存(任一启用即放行)
- Outlook 其它生产用 action(批量删除 / 移动 folder 等)
- 多 OAuth provider(Slack / Discord / 等扩展)

---

## 3. 子系统设计

### 3.1 Multi-user OAuth

**改动:仅 3 行 + 前端 doc**

```go
// internal/agent/tools/gmail_agent.go (current)
CheckFn: func() bool {
    if deps.Cfg.MultiUserMode { return false }  // ← 删
    /* ... */
}

// outlook_agent.go / gcal_agent.go 同款
```

替换为:

```go
CheckFn: func() bool {
    // 多用户模式需要 per-user 配置;CheckFn 在 Builder.Build 时调用,
    // 那时 tc.User 已经填好,所以 multi-user 也能用。
    if tc.User == nil { return false }
    // Outlook 还要检查 token
    if deps.OutlookStore != nil {
        _, err := deps.OutlookStore.Get(tc.Ctx, tc.User.ID)
        if err != nil { return false }
    }
    return /* config-present check */
}
```

`OAuthHandler.OutlookAuthorize` 已经按 `c.MustGet("user")` 走,zero change。

**前端 work(文档化,不实施)**:Settings → OAuth → 每个 user 看到自己的"Connect Outlook"按钮,而不是 admin-only 一个全局按钮。

### 3.2 Attachment 上传

**args 形状扩展**:

```json
{
  "action": "send_email",
  "to": "boss@example.com",
  "subject": "Q4 Report",
  "body": "See attached.",
  "attachments": [
    {"filename": "report.pdf", "data_base64": "JVBERi0xLjQ..."},
    {"filename": "data.xlsx", "data_base64": "UEsDBBQ..."}
  ]
}
```

**流程**(Node parity):

```
1. Handler 内,decode base64 → bytes
2. 走 collector.ParseDocument(filename, bytes) 拿到 parsed text
3. 在 mail body 末尾插入:"\n\n--- Attached file: report.pdf ---\n<parsed content>\n"
4. 调 Apps Script (gmail) 或 Graph API (outlook) 不携带二进制 attachment,只发增强的 body
```

> **简化**:不直接发 binary attachment 上行(实际 Graph/Apps Script 支持二进制 attachment 但 schema 复杂);v1 走"parse 成文本 inline"的简化路径。Node 端也是这么做的。

**Helper**(放 `oauth/attachment.go`):

```go
type Attachment struct {
    Filename   string
    DataBase64 string
}

func ParseAttachments(ctx context.Context, coll *collector.Client, atts []Attachment) (string, error) {
    var out strings.Builder
    for _, a := range atts {
        data, err := base64.StdEncoding.DecodeString(a.DataBase64)
        if err != nil { return "", err }
        if len(data) > 10<<20 {  // 10 MiB cap
            return "", fmt.Errorf("attachment %s exceeds 10 MiB", a.Filename)
        }
        parsed, err := coll.ParseInMemory(ctx, a.Filename, data)
        if err != nil { return "", err }
        fmt.Fprintf(&out, "\n\n--- Attached file: %s ---\n%s\n", a.Filename, parsed)
    }
    return out.String(), nil
}
```

> `collector.ParseInMemory` 可能不存在 — 实施时 grep 验证 `internal/collector/`;若仅有 `ParseURL/ParseUpload`,先扩 collector 包加 `ParseInMemory`。

### 3.3 Outlook 10 action 补齐

PR-AR-7 已实装的:`search` / `read_thread` / `read_message` / `create_draft` / `send_email`。

补齐:

| Action | Graph API path | 是否需要 approval | 备注 |
|---|---|---|---|
| `get_inbox` | `GET /me/mailFolders/inbox/messages?$top=N` | ❌ | 读 |
| `list_drafts` | `GET /me/mailFolders/drafts/messages?$top=N` | ❌ | 读 |
| `get_draft` | `GET /me/messages/{id}` | ❌ | 读 |
| `update_draft` | `PATCH /me/messages/{id}` | ✅ | 改 |
| `delete_draft` | `DELETE /me/messages/{id}` | ✅ | 删 |
| `send_draft` | `POST /me/messages/{id}/send` | ✅ | 发 |
| `create_draft_reply` | `POST /me/messages/{id}/createReply` | ✅ | 写 |
| `reply_to_message` | `POST /me/messages/{id}/reply` | ✅ | 发 |
| `mark_read` | `PATCH /me/messages/{id}` body `{"isRead":true}` | ✅ | 改 |
| `mark_unread` | `PATCH /me/messages/{id}` body `{"isRead":false}` | ✅ | 改 |

**实装方式**:在 `outlook_agent.go` 已有 switch 加 10 个 case;每个 case 内构造 Graph URL + body,调用 `graphGET/POST/PATCH/DELETE`(后两个需要新增 helper)。

### 3.4 AgentSkillWhitelist

**Go 端实现**:

```go
// internal/services/agent_skill_whitelist.go
type AgentSkillWhitelistService struct {
    sysSvc *SystemService
}

func NewAgentSkillWhitelistService(sysSvc *SystemService) *AgentSkillWhitelistService { /* ... */ }

func (s *AgentSkillWhitelistService) label(userID *int) string {
    if userID == nil || *userID == 0 { return "whitelisted_agent_skills" }
    return fmt.Sprintf("user_%d_whitelisted_agent_skills", *userID)
}

func (s *AgentSkillWhitelistService) Get(ctx context.Context, userID *int) ([]string, error) {
    raw, err := s.sysSvc.GetSetting(ctx, s.label(userID))
    if err != nil || raw == "" { return nil, nil }
    var arr []string
    if err := json.Unmarshal([]byte(raw), &arr); err != nil { return nil, nil }
    return arr, nil
}

func (s *AgentSkillWhitelistService) Add(ctx context.Context, userID *int, skill string) error {
    list, _ := s.Get(ctx, userID)
    for _, x := range list { if x == skill { return nil } }
    list = append(list, skill)
    raw, _ := json.Marshal(list)
    return s.sysSvc.SetSetting(ctx, s.label(userID), string(raw))
}

func (s *AgentSkillWhitelistService) Remove(ctx context.Context, userID *int, skill string) error { /* ... */ }
func (s *AgentSkillWhitelistService) IsWhitelisted(ctx context.Context, userID *int, skill string) bool { /* ... */ }
```

**Builder 接入(PR-AR-5 hook 扩展)**:

```go
// builder.go addWithApproval 改造
func (b *Builder) addWithApproval(reg *tool.Registry, seen map[string]string, e *tool.Entry, source string, requiresApproval bool, globalAutoApprove bool, whitelist []string) {
    if requiresApproval && !globalAutoApprove && !contains(whitelist, e.Name) && b.deps.Approval != nil {
        // wrap as before
    }
    // (else: bypass gate)
    // 还需要 wrap 时报告 reason "auto-approved (whitelisted)" 给 audit log
}
```

`Builder.Build` 接 `whitelistSvc`,在构造时一次性拉取 user 的 whitelist:

```go
whitelist, _ := b.deps.WhitelistSvc.Get(ctx, &user.ID)
```

**HTTP 路由**(给前端管理 UI 用):

```
GET    /api/agent-skill-whitelist               → 返回 caller user 的 whitelist  
POST   /api/agent-skill-whitelist               → body {skill} 添加
DELETE /api/agent-skill-whitelist/:skill        → 移除
GET    /api/admin/agent-skill-whitelist/:userId → admin 看任意 user(MultiUserMode)
```

### 3.5 OAuth clientSecret 加密

**Marker 协议**:SystemSetting value 是 JSON,内含 `"clientSecret"` 字段 → 写入时改为 `"clientSecret":"enc:<base64-AES-GCM>"`,读取时检测前缀。

**Helper**:

```go
// internal/services/encrypted_settings.go
const EncryptedPrefix = "enc:"

func (s *SystemService) GetSecretField(ctx context.Context, settingKey, jsonField string, enc *utils.EncryptionManager) (string, error) {
    raw, err := s.GetSetting(ctx, settingKey)
    if err != nil || raw == "" { return "", err }
    var obj map[string]any
    if err := json.Unmarshal([]byte(raw), &obj); err != nil { return "", err }
    v, _ := obj[jsonField].(string)
    if strings.HasPrefix(v, EncryptedPrefix) {
        return enc.Decrypt(strings.TrimPrefix(v, EncryptedPrefix))
    }
    return v, nil
}

func (s *SystemService) SetSecretField(ctx context.Context, settingKey, jsonField, plaintext string, enc *utils.EncryptionManager) error {
    raw, _ := s.GetSetting(ctx, settingKey)
    var obj map[string]any
    if raw != "" { _ = json.Unmarshal([]byte(raw), &obj) }
    if obj == nil { obj = map[string]any{} }
    ciphertext, err := enc.Encrypt(plaintext)
    if err != nil { return err }
    obj[jsonField] = EncryptedPrefix + ciphertext
    data, _ := json.Marshal(obj)
    return s.SetSetting(ctx, settingKey, string(data))
}
```

**接入点**:

- `handlers/oauth.go` 的 `loadConfig`:用 `GetSecretField("outlook_agent_config", "clientSecret", enc)` 替换直接 JSON 解构
- 写入入口(api/system/setting POST)对 `outlook_agent_config` / `gmail_agent_config` / `google_calendar_agent_config` 的特定字段(`clientSecret` / `apiKey`)自动走 `SetSecretField`

**迁移**:启动时扫一遍 SystemSetting,凡是相关 key 且未带 `enc:` 前缀的,加密重写一次(idempotent migration goroutine)。

### 3.6 OAuth refresh DB advisory lock

**问题域**:多 Go 进程同时调 `ValidAccessToken(ctx, userID)` 触发 refresh,**两个进程同时拿旧 refresh_token 调 Microsoft,先到的成功 + token rotate,后到的拿到 invalidated refresh_token 用 → 失败 → 用户被踢下线**。

**方案**:DB row lock(全 SQL 方言通吃):

```go
// internal/agent/tools/oauth/outlook_oauth.go
func (o *OutlookOAuth) ValidAccessToken(ctx context.Context, userID int, clientID, clientSecret string) (string, error) {
    // 用 DB transaction + row lock 替换 sync.Mutex
    var accessToken string
    err := o.store.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
        var row models.OutlookOAuthToken
        // SELECT ... FOR UPDATE on postgres/mysql; sqlite degrades to immediate-tx (single-writer naturally serializes)
        if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
            Where("user_id = ?", userID).First(&row).Error; err != nil {
            return err
        }
        // (recheck expiry inside transaction — another process might have just refreshed)
        if time.Now().Before(row.ExpiresAt) {
            pt, err := o.store.enc.Decrypt(row.EncryptedAccessToken)
            if err != nil { return err }
            accessToken = pt
            return nil
        }
        // refresh
        rt, _ := o.store.enc.Decrypt(row.EncryptedRefreshToken)
        newTS, err := o.refresh(ctx, rt, clientID, clientSecret, row.Tenant)
        if err != nil { return err }
        if newTS.RefreshToken == "" { newTS.RefreshToken = rt }
        // write back (uses same tx)
        at, _ := o.store.enc.Encrypt(newTS.AccessToken)
        rt2, _ := o.store.enc.Encrypt(newTS.RefreshToken)
        tx.Model(&row).Updates(map[string]any{
            "encrypted_access_token": at,
            "encrypted_refresh_token": rt2,
            "expires_at": newTS.ExpiresAt,
        })
        accessToken = newTS.AccessToken
        return nil
    })
    return accessToken, err
}
```

> sqlite 的 `FOR UPDATE` 不支持,但 GORM `clause.Locking` 在 sqlite 上 silently noop,**single-writer mode** 天然串行;实际效果等价。

`refreshMu sync.Mutex` 可以保留(单进程 fast path)或删除(冗余但无害)— 推荐保留作为本地优化。

### 3.7 WS CORS 通配域名

**当前**(PR-AR-5 `cors.go`):

```go
case raw == "*":   /* allow-any */
case raw == "":    /* same-host */
default:           /* CSV exact match */
```

**扩展**:CSV 支持 `*.example.com` 通配条目:

```go
type originMatcher struct {
    exact   map[string]bool
    suffix  []string   // "example.com" matches "a.example.com" / "b.example.com"
}

func parseAllowedOrigins(raw string) *originMatcher {
    m := &originMatcher{exact: map[string]bool{}}
    for _, o := range strings.Split(raw, ",") {
        o = strings.TrimSpace(o)
        if strings.HasPrefix(o, "*.") {
            m.suffix = append(m.suffix, strings.TrimPrefix(o, "*."))
        } else if o != "" {
            m.exact[o] = true
        }
    }
    return m
}

func (m *originMatcher) match(origin string) bool {
    if m.exact[origin] { return true }
    u, err := url.Parse(origin)
    if err != nil { return false }
    for _, suf := range m.suffix {
        if u.Host == suf || strings.HasSuffix(u.Host, "."+suf) { return true }
    }
    return false
}
```

---

## 4. 估算

| 子系统 | 工时 |
|---|---|
| 3.1 Multi-user OAuth gate(3 行 + 测试) | 1h |
| 3.2 Attachment 上传(parse 包 + 集成到 gmail/outlook) | 3h |
| 3.3 Outlook 10 action 补齐 | 3h |
| 3.4 AgentSkillWhitelist(service + builder hook + HTTP 4 routes) | 4h |
| 3.5 clientSecret 加密(helper + 接入 + migration) | 2h |
| 3.6 DB advisory lock(替换 sync.Mutex,sqlite 兼容) | 2h |
| 3.7 WS CORS 通配 | 1h |
| **Total** | **16h**(目标 15-18h ✓) |

---

## 5. 风险与权衡

| 风险 | 缓解 |
|---|---|
| AgentSkillWhitelist 与 PR-AR-5 全局开关并存 → 行为复杂 | 文档化:**两者 OR**(任一启用即放行);新功能优先 whitelist,全局开关进入 deprecation 通道 |
| Multi-user OAuth 暴露 admin client_id/secret 给所有 user | client_id/secret 仍是全局 SystemSetting(admin 配)只共享一份;per-user 只是 token 多行;**这是 Microsoft 推荐的模式** |
| Attachment 10 MiB 上限挡到合理用例 | 文档化;前端 UI 限制 + 友好错误 |
| Outlook 10 action 中 PATCH/DELETE 暴露删除能力 → 误操作风险 | 所有 PATCH/DELETE 都走 approval gate,与 send_email 同等级 |
| `enc:` 前缀迁移 goroutine 启动失败导致部分密钥裸文 | 迁移 idempotent,启动失败 log warning 不阻塞 boot;管理员重启即可重试 |
| Migration race condition(读到一半被改写) | 启动时一次性 migrate,boot 完成后只走 SetSecretField 写路径 |
| DB advisory lock 在 sqlite degrades to single-writer | sqlite 部署本来就是 single-process;无副作用 |
| sync.Mutex + DB lock 双层 — 死锁? | 顺序固定:先 mutex 后 DB tx,不会死锁;两者都是 fast-path 优化,可只留一个 |
| CORS 通配 `*.example.com` 误匹配 `evil-example.com` | 严格 `u.Host == suf || strings.HasSuffix(u.Host, "."+suf)` — 必须以 `.example.com` 结尾才匹配,杜绝前缀注入 |

---

## 6. 分期内容

一个 PR-V3C 全包(16h ≈ 2 工作日,单 PR 合理):

| Task | 内容 | 工时 |
|---|---|---|
| 0 | go.mod check + 4 个 decision artefact + 接线 | 0.5h |
| 1 | Multi-user CheckFn 调整 + 测试 | 1h |
| 2 | Attachment helper + gmail/outlook 集成 | 3h |
| 3 | Outlook 10 action(每 action ~15 分钟) | 3h |
| 4 | AgentSkillWhitelist service + handler + builder hook | 4h |
| 5 | clientSecret enc: 加密 helper + 启动 migration | 2h |
| 6 | DB advisory lock 替换 mutex | 2h |
| 7 | CORS 通配 + 最终验收 | 0.5h |
| **Total** | — | **16h** |

Plan 文件:`.gpowers/plans/2026-05-28-v3c-oauth-polish.md`(待写)

---

## 7. 后续

- 前端 per-user OAuth UI(独立 FE PR)
- AgentSkillWhitelist 前端管理 UI
- 多 OAuth provider(Slack/Discord/Notion 等)
- 旧 `agent_tool_auto_approve` 全局开关 deprecation 通道:文档先标 deprecated,下个大版本移除
- 凭据存储统一加密层(目前 token 用 EncryptionManager,SystemSetting 字段用 `enc:` marker;未来可以统一)
- Audit log 增强:approval skip 因为 whitelist / global toggle,各自记录原因

—— end of design
