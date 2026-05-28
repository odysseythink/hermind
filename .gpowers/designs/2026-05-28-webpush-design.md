# WebPush — Design

**Date**: 2026-05-28
**Status**: Draft
**Authors**: ranwei + Claude
**Companion docs**:
- `2026-05-28-scheduled-jobs-design.md` (upstream event producer)
- `2026-05-28-phase2-quick-wins-design.md`
- `2026-05-28-memories-system-design.md`

## 1. Purpose

Deliver browser push notifications (VAPID web-push) to users. Initial consumer: `scheduled_job_completed` / `scheduled_job_failed` events from Design B. Architecture is consumer-agnostic so future events (chat @-mention, document sync done, MCP error) can hook in.

## 2. Scope

### In
- VAPID keypair bootstrap stored in `SystemSetting` (encrypted)
- Per-user subscription stored in `users.web_push_subscription_config` (TEXT, JSON; mirrors anything-llm)
- 2 endpoints: `POST /web-push/subscribe`, `GET /web-push/pubkey`
- `WebPushService.Send(ctx, userID, payload)` consumer-facing API
- Event-log listener: subscribe to `scheduled_job_completed` / `_failed` and dispatch
- Use [`github.com/SherClockHolmes/webpush-go`](https://github.com/SherClockHolmes/webpush-go)

### Out
- Multi-device per user (single subscription column; anything-llm parity)
- Background push for non-scheduled-job events (extend later)
- Native mobile push (APNs/FCM)
- Web-push payload encryption beyond what `webpush-go` provides by default
- Frontend service worker (frontend repo concern)

## 3. Anything-LLM source comparison

- `server/utils/PushNotifications/index.js` (228 LoC): singleton `PushNotifications` with VAPID keys at `<storage>/push-notifications/vapid-keys.json`, primary-user subscription at `primary-subscription.json` (single-user mode) or DB column (multi-user). In-memory `Map<userId, subscription>` for fast send.
- `server/endpoints/webPush.js` (27 LoC): just two routes.
- Send sites: `server/jobs/helpers/scheduled-job-helper.js → sendWebPushNotification(job, runId, text)` (the only consumer in the codebase).

### Backend current state

- `users.web_push_subscription_config` **already exists** at `models/user.go:17` (added in earlier migration).
- No VAPID, no endpoints, no service, no EventLog pub/sub.
- Gap analysis §16 marks WebPush as "DB字段存在，无实现" — the column exists but no backend logic consumes it.

## 4. Design

### 4.1 Schema

`models/user.go` already has the required column (line 17); no schema change needed:
```go
WebPushSubscriptionConfig *string `gorm:"type:text" json:"-"`  // encrypted JSON
```

VAPID keys live in two `SystemSetting` rows, both encrypted via the existing `EncryptionManager`:
- `webpush_vapid_public_key`
- `webpush_vapid_private_key`

(Public key is technically not secret, but storing it via the same path keeps the bootstrap simple.)

### 4.2 Bootstrap

```go
func (s *WebPushService) Init(ctx context.Context) error {
    pub, _ := s.settings.GetEncrypted(ctx, "webpush_vapid_public_key")
    priv, _ := s.settings.GetEncrypted(ctx, "webpush_vapid_private_key")
    if pub == "" || priv == "" {
        priv, pub, err := webpush.GenerateVAPIDKeys()
        if err != nil { return err }
        s.settings.SetEncrypted(ctx, "webpush_vapid_public_key", pub)
        s.settings.SetEncrypted(ctx, "webpush_vapid_private_key", priv)
    }
    s.vapidPub = pub
    s.vapidPriv = priv
    return s.loadSubscriptions(ctx)
}

func (s *WebPushService) loadSubscriptions(ctx context.Context) error {
    var users []models.User
    s.db.Where("web_push_subscription_config IS NOT NULL").Find(&users)
    for _, u := range users {
        plain, err := s.crypto.Decrypt(*u.WebPushSubscriptionConfig)
        if err != nil { continue }
        var sub webpush.Subscription
        if json.Unmarshal([]byte(plain), &sub) == nil {
            s.subs.Store(u.ID, sub)
        }
    }
    return nil
}
```

The in-memory `sync.Map` accelerates send hot path; DB stays the source of truth.

### 4.3 Subscribe / pubkey endpoints

```go
// POST /web-push/subscribe
// body: webpush.Subscription JSON (endpoint, keys.p256dh, keys.auth)
// header: standard auth → resolves user
func (h *WebPushHandler) Subscribe(c *gin.Context) {
    user := middleware.UserFromContext(c)
    var sub webpush.Subscription
    if err := c.ShouldBindJSON(&sub); err != nil { c.JSON(400, gin.H{"error": err.Error()}); return }

    encrypted, _ := h.crypto.Encrypt(mustMarshal(sub))
    h.userSvc.UpdateField(c, user.ID, "web_push_subscription_config", &encrypted)
    h.svc.subs.Store(user.ID, sub)
    c.JSON(201, gin.H{})
}

// GET /web-push/pubkey
func (h *WebPushHandler) PubKey(c *gin.Context) {
    c.JSON(200, gin.H{"publicKey": h.svc.vapidPub})
}
```

### 4.4 Send API

```go
type Payload struct {
    Title   string      `json:"title"`
    Body    string      `json:"body"`
    Data    any         `json:"data,omitempty"`    // {onClickUrl: "..."}
    Actions []Action    `json:"actions,omitempty"` // [{action, title}]
    Image   string      `json:"image,omitempty"`
}

func (s *WebPushService) Send(ctx context.Context, userID int, p Payload) error {
    v, ok := s.subs.Load(userID)
    if !ok { return ErrNoSubscription }
    body, _ := json.Marshal(p)
    resp, err := webpush.SendNotification(body, v.(*webpush.Subscription), &webpush.Options{
        Subscriber:      s.cfg.MailTo,
        VAPIDPublicKey:  s.vapidPub,
        VAPIDPrivateKey: s.vapidPriv,
        TTL:             60,
    })
    if err != nil { return err }
    defer resp.Body.Close()
    // 410 GONE → subscription expired; remove
    if resp.StatusCode == http.StatusGone || resp.StatusCode == http.StatusNotFound {
        s.userSvc.UpdateField(ctx, userID, "web_push_subscription_config", nil)
        s.subs.Delete(userID)
    }
    return nil
}
```

### 4.5 Event-log consumer (the scheduled-job hook)

Two integration options — pick (B) for Phase 2 because it keeps WebPush independent of the scheduler package:

**(A) Direct call from `Scheduler.runOnce` after terminal-state write.** Tight coupling, hard to reuse.

**(B) `EventLogService` adds a `Subscribe(type, handler)` registry.** When the scheduler emits `eventLogSvc.LogEvent(ctx, "scheduled_job_completed", payload, nil)`, registered handlers run in their own goroutines after the DB row is written. WebPush registers a handler that calls `Send`.

```go
func (s *WebPushService) Boot(eventBus *eventlog.Bus) {
    eventBus.Subscribe("scheduled_job_completed", s.onJobCompleted)
    eventBus.Subscribe("scheduled_job_failed", s.onJobFailed)
}

func (s *WebPushService) onJobCompleted(ctx context.Context, e eventlog.Event) {
    var p struct{ JobID, RunID int; JobName, ResultText string; UserID *int }
    json.Unmarshal(e.Metadata, &p)
    if p.UserID == nil { return } // jobs aren't owned by users in Phase 2 — skip
    s.Send(ctx, *p.UserID, Payload{
        Title: "Scheduled job finished",
        Body:  truncate(p.ResultText, 200),
        Data:  map[string]any{"onClickUrl": fmt.Sprintf("/workspace/scheduled-jobs?run=%d", p.RunID)},
    })
}
```

> Implementation note: backend currently has `services/event_log_service.go` with `LogEvent` but no pub/sub. PR1 of this design adds the `Subscribe/Publish` extension to `EventLogService`.

## 5. Configuration

| Env | Default | Purpose |
|---|---|---|
| `WEBPUSH_MAIL_TO` | `mailto:webpush@hermind.local` | required by VAPID per RFC 8292 §2 |
| `WEBPUSH_TTL_SECONDS` | `60` | message TTL passed to push service |

## 6. PR breakdown

**PR1 · eventlog pub/sub (~120 LoC)**
- `services/event_log_service.go`: add `Subscribe(type, handler)` map + `notifySubscribers` called from `Append`
- Handlers run in goroutines so a slow listener can't block log writes
- Test: register handler, append event, assert handler called within 100ms

**PR2 · VAPID + service + endpoints (~350 LoC)**
- `services/webpush_service.go` (Init/loadSubscriptions/Send)
- `handlers/webpush.go` (Subscribe/PubKey)
- `models/user.go`: add `WebPushSubscriptionConfig *string`; AutoMigrate
- `cmd/server/main.go`: `webpushSvc.Init(ctx)` after `services.AutoMigrate`
- Tests against mocked push transport

**PR3 · scheduled-job consumer (~80 LoC)**
- `services/webpush_service.go`: `Boot(eventBus)`; `onJobCompleted` / `onJobFailed` handlers
- Wire `webpushSvc.Boot(eventLogBus)` in main.go
- Integration test: run a scheduled job to completion, assert `webpush.Send` mock invoked with expected payload

## 7. Risk register

| # | Risk | Mitigation |
|---|------|------------|
| R1 | Subscription column stores JSON encrypted at rest — schema migration on existing DBs | Column is nullable; AutoMigrate is additive. Document in CHANGELOG |
| R2 | VAPID keys regenerated by mistake → all clients silently fail | Init only generates if both keys missing; once written, never overwritten. Admin "rotate VAPID" requires explicit endpoint (out of scope) |
| R3 | Slow push provider (e.g., FCM tarpit) blocks event handler | Subscribe handler runs in its own goroutine; per-send 10s context timeout |
| R4 | `webpush-go` library version flux | Pin to a known-good release; verify license (MIT) at PR time |
| R5 | Scheduled job lacks user owner → can't route notification | Skip notification cleanly (no panic); Phase 4 may add per-job owner |
| R6 | EventBus is in-process — multi-replica deployment loses events | Phase 2 deploys are single-process; document the constraint. Future: pub/sub via DB-LISTEN or Redis |

## 8. Done criteria

- `go test ./backend/...` green
- Integration test: scheduled job completes → assert webpush.SendNotification called against the user's subscription with `title = "Scheduled job finished"`
- Manual smoke: register a subscription via `POST /web-push/subscribe` with a real Chrome service-worker subscription → trigger a scheduled job → notification appears in Chrome
- `GET /web-push/pubkey` returns a non-empty base64url string

## 9. PR sequencing

PR1 → PR2 → PR3. PR1 has no external dependency. PR2 depends on PR1 only for the eventlog extension type. PR3 depends on Design B (scheduled-jobs) being merged before integration test can pass; can be developed in parallel against a fake event.

## 10. Open questions

None at design time. All resolved during brainstorm:
- Subscription storage: single column on `users` (anything-llm parity)
- VAPID key storage: `SystemSetting` encrypted (not file)
- Event coupling: pub/sub on EventLog rather than direct scheduler call
- Library: `github.com/SherClockHolmes/webpush-go` (MIT, mature)
