# Phase 2 Quick-Wins Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use gpowers:subagent-driven-development (recommended) or gpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship three independent Phase-2 features in one branch — enable the already-implemented Filesystem + Create Files agents, add pptx generation, and add prompt-change history.

**Architecture:** Three sequential PRs against the same branch.
- PR1 flips two `is-available` stubs in `handlers/agent_skill.go` to read from `config.Config` (zero downstream changes — tools already registered in `agent/tools/builder.go`).
- PR2 adds `pptx` to the existing `create-files-agent` tool via `github.com/unidoc/unioffice` (gated on license approval).
- PR3 adds a `PromptHistory` GORM model + service + 3 endpoints; hook fires before `WorkspaceService.Update` writes a new `OpenAiPrompt`.

**Tech Stack:** Go 1.23, Gin, GORM (sqlite/postgres), testify, `github.com/unidoc/unioffice` (PR2 only — AGPL-3.0/commercial).

**Source design:** `.gpowers/designs/2026-05-28-phase2-quick-wins-design.md`

**Deviation from design:** Design §4.1 mentions changing `admin.go:128, 267` defaults — that read was wrong. Lines 128/267 are *allow-list* maps (which keys can be read/written), not default values. The real default for `disabled_filesystem_skills` is already `[]any{}` (empty list = no skills disabled) at `admin.go:205`. **No `admin.go` changes are needed** — the plan reflects this.

---

## File Structure

### Created
- `backend/internal/agent/tools/create_files_pptx.go` — pptx writer using unioffice (PR2)
- `backend/internal/agent/tools/create_files_pptx_test.go` — pptx writer tests (PR2)
- `backend/internal/models/prompt_history.go` — GORM model (PR3)
- `backend/internal/services/prompt_history_service.go` — Log/List/Delete (PR3)
- `backend/internal/services/prompt_history_service_test.go` — service tests (PR3)
- `backend/internal/handlers/prompt_history.go` — 3 HTTP routes (PR3)
- `backend/internal/handlers/prompt_history_test.go` — handler tests (PR3)

### Modified
- `backend/internal/handlers/agent_skill.go` — inject `cfg`, read `cfg.AgentFilesystemEnabled` / `cfg.AgentCreateFilesEnabled` (PR1)
- `backend/internal/handlers/agent_skill_test.go` (may need creation if absent — verify in Task 1)
- `backend/cmd/server/main.go:248` — pass `cfg` to `RegisterAgentSkillRoutes` (PR1)
- `backend/internal/agent/tools/create_files_agent.go:46` — drop `pptx` early-return, call `writePptxFile` (PR2)
- `backend/go.mod` — add unioffice dep (PR2)
- `backend/internal/services/workspace_service.go:78` — extend `Update` signature with `userID *int`; emit prompt-history hook (PR3)
- `backend/internal/handlers/api_workspace.go:72` — pass `nil` userID to updated `Update` (PR3)
- `backend/internal/handlers/workspace.go:63` — pass `user.ID` to updated `Update` (PR3)
- `backend/internal/services/db.go:30` — add `&models.PromptHistory{}` to `AutoMigrate` (PR3)

---

# PR1 · Enable Filesystem + Create-Files agents (~80 LoC, no deps)

## Task 1: Verify current state and make a failing test

**Files:**
- Read: `backend/internal/handlers/agent_skill.go`
- Read: `backend/internal/handlers/agent_skill_test.go` (may not exist — Step 2 creates it)

- [ ] **Step 1: Confirm there is no existing test file**

```bash
ls backend/internal/handlers/agent_skill_test.go 2>&1
```

If "No such file" → continue to Step 2. If it exists → read it and integrate the new tests below into it instead of creating a new file.

- [ ] **Step 2: Create test scaffold + failing test**

Create `backend/internal/handlers/agent_skill_test.go`:

```go
package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestFileSystemAgentAvailable_ReadsConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cases := []struct {
		name    string
		enabled bool
		want    bool
	}{
		{"enabled", true, true},
		{"disabled", false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.Config{AgentFilesystemEnabled: tc.enabled}
			h := NewAgentSkillHandler(nil, cfg)
			r := gin.New()
			r.GET("/test", h.FileSystemAgentAvailable)
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code)
			var body map[string]bool
			assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
			assert.Equal(t, tc.want, body["available"])
		})
	}
}

func TestCreateFilesAgentAvailable_ReadsConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{AgentCreateFilesEnabled: true}
	h := NewAgentSkillHandler(nil, cfg)
	r := gin.New()
	r.GET("/test", h.CreateFilesAgentAvailable)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]bool
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.True(t, body["available"])
}
```

- [ ] **Step 3: Run test — expect compile error**

