# Memories System Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use gpowers:subagent-driven-development (recommended) or gpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Independent long-term memory store (mirroring anything-llm's `memories` table) — per-user, per-(workspace|global) scope, capped, auto-injected into chat system prompt, with optional periodic Observer→Reflector LLM extractor.

**Architecture:** New `memories` table + `MemoryService` (CRUD + limit enforcement + transactional batch-apply) + `MemoryInjector` hook called from `ChatService.buildRAGContext` between system-prompt resolution and the RAG context concat. PR2 adds a `workspace_chats.memory_processed` column and a 3-hour cron worker that runs a two-phase LLM extraction (Observer extracts candidates; Reflector classifies + dedupes).

**Tech Stack:** Go 1.23, Gin, GORM, existing `reranker.Reranker` abstraction, `pantheon/core` LLM client, `robfig/cron/v3` via existing `workers.Manager`.

**Source design:** `.gpowers/designs/2026-05-28-memories-system-design.md`

---

## File Structure

### Created
- `backend/internal/models/memory.go` — GORM model
- `backend/internal/services/memory_service.go` — CRUD + ApplyExtracted
- `backend/internal/services/memory_service_test.go`
- `backend/internal/services/memory_injector.go` — `PromptWithMemories` hook
- `backend/internal/services/memory_injector_test.go`
- `backend/internal/handlers/memory.go` — 7 routes
- `backend/internal/handlers/memory_test.go`
- `backend/internal/services/memory_extractor.go` (PR2) — Observer+Reflector
- `backend/internal/services/memory_extractor_test.go` (PR2)
- `backend/internal/workers/extract_memories.go` (PR2) — `workers.Job` impl
- `backend/prompts/memory_observer.txt` (PR2) — extractor system prompt
- `backend/prompts/memory_reflector.txt` (PR2) — reflector system prompt

### Modified
- `backend/internal/services/db.go` — append `&models.Memory{}` to `AutoMigrate` (after `&models.PromptHistory{}`)
- `backend/internal/services/chat_service.go:53-57` — call `MemoryInjector.PromptWithMemories`
- `backend/internal/services/chat_service.go:34` — accept `*MemoryInjector` in `NewChatService`
- `backend/internal/models/workspace_chat.go` — add `MemoryProcessed *bool` (PR2 only)
- `backend/cmd/server/main.go` — wire `MemoryService`, `MemoryInjector` (PR1); add `extract-memories` worker (PR2)

---

# PR1 · CRUD + injection (~600 LoC)

## Task 1: Memory model + AutoMigrate

**Files:**
- Create: `backend/internal/models/memory.go`
- Modify: `backend/internal/services/db.go` (append to AutoMigrate list)

- [ ] **Step 1: Write the model**

```go
package models

import "time"

// Memory scopes.
const (
	MemoryScopeWorkspace = "workspace"
	MemoryScopeGlobal    = "global"
)

// Limits — match anything-llm exactly.
const (
	GlobalMemoryLimit         = 5
	WorkspaceMemoryLimit      = 20
	MaxInjectedWorkspaceLimit = 5
)

type Memory struct {
	ID          int        `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID      *int       `gorm:"index:idx_mem_user_ws;index:idx_mem_user_scope" json:"userId"`
	WorkspaceID *int       `gorm:"index:idx_mem_user_ws" json:"workspaceId"`
	Scope       string     `gorm:"not null;default:workspace;index:idx_mem_user_scope" json:"scope"`
	Content     string     `gorm:"type:text;not null" json:"content"`
	LastUsedAt  *time.Time `json:"lastUsedAt,omitempty"`
	CreatedAt   time.Time  `gorm:"autoCreateTime" json:"createdAt"`
	UpdatedAt   time.Time  `gorm:"autoUpdateTime" json:"updatedAt"`
}

func (Memory) TableName() string { return "memories" }
```

- [ ] **Step 2: Register in AutoMigrate**

In `backend/internal/services/db.go`, append after `&models.PromptHistory{}`:

```go
&models.Memory{},
```

- [ ] **Step 3: Build**

```bash
cd backend && go build ./...
```

Expected: no errors.

## Task 2: MemoryService — failing tests first

**Files:**
- Create: `backend/internal/services/memory_service_test.go`

- [ ] **Step 1: Write tests**

```go
package services

import (
	"context"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newMemTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Memory{}))
	return db
}

func TestMemoryService_CreateAndList(t *testing.T) {
	svc := NewMemoryService(newMemTestDB(t))
	uid, wid := 1, 10
	_, err := svc.Create(context.Background(), &uid, &wid, models.MemoryScopeWorkspace, "fact A")
	require.NoError(t, err)
	_, err = svc.Create(context.Background(), &uid, &wid, models.MemoryScopeWorkspace, "fact B")
	require.NoError(t, err)

	rows, err := svc.ListWorkspace(context.Background(), &uid, wid)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.Equal(t, "fact B", rows[0].Content) // DESC by createdAt
}

func TestMemoryService_GlobalLimit(t *testing.T) {
	svc := NewMemoryService(newMemTestDB(t))
	uid := 7
	for i := 0; i < models.GlobalMemoryLimit; i++ {
		_, err := svc.Create(context.Background(), &uid, nil, models.MemoryScopeGlobal, "g")
		require.NoError(t, err)
	}
	_, err := svc.Create(context.Background(), &uid, nil, models.MemoryScopeGlobal, "g")
	assert.ErrorIs(t, err, ErrMemoryLimitReached)
}

func TestMemoryService_PromoteAndDemote(t *testing.T) {
	svc := NewMemoryService(newMemTestDB(t))
	uid, wid := 3, 5
	m, _ := svc.Create(context.Background(), &uid, &wid, models.MemoryScopeWorkspace, "x")

	promoted, err := svc.PromoteToGlobal(context.Background(), m.ID)
	require.NoError(t, err)
	assert.Equal(t, models.MemoryScopeGlobal, promoted.Scope)
	assert.Nil(t, promoted.WorkspaceID)

	demoted, err := svc.DemoteToWorkspace(context.Background(), m.ID, wid)
	require.NoError(t, err)
	assert.Equal(t, models.MemoryScopeWorkspace, demoted.Scope)
	require.NotNil(t, demoted.WorkspaceID)
	assert.Equal(t, wid, *demoted.WorkspaceID)
}

func TestMemoryService_ApplyExtracted(t *testing.T) {
	svc := NewMemoryService(newMemTestDB(t))
	uid, wid := 1, 1
	existing, _ := svc.Create(context.Background(), &uid, &wid, models.MemoryScopeWorkspace, "old")

	res, err := svc.ApplyExtracted(context.Background(), &uid, wid, []ExtractedAction{
		{Action: "create", Scope: "WORKSPACE", Content: "new ws"},
		{Action: "create", Scope: "GLOBAL", Content: "new global"},
		{Action: "update", UpdateID: &existing.ID, Content: "old revised"},
	}, models.GlobalMemoryLimit)
	require.NoError(t, err)
	assert.Equal(t, 1, res.WS)
	assert.Equal(t, 1, res.Global)
	assert.Equal(t, 1, res.Updated)

	rows, _ := svc.ListWorkspace(context.Background(), &uid, wid)
	var found bool
	for _, r := range rows {
		if r.ID == existing.ID && r.Content == "old revised" {
			found = true
		}
	}
	assert.True(t, found)
}

