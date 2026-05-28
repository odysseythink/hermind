# Agent Runtime PR-AR-7 — gmail / google-calendar / outlook Skills Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `gpowers:subagent-driven-development` (recommended) or `gpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Land three Node-parity agent skills — `gmail-agent`, `google-calendar-agent` (both Google Apps Script bridges, ~7h combined), and `outlook-agent` (real Microsoft OAuth 2.0, ~18h). All three are single-user-mode-only, wrapped by PR-AR-5's approval gate on destructive actions, and packaged as single-tool-with-action-enum (PR-AR-6 pattern). After PR-AR-7, the Go default-skill set matches Node's exactly modulo intentional v1 omissions documented in decision artefacts.

**Architecture:**
- A shared `internal/agent/tools/oauth/` sub-package holds: `BridgeClient` (Apps Script HTTP wire), `OutlookOAuth` (authorize URL + code exchange + auto-refresh), `TokenStore` (encrypted persistence), `state.go` (HMAC-signed state with CSRF + open-redirect + replay defenses).
- Three skills (`gmail_agent.go`, `gcal_agent.go`, `outlook_agent.go`) sit alongside PR-AR-6's skills, each a single `tool.Entry` with `action` enum dispatching into BridgeClient or OutlookOAuth+Graph API.
- A new `handlers/oauth.go` exposes 4 routes (`/api/oauth/outlook/{authorize,callback,disconnect,status}`); only `callback` is unauthenticated (state-self-authenticated).
- A new GORM model `OutlookOAuthToken` stores per-user refresh tokens encrypted with the existing `pkg/utils.EncryptionManager.Encrypt/Decrypt` (AES-GCM).
- Apps Script templates ship under `backend/assets/apps-script/{gmail,gcal}/` with deployment README; admins deploy independently from any Node-side Apps Script.

**Tech Stack:** Go 1.25.5; new deps: none for Apps Script bridge; **no new Go deps for Outlook OAuth either** — stdlib `net/http` + `crypto/hmac` + `crypto/sha256` cover everything. All pantheon + gorilla + cron deps already present.

**Source spec:** `.gpowers/designs/2026-05-27-oauth-agent-skills-design.md` (full design, 13 sections).

**Reference Node implementation:**
- `server/utils/agents/aibitat/plugins/gmail/{index.js, lib.js, account/, drafts/, search/, send/, threads/}` — GmailBridge + 12-action surface
- `server/utils/agents/aibitat/plugins/google-calendar/{index.js, lib.js, calendars/, events/}` — GoogleCalendarBridge + 8-action surface
- `server/utils/agents/aibitat/plugins/outlook/{index.js, lib.js, account/, drafts/, search/, send/}` — OutlookBridge with `MICROSOFT_AUTH_URL`, `SCOPES`, `#refreshAccessToken`, `#getValidAccessToken`

---

## Pre-task: Read this section once before starting

### What landed in PR-AR-1 to PR-AR-6 (use, don't re-implement)

- `internal/agent/tools/context.go` — `ToolContext{Workspace, User, Settings, LM, Approval, Cfg, ...}`. PR-AR-7 uses `tc.Settings["gmail_agent_config"]` / `tc.Settings["google_calendar_agent_config"]` / `tc.Settings["outlook_agent_config"]` directly; no new context fields.
- `internal/agent/tools/builder.go` — registration funnel `addWithApproval(reg, seen, e, source, requiresApproval, globalAutoApprove)`. PR-AR-7 calls with `requiresApproval=false` and routes each destructive action through `tc.Approval` inline (PR-AR-6 pattern).
- `internal/agent/tools/builder.go:BuilderDeps` — already has `Cfg *config.Config`. PR-AR-7 adds three fields: `Bridge *oauth.BridgeClient`, `OutlookOAuth *oauth.OutlookOAuth`, `OutlookStore *oauth.TokenStore`.
- `pkg/utils.EncryptionManager.Encrypt(plaintext string) (string, error)` / `.Decrypt(ciphertext string) (string, error)` — AES-GCM helper already used by `auth_service.go` for `cfg.AuthToken` encryption. PR-AR-7 reuses **identically** for both `outlook_agent_config.clientSecret` and the per-user refresh_token storage.
- `internal/services/system_service.go:GetSetting(ctx, key) (string, error)` / `SetSetting(ctx, key, value) error` — used for the three `*_agent_config` SystemSettings.
- `internal/config/config.go:MultiUserMode bool` — already present. PR-AR-7 adds `PublicBaseURL` and `OutlookAuthority` knobs.
- `internal/handlers/api_setup_test.go:apiTestEnv` pattern — used wherever HTTP-level e2e tests are needed (mainly Task 5).
- `internal/middleware/auth.go:ValidatedRequest(authSvc)` — used on `/authorize`, `/disconnect`, `/status` routes. Callback intentionally **does not** use this middleware (state is the auth boundary).

### Node behavior verified by reading source

```
server/utils/agents/aibitat/plugins/gmail/lib.js:339  →  POST script.google.com/macros/s/<id>/exec
server/utils/agents/aibitat/plugins/outlook/lib.js:468  →  SCOPES = "offline_access Mail.Read Mail.ReadWrite Mail.Send User.Read"
server/utils/agents/aibitat/plugins/outlook/lib.js:668  →  expiresAt = Date.now() + (expires_in - 60)*1000  (60s leeway)
server/utils/agents/aibitat/plugins/outlook/lib.js:746  →  refreshToken = data.refresh_token || config.refreshToken  (preserve old when MS doesn't rotate)
```

### New surface (this PR)

```
backend/internal/agent/tools/oauth/
├── doc.go                       # package comment
├── bridge_client.go             # Apps Script wire (gmail + gcal share)
├── bridge_client_test.go
├── outlook_oauth.go             # AuthorizeURL / ExchangeCode / ValidAccessToken (auto-refresh)
├── outlook_oauth_test.go
├── outlook_token_store.go       # encrypted CRUD over outlook_oauth_tokens table
├── outlook_token_store_test.go
├── state.go                     # HMAC-signed state (CSRF + open-redirect + replay)
└── state_test.go

backend/internal/agent/tools/
├── gmail_agent.go               # 12-action skill
├── gmail_agent_test.go
├── gcal_agent.go                # 8-action skill
├── gcal_agent_test.go
├── outlook_agent.go             # 5-action skill
├── outlook_agent_test.go
└── builder.go                   # MODIFY — register 3 skills + 3 BuilderDeps fields

backend/internal/handlers/
├── oauth.go                     # NEW — 4 routes
└── oauth_test.go

backend/internal/models/
└── outlook_oauth_token.go       # NEW — GORM model + AutoMigrate hook

backend/internal/models/db.go  # MODIFY — add OutlookOAuthToken to AutoMigrate list

backend/internal/config/config.go  # MODIFY — add PublicBaseURL, OutlookAuthority

backend/cmd/server/main.go     # MODIFY — wire BridgeClient + OutlookOAuth + TokenStore + handlers

backend/assets/apps-script/
├── gmail/{Code.gs,appsscript.json,README.md}
└── gcal/{Code.gs,appsscript.json,README.md}
```

### Methods to ship (PR-AR-7 scope)

| # | Owner | Signature | Notes |
|---|---|---|---|
| 1 | `oauth.BridgeClient` | `NewBridgeClient(httpTimeout time.Duration) *BridgeClient` | 30s default |
| 2 | `oauth.BridgeClient` | `Call(ctx, deploymentID, apiKey, action string, params map[string]any) (json.RawMessage, error)` | Wire format per design §4.1 |
| 3 | `oauth.OutlookOAuth` | `NewOutlookOAuth(enc, store, cfg, http *http.Client) *OutlookOAuth` | |
| 4 | `oauth.OutlookOAuth` | `AuthorizeURL(state, clientID, authority string) string` | |
| 5 | `oauth.OutlookOAuth` | `ExchangeCode(ctx, code, clientID, clientSecret, authority string) (*TokenSet, error)` | |
| 6 | `oauth.OutlookOAuth` | `ValidAccessToken(ctx, userID int, clientID, clientSecret string) (string, error)` | Auto-refresh under mutex; preserves old refresh_token when MS doesn't rotate |
| 7 | `oauth.TokenStore` | `NewTokenStore(db *gorm.DB, enc *utils.EncryptionManager) *TokenStore` | |
| 8 | `oauth.TokenStore` | `Get(ctx, userID int) (*TokenSet, error)` | returns `ErrTokenNotFound` if absent |
| 9 | `oauth.TokenStore` | `Save(ctx, userID int, ts *TokenSet) error` | upsert by `user_id` |
| 10 | `oauth.TokenStore` | `Delete(ctx, userID int) error` | idempotent |
| 11 | `oauth.EncodeState` | `(secret []byte, p StatePayload) string` | base64url(json) + "." + base64url(HMAC) |
| 12 | `oauth.DecodeState` | `(secret []byte, encoded, publicBaseURL string) (*StatePayload, error)` | Verifies HMAC + exp + ReturnTo prefix |
| 13 | `tools.NewGmailAgentSkill(tc, deps) *tool.Entry` | 12-action surface | |
| 14 | `tools.NewGCalAgentSkill(tc, deps) *tool.Entry` | 8-action surface | |
| 15 | `tools.NewOutlookAgentSkill(tc, deps) *tool.Entry` | 5-action surface | |
| 16 | `handlers.OAuthHandler.OutlookAuthorize` | `GET /api/oauth/outlook/authorize?return_to=` → 302 | |
| 17 | `handlers.OAuthHandler.OutlookCallback` | `GET /api/oauth/outlook/callback?code=&state=` → 302 or error HTML | |
| 18 | `handlers.OAuthHandler.OutlookDisconnect` | `POST` → delete token | |
| 19 | `handlers.OAuthHandler.OutlookStatus` | `GET` → `{connected: bool, expiresAt}` | |