```bash
cd backend && go test ./internal/handlers/ -run TestFileSystemAgentAvailable_ReadsConfig
```

Expected: build failure complaining about `NewAgentSkillHandler` taking 1 arg, not 2.

## Task 2: Inject cfg into AgentSkillHandler and flip handlers

**Files:**
- Modify: `backend/internal/handlers/agent_skill.go`

- [ ] **Step 1: Extend constructor + handlers**

Replace `backend/internal/handlers/agent_skill.go` lines 14–30 with:

```go
type AgentSkillHandler struct {
	sysSvc *services.SystemService
	cfg    *config.Config
}

func NewAgentSkillHandler(sysSvc *services.SystemService, cfg *config.Config) *AgentSkillHandler {
	return &AgentSkillHandler{sysSvc: sysSvc, cfg: cfg}
}

func (h *AgentSkillHandler) FileSystemAgentAvailable(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"available": h.cfg.AgentFilesystemEnabled})
}

func (h *AgentSkillHandler) CreateFilesAgentAvailable(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"available": h.cfg.AgentCreateFilesEnabled})
}
```

- [ ] **Step 2: Add config import**

Edit the import block at the top of `agent_skill.go` — add the import line so the block reads:

```go
import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/middleware"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/services"
)
```

- [ ] **Step 3: Extend route registration**

In the same file, replace `RegisterAgentSkillRoutes` (lines 81–93) with:

```go
func RegisterAgentSkillRoutes(r *gin.RouterGroup, sysSvc *services.SystemService, authSvc *services.AuthService, cfg *config.Config) {
	h := NewAgentSkillHandler(sysSvc, cfg)
	r.GET("/agent-skills/filesystem-agent/is-available",
		middleware.ValidatedRequest(authSvc),
		h.FileSystemAgentAvailable)
	r.GET("/agent-skills/create-files-agent/is-available",
		middleware.ValidatedRequest(authSvc),
		h.CreateFilesAgentAvailable)
	r.POST("/agent-skills/whitelist/add",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"all"}),
		h.AddToWhitelist)
}
```

## Task 3: Wire cfg through main.go

**Files:**
- Modify: `backend/cmd/server/main.go:248`

- [ ] **Step 1: Pass cfg to route registration**

In `backend/cmd/server/main.go` find line 248:

```go
handlers.RegisterAgentSkillRoutes(api, sysSvc, authSvc)
```

Replace with:

```go
handlers.RegisterAgentSkillRoutes(api, sysSvc, authSvc, cfg)
```

(`cfg` is already in scope — verify via `grep -n '^\s*cfg\b' backend/cmd/server/main.go` returning a binding near line 60–90.)

## Task 4: Verify tests pass and codebase builds

- [ ] **Step 1: Run new tests — expect PASS**

```bash
cd backend && go test ./internal/handlers/ -run 'TestFileSystemAgentAvailable_ReadsConfig|TestCreateFilesAgentAvailable_ReadsConfig' -v
```

Expected: 3 sub-test cases PASS.

- [ ] **Step 2: Full build**

```bash
cd backend && go build ./...
```

Expected: no output, exit 0.

- [ ] **Step 3: Full test suite (sanity)**

```bash
cd backend && go test ./... 2>&1 | tail -30
```

Expected: all PASS or pre-existing failures only (compare to `git stash && go test ./... && git stash pop` if uncertain).

## Task 5: Commit PR1

- [ ] **Step 1: Stage and commit**

```bash
cd /Users/ranwei/workspace/go_work/hermes-agent-rewrite/hermind
git add backend/internal/handlers/agent_skill.go \
        backend/internal/handlers/agent_skill_test.go \
        backend/cmd/server/main.go
git commit -m "$(cat <<'EOF'
feat(agent-skills): wire filesystem + create-files is-available to config

Both agent tools were fully implemented in agent/tools/builder.go but the
GET /agent-skills/{name}/is-available endpoints hard-coded {"available":
false}, hiding them from the admin UI. Inject *config.Config into
AgentSkillHandler and return the corresponding cfg.AgentFilesystemEnabled
/ cfg.AgentCreateFilesEnabled value. No other changes — tools are already
registered with proper sandbox/approval gates.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

# PR2 · pptx via unioffice [LICENSE-GATED — do not merge until owner approves AGPL-3.0/commercial]

## Task 6: Add unioffice dependency

> **GATE:** Confirm with the project owner that AGPL-3.0 or commercial license is acceptable before continuing. If not, abort PR2 and update the design doc to say pptx remains a stub.

**Files:**
- Modify: `backend/go.mod`, `backend/go.sum`

- [ ] **Step 1: Pull latest unioffice**

```bash
cd backend && go get github.com/unidoc/unioffice@latest
```

- [ ] **Step 2: Verify module graph**

```bash
cd backend && go mod why github.com/unidoc/unioffice
```

Expected: shows direct dependency. Record the binary-size delta — note it in the PR description later via:

```bash
go build -o /tmp/srv-after ./cmd/server/ && du -h /tmp/srv-after
```

(Compare to pre-PR2 size if available.)

## Task 7: Write failing test for writePptxFile

**Files:**
- Create: `backend/internal/agent/tools/create_files_pptx_test.go`

- [ ] **Step 1: Write the test**

```go
package tools

