# Hermind Telegram Integration Design

> Deep comparison against anything-llm source code (PR #5190, v1.12.0) before design.

## 1. Goals

Bring full-featured Telegram bot integration to Hermind's Go backend, achieving **functional parity** with the original Node.js implementation in anything-llm v1.12.0.

**Feature scope (complete parity):**
- Text chat (RAG + LLM)
- `@agent` invocation with full tool support
- Voice messages (STT via Collector)
- Photo messages (Vision via base64 data URL)
- Document messages (parse via Collector)
- TTS voice replies (`text_only` / `mirror` / `always_voice`)
- Workspace / thread selection
- Pairing-based user approval
- Inline keyboard tool approval
- Polling with exponential backoff retry
- 401 self-cleanup

**Non-goals:**
- Webhook mode (Phase 2)
- Multi-user mode support (explicitly disabled, same as anything-llm)

---

## 2. Deep Comparison: anything-llm vs Hermind

### 2.1 anything-llm Implementation

**Source files studied:**
- `server/endpoints/telegram.js` — 9 HTTP routes
- `server/utils/telegramBot/index.js` — `TelegramBotService` singleton (800+ lines)
- `server/utils/telegramBot/utils/verification.js` — pairing flow
- `server/utils/telegramBot/utils/media.js` — voice/document/photo handling
- `server/models/externalCommunicationConnector.js` — config storage
- `server/jobs/handle-telegram-chat.js` — background worker per chat
- `server/utils/agents/index.js` — agent runtime (for understanding handoff)

**Key architectural decisions in anything-llm:**

| Aspect | anything-llm Choice |
|---|---|
| Telegram SDK | `node-telegram-bot-api` (polling) |
| Config storage | `external_communication_connectors` table with JSON config |
| Token security | AES-GCM encryption at rest |
| User approval | 6-digit pairing code + admin UI approve/deny/revoke |
| Per-chat concurrency | **Bree sub-process** per chat (`handle-telegram-chat.js`) |
| Message ordering | Per-chat `MessageQueue` (sequential) |
| Agent path | Sub-process reuses `streamResponse` with `http-socket` plugin (pseudo-WS) |
| Tool approval | Inline keyboard `tool:approve:{requestId}` / `tool:deny:{requestId}` |
| Polling errors | Exponential backoff (max 10 retries, base 1s, cap 5min); 401 = delete connector |
| Startup cleanup | `getUpdates(limit=100)` then keep only last message per chat |

### 2.2 Hermind Constraints & Opportunities

| Aspect | Hermind State |
|---|---|
| Language | Go 1.26 |
| Web Framework | Gin |
| ORM | GORM (SQLite default) |
| LLM/Agent SDK | Pantheon |
| Agent Runtime | WebSocket-driven (`gorilla/websocket`) |
| Chat Service | `ChatService.Stream` / `ChatService.Complete` |
| Existing Stub | `handlers/telegram.go` — 9 routes returning empty stubs |
| Worker Framework | `robfig/cron/v3` based (`workers.Manager`) — **no subprocess workers** |
| Test DB Pattern | `file:test?mode=memory&cache=shared` + `SetMaxOpenConns(1)` |

**Key differences from anything-llm:**
1. **No subprocess workers** — Go uses goroutines, not Bree. Per-chat LLM processing is a goroutine, not a separate process.
2. **Agent runtime is WS-native** — `Session` has `*wsConn` hard-coded. Telegram has no WebSocket.
3. **No `external_communication_connectors` table** — Node schema has it; Go does not.

---

## 3. Architecture

### 3.1 High-Level Components

```
┌─────────────────────────────────────────────────────────────┐
│                     TelegramBotService                        │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐  │
│  │  Polling    │  │  Message    │  │   Pairing / Auth    │  │
│  │  Loop       │  │  Queue      │  │   Registry          │  │
│  │  (tgbotapi) │  │  (per-chat) │  │   (pending/approved)│  │
│  └──────┬──────┘  └──────┬──────┘  └─────────────────────┘  │
│         │                │                                    │
│         ▼                ▼                                    │
│  ┌─────────────────────────────────────────────────────────┐ │
│  │              Update Router / Command Dispatcher            │ │
│  └─────────────────────────┬───────────────────────────────┘ │
│                            │                                  │
│         ┌──────────────────┼──────────────────┐               │
│         ▼                  ▼                  ▼               │
│  ┌────────────┐   ┌──────────────┐   ┌───────────────┐      │
│  │ /commands  │   │ Plain Text   │   │ Media (voice/ │      │
│  │ handler    │   │ → ChatService│   │ photo/doc)    │      │
│  └────────────┘   └──────┬───────┘   └───────┬───────┘      │
│                          │                     │              │
│                    ┌─────┴─────┐              │              │
│                    ▼           ▼              ▼              │
│              ┌─────────┐ ┌──────────┐ ┌────────────┐        │
│              │Regular  │ │ @agent   │ │ Collector  │        │
│              │Chat     │ │ → AgentIO│ │ (STT/parse)│        │
│              └─────────┘ └──────────┘ └────────────┘        │
└─────────────────────────────────────────────────────────────┘
```

### 3.2 Service Lifecycle

```go
// main.go wiring order:
telegramSvc := services.NewTelegramBotService(db, cfg, sysSvc, enc, chatSvc, runtime, ttsSvc)
telegramSvc.Boot(ctx)   // auto-start if DB has active connector
handlers.RegisterTelegramRoutes(api, cfg, authSvc, telegramSvc)
```

**States:**
- `Boot()` — called once at server startup. Reads DB; if active connector exists, decrypts token and starts polling.
- `Start(config)` — called by `POST /telegram/connect`. Verifies token with Telegram API, persists config, starts polling.
- `Stop()` — called by `POST /telegram/disconnect` or on error. Stops polling, clears queues, removes connector from DB.

---

## 4. Data Model

### 4.1 New Table: `external_communication_connectors`

Aligned with original Node.js Prisma schema.

```go
type ExternalCommunicationConnector struct {
    ID            int       `gorm:"primaryKey;autoIncrement" json:"id"`
    Type          string    `gorm:"unique" json:"type"`        // "telegram"
    Config        string    `json:"config"`                     // encrypted JSON
    Active        bool      `gorm:"default:false" json:"active"`
    CreatedAt     time.Time `json:"createdAt"`
    LastUpdatedAt time.Time `json:"lastUpdatedAt"`
}
```

Registered in `services.AutoMigrate`.

### 4.2 Config Structure

```go
type TelegramConfig struct {
    BotToken          string         `json:"-"`              // plaintext, memory-only
    BotUsername       string         `json:"bot_username"`
    DefaultWorkspace  string         `json:"default_workspace"`
    ApprovedUsers     []TelegramUser `json:"approved_users"`
    VoiceResponseMode string         `json:"voice_response_mode"` // text_only | mirror | always_voice
}

type TelegramUser struct {
    ChatID          string `json:"chatId"`
    Username        string `json:"username,omitempty"`
    FirstName       string `json:"firstName,omitempty"`
    ActiveWorkspace string `json:"active_workspace,omitempty"`
    ActiveThread    string `json:"active_thread,omitempty"`
}
```

### 4.3 Token Encryption

`bot_token` is encrypted with `utils.EncryptionManager` (AES-GCM) before persistence. The plaintext token exists only in memory during `TelegramBotService.config` lifetime.

### 4.4 State Persistence Strategy

| State | Storage | Notes |
|---|---|---|
| Bot config | `external_communication_connectors` (encrypted JSON) | |
| Approved users | Embedded in connector config JSON | Includes `active_workspace` / `active_thread` per user |
| Pending pairings | In-memory `sync.Map` only | Lost on restart; users re-`/start` |
| Per-chat queues | In-memory goroutines | Lost on restart |

---

## 5. Agent Runtime Refactoring

### 5.1 Problem

`Session` is hard-coded to `*wsConn` (Gorilla WebSocket). Telegram has no WebSocket. We need a **transport-neutral abstraction**.

### 5.2 Solution: AgentIO + AgentInput

**AgentIO** — output sink for session events:

```go
type AgentIO interface {
    Send(frame ServerFrame) error
    Close() error
}
```

**AgentInput** — input source for user/transport actions:

```go
type AgentInput interface {
    Read(ctx context.Context) (InputAction, error)
}

type InputAction struct {
    Type        InputType
    Content     string
    RequestID   string
    Approved    bool
    AutoApprove bool
}

type InputType int
const (
    InputContinue InputType = iota
    InputAbort
    InputToolApprovalResponse
    InputSetAutoApprove
)
```

### 5.3 Session Changes

| Before | After |
|---|---|
| `wsConn *wsConn` | `io AgentIO` |
| `readerLoop()` reads from `wsConn` | `readerLoopWithInput(input AgentInput)` reads from `AgentInput` |
| `newSession(..., conn *wsConn, ...)` | `newSession(..., io AgentIO, ...)` |

All internal `s.wsConn.Send(...)` calls become `s.io.Send(...)`.

### 5.4 Runtime New Entrypoint

```go
// HandleWS keeps existing signature; internally wraps wsConn as AgentIO + wsInput as AgentInput.
func (r *Runtime) HandleWS(c *gin.Context) { ... }

// RunAgentDirectly is the non-WS entrypoint for Telegram (and future Discord/Slack).
func (r *Runtime) RunAgentDirectly(
    ctx context.Context,
    invUUID string,
    io AgentIO,
    input AgentInput,
) error
```

`RunAgentDirectly` duplicates the core logic of `HandleWS` (workspace lookup, LLM resolution, registry build, session creation, Run, cleanup) but bypasses HTTP upgrade entirely.

### 5.5 WS Path Compatibility

`wsConn` already satisfies `AgentIO` (has `Send` and `Close`). A thin `wsInput` adapter wraps `wsConn.Read()` into `AgentInput.Read()`. The existing frontend WS path requires **minimal changes**:

```go
// In HandleWS:
sess := newSession(sessCtx, inv.UUID, &ws, user, lm, systemPrompt, tool.NewRegistry(), wc, ttl, r.deps.EventLog)
// ...
go sess.readerLoopWithInput(&wsInput{conn: wc})
```

### 5.6 Telegram AgentIO Implementation

```go
type telegramAgentIO struct {
    bot    *tgbotapi.BotAPI
    chatID int64
}

func (t *telegramAgentIO) Send(frame ServerFrame) error {
    switch frame.Type {
    case FrameStatusResponse:
        _, err := t.bot.Send(tgbotapi.NewMessage(t.chatID, frame.Content))
        return err
    case FrameToolApprovalReq:
        msg := tgbotapi.NewMessage(t.chatID, formatApprovalRequest(frame))
        msg.ParseMode = tgbotapi.ModeMarkdown
        msg.ReplyMarkup = approvalInlineKeyboard(frame.RequestID)
        _, err := t.bot.Send(msg)
        return err
    case FrameWSSFailure:
        _, err := t.bot.Send(tgbotapi.NewMessage(t.chatID, "❌ "+frame.Content))
        return err
    // TextResponse chunks are accumulated by TelegramBotService, not sent per-chunk
    }
    return nil
}

func (t *telegramAgentIO) Close() error { return nil }
```

**Streaming strategy:** Telegram does not support editing messages with live token streams. `telegramAgentIO` accumulates `FrameTextResponseChunk` payloads in an internal `strings.Builder` and flushes the complete message on `FrameFinalizeResponseStream`. For very long responses (> 4096 chars, Telegram's message limit), the output is split into multiple messages. This matches anything-llm's behavior (Bree worker collects full output before sending).

### 5.7 Tool Approval Flow in Telegram

1. Agent calls `RequestApproval` → `Session` sends `FrameToolApprovalReq` via `AgentIO`
2. `telegramAgentIO.Send` renders inline keyboard: `[✅ Approve] [❌ Deny]`
3. User clicks → Telegram sends `CallbackQuery` to bot
4. `TelegramBotService` routes callback to `handleToolApproval(chatID, requestID, approved)`
5. `handleToolApproval` writes `InputAction{Type: InputToolApprovalResponse, ...}` to the chat's `telegramInput.ch`
6. `readerLoopWithInput` reads the action → calls `session.handleApprovalResponse()`
7. Agent resumes execution

---

## 6. Message Processing Pipeline

### 6.1 Text Message

```
Telegram Update (text)
  → messageQueue.enqueue
    → #handleTextMessage
      → isAgentInvocation(text)?
        → YES: runtime.CreateInvocation → runtime.RunAgentDirectly(invUUID, telegramAgentIO, telegramInput)
        → NO:  chatSvc.Complete(ctx, ws, user, threadID, dto.ChatRequest{Message: text})
                 → LLM response → saveChatResponse → bot.SendMessage(chatID, response)
```

Regular chat uses `ChatService.Complete` (non-streaming) because Telegram cannot receive token-by-token updates.

### 6.2 Voice Message

1. `bot.GetFile` → download audio buffer
2. Upload to Collector for STT (`parseDocument` with audio MIME type)
3. Transcription text is treated as a plain text message
4. If `voice_response_mode == "mirror"`, the final LLM response is also sent as TTS voice

### 6.3 Photo Message

1. Download largest `PhotoSize`
2. Base64 encode → `data:image/jpeg;base64,...`
3. `ChatService.Complete` with `Attachments: []string{dataURL}`
4. Pantheon `core.ImagePart{URL: dataURL}` enables vision

### 6.4 Document Message

1. Download document buffer
2. Upload to Collector for parsing (`parseDocument`)
3. Construct prompt: `"The user shared a document named X. Content: ... User request: {caption}"`
4. `ChatService.Complete`

### 6.5 TTS Voice Reply

| Mode | Behavior |
|---|---|
| `text_only` | Always text (default) |
| `mirror` | Voice reply only when user sent voice |
| `always_voice` | Always voice |

Fallback: if TTS fails, send text instead.

---

## 7. Commands & Interaction

### 7.1 Registered Commands

| Command | Description | Requires Approval |
|---|---|---|
| `/start` | Show pairing code or welcome | No |
| `/help` | List commands | No |
| `/switch` | Select workspace/thread (inline keyboard) | Yes |
| `/history [n]` | Show last n chat messages (default 10) | Yes |
| `/model` | Show current model info | Yes |
| `/reset` | Clear chat history for current thread | Yes |

### 7.2 Pairing Flow

1. Unapproved user sends `/start`
2. Bot generates 6-digit pairing code (`%06d` random)
3. Stores in `pendingPairings` (memory, max 10 entries, LIFO eviction of oldest)
4. User goes to UI → Settings → Telegram → sees pending user → approves/denies
5. On approve: user added to `ApprovedUsers`, persisted to DB
6. Bot sends `"You've been approved!"`

### 7.3 Inline Keyboard Usage

**Workspace selection** (`/switch`):
```
Select a workspace:
[Workspace A] [Workspace B]
[Workspace C]
```

**Thread selection** (after workspace chosen):
```
Select a thread:
[General] [Thread-1]
[Thread-2]
```

**Tool approval** (during agent execution):
```
🔧 Tool Approval Required
The agent wants to execute: web-scraping

Do you want to allow this action?
[✅ Approve] [❌ Deny]
```

---

## 8. Resilience & Security

### 8.1 Polling Error Handling

Three classes of errors:

1. **401 Unauthorized** → `selfCleanup()`: stop bot, delete connector from DB
2. **Network errors** (timeout, reset, flood, 5xx, etc.) → exponential backoff retry:
   - Base delay: 1s
   - Max retries: 10
   - Cap: 5min
   - Formula: `min(1s * 2^(n-1), 5min)`
3. **Other errors** (e.g., bad request) → stop immediately, keep DB config

### 8.2 Startup Backlog Cleanup

On `Start()`:
1. Fetch pending updates (`limit=100, timeout=0`)
2. Keep only the **last message per chat** in a map
3. Acknowledge all updates via `offset=lastUpdateID+1`
4. Process the retained last messages

This prevents processing a huge backlog after server restart.

### 8.3 Security Measures

| Measure | Implementation |
|---|---|
| Single-user mode guard | `singleUserMode` middleware on all routes (403 if multi-user). Runtime check in service also calls `selfCleanup` if mode switches to multi-user. |
| Token encryption | `EncryptionManager` AES-GCM at rest |
| Pairing code | 6-digit random, memory-only, 10-entry cap |
| File download | Telegram API domain whitelist (`api.telegram.org`) |
| Queue resource cap | Max 1000 active per-chat queues to prevent goroutine exhaustion |
| SSRF | `validatePushEndpoint` pattern reused for any external URL validation |

---

## 9. Testing Strategy

### 9.1 Test Infrastructure

**SQLite test isolation:**
```go
db, _ := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
sqlDB, _ := db.DB()
sqlDB.SetMaxOpenConns(1)
```

**Telegram API stub:**
```go
httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    // mock /getMe, /sendMessage, /getUpdates, /getFile
}))
```

**AgentIO mock:**
```go
type mockAgentIO struct {
    frames []agent.ServerFrame
}
func (m *mockAgentIO) Send(f agent.ServerFrame) error { m.frames = append(m.frames, f); return nil }
func (m *mockAgentIO) Close() error { return nil }
```

### 9.2 Key Test Cases by PR

**PR1 — Data Layer:**
- `TestTelegramConfig_SaveLoad` — encrypted round-trip
- `TestTelegramConfig_PersistChatState` — workspace/thread mutation survives reload

**PR2 — Bot Core:**
- `TestTelegramBotService_PairingFlow` — /start → code → approve → access
- `TestTelegramBotService_MaxPendingCap` — 11th pending evicts 1st
- `TestTelegramBotService_PollingRetry_ExponentialBackoff` — network error → retry with doubling delay
- `TestTelegramBotService_PollingError_401_SelfCleanup` — 401 deletes connector

**PR3 — Regular Chat:**
- `TestTelegramBotService_TextMessage` — complete chat round-trip
- `TestTelegramBotService_WorkspaceSwitch` — /switch changes routing

**PR4 — Agent Refactor:**
- `TestSession_AgentIO_MockOutput` — Run with mockIO asserts all output frames
- `TestSession_AgentInput_Sequence` — Continue → Abort sequence handled correctly
- `TestRuntime_RunAgentDirectly_EndToEnd` — full invocation lifecycle without WS
- `TestTelegramAgentIO_ToolApproval` — inline keyboard → deny → agent receives rejection
- `TestRuntime_HandleWS_Regression` — existing WS path still works after refactor

**PR5 — Media:**
- `TestTelegramBotService_VoiceMessage_STT` — voice → collector stub → chat
- `TestTelegramBotService_PhotoMessage_Vision` — photo → base64 data URL → attachments
- `TestTelegramBotService_TTS_MirrorMode` — voice in → voice out

---

## 10. Implementation Plan (5 PRs)

| PR | Scope | Files | Est. Lines |
|---|---|---|---|
| **PR1** | Data model + HTTP routes | `models/connector.go`, `services/telegram_config.go`, `handlers/telegram.go`, `services/db.go` | +300 |
| **PR2** | Bot core service | `services/telegram_bot.go`, `services/telegram_queue.go`, `services/telegram_pairing.go` | +400 |
| **PR3** | Regular chat integration | `services/telegram_chat.go`, `services/telegram_commands.go` | +250 |
| **PR4** | Agent runtime refactor + @agent | `agent/types.go`, `agent/session.go`, `agent/handler.go`, `agent/runtime.go`, `services/telegram_agent.go` | +350 |
| **PR5** | Media + TTS | `services/telegram_media.go`, `services/telegram_tts.go` | +300 |

**Total:** ~20 files, ~1600 lines.

---

## 11. Risks & Mitigations

| Risk | Impact | Mitigation |
|---|---|---|
| Agent runtime refactor breaks frontend WS | High | Comprehensive regression test on `HandleWS` before PR4 merge |
| Collector STT not ready in Go backend | Medium | Voice messages degrade gracefully to "STT unavailable" text |
| TTS service lacks buffer API | Medium | Phase 5 can be deferred if TTS interface needs expansion |
| `go-telegram-bot-api` archived | Low | Community fork is actively maintained; API is stable |
| Single-user mode assumption invalid | Low | Explicit middleware + runtime guard; multi-user triggers self-cleanup |

---

## 12. References

- anything-llm PR #5190 — Telegram bot connector (by @shatfield4)
- anything-llm PR #5306 — Telegram bot settings UI redesign
- anything-llm `server/utils/telegramBot/index.js` — `TelegramBotService`
- anything-llm `server/utils/telegramBot/utils/verification.js` — pairing flow
- anything-llm `server/utils/telegramBot/utils/media.js` — voice/photo/document
- anything-llm `server/endpoints/telegram.js` — HTTP routes
- Hermind `backend/internal/agent/session.go` — current Session structure
- Hermind `backend/internal/agent/handler.go` — current `HandleWS`
- Hermind `backend/internal/services/chat_service.go` — `Complete` / `Stream`