### Action enums (per skill)

**gmail-agent** (12 actions): `search`, `read_thread`, `list_drafts`, `get_draft`, `mailbox_stats` (read-only — no approval) + `create_draft`, `update_draft`, `send_draft`, `send_email`, `reply_to_thread`, `delete_draft`, `move_to_trash` (destructive — approval required)

**google-calendar-agent** (8 actions): `list_calendars`, `get_calendar`, `get_event`, `get_events_for_day`, `get_events` (read-only) + `quick_add`, `create_event`, `update_event` (destructive)

**outlook-agent** (5 actions): `search`, `read_thread`, `read_message` (read-only) + `create_draft`, `send_email` (destructive)

> **Outlook action count delta vs Node**: Node exposes ~20 outlook actions; v1 ships only 5 (the agent core loop). Decision artefact `2026-05-27-outlook-action-subset.md` records: `reply_to_thread`, `update_draft`, `mark_read/unread`, `move_to_*` etc. are deferred. LLM can do `read_thread` + `send_email` to a captured thread-id+from to approximate reply.

### Approval gate granularity (action-level, PR-AR-6 pattern)

Each Handler does:

```go
if isDestructive[args.Action] && tc.Approval != nil {
    desc := fmt.Sprintf("%s.%s on %s", skillName, args.Action, args.PrimaryArg)
    if ok, reason := tc.Approval(ctx, skillName+":"+args.Action, args, desc); !ok {
        return tool.Error("rejected: " + reason), nil
    }
}
```

`isDestructive` is a per-skill map literal at top of file.

### CheckFn — three gates per skill

```go
// gmail / gcal share this shape:
CheckFn: func() bool {
    if deps.Cfg.MultiUserMode { return false }           // 1. single-user mode only
    raw := tc.Settings[gmailConfigKey]                   // 2. config row present
    cfg, ok := parseBridgeConfig(raw)
    return ok && cfg.DeploymentID != "" && cfg.APIKey != ""  // 3. config complete
}

// outlook adds a fourth: token exists
CheckFn: func() bool {
    if deps.Cfg.MultiUserMode { return false }
    cfg, ok := parseOutlookConfig(tc.Settings["outlook_agent_config"])
    if !ok || cfg.ClientID == "" || cfg.ClientSecret == "" { return false }
    _, err := deps.OutlookStore.Get(tc.Ctx, currentUserID(tc.User))
    return err == nil  // token has been issued
}
```

### Out of scope (explicit)

- **Attachment upload/download** — Node uses collector; Go's `prepareAttachment` integration is Phase 2
- **Multi-user OAuth** — strict single-user gate; multi-user requires per-user `client_id` UX which is its own design
- **Direct Google Cloud OAuth** (replacing Apps Script bridge) — design §13 lists this as a long-term path
- **Microsoft Graph extensions** (OneDrive / Teams / Calendar) — only Mail.* scopes
- **OAuth client_id/secret admin UI** — admins POST to existing `/api/system/setting` directly until a follow-up
- **Token refresh in DB advisory lock** — `sync.Mutex` is the single-process bound (acceptable for single-user)
- **Outlook 20+ actions** — only 5 ship; subset documented (see decision artefact above)
- **Apps Script template auto-deploy** — admins deploy manually with README guidance
- **OAuth state via Redis/distributed cache** — HMAC-self-contained, no server-side storage

### TDD discipline

Each task lands as **one commit**. Failing test → impl → green → full suite green → commit.

---

## Task 0: Decision artefacts + config knobs + EncryptionManager smoke

**Files:**
- `.gpowers/decisions/2026-05-27-oauth-single-user-only.md` (NEW)
- `.gpowers/decisions/2026-05-27-apps-script-go-template.md` (NEW)
- `.gpowers/decisions/2026-05-27-outlook-action-subset.md` (NEW)
- `backend/internal/config/config.go` (MODIFY)
- `backend/internal/agent/tools/oauth/doc.go` (NEW — package comment + smoke comment)
- `backend/internal/agent/tools/oauth/encryption_smoke_test.go` (NEW)

**Tests:**
- `TestEncryptionManager_RoundTripFromOauthPackage` (verifies the existing `Encrypt(plaintext)`/`Decrypt(ciphertext)` API is reachable from `oauth/` and the AES-GCM round-trip works on a refresh-token-shaped string)

### Steps

- [ ] Write `.gpowers/decisions/2026-05-27-oauth-single-user-only.md`:
  ```markdown
  # OAuth Skills — Single-User Mode Only

  **Date**: 2026-05-27
  **Status**: Adopted
  **Context**: gmail / google-calendar / outlook all hold credentials that grant access to a real human mailbox/calendar. Node disables them in `MULTI_USER_MODE=true` for the same reason.

  **Decision**: All three CheckFn return false when `cfg.MultiUserMode == true`. Multi-user OAuth (per-user credentials, per-user tokens) is out of scope for PR-AR-7.
  ```

- [ ] Write `.gpowers/decisions/2026-05-27-apps-script-go-template.md`:
  ```markdown
  # Apps Script — Go Project Owns Its Own Template

  **Date**: 2026-05-27
  **Status**: Adopted
  **Context**: Node ships an Apps Script template; admins deploy and store deploymentId+apiKey in SystemSetting. Go could reuse the Node-deployed script.

  **Decision**: Go ships its own template under `assets/apps-script/{gmail,gcal}/`. Admins running both Node and Go can either deploy twice (separate scripts) or share — protocol is wire-compatible (same action names, same envelope). Go ownership lets us tighten the protocol in future without coordinating with Node releases.
  ```

- [ ] Write `.gpowers/decisions/2026-05-27-outlook-action-subset.md`:
  ```markdown
  # Outlook Agent — v1 Action Subset (5 of 20+)

  **Date**: 2026-05-27
  **Status**: Adopted
  **Context**: Node exposes ~20 outlook actions (reply, mark_read, move_to_*, multiple draft helpers). The agent loop only needs a minimal surface.

  **Decision**: v1 ships `search`, `read_thread`, `read_message`, `create_draft`, `send_email`. Other actions defer to a follow-up PR. LLM can simulate `reply_to_thread` via `read_thread` then `send_email` with the captured subject/recipients.
  ```

- [ ] Add config knobs:
  ```go
  // config.go — add to existing struct
  PublicBaseURL    string `env:"PUBLIC_BASE_URL"`              // e.g. "https://anything.example.com"
  OutlookAuthority string `env:"OUTLOOK_AUTHORITY" envDefault:"common"`  // common | consumers | <tenant-id>
  ```

  In `Load()`:
  ```go
  if cfg.PublicBaseURL == "" {
      cfg.PublicBaseURL = "http://localhost:" + cfg.ServerPort
  }
  cfg.PublicBaseURL = strings.TrimRight(cfg.PublicBaseURL, "/")
  ```

- [ ] Write `internal/agent/tools/oauth/doc.go`:
  ```go
  // Package oauth provides the shared OAuth + Apps Script bridge infrastructure
  // used by gmail-agent, google-calendar-agent, and outlook-agent skills.
  //
  // BridgeClient is a thin HTTP wrapper around the Google Apps Script
  // deployment URL used by Gmail and Calendar. It does NOT speak Google
  // OAuth — Google access lives on the Apps Script side, executed as the
  // deploying admin.
  //
  // OutlookOAuth implements the Microsoft OAuth 2.0 authorization-code +
  // refresh-token flow. Refresh-token storage is encrypted with the existing
  // pkg/utils.EncryptionManager (AES-GCM); see TokenStore.
  //
  // state.go provides HMAC-signed state-parameter (CSRF + replay defense).
  //
  // All three skills are single-user-mode only.
  package oauth
  ```