import (
	"archive/zip"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWritePptxFile_SingleSlide(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "single.pptx")
	err := writePptxFile(context.Background(), dst, "Hello world\nBody line one", "single")
	require.NoError(t, err)
	info, err := os.Stat(dst)
	require.NoError(t, err)
	assert.Greater(t, info.Size(), int64(1024)) // pptx zips are >1KB

	r, err := zip.OpenReader(dst)
	require.NoError(t, err)
	defer r.Close()
	var slideCount int
	for _, f := range r.File {
		if strings.HasPrefix(f.Name, "ppt/slides/slide") && strings.HasSuffix(f.Name, ".xml") {
			slideCount++
		}
	}
	assert.Equal(t, 1, slideCount)
}

func TestWritePptxFile_ThreeSlides(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "three.pptx")
	body := "Slide one\nBody A\n---\nSlide two\nBody B\n---\nSlide three"
	err := writePptxFile(context.Background(), dst, body, "three")
	require.NoError(t, err)

	r, err := zip.OpenReader(dst)
	require.NoError(t, err)
	defer r.Close()
	var slideCount int
	for _, f := range r.File {
		if strings.HasPrefix(f.Name, "ppt/slides/slide") && strings.HasSuffix(f.Name, ".xml") {
			slideCount++
		}
	}
	assert.Equal(t, 3, slideCount)
}

func TestWritePptxFile_EmptyContent(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "empty.pptx")
	err := writePptxFile(context.Background(), dst, "", "empty")
	require.NoError(t, err)
	_, err = os.Stat(dst)
	assert.NoError(t, err)
}
```

- [ ] **Step 2: Run — expect compile error**

```bash
cd backend && go test ./internal/agent/tools/ -run TestWritePptxFile
```

Expected: undefined `writePptxFile`.

## Task 8: Implement writePptxFile

**Files:**
- Create: `backend/internal/agent/tools/create_files_pptx.go`

- [ ] **Step 1: Write the implementation**

```go
package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/unidoc/unioffice/presentation"
)

// writePptxFile creates a .pptx file at dst. The content is split into slides
// by lines containing exactly "---"; each slide's first non-empty line becomes
// its title, remaining lines become bullets. An empty content produces a
// single blank slide.
func writePptxFile(ctx context.Context, dst, content, _ string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	ppt := presentation.New()
	defer ppt.Close()

	slides := splitSlides(content)
	if len(slides) == 0 {
		slides = []slideContent{{Title: "", Body: nil}}
	}

	for _, sl := range slides {
		s, err := ppt.AddDefaultSlideWithLayout()
		if err != nil {
			return fmt.Errorf("add slide: %w", err)
		}
		if sl.Title != "" {
			// First placeholder = title
			placeholders := s.PlaceHolders()
			if len(placeholders) > 0 {
				placeholders[0].SetText(sl.Title)
			}
		}
		if len(sl.Body) > 0 && len(s.PlaceHolders()) > 1 {
			s.PlaceHolders()[1].SetText(strings.Join(sl.Body, "\n"))
		}
	}

	if err := ppt.SaveToFile(dst); err != nil {
		return fmt.Errorf("save pptx: %w", err)
	}
	return nil
}

type slideContent struct {
	Title string
	Body  []string
}

func splitSlides(content string) []slideContent {
	if strings.TrimSpace(content) == "" {
		return nil
	}
	chunks := strings.Split(content, "\n---\n")
	out := make([]slideContent, 0, len(chunks))
	for _, chunk := range chunks {
		lines := strings.Split(chunk, "\n")
		var title string
		var body []string
		for i, l := range lines {
			if strings.TrimSpace(l) == "" {
				continue
			}
			if title == "" {
				title = l
				body = append(body, lines[i+1:]...)
				break
			}
		}
		out = append(out, slideContent{Title: title, Body: body})
	}
	return out
}
```

- [ ] **Step 2: Run tests — expect PASS**

```bash
cd backend && go test ./internal/agent/tools/ -run TestWritePptxFile -v
```

Expected: 3 tests PASS.

> **If unioffice's `presentation.New()` API differs from this exact signature** (the library has changed shape across versions), prefer the version's documented "create new presentation" entry point and re-derive the slide-add call. Keep the slide-counting test as the contract — implementation may rotate around the library version.

## Task 9: Drop pptx stub from create_files_agent.go

**Files:**
- Modify: `backend/internal/agent/tools/create_files_agent.go`

- [ ] **Step 1: Read current pptx branch**

```bash
grep -nB1 -A5 'pptx' backend/internal/agent/tools/create_files_agent.go
```

You'll find a switch case around line 46 that returns an error for `pptx`. Replace it with a real call.

- [ ] **Step 2: Wire writePptxFile into the dispatch**

Locate the format switch in `create_files_agent.go` (search for `case "pptx"`). Replace the early-return block with the docx-style branch:

```go
case "pptx":
	contentStr, _ := args.Content.(string)
	if err := writePptxFile(ctx, dst, contentStr, args.Filename); err != nil {
		return tool.Error(err.Error()), nil
	}
