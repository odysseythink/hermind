# v3-C — OAuth Polish Pack Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `gpowers:subagent-driven-development` (recommended) or `gpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 一次性吃掉 PR-AR-5 / PR-AR-7 / 多个 plan 显式 punt 的 OAuth 完善尾巴,共 7 块:Multi-user OAuth / Attachment 上传 / Outlook 10 个补齐 action / AgentSkillWhitelist / clientSecret `enc:` 加密 / DB advisory lock / WS CORS 通配。**估算 16h**。

**先决条件**: PR-AR-5 + PR-AR-7 已合(本 plan 基于他们留的 hook 点)。

**Source spec:** `.gpowers/designs/2026-05-27-v3c-oauth-polish-design.md`。

---

## Pre-task

### 现状速查(实测 5/28,所有点位都在 PR-AR-7 后)

| 改动点 | 当前位置 | PR-AR-7 留的状态 |
|---|---|---|
| Multi-user OAuth gate | `gmail_agent.go` / `gcal_agent.go` / `outlook_agent.go` 的 `CheckFn` | `if deps.Cfg.MultiUserMode { return false }` 明文短路 |
| Attachment | `tools/oauth/` | 无 |
| Outlook 5/15 action 已实装 | `outlook_agent.go` 的 switch | `search`/`read_thread`/`read_message`/`create_draft`/`send_email` |
| AgentSkillWhitelist | `internal/services/` | 无 |
| clientSecret 加密 | `handlers/oauth.go` `loadConfig` | 直接 `json.Unmarshal` SystemSetting,**明文** |
| OAuth refresh lock | `outlook_oauth.go` `ValidAccessToken` | `sync.Mutex`(单进程有效) |
| WS CORS wildcard | `internal/agent/cors.go` `buildCheckOrigin` | exact 匹配,无 `*.example.com` 通配 |

### 新增/改动 surface

```
backend/internal/agent/tools/oauth/
└── attachment.go               # NEW — ParseAttachments helper

backend/internal/agent/tools/
├── gmail_agent.go              # MODIFY — multi-user CheckFn + attachment 支持
├── gcal_agent.go               # MODIFY — multi-user CheckFn
├── outlook_agent.go            # MODIFY — multi-user CheckFn + 10 个新 action + attachment
└── builder.go                  # MODIFY — addWithApproval 接收 whitelist

backend/internal/services/
├── agent_skill_whitelist.go    # NEW — service
├── agent_skill_whitelist_test.go
└── encrypted_settings.go       # NEW — GetSecretField / SetSecretField

backend/internal/handlers/
├── agent_skill_whitelist.go    # NEW — 4 个 HTTP route
├── agent_skill_whitelist_test.go
└── oauth.go                    # MODIFY — loadConfig 改用 GetSecretField

backend/internal/agent/cors.go  # MODIFY — wildcard suffix matcher
backend/internal/agent/cors_test.go

backend/internal/agent/tools/oauth/outlook_oauth.go  # MODIFY — DB lock 替换 mutex
backend/internal/agent/tools/oauth/outlook_oauth_test.go

backend/cmd/server/main.go     # MODIFY — wire whitelist svc + boot migration
```

### TDD discipline

每 task 一 commit。

---

## Task 0: Decision artefacts + go.mod check

**Files:**
- `.gpowers/decisions/2026-05-28-v3c-multi-user-oauth-enabled.md` (NEW)
- `.gpowers/decisions/2026-05-28-v3c-attachment-inline-text.md` (NEW)
- `.gpowers/decisions/2026-05-28-v3c-whitelist-coexists-with-global-toggle.md` (NEW)
- `.gpowers/decisions/2026-05-28-v3c-db-advisory-lock-strategy.md` (NEW)

**Tests:** none.

### Steps

- [ ] 4 个 decision artefact:

  **multi-user-oauth-enabled.md**:
  ```markdown
  # v3-C — Multi-User OAuth Enabled
  **Date**: 2026-05-28 **Status**: Adopted
  PR-AR-7 在 3 个 OAuth skill 的 CheckFn 里硬拒绝 multi-user 模式。底座(TokenStore.UserID uniqueIndex)已经支持 per-user。本 PR 移除短路。client_id/secret 仍是全局共享(admin 配一份),token 多行 per user。
  ```

  **attachment-inline-text.md**:
  ```markdown
  # v3-C — Attachment Inline-Text Strategy
  **Date**: 2026-05-28 **Status**: Adopted
  Gmail/Outlook v1 attachment 走"解析为文本 + inline 进 mail body"的简化路径,**不发送二进制附件**。理由:Apps Script 与 Graph API 的二进制 attachment schema 都很复杂;Node 端也是这么做的;90% 用例是让 LLM 看到附件内容。
  ```

  **whitelist-coexists-with-global-toggle.md**:
  ```markdown
  # v3-C — AgentSkillWhitelist 与全局 auto-approve 共存
  **Date**: 2026-05-28 **Status**: Adopted
  PR-AR-5 的全局 `agent_tool_auto_approve` SystemSetting 不移除。两者 OR:任一启用即放行。Whitelist 是更细粒度的能力,未来 deprecation 后会移除全局开关 — 但本 PR 不动它。
  ```

  **db-advisory-lock-strategy.md**:
  ```markdown
  # v3-C — OAuth Refresh DB Advisory Lock
  **Date**: 2026-05-28 **Status**: Adopted
  PR-AR-7 用 sync.Mutex 串行化 ValidAccessToken,单进程有效。v3-C 替换为 GORM clause.Locking{Strength:"UPDATE"} + Transaction。在 postgres/mysql 上是真 row lock;sqlite single-writer 模式天然串行。保留 sync.Mutex 作为 fast path(无害)。
  ```

