# P4b System User-State Routes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use gpowers:subagent-driven-development (recommended) or gpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the 5 missing `/api/system/*` user-state routes (`check-token`, `refresh-user`, `update-password`, `enable-multi-user`, `POST /system/user`) in backend with response parity to Node `server/endpoints/system.js`.

**Architecture:** Routes mount in `internal/handlers/system.go` (move the existing stub `UpdatePassword` away from `AuthHandler`). Credential mutation (AUTH_TOKEN / JWT_SECRET / MULTI_USER_MODE) lives in `AuthService` (single source of truth for credentials) and is dual-written: in-memory `*config.Config` field assignment + persistence to `system_settings`. No new external infra.

**Tech Stack:** Go 1.22+, Gin, GORM, SQLite (test), bcrypt (`pkg/utils.HashPassword`), `pkg/utils.GenerateJWT`, testify.

**Source spec:** `.gpowers/designs/2026-05-22-backend-api-routes-design.md` §5.1 (用户态 5 条).

**Reference Node implementation:**
- `server/endpoints/system.js:113-181` (check-token, refresh-user)
- `server/endpoints/system.js:557-645` (update-password, enable-multi-user)
- `server/endpoints/system.js:1176-1210` (POST /system/user)
- `server/models/user.js:16-96` (`usernameRegex`, `validations`, `filterFields`, `checkPasswordComplexity`)

---

## Pre-task: Read this section once before starting

### Existing Go surface (do not duplicate)

- `handlers.SystemHandler` is the host for all `/system/*` routes. Already has `Ping`, `SetupComplete`, `MultiUserMode`, `Logo`, `ApiKeys`, `GenerateApiKey`, `DeleteApiKey`, `EventLogs`, `Metrics`, `CustomModels`, etc. — keep all.
- `handlers.AuthHandler.UpdatePassword` is a stub (`handlers/auth.go:88-95`) that always returns `{success: true}`. It is registered at `POST /api/system/update-password` in `auth.go:129`. **P4b Task 5 moves it to `SystemHandler` and replaces the stub body with a real impl.** Remove the stub method + route line from auth.go.
- `services.AuthService` already wraps `cfg` and `db`. We will extend it with `RotateCredentials(ctx, newPassword)` and `EnableMultiUser(ctx, username, password)`.
- `services.SystemService.SetSetting` persists `system_settings.key=value` and caches in `sync.Map`. Used for DB-level persistence.
- `middleware.ValidatedRequest(authSvc)` sets `c.Set("user", *models.User)`. P4b handlers read it via `c.MustGet("user").(*models.User)`.
- `models.User` fields: `ID`, `Username *string`, `Password`, `Role`, `Suspended int`, `Bio *string`, `PfpFilename *string`, `DailyMessageLimit *int`, `SeenRecoveryCodes *bool`, `WebPushSubscriptionConfig *string`, `CreatedAt`, `LastUpdatedAt`. Password has `json:"-"`. WebPushSubscriptionConfig should be stripped from API responses to match Node's `filterFields`.
- `services.AdminService.CreateUser` (added in P4a) already covers user creation with bcrypt + uniqueness mapping. P4b reuses it via `AuthService` for `enable-multi-user`. Alternatively, `AuthService.Register` (which lacks role handling) is too narrow — prefer reusing `AdminService.CreateUser` with `Role: "admin"`.
- `pkg/utils.HashPassword`, `utils.CheckPassword`, `utils.GenerateJWT`, `utils.Ptr` already available.
- `services.NewSystemService(db).SetSetting(ctx, key, value)` returns `error`.

### Routes to add (5)

| # | Method | Path | Middleware | Body shape |
|---|---|---|---|---|
| 1 | GET | `/api/system/check-token` | `ValidatedRequest` | — |
| 2 | GET | `/api/system/refresh-user` | `ValidatedRequest` | — |
| 3 | POST | `/api/system/update-password` | `ValidatedRequest` | `{usePassword: bool, newPassword: string}` |
| 4 | POST | `/api/system/enable-multi-user` | `ValidatedRequest` | `{username: string, password: string}` |
| 5 | POST | `/api/system/user` | `ValidatedRequest` | `{username?: string, password?: string, bio?: string}` |

All five sit under `RegisterSystemRoutes` (already in main.go:134). No new top-level wiring change.

### Cross-cutting helper: `services.user_validations.go`

Node has `User.validations.username`, `User.validations.role`, `User.validations.bio`, `User.checkPasswordComplexity`. P4a inlined a partial username + role check inside `AdminService.CreateUser` (admin_service.go:151-165). P4b extracts them into a new file `services/user_validations.go` so admin + system handlers share one implementation. **Do NOT change the wire-level behavior of any P4a admin handler when extracting** — the inline check stays semantically identical.

Helpers to add:

```go
// File: backend/internal/services/user_validations.go
package services

import (
    "errors"
    "regexp"
)

// usernameRegex matches Node's User.usernameRegex: lowercase start, then [a-z0-9._@-]
var usernameRegex = regexp.MustCompile(`^[a-z][a-z0-9._@-]*$`)

// ValidateUsername returns the trimmed username or an error matching Node's User.validations.username.
func ValidateUsername(raw string) (string, error) {
    if len(raw) > 32 {
        return "", errors.New("Username cannot be longer than 32 characters")
    }
    if len(raw) < 2 {
        return "", errors.New("Username must be at least 2 characters")
    }
    if !usernameRegex.MatchString(raw) {
        return "", errors.New("Username must start with a lowercase letter and only contain lowercase letters, numbers, underscores, hyphens, and periods")
    }
    return raw, nil
}

// ValidateBio matches Node's User.validations.bio: empty allowed, max 1000 chars.
func ValidateBio(raw string) (string, error) {
    if raw == "" {
        return "", nil
    }
    if len(raw) > 1000 {
        return "", errors.New("Bio cannot be longer than 1,000 characters")
    }
    return raw, nil
}

// CheckPasswordComplexity returns nil for "ok"; an error otherwise.
// P4b uses Node's lenient defaults: min=8, max=250, no character-class requirements.
// Env-driven overrides (PASSWORDMINCHAR etc.) are an explicit non-goal for P4b; track as
// follow-up if user requests it.
func CheckPasswordComplexity(pw string) error {
    if len(pw) < 8 {
        return errors.New("\"password\" length must be at least 8 characters long")
    }
    if len(pw) > 250 {
        return errors.New("\"password\" length must be less than or equal to 250 characters long")
    }
    return nil
}
```

`services.AdminService.CreateUser` and `services.AdminService.UpdateUser` (P4a) **may** be retrofitted to call these helpers in a small Task 2 refactor — only if their existing tests stay green.

### Out of scope (explicit)