```

Also delete the format-validation early-return for `pptx` higher up (search for `"pptx format not supported"`) — adjust the allowed-format switch (`case "txt", "md", "docx", "pdf", "xlsx":`) to add `"pptx"`.

- [ ] **Step 3: Build + test**

```bash
cd backend && go build ./... && go test ./internal/agent/tools/ -v
```

Expected: all PASS.

## Task 10: Commit PR2

- [ ] **Step 1: Verify GATE — license approved**

This commit must NOT be made if license approval is still pending. The work can sit on the branch (uncommitted) until then.

- [ ] **Step 2: Stage and commit**

```bash
git add backend/go.mod backend/go.sum \
        backend/internal/agent/tools/create_files_pptx.go \
        backend/internal/agent/tools/create_files_pptx_test.go \
        backend/internal/agent/tools/create_files_agent.go
git commit -m "$(cat <<'EOF'
feat(create-files): generate pptx via unioffice

Drop the "pptx format not supported" stub; route pptx through a new
writePptxFile helper backed by github.com/unidoc/unioffice. Slides are
split on "---" lines; first non-empty line is the title; remaining lines
become the body.

License: unioffice is AGPL-3.0 / commercial. Owner has signed off; see PR
description for details.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

# PR3 · Prompt History (~250 LoC, no deps)

## Task 11: Add PromptHistory GORM model

**Files:**
- Create: `backend/internal/models/prompt_history.go`
- Modify: `backend/internal/services/db.go:30`

- [ ] **Step 1: Create the model**

```go
package models

import "time"

type PromptHistory struct {
	ID          int       `gorm:"primaryKey;autoIncrement" json:"id"`
	WorkspaceID int       `gorm:"index;not null" json:"workspaceId"`
	Prompt      string    `gorm:"type:text;not null" json:"prompt"`
	ModifiedBy  *int      `gorm:"index" json:"modifiedBy,omitempty"`
	ModifiedAt  time.Time `gorm:"autoCreateTime" json:"modifiedAt"`
}

// TableName matches the anything-llm prisma model name for parity.
func (PromptHistory) TableName() string { return "prompt_history" }
```

- [ ] **Step 2: Register in AutoMigrate**

In `backend/internal/services/db.go:30`, locate the `AutoMigrate` call and add `&models.PromptHistory{}` to the list (alphabetical or end is fine):

```go
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&models.User{},
		&models.Invite{},
		// ... existing entries unchanged ...
		&models.OutlookOAuthToken{},
		&models.PromptHistory{},
	)
}
```

- [ ] **Step 3: Verify migration runs**

```bash
cd backend && go build ./... && go test ./internal/services/ -run 'TestSomethingThat MigratesDB' 2>&1 | head -20
```

(If no such test exists, the build is enough — AutoMigrate is exercised by any integration test.)

## Task 12: PromptHistoryService — write failing tests, then implement

**Files:**
- Create: `backend/internal/services/prompt_history_service.go`
- Create: `backend/internal/services/prompt_history_service_test.go`

- [ ] **Step 1: Write failing tests**

Create `backend/internal/services/prompt_history_service_test.go`:

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

func newPHTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.PromptHistory{}))
	return db
}

func TestPromptHistoryService_LogAndList(t *testing.T) {
	db := newPHTestDB(t)
	svc := NewPromptHistoryService(db)
	uid := 7

	require.NoError(t, svc.Log(context.Background(), 100, "old prompt one", &uid))
	require.NoError(t, svc.Log(context.Background(), 100, "old prompt two", &uid))
	require.NoError(t, svc.Log(context.Background(), 200, "different workspace", nil))

	rows, err := svc.ListByWorkspace(context.Background(), 100, 10)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	// DESC order — newest first
	assert.Equal(t, "old prompt two", rows[0].Prompt)
	assert.Equal(t, "old prompt one", rows[1].Prompt)
	assert.Equal(t, 7, *rows[0].ModifiedBy)
}