- [ ] `go build ./...` + `go vet ./...` 干净。

### Acceptance

- 4 个 decision artefact 落地
- 无代码变更

### Commit

`chore(v3c): four decision artefacts for OAuth polish pack`

---

## Task 1: Multi-user OAuth gate(3 行 + 测试)

**Files:**
- `backend/internal/agent/tools/gmail_agent.go` (MODIFY)
- `backend/internal/agent/tools/gcal_agent.go` (MODIFY)
- `backend/internal/agent/tools/outlook_agent.go` (MODIFY)
- 各自的 `_test.go` (MODIFY)

**Tests:**
- `TestGmailAgent_MultiUserMode_NowEnabled` (之前是 false,现在 true)
- `TestGCalAgent_MultiUserMode_NowEnabled`
- `TestOutlookAgent_MultiUserMode_RequiresPerUserToken` (multi-user 模式下,缺 token 仍然 false)
- `TestGmailAgent_NoUser_StillFalse` (`tc.User == nil` 必须 false,防 nil deref)

### Steps

- [ ] gmail_agent.go `CheckFn` 改造:
  ```go
  // 之前
  if deps.Cfg.MultiUserMode { return false }

  // 之后
  // (该行删除)
  if tc.User == nil { return false }
  ```

- [ ] gcal_agent.go 同款。

- [ ] outlook_agent.go(原 CheckFn 已经检查 token,只需删 multi-user 短路):
  ```go
  CheckFn: func() bool {
      // 删除:if deps.Cfg.MultiUserMode { return false }
      if deps.OutlookOAuth == nil || deps.OutlookStore == nil { return false }
      cfg, ok := parseOutlookSkillConfig(tc.Settings[outlookConfigKey])
      if !ok { return false }
      _ = cfg
      if tc.User == nil { return false }
      _, err := deps.OutlookStore.Get(tc.Ctx, tc.User.ID)
      return err == nil
  },
  ```

- [ ] 4 个测试:
  ```go
  func TestOutlookAgent_MultiUserMode_RequiresPerUserToken(t *testing.T) {
      env := newAgentTestEnv(t)
      cfg := &config.Config{MultiUserMode: true}
      // 不预先 issue token
      tc := &tools.ToolContext{Ctx: ctx, User: env.User, Settings: validOutlookSettings(), Cfg: cfg}
      deps := tools.BuilderDeps{Cfg: cfg, OutlookOAuth: env.OutlookOAuth, OutlookStore: env.TokenStore}
      e := tools.NewOutlookAgentSkill(tc, deps)
      require.False(t, e.CheckFn())  // 缺 token

      // 现在 issue token
      env.IssueOutlookToken(t, env.User.ID)
      e2 := tools.NewOutlookAgentSkill(tc, deps)
      require.True(t, e2.CheckFn())  // 有 token,multi-user 也能用
  }
  ```

- [ ] Full suite green。

### Acceptance

- 4 个测试通过
- `MultiUserMode=true` 不再隐藏 3 个 skill
- `tc.User == nil` 仍然 false(零回归)

### Commit

`feat(agent/tools): enable OAuth skills in multi-user mode`

---

## Task 2: Attachment 解析 helper + gmail/outlook 集成

**Files:**
- `backend/internal/agent/tools/oauth/attachment.go` (NEW)
- `backend/internal/agent/tools/oauth/attachment_test.go` (NEW)
- `backend/internal/collector/` (MAYBE MODIFY — 如果 `ParseInMemory` 不存在则加)
- `backend/internal/agent/tools/gmail_agent.go` (MODIFY — 处理 attachments)
- `backend/internal/agent/tools/outlook_agent.go` (MODIFY — 处理 attachments)

**Tests:**
- `TestParseAttachments_HappyPath_AppendsParsedText`
- `TestParseAttachments_InvalidBase64_ReturnsError`
- `TestParseAttachments_ExceedsSizeCap_ReturnsError` (>10 MiB)
- `TestParseAttachments_CollectorError_ReturnsError`
- `TestParseAttachments_NoAttachments_ReturnsEmpty`
- `TestGmailAgent_SendEmail_WithAttachment_InlinesText`
- `TestOutlookAgent_SendEmail_WithAttachment_InlinesText`

### Steps

- [ ] 先验证 `collector.ParseInMemory` 是否存在:
  ```bash
  grep -nE "ParseInMemory|ParseDocument|ParseUpload" /Users/ranwei/workspace/go_work/go-hermind/backend/internal/collector/*.go
  ```
  若不存在,在 `internal/collector/` 加一个签名为 `func (c *Client) ParseInMemory(ctx, filename string, data []byte) (string, error)` 的方法(可以 wrap 已有的 ParseUpload — 写入临时文件 → Parse → 删除)。