- [ ] Write `internal/agent/tools/oauth/encryption_smoke_test.go`:
  ```go
  package oauth_test

  import (
      "testing"
      "github.com/odysseythink/hermind/backend/pkg/utils"
      "github.com/stretchr/testify/require"
  )

  func TestEncryptionManager_RoundTripFromOauthPackage(t *testing.T) {
      enc, err := utils.NewEncryptionManager(t.TempDir())
      require.NoError(t, err)
      const plain = "OAQABAAAAAAA-fake-refresh-token-shape"
      ct, err := enc.Encrypt(plain)
      require.NoError(t, err)
      require.NotEqual(t, plain, ct)
      pt, err := enc.Decrypt(ct)
      require.NoError(t, err)
      require.Equal(t, plain, pt)
  }
  ```

- [ ] `go build ./...` + `go vet ./...` clean; `go test ./internal/agent/tools/oauth/...` passes.

### Acceptance

- Three decision artefacts present
- `cfg.PublicBaseURL` + `cfg.OutlookAuthority` parse with sane defaults; trailing slash trimmed
- `oauth/doc.go` describes the three pieces (Bridge / OutlookOAuth / state)
- Encryption round-trip works from inside the oauth subpackage
- No other production code touched

### Commit

`feat(agent/tools/oauth): bootstrap package + config knobs + decision artefacts`

---

## Task 1: BridgeClient (shared Apps Script wire)

**Files:**
- `backend/internal/agent/tools/oauth/bridge_client.go` (NEW)
- `backend/internal/agent/tools/oauth/bridge_client_test.go` (NEW)

**Tests:**
- `TestBridgeClient_HappyPath_ReturnsData`
- `TestBridgeClient_ErrorEnvelope_ReturnsError` (HTTP 200, `{"status":"error","error":"..."}`)
- `TestBridgeClient_HTTPError_PropagatesStatus` (HTTP 500)
- `TestBridgeClient_Timeout_ReturnsContextError`
- `TestBridgeClient_LargeResponseCapped_ReturnsError` (response > 4 MiB)
- `TestBridgeClient_NonJSONResponse_ReturnsError`
- `TestBridgeClient_SetsHeaders` (User-Agent + Content-Type)
- `TestBridgeClient_BodyShape` (verify request body has `key`, `action`, and params at top level)

### Steps

- [ ] Write failing tests using `httptest.NewServer` to stub Apps Script:
  ```go
  func TestBridgeClient_HappyPath_ReturnsData(t *testing.T) {
      srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
          var body map[string]any
          json.NewDecoder(r.Body).Decode(&body)
          require.Equal(t, "secret-key", body["key"])
          require.Equal(t, "search", body["action"])
          require.Equal(t, "is:inbox", body["query"])
          w.Header().Set("Content-Type", "application/json")
          json.NewEncoder(w).Encode(map[string]any{
              "status": "ok",
              "data": []map[string]any{{"id":"123","subject":"hi"}},
          })
      }))
      defer srv.Close()

      bc := oauth.NewBridgeClient(5 * time.Second)
      // Inject test URL via package-local override (see Steps below)
      oauth.SetTestBaseURL(srv.URL)
      defer oauth.SetTestBaseURL("")

      raw, err := bc.Call(context.Background(), "test-deployment", "secret-key", "search", map[string]any{"query":"is:inbox"})
      require.NoError(t, err)
      require.Contains(t, string(raw), `"subject":"hi"`)
  }
  ```

- [ ] Implement `bridge_client.go`:
  ```go
  package oauth

  import (
      "bytes"
      "context"
      "encoding/json"
      "fmt"
      "io"
      "net/http"
      "time"
  )

  const (
      defaultBridgeTimeout = 30 * time.Second
      maxBridgeRespBytes   = 4 << 20 // 4 MiB
  )

  // testBaseURL is overridden by SetTestBaseURL in tests.
  var testBaseURL string

  // SetTestBaseURL overrides the script.google.com base URL during tests.
  // Pass "" to clear. Production callers MUST NOT use this.
  func SetTestBaseURL(u string) { testBaseURL = u }

  type BridgeClient struct {
      http *http.Client
  }

  func NewBridgeClient(timeout time.Duration) *BridgeClient {
      if timeout <= 0 { timeout = defaultBridgeTimeout }
      return &BridgeClient{http: &http.Client{Timeout: timeout}}
  }

  type bridgeEnvelope struct {
      Status string          `json:"status"`
      Data   json.RawMessage `json:"data"`
      Error  string          `json:"error"`
  }

  func (b *BridgeClient) Call(ctx context.Context, deploymentID, apiKey, action string, params map[string]any) (json.RawMessage, error) {
      url := b.endpoint(deploymentID)

      body := make(map[string]any, len(params)+2)
      body["key"] = apiKey
      body["action"] = action
      for k, v := range params { body[k] = v }
      payload, err := json.Marshal(body)
      if err != nil { return nil, fmt.Errorf("marshal request: %w", err) }

      req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(payload))
      if err != nil { return nil, fmt.Errorf("build request: %w", err) }
      req.Header.Set("Content-Type", "application/json")
      req.Header.Set("X-Hermind-UA", "Hermind-Agent-Go/1.0")

      resp, err := b.http.Do(req)
      if err != nil { return nil, fmt.Errorf("bridge call: %w", err) }
      defer resp.Body.Close()
      if resp.StatusCode != http.StatusOK {
          return nil, fmt.Errorf("bridge HTTP %d", resp.StatusCode)
      }

      raw, err := io.ReadAll(io.LimitReader(resp.Body, maxBridgeRespBytes+1))
      if err != nil { return nil, fmt.Errorf("read response: %w", err) }
      if len(raw) > maxBridgeRespBytes {
          return nil, fmt.Errorf("bridge response exceeds %d bytes", maxBridgeRespBytes)
      }

      var env bridgeEnvelope
      if err := json.Unmarshal(raw, &env); err != nil {
          return nil, fmt.Errorf("bridge response not JSON: %w", err)
      }
      if env.Status == "error" {
          return nil, fmt.Errorf("apps script error: %s", env.Error)
      }
      return env.Data, nil
  }

  func (b *BridgeClient) endpoint(deploymentID string) string {
      if testBaseURL != "" {
          return testBaseURL
      }
      return "https://script.google.com/macros/s/" + deploymentID + "/exec"
  }
  ```

- [ ] Run all 8 tests; verify pass.

### Acceptance

- All 8 tests pass
- 4 MiB response cap enforced
- `Status: "error"` in JSON body produces error even on HTTP 200
- Context cancellation propagates
- Request body shape verified (params merge at top level, alongside `key`+`action`)

### Commit

`feat(agent/tools/oauth): BridgeClient — Apps Script HTTP wire`

---

## Task 2: gmail-agent skill (12 actions)

**Files:**
- `backend/internal/agent/tools/gmail_agent.go` (NEW)
- `backend/internal/agent/tools/gmail_agent_test.go` (NEW)
- `backend/assets/apps-script/gmail/Code.gs` (NEW)
- `backend/assets/apps-script/gmail/appsscript.json` (NEW)
- `backend/assets/apps-script/gmail/README.md` (NEW)
- `backend/internal/agent/tools/builder.go` (MODIFY — add Bridge dep field)

**Tests:**
- `TestGmailAgent_CheckFn_FalseInMultiUserMode`
- `TestGmailAgent_CheckFn_FalseWhenConfigMissing`
- `TestGmailAgent_CheckFn_TrueWhenConfigured`
- `TestGmailAgent_Search_ForwardsToBridge` (verify params correctly forwarded)
- `TestGmailAgent_SendEmail_TriggersApproval`
- `TestGmailAgent_SendEmail_RejectedByUser_ReturnsToolError`
- `TestGmailAgent_ReadOnlyActions_BypassApproval` (table: search/read_thread/list_drafts/get_draft/mailbox_stats)
- `TestGmailAgent_BridgeError_Surfaced`
- `TestGmailAgent_UnknownAction_ReturnsToolError`
- `TestGmailAgent_DispatchViaRegistry` (e2e via `tool.Registry.Dispatch`)

### Steps

- [ ] Add `Bridge *oauth.BridgeClient` to `BuilderDeps`:
  ```go
  type BuilderDeps struct {
      // ... existing ...
      Bridge *oauth.BridgeClient  // NEW
  }
  ```