func TestPromptHistoryService_Delete(t *testing.T) {
	db := newPHTestDB(t)
	svc := NewPromptHistoryService(db)

	require.NoError(t, svc.Log(context.Background(), 1, "p1", nil))
	require.NoError(t, svc.Log(context.Background(), 1, "p2", nil))

	rows, _ := svc.ListByWorkspace(context.Background(), 1, 10)
	require.Len(t, rows, 2)
	require.NoError(t, svc.Delete(context.Background(), rows[0].ID))
	rows, _ = svc.ListByWorkspace(context.Background(), 1, 10)
	assert.Len(t, rows, 1)
}

func TestPromptHistoryService_DeleteAll(t *testing.T) {
	db := newPHTestDB(t)
	svc := NewPromptHistoryService(db)
	require.NoError(t, svc.Log(context.Background(), 1, "p1", nil))
	require.NoError(t, svc.Log(context.Background(), 1, "p2", nil))
	require.NoError(t, svc.Log(context.Background(), 2, "p3", nil))

	require.NoError(t, svc.DeleteAll(context.Background(), 1))
	rows, _ := svc.ListByWorkspace(context.Background(), 1, 10)
	assert.Len(t, rows, 0)
	rows, _ = svc.ListByWorkspace(context.Background(), 2, 10)
	assert.Len(t, rows, 1)
}

func TestPromptHistoryService_ListLimit(t *testing.T) {
	db := newPHTestDB(t)
	svc := NewPromptHistoryService(db)
	for i := 0; i < 5; i++ {
		require.NoError(t, svc.Log(context.Background(), 1, "p", nil))
	}
	rows, err := svc.ListByWorkspace(context.Background(), 1, 3)
	require.NoError(t, err)
	assert.Len(t, rows, 3)
}
```

- [ ] **Step 2: Run — expect compile error**

```bash
cd backend && go test ./internal/services/ -run TestPromptHistoryService
```

Expected: undefined `NewPromptHistoryService`.

- [ ] **Step 3: Implement the service**

Create `backend/internal/services/prompt_history_service.go`:

```go
package services

import (
	"context"

	"github.com/odysseythink/hermind/backend/internal/models"
	"gorm.io/gorm"
)

type PromptHistoryService struct {
	db *gorm.DB
}

func NewPromptHistoryService(db *gorm.DB) *PromptHistoryService {
	return &PromptHistoryService{db: db}
}

// Log persists a prior prompt for a workspace. Failure is returned but should
// typically be treated as non-fatal by callers (the workspace update itself
// must not be blocked).
func (s *PromptHistoryService) Log(ctx context.Context, workspaceID int, prevPrompt string, userID *int) error {
	row := models.PromptHistory{
		WorkspaceID: workspaceID,
		Prompt:      prevPrompt,
		ModifiedBy:  userID,
	}
	return s.db.WithContext(ctx).Create(&row).Error
}

// ListByWorkspace returns the most recent N rows for a workspace, newest first.
// limit <= 0 returns at most 50 rows (matching anything-llm's implicit cap).
func (s *PromptHistoryService) ListByWorkspace(ctx context.Context, workspaceID int, limit int) ([]models.PromptHistory, error) {
	if limit <= 0 {
		limit = 50
	}
	var rows []models.PromptHistory
	err := s.db.WithContext(ctx).
		Where("workspace_id = ?", workspaceID).
		Order("modified_at DESC").
		Limit(limit).
		Find(&rows).Error
	return rows, err
}

func (s *PromptHistoryService) Delete(ctx context.Context, id int) error {
	return s.db.WithContext(ctx).Delete(&models.PromptHistory{}, id).Error
}

func (s *PromptHistoryService) DeleteAll(ctx context.Context, workspaceID int) error {
	return s.db.WithContext(ctx).
		Where("workspace_id = ?", workspaceID).
		Delete(&models.PromptHistory{}).Error
}
```

- [ ] **Step 4: Run tests — expect PASS**

```bash
cd backend && go test ./internal/services/ -run TestPromptHistoryService -v
```

Expected: 4 tests PASS.

## Task 13: Extend WorkspaceService.Update to emit prompt-history hook

**Files:**
- Modify: `backend/internal/services/workspace_service.go:17-24, 78-116`

- [ ] **Step 1: Inject PromptHistoryService into WorkspaceService**

In `backend/internal/services/workspace_service.go` replace the struct (lines 17–24):

```go
type WorkspaceService struct {
	db        *gorm.DB
	cfg       *config.Config
	phSvc     *PromptHistoryService
	defaultPrompt string
}