func TestMemoryService_ReplaceWorkspace_Transactional(t *testing.T) {
	svc := NewMemoryService(newMemTestDB(t))
	uid, wid := 1, 1
	for i := 0; i < 3; i++ {
		_, _ = svc.Create(context.Background(), &uid, &wid, models.MemoryScopeWorkspace, "x")
	}
	require.NoError(t, svc.ReplaceWorkspace(context.Background(), &uid, wid, []string{"a", "b"}))
	rows, _ := svc.ListWorkspace(context.Background(), &uid, wid)
	assert.Len(t, rows, 2)
}
```

- [ ] **Step 2: Run — expect compile error**

```bash
cd backend && go test ./internal/services/ -run TestMemoryService
```

Expected: undefined `NewMemoryService`, `ErrMemoryLimitReached`, `ExtractedAction`.

## Task 3: Implement MemoryService

**Files:**
- Create: `backend/internal/services/memory_service.go`

- [ ] **Step 1: Write the service**

```go
package services

import (
	"context"
	"errors"
	"time"

	"github.com/odysseythink/hermind/backend/internal/models"
	"gorm.io/gorm"
)

var (
	ErrMemoryLimitReached = errors.New("memory limit reached")
	ErrMemoryNotFound     = errors.New("memory not found")
)

type MemoryService struct {
	db *gorm.DB
}

func NewMemoryService(db *gorm.DB) *MemoryService {
	return &MemoryService{db: db}
}

type ExtractedAction struct {
	Action   string // "create" or "update"
	Scope    string // "WORKSPACE" or "GLOBAL" (uppercase from LLM)
	Content  string
	UpdateID *int // populated when Action == "update"
}

type ApplyResult struct {
	WS, Global, Updated int
}

func (s *MemoryService) countForScope(ctx context.Context, userID *int, workspaceID *int, scope string) (int64, error) {
	q := s.db.WithContext(ctx).Model(&models.Memory{}).Where("scope = ?", scope)
	q = applyUser(q, userID)
	if scope == models.MemoryScopeWorkspace {
		q = applyWorkspace(q, workspaceID)
	}
	var count int64
	return count, q.Count(&count).Error
}

func applyUser(q *gorm.DB, userID *int) *gorm.DB {
	if userID == nil {
		return q.Where("user_id IS NULL")
	}
	return q.Where("user_id = ?", *userID)
}

func applyWorkspace(q *gorm.DB, wsID *int) *gorm.DB {
	if wsID == nil {
		return q.Where("workspace_id IS NULL")
	}
	return q.Where("workspace_id = ?", *wsID)
}

func (s *MemoryService) Create(ctx context.Context, userID *int, workspaceID *int, scope, content string) (*models.Memory, error) {
	if scope != models.MemoryScopeWorkspace && scope != models.MemoryScopeGlobal {
		return nil, errors.New("invalid scope")
	}
	limit := models.WorkspaceMemoryLimit
	if scope == models.MemoryScopeGlobal {
		limit = models.GlobalMemoryLimit
	}
	count, err := s.countForScope(ctx, userID, workspaceID, scope)
	if err != nil {
		return nil, err
	}
	if count >= int64(limit) {
		return nil, ErrMemoryLimitReached
	}
	m := &models.Memory{
		UserID:      userID,
		WorkspaceID: workspaceID,
		Scope:       scope,
		Content:     content,
	}
	if scope == models.MemoryScopeGlobal {
		m.WorkspaceID = nil
	}
	if err := s.db.WithContext(ctx).Create(m).Error; err != nil {
		return nil, err
	}
	return m, nil
}

func (s *MemoryService) Update(ctx context.Context, id int, content string) (*models.Memory, error) {
	if err := s.db.WithContext(ctx).Model(&models.Memory{}).
		Where("id = ?", id).
		Updates(map[string]any{"content": content, "updated_at": time.Now()}).Error; err != nil {
		return nil, err
	}
	return s.Get(ctx, id)
}

func (s *MemoryService) Delete(ctx context.Context, id int) error {
	return s.db.WithContext(ctx).Delete(&models.Memory{}, id).Error
}

func (s *MemoryService) Get(ctx context.Context, id int) (*models.Memory, error) {
	var m models.Memory
	if err := s.db.WithContext(ctx).First(&m, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrMemoryNotFound
		}
		return nil, err
	}
	return &m, nil
}

func (s *MemoryService) ListWorkspace(ctx context.Context, userID *int, workspaceID int) ([]models.Memory, error) {
	var rows []models.Memory
	q := s.db.WithContext(ctx).Where("workspace_id = ? AND scope = ?", workspaceID, models.MemoryScopeWorkspace).
		Order("created_at DESC")
	q = applyUser(q, userID)
	err := q.Find(&rows).Error
	return rows, err
}

func (s *MemoryService) ListGlobal(ctx context.Context, userID *int) ([]models.Memory, error) {
	var rows []models.Memory
	q := s.db.WithContext(ctx).Where("scope = ?", models.MemoryScopeGlobal).Order("created_at DESC")
	q = applyUser(q, userID)
	err := q.Find(&rows).Error
	return rows, err
}

func (s *MemoryService) PromoteToGlobal(ctx context.Context, id int) (*models.Memory, error) {
	m, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if m.Scope == models.MemoryScopeGlobal {
		return m, nil
	}
	count, err := s.countForScope(ctx, m.UserID, nil, models.MemoryScopeGlobal)
	if err != nil {
		return nil, err
	}
	if count >= int64(models.GlobalMemoryLimit) {
		return nil, ErrMemoryLimitReached
	}
	if err := s.db.WithContext(ctx).Model(&models.Memory{}).Where("id = ?", id).
		Updates(map[string]any{
			"scope":        models.MemoryScopeGlobal,
			"workspace_id": nil,
			"updated_at":   time.Now(),
		}).Error; err != nil {
		return nil, err
	}
	return s.Get(ctx, id)
}

func (s *MemoryService) DemoteToWorkspace(ctx context.Context, id, workspaceID int) (*models.Memory, error) {
	m, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if m.Scope == models.MemoryScopeWorkspace {
		return m, nil
	}
	count, err := s.countForScope(ctx, m.UserID, &workspaceID, models.MemoryScopeWorkspace)
	if err != nil {
		return nil, err
	}
	if count >= int64(models.WorkspaceMemoryLimit) {
		return nil, ErrMemoryLimitReached
	}
	if err := s.db.WithContext(ctx).Model(&models.Memory{}).Where("id = ?", id).
		Updates(map[string]any{
			"scope":        models.MemoryScopeWorkspace,
			"workspace_id": workspaceID,
			"updated_at":   time.Now(),
		}).Error; err != nil {
		return nil, err
	}
	return s.Get(ctx, id)
}