- [ ] Implement `gmail_agent.go`:
  ```go
  package tools

  import (
      "context"
      "encoding/json"
      "fmt"

      "github.com/odysseythink/hermind/backend/internal/agent/tools/oauth"
      "github.com/odysseythink/pantheon/core"
      "github.com/odysseythink/pantheon/tool"
  )

  const gmailConfigKey = "gmail_agent_config"

  type gmailBridgeConfig struct {
      DeploymentID string `json:"deploymentId"`
      APIKey       string `json:"apiKey"`
  }

  func parseGmailBridgeConfig(raw string) (gmailBridgeConfig, bool) {
      if raw == "" { return gmailBridgeConfig{}, false }
      var c gmailBridgeConfig
      if err := json.Unmarshal([]byte(raw), &c); err != nil { return c, false }
      return c, c.DeploymentID != "" && c.APIKey != ""
  }

  var gmailDestructiveActions = map[string]bool{
      "create_draft": true, "update_draft": true, "send_draft": true,
      "send_email":  true, "reply_to_thread": true, "delete_draft": true,
      "move_to_trash": true,
  }

  func NewGmailAgentSkill(tc *ToolContext, deps BuilderDeps) *tool.Entry {
      return &tool.Entry{
          Name:           "gmail-agent",
          Toolset:        "gmail",
          Description:    "Search, read, draft, and send Gmail messages via a Google Apps Script bridge configured by the admin.",
          Emoji:          "✉️",
          MaxResultChars: 16 * 1024,
          CheckFn: func() bool {
              if deps.Cfg == nil || deps.Cfg.MultiUserMode { return false }
              if deps.Bridge == nil { return false }
              _, ok := parseGmailBridgeConfig(tc.Settings[gmailConfigKey])
              return ok
          },
          Schema: core.ToolDefinition{
              Name:        "gmail-agent",
              Description: "Gmail operations",
              Parameters:  gmailAgentSchema(),
          },
          Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
              var args map[string]any
              if err := json.Unmarshal(raw, &args); err != nil {
                  return tool.Error("invalid args: " + err.Error()), nil
              }
              action, _ := args["action"].(string)
              if action == "" { return tool.Error("action is required"), nil }

              cfg, ok := parseGmailBridgeConfig(tc.Settings[gmailConfigKey])
              if !ok { return tool.Error("gmail not configured"), nil }

              if gmailDestructiveActions[action] && tc.Approval != nil {
                  desc := fmt.Sprintf("Gmail %s", action)
                  if to, _ := args["to"].(string); to != "" { desc += " to " + to }
                  if ok, reason := tc.Approval(ctx, "gmail-agent:"+action, args, desc); !ok {
                      return tool.Error("rejected: " + reason), nil
                  }
              }

              tc.Emit("Gmail: " + action)

              // strip action+key from params before forwarding
              params := make(map[string]any, len(args))
              for k, v := range args {
                  if k == "action" { continue }
                  params[k] = v
              }
              data, err := deps.Bridge.Call(ctx, cfg.DeploymentID, cfg.APIKey, action, params)
              if err != nil { return tool.Error(err.Error()), nil }
              return string(data), nil
          },
      }
  }
  ```

- [ ] Write `gmail_agent_test.go` covering all 10 tests with `oauth.SetTestBaseURL` + `httptest.NewServer`.

- [ ] Write `assets/apps-script/gmail/Code.gs` (per design §7.2):
  - `doPost(e)` with API_KEY check
  - dispatch table for all 12 actions
  - GmailApp.* implementations
  - `jsonResponse` helper

- [ ] Write `assets/apps-script/gmail/appsscript.json`:
  ```json
  {
    "timeZone": "America/New_York",
    "exceptionLogging": "STACKDRIVER",
    "runtimeVersion": "V8",
    "oauthScopes": [
      "https://www.googleapis.com/auth/script.send_mail",
      "https://www.googleapis.com/auth/gmail.modify",
      "https://www.googleapis.com/auth/gmail.compose",
      "https://www.googleapis.com/auth/gmail.readonly",
      "https://www.googleapis.com/auth/script.external_request"
    ],
    "webapp": {
      "executeAs": "USER_DEPLOYING",
      "access": "ANYONE_ANONYMOUS"
    }
  }
  ```

- [ ] Write `assets/apps-script/gmail/README.md` (per design §7.3, with 8-step deployment guide).

- [ ] Run all tests; verify pass.

### Acceptance

- All 10 tests pass
- 5 read-only actions never invoke `tc.Approval`
- 7 destructive actions always invoke it
- Bridge errors propagate as `tool.Error`
- Apps Script template + README present in `assets/apps-script/gmail/`
- Dispatch through `tool.Registry` works

### Commit

`feat(agent/tools): gmail-agent skill — 12-action Apps Script bridge`

---

## Task 3: google-calendar-agent skill (8 actions)

**Files:**
- `backend/internal/agent/tools/gcal_agent.go` (NEW)
- `backend/internal/agent/tools/gcal_agent_test.go` (NEW)
- `backend/assets/apps-script/gcal/Code.gs` (NEW)
- `backend/assets/apps-script/gcal/appsscript.json` (NEW)
- `backend/assets/apps-script/gcal/README.md` (NEW)

**Tests:**
- `TestGCalAgent_CheckFn_FalseInMultiUserMode`
- `TestGCalAgent_CheckFn_FalseWhenConfigMissing`
- `TestGCalAgent_CheckFn_TrueWhenConfigured`
- `TestGCalAgent_ListCalendars_ForwardsToBridge`
- `TestGCalAgent_CreateEvent_TriggersApproval`
- `TestGCalAgent_CreateEvent_RejectedByUser_ReturnsToolError`
- `TestGCalAgent_ReadOnlyActions_BypassApproval`
- `TestGCalAgent_QuickAdd_TriggersApproval` (natural-language → event creation is destructive)
- `TestGCalAgent_DispatchViaRegistry`

### Steps

- [ ] Implement `gcal_agent.go` (mirrors gmail_agent.go structure):
  ```go
  const gcalConfigKey = "google_calendar_agent_config"

  var gcalDestructiveActions = map[string]bool{
      "quick_add": true, "create_event": true, "update_event": true,
  }

  func NewGCalAgentSkill(tc *ToolContext, deps BuilderDeps) *tool.Entry {
      return &tool.Entry{
          Name:           "google-calendar-agent",
          Toolset:        "google-calendar",
          Description:    "List calendars, query events, and create/update events via a Google Apps Script bridge.",
          Emoji:          "📅",
          MaxResultChars: 12 * 1024,
          CheckFn: func() bool {
              if deps.Cfg == nil || deps.Cfg.MultiUserMode { return false }
              if deps.Bridge == nil { return false }
              _, ok := parseGmailBridgeConfig(tc.Settings[gcalConfigKey])  // same shape
              return ok
          },
          Schema: core.ToolDefinition{
              Name:        "google-calendar-agent",
              Description: "Google Calendar operations",
              Parameters:  gcalAgentSchema(),
          },
          Handler: /* identical structure to gmail, with gcalDestructiveActions + gcalConfigKey */,
      }
  }
  ```

- [ ] Refactor: extract a private helper to share Handler bodies between gmail/gcal (both are just `(configKey, destructiveSet) → Handler`):
  ```go
  // tools/oauth_skill_helpers.go (NEW)
  func newBridgeBackedSkill(name, toolset, desc string, schema *core.Schema, configKey string, destructive map[string]bool, tc *ToolContext, deps BuilderDeps) *tool.Entry {
      // ... unified Handler ...
  }
  ```

  Then gmail/gcal each become 15-line factories. **Decision point**: ship the duplication (clearer, ~50 lines each) or share (~20 lines + helper). Pick **share** for maintainability.

- [ ] Write `gcal_agent_test.go` with all 9 tests.

- [ ] Write `assets/apps-script/gcal/Code.gs` (CalendarApp.* dispatch table for 8 actions).

- [ ] Write `assets/apps-script/gcal/appsscript.json` and `README.md` (parallel to gmail).

- [ ] Run all tests; verify pass.

### Acceptance

- All 9 tests pass
- Shared helper used by both gmail + gcal; both skills are <30 lines each
- `quick_add` correctly classified as destructive (creates a calendar event)
- Apps Script template + README present

### Commit

`feat(agent/tools): google-calendar-agent skill + shared bridge helper`

---

## Task 4: Outlook OAuth core (state, OAuth flow, TokenStore)

**Files:**
- `backend/internal/agent/tools/oauth/state.go` (NEW)
- `backend/internal/agent/tools/oauth/state_test.go` (NEW)
- `backend/internal/models/outlook_oauth_token.go` (NEW)
- `backend/internal/models/db.go` (MODIFY — add to AutoMigrate)
- `backend/internal/agent/tools/oauth/outlook_token_store.go` (NEW)
- `backend/internal/agent/tools/oauth/outlook_token_store_test.go` (NEW)
- `backend/internal/agent/tools/oauth/outlook_oauth.go` (NEW)
- `backend/internal/agent/tools/oauth/outlook_oauth_test.go` (NEW)