func NewWorkspaceService(db *gorm.DB, cfg *config.Config, phSvc *PromptHistoryService) *WorkspaceService {
	return &WorkspaceService{
		db:            db,
		cfg:           cfg,
		phSvc:         phSvc,
		defaultPrompt: "Given the following conversation, relevant context, and a follow up question, reply with an answer to the current question the user is asking.",
	}
}
```

- [ ] **Step 2: Extend Update signature and add hook**

Replace `Update` (lines 78–116) with:

```go
func (s *WorkspaceService) Update(ctx context.Context, slug string, req dto.UpdateWorkspaceRequest, userID *int) error {
	var ws models.Workspace
	if err := s.db.Where("slug = ?", slug).First(&ws).Error; err != nil {
		return err
	}

	// Mirror anything-llm's 4-condition prompt-history hook (server/models/workspace.js:526–532):
	// fires when new prompt is non-empty AND prev was non-empty AND prev != defaultPrompt AND prev != new.
	if req.OpenAiPrompt != nil && *req.OpenAiPrompt != "" &&
		ws.OpenAiPrompt != nil && *ws.OpenAiPrompt != "" &&
		*ws.OpenAiPrompt != s.defaultPrompt &&
		*ws.OpenAiPrompt != *req.OpenAiPrompt {
		if s.phSvc != nil {
			// Non-fatal — log on failure but never block the update.
			_ = s.phSvc.Log(ctx, ws.ID, *ws.OpenAiPrompt, userID)
		}
	}

	updates := map[string]any{}
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.OpenAiTemp != nil {
		updates["open_ai_temp"] = *req.OpenAiTemp
	}
	if req.OpenAiHistory > 0 {
		updates["open_ai_history"] = req.OpenAiHistory
	}
	if req.OpenAiPrompt != nil {
		updates["open_ai_prompt"] = *req.OpenAiPrompt
	}
	if req.SimilarityThreshold != nil {
		updates["similarity_threshold"] = *req.SimilarityThreshold
	}
	if req.ChatProvider != nil {
		updates["chat_provider"] = *req.ChatProvider
	}
	if req.ChatModel != nil {
		updates["chat_model"] = *req.ChatModel
	}
	if req.TopN != nil {
		updates["top_n"] = *req.TopN
	}
	if req.ChatMode != nil {
		updates["chat_mode"] = *req.ChatMode
	}
	if req.QueryRefusalResponse != nil {
		updates["query_refusal_response"] = *req.QueryRefusalResponse
	}
	updates["last_updated_at"] = time.Now()
	return s.db.Model(&ws).Updates(updates).Error
}
```

- [ ] **Step 3: Write failing test for the hook (4 conditions)**

Create or append to `backend/internal/services/workspace_service_test.go`:

```go
func TestWorkspaceService_Update_PromptHistoryHook(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Workspace{}, &models.PromptHistory{}))
	phSvc := NewPromptHistoryService(db)
	svc := NewWorkspaceService(db, &config.Config{}, phSvc)

	def := svc.defaultPrompt
	old := "Custom prompt v1"
	newP := "Custom prompt v2"

	cases := []struct {
		name        string
		prevPrompt  *string
		newPrompt   *string
		expectRow   bool
	}{
		{"both set, distinct, not default", &old, &newP, true},
		{"prev is default", &def, &newP, false},
		{"prev is nil", nil, &newP, false},
		{"prev equals new", &old, &old, false},
		{"new is empty string", &old, strPtrFunc(""), false},
		{"new is nil", &old, nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ws := models.Workspace{
				Name: "t", Slug: "s-" + tc.name, OpenAiPrompt: tc.prevPrompt,
			}
			require.NoError(t, db.Create(&ws).Error)
			uid := 1
			err := svc.Update(context.Background(), ws.Slug,
				dto.UpdateWorkspaceRequest{OpenAiPrompt: tc.newPrompt}, &uid)
			require.NoError(t, err)
			var count int64
			db.Model(&models.PromptHistory{}).Where("workspace_id = ?", ws.ID).Count(&count)
			if tc.expectRow {
				assert.Equal(t, int64(1), count, "expected one history row")
			} else {
				assert.Equal(t, int64(0), count, "expected no history row")
			}
		})
	}
}