func (s *MemoryService) UpdateLastUsed(ctx context.Context, ids []int) error {
	if len(ids) == 0 {
		return nil
	}
	return s.db.WithContext(ctx).Model(&models.Memory{}).
		Where("id IN ?", ids).Update("last_used_at", time.Now()).Error
}

// ReplaceWorkspace transactionally deletes all workspace-scoped memories for
// (user, workspace) and inserts up to WorkspaceMemoryLimit new contents.
func (s *MemoryService) ReplaceWorkspace(ctx context.Context, userID *int, workspaceID int, contents []string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		q := tx.Where("workspace_id = ? AND scope = ?", workspaceID, models.MemoryScopeWorkspace)
		q = applyUser(q, userID)
		if err := q.Delete(&models.Memory{}).Error; err != nil {
			return err
		}
		for i, c := range contents {
			if i >= models.WorkspaceMemoryLimit {
				break
			}
			if err := tx.Create(&models.Memory{
				UserID: userID, WorkspaceID: &workspaceID,
				Scope: models.MemoryScopeWorkspace, Content: c,
			}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// ApplyExtracted atomically applies a batch of Observer/Reflector decisions.
func (s *MemoryService) ApplyExtracted(ctx context.Context, userID *int, workspaceID int, actions []ExtractedAction, globalSlots int) (ApplyResult, error) {
	var result ApplyResult
	// Split + cap
	wsCreates, glCreates, updates := splitActions(actions, globalSlots)
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, a := range wsCreates {
			if err := tx.Create(&models.Memory{
				UserID: userID, WorkspaceID: &workspaceID,
				Scope: models.MemoryScopeWorkspace, Content: a.Content,
			}).Error; err != nil {
				return err
			}
		}
		for _, a := range glCreates {
			if err := tx.Create(&models.Memory{
				UserID: userID, Scope: models.MemoryScopeGlobal, Content: a.Content,
			}).Error; err != nil {
				return err
			}
		}
		for _, a := range updates {
			if err := tx.Model(&models.Memory{}).Where("id = ?", *a.UpdateID).
				Updates(map[string]any{"content": a.Content, "updated_at": time.Now()}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	result.WS = len(wsCreates)
	result.Global = len(glCreates)
	result.Updated = len(updates)
	return result, err
}

func splitActions(actions []ExtractedAction, globalSlots int) (ws, gl, upd []ExtractedAction) {
	for _, a := range actions {
		switch {
		case a.Action == "update" && a.UpdateID != nil:
			upd = append(upd, a)
		case a.Action == "create" && a.Scope == "WORKSPACE":
			if len(ws) < models.WorkspaceMemoryLimit {
				ws = append(ws, a)
			}
		case a.Action == "create" && a.Scope == "GLOBAL":
			if len(gl) < globalSlots {
				gl = append(gl, a)
			}
		}
	}
	return
}
```

- [ ] **Step 2: Run tests — expect PASS**

```bash
cd backend && go test ./internal/services/ -run TestMemoryService -v
```

Expected: 5 tests PASS.

## Task 4: MemoryInjector — failing tests

**Files:**
- Create: `backend/internal/services/memory_injector_test.go`

- [ ] **Step 1: Write tests**

```go
package services

import (
	"context"
	"strings"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/reranker"
	"github.com/odysseythink/pantheon/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryInjector_NoMemories_ReturnsBase(t *testing.T) {
	memSvc := NewMemoryService(newMemTestDB(t))
	inj := NewMemoryInjector(memSvc, &fakeSettings{enabled: true}, &reranker.NoopReranker{})

	out := inj.PromptWithMemories(context.Background(), "base prompt", nil, 1, "q", nil)
	assert.Equal(t, "base prompt", out)
}

func TestMemoryInjector_DisabledReturnsBase(t *testing.T) {
	db := newMemTestDB(t)
	memSvc := NewMemoryService(db)
	uid := 1
	_, _ = memSvc.Create(context.Background(), &uid, intPtr(1), models.MemoryScopeWorkspace, "x")

	inj := NewMemoryInjector(memSvc, &fakeSettings{enabled: false}, &reranker.NoopReranker{})
	out := inj.PromptWithMemories(context.Background(), "base", &uid, 1, "q", nil)
	assert.Equal(t, "base", out)
}

func TestMemoryInjector_EnabledRenders(t *testing.T) {
	db := newMemTestDB(t)
	memSvc := NewMemoryService(db)
	uid := 1
	_, _ = memSvc.Create(context.Background(), &uid, intPtr(1), models.MemoryScopeWorkspace, "ws fact")
	_, _ = memSvc.Create(context.Background(), &uid, nil, models.MemoryScopeGlobal, "global fact")

	inj := NewMemoryInjector(memSvc, &fakeSettings{enabled: true}, &reranker.NoopReranker{})
	out := inj.PromptWithMemories(context.Background(), "base", &uid, 1, "q", nil)
	assert.True(t, strings.Contains(out, "## Things I Remember About You"))
	assert.True(t, strings.Contains(out, "- global fact"))
	assert.True(t, strings.Contains(out, "- ws fact"))
}

func TestMemoryInjector_RerankCapsAtMaxInjected(t *testing.T) {
	db := newMemTestDB(t)
	memSvc := NewMemoryService(db)
	uid := 1
	for i := 0; i < models.MaxInjectedWorkspaceLimit+3; i++ {
		_, _ = memSvc.Create(context.Background(), &uid, intPtr(1), models.MemoryScopeWorkspace, "ws"+string(rune('a'+i)))
	}
	inj := NewMemoryInjector(memSvc, &fakeSettings{enabled: true}, &reranker.NoopReranker{})
	out := inj.PromptWithMemories(context.Background(), "base", &uid, 1, "q", []core.Message{})
	bullets := strings.Count(out, "\n- ")
	assert.Equal(t, models.MaxInjectedWorkspaceLimit, bullets)
}

type fakeSettings struct{ enabled bool }

func (f *fakeSettings) MemoriesEnabled(ctx context.Context) bool { return f.enabled }

func intPtr(i int) *int { return &i }
```

- [ ] **Step 2: Run — expect compile errors**

```bash
cd backend && go test ./internal/services/ -run TestMemoryInjector
```

Expected: undefined `NewMemoryInjector`, `SettingsReader`.

## Task 5: Implement MemoryInjector

**Files:**
- Create: `backend/internal/services/memory_injector.go`

- [ ] **Step 1: Write injector**

```go
package services

import (
	"context"
	"fmt"
	"strings"

	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/reranker"
	"github.com/odysseythink/mlog"
	"github.com/odysseythink/pantheon/core"
)

// SettingsReader is the minimal interface MemoryInjector needs. SystemService
// satisfies it via a thin adapter — see Task 6.
type SettingsReader interface {
	MemoriesEnabled(ctx context.Context) bool
}

type MemoryInjector struct {
	memSvc   *MemoryService
	settings SettingsReader
	rerank   reranker.Reranker
}

func NewMemoryInjector(memSvc *MemoryService, settings SettingsReader, r reranker.Reranker) *MemoryInjector {
	return &MemoryInjector{memSvc: memSvc, settings: settings, rerank: r}
}

// PromptWithMemories appends a "## Things I Remember About You" section to base.
// Safe to call when memSvc/settings are nil — returns base unchanged.
func (mi *MemoryInjector) PromptWithMemories(ctx context.Context, base string,
	userID *int, workspaceID int, currentMessage string, history []core.Message) string {

	if mi == nil || mi.memSvc == nil || mi.settings == nil {
		return base
	}
	if !mi.settings.MemoriesEnabled(ctx) {
		return base
	}

	globals, _ := mi.memSvc.ListGlobal(ctx, userID)
	wsMems, _ := mi.memSvc.ListWorkspace(ctx, userID, workspaceID)
	if len(globals) == 0 && len(wsMems) == 0 {
		return base
	}

	selected := wsMems
	if len(wsMems) > models.MaxInjectedWorkspaceLimit {
		query := buildRerankQuery(currentMessage, history)
		texts := make([]string, len(wsMems))
		for i, m := range wsMems {
			texts[i] = m.Content
		}
		if ranked, err := mi.rerank.Rerank(ctx, query, texts, models.MaxInjectedWorkspaceLimit); err == nil {
			out := make([]models.Memory, 0, len(ranked))
			for _, r := range ranked {
				if r.Index >= 0 && r.Index < len(wsMems) {
					out = append(out, wsMems[r.Index])
				}
			}
			selected = out
		} else {
			mlog.Warning("memory inject rerank failed, using recency", mlog.Err(err))
			selected = wsMems[:models.MaxInjectedWorkspaceLimit]
		}
	}
	if len(selected) > models.MaxInjectedWorkspaceLimit {
		selected = selected[:models.MaxInjectedWorkspaceLimit]
	}

	// Stamp last-used fire-and-forget
	ids := make([]int, 0, len(globals)+len(selected))
	for _, m := range globals {
		ids = append(ids, m.ID)
	}
	for _, m := range selected {
		ids = append(ids, m.ID)
	}
	go func(stampIDs []int) {
		_ = mi.memSvc.UpdateLastUsed(context.Background(), stampIDs)
	}(ids)

	var b strings.Builder
	b.WriteString(base)
	b.WriteString("\n\n## Things I Remember About You\n")
	for _, m := range globals {
		fmt.Fprintf(&b, "- %s\n", m.Content)
	}
	for _, m := range selected {
		fmt.Fprintf(&b, "- %s\n", m.Content)
	}
	return strings.TrimRight(b.String(), "\n")
}

// buildRerankQuery concatenates the current message with up to the last 3
// user-role history texts.
func buildRerankQuery(currentMessage string, history []core.Message) string {
	parts := []string{currentMessage}
	count := 0
	for i := len(history) - 1; i >= 0 && count < 3; i-- {
		m := history[i]
		if m.Role != core.MESSAGE_ROLE_USER {
			continue
		}
		// Best-effort text extraction; pantheon's text part lives in Content[0].TextPart.
		for _, p := range m.Content {
			if tp, ok := p.(core.TextPart); ok {
				parts = append(parts, tp.Text)
				break
			}
		}
		count++
	}
	return strings.Join(parts, " ")
}
```

- [ ] **Step 2: Run tests — expect PASS**

```bash
cd backend && go test ./internal/services/ -run TestMemoryInjector -v
```

Expected: 4 tests PASS.

## Task 6: SystemService.MemoriesEnabled adapter

**Files:**
- Modify: `backend/internal/services/admin_service.go` or wherever `SystemService` lives

- [ ] **Step 1: Find the system-settings reader**

```bash
grep -rn 'func.*SystemService.*GetSetting' backend/internal/services/ | head -5
```

Add a typed helper on `SystemService`:

```go
func (s *SystemService) MemoriesEnabled(ctx context.Context) bool {
	v, _ := s.GetSetting(ctx, "memories_enabled")
	// Default: true. Explicit "false" disables.
	return v != "false"
}
```

This satisfies `SettingsReader` without an extra interface.

- [ ] **Step 2: Build**

```bash
cd backend && go build ./...
```

## Task 7: Wire MemoryInjector into ChatService

**Files:**
- Modify: `backend/internal/services/chat_service.go:28-37`, `:53-57`

- [ ] **Step 1: Add field + constructor arg**

In `chat_service.go` find the `ChatService` struct definition and add:

```go
memInj *MemoryInjector
```

Update `NewChatService` signature to accept and store it:

```go
func NewChatService(db *gorm.DB, cfg *config.Config, vectorSvc *VectorService,
	llmProv providers.LLMProvider, embedder embedder.Embedder,
	agentInvoker AgentInvoker, reranker reranker.Reranker,
	memInj *MemoryInjector) *ChatService {
	return &ChatService{
		db: db, cfg: cfg, vectorSvc: vectorSvc, llmProv: llmProv,
		embedder: embedder, agentInvoker: agentInvoker, reranker: reranker,
		memInj: memInj,
	}
}
```

- [ ] **Step 2: Call PromptWithMemories in buildRAGContext**

In `chat_service.go:53-57` (after `systemPrompt` is resolved from override or workspace, before the RAG concat at line 112), insert:

```go
// Inject long-term memories (no-op when memInj is nil or disabled).
var userID *int
if user != nil {
	userID = &user.ID
}
systemPrompt = s.memInj.PromptWithMemories(ctx, systemPrompt, userID, ws.ID, message, history)
```

`memInj` is safe to call as a nil receiver (Step 1 of Task 5 — guard included).

- [ ] **Step 3: Update main.go and tests**

```bash
grep -rn 'NewChatService(' backend/ --include='*.go'
```

For every match, insert `memInj` as the last arg (or `nil` in tests that don't care about it). In `cmd/server/main.go`:

```go
memSvc := services.NewMemoryService(db)
memInj := services.NewMemoryInjector(memSvc, sysSvc, rerankerImpl) // rerankerImpl is already in scope
chatSvc := services.NewChatService(db, cfg, vectorSvc, llmProv, embedderImpl, agentInvoker, rerankerImpl, memInj)
```

- [ ] **Step 4: Build + run all chat tests**

```bash
cd backend && go build ./... && go test ./internal/services/ -run Chat -v
```

Expected: all PASS.

## Task 8: Endpoints — failing test first

**Files:**
- Create: `backend/internal/handlers/memory_test.go`

- [ ] **Step 1: Write tests**

```go
package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newMemHandlerEnv(t *testing.T) (*gin.Engine, *gorm.DB) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Memory{}, &models.Workspace{}, &models.User{}))
	memSvc := services.NewMemoryService(db)
	wsSvc := services.NewWorkspaceService(db, &config.Config{}, nil)
	authSvc := services.NewAuthService(db, &config.Config{Secret: "t"})

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("user", &models.User{ID: 1, Role: "admin"})
		c.Next()
	})
	api := r.Group("/api")
	RegisterMemoryRoutes(api, memSvc, wsSvc, authSvc)
	return r, db
}

