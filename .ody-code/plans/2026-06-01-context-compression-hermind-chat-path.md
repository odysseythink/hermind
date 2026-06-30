# Context Compression — Hermind Chat Path

> **Local goal:** Wire context compression into Hermind's Regular Chat path (`ChatService.Stream` and `ChatService.Complete`). Adds incremental history reads via `thread_compactions`, a compression gate before LLM invocation, and soft-delete of compressed rows. This sub-plan produces buildable, testable chat-path integration.
> **Depends on file:** `2026-06-01-context-compression-hermind-core.md` (Tasks C1–C4 — adapter layer must exist) and `2026-06-01-context-compression-hermind-models.md` (Tasks M1–M3 — models must exist)

---

## File Structure

| Path | Responsibility |
|---|---|
| `backend/internal/services/chat_service.go` | Incremental `buildChatHistory`; compaction loading in `buildRAGContext`; compression gate in `Stream`/`Complete`; soft-delete + persistence helpers |
| `backend/internal/services/chat_service_test.go` | Existing tests updated for new signatures; new behavioral tests for incremental read and compression gate |
| `backend/cmd/server/main.go` | Wire `CompactionStore` and `SystemService` into `ChatService` constructor |
| `backend/internal/handlers/api_openai_test.go` | Update `NewChatService` call to new arity |
| `backend/internal/handlers/tts_test.go` | Update `NewChatService` call to new arity |
| `backend/internal/agent/e2e_handoff_test.go` | Update `NewChatService` call to new arity |
| `backend/tests/integration/setup_test.go` | Update `NewChatService` call to new arity |
| `backend/tests/integration/chat_management_test.go` | Update `NewChatService` call to new arity |
| `backend/tests/integration/chat_test.go` | Update `NewChatService` call to new arity |

## Dependency Overview

```
Task H1  buildChatHistory signature + incremental behavior
  -> Task H2  ChatService constructor ripple (adds compStore + sysSvc)
       -> Task H3  Compression gate in Stream/Complete + persistence
```

H1 and H2 could theoretically be merged, but H2 is a large shared-signature change that touches 9 files. Keeping H1 separate keeps the incremental-read logic reviewable on its own.

## Risks & Open Questions

| # | Risk | Assumption | Impact if wrong |
|---|---|---|---|
| 1 | Pantheon `PreviousSummary()` missing | We extract summary from compressed message text (parsable prefix) | If upstream P5 changes the prefix string, parser breaks — but prefix change is in Pantheon's assemble.go which we control |
| 2 | `buildRAGContext` now queries `compStore` on every chat | `compStore.SeedForSession` is a single indexed query (fast) | Extra latency per chat request; mitigation: query only when global setting is enabled |
| 3 | Soft-deleting tail chats hides them from UI | Frontend chat list queries do **not** filter on `include` | Users lose visible history for compressed sessions; mitigation noted in index Risk #1 |
| 4 | Compression runs inside the streaming goroutine | `Stream` spawns a goroutine; DB writes inside it are fine (GORM handles per-connection) | Panic in goroutine crashes stream but not server; acceptable for V1 |

---

### Task H1: `buildChatHistory` Incremental Read

**Depends on:** Task C3 (`CompactionStore` exists), Task M1 (`ThreadCompaction` model exists)

**Files:**
- Modify: `backend/internal/services/chat_service.go:306-325`
- Modify: `backend/internal/services/chat_service.go:48` (`buildRAGContext` internal call)
- Modify: `backend/internal/services/chat_service_test.go`

- [ ] **Step 1: Write the failing test**