- `EventLogs.logEvent` calls (`multi_user_mode_enabled`, etc.) — Node logs these. **Skip**; no `EventLog` model in Go yet.
- `Telemetry.sendTelemetry("enabled_multi_user_mode", ...)` — no telemetry pipe in Go yet.
- `BrowserExtensionApiKey.migrateApiKeysToMultiUser(user.id)` (called inside Node's `enable-multi-user`) — `models.BrowserExtensionApiKey` does not exist in Go (handlers/browser_extension.go returns empty). Defer to P4e Misc.
- `AgentSkillWhitelist.clearSingleUserWhitelist()` (called inside Node's `enable-multi-user`) — agent skill subsystem is P10. Defer.
- `User.update`'s automatic `seen_recovery_codes` flip on first password rotation — single-user has no recovery codes, so this is a no-op for P4b.
- **Startup overlay** for cfg-from-DB rehydration: Go cfg is parsed from env at process start by caarlos0/env and never refreshed. Both P4b mutations (`update-password`, `enable-multi-user`) dual-write: persist to `system_settings` **and** mutate `*cfg` in-memory. This is consistent with the existing `POST /system/update-env` handler (system.go:119). After process restart, env values override DB until a startup-overlay task is done. Document this in commit message and design `Known Gaps`. **Do not** add the overlay in P4b.
- `process.env.PASSWORDMINCHAR` and the other 6 password complexity env knobs — P4b uses fixed `min=8 max=250 reqCount=0` to match Node's lenient default. Track as follow-up if users complain.

### Response-shape conventions (match Node exactly)

- `check-token`: 200 empty body if OK; 403 empty body if suspended in multi-user mode. No JSON.
- `refresh-user`: 200 JSON `{success, user, message}`. Single-user → `{success: true, user: null, message: null}`. Suspended → `{success: false, user: null, message: "User is suspended."}`. Missing session → `{success: false, user: null, message: "Session expired or invalid."}`. OK → `{success: true, user: <filtered>, message: null}`.
- `update-password`: always 200 (even on errors) `{success: bool, error: string|null}`. Reject with 401 empty body in multi-user mode (matches Node `response.sendStatus(401).end()`).
- `enable-multi-user`: 200 `{success: bool, error: string|null}`. Specifically `{success: false, error: "Multi-user mode is already enabled."}` if already on.
- `POST /system/user`: 200 `{success: bool, error: string|null}`. 400 `{success: false, error: "..."}` for bad input (no body / invalid id / no updates).

### User-filter helper

Add a small `services.FilterUserFields(u *models.User) map[string]any` that returns Node-equivalent shape (strip `password`, `webPushSubscriptionConfig`). Use it in `refresh-user`. Place inside `services/user_validations.go` (same file as validators):

```go
func FilterUserFields(u *models.User) map[string]any {
    if u == nil {
        return nil
    }
    return map[string]any{
        "id":                  u.ID,
        "username":            u.Username,
        "pfpFilename":         u.PfpFilename,
        "role":                u.Role,
        "suspended":           u.Suspended,
        "seenRecoveryCodes":   u.SeenRecoveryCodes,
        "createdAt":           u.CreatedAt,
        "lastUpdatedAt":       u.LastUpdatedAt,
        "dailyMessageLimit":   u.DailyMessageLimit,
        "bio":                 u.Bio,
    }
}
```

### TDD discipline

Each task: write failing test → run & confirm fail → implement → run & confirm pass → commit. Tests hit HTTP routes via `httptest` unless the task explicitly creates a service-layer test (Tasks 1, 2 do; 3-8 are integration).

### Test setup helper

P4a created `setupAdminRouter(t)` in `tests/integration/admin_test.go:22-43`. P4b creates a parallel `setupSystemUserRouter(t)` in a new file `tests/integration/system_user_test.go` that wires only the routes touched by P4b. Copy the pattern:

```go
func setupSystemUserRouter(t *testing.T) (*gin.Engine, *gorm.DB, *services.AuthService, *config.Config) {
    gin.SetMode(gin.TestMode)
    cfg := &config.Config{
        StorageDir:    t.TempDir(),
        JWTSecret:     "test-secret",
        AuthToken:     "test-auth-token",
        MultiUserMode: false, // tests flip this per-case
    }
    db, err := services.NewDB(cfg)
    assert.NoError(t, err)
    t.Cleanup(func() {
        if sqlDB, _ := db.DB(); sqlDB != nil { sqlDB.Close() }
    })
    assert.NoError(t, services.AutoMigrate(db))
    enc, _ := utils.NewEncryptionManager(cfg.StorageDir)
    authSvc := services.NewAuthService(db, cfg, enc)
    sysSvc  := services.NewSystemService(db)
    adminSvc := services.NewAdminService(db)
    apiKeySvc := services.NewAPIKeyService(db)
    r := gin.New()
    handlers.RegisterSystemRoutes(r.Group("/api"), sysSvc, apiKeySvc, cfg, authSvc, adminSvc)
    return r, db, authSvc, cfg
}

func seedAuthedUser(t *testing.T, db *gorm.DB, cfg *config.Config, role string) (*models.User, string) {
    t.Helper()
    hash, _ := utils.HashPassword("pw")
    u := &models.User{Username: utils.Ptr("alice"), Password: hash, Role: role, Bio: utils.Ptr("")}
    assert.NoError(t, db.Create(u).Error)
    tok, err := utils.GenerateJWT(cfg.JWTSecret, map[string]any{"userId": u.ID}, time.Hour)
    assert.NoError(t, err)
    return u, tok
}

func singleUserToken(t *testing.T, cfg *config.Config, authSvc *services.AuthService) string {
    t.Helper()
    tok, err := authSvc.CreateSingleUserToken(context.Background())
    assert.NoError(t, err)
    return tok
}
```

Note: `RegisterSystemRoutes` will gain an `adminSvc *services.AdminService` parameter in Task 6 (for `enable-multi-user` → `AdminService.CreateUser`). Update main.go:134 accordingly when that task lands.

---

## Task 1: Extract username / password / bio validators

Add `services/user_validations.go` with `ValidateUsername`, `ValidateBio`, `CheckPasswordComplexity`, `FilterUserFields`. Add `TestUserValidations_*` with table-driven tests.

**Files:**
- Create: `backend/internal/services/user_validations.go`
- Create: `backend/internal/services/user_validations_test.go`

- [ ] **Step 1: Write the failing test**

```go
// File: backend/internal/services/user_validations_test.go
package services

import (
    "testing"

    "github.com/odysseythink/hermind/backend/internal/models"
    "github.com/odysseythink/hermind/backend/pkg/utils"
)

func TestValidateUsername(t *testing.T) {
    cases := []struct {
        name string
        in   string
        ok   bool
    }{
        {"ok lower", "alice", true},
        {"ok dots/dashes", "a.b-c_d@e", true},
        {"too short", "a", false},
        {"too long", "a23456789012345678901234567890123", false}, // 33 chars
        {"starts with digit", "1abc", false},
        {"upper case", "Alice", false},
        {"space", "al ice", false},
    }
    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            _, err := ValidateUsername(tc.in)
            if (err == nil) != tc.ok {
                t.Fatalf("ValidateUsername(%q) ok=%v, err=%v", tc.in, tc.ok, err)
            }
        })
    }
}

func TestValidateBio(t *testing.T) {
    if _, err := ValidateBio(""); err != nil { t.Fatal("empty bio should be ok") }
    if _, err := ValidateBio("hi"); err != nil { t.Fatal("short bio should be ok") }
    big := make([]byte, 1001)
    for i := range big { big[i] = 'a' }
    if _, err := ValidateBio(string(big)); err == nil { t.Fatal("1001-char bio should error") }
}

func TestCheckPasswordComplexity(t *testing.T) {
    if err := CheckPasswordComplexity("short"); err == nil { t.Fatal("len<8 should error") }
    if err := CheckPasswordComplexity("abcdefgh"); err != nil { t.Fatalf("len=8 should pass, got %v", err) }
    long := make([]byte, 251)
    for i := range long { long[i] = 'a' }
    if err := CheckPasswordComplexity(string(long)); err == nil { t.Fatal("len>250 should error") }
}

func TestFilterUserFields(t *testing.T) {
    u := &models.User{
        ID: 7, Username: utils.Ptr("alice"), Password: "SECRET",
        WebPushSubscriptionConfig: utils.Ptr("{...}"), Role: "default", Bio: utils.Ptr("hi"),
    }
    out := FilterUserFields(u)
    if _, has := out["password"]; has { t.Fatal("password must be stripped") }
    if _, has := out["webPushSubscriptionConfig"]; has { t.Fatal("webPushSubscriptionConfig must be stripped") }
    if out["id"] != 7 || out["role"] != "default" { t.Fatalf("unexpected output: %+v", out) }
}
```

- [ ] **Step 2: Run tests, confirm failures**

```bash
cd /Users/ranwei/workspace/go_work/go-hermind/backend
go test ./internal/services -run 'TestValidateUsername|TestValidateBio|TestCheckPasswordComplexity|TestFilterUserFields' -v
```

Expect: undefined references for all four symbols.

- [ ] **Step 3: Implement**

Write `internal/services/user_validations.go` exactly as in the Pre-task section.

- [ ] **Step 4: Run tests, confirm pass**

```bash
go test ./internal/services -run 'TestValidateUsername|TestValidateBio|TestCheckPasswordComplexity|TestFilterUserFields' -v
```

All four pass. Also run `go build ./...` to make sure nothing else broke.

- [ ] **Step 5: Commit**

```
feat(system): add username/bio/password validators + FilterUserFields helper

Extracted from Node server/models/user.js validations + filterFields. Reused
by upcoming P4b system user-state routes.
```

---

## Task 2: AuthService.RotateCredentials + EnableMultiUser

Add two methods on `*services.AuthService` that own the credential-mutation logic. These touch `cfg` (in-memory) AND `system_settings` (DB). Tests stub a `*config.Config` and a `*services.SystemService` to assert both side effects.

**Files:**
- Modify: `backend/internal/services/auth_service.go`
- Create: `backend/internal/services/auth_service_test.go`

### RotateCredentials signature

```go
// RotateCredentials replaces AUTH_TOKEN (if usePassword) and always rotates JWT_SECRET.
// When usePassword=false, BOTH are cleared (matches Node's "disable password" branch).
// Side effects: cfg fields mutated; system_settings.{auth_token, jwt_secret} persisted.
func (s *AuthService) RotateCredentials(ctx context.Context, sysSvc *SystemService, usePassword bool, newPassword string) error
```

Behavior:
- If `usePassword == false`: set `cfg.AuthToken = ""`, `cfg.JWTSecret = ""`, persist both as empty strings (Node bypasses validation in this branch and clears `process.env`).
- If `usePassword == true`: validate `newPassword` via `CheckPasswordComplexity`; on fail return the validation error verbatim. On pass, set `cfg.AuthToken = newPassword` (raw, matching Node's storage of raw AUTH_TOKEN — bcrypt happens at compare-time inside `validateSingleUserToken`). Generate a new UUIDv4 → set `cfg.JWTSecret = newSecret`. Persist both via `sysSvc.SetSetting`.

### EnableMultiUser signature

```go
// EnableMultiUser creates the initial admin user, persists multi_user_mode=true,
// and rotates JWT_SECRET. Returns (createdUser, businessError, systemError).
// businessError is the user-facing message; systemError is non-nil only on DB / hash failures.
func (s *AuthService) EnableMultiUser(
    ctx context.Context,
    adminSvc *AdminService,
    sysSvc *SystemService,
    username, password string,
) (*models.User, string, error)
```

Behavior:
- If `s.cfg.MultiUserMode` already true → return `(nil, "Multi-user mode is already enabled.", nil)`.
- Validate `username` via `ValidateUsername` → on fail return businessError.
- Validate `password` via `CheckPasswordComplexity` → on fail return businessError.
- Call `adminSvc.CreateUser(ctx, CreateUserInput{Username: username, Password: password, Role: "admin"})` → if it returns businessError, propagate. If it returns systemError, propagate (rollback unnecessary — DB op was the only side effect; further side effects are below).
- Persist `multi_user_mode=true` via `sysSvc.SetSetting(ctx, "multi_user_mode", "true")`. On DB error, attempt to delete the just-created user and return systemError.
- If `s.cfg.JWTSecret == ""` → generate UUIDv4 → set `cfg.JWTSecret = newSecret` → persist `jwt_secret=newSecret`. (Matches Node's `updateENV({JWTSecret: process.env.JWT_SECRET || v4()}, true)`.)
- Set `s.cfg.MultiUserMode = true`.
- Return `(user, "", nil)`.

### POST /system/user → UpdateOwnProfile

Add a third method:

```go
// UpdateOwnProfile applies username / password / bio to the calling user.
// Returns (businessError, systemError). It does NOT use the RBAC helpers (self-edit).
func (s *AuthService) UpdateOwnProfile(
    ctx context.Context,
    user *models.User,
    newUsername, newPassword, newBio *string,
) (string, error)
```

Behavior:
- Build an updates map. Only include `username` if `newUsername != nil` AND `*newUsername != *user.Username`; validate via `ValidateUsername`.
- Only include `password` if `newPassword != nil && *newPassword != ""`; validate via `CheckPasswordComplexity`; bcrypt-hash.
- Only include `bio` if `newBio != nil && *newBio != ""` (matches Node `if (bio)` — empty bio is "no change" not "clear to empty"); validate via `ValidateBio`.
- If updates map is empty → return `("No updates provided", nil)`.
- Stamp `last_updated_at = time.Now()`.
- `db.Model(&User{}).Where("id=?", user.ID).Updates(updates)` — on unique-constraint error return `("A user with that username already exists", nil)`; on other DB error return `("", err)`.

- [ ] **Step 1: Write the failing tests**

```go
// File: backend/internal/services/auth_service_test.go
package services

import (
    "context"
    "testing"

    "github.com/odysseythink/hermind/backend/internal/config"
    "github.com/odysseythink/hermind/backend/internal/models"
    "github.com/odysseythink/hermind/backend/pkg/utils"
    "github.com/stretchr/testify/assert"
)

func newTestAuthEnv(t *testing.T) (*AuthService, *SystemService, *AdminService, *config.Config) {
    t.Helper()
    cfg := &config.Config{StorageDir: t.TempDir(), JWTSecret: "old-secret", AuthToken: "old-token", MultiUserMode: false}
    db, err := NewDB(cfg)
    assert.NoError(t, err)
    t.Cleanup(func() { if sqlDB, _ := db.DB(); sqlDB != nil { sqlDB.Close() } })
    assert.NoError(t, AutoMigrate(db))
    enc, _ := utils.NewEncryptionManager(cfg.StorageDir)
    return NewAuthService(db, cfg, enc), NewSystemService(db), NewAdminService(db), cfg
}

func TestRotateCredentials_DisablePassword(t *testing.T) {
    auth, sys, _, cfg := newTestAuthEnv(t)
    assert.NoError(t, auth.RotateCredentials(context.Background(), sys, false, ""))
    assert.Equal(t, "", cfg.AuthToken)
    assert.Equal(t, "", cfg.JWTSecret)
    v, _ := sys.GetSetting(context.Background(), "auth_token")
    assert.Equal(t, "", v)
}

func TestRotateCredentials_NewPassword_Persisted(t *testing.T) {
    auth, sys, _, cfg := newTestAuthEnv(t)
    assert.NoError(t, auth.RotateCredentials(context.Background(), sys, true, "newPassw0rd"))
    assert.Equal(t, "newPassw0rd", cfg.AuthToken)
    assert.NotEqual(t, "old-secret", cfg.JWTSecret)
    assert.NotEqual(t, "", cfg.JWTSecret)
    persisted, _ := sys.GetSetting(context.Background(), "auth_token")
    assert.Equal(t, "newPassw0rd", persisted)
}

func TestRotateCredentials_ShortPassword(t *testing.T) {
    auth, sys, _, _ := newTestAuthEnv(t)
    err := auth.RotateCredentials(context.Background(), sys, true, "short")
    assert.Error(t, err)
}

func TestEnableMultiUser_Success(t *testing.T) {
    auth, sys, admin, cfg := newTestAuthEnv(t)
    u, bizErr, sysErr := auth.EnableMultiUser(context.Background(), admin, sys, "newadmin", "newPassw0rd")
    assert.NoError(t, sysErr)
    assert.Equal(t, "", bizErr)
    assert.NotNil(t, u)
    assert.Equal(t, "admin", u.Role)
    assert.True(t, cfg.MultiUserMode)
    flag, _ := sys.GetSetting(context.Background(), "multi_user_mode")
    assert.Equal(t, "true", flag)
}

func TestEnableMultiUser_AlreadyOn(t *testing.T) {
    auth, sys, admin, cfg := newTestAuthEnv(t)
    cfg.MultiUserMode = true
    _, bizErr, sysErr := auth.EnableMultiUser(context.Background(), admin, sys, "x", "y")
    assert.NoError(t, sysErr)
    assert.Equal(t, "Multi-user mode is already enabled.", bizErr)
}

func TestEnableMultiUser_BadUsername(t *testing.T) {
    auth, sys, admin, _ := newTestAuthEnv(t)
    _, bizErr, _ := auth.EnableMultiUser(context.Background(), admin, sys, "A", "newPassw0rd")
    assert.NotEmpty(t, bizErr)
}

func TestUpdateOwnProfile_BioOnly(t *testing.T) {
    auth, _, _, cfg := newTestAuthEnv(t)
    hash, _ := utils.HashPassword("pw")
    u := &models.User{Username: utils.Ptr("alice"), Password: hash, Role: "default"}
    assert.NoError(t, auth.db.Create(u).Error)
    bizErr, sysErr := auth.UpdateOwnProfile(context.Background(), u, nil, nil, utils.Ptr("new bio"))
    assert.NoError(t, sysErr)
    assert.Equal(t, "", bizErr)
    var reloaded models.User
    auth.db.First(&reloaded, u.ID)
    assert.Equal(t, "new bio", *reloaded.Bio)
    _ = cfg
}

func TestUpdateOwnProfile_NoUpdates(t *testing.T) {
    auth, _, _, _ := newTestAuthEnv(t)
    hash, _ := utils.HashPassword("pw")
    u := &models.User{Username: utils.Ptr("alice"), Password: hash, Role: "default"}
    assert.NoError(t, auth.db.Create(u).Error)
    bizErr, _ := auth.UpdateOwnProfile(context.Background(), u, utils.Ptr("alice"), nil, nil)
    // Username unchanged (same as current) and no other field → empty updates
    assert.Equal(t, "No updates provided", bizErr)
}
```

- [ ] **Step 2: Run tests, confirm failures**

```bash
go test ./internal/services -run 'TestRotateCredentials|TestEnableMultiUser|TestUpdateOwnProfile' -v
```

Expect: undefined methods on `AuthService`.

- [ ] **Step 3: Implement**

In `auth_service.go`:

```go
// add at the top of file imports if not already present:
//   "github.com/google/uuid"

func (s *AuthService) RotateCredentials(ctx context.Context, sysSvc *SystemService, usePassword bool, newPassword string) error {
    if !usePassword {
        s.cfg.AuthToken = ""
        s.cfg.JWTSecret = ""
        if err := sysSvc.SetSetting(ctx, "auth_token", ""); err != nil { return err }
        if err := sysSvc.SetSetting(ctx, "jwt_secret", ""); err != nil { return err }
        return nil
    }
    if err := CheckPasswordComplexity(newPassword); err != nil {
        return err
    }
    newSecret := uuid.New().String()
    s.cfg.AuthToken = newPassword
    s.cfg.JWTSecret = newSecret
    if err := sysSvc.SetSetting(ctx, "auth_token", newPassword); err != nil { return err }
    if err := sysSvc.SetSetting(ctx, "jwt_secret", newSecret); err != nil { return err }
    return nil
}

func (s *AuthService) EnableMultiUser(
    ctx context.Context,
    adminSvc *AdminService,
    sysSvc *SystemService,
    username, password string,
) (*models.User, string, error) {
    if s.cfg.MultiUserMode {
        return nil, "Multi-user mode is already enabled.", nil
    }
    if _, err := ValidateUsername(username); err != nil {
        return nil, err.Error(), nil
    }
    if err := CheckPasswordComplexity(password); err != nil {
        return nil, err.Error(), nil
    }

    user, bizErr, sysErr := adminSvc.CreateUser(ctx, CreateUserInput{
        Username: username, Password: password, Role: "admin",
    })
    if sysErr != nil { return nil, "", sysErr }
    if bizErr != "" { return nil, bizErr, nil }

    if err := sysSvc.SetSetting(ctx, "multi_user_mode", "true"); err != nil {
        // best-effort rollback
        s.db.Delete(&models.User{}, user.ID)
        return nil, "", err
    }
    if s.cfg.JWTSecret == "" {
        newSecret := uuid.New().String()
        s.cfg.JWTSecret = newSecret
        _ = sysSvc.SetSetting(ctx, "jwt_secret", newSecret)
    }
    s.cfg.MultiUserMode = true
    return user, "", nil
}

func (s *AuthService) UpdateOwnProfile(
    ctx context.Context,
    user *models.User,
    newUsername, newPassword, newBio *string,
) (string, error) {
    if user == nil || user.ID == 0 {
        return "Invalid user ID", nil
    }
    updates := map[string]any{}
    if newUsername != nil {
        cur := ""
        if user.Username != nil { cur = *user.Username }
        if *newUsername != cur {
            validated, err := ValidateUsername(*newUsername)
            if err != nil { return err.Error(), nil }
            updates["username"] = validated
        }
    }
    if newPassword != nil && *newPassword != "" {
        if err := CheckPasswordComplexity(*newPassword); err != nil { return err.Error(), nil }
        hash, err := utils.HashPassword(*newPassword)
        if err != nil { return "", err }
        updates["password"] = hash
    }
    if newBio != nil && *newBio != "" {
        validated, err := ValidateBio(*newBio)
        if err != nil { return err.Error(), nil }
        updates["bio"] = validated
    }
    if len(updates) == 0 {
        return "No updates provided", nil
    }
    updates["last_updated_at"] = time.Now()
    res := s.db.WithContext(ctx).Model(&models.User{}).Where("id = ?", user.ID).Updates(updates)
    if res.Error != nil {
        lower := strings.ToLower(res.Error.Error())
        if strings.Contains(lower, "unique") || strings.Contains(lower, "duplicate") {
            return "A user with that username already exists", nil
        }
        return "", res.Error
    }
    return "", nil
}
```

You will need to import `strings`, `time`, and `github.com/google/uuid` at the top of `auth_service.go`. `uuid` is already a dep (used by RecoverAccount).

- [ ] **Step 4: Run tests, confirm pass**

```bash
go test ./internal/services -run 'TestRotateCredentials|TestEnableMultiUser|TestUpdateOwnProfile' -v
go build ./...
```

All AuthService tests pass; admin tests still pass.

- [ ] **Step 5: Commit**

```
feat(system): add AuthService.RotateCredentials/EnableMultiUser/UpdateOwnProfile

Credential mutation lives in AuthService. Dual-writes cfg in-memory + system_settings
table. Reused by upcoming P4b handlers.
```

---

## Task 3: GET /system/check-token

**Files:**
- Modify: `backend/internal/handlers/system.go`
- Create: `backend/tests/integration/system_user_test.go` (test setup + first test)

- [ ] **Step 1: Write the failing test**

```go
// File: backend/tests/integration/system_user_test.go
package integration

import (
    "context"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"
    "time"

    "github.com/gin-gonic/gin"
    "github.com/odysseythink/hermind/backend/internal/config"
    "github.com/odysseythink/hermind/backend/internal/handlers"
    "github.com/odysseythink/hermind/backend/internal/models"
    "github.com/odysseythink/hermind/backend/internal/services"
    "github.com/odysseythink/hermind/backend/pkg/utils"
    "github.com/stretchr/testify/assert"
    "gorm.io/gorm"
)

func setupSystemUserRouter(t *testing.T) (*gin.Engine, *gorm.DB, *services.AuthService, *config.Config) {
    gin.SetMode(gin.TestMode)
    cfg := &config.Config{StorageDir: t.TempDir(), JWTSecret: "test-secret", AuthToken: "test-auth-token", MultiUserMode: false}
    db, err := services.NewDB(cfg)
    assert.NoError(t, err)
    t.Cleanup(func() { if sqlDB, _ := db.DB(); sqlDB != nil { sqlDB.Close() } })
    assert.NoError(t, services.AutoMigrate(db))
    enc, _ := utils.NewEncryptionManager(cfg.StorageDir)
    authSvc := services.NewAuthService(db, cfg, enc)
    sysSvc := services.NewSystemService(db)
    apiKeySvc := services.NewAPIKeyService(db)
    adminSvc := services.NewAdminService(db)
    r := gin.New()
    handlers.RegisterSystemRoutes(r.Group("/api"), sysSvc, apiKeySvc, cfg, authSvc, adminSvc)
    return r, db, authSvc, cfg
}

func seedMultiUserAdmin(t *testing.T, db *gorm.DB, cfg *config.Config, role string, suspended int) (*models.User, string) {
    t.Helper()
    hash, _ := utils.HashPassword("pw")
    u := &models.User{Username: utils.Ptr("alice"), Password: hash, Role: role, Suspended: suspended, Bio: utils.Ptr("")}
    assert.NoError(t, db.Create(u).Error)
    tok, err := utils.GenerateJWT(cfg.JWTSecret, map[string]any{"userId": u.ID}, time.Hour)
    assert.NoError(t, err)
    return u, tok
}

func singleUserBearer(t *testing.T, authSvc *services.AuthService) string {
    t.Helper()
    tok, err := authSvc.CreateSingleUserToken(context.Background())
    assert.NoError(t, err)
    return tok
}

func TestSystem_CheckToken_SingleUser_OK(t *testing.T) {
    r, _, authSvc, _ := setupSystemUserRouter(t)
    tok := singleUserBearer(t, authSvc)
    req := httptest.NewRequest(http.MethodGet, "/api/system/check-token", nil)
    req.Header.Set("Authorization", "Bearer "+tok)
    w := httptest.NewRecorder()
    r.ServeHTTP(w, req)
    assert.Equal(t, http.StatusOK, w.Code)
    assert.Empty(t, w.Body.String())
}

func TestSystem_CheckToken_MultiUser_Suspended_403(t *testing.T) {
    r, db, _, cfg := setupSystemUserRouter(t)
    cfg.MultiUserMode = true
    _, tok := seedMultiUserAdmin(t, db, cfg, "default", 1)
    req := httptest.NewRequest(http.MethodGet, "/api/system/check-token", nil)
    req.Header.Set("Authorization", "Bearer "+tok)
    w := httptest.NewRecorder()
    r.ServeHTTP(w, req)
    assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestSystem_CheckToken_MultiUser_OK(t *testing.T) {
    r, db, _, cfg := setupSystemUserRouter(t)
    cfg.MultiUserMode = true
    _, tok := seedMultiUserAdmin(t, db, cfg, "default", 0)
    req := httptest.NewRequest(http.MethodGet, "/api/system/check-token", nil)
    req.Header.Set("Authorization", "Bearer "+tok)
    w := httptest.NewRecorder()
    r.ServeHTTP(w, req)
    assert.Equal(t, http.StatusOK, w.Code)
}

// json import will be used by later tests
var _ = json.Marshal
```

- [ ] **Step 2: Run tests, confirm failures**

```bash
cd /Users/ranwei/workspace/go_work/go-hermind/backend
go test ./tests/integration -run TestSystem_CheckToken -v
```

Expect: compilation error (`RegisterSystemRoutes` signature mismatch — needs `adminSvc`) or 404 on the route. Both are valid "red" states for the next step.

- [ ] **Step 3: Implement**

In `internal/handlers/system.go`:

1. Update `RegisterSystemRoutes` signature to accept `adminSvc *services.AdminService` and store it in the handler.

```go
type SystemHandler struct {
    sysSvc    *services.SystemService
    apiKeySvc *services.APIKeyService
    adminSvc  *services.AdminService
    authSvc   *services.AuthService
    cfg       *config.Config
}

func NewSystemHandler(sysSvc *services.SystemService, apiKeySvc *services.APIKeyService, adminSvc *services.AdminService, authSvc *services.AuthService, cfg *config.Config) *SystemHandler {
    return &SystemHandler{sysSvc: sysSvc, apiKeySvc: apiKeySvc, adminSvc: adminSvc, authSvc: authSvc, cfg: cfg}
}

func RegisterSystemRoutes(r *gin.RouterGroup, sysSvc *services.SystemService, apiKeySvc *services.APIKeyService, cfg *config.Config, authSvc *services.AuthService, adminSvc *services.AdminService) {
    h := NewSystemHandler(sysSvc, apiKeySvc, adminSvc, authSvc, cfg)
    // ... keep all existing routes ...
    r.GET("/system/check-token", middleware.ValidatedRequest(authSvc), h.CheckToken)
}
```

2. Add the handler:

```go
func (h *SystemHandler) CheckToken(c *gin.Context) {
    if !h.cfg.MultiUserMode {
        c.Status(http.StatusOK)
        return
    }
    userVal, _ := c.Get("user")
    user, _ := userVal.(*models.User)
    if user == nil || user.Suspended != 0 {
        c.Status(http.StatusForbidden)
        return
    }
    c.Status(http.StatusOK)
}
```

3. Update `cmd/server/main.go:134`:

```go
handlers.RegisterSystemRoutes(api, sysSvc, apiKeySvc, cfg, authSvc, adminSvc)
```

4. Update the other two callers of `RegisterSystemRoutes`:
   - `backend/tests/integration/onboarding_test.go:41` — pass `adminSvc` (construct via `services.NewAdminService(db)`).
   - `backend/tests/integration/system_test.go:33` — same.

Confirm they still compile and their existing assertions still pass.

- [ ] **Step 4: Run tests, confirm pass**

```bash
go test ./tests/integration -run TestSystem_CheckToken -v
go build ./...
```

- [ ] **Step 5: Commit**

```
feat(system): GET /system/check-token (multi-user suspension gate)
```

---

## Task 4: GET /system/refresh-user

**Files:**
- Modify: `backend/internal/handlers/system.go`
- Modify: `backend/tests/integration/system_user_test.go`

- [ ] **Step 1: Write the failing test**

Append:

```go
func TestSystem_RefreshUser_SingleUser(t *testing.T) {
    r, _, authSvc, _ := setupSystemUserRouter(t)
    tok := singleUserBearer(t, authSvc)
    req := httptest.NewRequest(http.MethodGet, "/api/system/refresh-user", nil)
    req.Header.Set("Authorization", "Bearer "+tok)
    w := httptest.NewRecorder()
    r.ServeHTTP(w, req)
    assert.Equal(t, http.StatusOK, w.Code)
    var body map[string]any
    assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
    assert.Equal(t, true, body["success"])
    assert.Nil(t, body["user"])
    assert.Nil(t, body["message"])
}

func TestSystem_RefreshUser_MultiUser_OK(t *testing.T) {
    r, db, _, cfg := setupSystemUserRouter(t)
    cfg.MultiUserMode = true
    _, tok := seedMultiUserAdmin(t, db, cfg, "default", 0)
    req := httptest.NewRequest(http.MethodGet, "/api/system/refresh-user", nil)
    req.Header.Set("Authorization", "Bearer "+tok)
    w := httptest.NewRecorder()
    r.ServeHTTP(w, req)
    assert.Equal(t, http.StatusOK, w.Code)
    var body map[string]any
    assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
    assert.Equal(t, true, body["success"])
    user, ok := body["user"].(map[string]any)
    assert.True(t, ok)
    assert.Equal(t, "alice", user["username"])
    _, hasPw := user["password"]
    assert.False(t, hasPw, "password must be stripped")
}

func TestSystem_RefreshUser_MultiUser_Suspended(t *testing.T) {
    r, db, _, cfg := setupSystemUserRouter(t)
    cfg.MultiUserMode = true
    _, tok := seedMultiUserAdmin(t, db, cfg, "default", 1)
    req := httptest.NewRequest(http.MethodGet, "/api/system/refresh-user", nil)
    req.Header.Set("Authorization", "Bearer "+tok)
    w := httptest.NewRecorder()
    r.ServeHTTP(w, req)
    // suspended users still pass ValidatedRequest (which doesn't check suspended).
    // refresh-user is where the suspended check lives.
    assert.Equal(t, http.StatusOK, w.Code)
    var body map[string]any
    assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
    assert.Equal(t, false, body["success"])
    assert.Equal(t, "User is suspended.", body["message"])
}
```

- [ ] **Step 2: Run tests, confirm failures**

```bash
go test ./tests/integration -run TestSystem_RefreshUser -v
```

- [ ] **Step 3: Implement**

```go
func (h *SystemHandler) RefreshUser(c *gin.Context) {
    if !h.cfg.MultiUserMode {
        c.JSON(http.StatusOK, gin.H{"success": true, "user": nil, "message": nil})
        return
    }
    userVal, _ := c.Get("user")
    user, _ := userVal.(*models.User)
    if user == nil {
        c.JSON(http.StatusOK, gin.H{"success": false, "user": nil, "message": "Session expired or invalid."})
        return
    }
    if user.Suspended != 0 {
        c.JSON(http.StatusOK, gin.H{"success": false, "user": nil, "message": "User is suspended."})
        return
    }
    c.JSON(http.StatusOK, gin.H{"success": true, "user": services.FilterUserFields(user), "message": nil})
}
```

Register in `RegisterSystemRoutes`:

```go
r.GET("/system/refresh-user", middleware.ValidatedRequest(authSvc), h.RefreshUser)
```

- [ ] **Step 4: Run tests, confirm pass**

```bash
go test ./tests/integration -run TestSystem_RefreshUser -v
```

- [ ] **Step 5: Commit**

```
feat(system): GET /system/refresh-user
```

---

## Task 5: POST /system/update-password (replace stub)

**Files:**
- Modify: `backend/internal/handlers/system.go` (add handler)
- Modify: `backend/internal/handlers/auth.go` (remove `UpdatePassword` method + route line)
- Modify: `backend/tests/integration/system_user_test.go`

- [ ] **Step 1: Write the failing test**

```go
func TestSystem_UpdatePassword_MultiUserMode_Rejects(t *testing.T) {
    r, db, _, cfg := setupSystemUserRouter(t)
    cfg.MultiUserMode = true
    _, tok := seedMultiUserAdmin(t, db, cfg, "admin", 0)
    body := []byte(`{"usePassword": true, "newPassword": "newPassw0rd"}`)
    req := httptest.NewRequest(http.MethodPost, "/api/system/update-password", bytes.NewReader(body))
    req.Header.Set("Authorization", "Bearer "+tok)
    req.Header.Set("Content-Type", "application/json")
    w := httptest.NewRecorder()
    r.ServeHTTP(w, req)
    assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestSystem_UpdatePassword_SingleUser_Sets(t *testing.T) {
    r, _, authSvc, cfg := setupSystemUserRouter(t)
    tok := singleUserBearer(t, authSvc)
    body := []byte(`{"usePassword": true, "newPassword": "newPassw0rd"}`)
    req := httptest.NewRequest(http.MethodPost, "/api/system/update-password", bytes.NewReader(body))
    req.Header.Set("Authorization", "Bearer "+tok)
    req.Header.Set("Content-Type", "application/json")
    w := httptest.NewRecorder()
    r.ServeHTTP(w, req)
    assert.Equal(t, http.StatusOK, w.Code)
    var resp map[string]any
    assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
    assert.Equal(t, true, resp["success"])
    assert.Equal(t, "newPassw0rd", cfg.AuthToken)
    assert.NotEqual(t, "test-secret", cfg.JWTSecret)
}

func TestSystem_UpdatePassword_Disable(t *testing.T) {
    r, _, authSvc, cfg := setupSystemUserRouter(t)
    tok := singleUserBearer(t, authSvc)
    body := []byte(`{"usePassword": false}`)
    req := httptest.NewRequest(http.MethodPost, "/api/system/update-password", bytes.NewReader(body))
    req.Header.Set("Authorization", "Bearer "+tok)
    req.Header.Set("Content-Type", "application/json")
    w := httptest.NewRecorder()
    r.ServeHTTP(w, req)
    assert.Equal(t, http.StatusOK, w.Code)
    assert.Equal(t, "", cfg.AuthToken)
    assert.Equal(t, "", cfg.JWTSecret)
}

func TestSystem_UpdatePassword_ShortPassword(t *testing.T) {
    r, _, authSvc, _ := setupSystemUserRouter(t)
    tok := singleUserBearer(t, authSvc)
    body := []byte(`{"usePassword": true, "newPassword": "short"}`)
    req := httptest.NewRequest(http.MethodPost, "/api/system/update-password", bytes.NewReader(body))
    req.Header.Set("Authorization", "Bearer "+tok)
    req.Header.Set("Content-Type", "application/json")
    w := httptest.NewRecorder()
    r.ServeHTTP(w, req)
    assert.Equal(t, http.StatusOK, w.Code)
    var resp map[string]any
    assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
    assert.Equal(t, false, resp["success"])
    assert.NotEmpty(t, resp["error"])
}
```

`bytes` import will be needed; add to the imports block at the top of the file.

- [ ] **Step 2: Run tests, confirm failures**

```bash
go test ./tests/integration -run TestSystem_UpdatePassword -v
```

- [ ] **Step 3: Implement**

In `system.go`:

```go
func (h *SystemHandler) UpdatePassword(c *gin.Context) {
    if h.cfg.MultiUserMode {
        c.Status(http.StatusUnauthorized)
        return
    }
    var req dto.UpdatePasswordRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusOK, gin.H{"success": false, "error": err.Error()})
        return
    }
    if err := h.authSvc.RotateCredentials(c.Request.Context(), h.sysSvc, req.UsePassword, req.NewPassword); err != nil {
        c.JSON(http.StatusOK, gin.H{"success": false, "error": err.Error()})
        return
    }
    c.JSON(http.StatusOK, gin.H{"success": true, "error": nil})
}
```

Register in `RegisterSystemRoutes` (mid-block, with other authed POSTs):

```go
r.POST("/system/update-password", middleware.ValidatedRequest(authSvc), h.UpdatePassword)
```

In `auth.go`:
- Delete the `UpdatePassword` method (lines ~88-95).
- Delete the route line at ~129: `r.POST("/system/update-password", ...)`.
- The `UpdatePasswordRequest` DTO stays in `dto/auth.go` (still imported by system.go).

- [ ] **Step 4: Run tests, confirm pass**

```bash
go test ./tests/integration -run TestSystem_UpdatePassword -v
go test ./tests/integration -run TestAuth -v   # confirm auth tests still green
go build ./...
```

- [ ] **Step 5: Commit**

```
feat(system): POST /system/update-password (real impl, move out of AuthHandler)

Replaces the always-true stub at AuthHandler.UpdatePassword. Persists AUTH_TOKEN
and JWT_SECRET via AuthService.RotateCredentials. Rejects in multi-user mode
matching Node behavior. Existing tokens become invalid (JWT_SECRET rotates).
```

---

## Task 6: POST /system/enable-multi-user

**Files:**
- Modify: `backend/internal/handlers/system.go`
- Modify: `backend/tests/integration/system_user_test.go`

- [ ] **Step 1: Write the failing test**

```go
func TestSystem_EnableMultiUser_Success(t *testing.T) {
    r, _, authSvc, cfg := setupSystemUserRouter(t)
    tok := singleUserBearer(t, authSvc)
    body := []byte(`{"username":"newadmin","password":"newPassw0rd"}`)
    req := httptest.NewRequest(http.MethodPost, "/api/system/enable-multi-user", bytes.NewReader(body))
    req.Header.Set("Authorization", "Bearer "+tok)
    req.Header.Set("Content-Type", "application/json")
    w := httptest.NewRecorder()
    r.ServeHTTP(w, req)
    assert.Equal(t, http.StatusOK, w.Code)
    var resp map[string]any
    assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
    assert.Equal(t, true, resp["success"])
    assert.True(t, cfg.MultiUserMode)
}

func TestSystem_EnableMultiUser_AlreadyOn(t *testing.T) {
    r, db, _, cfg := setupSystemUserRouter(t)
    cfg.MultiUserMode = true
    _, tok := seedMultiUserAdmin(t, db, cfg, "admin", 0)
    body := []byte(`{"username":"newadmin","password":"newPassw0rd"}`)
    req := httptest.NewRequest(http.MethodPost, "/api/system/enable-multi-user", bytes.NewReader(body))
    req.Header.Set("Authorization", "Bearer "+tok)
    req.Header.Set("Content-Type", "application/json")
    w := httptest.NewRecorder()
    r.ServeHTTP(w, req)
    assert.Equal(t, http.StatusOK, w.Code)
    var resp map[string]any
    assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
    assert.Equal(t, false, resp["success"])
    assert.Equal(t, "Multi-user mode is already enabled.", resp["error"])
}

func TestSystem_EnableMultiUser_BadInput(t *testing.T) {
    r, _, authSvc, _ := setupSystemUserRouter(t)
    tok := singleUserBearer(t, authSvc)
    body := []byte(`{"username":"A","password":"newPassw0rd"}`)
    req := httptest.NewRequest(http.MethodPost, "/api/system/enable-multi-user", bytes.NewReader(body))
    req.Header.Set("Authorization", "Bearer "+tok)
    req.Header.Set("Content-Type", "application/json")
    w := httptest.NewRecorder()
    r.ServeHTTP(w, req)
    // Node returns 400 in the create-fail branch; matching that.
    assert.Equal(t, http.StatusBadRequest, w.Code)
}
```

- [ ] **Step 2: Run tests, confirm failures**

```bash
go test ./tests/integration -run TestSystem_EnableMultiUser -v
```

- [ ] **Step 3: Implement**

```go
func (h *SystemHandler) EnableMultiUser(c *gin.Context) {
    var req struct {
        Username string `json:"username"`
        Password string `json:"password"`
    }
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
        return
    }
    user, bizErr, sysErr := h.authSvc.EnableMultiUser(c.Request.Context(), h.adminSvc, h.sysSvc, req.Username, req.Password)
    if sysErr != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": sysErr.Error()})
        return
    }
    if bizErr != "" {
        // Node returns 200 for "already enabled" branch but 400 for create-failed branch.
        // Distinguish by checking the cfg state: if MultiUserMode already on, send 200.
        if bizErr == "Multi-user mode is already enabled." {
            c.JSON(http.StatusOK, gin.H{"success": false, "error": bizErr})
            return
        }
        c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": bizErr})
        return
    }
    _ = user // future: include user.id in response if needed
    c.JSON(http.StatusOK, gin.H{"success": true, "error": nil})
}
```

Register:

```go
r.POST("/system/enable-multi-user", middleware.ValidatedRequest(authSvc), h.EnableMultiUser)
```

- [ ] **Step 4: Run tests, confirm pass**

```bash
go test ./tests/integration -run TestSystem_EnableMultiUser -v
go build ./...
```

- [ ] **Step 5: Commit**

```
feat(system): POST /system/enable-multi-user (creates initial admin, rotates JWT)

Persists multi_user_mode=true in system_settings AND flips cfg.MultiUserMode in
memory. Skips Telemetry/EventLogs/BrowserExtension/AgentSkill side-effects until
those subsystems land (P4e/P10).
```

---

## Task 7: POST /system/user (self-update profile)

**Files:**
- Modify: `backend/internal/handlers/system.go`
- Modify: `backend/tests/integration/system_user_test.go`

- [ ] **Step 1: Write the failing test**

```go
func TestSystem_UpdateOwnUser_Bio(t *testing.T) {
    r, db, _, cfg := setupSystemUserRouter(t)
    cfg.MultiUserMode = true
    _, tok := seedMultiUserAdmin(t, db, cfg, "default", 0)
    body := []byte(`{"username":"alice","bio":"hi there"}`)
    req := httptest.NewRequest(http.MethodPost, "/api/system/user", bytes.NewReader(body))
    req.Header.Set("Authorization", "Bearer "+tok)
    req.Header.Set("Content-Type", "application/json")
    w := httptest.NewRecorder()
    r.ServeHTTP(w, req)
    assert.Equal(t, http.StatusOK, w.Code)
    var resp map[string]any
    assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
    assert.Equal(t, true, resp["success"])
    var reloaded models.User
    db.Where("username = ?", "alice").First(&reloaded)
    assert.Equal(t, "hi there", *reloaded.Bio)
}

func TestSystem_UpdateOwnUser_UsernameChange(t *testing.T) {
    r, db, _, cfg := setupSystemUserRouter(t)
    cfg.MultiUserMode = true
    _, tok := seedMultiUserAdmin(t, db, cfg, "default", 0)
    body := []byte(`{"username":"alice2"}`)
    req := httptest.NewRequest(http.MethodPost, "/api/system/user", bytes.NewReader(body))
    req.Header.Set("Authorization", "Bearer "+tok)
    req.Header.Set("Content-Type", "application/json")
    w := httptest.NewRecorder()
    r.ServeHTTP(w, req)
    assert.Equal(t, http.StatusOK, w.Code)
    var reloaded models.User
    db.Where("username = ?", "alice2").First(&reloaded)
    assert.NotZero(t, reloaded.ID)
}

func TestSystem_UpdateOwnUser_NoUpdates(t *testing.T) {
    r, db, _, cfg := setupSystemUserRouter(t)
    cfg.MultiUserMode = true
    _, tok := seedMultiUserAdmin(t, db, cfg, "default", 0)
    body := []byte(`{"username":"alice"}`) // same as current — no diff
    req := httptest.NewRequest(http.MethodPost, "/api/system/user", bytes.NewReader(body))
    req.Header.Set("Authorization", "Bearer "+tok)
    req.Header.Set("Content-Type", "application/json")
    w := httptest.NewRecorder()
    r.ServeHTTP(w, req)
    assert.Equal(t, http.StatusBadRequest, w.Code)
    var resp map[string]any
    assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
    assert.Equal(t, "No updates provided", resp["error"])
}

func TestSystem_UpdateOwnUser_InvalidUsername(t *testing.T) {
    r, db, _, cfg := setupSystemUserRouter(t)
    cfg.MultiUserMode = true
    _, tok := seedMultiUserAdmin(t, db, cfg, "default", 0)
    body := []byte(`{"username":"A"}`)
    req := httptest.NewRequest(http.MethodPost, "/api/system/user", bytes.NewReader(body))
    req.Header.Set("Authorization", "Bearer "+tok)
    req.Header.Set("Content-Type", "application/json")
    w := httptest.NewRecorder()
    r.ServeHTTP(w, req)
    assert.Equal(t, http.StatusOK, w.Code)
    var resp map[string]any
    assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
    assert.Equal(t, false, resp["success"])
    assert.NotEmpty(t, resp["error"])
}
```

- [ ] **Step 2: Run tests, confirm failures**

```bash
go test ./tests/integration -run TestSystem_UpdateOwnUser -v
```

- [ ] **Step 3: Implement**

```go
func (h *SystemHandler) UpdateOwnUser(c *gin.Context) {
    userVal, _ := c.Get("user")
    user, _ := userVal.(*models.User)
    if user == nil || user.ID == 0 {
        c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid user ID"})
        return
    }
    var req struct {
        Username *string `json:"username"`
        Password *string `json:"password"`
        Bio      *string `json:"bio"`
    }
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
        return
    }
    bizErr, sysErr := h.authSvc.UpdateOwnProfile(c.Request.Context(), user, req.Username, req.Password, req.Bio)
    if sysErr != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": sysErr.Error()})
        return
    }
    if bizErr != "" {
        // Match Node: "No updates provided" → 400; other business errors → 200 with error.
        if bizErr == "No updates provided" {
            c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": bizErr})
            return
        }
        c.JSON(http.StatusOK, gin.H{"success": false, "error": bizErr})
        return
    }
    c.JSON(http.StatusOK, gin.H{"success": true, "error": nil})
}
```

Register:

```go
r.POST("/system/user", middleware.ValidatedRequest(authSvc), h.UpdateOwnUser)
```

- [ ] **Step 4: Run tests, confirm pass**

```bash
go test ./tests/integration -run TestSystem_UpdateOwnUser -v
go build ./...
```

- [ ] **Step 5: Commit**

```
feat(system): POST /system/user (self-update username/password/bio)
```

---

## Task 8: Full P4b sanity sweep

- [ ] **Step 1: Run all integration + service tests**

```bash
cd /Users/ranwei/workspace/go_work/go-hermind/backend
go vet ./...
go build ./...
go test ./internal/services -v
go test ./tests/integration -v
```

All must pass. Pay particular attention to:
- `auth_test.go` / `auth_extended_test.go` — confirm the removed `AuthHandler.UpdatePassword` did not break them (they should not rely on `/system/update-password` being a no-op success).
- `admin_test.go` — P4a tests must still pass; the `setupAdminRouter` signature stays the same.
- `onboarding_test.go` / `system_test.go` — confirm `RegisterSystemRoutes` signature change is propagated.

- [ ] **Step 2: Wire-level diff check vs Node**

For each of the 5 routes, hit the Go server with the same payload as the Node server returns and confirm parity:
- `check-token` — empty body, status code matches.
- `refresh-user` — `success/user/message` keys present; password and `webPushSubscriptionConfig` absent.
- `update-password` — `success/error` keys; status 401 in multi-user.
- `enable-multi-user` — `success/error` keys; status 200 on already-on, 400 on bad input.
- `POST /system/user` — `success/error`; status 400 on no updates.

This is an inspect-only step. No code change.

- [ ] **Step 3: Commit (only if Step 1 surfaced fixes)**

```
chore(system): P4b sanity sweep — all tests green
```

If no follow-ups, skip the commit and move to Known Gaps.

---

## Known gaps after P4b (track but DO NOT implement here)

| # | Gap | Owner phase |
|---|-----|---|
| 1 | `EventLogs.logEvent("multi_user_mode_enabled", ...)` | P4e Misc |
| 2 | `Telemetry.sendTelemetry("enabled_multi_user_mode", ...)` | P4e Misc |
| 3 | `BrowserExtensionApiKey.migrateApiKeysToMultiUser(user.id)` on enable-multi-user | P4e Misc |
| 4 | `AgentSkillWhitelist.clearSingleUserWhitelist()` on enable-multi-user | P10 |
| 5 | Startup overlay: re-hydrate `cfg.AuthToken/JWTSecret/MultiUserMode` from `system_settings` on boot (otherwise env wins after restart) | P4e Misc (or a dedicated infra ticket) |
| 6 | Env-driven password complexity overrides (PASSWORDMINCHAR etc.) | Defer; track in v2 design |
| 7 | `user.seen_recovery_codes` auto-flip after first password rotation in single-user mode (Node does this) — currently no-op in Go because single-user has no recovery codes | Defer |

These gaps are not test failures — handlers behave correctly on the happy path. Document in commit messages for traceability.

---

## Acceptance criteria

- All 5 new routes return responses matching Node shape (success keys, error keys, status codes).
- `go vet ./... && go build ./... && go test ./...` all green.
- No P4a admin tests regress.
- `cfg.MultiUserMode`, `cfg.AuthToken`, `cfg.JWTSecret` mutate in-memory after their respective routes succeed.
- `system_settings` rows exist for `multi_user_mode`, `auth_token`, `jwt_secret` after the relevant calls.
- The 5 known gaps listed above are documented in this plan but NOT implemented.