func TestMemory_CreateListDelete(t *testing.T) {
	r, db := newMemHandlerEnv(t)
	ws := models.Workspace{Name: "w", Slug: "w"}
	require.NoError(t, db.Create(&ws).Error)

	// Create
	body, _ := json.Marshal(map[string]any{"scope": "workspace", "workspaceId": ws.ID, "content": "fact"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/memory", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var created struct{ Memory models.Memory }
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))

	// List for slug
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/memory/workspace/w", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Delete
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, "/api/memory/"+strconv.Itoa(created.Memory.ID), nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestMemory_LimitReached(t *testing.T) {
	r, db := newMemHandlerEnv(t)
	ws := models.Workspace{Name: "w", Slug: "w"}
	require.NoError(t, db.Create(&ws).Error)

	for i := 0; i < models.GlobalMemoryLimit+1; i++ {
		body, _ := json.Marshal(map[string]any{"scope": "global", "content": "fact"})
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/memory", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		if i < models.GlobalMemoryLimit {
			require.Equal(t, http.StatusOK, w.Code)
		} else {
			assert.Equal(t, http.StatusConflict, w.Code)
		}
	}
}
```

- [ ] **Step 2: Run — expect compile error**

```bash
cd backend && go test ./internal/handlers/ -run TestMemory_
```

Expected: undefined `RegisterMemoryRoutes`.

## Task 9: Implement endpoints

**Files:**
- Create: `backend/internal/handlers/memory.go`

- [ ] **Step 1: Write handler**

```go
package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/dto"
	"github.com/odysseythink/hermind/backend/internal/middleware"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/services"
)