func strPtrFunc(s string) *string { return &s }
```

- [ ] **Step 4: Run hook test — expect PASS**

```bash
cd backend && go test ./internal/services/ -run TestWorkspaceService_Update_PromptHistoryHook -v
```

Expected: 6 sub-cases PASS.

## Task 14: Update Update() callers to pass userID

**Files:**
- Modify: `backend/internal/handlers/workspace.go:63`
- Modify: `backend/internal/handlers/api_workspace.go:72`

- [ ] **Step 1: UI handler — pass user.ID**

In `backend/internal/handlers/workspace.go` find the `UpdateWorkspace` (or similar) method around line 63. The pattern is:

```go
user := c.MustGet("user").(*models.User)
// ... bind req ...
if err := h.wsSvc.Update(c.Request.Context(), ws.Slug, req); err != nil {
```

Change the `Update` call to:

```go
if err := h.wsSvc.Update(c.Request.Context(), ws.Slug, req, &user.ID); err != nil {
```

- [ ] **Step 2: API v1 handler — pass nil**

In `backend/internal/handlers/api_workspace.go:72`:

```go
if err := h.wsSvc.Update(c.Request.Context(), ws.Slug, req); err != nil {
```

Change to:

```go
if err := h.wsSvc.Update(c.Request.Context(), ws.Slug, req, nil); err != nil {
```

(API key context has no user — `nil` is the correct value.)

- [ ] **Step 3: Update all WorkspaceService constructor call sites**

`NewWorkspaceService` now takes 3 args. Find all callers:

```bash
grep -rn 'NewWorkspaceService(' backend/ --include='*.go'
```

For each match, add `, phSvc` as the third argument. Expected call sites: `cmd/server/main.go`, and any test helpers. In `cmd/server/main.go`, the construction order is:

```go
phSvc := services.NewPromptHistoryService(db)
wsSvc := services.NewWorkspaceService(db, cfg, phSvc)
```

Add the `phSvc` line right before `wsSvc` initialization (search for the existing `NewWorkspaceService` call).

- [ ] **Step 4: Build**

```bash
cd backend && go build ./...
```

Expected: no errors. If a test fixture or service_test.go has its own `NewWorkspaceService(db, cfg)` call, update it to pass `nil` for the third arg (or create a noop phSvc — `nil` is fine because the hook code checks `s.phSvc != nil`).

## Task 15: Add 3 HTTP endpoints

**Files:**
- Create: `backend/internal/handlers/prompt_history.go`

- [ ] **Step 1: Create handler**

```go
package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/dto"
	"github.com/odysseythink/hermind/backend/internal/middleware"
	"github.com/odysseythink/hermind/backend/internal/services"
)

type PromptHistoryHandler struct {
	phSvc *services.PromptHistoryService
	wsSvc *services.WorkspaceService
}

func NewPromptHistoryHandler(phSvc *services.PromptHistoryService, wsSvc *services.WorkspaceService) *PromptHistoryHandler {
	return &PromptHistoryHandler{phSvc: phSvc, wsSvc: wsSvc}
}

func (h *PromptHistoryHandler) List(c *gin.Context) {
	slug := c.Param("slug")
	ws, err := h.wsSvc.GetBySlug(c.Request.Context(), slug)
	if err != nil {
		c.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "workspace not found"})
		return
	}
	rows, err := h.phSvc.ListByWorkspace(c.Request.Context(), ws.ID, 50)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"history": rows})
}

func (h *PromptHistoryHandler) DeleteAll(c *gin.Context) {
	slug := c.Param("slug")
	ws, err := h.wsSvc.GetBySlug(c.Request.Context(), slug)
	if err != nil {
		c.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "workspace not found"})
		return
	}
	if err := h.phSvc.DeleteAll(c.Request.Context(), ws.ID); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *PromptHistoryHandler) DeleteOne(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid id"})
		return
	}
	if err := h.phSvc.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func RegisterPromptHistoryRoutes(r *gin.RouterGroup, phSvc *services.PromptHistoryService, wsSvc *services.WorkspaceService, authSvc *services.AuthService) {
	h := NewPromptHistoryHandler(phSvc, wsSvc)
	r.GET("/workspace/:slug/prompt-history",
		middleware.ValidatedRequest(authSvc),
		h.List)
	r.DELETE("/workspace/:slug/prompt-history",
		middleware.ValidatedRequest(authSvc),
		h.DeleteAll)
	r.DELETE("/workspace/prompt-history/:id",
		middleware.ValidatedRequest(authSvc),
		h.DeleteOne)
}
```

- [ ] **Step 2: Wire routes into main.go**

In `cmd/server/main.go` find where other workspace routes are registered (search `RegisterWorkspaceRoutes`). Add immediately after:

```go
handlers.RegisterPromptHistoryRoutes(api, phSvc, wsSvc, authSvc)
```

## Task 16: Integration test for endpoints

**Files:**
- Create: `backend/internal/handlers/prompt_history_test.go`

- [ ] **Step 1: Write integration test**

```go
package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

func newPHHandlerTestEnv(t *testing.T) (*gin.Engine, *gorm.DB) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Workspace{}, &models.PromptHistory{}, &models.User{}))
	phSvc := services.NewPromptHistoryService(db)
	wsSvc := services.NewWorkspaceService(db, &config.Config{}, phSvc)
	authSvc := services.NewAuthService(db, &config.Config{Secret: "test"})

	r := gin.New()
	// Inject a fake user so middleware.ValidatedRequest sees an authenticated context.
	// For this test we attach the bearer token path directly:
	r.Use(func(c *gin.Context) {
		c.Set("user", &models.User{ID: 1, Role: "admin"})
		c.Next()
	})
	api := r.Group("/api")
	handlers.RegisterPromptHistoryRoutes(api, phSvc, wsSvc, authSvc)
	return r, db
}

