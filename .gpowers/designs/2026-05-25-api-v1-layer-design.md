# API v1 Layer Design — backend

> Scope: Implement all 55 missing API v1 routes (51 REST + 4 OpenAI-compatible) in backend to achieve feature parity with Node.js `server/endpoints/api/`.

---

## 1. Goals

- Provide external programmatic access to Hermind via API Key authentication.
- Achieve 100% route parity with Node.js API v1 layer.
- Enable OpenAI-compatible drop-in replacement for existing integrations.
- Reuse existing service layer to minimize code duplication and maintenance burden.

## 2. Non-Goals

- Swagger/OpenAPI auto-generation (out of scope; can be added later).
- Changing existing Web handler behavior or authentication flow.
- Adding new business logic not present in Node.js API v1.

## 3. Architecture

### 3.1 Pattern: Thin Handler Layer

All API v1 handlers are thin adapters:

```
HTTP Request → ValidAPIKey middleware → API v1 Handler → Existing Service → DB/VectorDB/LLM
```

- **Handler**: parameter binding, API Key auth, JSON response formatting.
- **Service**: existing business logic (zero duplication).
- Only OpenAI-compatible routes require request/response format translation.

### 3.2 Module Split

| File | Routes | Auth | Strategy |
|------|--------|------|----------|
| `api_auth.go` | 1 | API Key | Trivial |
| `api_user.go` | 2 | API Key | Thin wrapper over Admin/TempToken service |
| `api_admin.go` | 13 | API Key | Thin wrapper over Admin/System/Workspace service |
| `api_system.go` | 6 | API Key | Thin wrapper over System/Vector/Document/Chat service |
| `api_workspace.go` | 11 | API Key | Thin wrapper over Workspace/Chat/VectorSearch/Document service |
| `api_document.go` | 12 | API Key | Thin wrapper over Document service; file upload via multipart |
| `api_thread.go` | 6 | API Key | Thin wrapper over Thread/Chat service |
| `api_openai.go` | 4 | API Key | Independent: format translation + calls Chat/Embed/Vector service |

### 3.3 Registration (main.go)

Stay consistent with the existing `RegisterAPIEmbedRoutes` convention
(`handlers/api_embed.go:130`): each `RegisterAPI*Routes` takes the **same `api`
group** the web handlers use and registers absolute paths under `/v1/...`. Do
**not** introduce a separate `v1 := api.Group("/v1")`, which would either
double-prefix `api_embed.go` or force a churn refactor of an already-tested
file.

```go
api := r.Group("/api") // existing
handlers.RegisterAPIAuthRoutes(api, apiKeySvc)
handlers.RegisterAPIUserRoutes(api, apiKeySvc, adminSvc, tempTokenSvc, sysSvc)
handlers.RegisterAPIAdminRoutes(api, apiKeySvc, adminSvc, sysSvc, wsSvc, wsChatSvc)
handlers.RegisterAPISystemRoutes(api, apiKeySvc, sysSvc, vectorSvc, docSvc, wsChatSvc)
handlers.RegisterAPIWorkspaceRoutes(api, apiKeySvc, wsSvc, chatSvc, vectorSearchSvc, docSvc)
handlers.RegisterAPIDocumentRoutes(api, apiKeySvc, docSvc, coll, cfg)
handlers.RegisterAPIThreadRoutes(api, apiKeySvc, threadSvc, chatSvc)
handlers.RegisterAPIOpenAIRoutes(api, apiKeySvc, wsSvc, chatSvc, threadSvc, emb)
```

Inside each handler file, routes are written like:

```go
r.GET("/v1/system", middleware.ValidAPIKey(apiKeySvc), h.GetAllSettings)
```

**Conditional services**: `vectorSearchSvc` (main.go:97-100) and `emb` are only
initialized when an embedder provider succeeds at startup. Routes that depend on
either (`/v1/workspace/:slug/vector-search`, `/v1/workspace/:slug/update-embeddings`,
`/v1/openai/embeddings`) must return **HTTP 503** with
`{"error":"embedder not configured"}` when the dependency is nil, instead of
panicking on a nil dereference. Register these handlers behind a nil-check
wrapper or assert in `NewAPI*Handler`.