type MemoryHandler struct {
	mem *services.MemoryService
	ws  *services.WorkspaceService
}

func NewMemoryHandler(mem *services.MemoryService, ws *services.WorkspaceService) *MemoryHandler {
	return &MemoryHandler{mem: mem, ws: ws}
}

type createMemReq struct {
	Scope       string `json:"scope"`
	WorkspaceID *int   `json:"workspaceId"`
	Content     string `json:"content"`
}

func userIDFromCtx(c *gin.Context) *int {
	v, ok := c.Get("user")
	if !ok {
		return nil
	}
	u, ok := v.(*models.User)
	if !ok {
		return nil
	}
	return &u.ID
}

func (h *MemoryHandler) Create(c *gin.Context) {
	var req createMemReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}
	m, err := h.mem.Create(c.Request.Context(), userIDFromCtx(c), req.WorkspaceID, req.Scope, req.Content)
	if err != nil {
		if errors.Is(err, services.ErrMemoryLimitReached) {
			c.JSON(http.StatusConflict, dto.ErrorResponse{Error: err.Error()})
			return
		}
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"memory": m})
}

func (h *MemoryHandler) List(c *gin.Context) {
	slug := c.Param("slug")
	ws, err := h.ws.GetBySlug(c.Request.Context(), slug)
	if err != nil {
		c.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "workspace not found"})
		return
	}
	uid := userIDFromCtx(c)
	ws_mems, _ := h.mem.ListWorkspace(c.Request.Context(), uid, ws.ID)
	glob, _ := h.mem.ListGlobal(c.Request.Context(), uid)
	c.JSON(http.StatusOK, gin.H{"workspace": ws_mems, "global": glob})
}

type updateMemReq struct {
	Content string `json:"content"`
}

func (h *MemoryHandler) Update(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "bad id"})
		return
	}
	var req updateMemReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}
	m, err := h.mem.Update(c.Request.Context(), id, req.Content)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"memory": m})
}

func (h *MemoryHandler) Delete(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "bad id"})
		return
	}
	if err := h.mem.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *MemoryHandler) Promote(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "bad id"})
		return
	}
	m, err := h.mem.PromoteToGlobal(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, services.ErrMemoryLimitReached) {
			c.JSON(http.StatusConflict, dto.ErrorResponse{Error: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"memory": m})
}

func (h *MemoryHandler) Demote(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	wsID, _ := strconv.Atoi(c.Param("workspaceId"))
	m, err := h.mem.DemoteToWorkspace(c.Request.Context(), id, wsID)
	if err != nil {
		if errors.Is(err, services.ErrMemoryLimitReached) {
			c.JSON(http.StatusConflict, dto.ErrorResponse{Error: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"memory": m})
}

type replaceMemReq struct {
	Memories []string `json:"memories"`
}

func (h *MemoryHandler) ReplaceWorkspace(c *gin.Context) {
	slug := c.Param("slug")
	ws, err := h.ws.GetBySlug(c.Request.Context(), slug)
	if err != nil {
		c.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "workspace not found"})
		return
	}
	var req replaceMemReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}
	if err := h.mem.ReplaceWorkspace(c.Request.Context(), userIDFromCtx(c), ws.ID, req.Memories); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func RegisterMemoryRoutes(r *gin.RouterGroup, mem *services.MemoryService, ws *services.WorkspaceService, authSvc *services.AuthService) {
	h := NewMemoryHandler(mem, ws)
	g := r.Group("", middleware.ValidatedRequest(authSvc))
	g.GET("/memory/workspace/:slug", h.List)
	g.POST("/memory", h.Create)
	g.PATCH("/memory/:id", h.Update)
	g.DELETE("/memory/:id", h.Delete)
	g.POST("/memory/:id/promote", h.Promote)
	g.POST("/memory/:id/demote/:workspaceId", h.Demote)
	g.PUT("/memory/workspace/:slug/replace", h.ReplaceWorkspace)
}
```

- [ ] **Step 2: Wire in main.go**

In `cmd/server/main.go` after `memSvc := services.NewMemoryService(db)` add:

```go
handlers.RegisterMemoryRoutes(api, memSvc, wsSvc, authSvc)
```

- [ ] **Step 3: Run tests — expect PASS**

```bash
cd backend && go test ./internal/handlers/ -run TestMemory_ -v
```

Expected: 2 tests PASS.

## Task 10: Commit PR1

- [ ] **Step 1: Stage + commit**

```bash
git add backend/internal/models/memory.go \
        backend/internal/services/db.go \
        backend/internal/services/memory_service.go \
        backend/internal/services/memory_service_test.go \
        backend/internal/services/memory_injector.go \
        backend/internal/services/memory_injector_test.go \
        backend/internal/services/admin_service.go \
        backend/internal/services/chat_service.go \
        backend/internal/handlers/memory.go \
        backend/internal/handlers/memory_test.go \
        backend/cmd/server/main.go
git commit -m "$(cat <<'EOF'
feat(memories): CRUD + chat-prompt injection (manual mode)

Adds memories table with GLOBAL=5 / WORKSPACE=20 / inject<=5 limits
(parity with anything-llm). MemoryInjector hooks into ChatService.
buildRAGContext to append "## Things I Remember About You" between the
workspace system prompt and the RAG context. Reranker selects top-5
when workspace memories exceed the cap; falls back to recency on
failure. Auto-extraction (PR2) wires into the same MemoryService.