```go
// backend/internal/services/chat_service_test.go
func TestBuildChatHistory_IncrementalRead(t *testing.T) {
	db := setupChatDB(t)
	cfg := &config.Config{}
	svc := NewChatService(db, cfg, NewVectorService(cfg), nil, nil, nil, nil, nil, nil, nil, nil)

	ws := &models.Workspace{Name: "ws", Slug: "ws"}
	require.NoError(t, db.Create(ws).Error)

	// Insert chats with sequential IDs
	for i := 1; i <= 3; i++ {
		require.NoError(t, db.Create(&models.WorkspaceChat{
			WorkspaceID: ws.ID,
			Prompt:      fmt.Sprintf("q%d", i),
			Response:    fmt.Sprintf("a%d", i),
			Include:     true,
		}).Error)
	}

	// Normal read (afterChatID=0) returns all 3 chats → 6 messages
	history, maxID, err := svc.buildChatHistory(context.Background(), ws.ID, nil, 20, 0)
	require.NoError(t, err)
	assert.Len(t, history, 6)
	assert.Equal(t, 3, maxID)

	// Incremental read after chat 1 returns chats 2 and 3 → 4 messages
	history, maxID, err = svc.buildChatHistory(context.Background(), ws.ID, nil, 20, 1)
	require.NoError(t, err)
	assert.Len(t, history, 4)
	assert.Equal(t, "q2", history[0].Text()) // first message is user prompt of chat 2
	assert.Equal(t, 3, maxID)

	// Incremental read after chat 3 returns nothing
	history, maxID, err = svc.buildChatHistory(context.Background(), ws.ID, nil, 20, 3)
	require.NoError(t, err)
	assert.Len(t, history, 0)
	assert.Equal(t, 0, maxID)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/services/ -run TestBuildChatHistory_IncrementalRead -v`
Expected: FAIL — `buildChatHistory` returns 2 values, not 3

- [ ] **Step 3: Change `buildChatHistory` signature and behavior**

Replace `backend/internal/services/chat_service.go` lines 306–325:

```go
func (s *ChatService) buildChatHistory(ctx context.Context, workspaceID int, threadID *int, limit int, afterChatID int) ([]core.Message, int, error) {
	var chats []models.WorkspaceChat
	query := s.db.Where("workspace_id = ? AND include = ?", workspaceID, true)
	if threadID != nil {
		query = query.Where("thread_id = ?", *threadID)
	} else {
		query = query.Where("thread_id IS NULL")
	}
	if afterChatID > 0 {
		query = query.Where("id > ?", afterChatID)
	}
	if err := query.Order("id DESC").Limit(limit).Find(&chats).Error; err != nil {
		return nil, 0, err
	}

	history := make([]core.Message, 0, len(chats)*2)
	for i := len(chats) - 1; i >= 0; i-- {
		c := chats[i]
		history = append(history, core.NewTextMessage(core.MESSAGE_ROLE_USER, c.Prompt))
		history = append(history, core.NewTextMessage(core.MESSAGE_ROLE_ASSISTANT, c.Response))
	}

	maxChatID := 0
	if len(chats) > 0 {
		maxChatID = chats[0].ID // DESC order: first element has highest ID
	}
	return history, maxChatID, nil
}
```

Update the call site inside `buildRAGContext` (line 48) from:
```go
history, err = s.buildChatHistory(ctx, ws.ID, threadID, historyLimit)
```
to:
```go
history, _, err = s.buildChatHistory(ctx, ws.ID, threadID, historyLimit, 0)
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd backend && go test ./internal/services/ -run TestBuildChatHistory_IncrementalRead -v`
Expected: PASS

- [ ] **Step 5: Whole-tree typecheck**

Run: `cd backend && go vet ./...`
Expected: no errors (`buildRAGContext` call updated, no stale callers)

- [ ] **Step 6: Commit**

```bash
git add backend/internal/services/chat_service.go backend/internal/services/chat_service_test.go
git commit -m "feat(compression): buildChatHistory incremental read with afterChatID"
```

---

### Task H2: ChatService Constructor + Shared Signature Ripple

**Depends on:** Task H1