### 3.4 API Key Scoping (known gap)

The current `middleware.ValidAPIKey` (`middleware/api_key.go:21`) only validates
that a key is unexpired and present in `api_keys`. It does **not** enforce
role-based scoping (admin / manager / default). Node's `/v1/admin/*` shows the
same posture: any valid key can call admin endpoints.

The v1 Go port intentionally mirrors Node here to preserve client compatibility.
Finer scoping (e.g. `middleware.RequireAPIKeyRole("admin")` driven by
`api_key.created_by_user_id → user.role`) is **out of scope** for the 55-route
parity work. Filed as a follow-up in the risks table (§9).

## 4. Route Mapping

### 4.1 Auth (1 route)

| Route | Method | Service | Notes |
|-------|--------|---------|-------|
| `/v1/auth` | GET | — | Returns `{authenticated: true}` if API key is valid |

### 4.2 User Management (2 routes)

| Route | Method | Service | Notes |
|-------|--------|---------|-------|
| `/v1/users` | GET | `AdminService.ListUsers` | Filters to `id, username, role` |
| `/v1/users/:id/issue-auth-token` | GET | `TempAuthTokenService.Issue` | Requires multi-user mode + simple SSO |

### 4.3 Admin (13 routes)

| Route | Method | Service | Notes |
|-------|--------|---------|-------|
| `/v1/admin/is-multi-user-mode` | GET | `SystemService.IsMultiUserMode` | |
| `/v1/admin/users` | GET | `AdminService.ListUsers` | |
| `/v1/admin/users/new` | POST | `AdminService.CreateUser` | |
| `/v1/admin/users/:id` | POST | `AdminService.UpdateUser` | |
| `/v1/admin/users/:id` | DELETE | `AdminService.DeleteUser` | |
| `/v1/admin/invites` | GET | `AdminService.ListInvites` | |
| `/v1/admin/invite/new` | POST | `AdminService.CreateInvite` | |
| `/v1/admin/invite/:id` | DELETE | `AdminService.DeactivateInvite` | |
| `/v1/admin/workspaces/:workspaceId/users` | GET | `WorkspaceService.ListWorkspaceUsers` | |
| `/v1/admin/workspaces/:workspaceId/update-users` | POST | `WorkspaceService.UpdateUsers` | **Deprecated** |
| `/v1/admin/workspaces/:workspaceSlug/manage-users` | POST | `WorkspaceService.UpdateUsers` + create new users | Supports `reset` flag |
| `/v1/admin/workspace-chats` | POST | `WorkspaceChatService.List` | Pagination: offset * 20 |
| `/v1/admin/preferences` | POST | `SystemService.SetSetting` | |

### 4.4 System (6 routes)

| Route | Method | Service | Notes |
|-------|--------|---------|-------|
| `/v1/system` | GET | `SystemService.GetAllSettings` | |
| `/v1/system/env-dump` | GET | reuse `SystemHandler.EnvDump` noop (`handlers/system.go:503`) | Go persists settings in DB; intentional noop 200 (no `.env` file in Go runtime) |
| `/v1/system/vector-count` | GET | `VectorService.TotalVectors` (`vector_service.go:78`) | Already implemented |
| `/v1/system/update-env` | POST | reuse `SystemHandler.UpdateEnv` (`handlers/system.go:134`) | Already implemented; no new service method |
| `/v1/system/export-chats` | GET | `WorkspaceChatService.ExportChats` (`workspace_chat_service.go:150`) | Already implemented; query param `type`: jsonl/json/csv/jsonAlpaca |
| `/v1/system/remove-documents` | DELETE | `DocumentService.PurgeByDocName` (**new**) | Body: `{names: []string}`; loops + calls purge per name |