- [ ] 实装 `attachment.go`:
  ```go
  package oauth

  import (
      "context"
      "encoding/base64"
      "fmt"
      "strings"

      "github.com/odysseythink/hermind/backend/internal/collector"
  )

  const MaxAttachmentBytes = 10 << 20  // 10 MiB

  type Attachment struct {
      Filename   string `json:"filename"`
      DataBase64 string `json:"data_base64"`
  }

  // ParseAttachments decodes + parses each attachment via collector,
  // returns concatenated "--- Attached file: X ---\n<text>" blocks.
  // Empty input returns empty string with nil error.
  func ParseAttachments(ctx context.Context, coll *collector.Client, atts []Attachment) (string, error) {
      if len(atts) == 0 { return "", nil }
      var out strings.Builder
      for _, a := range atts {
          if a.Filename == "" { return "", fmt.Errorf("attachment missing filename") }
          data, err := base64.StdEncoding.DecodeString(a.DataBase64)
          if err != nil { return "", fmt.Errorf("decode %s: %w", a.Filename, err) }
          if len(data) > MaxAttachmentBytes {
              return "", fmt.Errorf("attachment %s exceeds %d bytes", a.Filename, MaxAttachmentBytes)
          }
          parsed, err := coll.ParseInMemory(ctx, a.Filename, data)
          if err != nil { return "", fmt.Errorf("parse %s: %w", a.Filename, err) }
          fmt.Fprintf(&out, "\n\n--- Attached file: %s ---\n%s\n", a.Filename, parsed)
      }
      return out.String(), nil
  }
  ```

- [ ] Gmail/Outlook agent Handler 在 `send_email` / `create_draft` 等动作前调:
  ```go
  // gmail_agent.go (send_email branch 示意)
  rawAtts, _ := args["attachments"].([]any)
  var atts []oauth.Attachment
  for _, item := range rawAtts {
      blob, _ := json.Marshal(item)
      var a oauth.Attachment
      _ = json.Unmarshal(blob, &a)
      atts = append(atts, a)
  }
  attText, err := oauth.ParseAttachments(ctx, deps.Collector, atts)
  if err != nil { return tool.Error("attachment parse: " + err.Error()), nil }
  if attText != "" {
      // 把 attText 拼进 body
      body, _ := args["body"].(string)
      args["body"] = body + attText
  }
  ```

- [ ] BuilderDeps 加 `Collector *collector.Client`(nilable;nil 时 attachment 走 skip + warning)。

- [ ] 5 个 helper test + 2 个 agent 集成 test。

### Acceptance

- 7 个测试通过
- 10 MiB 上限严格执行
- Base64 错误 / 文件名缺失 / collector 错误,各自返回明确 error
- 无 attachments 时不影响原行为

### Commit

`feat(agent/tools/oauth): attachment parser + gmail/outlook integration`

---

## Task 3: Outlook 10 个补齐 action

**Files:**
- `backend/internal/agent/tools/outlook_agent.go` (MODIFY — 加 10 个 case)
- `backend/internal/agent/tools/outlook_agent_test.go` (MODIFY — 表驱动测试)

**Tests:**
- 1 个表驱动 `TestOutlookAgent_AllActions_RouteCorrectly`,跑 10 个 case,每 case 验证 Graph URL + method 正确

### Steps

- [ ] 补 `graphPATCH` / `graphDELETE` helper(PR-AR-7 已有 `graphGET` / `graphPOST`):
  ```go
  func graphPATCH(ctx context.Context, tok, path string, body map[string]any) (string, error) {
      payload, _ := json.Marshal(body)
      req, _ := http.NewRequestWithContext(ctx, "PATCH", graphBase()+path, bytes.NewReader(payload))
      req.Header.Set("Authorization", "Bearer "+tok)
      req.Header.Set("Content-Type", "application/json")
      return doGraph(req)
  }

  func graphDELETE(ctx context.Context, tok, path string) (string, error) {
      req, _ := http.NewRequestWithContext(ctx, "DELETE", graphBase()+path, nil)
      req.Header.Set("Authorization", "Bearer "+tok)
      return doGraph(req)
  }
  ```