**Files:**
- Modify: `backend/internal/services/chat_service.go` (struct, constructor, `buildRAGContext`)
- Modify: `backend/cmd/server/main.go:253`
- Modify: `backend/internal/services/chat_service_test.go` (all `NewChatService` calls)
- Modify: `backend/internal/handlers/api_openai_test.go:50`
- Modify: `backend/internal/handlers/tts_test.go:48`
- Modify: `backend/internal/agent/e2e_handoff_test.go:42`
- Modify: `backend/tests/integration/setup_test.go:89`
- Modify: `backend/tests/integration/chat_management_test.go:39`
- Modify: `backend/tests/integration/chat_test.go:67`

- [ ] **Step 1: Change the ChatService struct + constructor + write test**

Modify `backend/internal/services/chat_service.go`:

1. Add imports (if not already present):
```go
import (
    // existing imports...
    agentcompression "github.com/odysseythink/hermind/backend/internal/agent/compression"
)
```

2. Add fields to `ChatService` struct (after `autoTitleSvc`):
```go
	compStore *agentcompression.CompactionStore
	sysSvc    *SystemService
```

3. Update `NewChatService` signature and body:
```go
func NewChatService(db *gorm.DB, cfg *config.Config, vectorSvc *VectorService, llmProv providers.LLMProvider, embedder embedder.Embedder, agentInvoker AgentInvoker, reranker reranker.Reranker, memInj *MemoryInjector, autoTitleSvc *AutoTitleService, compStore *agentcompression.CompactionStore, sysSvc *SystemService) *ChatService {
	return &ChatService{db: db, cfg: cfg, vectorSvc: vectorSvc, llmProv: llmProv, embedder: embedder, agentInvoker: agentInvoker, reranker: reranker, memInj: memInj, autoTitleSvc: autoTitleSvc, compStore: compStore, sysSvc: sysSvc}
}
```

4. Update `buildRAGContext` to load compaction and prepend summary. Inside the `historyOverride == nil` branch (around line 44–52):

```go
		var afterChatID int
		var summary string
		if s.compStore != nil {
			summary, afterChatID, _ = s.compStore.SeedForSession(ws.ID, threadID)
		}
		history, _, err = s.buildChatHistory(ctx, ws.ID, threadID, historyLimit, afterChatID)
		if err != nil {
			return "", nil, nil, err
		}
		if summary != "" {
			history = append([]core.Message{core.NewTextMessage(core.MESSAGE_ROLE_ASSISTANT, summary)}, history...)
		}
```

- [ ] **Step 2: Find and update EVERY caller (prod + tests)**

Run: `grep -rn "NewChatService(" backend/`

Expected files to change (pass `nil, nil` for new params in tests):

| File | Current Call | New Call |
|---|---|---|
| `backend/cmd/server/main.go:253` | `NewChatService(db, cfg, vectorSvc, llmProv, emb, agentRuntime, rerankerSvc, memInj, autoTitleSvc)` | `NewChatService(db, cfg, vectorSvc, llmProv, emb, agentRuntime, rerankerSvc, memInj, autoTitleSvc, compStore, sysSvc)` |
| `backend/internal/services/chat_service_test.go` | `NewChatService(db, cfg, vec, nil, nil, nil, nil, nil, nil)` | `NewChatService(db, cfg, vec, nil, nil, nil, nil, nil, nil, nil, nil)` |
| `backend/internal/handlers/api_openai_test.go:50` | `NewChatService(env.DB, cfg, vec, &mockLLM{text: llmText}, nil, nil, nil, nil, nil)` | `NewChatService(env.DB, cfg, vec, &mockLLM{text: llmText}, nil, nil, nil, nil, nil, nil, nil)` |
| `backend/internal/handlers/tts_test.go:48` | `NewChatService(db, cfg, nil, nil, nil, nil, nil, nil, nil)` | `NewChatService(db, cfg, nil, nil, nil, nil, nil, nil, nil, nil, nil)` |
| `backend/internal/agent/e2e_handoff_test.go:42` | `NewChatService(db, cfg, vec, nil, nil, rt, nil, nil, nil)` | `NewChatService(db, cfg, vec, nil, nil, rt, nil, nil, nil, nil, nil)` |
| `backend/tests/integration/setup_test.go:89` | `NewChatService(db, cfg, vectorSvc, llm, nil, nil, nil, nil, nil)` | `NewChatService(db, cfg, vectorSvc, llm, nil, nil, nil, nil, nil, nil, nil)` |
| `backend/tests/integration/chat_management_test.go:39` | `NewChatService(db, cfg, nil, nil, nil, nil, nil, nil, nil)` | `NewChatService(db, cfg, nil, nil, nil, nil, nil, nil, nil, nil, nil)` |
| `backend/tests/integration/chat_test.go:67` | `NewChatService(db, cfg, vectorSvc, llm, nil, nil, nil, nil, nil)` | `NewChatService(db, cfg, vectorSvc, llm, nil, nil, nil, nil, nil, nil, nil)` |

