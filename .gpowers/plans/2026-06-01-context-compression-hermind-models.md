# Context Compression — Hermind Models & Persistence

> **Local goal:** Add `ThreadCompaction` GORM model, extend `Workspace`/`WorkspaceThread` with compression fields, and wire AutoMigrate + system defaults. This sub-plan produces a buildable, testable database layer.
> **Depends on file:** `2026-06-01-context-compression-pantheon-upstream.md` (Pantheon engine must exist, but no symbol dependency — Hermind models compile independently)

---

## File Structure

| Path | Responsibility |
|---|---|
| `backend/internal/models/thread_compaction.go` | New GORM model: `ThreadCompaction` — persisted summary + `UpToChatID` |
| `backend/internal/models/thread_compaction_test.go` | CRUD + foreign-key behavioral tests for `ThreadCompaction` |
| `backend/internal/models/workspace.go` | Add `CompressEnabled`, `CompressThreshold`, `CompressContextLen` |
| `backend/internal/models/workspace_thread.go` | Add `ParentThreadID` |
| `backend/internal/services/db.go` | AutoMigrate `ThreadCompaction`; seed `context_compress_enabled=false` default |

## Dependency Overview

```
Task M1 (ThreadCompaction model + test)
  -> Task M2 (Workspace/WorkspaceThread fields + test)
       -> Task M3 (AutoMigrate + SeedDefaults + test)
```

All three are sequential: M2's tests can run without M3 (they use in-memory DBs and call `AutoMigrate` themselves), but M3 is the production wiring that makes the new tables exist on boot.

## Risks & Open Questions

| # | Risk | Assumption | Impact if wrong |
|---|---|---|---|
| 1 | `ThreadID` nil vs non-nil semantics | `ThreadID == nil` means default workspace session (no explicit thread) | Queries must handle both; if wrong, compactions leak across threads |
| 2 | `UpToChatID` ordering | `WorkspaceChat.ID` is monotonically increasing (auto-increment primary key) | If not, "messages after UpToChatID" logic is broken |

---

### Task M1: ThreadCompaction GORM Model

**Depends on:** none

**Files:**
- Create: `backend/internal/models/thread_compaction.go`
- Create: `backend/internal/models/thread_compaction_test.go`

- [ ] **Step 1: Write the failing test**

```go
// backend/internal/models/thread_compaction_test.go
package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestThreadCompaction_CRUD(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&_pragma=foreign_keys(1)"), &gorm.Config{})
	require.NoError(t, err)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	defer sqlDB.Close()

	require.NoError(t, db.AutoMigrate(&ThreadCompaction{}))

	// Create
	c := ThreadCompaction{
		WorkspaceID: 1,
		ThreadID:    intPtr(2),
		Summary:     "Summary of chats 1–5",
		UpToChatID:  5,
		CreatedAt:   time.Now(),
		LastUpdatedAt: time.Now(),
	}
	require.NoError(t, db.Create(&c).Error)
	assert.NotZero(t, c.ID)

	// Read back
	var loaded ThreadCompaction
	require.NoError(t, db.First(&loaded, c.ID).Error)
	assert.Equal(t, 1, loaded.WorkspaceID)
	assert.Equal(t, 2, *loaded.ThreadID)
	assert.Equal(t, "Summary of chats 1–5", loaded.Summary)
	assert.Equal(t, 5, loaded.UpToChatID)

	// Nil ThreadID (default workspace session)
	c2 := ThreadCompaction{
		WorkspaceID: 1,
		ThreadID:    nil,
		Summary:     "Default session summary",
		UpToChatID:  10,
		CreatedAt:   time.Now(),
		LastUpdatedAt: time.Now(),
	}
	require.NoError(t, db.Create(&c2).Error)
	var loaded2 ThreadCompaction
	require.NoError(t, db.First(&loaded2, c2.ID).Error)
	assert.Nil(t, loaded2.ThreadID)

	// Latest-for-query ordering
	c3 := ThreadCompaction{
		WorkspaceID: 1,
		ThreadID:    intPtr(2),
		Summary:     "Newer summary",
		UpToChatID:  8,
		CreatedAt:   time.Now().Add(time.Minute),
		LastUpdatedAt: time.Now().Add(time.Minute),
	}
	require.NoError(t, db.Create(&c3).Error)

	var latest ThreadCompaction
	require.NoError(t, db.Where("workspace_id = ? AND thread_id = ?", 1, 2).
		Order("created_at DESC").First(&latest).Error)
	assert.Equal(t, 8, latest.UpToChatID)
	assert.Equal(t, "Newer summary", latest.Summary)
}

func intPtr(i int) *int { return &i }
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/models/ -run TestThreadCompaction_CRUD -v`
Expected: FAIL with `ThreadCompaction not defined`