memories_enabled SystemSetting (default true) gates injection.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

# PR2 · Observer + Reflector extractor (~900 LoC)

## Task 11: Add memory_processed column to workspace_chats

**Files:**
- Modify: `backend/internal/models/workspace_chat.go`

- [ ] **Step 1: Add field**

```go
type WorkspaceChat struct {
	ID              int       `gorm:"primaryKey;autoIncrement" json:"id"`
	WorkspaceID     int       `json:"workspaceId"`
	Prompt          string    `json:"prompt"`
	Response        string    `json:"response"`
	Include         bool      `gorm:"default:true" json:"include"`
	UserID          *int      `json:"userId"`
	ThreadID        *int      `json:"threadId"`
	APISessionID    *string   `json:"apiSessionId"`
	CreatedAt       time.Time `json:"createdAt"`
	LastUpdatedAt   time.Time `json:"lastUpdatedAt"`
	FeedbackScore   *bool     `json:"feedbackScore"`
	MemoryProcessed *bool     `gorm:"index" json:"memoryProcessed,omitempty"`
}
```

- [ ] **Step 2: AutoMigrate verifies the column on next boot — confirm:**

```bash
cd backend && go build ./...
```

## Task 12: Prompt templates

**Files:**
- Create: `backend/prompts/memory_observer.txt`
- Create: `backend/prompts/memory_reflector.txt`

- [ ] **Step 1: Observer template**

```
You are a memory extraction assistant. Given a recent conversation between
a user and an AI assistant, identify durable facts about the user worth
remembering for future conversations.

CRITERIA for a fact:
- Concrete (a specific preference, expertise, project, role, or stable belief)
- Re-usable across distinct conversation topics
- Not transient (not "is currently confused about Y")
- Not derivable from the assistant's general knowledge

Call the extract_candidate_facts tool. Each fact has:
- content: short third-person statement ("User prefers Go over Python")
- confidence: 0.0..1.0
- reasoning: one sentence explaining why this is durable

If no durable facts, call the tool with facts: [].

CONVERSATION:
{{CONVERSATION}}
```

- [ ] **Step 2: Reflector template**

```
You are a memory curator. Given a set of candidate facts from the Observer
and the user's existing memories, decide which candidates to keep, dedupe,
or use to update an existing memory.

For each accepted memory call decide_memory_actions with:
- content: final memory text
- scope: "WORKSPACE" (specific to this workspace) or "GLOBAL" (applies everywhere)
- action: "create" (new memory) or "update" (revise an existing memory by id)
- updateId: integer (required when action == "update"; must reference an
  existing memory id from the lists below)
- reasoning: one sentence

RULES:
- Skip candidates that duplicate or are subsumed by existing memories
- Prefer "update" when a candidate refines an existing memory's content
- Never create a memory that contradicts an existing one — reject silently
- Available global slots: {{GLOBAL_SLOTS}}; do not exceed
- The workspace cap is 20; selecting more workspace creates will be truncated

EXISTING WORKSPACE MEMORIES (id: content):
{{WORKSPACE_MEMORIES}}

EXISTING GLOBAL MEMORIES (id: content):
{{GLOBAL_MEMORIES}}

CANDIDATE FACTS:
{{CANDIDATES}}
```

## Task 13: MemoryExtractor — failing tests with mock LLM

**Files:**
- Create: `backend/internal/services/memory_extractor_test.go`

- [ ] **Step 1: Define mock LLM and write a smoke test**

```go
package services

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/pantheon/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockLLM struct {
	observerResp string // raw JSON for `extract_candidate_facts` tool call
	reflectorResp string
}

func (m *mockLLM) Generate(_ context.Context, req *core.Request) (*core.Response, error) {
	if len(req.Tools) == 0 {
		return &core.Response{}, nil
	}
	// Pick by tool name in req.Tools[0].
	switch req.Tools[0].Name {
	case "extract_candidate_facts":
		return toolCallResp(req.Tools[0].Name, m.observerResp), nil
	case "decide_memory_actions":
		return toolCallResp(req.Tools[0].Name, m.reflectorResp), nil
	}
	return &core.Response{}, nil
}

func toolCallResp(name, args string) *core.Response {
	return &core.Response{
		Content: []core.ContentParter{
			core.ToolUsePart{ID: "1", Name: name, Input: json.RawMessage(args)},
		},
	}
}

func TestMemoryExtractor_RoundTrip(t *testing.T) {
	db := newMemTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.WorkspaceChat{}, &models.Workspace{}))
	memSvc := NewMemoryService(db)
	uid, wid := 1, 1
	chats := []models.WorkspaceChat{
		{WorkspaceID: wid, UserID: &uid, Prompt: "I'm a Go dev", Response: "noted"},
		{WorkspaceID: wid, UserID: &uid, Prompt: "I work at Acme", Response: "noted"},
	}
	for i := range chats {
		require.NoError(t, db.Create(&chats[i]).Error)
	}

	llm := &mockLLM{
		observerResp: `{"facts":[{"content":"User is a Go developer","confidence":0.9,"reasoning":"explicit"}]}`,
		reflectorResp: `{"memories":[{"content":"User is a Go developer","scope":"GLOBAL","action":"create","reasoning":"durable"}]}`,
	}
	ext := NewMemoryExtractor(memSvc, llm, "obs prompt {{CONVERSATION}}", "ref prompt {{CANDIDATES}}")

	err := ext.ProcessGroup(context.Background(), &uid, wid, chats)
	require.NoError(t, err)

	globals, _ := memSvc.ListGlobal(context.Background(), &uid)
	require.Len(t, globals, 1)
	assert.Equal(t, "User is a Go developer", globals[0].Content)
}
```

- [ ] **Step 2: Run — expect compile error**

```bash
cd backend && go test ./internal/services/ -run TestMemoryExtractor_RoundTrip
```

Expected: undefined `NewMemoryExtractor`.

## Task 14: Implement MemoryExtractor

**Files:**
- Create: `backend/internal/services/memory_extractor.go`

- [ ] **Step 1: Write extractor**