### 4.5 Workspace (11 routes)

| Route | Method | Service | Notes |
|-------|--------|---------|-------|
| `/v1/workspaces` | GET | `WorkspaceService.List` | |
| `/v1/workspace/new` | POST | `WorkspaceService.Create` | No userID (API context) |
| `/v1/workspace/:slug` | GET | `WorkspaceService.GetBySlug` | |
| `/v1/workspace/:slug/update` | POST | `WorkspaceService.Update` | |
| `/v1/workspace/:slug/update-pin` | POST | `WorkspaceService.UpdatePin` | **New service method** |
| `/v1/workspace/:slug` | DELETE | `WorkspaceService.Delete` | |
| `/v1/workspace/:slug/chats` | GET | `WorkspaceService.GetChats` | |
| `/v1/workspace/:slug/stream-chat` | POST | `ChatService.Stream` | |
| `/v1/workspace/:slug/chat` | POST | `ChatService.Complete` | |
| `/v1/workspace/:slug/vector-search` | POST | `VectorSearchService.Search` | |
| `/v1/workspace/:slug/update-embeddings` | POST | `DocumentService.UpdateEmbeddings` | |

### 4.6 Document (12 routes)

| Route | Method | Service | Notes |
|-------|--------|---------|-------|
| `/v1/document/upload` | POST | `DocumentService.SaveUpload` | Multipart; optional `addToWorkspaces` + `metadata` |
| `/v1/document/upload/:folderName` | POST | `DocumentService.SaveUpload` | Upload into folder |
| `/v1/document/upload-link` | POST | `DocumentService.UploadLink` | |
| `/v1/document/raw-text` | POST | `DocumentService.SaveRawText` | **New service method** |
| `/v1/document/create-folder` | POST | `DocumentService.CreateFolder` | |
| `/v1/document/move-files` | POST | `DocumentService.MoveFiles` | |
| `/v1/documents` | GET | `DocumentService.ListDocuments` | |
| `/v1/documents/folder/:folderName` | GET | `DocumentService.ListFolderDocuments` | |
| `/v1/document/:docName` | GET | `DocumentService.GetByDocName` | |
| `/v1/document/metadata-schema` | GET | — | Hardcoded schema response |
| `/v1/document/accepted-file-types` | GET | — | Hardcoded file type list |
| `/v1/document/remove-folder` | DELETE | `DocumentService.RemoveFolder` | **New service method** |

### 4.7 Thread (6 routes)

| Route | Method | Service | Notes |
|-------|--------|---------|-------|
| `/v1/workspace/:slug/thread/new` | POST | `ThreadService.Create` | |
| `/v1/workspace/:slug/thread/:threadSlug/update` | POST | `ThreadService.Update` | |
| `/v1/workspace/:slug/thread/:threadSlug` | DELETE | `ThreadService.Delete` | |
| `/v1/workspace/:slug/thread/:threadSlug/chats` | GET | `ThreadService.GetThreadChats` | |
| `/v1/workspace/:slug/thread/:threadSlug/chat` | POST | `ChatService.Complete` | Pass threadID |
| `/v1/workspace/:slug/thread/:threadSlug/stream-chat` | POST | `ChatService.Stream` | Pass threadID |

### 4.8 OpenAI Compatible (4 routes)

| Route | Method | Service | Notes |
|-------|--------|---------|-------|
| `/v1/openai/models` | GET | `WorkspaceService.List` | Returns workspaces as models |
| `/v1/openai/chat/completions` | POST | `ChatService.Complete` / `ChatService.Stream` (with `SystemPromptOverride`, see §5.2) | Format translation; `model` may be `slug` or `slug:threadSlug` |
| `/v1/openai/embeddings` | POST | `Embedder.EmbedChunks` | |
| `/v1/openai/vector_stores` | GET | `WorkspaceService.List` + `Document.Count` | |

## 5. Service Extensions