- [ ] **Step 3: Write the model**

```go
// backend/internal/models/thread_compaction.go
package models

import "time"

type ThreadCompaction struct {
	ID            int       `gorm:"primaryKey;autoIncrement" json:"id"`
	WorkspaceID   int       `json:"workspaceId"`
	ThreadID      *int      `json:"threadId"`
	Summary       string    `json:"summary"`
	UpToChatID    int       `json:"upToChatId"`
	CreatedAt     time.Time `json:"createdAt"`
	LastUpdatedAt time.Time `json:"lastUpdatedAt"`
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd backend && go test ./internal/models/ -run TestThreadCompaction_CRUD -v`
Expected: PASS

- [ ] **Step 5: Whole-tree typecheck**

Run: `cd backend && go vet ./...`
Expected: no errors (new package compiles, no stale callers)

- [ ] **Step 6: Commit**

```bash
git add backend/internal/models/thread_compaction.go backend/internal/models/thread_compaction_test.go
git commit -m "feat(compression): add ThreadCompaction model with CRUD test"
```

---

### Task M2: Extend Workspace and WorkspaceThread Models

**Depends on:** Task M1 (no symbol dependency, but logical ordering — models file should exist first)

**Files:**
- Modify: `backend/internal/models/workspace.go`
- Modify: `backend/internal/models/workspace_thread.go`
- Create: `backend/internal/models/workspace_compress_test.go`

- [ ] **Step 1: Write the failing test**

```go
// backend/internal/models/workspace_compress_test.go
package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestWorkspace_CompressFields(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	defer sqlDB.Close()

	require.NoError(t, db.AutoMigrate(&Workspace{}))

	// Create with compression fields
	ws := Workspace{
		Name:               "test-ws",
		Slug:               "test-ws",
		CompressEnabled:    boolPtr(true),
		CompressThreshold:  floatPtr(0.65),
		CompressContextLen: intPtr(16384),
	}
	require.NoError(t, db.Create(&ws).Error)
	assert.NotZero(t, ws.ID)

	// Read back
	var loaded Workspace
	require.NoError(t, db.First(&loaded, ws.ID).Error)
	assert.NotNil(t, loaded.CompressEnabled)
	assert.True(t, *loaded.CompressEnabled)
	assert.NotNil(t, loaded.CompressThreshold)
	assert.InDelta(t, 0.65, *loaded.CompressThreshold, 0.001)
	assert.NotNil(t, loaded.CompressContextLen)
	assert.Equal(t, 16384, *loaded.CompressContextLen)

	// Nil fields (use global default)
	ws2 := Workspace{Name: "test-ws-2", Slug: "test-ws-2"}
	require.NoError(t, db.Create(&ws2).Error)
	var loaded2 Workspace
	require.NoError(t, db.First(&loaded2, ws2.ID).Error)
	assert.Nil(t, loaded2.CompressEnabled)
	assert.Nil(t, loaded2.CompressThreshold)
	assert.Nil(t, loaded2.CompressContextLen)
}

func TestWorkspaceThread_ParentThreadID(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	defer sqlDB.Close()

	require.NoError(t, db.AutoMigrate(&WorkspaceThread{}))

	// Thread with parent
	wt := WorkspaceThread{
		Name:           "child",
		Slug:           "child",
		WorkspaceID:    1,
		ParentThreadID: intPtr(99),
	}
	require.NoError(t, db.Create(&wt).Error)

	var loaded WorkspaceThread
	require.NoError(t, db.First(&loaded, wt.ID).Error)
	assert.NotNil(t, loaded.ParentThreadID)
	assert.Equal(t, 99, *loaded.ParentThreadID)

	// Thread without parent
	wt2 := WorkspaceThread{Name: "orphan", Slug: "orphan", WorkspaceID: 1}
	require.NoError(t, db.Create(&wt2).Error)
	var loaded2 WorkspaceThread
	require.NoError(t, db.First(&loaded2, wt2.ID).Error)
	assert.Nil(t, loaded2.ParentThreadID)
}

func boolPtr(b bool) *bool     { return &b }
func floatPtr(f float64) *float64 { return &f }
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/models/ -run TestWorkspace_CompressFields -v`
Expected: FAIL with `CompressEnabled undefined`

Run: `cd backend && go test ./internal/models/ -run TestWorkspaceThread_ParentThreadID -v`
Expected: FAIL with `ParentThreadID undefined`

- [ ] **Step 3: Add fields to Workspace and WorkspaceThread**