**Tests:**
- `TestEncodeDecodeState_RoundTrip`
- `TestDecodeState_TamperedNonce_Rejected`
- `TestDecodeState_TamperedReturnTo_Rejected`
- `TestDecodeState_Expired_Rejected`
- `TestDecodeState_OpenRedirect_Rejected` (ReturnTo not starting with publicBaseURL)
- `TestDecodeState_MalformedBase64_Rejected`
- `TestTokenStore_SaveGet_RoundTrip`
- `TestTokenStore_Save_OverwritesExisting`
- `TestTokenStore_Get_NotFound_ReturnsErrTokenNotFound`
- `TestTokenStore_Delete_Idempotent`
- `TestTokenStore_AccessTokenAndRefreshAreEncryptedAtRest` (read row directly from DB, verify ciphertext ≠ plaintext)
- `TestOutlookOAuth_AuthorizeURL_ContainsRequiredParams`
- `TestOutlookOAuth_ExchangeCode_HappyPath` (mock Microsoft token endpoint)
- `TestOutlookOAuth_ExchangeCode_BadStatus_ReturnsError`
- `TestOutlookOAuth_ValidAccessToken_NotExpired_NoRefresh`
- `TestOutlookOAuth_ValidAccessToken_Expired_TriggersRefresh`
- `TestOutlookOAuth_ValidAccessToken_RefreshMissingNewRefreshToken_PreservesOld`
- `TestOutlookOAuth_ValidAccessToken_RefreshFails_ReturnsError`
- `TestOutlookOAuth_ConcurrentValidAccessToken_RefreshOnce` (10 goroutines, single refresh call observed)

### Steps

#### 4a. State encoding

- [ ] Implement `state.go`:
  ```go
  package oauth

  import (
      "crypto/hmac"
      "crypto/sha256"
      "encoding/base64"
      "encoding/json"
      "errors"
      "fmt"
      "strings"
      "time"
  )

  var (
      ErrStateInvalid  = errors.New("invalid state")
      ErrStateExpired  = errors.New("state expired")
      ErrStateRedirect = errors.New("state return_to outside trusted prefix")
  )

  type StatePayload struct {
      UserID    int    `json:"u"`
      Nonce     string `json:"n"`
      ReturnTo  string `json:"r"`
      ExpiresAt int64  `json:"e"`  // unix seconds
  }

  func EncodeState(secret []byte, p StatePayload) string {
      raw, _ := json.Marshal(p)
      mac := hmac.New(sha256.New, secret)
      mac.Write(raw)
      sig := mac.Sum(nil)
      return base64.RawURLEncoding.EncodeToString(raw) + "." +
          base64.RawURLEncoding.EncodeToString(sig)
  }

  func DecodeState(secret []byte, encoded, publicBaseURL string) (*StatePayload, error) {
      parts := strings.SplitN(encoded, ".", 2)
      if len(parts) != 2 { return nil, fmt.Errorf("%w: malformed", ErrStateInvalid) }
      raw, err := base64.RawURLEncoding.DecodeString(parts[0])
      if err != nil { return nil, fmt.Errorf("%w: bad base64 payload", ErrStateInvalid) }
      sig, err := base64.RawURLEncoding.DecodeString(parts[1])
      if err != nil { return nil, fmt.Errorf("%w: bad base64 signature", ErrStateInvalid) }

      mac := hmac.New(sha256.New, secret)
      mac.Write(raw)
      if !hmac.Equal(mac.Sum(nil), sig) {
          return nil, fmt.Errorf("%w: signature mismatch", ErrStateInvalid)
      }
      var p StatePayload
      if err := json.Unmarshal(raw, &p); err != nil {
          return nil, fmt.Errorf("%w: bad json", ErrStateInvalid)
      }
      if time.Now().Unix() > p.ExpiresAt {
          return nil, ErrStateExpired
      }
      if !strings.HasPrefix(p.ReturnTo, publicBaseURL) {
          return nil, ErrStateRedirect
      }
      return &p, nil
  }
  ```

- [ ] Write all 6 state tests; verify pass.

#### 4b. OutlookOAuthToken model + TokenStore

- [ ] Implement `models/outlook_oauth_token.go`:
  ```go
  package models

  import "time"

  type OutlookOAuthToken struct {
      ID                    int       `gorm:"primaryKey"`
      UserID                int       `gorm:"uniqueIndex;not null"`
      Tenant                string    `gorm:"not null"`
      EncryptedAccessToken  string    `gorm:"type:text;not null"`
      EncryptedRefreshToken string    `gorm:"type:text;not null"`
      ExpiresAt             time.Time `gorm:"not null"`
      CreatedAt             time.Time
      UpdatedAt             time.Time
  }

  func (OutlookOAuthToken) TableName() string { return "outlook_oauth_tokens" }
  ```

- [ ] Add `&OutlookOAuthToken{}` to `models/db.go` AutoMigrate list.

- [ ] Implement `oauth/outlook_token_store.go`:
  ```go
  package oauth

  import (
      "context"
      "errors"
      "fmt"
      "time"

      "github.com/odysseythink/hermind/backend/internal/models"
      "github.com/odysseythink/hermind/backend/pkg/utils"
      "gorm.io/gorm"
  )

  var ErrTokenNotFound = errors.New("outlook token not found")

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

  func NewTokenStore(db *gorm.DB, enc *utils.EncryptionManager) *TokenStore {
      return &TokenStore{db: db, enc: enc}
  }

  func (s *TokenStore) Get(ctx context.Context, userID int) (*TokenSet, error) {
      var row models.OutlookOAuthToken
      err := s.db.WithContext(ctx).Where("user_id = ?", userID).First(&row).Error
      if errors.Is(err, gorm.ErrRecordNotFound) { return nil, ErrTokenNotFound }
      if err != nil { return nil, err }

      at, err := s.enc.Decrypt(row.EncryptedAccessToken)
      if err != nil { return nil, fmt.Errorf("decrypt access: %w", err) }
      rt, err := s.enc.Decrypt(row.EncryptedRefreshToken)
      if err != nil { return nil, fmt.Errorf("decrypt refresh: %w", err) }

      return &TokenSet{
          AccessToken: at, RefreshToken: rt,
          ExpiresAt: row.ExpiresAt, Tenant: row.Tenant,
      }, nil
  }

  func (s *TokenStore) Save(ctx context.Context, userID int, ts *TokenSet) error {
      at, err := s.enc.Encrypt(ts.AccessToken)
      if err != nil { return fmt.Errorf("encrypt access: %w", err) }
      rt, err := s.enc.Encrypt(ts.RefreshToken)
      if err != nil { return fmt.Errorf("encrypt refresh: %w", err) }

      row := models.OutlookOAuthToken{
          UserID: userID, Tenant: ts.Tenant,
          EncryptedAccessToken: at, EncryptedRefreshToken: rt,
          ExpiresAt: ts.ExpiresAt,
      }
      // Upsert by user_id
      return s.db.WithContext(ctx).
          Where("user_id = ?", userID).
          Assign(row).
          FirstOrCreate(&row).Error
  }

  func (s *TokenStore) Delete(ctx context.Context, userID int) error {
      return s.db.WithContext(ctx).
          Where("user_id = ?", userID).
          Delete(&models.OutlookOAuthToken{}).Error
  }
  ```

- [ ] Write all 5 TokenStore tests with a sqlite-mem DB.

#### 4c. OutlookOAuth