After re-auditing existing Go services, only **four genuinely new** service
methods are needed. Two prior entries (`WorkspaceChatService.Export`,
`SystemService.DumpEnv`) are dropped: the former already exists as
`ExportChats`, the latter is intentionally a noop handler. The
`ChatService.CompleteWithHistory` refactor was found to be unnecessary — a small
DTO addition replaces it.

### 5.1 Net-new service methods

1. **`WorkspaceService.UpdatePin(ctx, slug string, docPath string, pinned bool) error`**
   - Pin/unpin a single document in a workspace. Node body: `{docPath, pinValue: bool}`.
   - Updates `workspace_documents.pinned`. Cheap.

2. **`DocumentService.RemoveFolder(ctx, folderName string) error`** *(orchestrator)*
   - Must do more than `FileSystemService.RemoveFolder` (`filesystem_service.go:71`):
     a. Refuse to delete the reserved `custom-documents` folder.
     b. For each `.json` in the folder: purge per-doc vector cache + unembed from any workspace.
     c. Cascade `workspace_documents` rows.
     d. Call `FileSystemService.RemoveFolder` to delete the directory.
   - Loop order matters — DB cascade after vector purge to keep state consistent on partial failure.

3. **`DocumentService.SaveRawText(ctx, text, title string, metadata map[string]any, workspaceSlugs []string) ([]*models.WorkspaceDocument, error)`**
   - Persist `text` as a `.txt` document under `custom-documents/`.
   - If `workspaceSlugs` is non-empty: embed into each. Returns one
     `WorkspaceDocument` per successful workspace bind.
   - Handler accepts Node's comma-delimited `addToWorkspaces` string and splits before calling.

4. **`DocumentService.PurgeByDocName(ctx, docName string) error`**
   - Cross-workspace purge by document name (the doc name embeds the `.json` filename uniquely).
   - Calls vector purge + workspace document cascade + `FileSystemService.RemoveDocument`.

### 5.2 DTO-only extensions (no service refactor needed)

For OpenAI compatibility, **do not refactor `ChatService.Complete`**. Instead,
add two optional fields to the existing request DTO and translate in the handler:

```go
type ChatRequest struct {
    Message            string
    Mode               string
    Attachments        []dto.Attachment
    SystemPromptOverride *string  // NEW: set when OpenAI request includes role:system
    TemperatureOverride  *float64 // NEW: pass-through from OpenAI request
}
```

`ChatService.buildRAGContext` (`chat_service.go:35`) reads `SystemPromptOverride`
when non-nil instead of pulling from `workspace.OpenAiPrompt`. Diff is ~5 lines.

### 5.3 Pre-existing services to verify (no changes expected)

| Method | Location | Used by |
|---|---|---|
| `WorkspaceChatService.ExportChats` | `workspace_chat_service.go:150` | `/v1/system/export-chats` |
| `VectorService.TotalVectors` | `vector_service.go:78` | `/v1/system/vector-count` |
| `SystemHandler.UpdateEnv` | `handlers/system.go:134` | `/v1/system/update-env` |
| `SystemHandler.EnvDump` (noop) | `handlers/system.go:503` | `/v1/system/env-dump` |
| `WorkspaceService.{List,Create,Update,Delete,GetBySlug,GetChats,UpdateUsers,ListWorkspaceUsers}` | `workspace_service.go` | most workspace + admin routes |
| `AdminService.{ListUsers,CreateUser,UpdateUser,DeleteUser,ListInvites,CreateInvite,DeactivateInvite}` | `admin_service.go` | all admin user/invite routes |
| `ThreadService.{Create,Update,Delete,GetThreadChats}` | `thread_service.go` | all v1 thread routes |
| `TemporaryAuthTokenService.Issue` | `temporary_auth_token_service.go:25` | `/v1/users/:id/issue-auth-token` |

## 6. OpenAI Compatible Layer Detail

### 6.1 Request Parsing