Modify `backend/internal/models/workspace.go` — append three new fields before the closing brace:

```go
	CompressEnabled    *bool    `json:"compressEnabled"`
	CompressThreshold  *float64 `json:"compressThreshold"`
	CompressContextLen *int     `json:"compressContextLen"`
```

The full file becomes:

```go
package models

import "time"

type Workspace struct {
	ID                   int       `gorm:"primaryKey;autoIncrement" json:"id"`
	Name                 string    `json:"name"`
	Slug                 string    `gorm:"unique" json:"slug"`
	VectorTag            *string   `json:"vectorTag"`
	CreatedAt            time.Time `json:"createdAt"`
	OpenAiTemp           *float64  `json:"openAiTemp"`
	OpenAiHistory        int       `gorm:"default:20" json:"openAiHistory"`
	LastUpdatedAt        time.Time `json:"lastUpdatedAt"`
	OpenAiPrompt         *string   `json:"openAiPrompt"`
	SimilarityThreshold  *float64  `gorm:"default:0.25" json:"similarityThreshold"`
	ChatProvider         *string   `json:"chatProvider"`
	ChatModel            *string   `json:"chatModel"`
	TopN                 *int      `gorm:"default:4" json:"topN"`
	ChatMode             *string   `gorm:"default:chat" json:"chatMode"`
	PfpFilename          *string   `json:"pfpFilename"`
	AgentProvider        *string   `json:"agentProvider"`
	AgentModel           *string   `json:"agentModel"`
	QueryRefusalResponse *string   `json:"queryRefusalResponse"`
	VectorSearchMode     *string   `gorm:"default:default" json:"vectorSearchMode"`
	CompressEnabled      *bool     `json:"compressEnabled"`
	CompressThreshold    *float64  `json:"compressThreshold"`
	CompressContextLen   *int      `json:"compressContextLen"`
}
```

Modify `backend/internal/models/workspace_thread.go` — add `ParentThreadID` after `UserID`:

```go
type WorkspaceThread struct {
	ID             int        `gorm:"primaryKey;autoIncrement" json:"id"`
	Name           string     `json:"name"`
	Slug           string     `gorm:"unique" json:"slug"`
	WorkspaceID    int        `json:"workspaceId"`
	UserID         *int       `json:"userId"`
	ParentThreadID *int       `json:"parentThreadId"`
	CreatedAt      time.Time  `json:"createdAt"`
	LastUpdatedAt  time.Time  `json:"lastUpdatedAt"`
	Workspace      *Workspace `gorm:"foreignKey:WorkspaceID" json:"workspace,omitempty"`
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd backend && go test ./internal/models/ -run TestWorkspace_CompressFields -v`
Expected: PASS

Run: `cd backend && go test ./internal/models/ -run TestWorkspaceThread_ParentThreadID -v`
Expected: PASS

- [ ] **Step 5: Whole-tree typecheck**

Run: `cd backend && go vet ./...`
Expected: no errors

- [ ] **Step 6: Commit**

```bash
git add backend/internal/models/workspace.go backend/internal/models/workspace_thread.go backend/internal/models/workspace_compress_test.go
git commit -m "feat(compression): add compression fields to Workspace and ParentThreadID to WorkspaceThread"
```

---

### Task M3: AutoMigrate ThreadCompaction and Seed Compression Defaults

**Depends on:** Task M2

**Files:**
- Modify: `backend/internal/services/db.go`
- Create: `backend/internal/services/db_compaction_test.go`

- [ ] **Step 1: Write the failing test**

```go
// backend/internal/services/db_compaction_test.go
package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/odysseythink/hermind/backend/internal/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAutoMigrate_IncludesThreadCompaction(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	defer sqlDB.Close()

	require.NoError(t, AutoMigrate(db))

	// ThreadCompaction table should exist and accept writes
	c := models.ThreadCompaction{
		WorkspaceID: 1,
		Summary:     "test",
		UpToChatID:  1,
	}
	require.NoError(t, db.Create(&c).Error)
	assert.NotZero(t, c.ID)
}

func TestSeedDefaults_SetsCompressionEnabled(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	defer sqlDB.Close()

	require.NoError(t, db.AutoMigrate(&models.SystemSetting{}))
	require.NoError(t, SeedDefaults(db))

	var s models.SystemSetting
	require.NoError(t, db.Where("`key` = ?", "context_compress_enabled").First(&s).Error)
	require.NotNil(t, s.Value)
	assert.Equal(t, "false", *s.Value)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd backend && go test ./internal/services/ -run TestAutoMigrate_IncludesThreadCompaction -v`