In `backend/cmd/server/main.go`, add before `chatSvc` creation (around line 252):
```go
	compStore := agentcompression.NewCompactionStore(db)
```
And pass `compStore, sysSvc` to `NewChatService`.

Also add import for `agentcompression` in `main.go`:
```go
	agentcompression "github.com/odysseythink/hermind/backend/internal/agent/compression"
```

- [ ] **Step 3: Write constructor + integration test**

```go
// backend/internal/services/chat_service_test.go
func TestBuildRAGContext_IncrementalReadWithCompaction(t *testing.T) {
	db := setupChatDB(t)
	cfg := &config.Config{}
	compStore := agentcompression.NewCompactionStore(db)
	// Create sysSvc and seed the global setting
	sysSvc := NewSystemService(db)
	require.NoError(t, sysSvc.SetSetting(context.Background(), "context_compress_enabled", "true"))

	svc := NewChatService(db, cfg, NewVectorService(cfg), nil, nil, nil, nil, nil, nil, compStore, sysSvc)

	ws := &models.Workspace{Name: "ws", Slug: "ws"}
	require.NoError(t, db.Create(ws).Error)

	// Insert 3 chats
	for i := 1; i <= 3; i++ {
		require.NoError(t, db.Create(&models.WorkspaceChat{
			WorkspaceID: ws.ID,
			Prompt:      fmt.Sprintf("q%d", i),
			Response:    fmt.Sprintf("a%d", i),
			Include:     true,
		}).Error)
	}

	// Create compaction up to chat 1
	require.NoError(t, compStore.Save(&models.ThreadCompaction{
		WorkspaceID: ws.ID,
		ThreadID:    nil,
		Summary:     "Summary of chat 1",
		UpToChatID:  1,
	}))

	_, _, history, err := svc.buildRAGContext(context.Background(), ws, nil, nil, "hi", nil, nil)
	require.NoError(t, err)
	require.Len(t, history, 5) // summary + 2 chats (4 messages)
	assert.Equal(t, "Summary of chat 1", history[0].Text())
}
```

- [ ] **Step 4: Run targeted tests**

Run: `cd backend && go test ./internal/services/ -run TestBuildRAGContext_IncrementalReadWithCompaction -v`
Expected: PASS

- [ ] **Step 5: Whole-tree typecheck (incl. tests)**

Run: `cd backend && go vet ./...`
Expected: no errors — proves no stale caller anywhere, including `_test.go` files

- [ ] **Step 6: Commit**

```bash
git add backend/internal/services/chat_service.go backend/internal/services/chat_service_test.go backend/cmd/server/main.go backend/internal/handlers/api_openai_test.go backend/internal/handlers/tts_test.go backend/internal/agent/e2e_handoff_test.go backend/tests/integration/setup_test.go backend/tests/integration/chat_management_test.go backend/tests/integration/chat_test.go
git commit -m "refactor: extend ChatService with compStore + sysSvc + incremental RAG context"
```