```go
type OpenAIChatRequest struct {
    Model       string          `json:"model"`       // workspace slug
    Messages    []OpenAIMessage `json:"messages"`
    Temperature *float64        `json:"temperature"`
    Stream      bool            `json:"stream"`
}

type OpenAIMessage struct {
    Role    string `json:"role"`
    Content string `json:"content"` // text or text+attachments
}
```

Parsing logic:
- `model` → workspace slug. Node also accepts `<workspaceSlug>:<threadSlug>` for
  thread-scoped chat completion; v1 Go port honors this by splitting on `:`,
  resolving the thread via `ThreadService.GetBySlug`, and passing
  `threadID != nil` to `ChatService.Complete` / `Stream`.
- Last message with `role == "user"` → prompt.
- Message with `role == "system"` → `req.SystemPromptOverride` (see §5.2).
- All other messages (excluding system) → history.
- `content` supports text + base64 attachments (same as Node.js `extractTextContent` / `extractAttachments`).
- `temperature` from the request → `req.TemperatureOverride`.

### 6.2 Non-Streaming Response

```json
{
  "id": "chatcmpl-uuid",
  "object": "chat.completion",
  "created": 1692851630,
  "model": "workspace-slug",
  "choices": [{
    "index": 0,
    "message": {"role": "assistant", "content": "..."},
    "finish_reason": "stop"
  }]
}
```

### 6.3 Streaming Response (SSE)

```
data: {"id":"...","object":"chat.completion.chunk","created":123,"model":"slug","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}

data: {"id":"...","object":"chat.completion.chunk",...,"delta":{"content":" world"},...}

data: {"id":"...","object":"chat.completion.chunk",...,"delta":{},"finish_reason":"stop"}

data: [DONE]
```