```go
package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	_ "embed"

	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/mlog"
	"github.com/odysseythink/pantheon/core"
)

//go:embed ../../prompts/memory_observer.txt
var memObserverPrompt string

//go:embed ../../prompts/memory_reflector.txt
var memReflectorPrompt string

// LLMClient is the slice of pantheon's LLM API the extractor needs.
type LLMClient interface {
	Generate(ctx context.Context, req *core.Request) (*core.Response, error)
}

type MemoryExtractor struct {
	memSvc    *MemoryService
	llm       LLMClient
	observerT string
	reflectorT string
}

func NewMemoryExtractor(memSvc *MemoryService, llm LLMClient, observerT, reflectorT string) *MemoryExtractor {
	if observerT == "" {
		observerT = memObserverPrompt
	}
	if reflectorT == "" {
		reflectorT = memReflectorPrompt
	}
	return &MemoryExtractor{memSvc: memSvc, llm: llm, observerT: observerT, reflectorT: reflectorT}
}

type observerCandidate struct {
	Content    string  `json:"content"`
	Confidence float64 `json:"confidence"`
	Reasoning  string  `json:"reasoning"`
}

type reflectorAction struct {
	Content   string `json:"content"`
	Scope     string `json:"scope"`     // WORKSPACE | GLOBAL
	Action    string `json:"action"`    // create | update
	UpdateID  *int   `json:"updateId,omitempty"`
	Reasoning string `json:"reasoning"`
}

func (e *MemoryExtractor) ProcessGroup(ctx context.Context, userID *int, workspaceID int, chats []models.WorkspaceChat) error {
	if len(chats) == 0 {
		return nil
	}

	// 1. Observer
	convo := renderConversation(chats)
	obsPrompt := strings.ReplaceAll(e.observerT, "{{CONVERSATION}}", convo)
	candidates, err := e.runObserver(ctx, obsPrompt)
	if err != nil || len(candidates) == 0 {
		if err != nil {
			mlog.Warning("memory observer failed", mlog.Err(err))
		}
		return nil
	}

	// 2. Reflector
	wsMems, _ := e.memSvc.ListWorkspace(ctx, userID, workspaceID)
	glMems, _ := e.memSvc.ListGlobal(ctx, userID)
	globalSlots := models.GlobalMemoryLimit - len(glMems)
	if globalSlots <= 0 && len(wsMems) >= models.WorkspaceMemoryLimit {
		return nil
	}
	refPrompt := e.reflectorT
	refPrompt = strings.ReplaceAll(refPrompt, "{{WORKSPACE_MEMORIES}}", renderMems(wsMems))
	refPrompt = strings.ReplaceAll(refPrompt, "{{GLOBAL_MEMORIES}}", renderMems(glMems))
	refPrompt = strings.ReplaceAll(refPrompt, "{{GLOBAL_SLOTS}}", fmt.Sprintf("%d", globalSlots))
	refPrompt = strings.ReplaceAll(refPrompt, "{{CANDIDATES}}", renderCandidates(candidates))

	actions, err := e.runReflector(ctx, refPrompt)
	if err != nil || len(actions) == 0 {
		if err != nil {
			mlog.Warning("memory reflector failed", mlog.Err(err))
		}
		return nil
	}

	// 3. Apply
	extracted := make([]ExtractedAction, 0, len(actions))
	for _, a := range actions {
		extracted = append(extracted, ExtractedAction{
			Action: a.Action, Scope: a.Scope, Content: a.Content, UpdateID: a.UpdateID,
		})
	}
	_, err = e.memSvc.ApplyExtracted(ctx, userID, workspaceID, extracted, globalSlots)
	return err
}

func (e *MemoryExtractor) runObserver(ctx context.Context, prompt string) ([]observerCandidate, error) {
	resp, err := e.llm.Generate(ctx, &core.Request{
		SystemPrompt: "You extract durable user facts via the provided tool.",
		Messages: []core.Message{{
			Role:    core.MESSAGE_ROLE_USER,
			Content: []core.ContentParter{core.TextPart{Text: prompt}},
		}},
		Tools: []core.ToolDefinition{{Name: "extract_candidate_facts", Description: "Emit candidate facts", Parameters: candidateSchema()}},
	})
	if err != nil {
		return nil, err
	}
	args := firstToolUseInput(resp)
	if args == nil {
		return nil, nil
	}
	var body struct{ Facts []observerCandidate `json:"facts"` }
	if err := json.Unmarshal(args, &body); err != nil {
		return nil, err
	}
	return body.Facts, nil
}

func (e *MemoryExtractor) runReflector(ctx context.Context, prompt string) ([]reflectorAction, error) {
	resp, err := e.llm.Generate(ctx, &core.Request{
		SystemPrompt: "You curate memories via the provided tool.",
		Messages: []core.Message{{
			Role:    core.MESSAGE_ROLE_USER,
			Content: []core.ContentParter{core.TextPart{Text: prompt}},
		}},
		Tools: []core.ToolDefinition{{Name: "decide_memory_actions", Description: "Decide actions", Parameters: actionSchema()}},
	})
	if err != nil {
		return nil, err
	}
	args := firstToolUseInput(resp)
	if args == nil {
		return nil, nil
	}
	var body struct{ Memories []reflectorAction `json:"memories"` }
	if err := json.Unmarshal(args, &body); err != nil {
		return nil, err
	}
	return body.Memories, nil
}

func firstToolUseInput(resp *core.Response) json.RawMessage {
	if resp == nil {
		return nil
	}
	for _, p := range resp.Content {
		if t, ok := p.(core.ToolUsePart); ok {
			return t.Input
		}
	}
	return nil
}

func renderConversation(chats []models.WorkspaceChat) string {
	var b strings.Builder
	for _, c := range chats {
		fmt.Fprintf(&b, "USER: %s\nASSISTANT: %s\n\n", c.Prompt, truncateResp(c.Response))
	}
	return b.String()
}

func truncateResp(s string) string {
	if len(s) <= 600 {
		return s
	}
	return s[:600] + "…"
}

func renderMems(mems []models.Memory) string {
	if len(mems) == 0 {
		return "(none)"
	}
	var b strings.Builder
	for _, m := range mems {
		fmt.Fprintf(&b, "%d: %s\n", m.ID, m.Content)
	}
	return b.String()
}

func renderCandidates(cs []observerCandidate) string {
	var b strings.Builder
	for _, c := range cs {
		fmt.Fprintf(&b, "- [%0.2f] %s — %s\n", c.Confidence, c.Content, c.Reasoning)
	}
	return b.String()
}

func candidateSchema() core.JSONSchema {
	return core.JSONSchema{
		"type": "object",
		"properties": map[string]any{
			"facts": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"content":    map[string]any{"type": "string"},
						"confidence": map[string]any{"type": "number"},
						"reasoning":  map[string]any{"type": "string"},
					},
					"required": []string{"content"},
				},
			},
		},
		"required": []string{"facts"},
	}
}

func actionSchema() core.JSONSchema {
	return core.JSONSchema{
		"type": "object",
		"properties": map[string]any{
			"memories": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"content":   map[string]any{"type": "string"},
						"scope":     map[string]any{"type": "string", "enum": []string{"WORKSPACE", "GLOBAL"}},
						"action":    map[string]any{"type": "string", "enum": []string{"create", "update"}},
						"updateId":  map[string]any{"type": "integer"},
						"reasoning": map[string]any{"type": "string"},
					},
					"required": []string{"content", "scope", "action"},
				},
			},
		},
		"required": []string{"memories"},
	}
}
```

> Note: `core.JSONSchema` and `core.ToolUsePart.Input` reflect the pantheon types as used elsewhere in the backend (e.g., `agent/native_tool_calling.go`). If your pantheon version names them differently (`schema.JSONSchema`, `core.ToolInputSchema`, etc.), adapt the type names — the conceptual schema is the same. Run `grep -rn 'ToolDefinition\|ToolUsePart' backend/` to find the canonical names.

