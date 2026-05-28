# API v1 PR2 — DTO Layer + ChatRequest Overrides Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `gpowers:subagent-driven-development` (recommended) or `gpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Land the DTO scaffolding (`internal/dto/api.go`) required by all API v1 handlers, plus the `SystemPromptOverride` / `TemperatureOverride` extensions on `ChatRequest` + `StreamChatRequest` needed by the OpenAI compat layer. Wire `SystemPromptOverride` through `ChatService.buildRAGContext`. No handlers, no service business logic beyond the 5-line override branch (those land in PR1/3/4/5).

**Architecture:** All new types live in `internal/dto/api.go`. Existing `ChatRequest` + `StreamChatRequest` get two optional pointer fields. `ChatService.buildRAGContext` gains one extra parameter `systemPromptOverride *string`; both callsites (`Stream`, `Complete`) pass `req.SystemPromptOverride`. The branch logic is: when `systemPromptOverride != nil` use it instead of `ws.OpenAiPrompt`.

**Tech Stack:** Go 1.22+, GORM, sqlite (test), testify, encoding/json.

**Source spec:** `.gpowers/designs/2026-05-25-api-v1-layer-design.md` §5.2 + §7.

**Reference Node implementation:**
- `server/endpoints/api/openai/index.js:83-200` — chat/completions handler that constructs the prompt from messages.
- `server/utils/chats/openaiCompatible.js` — system-prompt + history extraction.

**Independence from PR1:** This PR touches only `dto/` + `chat_service.go`. PR1 touches only `workspace_service.go` + `document_service.go`. The two PRs can land in either order.

---

## Pre-task: Read this section once before starting

### Existing Go surface (do not duplicate)

- `dto/chat.go`:
  - `ChatRequest{ Message, Mode, SessionID, Reset, Attachments }`
  - `StreamChatRequest{ Message, Attachments }`
  - `ChatResponse`, `StreamChatResponse`, `UpdateChatRequest`
- `ChatService.buildRAGContext(ctx, ws, user, threadID, message string)` at `chat_service.go:35`. Reads `ws.OpenAiPrompt` at line 46 — this is the **only** place to override.
- `ChatService.Stream` at `chat_service.go:94` — calls `buildRAGContext` at line 103.
- `ChatService.Complete` at `chat_service.go:191` — calls `buildRAGContext` similarly.

### Adds (this PR)

| File | Add |
|---|---|
| `internal/dto/api.go` (new) | `OpenAIChatRequest`, `OpenAIMessage`, `OpenAIEmbeddingRequest`, `APIDocumentUploadRequest`, `APIRawTextRequest`, `APIDocumentRemoveFolderRequest`, `APISystemRemoveDocumentsRequest`, `APIUpdatePinRequest`, `APIAdminPreferencesRequest` |
| `internal/dto/chat.go` (modify) | add `SystemPromptOverride *string`, `TemperatureOverride *float64` to `ChatRequest` + `StreamChatRequest` |
| `internal/services/chat_service.go` (modify) | extend `buildRAGContext` signature with `systemPromptOverride *string`; honor it before reading `ws.OpenAiPrompt`; update both callsites in `Stream` and `Complete` |
| `internal/services/chat_service_test.go` (new) | focused tests for the override branch using a no-op vector setup |

### Out of scope (explicit)

- **Service-side enforcement of `TemperatureOverride`** — `providers.LLMProvider` does not currently accept a per-call temperature. The field is plumbed in DTO only; PR5 (OpenAI compat) handler will read it but pass-through requires a follow-up `LLMProvider` change. Document this in §"Known gaps".
- **Handler-level translation** of OpenAI messages → `ChatRequest` (PR5).
- **Attachments base64 decoding for OpenAI content arrays** — covered in PR5.
- **`StreamChatRequest` field additions for `SessionID`** — separate concern; do not piggyback.

### Field naming and JSON tags