Expected: FAIL — `ThreadCompaction` not in AutoMigrate list, table does not exist

Run: `cd backend && go test ./internal/services/ -run TestSeedDefaults_SetsCompressionEnabled -v`
Expected: FAIL — `context_compress_enabled` not seeded

- [ ] **Step 3: Update AutoMigrate and SeedDefaults**

Modify `backend/internal/services/db.go`:

1. Add `&models.ThreadCompaction{}` to the `AutoMigrate` slice (any position, append at end is fine).

2. Add the compression default to `SeedDefaults`:

```go
	defaults := []models.SystemSetting{
		{Key: "setup_complete", Value: strPtr("false")},
		{Key: "llm_provider", Value: strPtr("openai")},
		{Key: "vector_db", Value: strPtr("lancedb")},
		{Key: "context_compress_enabled", Value: strPtr("false")},
	}
```

The full modified sections:

```go
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&models.User{},
		&models.Invite{},
		&models.APIKey{},
		&models.PasswordResetToken{},
		&models.RecoveryCode{},
		&models.Workspace{},
		&models.WorkspaceUser{},
		&models.WorkspaceChat{},
		&models.WorkspaceDocument{},
		&models.DocumentVector{},
		&models.WorkspaceThread{},
		&models.SystemSetting{},
		&models.EmbedConfig{},
		&models.EmbedChat{},
		&models.PromptPreset{},
		&models.PromptVariable{},
		&models.EventLog{},
		&models.TemporaryAuthToken{},
		&models.WorkspaceAgentInvocation{},
		&models.WorkspaceParsedFile{},
		&models.DocumentSyncQueue{},
		&models.OutlookOAuthToken{},
		&models.PromptHistory{},
		&models.ScheduledJob{},
		&models.ScheduledJobRun{},
		&models.Memory{},
		&models.ExternalCommunicationConnector{},
		&models.BrowserExtensionApiKey{},
		&models.AgentSkill{},
		&models.AgentSkillFile{},
		&models.ThreadCompaction{},
	)
}
```

```go
	defaults := []models.SystemSetting{
		{Key: "setup_complete", Value: strPtr("false")},
		{Key: "llm_provider", Value: strPtr("openai")},
		{Key: "vector_db", Value: strPtr("lancedb")},
		{Key: "context_compress_enabled", Value: strPtr("false")},
	}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd backend && go test ./internal/services/ -run TestAutoMigrate_IncludesThreadCompaction -v`
Expected: PASS

Run: `cd backend && go test ./internal/services/ -run TestSeedDefaults_SetsCompressionEnabled -v`
Expected: PASS

- [ ] **Step 5: Whole-tree typecheck**

Run: `cd backend && go vet ./...`
Expected: no errors

- [ ] **Step 6: Commit**

```bash
git add backend/internal/services/db.go backend/internal/services/db_compaction_test.go
git commit -m "feat(compression): AutoMigrate ThreadCompaction and seed context_compress_enabled default"
```

---

## Self-Review

- [ ] **1. Spec coverage**

| Design § | Requirement | Task | Status |
|---|---|---|---|
| §1.1 | Persistence `thread_compactions` | M1 | covered |
| §1.1 | Global + per-workspace switch (model fields) | M2, M3 | covered |
| §11.1 | `buildChatHistory` incremental read (needs model) | M1 | prerequisite |
| §18.2 | Cross-thread handoff (`ParentThreadID`) | M2 | covered |
| §5 | Config defaults (seed `context_compress_enabled=false`) | M3 | covered |

- [ ] **2. Placeholder scan:** No `TODO`, `TBD`, or deferred-by-dependency placeholders.
- [ ] **3. No phantom tasks:** Every task creates files and passes tests. No `--allow-empty`.
- [ ] **4. Dependency soundness:** M1 → M2 → M3. M1 has no dependencies. M2 only needs model package to compile. M3 needs M2's fields to exist in AutoMigrate but compiles independently.
- [ ] **5. Caller & build soundness:** No shared signatures changed in this sub-plan (only struct fields added, no constructors or interfaces modified). `go vet ./...` verifies whole-tree compilation including test files.
- [ ] **6. Test-the-risk:** M1 tests CRUD + nil ThreadID + latest-for-query ordering (the exact mutations that will be used by the persistence layer). M2 tests nullable field round-trips. M3 tests production migration and seeding.
- [ ] **7. Type consistency:** `ThreadCompaction.ThreadID *int` matches `WorkspaceChat.ThreadID *int`. `Workspace.CompressEnabled *bool` follows existing nullable pattern (`ChatProvider *string`, etc.).