- [ ] Implement `oauth/outlook_oauth.go`:
  ```go
  package oauth

  import (
      "context"
      "encoding/json"
      "fmt"
      "io"
      "net/http"
      "net/url"
      "strings"
      "sync"
      "time"
  )

  const (
      microsoftAuthBase = "https://login.microsoftonline.com"
      outlookScopes     = "offline_access Mail.Read Mail.ReadWrite Mail.Send User.Read"
      tokenExpiryLeeway = 60 * time.Second
  )

  // testMicrosoftBase overrides login.microsoftonline.com in tests.
  var testMicrosoftBase string

  func SetTestMicrosoftBase(u string) { testMicrosoftBase = u }

  type OutlookOAuth struct {
      store        *TokenStore
      redirectURI  string
      defaultAuth  string
      http         *http.Client
      refreshMu    sync.Mutex
  }

  func NewOutlookOAuth(store *TokenStore, publicBaseURL, defaultAuthority string, httpClient *http.Client) *OutlookOAuth {
      if httpClient == nil { httpClient = &http.Client{Timeout: 15 * time.Second} }
      return &OutlookOAuth{
          store:       store,
          redirectURI: publicBaseURL + "/api/oauth/outlook/callback",
          defaultAuth: defaultAuthority,
          http:        httpClient,
      }
  }

  func (o *OutlookOAuth) authBase() string {
      if testMicrosoftBase != "" { return testMicrosoftBase }
      return microsoftAuthBase
  }

  func (o *OutlookOAuth) RedirectURI() string { return o.redirectURI }

  func (o *OutlookOAuth) AuthorizeURL(state, clientID, authority string) string {
      if authority == "" { authority = o.defaultAuth }
      v := url.Values{}
      v.Set("client_id", clientID)
      v.Set("response_type", "code")
      v.Set("redirect_uri", o.redirectURI)
      v.Set("response_mode", "query")
      v.Set("scope", outlookScopes)
      v.Set("state", state)
      v.Set("prompt", "select_account")
      return fmt.Sprintf("%s/%s/oauth2/v2.0/authorize?%s", o.authBase(), authority, v.Encode())
  }

  func (o *OutlookOAuth) ExchangeCode(ctx context.Context, code, clientID, clientSecret, authority string) (*TokenSet, error) {
      if authority == "" { authority = o.defaultAuth }
      form := url.Values{}
      form.Set("client_id", clientID)
      form.Set("client_secret", clientSecret)
      form.Set("code", code)
      form.Set("redirect_uri", o.redirectURI)
      form.Set("grant_type", "authorization_code")
      form.Set("scope", outlookScopes)
      return o.tokenPOST(ctx, authority, form)
  }

  func (o *OutlookOAuth) refresh(ctx context.Context, refreshToken, clientID, clientSecret, authority string) (*TokenSet, error) {
      if authority == "" { authority = o.defaultAuth }
      form := url.Values{}
      form.Set("client_id", clientID)
      form.Set("client_secret", clientSecret)
      form.Set("refresh_token", refreshToken)
      form.Set("grant_type", "refresh_token")
      form.Set("scope", outlookScopes)
      return o.tokenPOST(ctx, authority, form)
  }

  type msTokenResp struct {
      AccessToken  string `json:"access_token"`
      RefreshToken string `json:"refresh_token"`
      ExpiresIn    int    `json:"expires_in"`
      TokenType    string `json:"token_type"`
      Error        string `json:"error"`
      ErrorDesc    string `json:"error_description"`
  }

  func (o *OutlookOAuth) tokenPOST(ctx context.Context, authority string, form url.Values) (*TokenSet, error) {
      url := fmt.Sprintf("%s/%s/oauth2/v2.0/token", o.authBase(), authority)
      req, _ := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(form.Encode()))
      req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

      resp, err := o.http.Do(req)
      if err != nil { return nil, fmt.Errorf("token endpoint: %w", err) }
      defer resp.Body.Close()
      body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
      var data msTokenResp
      if err := json.Unmarshal(body, &data); err != nil {
          return nil, fmt.Errorf("token response not JSON: %w (body=%s)", err, body)
      }
      if data.Error != "" {
          return nil, fmt.Errorf("microsoft oauth error: %s: %s", data.Error, data.ErrorDesc)
      }
      if data.AccessToken == "" {
          return nil, fmt.Errorf("token response missing access_token (status=%d)", resp.StatusCode)
      }
      return &TokenSet{
          AccessToken:  data.AccessToken,
          RefreshToken: data.RefreshToken,
          ExpiresAt:    time.Now().Add(time.Duration(data.ExpiresIn)*time.Second - tokenExpiryLeeway),
          Tenant:       authority,
      }, nil
  }

  func (o *OutlookOAuth) ValidAccessToken(ctx context.Context, userID int, clientID, clientSecret string) (string, error) {
      o.refreshMu.Lock()
      defer o.refreshMu.Unlock()

      ts, err := o.store.Get(ctx, userID)
      if err != nil { return "", err }
      if time.Now().Before(ts.ExpiresAt) {
          return ts.AccessToken, nil
      }
      newTS, err := o.refresh(ctx, ts.RefreshToken, clientID, clientSecret, ts.Tenant)
      if err != nil { return "", fmt.Errorf("refresh: %w", err) }
      if newTS.RefreshToken == "" { newTS.RefreshToken = ts.RefreshToken }
      if newTS.Tenant == "" { newTS.Tenant = ts.Tenant }
      if err := o.store.Save(ctx, userID, newTS); err != nil {
          return "", fmt.Errorf("save refreshed token: %w", err)
      }
      return newTS.AccessToken, nil
  }
  ```

- [ ] Write all 8 OutlookOAuth tests. For `TestOutlookOAuth_ConcurrentValidAccessToken_RefreshOnce`:
  ```go
  // 10 goroutines call ValidAccessToken simultaneously after expiry;
  // mock token endpoint counts hits; expect exactly 1.
  ```

- [ ] Full suite green.

### Acceptance

- All 19 tests pass (6 state + 5 store + 8 oauth)
- DB schema migrated; `outlook_oauth_tokens` table created
- Access + refresh tokens encrypted at rest (verified by reading row bytes)
- Concurrent `ValidAccessToken` after expiry refreshes exactly once
- Refresh response without `refresh_token` preserves the old one
- Open-redirect defense rejects `return_to=https://evil.com`

### Commit

`feat(agent/tools/oauth): state HMAC + TokenStore + OutlookOAuth (auth/exchange/refresh)`

---

## Task 5: OAuth HTTP handlers (4 routes)

**Files:**
- `backend/internal/handlers/oauth.go` (NEW)
- `backend/internal/handlers/oauth_test.go` (NEW)
- `backend/cmd/server/main.go` (MODIFY — wire deps + register routes)

**Tests:**
- `TestOutlookAuthorize_Unauthenticated_401`
- `TestOutlookAuthorize_HappyPath_302WithCorrectURL`
- `TestOutlookAuthorize_MissingConfig_503`
- `TestOutlookCallback_TamperedState_400ErrorPage`
- `TestOutlookCallback_ExpiredState_400ErrorPage`
- `TestOutlookCallback_OpenRedirect_400ErrorPage`
- `TestOutlookCallback_HappyPath_302ToReturnTo_AndTokenSaved`
- `TestOutlookCallback_MicrosoftError_500ErrorPage`
- `TestOutlookCallback_ErrorPageEscapesHTMLInMessage` (XSS defense)
- `TestOutlookDisconnect_Authenticated_DeletesToken`
- `TestOutlookDisconnect_Unauthenticated_401`
- `TestOutlookStatus_Connected_ReturnsExpiresAt`
- `TestOutlookStatus_Disconnected_ReturnsConnectedFalse`

### Steps

- [ ] Implement `handlers/oauth.go`:
  ```go
  package handlers

  import (
      "context"
      "crypto/rand"
      "encoding/hex"
      "encoding/json"
      "errors"
      "html/template"
      "net/http"
      "time"

      "github.com/gin-gonic/gin"
      "github.com/odysseythink/hermind/backend/internal/agent/tools/oauth"
      "github.com/odysseythink/hermind/backend/internal/middleware"
      "github.com/odysseythink/hermind/backend/internal/models"
      "github.com/odysseythink/hermind/backend/internal/services"
  )

  type OAuthHandler struct {
      outlook     *oauth.OutlookOAuth
      store       *oauth.TokenStore
      sysSvc      *services.SystemService
      stateSecret []byte
      publicBaseURL string
  }

  func NewOAuthHandler(outlook *oauth.OutlookOAuth, store *oauth.TokenStore, sysSvc *services.SystemService, stateSecret []byte, publicBaseURL string) *OAuthHandler {
      return &OAuthHandler{outlook: outlook, store: store, sysSvc: sysSvc, stateSecret: stateSecret, publicBaseURL: publicBaseURL}
  }

  func RegisterOAuthRoutes(r *gin.RouterGroup, h *OAuthHandler, authSvc *services.AuthService) {
      g := r.Group("/oauth/outlook")
      g.GET("/authorize", middleware.ValidatedRequest(authSvc), h.OutlookAuthorize)
      g.GET("/callback", h.OutlookCallback)  // state-self-authenticated
      g.POST("/disconnect", middleware.ValidatedRequest(authSvc), h.OutlookDisconnect)
      g.GET("/status", middleware.ValidatedRequest(authSvc), h.OutlookStatus)
  }

  type outlookConfig struct {
      ClientID     string `json:"clientId"`
      ClientSecret string `json:"clientSecret"`
      Tenant       string `json:"tenant,omitempty"`
  }

  func (h *OAuthHandler) loadConfig(ctx context.Context) (outlookConfig, error) {
      raw, err := h.sysSvc.GetSetting(ctx, "outlook_agent_config")
      if err != nil { return outlookConfig{}, err }
      if raw == "" { return outlookConfig{}, errors.New("outlook_agent_config not set") }
      var c outlookConfig
      if err := json.Unmarshal([]byte(raw), &c); err != nil { return outlookConfig{}, err }
      if c.ClientID == "" || c.ClientSecret == "" {
          return outlookConfig{}, errors.New("outlook_agent_config incomplete")
      }
      return c, nil
  }

  func (h *OAuthHandler) OutlookAuthorize(c *gin.Context) {
      user := c.MustGet("user").(*models.User)
      cfg, err := h.loadConfig(c.Request.Context())
      if err != nil {
          c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
          return
      }
      returnTo := c.Query("return_to")
      if returnTo == "" { returnTo = h.publicBaseURL }
      nonce := newNonce()
      state := oauth.EncodeState(h.stateSecret, oauth.StatePayload{
          UserID: user.ID, Nonce: nonce, ReturnTo: returnTo,
          ExpiresAt: time.Now().Add(10*time.Minute).Unix(),
      })
      url := h.outlook.AuthorizeURL(state, cfg.ClientID, cfg.Tenant)
      c.Redirect(http.StatusFound, url)
  }

  func (h *OAuthHandler) OutlookCallback(c *gin.Context) {
      code := c.Query("code")
      encState := c.Query("state")
      if code == "" || encState == "" {
          h.errorPage(c, http.StatusBadRequest, "Missing code or state")
          return
      }
      state, err := oauth.DecodeState(h.stateSecret, encState, h.publicBaseURL)
      if err != nil {
          h.errorPage(c, http.StatusBadRequest, err.Error())
          return
      }
      cfg, err := h.loadConfig(c.Request.Context())
      if err != nil {
          h.errorPage(c, http.StatusServiceUnavailable, err.Error())
          return
      }
      ts, err := h.outlook.ExchangeCode(c.Request.Context(), code, cfg.ClientID, cfg.ClientSecret, cfg.Tenant)
      if err != nil {
          h.errorPage(c, http.StatusInternalServerError, "OAuth exchange failed: "+err.Error())
          return
      }
      if err := h.store.Save(c.Request.Context(), state.UserID, ts); err != nil {
          h.errorPage(c, http.StatusInternalServerError, "Failed to save token: "+err.Error())
          return
      }
      c.Redirect(http.StatusFound, state.ReturnTo)
  }

  func (h *OAuthHandler) OutlookDisconnect(c *gin.Context) {
      user := c.MustGet("user").(*models.User)
      if err := h.store.Delete(c.Request.Context(), user.ID); err != nil {
          c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
          return
      }
      c.JSON(http.StatusOK, gin.H{"success": true})
  }

  func (h *OAuthHandler) OutlookStatus(c *gin.Context) {
      user := c.MustGet("user").(*models.User)
      ts, err := h.store.Get(c.Request.Context(), user.ID)
      if errors.Is(err, oauth.ErrTokenNotFound) {
          c.JSON(http.StatusOK, gin.H{"connected": false}); return
      }
      if err != nil {
          c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()}); return
      }
      c.JSON(http.StatusOK, gin.H{"connected": true, "expiresAt": ts.ExpiresAt})
  }

  // errorPage renders a minimal HTML error page with HTML-escaped message.
  func (h *OAuthHandler) errorPage(c *gin.Context, status int, message string) {
      safe := template.HTMLEscapeString(message)
      html := `<!doctype html><html><body>