- [ ] 在 switch 加 10 个 case:
  ```go
  case "get_inbox":
      limit := intArg(args, "limit", 25)
      return graphGET(ctx, tok, fmt.Sprintf("/me/mailFolders/inbox/messages?$top=%d", limit))
  case "list_drafts":
      limit := intArg(args, "limit", 25)
      return graphGET(ctx, tok, fmt.Sprintf("/me/mailFolders/drafts/messages?$top=%d", limit))
  case "get_draft":
      id, _ := args["draft_id"].(string)
      if id == "" { return tool.Error("draft_id required"), nil }
      return graphGET(ctx, tok, "/me/messages/"+id)
  case "update_draft":
      id, _ := args["draft_id"].(string)
      if id == "" { return tool.Error("draft_id required"), nil }
      return graphPATCH(ctx, tok, "/me/messages/"+id, buildOutlookMessage(args))
  case "delete_draft":
      id, _ := args["draft_id"].(string)
      if id == "" { return tool.Error("draft_id required"), nil }
      return graphDELETE(ctx, tok, "/me/messages/"+id)
  case "send_draft":
      id, _ := args["draft_id"].(string)
      if id == "" { return tool.Error("draft_id required"), nil }
      return graphPOST(ctx, tok, "/me/messages/"+id+"/send", nil)
  case "create_draft_reply":
      id, _ := args["message_id"].(string)
      if id == "" { return tool.Error("message_id required"), nil }
      return graphPOST(ctx, tok, "/me/messages/"+id+"/createReply", map[string]any{
          "comment": args["body"],
      })
  case "reply_to_message":
      id, _ := args["message_id"].(string)
      if id == "" { return tool.Error("message_id required"), nil }
      return graphPOST(ctx, tok, "/me/messages/"+id+"/reply", map[string]any{
          "comment": args["body"],
      })
  case "mark_read":
      id, _ := args["message_id"].(string)
      if id == "" { return tool.Error("message_id required"), nil }
      return graphPATCH(ctx, tok, "/me/messages/"+id, map[string]any{"isRead": true})
  case "mark_unread":
      id, _ := args["message_id"].(string)
      if id == "" { return tool.Error("message_id required"), nil }
      return graphPATCH(ctx, tok, "/me/messages/"+id, map[string]any{"isRead": false})
  ```

- [ ] 更新 `outlookDestructiveActions` map,把 6 个写动作加进去:
  ```go
  var outlookDestructiveActions = map[string]bool{
      "create_draft": true, "send_email": true,  // PR-AR-7 已有
      "update_draft": true, "delete_draft": true, "send_draft": true,
      "create_draft_reply": true, "reply_to_message": true,
      "mark_read": true, "mark_unread": true,  // 改状态也算 destructive
  }
  ```

- [ ] `intArg` helper(`outlook_agent.go` 或就近):
  ```go
  func intArg(args map[string]any, key string, fallback int) int {
      if v, ok := args[key]; ok {
          if n, ok := v.(float64); ok { return int(n) }
      }
      return fallback
  }
  ```

- [ ] 表驱动测试,httptest 桩 graph.microsoft.com 验证每 case 的 URL + method:
  ```go
  func TestOutlookAgent_AllActions_RouteCorrectly(t *testing.T) {
      cases := []struct {
          action       string
          args         string  // JSON
          expectMethod string
          expectPath   string  // contains
      }{
          {"get_inbox", `{"limit":50}`, "GET", "/me/mailFolders/inbox/messages"},
          {"list_drafts", `{"limit":10}`, "GET", "/me/mailFolders/drafts/messages"},
          {"get_draft", `{"draft_id":"abc"}`, "GET", "/me/messages/abc"},
          {"update_draft", `{"draft_id":"abc","subject":"X"}`, "PATCH", "/me/messages/abc"},
          {"delete_draft", `{"draft_id":"abc"}`, "DELETE", "/me/messages/abc"},
          {"send_draft", `{"draft_id":"abc"}`, "POST", "/me/messages/abc/send"},
          {"create_draft_reply", `{"message_id":"abc","body":"hi"}`, "POST", "/me/messages/abc/createReply"},
          {"reply_to_message", `{"message_id":"abc","body":"hi"}`, "POST", "/me/messages/abc/reply"},
          {"mark_read", `{"message_id":"abc"}`, "PATCH", "/me/messages/abc"},
          {"mark_unread", `{"message_id":"abc"}`, "PATCH", "/me/messages/abc"},
      }
      // ... httptest server records last req, run each case, assert.
  }
  ```

### Acceptance

- 15 个 outlook action 全部就绪(5 PR-AR-7 + 10 v3-C)
- 表驱动测试覆盖路径 + 方法
- 9 个 destructive action 都走 approval gate

### Commit

`feat(agent/tools): outlook-agent — 10 additional Graph API actions`

---

## Task 4: AgentSkillWhitelist 服务 + HTTP + Builder hook

**Files:**
- `backend/internal/services/agent_skill_whitelist.go` (NEW)
- `backend/internal/services/agent_skill_whitelist_test.go` (NEW)
- `backend/internal/handlers/agent_skill_whitelist.go` (NEW)
- `backend/internal/handlers/agent_skill_whitelist_test.go` (NEW)
- `backend/internal/agent/tools/builder.go` (MODIFY — addWithApproval 接 whitelist)
- `backend/internal/agent/tools/builder_test.go` (MODIFY)
- `backend/cmd/server/main.go` (MODIFY — wire whitelistSvc)

**Tests:**
- `TestWhitelistSvc_Add/Remove/Get_RoundTrip`
- `TestWhitelistSvc_LabelDifferent_PerUserVsSingleUser`
- `TestWhitelistSvc_IsWhitelisted_True`
- `TestWhitelistSvc_IsWhitelisted_False`
- `TestWhitelistSvc_ClearSingleUser`
- `TestWhitelistHandler_GET_ReturnsCallerList`
- `TestWhitelistHandler_POST_AddsSkill`
- `TestWhitelistHandler_DELETE_RemovesSkill`
- `TestWhitelistHandler_Admin_GetForUser_NonAdmin_Forbidden`
- `TestBuilder_WhitelistedSkill_BypassesApproval`
- `TestBuilder_NonWhitelisted_StillRequiresApproval`