- [ ] **Step 2: Run test — expect PASS**

```bash
cd backend && go test ./internal/services/ -run TestMemoryExtractor_RoundTrip -v
```

Expected: PASS. If pantheon's tool types differ in your version, the test's `toolCallResp` helper also needs adapting.

## Task 15: extract-memories worker

**Files:**
- Create: `backend/internal/workers/extract_memories.go`

- [ ] **Step 1: Define a `workers.Job` implementation**

```go
package workers

import (
	"context"
	"time"

	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/services"
	"github.com/odysseythink/mlog"
	"gorm.io/gorm"
)

const (
	MinChatsForExtract     = 5
	GroupIdleThresholdMS   = 20 * 60 * 1000 // 20 min
)

type ExtractMemoriesJob struct {
	db        *gorm.DB
	memSvc    *services.MemoryService
	extractor *services.MemoryExtractor
	sysSvc    *services.SystemService
}

func NewExtractMemoriesJob(db *gorm.DB, memSvc *services.MemoryService, ext *services.MemoryExtractor, sysSvc *services.SystemService) *ExtractMemoriesJob {
	return &ExtractMemoriesJob{db: db, memSvc: memSvc, extractor: ext, sysSvc: sysSvc}
}

func (j *ExtractMemoriesJob) Name() string     { return "extract-memories" }
func (j *ExtractMemoriesJob) Schedule() string { return "0 */3 * * *" }
func (j *ExtractMemoriesJob) Enabled(ctx context.Context) bool {
	v, _ := j.sysSvc.GetSetting(ctx, "memories_auto_extraction_enabled")
	return v == "true"
}

type groupKey struct{ UserID *int; WorkspaceID int }

func (j *ExtractMemoriesJob) Run(ctx context.Context) error {
	var unprocessed []models.WorkspaceChat
	if err := j.db.WithContext(ctx).
		Where("(memory_processed IS NULL OR memory_processed = ?) AND include = ?", false, true).
		Order("created_at ASC").
		Find(&unprocessed).Error; err != nil {
		return err
	}
	if len(unprocessed) == 0 {
		return nil
	}

	groups := map[groupKey][]models.WorkspaceChat{}
	for _, c := range unprocessed {
		k := groupKey{UserID: c.UserID, WorkspaceID: c.WorkspaceID}
		groups[k] = append(groups[k], c)
	}

	for k, chats := range groups {
		if len(chats) < MinChatsForExtract {
			continue
		}
		// Idle check: skip if last chat younger than threshold.
		if time.Since(chats[len(chats)-1].CreatedAt) < time.Duration(GroupIdleThresholdMS)*time.Millisecond {
			continue
		}
		if err := j.extractor.ProcessGroup(ctx, k.UserID, k.WorkspaceID, chats); err != nil {
			mlog.Warning("extract memories failed",
				mlog.Int("workspace", k.WorkspaceID), mlog.Err(err))
		}
		// Mark processed regardless of extractor outcome — anything-llm behavior.
		ids := make([]int, len(chats))
		for i, c := range chats {
			ids[i] = c.ID
		}
		j.markProcessed(ctx, ids)
	}
	return nil
}

func (j *ExtractMemoriesJob) markProcessed(ctx context.Context, ids []int) {
	if len(ids) == 0 {
		return
	}
	t := true
	if err := j.db.WithContext(ctx).Model(&models.WorkspaceChat{}).
		Where("id IN ?", ids).Update("memory_processed", &t).Error; err != nil {
		mlog.Warning("mark memory_processed failed", mlog.Err(err))
	}
}
```

- [ ] **Step 2: Wire in main.go**

In `cmd/server/main.go` after the existing workers register block:

```go
memExt := services.NewMemoryExtractor(memSvc, llmProv, "", "") // empty -> use embedded prompts
workerMgr.Register(workers.NewExtractMemoriesJob(db, memSvc, memExt, sysSvc))
```

`llmProv` must implement the `services.LLMClient` interface. If pantheon's `core.LanguageModel` is what you have, write a thin adapter:

```go
type llmAdapter struct{ inner core.LanguageModel }
func (a *llmAdapter) Generate(ctx context.Context, req *core.Request) (*core.Response, error) {
	return a.inner.Generate(ctx, req)
}
```

Pass `&llmAdapter{inner: llmCore}` where `llmCore` is whatever main already constructed.

- [ ] **Step 3: Build + test**

```bash
cd backend && go build ./... && go test ./internal/workers/ -v
```

Expected: builds clean; existing worker tests still PASS.

## Task 16: Commit PR2

- [ ] **Step 1: Stage + commit**

```bash
git add backend/internal/models/workspace_chat.go \
        backend/prompts/memory_observer.txt \
        backend/prompts/memory_reflector.txt \
        backend/internal/services/memory_extractor.go \
        backend/internal/services/memory_extractor_test.go \
        backend/internal/workers/extract_memories.go \
        backend/cmd/server/main.go
git commit -m "$(cat <<'EOF'
feat(memories): Observer/Reflector auto-extraction worker

Adds a 3-hour cron worker that walks unprocessed workspace_chats (new
memory_processed column), groups by (user, workspace), and runs a
two-phase LLM extraction:
  1. Observer extracts candidate user facts via tool call
  2. Reflector decides scope/dedup and emits create/update actions
Applied transactionally via MemoryService.ApplyExtracted.

Gated by memories_auto_extraction_enabled SystemSetting (default false
— users must opt in).

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Self-Review

| Spec § | Tasks |
|---|---|
| §4.1 schema | 1, 11 |
| §4.2 PR1 service | 2, 3 |
| §4.2 PR1 injector | 4, 5, 7 |
| §4.2 PR1 endpoints | 8, 9 |
| §4.3 PR2 column | 11 |
| §4.3 PR2 prompts | 12 |
| §4.3 PR2 extractor | 13, 14 |
| §4.3 PR2 worker | 15 |
| §5 config (memories_enabled, memories_auto_extraction_enabled) | 6 (enabled adapter), 15 (auto_extraction) |
| §6 wiring | 7, 9, 15 |

**Type consistency:** `ExtractedAction` defined in Task 3 is consumed by Task 14 (`reflectorAction` is the JSON shape, converted to `ExtractedAction` at call site). `LLMClient` interface defined in Task 14 is satisfied by main.go's adapter in Task 15.

**Pantheon type drift:** Tasks 14 + 15 depend on pantheon's `core` package shape (`ToolDefinition`, `ToolUsePart`, `JSONSchema`, `LanguageModel.Generate`). The plan includes a sentinel grep:

```bash
grep -rn 'ToolDefinition\|ToolUsePart' backend/
```

Adapt to the actual names if they've rotated.

**No placeholders.** Every step shows full code.

---

## Execution Handoff

Plan complete and saved to `.gpowers/plans/2026-05-28-memories-system.md`.