<h1>OAuth Error</h1>
<p>` + safe + `</p>
<p>Please close this window and try again.</p>
</body></html>`
      c.Header("Content-Type", "text/html; charset=utf-8")
      c.String(status, html)
  }

  func newNonce() string {
      b := make([]byte, 16)
      _, _ = rand.Read(b)
      return hex.EncodeToString(b)
  }
  ```

- [ ] Write all 13 tests using `apiTestEnv` + mocked Microsoft endpoint (via `oauth.SetTestMicrosoftBase`).

- [ ] Wire in `main.go`:
  ```go
  // Around the existing handler registrations (line ~166 area)
  bridgeClient := oauth.NewBridgeClient(30 * time.Second)
  tokenStore := oauth.NewTokenStore(db, enc)
  outlookOAuth := oauth.NewOutlookOAuth(tokenStore, cfg.PublicBaseURL, cfg.OutlookAuthority, nil)
  stateSecret := []byte(cfg.JWTSecret)  // reuse JWT secret per design §5.2
  oauthHandler := handlers.NewOAuthHandler(outlookOAuth, tokenStore, sysSvc, stateSecret, cfg.PublicBaseURL)
  handlers.RegisterOAuthRoutes(api, oauthHandler, authSvc)
  ```

- [ ] Run all tests; verify pass.

### Acceptance

- All 13 tests pass
- Callback error page escapes HTML in error message (XSS defense)
- Callback only succeeds with valid HMAC + non-expired state + return_to under publicBaseURL
- Disconnect deletes token; subsequent status returns `connected: false`
- main.go boots without panic; routes mounted under `/api/oauth/outlook/*`

### Commit

`feat(agent): OAuth HTTP handlers — authorize/callback/disconnect/status`

---

## Task 6: outlook-agent skill + Builder registration + final e2e

**Files:**
- `backend/internal/agent/tools/outlook_agent.go` (NEW)
- `backend/internal/agent/tools/outlook_agent_test.go` (NEW)
- `backend/internal/agent/tools/builder.go` (MODIFY — register 3 skills, pass new BuilderDeps fields)
- `backend/internal/agent/runtime.go` (MODIFY — Deps struct gains Bridge/OutlookOAuth/OutlookStore)
- `backend/internal/agent/handler.go` (MODIFY — populate BuilderDeps with the new fields)
- `backend/cmd/server/main.go` (MODIFY — pass new deps to agent.NewRuntime)

**Tests:**
- `TestOutlookAgent_CheckFn_FalseInMultiUserMode`
- `TestOutlookAgent_CheckFn_FalseWithoutConfig`
- `TestOutlookAgent_CheckFn_FalseWithoutToken` (config present, no token row)
- `TestOutlookAgent_CheckFn_TrueWithConfigAndToken`
- `TestOutlookAgent_Search_CallsGraphAPI` (mock graph.microsoft.com)
- `TestOutlookAgent_SendEmail_TriggersApproval`
- `TestOutlookAgent_SendEmail_RejectedByUser_ReturnsToolError`
- `TestOutlookAgent_TokenAutoRefreshOnExpiry`
- `TestOutlookAgent_DispatchViaRegistry`
- `TestBuilder_AllTenDefaultSkillsRegistered_InSingleUserMode` (rag + docSummarizer + webScraping + rechart + sql + filesystem + createFiles + gmail + gcal + outlook)
- `TestBuilder_OAuthSkillsHidden_InMultiUserMode` (3 OAuth skills missing)

### Steps