### Steps

- [ ] 实装 service:
  ```go
  package services

  import (
      "context"
      "encoding/json"
      "fmt"
  )

  type AgentSkillWhitelistService struct {
      sysSvc *SystemService
  }

  func NewAgentSkillWhitelistService(sysSvc *SystemService) *AgentSkillWhitelistService {
      return &AgentSkillWhitelistService{sysSvc: sysSvc}
  }

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
      if skill == "" { return fmt.Errorf("skill name required") }
      list, _ := s.Get(ctx, userID)
      for _, x := range list { if x == skill { return nil } }  // idempotent
      list = append(list, skill)
      raw, _ := json.Marshal(list)
      return s.sysSvc.SetSetting(ctx, s.label(userID), string(raw))
  }

  func (s *AgentSkillWhitelistService) Remove(ctx context.Context, userID *int, skill string) error {
      list, _ := s.Get(ctx, userID)
      out := make([]string, 0, len(list))
      for _, x := range list { if x != skill { out = append(out, x) } }
      raw, _ := json.Marshal(out)
      return s.sysSvc.SetSetting(ctx, s.label(userID), string(raw))
  }

  func (s *AgentSkillWhitelistService) IsWhitelisted(ctx context.Context, userID *int, skill string) bool {
      list, _ := s.Get(ctx, userID)
      for _, x := range list { if x == skill { return true } }
      return false
  }

  func (s *AgentSkillWhitelistService) ClearSingleUser(ctx context.Context) error {
      return s.sysSvc.SetSetting(ctx, "whitelisted_agent_skills", "[]")
  }
  ```

- [ ] 实装 handler:
  ```go
  func RegisterAgentSkillWhitelistRoutes(r *gin.RouterGroup, h *AgentSkillWhitelistHandler, authSvc *services.AuthService) {
      g := r.Group("/agent-skill-whitelist")
      g.Use(middleware.ValidatedRequest(authSvc))
      g.GET("", h.Get)                    // caller's list
      g.POST("", h.Add)                   // body: {skill: string}
      g.DELETE("/:skill", h.Remove)       // by skill name

      r.GET("/admin/agent-skill-whitelist/:userId",
          middleware.ValidatedRequest(authSvc),
          middleware.FlexUserRoleValid([]string{"admin"}),
          h.AdminGetForUser,
      )
  }
  ```

- [ ] Builder hook(在 `addWithApproval` 内):
  ```go
  // builder.go
  func (b *Builder) addWithApproval(reg *tool.Registry, seen map[string]string, e *tool.Entry, source string, requiresApproval bool, globalAutoApprove bool, whitelist []string) {
      if requiresApproval && !globalAutoApprove && !containsStr(whitelist, e.Name) && b.deps.Approval != nil {
          // wrap with approval as before
      }
      // ... rest unchanged
  }
  ```

  Build() 一次性预拉 whitelist:
  ```go
  func (b *Builder) Build(ctx, ws, user, emit, settings) (...) {
      var userID *int
      if user != nil { userID = &user.ID }
      var whitelist []string
      if b.deps.WhitelistSvc != nil {
          whitelist, _ = b.deps.WhitelistSvc.Get(ctx, userID)
      }
      // ... 后续 addWithApproval 调用都传 whitelist
  }
  ```

- [ ] BuilderDeps 加 `WhitelistSvc *services.AgentSkillWhitelistService`(nilable)。

- [ ] 11 个测试,涵盖 service CRUD + handler 4 routes + Builder bypass。

- [ ] main.go wire:
  ```go
  whitelistSvc := services.NewAgentSkillWhitelistService(sysSvc)
  agentRuntime := agent.NewRuntime(agent.Deps{
      /* ... */
      WhitelistSvc: whitelistSvc,
  })
  whitelistHandler := handlers.NewAgentSkillWhitelistHandler(whitelistSvc)
  handlers.RegisterAgentSkillWhitelistRoutes(api, whitelistHandler, authSvc)
  ```

### Acceptance

- 11 个测试通过
- Whitelisted skill 不调 Approval(verified via mock)
- 全局 auto-approve + whitelist 共存(OR 关系)
- Admin route 受 RBAC 保护

### Commit

`feat(services): AgentSkillWhitelist + HTTP routes + Builder bypass hook`

---

## Task 5: clientSecret `enc:` 加密 + 启动 migration

**Files:**
- `backend/internal/services/encrypted_settings.go` (NEW)
- `backend/internal/services/encrypted_settings_test.go` (NEW)
- `backend/internal/services/system_service.go` (MODIFY — 加方法)
- `backend/internal/handlers/oauth.go` (MODIFY — loadConfig 改用 GetSecretField)
- `backend/cmd/server/main.go` (MODIFY — 启动 migration goroutine)

**Tests:**
- `TestGetSecretField_PlainValue_ReturnsAsIs`
- `TestGetSecretField_EncPrefix_Decrypts`
- `TestSetSecretField_EncryptsBeforeSave`
- `TestSetSecretField_RoundTrip_ViaGet`
- `TestSetSecretField_OtherFieldsPreserved` (改 clientSecret 不动 clientId)
- `TestMigration_EncryptsExistingPlaintext`
- `TestMigration_Idempotent_DoesNotDoubleEncrypt`
- `TestOutlookCallback_LoadsEncryptedSecret`