func TestPromptHistoryEndpoints_RoundTrip(t *testing.T) {
	r, db := newPHHandlerTestEnv(t)

	ws := models.Workspace{Name: "x", Slug: "x"}
	require.NoError(t, db.Create(&ws).Error)
	require.NoError(t, db.Create(&models.PromptHistory{WorkspaceID: ws.ID, Prompt: "p1"}).Error)
	require.NoError(t, db.Create(&models.PromptHistory{WorkspaceID: ws.ID, Prompt: "p2"}).Error)

	// LIST
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/workspace/x/prompt-history", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	var listBody struct{ History []models.PromptHistory }
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &listBody))
	assert.Len(t, listBody.History, 2)

	// DELETE ONE
	firstID := listBody.History[0].ID
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, "/api/workspace/prompt-history/"+itoa(firstID), nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// DELETE ALL
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, "/api/workspace/x/prompt-history", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	rows, _ := services.NewPromptHistoryService(db).ListByWorkspace(context.Background(), ws.ID, 10)
	assert.Len(t, rows, 0)
}

func itoa(i int) string {
	return strconv.Itoa(i)
}
```

Note: the test bypasses real auth middleware via a stub gin handler that injects a user. If the project has a standard test-auth helper, use that instead — search:

```bash
grep -rn 'TestMain\|newTestEnv\|TestAuth' backend/internal/handlers/ | head -5
```

and adopt the existing pattern.

- [ ] **Step 2: Run integration test**

```bash
cd backend && go test ./internal/handlers/ -run TestPromptHistoryEndpoints_RoundTrip -v
```

Expected: PASS.

## Task 17: Full build + commit PR3

- [ ] **Step 1: Full build + suite**

```bash
cd backend && go build ./... && go test ./... 2>&1 | tail -30
```

Expected: all PASS.

- [ ] **Step 2: Commit**

```bash
git add backend/internal/models/prompt_history.go \
        backend/internal/services/db.go \
        backend/internal/services/prompt_history_service.go \
        backend/internal/services/prompt_history_service_test.go \
        backend/internal/services/workspace_service.go \
        backend/internal/services/workspace_service_test.go \
        backend/internal/handlers/workspace.go \
        backend/internal/handlers/api_workspace.go \
        backend/internal/handlers/prompt_history.go \
        backend/internal/handlers/prompt_history_test.go \
        backend/cmd/server/main.go
git commit -m "$(cat <<'EOF'
feat(workspace): persist prompt-history with rotate-on-update hook

Mirror anything-llm's PromptHistory: every workspace.openAiPrompt change
that is non-trivial (prev was non-default, prev != new, new non-empty)
logs the prior prompt into a new prompt_history table. Adds three routes
under /workspace/{:slug}/prompt-history matching the upstream surface.

Restore happens via the existing workspace update endpoint — there is no
dedicated restore route, again matching upstream.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Self-Review

Run through each spec section and confirm a task covers it:

| Spec §             | Tasks               |
|--------------------|---------------------|
| §4.1 PR1 wiring    | 1, 2, 3, 4, 5       |
| §4.2 PR2 pptx      | 6, 7, 8, 9, 10      |
| §4.3 PR3 model     | 11                  |
| §4.3 PR3 service   | 12                  |
| §4.3 PR3 hook      | 13                  |
| §4.3 PR3 endpoints | 14, 15              |
| §4.3 PR3 testing   | 16                  |
| §6 done criteria   | 4, 9, 17 (build + test sweeps) |

No `TBD`, no `TODO`, no "implement later". Each task has exact file paths and full code blocks. Function signatures (`NewPromptHistoryService`, `Log/ListByWorkspace/Delete/DeleteAll`) match across Tasks 12 → 13 → 15. The `Update` signature change in Task 13 is propagated to all callers in Task 14.

One deferred decision is preserved as a runtime fork: Task 8 notes the unioffice `presentation.New()` API may rotate across versions; the test in Task 7 is the contract, implementation should follow current library shape.

---

## Execution Handoff

Plan complete and saved to `.gpowers/plans/2026-05-28-phase2-quick-wins.md`. Two execution options:

1. **Subagent-Driven (recommended)** — I dispatch a fresh subagent per task, review between tasks, fast iteration
2. **Inline Execution** — Execute tasks in this session using executing-plans, batch execution with checkpoints

Pick when you're ready.