---

### Task H3: Chat Compression Gate + Persistence

**Depends on:** Task H2

**Files:**
- Modify: `backend/internal/services/chat_service.go` (`Stream`, `Complete`, new helpers)
- Modify: `backend/internal/services/chat_service_test.go`

- [ ] **Step 1: Write the failing test**

```go
// backend/internal/services/chat_service_test.go

func TestChatService_tryCompressHistory_Disabled(t *testing.T) {
	db := setupChatDB(t)
	cfg := &config.Config{}
	svc := NewChatService(db, cfg, NewVectorService(cfg), nil, nil, nil, nil, nil, nil, nil, nil)

	ws := &models.Workspace{Name: "ws", Slug: "ws"}
	require.NoError(t, db.Create(ws).Error)

	history := []core.Message{core.NewTextMessage(core.MESSAGE_ROLE_USER, "hello")}
	result, err := svc.tryCompressHistory(context.Background(), ws, nil, history)
	require.NoError(t, err)
	assert.Len(t, result, 1)
}

func TestChatService_saveCompactionAndSoftDelete(t *testing.T) {
	db := setupChatDB(t)
	cfg := &config.Config{}
	compStore := agentcompression.NewCompactionStore(db)
	svc := NewChatService(db, cfg, NewVectorService(cfg), nil, nil, nil, nil, nil, nil, compStore, nil)

	ws := &models.Workspace{Name: "ws", Slug: "ws"}
	require.NoError(t, db.Create(ws).Error)

	// Insert 3 chats
	for i := 1; i <= 3; i++ {
		require.NoError(t, db.Create(&models.WorkspaceChat{
			WorkspaceID: ws.ID,
			Prompt:      fmt.Sprintf("q%d", i),
			Response:    fmt.Sprintf("a%d", i),
			Include:     true,
		}).Error)
	}

	require.NoError(t, svc.saveCompactionAndSoftDelete(context.Background(), ws.ID, nil, "Test summary"))

	// Compaction should exist
	c, err := compStore.LoadLatest(ws.ID, nil)
	require.NoError(t, err)
	require.NotNil(t, c)
	assert.Equal(t, "Test summary", c.Summary)
	assert.Equal(t, 3, c.UpToChatID)

	// Chats 1–3 should be soft-deleted
	var includedCount int64
	require.NoError(t, db.Model(&models.WorkspaceChat{}).
		Where("workspace_id = ? AND include = ?", ws.ID, true).
		Where("thread_id IS NULL").
		Count(&includedCount).Error)
	assert.Equal(t, int64(0), includedCount)
}

func TestExtractSummaryFromCompressed(t *testing.T) {
	prefix := "[Compressed summary of earlier conversation]\n"
	msgs := []core.Message{
		core.NewTextMessage(core.MESSAGE_ROLE_ASSISTANT, prefix+"The user asked about Go."),
	}
	assert.Equal(t, "The user asked about Go.", extractSummaryFromCompressed(msgs))

	// No prefix
	msgs2 := []core.Message{
		core.NewTextMessage(core.MESSAGE_ROLE_ASSISTANT, "Just a normal response"),
	}
	assert.Equal(t, "", extractSummaryFromCompressed(msgs2))

	// Empty
	assert.Equal(t, "", extractSummaryFromCompressed(nil))
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd backend && go test ./internal/services/ -run TestChatService_tryCompressHistory_Disabled -v`
Expected: FAIL — `tryCompressHistory` not defined

Run: `cd backend && go test ./internal/services/ -run TestChatService_saveCompactionAndSoftDelete -v`
Expected: FAIL — `saveCompactionAndSoftDelete` not defined

Run: `cd backend && go test ./internal/services/ -run TestExtractSummaryFromCompressed -v`
Expected: FAIL — `extractSummaryFromCompressed` not defined

- [ ] **Step 3: Implement compression gate and helpers**