### Steps

- [ ] 实装 `encrypted_settings.go`:
  ```go
  package services

  import (
      "context"
      "encoding/json"
      "fmt"
      "strings"

      "github.com/odysseythink/hermind/backend/pkg/utils"
  )

  const EncryptedPrefix = "enc:"

  // GetSecretField reads SystemSetting[settingKey] as JSON, extracts string
  // field jsonField, and decrypts it if it carries the "enc:" prefix.
  func (s *SystemService) GetSecretField(ctx context.Context, settingKey, jsonField string, enc *utils.EncryptionManager) (string, error) {
      raw, err := s.GetSetting(ctx, settingKey)
      if err != nil { return "", err }
      if raw == "" { return "", nil }
      var obj map[string]any
      if err := json.Unmarshal([]byte(raw), &obj); err != nil {
          return "", fmt.Errorf("setting %q is not JSON: %w", settingKey, err)
      }
      v, _ := obj[jsonField].(string)
      if strings.HasPrefix(v, EncryptedPrefix) {
          plain, err := enc.Decrypt(strings.TrimPrefix(v, EncryptedPrefix))
          if err != nil { return "", fmt.Errorf("decrypt %s.%s: %w", settingKey, jsonField, err) }
          return plain, nil
      }
      return v, nil
  }

  // SetSecretField writes plaintext into SystemSetting[settingKey] JSON's
  // jsonField, encrypted with the "enc:" prefix. Preserves other fields.
  func (s *SystemService) SetSecretField(ctx context.Context, settingKey, jsonField, plaintext string, enc *utils.EncryptionManager) error {
      raw, _ := s.GetSetting(ctx, settingKey)
      obj := map[string]any{}
      if raw != "" { _ = json.Unmarshal([]byte(raw), &obj) }
      ciphertext, err := enc.Encrypt(plaintext)
      if err != nil { return err }
      obj[jsonField] = EncryptedPrefix + ciphertext
      data, _ := json.Marshal(obj)
      return s.SetSetting(ctx, settingKey, string(data))
  }
  ```

- [ ] OAuthHandler.loadConfig 改造:
  ```go
  func (h *OAuthHandler) loadConfig(ctx context.Context) (outlookConfig, error) {
      raw, err := h.sysSvc.GetSetting(ctx, "outlook_agent_config")
      if err != nil || raw == "" { return outlookConfig{}, errors.New("outlook_agent_config not set") }
      var c outlookConfig
      _ = json.Unmarshal([]byte(raw), &c)
      // clientSecret 可能加密
      secret, err := h.sysSvc.GetSecretField(ctx, "outlook_agent_config", "clientSecret", h.enc)
      if err != nil { return c, fmt.Errorf("decrypt clientSecret: %w", err) }
      c.ClientSecret = secret
      if c.ClientID == "" || c.ClientSecret == "" {
          return c, errors.New("outlook_agent_config incomplete")
      }
      return c, nil
  }
  ```

  OAuthHandler 构造加 `enc *utils.EncryptionManager` 字段。

- [ ] 启动 migration(在 main.go 启动后,goroutine 跑一次):
  ```go
  go func() {
      ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
      defer cancel()
      keys := []struct{ setting, field string }{
          {"outlook_agent_config", "clientSecret"},
          {"gmail_agent_config", "apiKey"},
          {"google_calendar_agent_config", "apiKey"},
      }
      for _, k := range keys {
          raw, err := sysSvc.GetSetting(ctx, k.setting)
          if err != nil || raw == "" { continue }
          var obj map[string]any
          if err := json.Unmarshal([]byte(raw), &obj); err != nil { continue }
          v, _ := obj[k.field].(string)
          if v == "" || strings.HasPrefix(v, services.EncryptedPrefix) { continue }  // 已加密
          if err := sysSvc.SetSecretField(ctx, k.setting, k.field, v, enc); err != nil {
              mlog.Warning("migrate ", k.setting, ".", k.field, ": ", err)
              continue
          }
          mlog.Info("migrated ", k.setting, ".", k.field, " to encrypted form")
      }
  }()
  ```

- [ ] 8 个测试。

### Acceptance

- 8 个测试通过
- 现有明文 SystemSetting 在启动后被升级为加密
- 第二次启动 idempotent(已加密的不重复加密)
- OAuth callback 仍然能读取加密的 clientSecret

### Commit

`feat(services): clientSecret encryption helper + boot migration`

---

## Task 6: DB advisory lock 替换 sync.Mutex

**Files:**
- `backend/internal/agent/tools/oauth/outlook_oauth.go` (MODIFY — ValidAccessToken)
- `backend/internal/agent/tools/oauth/outlook_oauth_test.go` (MODIFY — 增加并发测试)

**Tests:**
- `TestValidAccessToken_NoConcurrency_StillWorks` (回归)
- `TestValidAccessToken_ConcurrentExpired_RefreshOnceViaDBLock` (10 goroutine,期望恰好 1 次 refresh)
- `TestValidAccessToken_TxRollbackOnRefreshError` (refresh 失败,DB row 不变)
- `TestValidAccessToken_SqliteSingleWriterFallback` (sqlite 路径不报错,功能正常)