- [ ] Implement `outlook_agent.go`:
  ```go
  package tools

  import (
      "bytes"
      "context"
      "encoding/json"
      "fmt"
      "io"
      "net/http"

      "github.com/odysseythink/hermind/backend/internal/agent/tools/oauth"
      "github.com/odysseythink/pantheon/core"
      "github.com/odysseythink/pantheon/tool"
  )

  const (
      outlookConfigKey = "outlook_agent_config"
      graphAPIBase     = "https://graph.microsoft.com/v1.0"
  )

  var (
      // testGraphBase overrides graph.microsoft.com during tests.
      testGraphBase string
      outlookDestructiveActions = map[string]bool{
          "create_draft": true, "send_email": true,
      }
  )

  func SetTestGraphBase(u string) { testGraphBase = u }

  type outlookSkillConfig struct {
      ClientID     string `json:"clientId"`
      ClientSecret string `json:"clientSecret"`
      Tenant       string `json:"tenant,omitempty"`
  }

  func NewOutlookAgentSkill(tc *ToolContext, deps BuilderDeps) *tool.Entry {
      return &tool.Entry{
          Name:           "outlook-agent",
          Toolset:        "outlook",
          Description:    "Search, read, draft, and send Outlook messages via Microsoft Graph (single-user OAuth).",
          Emoji:          "📧",
          MaxResultChars: 16 * 1024,
          CheckFn: func() bool {
              if deps.Cfg == nil || deps.Cfg.MultiUserMode { return false }
              if deps.OutlookOAuth == nil || deps.OutlookStore == nil { return false }
              cfg, ok := parseOutlookSkillConfig(tc.Settings[outlookConfigKey])
              if !ok { return false }
              _ = cfg
              if tc.User == nil { return false }
              _, err := deps.OutlookStore.Get(tc.Ctx, tc.User.ID)
              return err == nil
          },
          Schema: core.ToolDefinition{
              Name:        "outlook-agent",
              Description: "Outlook mail operations",
              Parameters:  outlookAgentSchema(),
          },
          Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
              var args map[string]any
              if err := json.Unmarshal(raw, &args); err != nil { return tool.Error(err.Error()), nil }
              action, _ := args["action"].(string)
              if action == "" { return tool.Error("action is required"), nil }

              cfg, ok := parseOutlookSkillConfig(tc.Settings[outlookConfigKey])
              if !ok { return tool.Error("outlook not configured"), nil }

              if outlookDestructiveActions[action] && tc.Approval != nil {
                  desc := fmt.Sprintf("Outlook %s", action)
                  if to, _ := args["to"].(string); to != "" { desc += " to " + to }
                  if ok, reason := tc.Approval(ctx, "outlook-agent:"+action, args, desc); !ok {
                      return tool.Error("rejected: " + reason), nil
                  }
              }

              if tc.User == nil { return tool.Error("user required for outlook"), nil }
              tok, err := deps.OutlookOAuth.ValidAccessToken(ctx, tc.User.ID, cfg.ClientID, cfg.ClientSecret)
              if err != nil { return tool.Error("token refresh: " + err.Error()), nil }

              tc.Emit("Outlook: " + action)

              switch action {
              case "search":
                  q, _ := args["query"].(string)
                  return graphGET(ctx, tok, "/me/messages?$search=\""+q+"\"&$top=10")
              case "read_thread":
                  cid, _ := args["conversation_id"].(string)
                  return graphGET(ctx, tok, "/me/messages?$filter=conversationId eq '"+cid+"'")
              case "read_message":
                  mid, _ := args["message_id"].(string)
                  return graphGET(ctx, tok, "/me/messages/"+mid)
              case "create_draft":
                  return graphPOST(ctx, tok, "/me/messages", buildOutlookMessage(args))
              case "send_email":
                  return graphPOST(ctx, tok, "/me/sendMail", map[string]any{"message": buildOutlookMessage(args)})
              default:
                  return tool.Error("unknown action: " + action), nil
              }
          },
      }
  }

  func buildOutlookMessage(args map[string]any) map[string]any {
      to, _ := args["to"].(string)
      subject, _ := args["subject"].(string)
      body, _ := args["body"].(string)
      return map[string]any{
          "subject": subject,
          "body": map[string]any{"contentType": "Text", "content": body},
          "toRecipients": []map[string]any{
              {"emailAddress": map[string]any{"address": to}},
          },
      }
  }

  func parseOutlookSkillConfig(raw string) (outlookSkillConfig, bool) {
      if raw == "" { return outlookSkillConfig{}, false }
      var c outlookSkillConfig
      if err := json.Unmarshal([]byte(raw), &c); err != nil { return c, false }
      return c, c.ClientID != "" && c.ClientSecret != ""
  }

  func graphBase() string {
      if testGraphBase != "" { return testGraphBase }
      return graphAPIBase
  }

  func graphGET(ctx context.Context, tok, path string) (string, error) {
      req, _ := http.NewRequestWithContext(ctx, "GET", graphBase()+path, nil)
      req.Header.Set("Authorization", "Bearer "+tok)
      return doGraph(req)
  }

  func graphPOST(ctx context.Context, tok, path string, body map[string]any) (string, error) {
      payload, _ := json.Marshal(body)
      req, _ := http.NewRequestWithContext(ctx, "POST", graphBase()+path, bytes.NewReader(payload))
      req.Header.Set("Authorization", "Bearer "+tok)
      req.Header.Set("Content-Type", "application/json")
      return doGraph(req)
  }

  func doGraph(req *http.Request) (string, error) {
      client := &http.Client{Timeout: 30 * 1e9}
      resp, err := client.Do(req)
      if err != nil { return tool.Error("graph: " + err.Error()), nil }
      defer resp.Body.Close()
      raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
      if resp.StatusCode >= 400 {
          return tool.Error(fmt.Sprintf("graph HTTP %d: %s", resp.StatusCode, raw)), nil
      }
      return string(raw), nil
  }
  ```

  > **Note** on the `time.Second` literal: change `30 * 1e9` to `30 * time.Second` after import.

- [ ] Extend `BuilderDeps` to carry the three new fields:
  ```go
  type BuilderDeps struct {
      // ... existing ...
      Bridge        *oauth.BridgeClient
      OutlookOAuth  *oauth.OutlookOAuth
      OutlookStore  *oauth.TokenStore
  }
  ```

- [ ] Extend `agent.Deps`:
  ```go
  type Deps struct {
      // ... existing ...
      Bridge        *oauth.BridgeClient
      OutlookOAuth  *oauth.OutlookOAuth
      OutlookStore  *oauth.TokenStore
  }
  ```

- [ ] In `handler.go` `buildSessionRegistry`, populate the three new BuilderDeps fields from agent.Deps.

- [ ] Register the three skills in `Builder.Build` Source 1 (after `NewCreateFilesAgentSkill`):
  ```go
  NewGmailAgentSkill(tc, b.deps),
  NewGCalAgentSkill(tc, b.deps),
  NewOutlookAgentSkill(tc, b.deps),
  ```

- [ ] Wire `main.go`:
  ```go
  // Around the existing services bloc; bridgeClient/tokenStore/outlookOAuth already created in Task 5
  agentRuntime := agent.NewRuntime(agent.Deps{
      // ... existing fields ...
      Bridge:        bridgeClient,
      OutlookOAuth:  outlookOAuth,
      OutlookStore:  tokenStore,
  })
  ```

- [ ] Write all 11 tests including the e2e Builder verification.

- [ ] `go test ./... -race` clean; `go vet ./...` clean.

### Acceptance

- All 11 tests pass
- 10 default skills register in single-user mode; 3 OAuth skills missing in multi-user mode
- Outlook send_email auto-refreshes token before calling Graph API (verified by setting `ExpiresAt` to past)
- All three skills appear in `Definitions(nil)` only when fully configured

### Commit

`feat(agent/tools): outlook-agent skill + register all 3 OAuth skills`

---

## Post-PR checklist

- [ ] `go build ./...` clean
- [ ] `go vet ./...` clean
- [ ] `go test ./... -race` 100% green
- [ ] `gofmt -l . | wc -l` returns 0
- [ ] 3 decision artefacts in `.gpowers/decisions/`
- [ ] Apps Script templates (`assets/apps-script/{gmail,gcal}/`) include Code.gs + appsscript.json + README.md
- [ ] `OutlookOAuthToken` table created on fresh boot
- [ ] `cfg.PublicBaseURL` documented in `.env.example` (add an entry if missing)
- [ ] Manual smoke procedure documented in `internal/agent/doc.go`:
  ```
  1. Set OUTLOOK env vars + register OAuth app at portal.azure.com
  2. SetSetting "outlook_agent_config" with clientId/clientSecret
  3. GET /api/oauth/outlook/authorize?return_to=http://localhost:3001/dashboard
  4. Approve in Microsoft consent
  5. Land back at /dashboard; outlook-agent now appears in tool list
  6. @agent send an email to me@example.com saying "test"
  7. Approve in WS UI; mail sent
  ```
- [ ] FE companion brief written: `.gpowers/notes/2026-05-27-fe-companion-pr-AR-7.md` describing the 3 settings pages (gmail/gcal/outlook) the frontend needs to add for admin UX
- [ ] No new TODOs without ticket reference

## Risk notes

| Risk | Mitigation |
|---|---|
| Apps Script deployment fails because of OAuth consent screen | README walks through "Execute as: Me" + "Who has access: Anyone with the link"; first request triggers consent |
| Apps Script quota exceeded (script.google.com daily limit) | Documented in README; user-visible failure shows envelope error |
| OAuth state HMAC secret rotates (JWT secret rotation) | In-flight OAuth flow fails (10min window); document "stop OAuth flows before JWT rotation" |
| `refresh_token` encryption key changes (`STORAGE_DIR` move) | Decryption fails → user must reconnect; existing Hermind behavior |
| Microsoft revokes refresh token after long idle | `ValidAccessToken` returns error; user re-authorizes; tool surface temporarily hidden by CheckFn |
| `graph.microsoft.com` outage | Tool returns error envelope; agent loop continues; user retries |
| Concurrent agent sessions racing on refresh | `sync.Mutex` in `OutlookOAuth` serializes — bounded by single-user concurrency |
| Multi-process Go deployment (load balancer) → mutex doesn't protect | Out of scope; design §11 lists DB advisory lock for Phase 2 |
| Open-redirect via `return_to=https://evil.com/...?phish` | `DecodeState` rejects non-prefix matches against `cfg.PublicBaseURL` |
| XSS in OAuth error page (error message contains HTML) | `template.HTMLEscapeString` applied; verified by test |
| Apps Script API key leaked via env-vars or logs | Stored in SystemSetting (DB), never in env; not logged by BridgeClient |
| Outlook `clientSecret` in plaintext SystemSetting | Design §8.2 calls for `enc:` prefix encryption; Task 5 punted to "future cleanup" — file follow-up issue |
| Microsoft API breaking change | Pinned scopes + endpoint paths; monitor |
| Apps Script template diverges from Node's | Templates wire-compatible; admin can fall back to Node deployment |

## Estimate

| Task | Hours |
|---|---|
| 0. Wiring + decision artefacts + smoke | 2.0 |
| 1. BridgeClient (Apps Script wire) | 2.0 |
| 2. gmail-agent skill + Apps Script template | 4.0 |
| 3. gcal-agent skill + Apps Script template + shared helper | 3.0 |
| 4. OutlookOAuth + TokenStore + state | 6.0 |
| 5. OAuth HTTP handlers (4 routes) | 4.0 |
| 6. outlook-agent skill + Builder reg + final e2e | 4.0 |
| **Total** | **25.0** (design estimate 22-27h, mid-range ✓) |

—— end of plan