Add the following helpers to `backend/internal/services/chat_service.go` (before `buildChatHistory` or after `saveChatResponse`):

```go
// tryCompressHistory attempts to compress the conversation history before
// sending it to the LLM. If compression is disabled, the aux model is nil,
// or the threshold is not exceeded, the original history is returned unchanged.
func (s *ChatService) tryCompressHistory(ctx context.Context, ws *models.Workspace, threadID *int, history []core.Message) ([]core.Message, error) {
	if s.compStore == nil || s.sysSvc == nil {
		return history, nil
	}
	globalEnabledStr, _ := s.sysSvc.GetSetting(ctx, "context_compress_enabled")
	globalEnabled := globalEnabledStr == "true"
	if !agentcompression.IsEnabledForWorkspace(globalEnabled, ws.CompressEnabled) {
		return history, nil
	}
	comp := agentcompression.NewForChat(s.llmProv.LanguageModel(), ws, s.compStore)
	if comp == nil {
		return history, nil
	}
	compressed, err := comp.CompressMessages(ctx, history, "")
	if err != nil {
		mlog.Warning("ChatService: compression failed: ", err)
		return history, nil
	}
	summary := extractSummaryFromCompressed(compressed)
	if summary != "" {
		if err := s.saveCompactionAndSoftDelete(ctx, ws.ID, threadID, summary); err != nil {
			mlog.Warning("ChatService: compaction persistence failed: ", err)
		}
	}
	return compressed, nil
}

// saveCompactionAndSoftDelete persists the summary and soft-deletes the
// compressed chat rows so they are not re-read in future turns.
func (s *ChatService) saveCompactionAndSoftDelete(ctx context.Context, workspaceID int, threadID *int, summary string) error {
	maxChatID, err := s.maxChatIDForThread(ctx, workspaceID, threadID)
	if err != nil || maxChatID == 0 {
		return err
	}
	if err := s.compStore.Save(&models.ThreadCompaction{
		WorkspaceID: workspaceID,
		ThreadID:    threadID,
		Summary:     summary,
		UpToChatID:  maxChatID,
	}); err != nil {
		return fmt.Errorf("save compaction: %w", err)
	}
	if err := s.softDeleteChatsUpTo(ctx, workspaceID, threadID, maxChatID); err != nil {
		return fmt.Errorf("soft-delete chats: %w", err)
	}
	return nil
}

func (s *ChatService) maxChatIDForThread(ctx context.Context, workspaceID int, threadID *int) (int, error) {
	var maxID int
	q := s.db.Model(&models.WorkspaceChat{}).Select("COALESCE(MAX(id), 0)").
		Where("workspace_id = ? AND include = ?", workspaceID, true)
	if threadID != nil {
		q = q.Where("thread_id = ?", *threadID)
	} else {
		q = q.Where("thread_id IS NULL")
	}
	if err := q.Scan(&maxID).Error; err != nil {
		return 0, err
	}
	return maxID, nil
}

func (s *ChatService) softDeleteChatsUpTo(ctx context.Context, workspaceID int, threadID *int, upToChatID int) error {
	q := s.db.Model(&models.WorkspaceChat{}).
		Where("workspace_id = ? AND id <= ?", workspaceID, upToChatID)
	if threadID != nil {
		q = q.Where("thread_id = ?", *threadID)
	} else {
		q = q.Where("thread_id IS NULL")
	}
	return q.Update("include", false).Error
}

func extractSummaryFromCompressed(msgs []core.Message) string {
	const prefix = "[Compressed summary of earlier conversation]\n"
	for _, m := range msgs {
		if m.Role != core.MESSAGE_ROLE_ASSISTANT {
			continue
		}
		text := m.Text()
		if idx := strings.Index(text, prefix); idx >= 0 {
			return text[idx+len(prefix):]
		}
	}
	return ""
}
```

Now wire `tryCompressHistory` into `Stream` and `Complete`.