### Steps

- [ ] 改造 `ValidAccessToken`:
  ```go
  func (o *OutlookOAuth) ValidAccessToken(ctx context.Context, userID int, clientID, clientSecret string) (string, error) {
      // 保留 sync.Mutex 作为单进程 fast path
      o.refreshMu.Lock()
      defer o.refreshMu.Unlock()

      var accessToken string
      err := o.store.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
          var row models.OutlookOAuthToken
          // SELECT ... FOR UPDATE on postgres/mysql; sqlite degrades to immediate tx
          err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
              Where("user_id = ?", userID).First(&row).Error
          if errors.Is(err, gorm.ErrRecordNotFound) { return ErrTokenNotFound }
          if err != nil { return err }

          // 在 tx 内重新检查 expiry,可能另一个进程刚刷新过
          if time.Now().Before(row.ExpiresAt) {
              pt, err := o.store.enc.Decrypt(row.EncryptedAccessToken)
              if err != nil { return err }
              accessToken = pt
              return nil
          }

          // Refresh
          oldRT, err := o.store.enc.Decrypt(row.EncryptedRefreshToken)
          if err != nil { return err }
          newTS, err := o.refresh(ctx, oldRT, clientID, clientSecret, row.Tenant)
          if err != nil { return err }
          if newTS.RefreshToken == "" { newTS.RefreshToken = oldRT }
          if newTS.Tenant == "" { newTS.Tenant = row.Tenant }

          atEnc, err := o.store.enc.Encrypt(newTS.AccessToken)
          if err != nil { return err }
          rtEnc, err := o.store.enc.Encrypt(newTS.RefreshToken)
          if err != nil { return err }
          if err := tx.Model(&row).Updates(map[string]any{
              "encrypted_access_token":  atEnc,
              "encrypted_refresh_token": rtEnc,
              "expires_at":              newTS.ExpiresAt,
              "tenant":                  newTS.Tenant,
          }).Error; err != nil { return err }

          accessToken = newTS.AccessToken
          return nil
      })
      return accessToken, err
  }
  ```

- [ ] 4 个测试。`TestValidAccessToken_ConcurrentExpired_RefreshOnceViaDBLock` 关键:
  ```go
  func TestValidAccessToken_ConcurrentExpired_RefreshOnceViaDBLock(t *testing.T) {
      env := newOAuthTestEnv(t)
      var refreshCount atomic.Int32
      srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
          refreshCount.Add(1)
          time.Sleep(50 * time.Millisecond)  // 模拟网络延迟,放大竞态
          json.NewEncoder(w).Encode(map[string]any{
              "access_token":  fmt.Sprintf("new-token-%d", refreshCount.Load()),
              "refresh_token": "new-refresh",
              "expires_in":    3600,
          })
      }))
      defer srv.Close()
      oauth.SetTestMicrosoftBase(srv.URL)

      // Pre-seed expired token
      env.SeedExpiredToken(t, env.User.ID)

      var wg sync.WaitGroup
      for i := 0; i < 10; i++ {
          wg.Add(1)
          go func() {
              defer wg.Done()
              tok, err := env.OutlookOAuth.ValidAccessToken(context.Background(), env.User.ID, "cid", "secret")
              require.NoError(t, err)
              require.NotEmpty(t, tok)
          }()
      }
      wg.Wait()
      require.Equal(t, int32(1), refreshCount.Load(), "must refresh exactly once")
  }
  ```

### Acceptance

- 4 个测试通过
- 并发 10 路 → 仅 1 次 refresh
- 单进程性能没退化(mutex 仍在)
- sqlite/mysql/postgres 全部 work

### Commit

`feat(agent/tools/oauth): replace sync.Mutex with DB row-level lock for token refresh`

---

## Task 7: CORS 通配 + 最终验收

**Files:**
- `backend/internal/agent/cors.go` (MODIFY)
- `backend/internal/agent/cors_test.go` (MODIFY)

**Tests:**
- `TestCheckOrigin_WildcardSuffix_MatchesSubdomain`
- `TestCheckOrigin_WildcardSuffix_DoesNotMatchPrefixInjection` (`evil-example.com` 不应匹配 `*.example.com`)
- `TestCheckOrigin_WildcardSuffix_DoesNotMatchBareDomain` (`*.example.com` 不匹配 `example.com`,要严格 sub)
- `TestCheckOrigin_MixedExactAndWildcard` (CSV `"https://a.com,*.b.com"`)
- `TestCheckOrigin_AllExistingBehavior_Regressed` (PR-AR-5 已有的 4 个 case 不回归)

### Steps