**Implementation gotcha**: do **not** use Gin's `c.SSEvent("message", ...)` — it
emits `event: message\ndata: ...`, and the named event breaks OpenAI client
libraries (`openai-python`, LangChain's `ChatOpenAI`, OpenAI JS SDK), which
expect unnamed `data:` lines. Write raw SSE frames instead:

```go
c.Header("Content-Type", "text/event-stream")
c.Header("Cache-Control", "no-cache")
c.Header("Connection", "keep-alive")

flusher, _ := c.Writer.(http.Flusher)
for chunk := range stream {
    payload, _ := json.Marshal(chunk)
    fmt.Fprintf(c.Writer, "data: %s\n\n", payload)
    flusher.Flush()
}
fmt.Fprint(c.Writer, "data: [DONE]\n\n")
flusher.Flush()
```

Verify with at least one real OpenAI client (LangChain Python or `openai>=1.0`)
in integration tests, not just curl.

## 7. DTO Extensions

New DTOs in `backend/internal/dto/api.go`:

```go
// OpenAI compatible
 type OpenAIChatRequest struct { ... }
 type OpenAIMessage struct { ... }
 type OpenAIEmbeddingRequest struct { ... }

 // API v1 specific
 type APIDocumentUploadRequest struct {
     AddToWorkspaces string `json:"addToWorkspaces"` // comma-delimited slugs (Node parity)
     Metadata        any    `json:"metadata"`
 }
 type APIAdminPreferencesRequest map[string]any
 type APIRawTextRequest struct {
     Text            string `json:"textContent"`
     Title           string `json:"title"`
     Metadata        any    `json:"metadata"`
     AddToWorkspaces string `json:"addToWorkspaces"` // comma-delimited; handler splits before calling service
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
```

> **`addToWorkspaces` typing**: Node accepts a comma-delimited string
> (`"slug1,slug2"`). Keeping `string` here preserves API contract for existing
> integrations; converting to `[]string` would silently break clients.

Most other API v1 routes reuse existing Web DTOs (`CreateWorkspaceRequest`, `ChatRequest`, `UpdateWorkspaceRequest`, etc.) because request bodies are structurally identical.

## 8. Testing Strategy

### 8.1 Unit Tests

- One `*_test.go` per handler file.
- Test matrix per route:
  1. **Success path**: valid API key, valid params → 200 + expected body.
  2. **Auth failure**: missing/invalid API key → 401/403.
  3. **Validation failure**: malformed JSON, missing required fields → 400.
  4. **Not found**: invalid workspace slug, invalid doc name → 404.

- Use `httptest.NewRecorder` + `gin.CreateTestContext`.
- Mock services where appropriate, or use test DB (same pattern as P0 tests).

### 8.2 Integration Tests

- `tests/integration/api_v1_test.go`: end-to-end API v1 tests.
- Key scenarios:
  - Full chat flow: create workspace → upload doc → embed → chat via API v1.
  - OpenAI compatible: send `chat/completions` request, verify response format.
  - Stream test: verify SSE output chunks match OpenAI format.
  - File upload: multipart upload via API key, verify document created.

### 8.3 Test Estimation

| Category | Count |
|----------|-------|
| Unit tests | ~120 |
| Integration tests | ~45 |
| **Total** | **~165** |

## 9. Dependencies & Risks

### Dependencies
- Existing service layer must remain stable (no breaking changes to method signatures).
- `middleware.ValidAPIKey` already implemented and tested.
- `api_embed.go` serves as reference implementation.

### Risks
| Risk | Mitigation |
|------|------------|
| Node.js API v1 behavior drift | Cross-reference each route with Node.js implementation; add integration tests matching Node.js response format. |
| OpenAI stream format edge cases | Test against actual OpenAI client libraries (LangChain Python, `openai>=1.0`, OpenAI JS SDK). See §6.3 SSE gotcha. |
| Large PR review burden | Use the staged PR plan below. |
| API Key role scoping absent (§3.4) | Documented as known gap; mirror Node behavior in v1. Follow-up issue tracks `RequireAPIKeyRole`. |
| Nil-embedder panic on vector routes | Register vector-dependent handlers behind nil-check wrapper; return 503 (§3.3). |
| `addToWorkspaces` shape regression | Keep as comma-delimited `string`, not `[]string` (§7 callout). |

### PR staging

| PR | Scope | Files | Why this slice |
|---|---|---|---|
| 1 | **Service extensions** | `workspace_service.go` (+UpdatePin), `document_service.go` (+RemoveFolder/SaveRawText/PurgeByDocName) + unit tests | Lands the only new business logic; reviewable in isolation; can ship before any handler |
| 2 | **DTO + `ChatRequest` overrides** | `dto/api.go`, `dto/chat.go`, `chat_service.go` (`buildRAGContext` 5-line patch) + unit tests | Small surface; unblocks OpenAI compat |
| 3 | **API v1 trivial handlers** | `api_auth.go`, `api_user.go`, `api_admin.go`, `api_system.go` (22 routes = 1+2+13+6) | Highest-density wrapper layer; mostly mechanical |
| 4 | **API v1 workspace + thread + document handlers** | `api_workspace.go`, `api_thread.go`, `api_document.go` (29 routes) | Larger but cohesive; depends on PR 1 |
| 5 | **OpenAI compat layer** | `api_openai.go` + integration tests against real OpenAI clients | Depends on PR 2; isolated risk surface (SSE format, model:thread parsing) |

Each PR is independently revertable. PR 1 + 2 can land in either order. PRs 3-5 require 1 + 2 merged.

## 10. Success Criteria

- [ ] All 55 routes respond correctly with API Key authentication.
- [ ] OpenAI `/v1/openai/chat/completions` works with both `stream: true` and `stream: false`.
- [ ] `go test ./...` passes with >90% handler coverage.
- [ ] Integration tests verify parity with Node.js API v1 response formats.
- [ ] No regression in existing Web handlers (all existing tests pass).

---

*Design approved: 2026-05-25*
*Revised: 2026-05-25 — post-audit cleanup (M1–M6 + S1–S7):
admin count typo, SSE format bug, dropped redundant service extensions,
clarified registration pattern, API Key scoping gap, PR staging.*