In `Stream` (around line 186, after `buildRAGContext` and before adding the current user message):

```go
		systemPrompt, sources, history, err := s.buildRAGContext(ctx, ws, user, threadID, req.Message, req.SystemPromptOverride, req.HistoryOverride)
		if err != nil {
			// ... existing error handling ...
		}
		mlog.Info("ChatService.Stream: built history with ", len(history), " messages")

		// Compression gate
		history, err = s.tryCompressHistory(ctx, ws, threadID, history)
		if err != nil {
			mlog.Warning("ChatService.Stream: compression error: ", err)
		}
```

In `Complete` (around line 276, after `buildRAGContext`):

```go
		systemPrompt, sources, history, err := s.buildRAGContext(ctx, ws, user, threadID, req.Message, req.SystemPromptOverride, req.HistoryOverride)
		if err != nil {
			return &dto.ChatResponse{ID: msgID, Type: "abort", Close: true, Error: err.Error()}, nil
		}

		// Compression gate
		history, err = s.tryCompressHistory(ctx, ws, threadID, history)
		if err != nil {
			mlog.Warning("ChatService.Complete: compression error: ", err)
		}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd backend && go test ./internal/services/ -run TestChatService_tryCompressHistory_Disabled -v`
Expected: PASS

Run: `cd backend && go test ./internal/services/ -run TestChatService_saveCompactionAndSoftDelete -v`
Expected: PASS

Run: `cd backend && go test ./internal/services/ -run TestExtractSummaryFromCompressed -v`
Expected: PASS

- [ ] **Step 5: Whole-tree typecheck**

Run: `cd backend && go vet ./...`
Expected: no errors

- [ ] **Step 6: Commit**

```bash
git add backend/internal/services/chat_service.go backend/internal/services/chat_service_test.go
git commit -m "feat(compression): add chat compression gate with persistence and soft-delete"
```

---

## Self-Review

- [ ] **1. Spec coverage**

| Design § | Requirement | Task | Status |
|---|---|---|---|
| §1.1 | Regular Chat path compression | H3 | covered |
| §11.1 | `buildChatHistory` incremental read | H1, H2 | covered |
| §11.2 | Chat compression gate | H3 | covered |
| §8 | Degradation (nil return when disabled) | H3 | covered |
| §5 | Config defaults (Agent 0.50 / Chat 0.75) | H3 (via factory) | covered |

- [ ] **2. Placeholder scan:** No `TODO`, `TBD`, or deferred-by-dependency placeholders. Summary extraction parses message text (works with current Pantheon); upstream `PreviousSummary()` accessor is a future optimization, not a blocker.
- [ ] **3. No phantom tasks:** Every task creates verifiable changes. H1 adds incremental read behavior + test. H2 is a large but real signature refactor. H3 adds gate logic + DB mutations + tests.
- [ ] **4. Dependency soundness:** H1 → H2 → H3. H1 changes `buildChatHistory` signature (localized). H2 depends on H1. H3 depends on H2 (needs `compStore`/`sysSvc` fields). No task references unfinished external work.
- [ ] **5. Caller & build soundness:** H2 changes `NewChatService` (9 callers updated including `_test.go` files). H1 changes `buildChatHistory` (1 caller updated). Both end with `go vet ./...` to typecheck test files too. `NewChatService` is changed in exactly one task (H2).
- [ ] **6. Test-the-risk:** H1 tests incremental read boundary (afterChatID filtering). H2 tests compaction loading + summary prepending. H3 tests DB mutations (soft-delete, compaction save) and summary extraction.
- [ ] **7. Type consistency:** `buildChatHistory` returns `([]core.Message, int, error)` — `int` is max chat ID, used consistently. `NewChatService` receives `*agentcompression.CompactionStore` and `*SystemService` — matching types defined in C3 and existing services. `IsEnabledForWorkspace` takes `(bool, *bool)` matching C4.