- Match the camelCase Node payloads exactly (these DTOs are over the wire).
- `SystemPromptOverride` → `json:"systemPromptOverride,omitempty"`
- `TemperatureOverride` → `json:"temperatureOverride,omitempty"`
- Pointer types so that `omitempty` correctly skips them when absent (Go's zero-value rules require `*string`/`*float64`).

### TDD discipline

Each task: write failing test → run + confirm fail → implement → run + confirm pass → commit. Tests live alongside the file they cover.

---

## Task 1: Create `internal/dto/api.go` with API v1 DTOs

**Files:**
- Create: `backend/internal/dto/api.go`
- Create: `backend/internal/dto/api_test.go`

- [ ] **Step 1: Write the failing test**

```go
// File: backend/internal/dto/api_test.go
package dto

import (
    "encoding/json"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestAPIRawTextRequest_UnmarshalsNodeShape(t *testing.T) {
    raw := []byte(`{
        "textContent": "Hello world",
        "title": "greeting",
        "addToWorkspaces": "w1,w2",
        "metadata": {"title": "greeting", "docSource": "test"}
    }`)
    var got APIRawTextRequest
    require.NoError(t, json.Unmarshal(raw, &got))
    assert.Equal(t, "Hello world", got.Text)
    assert.Equal(t, "greeting", got.Title)
    assert.Equal(t, "w1,w2", got.AddToWorkspaces)
    assert.NotNil(t, got.Metadata)
}

func TestAPIUpdatePinRequest_NodeShape(t *testing.T) {
    raw := []byte(`{"docPath":"custom-documents/a.json","pinStatus":true}`)
    var got APIUpdatePinRequest
    require.NoError(t, json.Unmarshal(raw, &got))
    assert.Equal(t, "custom-documents/a.json", got.DocPath)
    assert.True(t, got.PinValue)
}

func TestAPIDocumentRemoveFolderRequest_NodeShape(t *testing.T) {
    raw := []byte(`{"name":"my-folder"}`)
    var got APIDocumentRemoveFolderRequest
    require.NoError(t, json.Unmarshal(raw, &got))
    assert.Equal(t, "my-folder", got.Name)
}

func TestAPISystemRemoveDocumentsRequest_NodeShape(t *testing.T) {
    raw := []byte(`{"names":["custom-documents/a.json","custom-documents/b.json"]}`)
    var got APISystemRemoveDocumentsRequest
    require.NoError(t, json.Unmarshal(raw, &got))
    assert.Equal(t, []string{"custom-documents/a.json", "custom-documents/b.json"}, got.Names)
}

func TestOpenAIChatRequest_NodeShape(t *testing.T) {
    raw := []byte(`{
        "model": "my-workspace",
        "messages": [
            {"role":"system","content":"Be helpful."},
            {"role":"user","content":"Hi"}
        ],
        "temperature": 0.4,
        "stream": true
    }`)
    var got OpenAIChatRequest
    require.NoError(t, json.Unmarshal(raw, &got))
    assert.Equal(t, "my-workspace", got.Model)
    require.Len(t, got.Messages, 2)
    assert.Equal(t, "system", got.Messages[0].Role)
    require.NotNil(t, got.Temperature)
    assert.InDelta(t, 0.4, *got.Temperature, 0.001)
    assert.True(t, got.Stream)
}

func TestOpenAIEmbeddingRequest_NodeShape_StringInput(t *testing.T) {
    raw := []byte(`{"model":"text-embedding-ada-002","input":"hello"}`)
    var got OpenAIEmbeddingRequest
    require.NoError(t, json.Unmarshal(raw, &got))
    assert.Equal(t, "text-embedding-ada-002", got.Model)
}

func TestAPIDocumentUploadRequest_NodeShape(t *testing.T) {
    raw := []byte(`{"addToWorkspaces":"w1,w2","metadata":{"title":"x"}}`)
    var got APIDocumentUploadRequest
    require.NoError(t, json.Unmarshal(raw, &got))
    assert.Equal(t, "w1,w2", got.AddToWorkspaces)
}
```

- [ ] **Step 2: Run the test and confirm it fails**

```bash
cd backend && go test ./internal/dto/ -count=1
```

Expected: undefined types.

- [ ] **Step 3: Create the file**

```go
// File: backend/internal/dto/api.go
package dto

// ---------- OpenAI compatible ----------

type OpenAIChatRequest struct {
    Model       string          `json:"model"`
    Messages    []OpenAIMessage `json:"messages"`
    Temperature *float64        `json:"temperature,omitempty"`
    Stream      bool            `json:"stream,omitempty"`
}

// OpenAIMessage.Content is `any` because OpenAI accepts either a string or an
// array of content parts (text + image_url). Handler-side parsing in PR5
// converts both forms to plain text + attachments.
type OpenAIMessage struct {
    Role    string `json:"role"`
    Content any    `json:"content"`
}

type OpenAIEmbeddingRequest struct {
    Model string `json:"model"`
    // Input is any because OpenAI accepts string or []string. Handler normalizes.
    Input any `json:"input"`
}

// ---------- API v1 specific request bodies ----------

type APIDocumentUploadRequest struct {
    AddToWorkspaces string `json:"addToWorkspaces"` // comma-delimited slugs (Node parity)
    Metadata        any    `json:"metadata,omitempty"`
}

type APIRawTextRequest struct {
    Text            string `json:"textContent"`
    Title           string `json:"title"`
    Metadata        any    `json:"metadata,omitempty"`
    AddToWorkspaces string `json:"addToWorkspaces"` // comma-delimited
}

type APIDocumentRemoveFolderRequest struct {
    Name string `json:"name"`
}

type APISystemRemoveDocumentsRequest struct {
    Names []string `json:"names"`
}

type APIUpdatePinRequest struct {
    DocPath  string `json:"docPath"`
    PinValue bool   `json:"pinStatus"`
}

type APIAdminPreferencesRequest map[string]any
```

- [ ] **Step 4: Run the test and confirm it passes**

```bash
cd backend && go test ./internal/dto/ -count=1
```

- [ ] **Step 5: Commit**

```bash
git add backend/internal/dto/api.go backend/internal/dto/api_test.go
git commit -m "feat(api-v1): scaffold API v1 + OpenAI request DTOs"
```

---

## Task 2: Extend `ChatRequest` + `StreamChatRequest` with override fields

**Files:**
- Modify: `backend/internal/dto/chat.go`
- Modify: `backend/internal/dto/api_test.go` (or new chat_test.go)

- [ ] **Step 1: Write the failing test**

Append to `backend/internal/dto/api_test.go` (or create `chat_test.go`):

```go
func TestChatRequest_AcceptsOverrides(t *testing.T) {
    raw := []byte(`{
        "message": "Hi",
        "mode": "chat",
        "systemPromptOverride": "You are a stoic.",
        "temperatureOverride": 0.2
    }`)
    var got ChatRequest
    require.NoError(t, json.Unmarshal(raw, &got))
    assert.Equal(t, "Hi", got.Message)
    require.NotNil(t, got.SystemPromptOverride)
    assert.Equal(t, "You are a stoic.", *got.SystemPromptOverride)
    require.NotNil(t, got.TemperatureOverride)
    assert.InDelta(t, 0.2, *got.TemperatureOverride, 0.001)
}

func TestChatRequest_OmitsOverridesWhenNil(t *testing.T) {
    req := ChatRequest{Message: "Hi"}
    out, err := json.Marshal(req)
    require.NoError(t, err)
    assert.NotContains(t, string(out), "systemPromptOverride")
    assert.NotContains(t, string(out), "temperatureOverride")
}

func TestStreamChatRequest_AcceptsOverrides(t *testing.T) {
    raw := []byte(`{"message":"Hi","systemPromptOverride":"X"}`)
    var got StreamChatRequest
    require.NoError(t, json.Unmarshal(raw, &got))
    require.NotNil(t, got.SystemPromptOverride)
    assert.Equal(t, "X", *got.SystemPromptOverride)
}
```

- [ ] **Step 2: Run the test and confirm it fails**

```bash
cd backend && go test ./internal/dto/ -count=1
```

- [ ] **Step 3: Implement**

In `backend/internal/dto/chat.go`:

```go
type ChatRequest struct {
    Message              string   `json:"message"`
    Mode                 string   `json:"mode,omitempty"`
    SessionID            string   `json:"sessionId,omitempty"`
    Reset                bool     `json:"reset,omitempty"`
    Attachments          []string `json:"attachments,omitempty"`
    SystemPromptOverride *string  `json:"systemPromptOverride,omitempty"`
    TemperatureOverride  *float64 `json:"temperatureOverride,omitempty"`
}

type StreamChatRequest struct {
    Message              string   `json:"message"`
    Attachments          []string `json:"attachments,omitempty"`
    SystemPromptOverride *string  `json:"systemPromptOverride,omitempty"`
    TemperatureOverride  *float64 `json:"temperatureOverride,omitempty"`
}
```

- [ ] **Step 4: Run the test and confirm it passes**

- [ ] **Step 5: Commit**

```bash
git add backend/internal/dto/chat.go backend/internal/dto/api_test.go
git commit -m "feat(api-v1): ChatRequest/StreamChatRequest accept system/temp overrides"
```

---

## Task 3: `buildRAGContext` honors `systemPromptOverride`

Extend the helper's signature with one new parameter. When non-nil, use it as the base system prompt instead of `ws.OpenAiPrompt`. RAG context augmentation (the `\n\nContext:\n...` suffix) still applies on top.

**Files:**
- Modify: `backend/internal/services/chat_service.go`
- Create: `backend/internal/services/chat_service_test.go`

- [ ] **Step 1: Write the failing test**

```go
// File: backend/internal/services/chat_service_test.go
package services

import (
    "context"
    "testing"

    "github.com/odysseythink/hermind/backend/internal/config"
    "github.com/odysseythink/hermind/backend/internal/models"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "gorm.io/driver/sqlite"
    "gorm.io/gorm"
)

func setupChatDB(t *testing.T) *gorm.DB {
    t.Helper()
    db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
    require.NoError(t, err)
    require.NoError(t, AutoMigrate(db))
    return db
}

func TestBuildRAGContext_OverrideTakesPrecedence(t *testing.T) {
    db := setupChatDB(t)
    cfg := &config.Config{}
    vec := NewVectorService(cfg) // nil provider — RAG section skipped
    svc := NewChatService(db, cfg, vec, nil, nil)

    wsPrompt := "default prompt"
    ws := &models.Workspace{
        Name:         "ws",
        Slug:         "ws",
        OpenAiPrompt: &wsPrompt,
    }
    require.NoError(t, db.Create(ws).Error)

    override := "OVERRIDE PROMPT"
    sys, _, _, err := svc.buildRAGContext(context.Background(), ws, nil, nil, "hi", &override)
    require.NoError(t, err)
    assert.Equal(t, "OVERRIDE PROMPT", sys)
}

func TestBuildRAGContext_NilOverrideFallsBackToWorkspacePrompt(t *testing.T) {
    db := setupChatDB(t)
    cfg := &config.Config{}
    vec := NewVectorService(cfg)
    svc := NewChatService(db, cfg, vec, nil, nil)

    wsPrompt := "default prompt"
    ws := &models.Workspace{Name: "ws", Slug: "ws", OpenAiPrompt: &wsPrompt}
    require.NoError(t, db.Create(ws).Error)

    sys, _, _, err := svc.buildRAGContext(context.Background(), ws, nil, nil, "hi", nil)
    require.NoError(t, err)
    assert.Equal(t, "default prompt", sys)
}

func TestBuildRAGContext_EmptyOverrideStringFallsBackToWorkspacePrompt(t *testing.T) {
    db := setupChatDB(t)
    cfg := &config.Config{}
    vec := NewVectorService(cfg)
    svc := NewChatService(db, cfg, vec, nil, nil)

    wsPrompt := "default prompt"
    ws := &models.Workspace{Name: "ws", Slug: "ws", OpenAiPrompt: &wsPrompt}
    require.NoError(t, db.Create(ws).Error)

    empty := ""
    // An explicit empty string override is treated as "no override" — Node parity:
    // openaiCompatible.js only uses systemMessage when it has content.
    sys, _, _, err := svc.buildRAGContext(context.Background(), ws, nil, nil, "hi", &empty)
    require.NoError(t, err)
    assert.Equal(t, "default prompt", sys)
}
```

- [ ] **Step 2: Run the test and confirm it fails**

```bash
cd backend && go test ./internal/services/ -run TestBuildRAGContext -count=1
```

Expected: signature mismatch.

- [ ] **Step 3: Implement**

In `chat_service.go`, change the signature and the system-prompt branch:

```go
func (s *ChatService) buildRAGContext(
    ctx context.Context,
    ws *models.Workspace,
    user *models.User,
    threadID *int,
    message string,
    systemPromptOverride *string,
) (systemPrompt string, sources []any, history []core.Message, err error) {
    historyLimit := ws.OpenAiHistory
    if historyLimit <= 0 {
        historyLimit = 20
    }
    history, err = s.buildChatHistory(ctx, ws.ID, threadID, historyLimit)
    if err != nil {
        return "", nil, nil, err
    }

    // PR2: API v1 OpenAI-compat may pass an explicit override; treat empty string as "no override".
    if systemPromptOverride != nil && *systemPromptOverride != "" {
        systemPrompt = *systemPromptOverride
    } else if ws.OpenAiPrompt != nil {
        systemPrompt = *ws.OpenAiPrompt
    }

    // ... existing RAG block unchanged
```

- [ ] **Step 4: Update both callsites**

In `Stream` (around `chat_service.go:103`):

```go
systemPrompt, sources, history, err := s.buildRAGContext(ctx, ws, user, threadID, req.Message, req.SystemPromptOverride)
```

In `Complete` (analogous call): pass `req.SystemPromptOverride` likewise.

- [ ] **Step 5: Run the test and confirm it passes**

```bash
cd backend && go test ./internal/services/ -run TestBuildRAGContext -count=1
```

- [ ] **Step 6: Run the wider suite to catch any unexpected regressions**

```bash
cd backend && go test ./... -count=1
```

If a test fails because the old `buildRAGContext` signature was called somewhere else (it's unexported; should only be the two callsites above), add the new argument there too.

- [ ] **Step 7: Commit**

```bash
git add backend/internal/services/chat_service.go backend/internal/services/chat_service_test.go
git commit -m "feat(api-v1): ChatService.buildRAGContext honors systemPromptOverride"
```

---

## Task 4: Full-suite verify + build + vet

- [ ] **Step 1: Run the entire test suite**

```bash
cd backend && go test ./... -count=1
```

Must be green. The only behavior changes are:
1. New optional DTO fields (back-compat for clients omitting them).
2. New `buildRAGContext` parameter (compile-time enforced; covered by Stream/Complete callsite updates).

- [ ] **Step 2: Build verify**

```bash
cd backend && go build ./...
```

- [ ] **Step 3: Vet**

```bash
cd backend && go vet ./...
```

---

## Acceptance criteria

- [ ] `internal/dto/api.go` exists with 9 typed structs matching Node JSON shapes.
- [ ] `ChatRequest` + `StreamChatRequest` accept `systemPromptOverride` (`*string`) and `temperatureOverride` (`*float64`), both `omitempty`.
- [ ] `ChatService.buildRAGContext` accepts a 6th argument `systemPromptOverride *string` and treats `nil` + `""` identically as "fall back to `ws.OpenAiPrompt`".
- [ ] `Stream` and `Complete` pass `req.SystemPromptOverride` through to `buildRAGContext`.
- [ ] `go test ./...` green; `go vet ./...` clean; `go build ./...` succeeds.
- [ ] No regression in any existing chat / stream test.

---

## Known gaps after PR2 (track but DO NOT implement here)

1. **`TemperatureOverride` is parsed but not enforced** — `providers.LLMProvider` does not currently accept a per-call temperature. PR2 plumbs the field so PR5 OpenAI handler can read it, but actual override at the LLM call site requires extending `LLMProvider` (`internal/providers/`) to accept per-request temperature. File as a PR5 prerequisite or separate hardening PR.
2. **OpenAI content-array parsing** — `OpenAIMessage.Content` is `any`. PR5 handler translates either `string` or `[]content_part` shapes; PR2 only ensures both unmarshal cleanly into the DTO.
3. **Attachments DTO shape mismatch** — `ChatRequest.Attachments []string` (base64 + mime) vs OpenAI's image_url parts. Handled in PR5 handler translation.
4. **`OpenAIEmbeddingRequest.Input` union** (`string | []string`) — handler-side normalization in PR5.
5. **Streaming `delta` chunk DTO** — OpenAI SSE chunk has `{id, object:"chat.completion.chunk", choices:[{delta:{content}}]}`. The internal `StreamChatResponse` shape is different; PR5 will add a `dto/openai_stream.go` for translation.