- [ ] 重构 `cors.go`(用 matcher struct):
  ```go
  type originMatcher struct {
      exact  map[string]bool
      suffix []string  // "example.com" matches "a.example.com" / "x.y.example.com"
      anyOrigin bool
  }

  func parseAllowedOrigins(raw string) *originMatcher {
      raw = strings.TrimSpace(raw)
      if raw == "*" { return &originMatcher{anyOrigin: true} }
      m := &originMatcher{exact: map[string]bool{}}
      for _, o := range strings.Split(raw, ",") {
          o = strings.TrimSpace(o)
          if o == "" { continue }
          if strings.HasPrefix(o, "*.") {
              m.suffix = append(m.suffix, strings.TrimPrefix(o, "*."))
          } else {
              m.exact[o] = true
          }
      }
      return m
  }

  func (m *originMatcher) match(origin, requestHost string) bool {
      if origin == "" { return true }  // non-browser
      if m.anyOrigin { return true }
      if m.exact[origin] { return true }
      u, err := url.Parse(origin)
      if err != nil { return false }
      for _, suf := range m.suffix {
          // strict: host must END with "." + suf (no prefix injection)
          if strings.HasSuffix(u.Host, "."+suf) { return true }
      }
      // 空配置 → same-host fallback
      if len(m.exact) == 0 && len(m.suffix) == 0 && !m.anyOrigin {
          return u.Host == requestHost
      }
      return false
  }

  func buildCheckOrigin(cfg *config.Config) func(*http.Request) bool {
      raw := strings.TrimSpace(cfg.AgentAllowedOrigins)
      if raw == "*" {
          mlog.Warning("agent: AGENT_ALLOWED_ORIGINS=* — allowing any origin. Tighten in production.")
      }
      m := parseAllowedOrigins(raw)
      return func(r *http.Request) bool {
          return m.match(r.Header.Get("Origin"), r.Host)
      }
  }
  ```

- [ ] 5 个测试,**特别覆盖前缀注入攻击**:
  ```go
  func TestCheckOrigin_WildcardSuffix_DoesNotMatchPrefixInjection(t *testing.T) {
      cfg := &config.Config{AgentAllowedOrigins: "*.example.com"}
      check := agent.BuildCheckOriginForTesting(cfg)
      req := httptest.NewRequest("GET", "/", nil)
      req.Header.Set("Origin", "https://evil-example.com")  // 不带 . 前缀
      require.False(t, check(req))
  }
  ```

- [ ] 最终验收: `go test ./... -race` 全绿。

### Acceptance

- 5 个测试通过
- 前缀注入 `evil-example.com` 被严格拒绝(必须有 `.` 分隔)
- PR-AR-5 已有的 CORS 行为零回归
- Full suite green

### Commit

`feat(agent): WS CORS — wildcard suffix matching with strict prefix-injection defense`

---

## Post-PR checklist

- [ ] `go build ./...` 干净
- [ ] `go vet ./...` 干净
- [ ] `go test ./... -race` 100% 绿
- [ ] `gofmt -l . | wc -l` 返回 0
- [ ] 4 个 decision artefact 落地
- [ ] 手测:
  - [ ] `MULTI_USER_MODE=true` 启动,两个 user 各自 connect outlook,token 互不污染
  - [ ] gmail send_email 带一个 PDF attachment,LLM 在生成时能看到附件内容
  - [ ] Outlook 10 个新 action 各跑一次,确认 PATCH/DELETE 都走 approval gate
  - [ ] AgentSkillWhitelist 把 `mcp-test-tool` 加进白名单,LLM 再调时不弹 approval
  - [ ] 启动一次后,SystemSetting 表里的 `clientSecret` 已变为 `enc:...` 前缀
  - [ ] WS CORS 配 `*.example.com`,从 `a.example.com` 拨号成功,从 `evil-example.com` 失败
- [ ] No new TODOs without follow-up reference

## Risk notes

| Risk | Mitigation |
|---|---|
| Boot migration 在 EncryptionManager 不可用时失败 | log warning + 跳过;不阻塞 boot |
| collector.ParseInMemory 不存在 → Task 2 阻塞 | Task 2 步骤 1 显式 verify;若缺则在 collector 包内加 |
| Multi-user 启用后,前端 UI 没改 → admin 看不到自己以外的 token | 前端 PR 独立;Go 端 status endpoint 已 per-user 工作,不阻塞 |
| Outlook 10 actions 改状态可能误触发邮件折叠 | mark_read/unread 是 PATCH `isRead` 字段,Microsoft 文档明确,无副作用 |
| Whitelist 列表过长 → JSON 反序列化慢 | 实测 <10K skill,JSON ~ms 级,无问题 |
| `enc:` 前缀字面巧合用户密钥 | 用户密钥 `"enc:xxx"` 会被误认为已加密 → 解密失败 → log error。概率极低;若发生用户重设即可 |
| DB lock 在 sqlite 上 noop | 文档明确;sqlite 本来就是 single-process |
| CORS 前缀注入测试覆盖的攻击场景外仍可能漏 | 严格只接受 `.+suffix` 后缀,no leniency |

## Estimate

| Task | Hours |
|---|---|
| 0. Decision artefacts | 0.5 |
| 1. Multi-user CheckFn | 1.0 |
| 2. Attachment helper + integration | 3.0 |
| 3. Outlook 10 actions | 3.0 |
| 4. AgentSkillWhitelist + Builder hook | 4.0 |
| 5. clientSecret `enc:` + migration | 2.0 |
| 6. DB advisory lock | 2.0 |
| 7. CORS wildcard | 0.5 |
| **Total** | **16.0** (design 估 15-18h,中段 ✓) |

—— end of plan
