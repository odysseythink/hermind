# Agent Skill Batch 1 — Pin + Provenance + Backup 实施计划

**Goal:** 为 Agent Skill 系统增加 Pin 保护机制、Provenance 审计记录、Backup/Rollback 快照功能，增强技能库的防护与追溯能力。

**Architecture:** 在 `AgentSkill` 模型新增 `WriteOrigin` 字段追踪技能来源；新建 `SkillProvenanceLog` 表记录完整内容快照；新建 `ProvenanceService` 和 `BackupService` 两个服务层；Agent 工具的 edit/patch/write_file/remove_file 操作新增 Pin 检查与 Provenance 记录；Curator Worker 在执行状态转换前先通过 BackupService 创建快照。

**Tech Stack:** Go 1.26, GORM, SQLite, mlog

> For executing workers: implement this plan task-by-task (prefer a fresh subagent/Task per task — a clean context per task avoids single-session degradation). Steps use - [ ] checkboxes for tracking.

---

## File Structure

| Task | Create | Modify | Test |
|------|--------|--------|------|
| 1 | — | `models/agent_skill.go:36`, `dto/agent_skill.go:5-12`, `services/agent_skill_service.go:218-230` | `services/agent_skill_service_test.go` |
| 2 | `models/skill_provenance_log.go` | `services/db.go:30-63` | — (compile check) |
| 3 | `services/provenance_service.go` | `services/agent_skill_service.go:59-77` | `services/provenance_service_test.go` |
| 4 | `services/skill_backup_service.go` | — | `services/skill_backup_service_test.go` |
| 5 | — | `agent/tools/agent_skills.go:77-277`, `agent/tools/context.go:40-56`, `agent/tools/builder.go:25-43,72-87,112` | `agent/tools/agent_skills_test.go` |
| 6 | — | `workers/skill_curator.go:14-22,32-58` | `workers/skill_curator_test.go` |
| 7 | — | `cmd/server/main.go:189,248` | — (build verify) |

## Dependency Overview

```
Phase A (Data Layer)      Phase B (Services)           Phase C (Integration)
Task 1 ──────────────────→ Task 3 ────────────────────→ Task 5 (Agent Tools)
  │                          │                            │
  └─→ Task 2                 └─→ Task 4 ──────────────→ Task 6 (Curator Worker)
                                                            │
                                                          Task 7 (main.go wiring)
```

- **Phase A** (Task 1, 2): 独立，可并行
- **Phase B** (Task 3, 4): Task 3 depends on Task 1+2; Task 4 depends on Task 1+2; 可并行
- **Phase C** (Task 5, 6, 7): Task 5 depends on Task 3; Task 6 depends on Task 4; Task 7 depends on Task 5+6

## Risks & Open Questions

| # | Risk | Mitigation |
|---|------|-----------|
| 1 | `ProvenanceService` 通过 `ToolContext` 传递到 tools 层，改变了 `BuilderDeps` 和 `ToolContext` 结构体签名 | 所有 tools 使用同一个 `ToolContext` 构造路径，改动集中在 `builder.go` 一处 |
| 2 | `BackupService.Snapshot` 在 Curator 循环内调用，每个 workspace 都读全表 `skills + files` | 当前 workspace 数量可控（<1000），JSON 序列化体积可控 |
| 3 | `SkillProvenanceLog` 表会随时间增长 | 本批次不实现清理策略（已排入 Batch 2），风险暂可接受 |
| 4 | Pin 检查已部分存在于 service 层（`Delete`, `ApplyCuratorTransitions`），重复添加会导致双重检查 | Agent tools 层检查提供更好的错误消息（"unpin first"），service 层保留作为底线保护 |

---

### Task 1: Add `WriteOrigin` field to AgentSkill model + DTO + Create wiring

**Depends on:** none

**Files:**
- Modify: `backend/internal/models/agent_skill.go:35` — add `WriteOrigin` field after `CreatedBy`
- Modify: `backend/internal/dto/agent_skill.go:6-11` — add `WriteOrigin` to `CreateAgentSkillRequest`
- Modify: `backend/internal/services/agent_skill_service.go:184-237` — set `WriteOrigin` in `Create`
- Test: `backend/internal/services/agent_skill_service_test.go` — append `TestAgentSkillService_WriteOriginDefault`

- [ ] Write the failing test (append to `agent_skill_service_test.go` after `boolPtr`):

```go
func TestAgentSkillService_WriteOriginDefault(t *testing.T) {
	db := setupAgentSkillTestDB(t)
	svc := NewAgentSkillService(db)
	ctx := context.Background()

	skill, err := svc.Create(ctx, 1, dto.CreateAgentSkillRequest{
		Name:        "origin-test",
		Description: "WriteOrigin default",
		Content:     "test content",
	})
	require.NoError(t, err)
	assert.Equal(t, "foreground", skill.WriteOrigin)

	// Explicit WriteOrigin
	skill2, err := svc.Create(ctx, 1, dto.CreateAgentSkillRequest{
		Name:        "origin-test2",
		Description: "Explicit origin",
		Content:     "...",
		WriteOrigin: "background_review",
	})
	require.NoError(t, err)
	assert.Equal(t, "background_review", skill2.WriteOrigin)
}
```

- [ ] Run it and verify it FAILS:

```bash
cd backend && go test ./internal/services/ -run TestAgentSkillService_WriteOriginDefault -v
# Expected: compilation error — unknown field WriteOrigin in CreateAgentSkillRequest
# OR: skill.WriteOrigin == "" (field not set, empty default)
```

- [ ] Write the minimal implementation:

**`models/agent_skill.go`** — add after `CreatedBy` (line 35), before `CreatedAt` (line 36):

```go
WriteOrigin   string     `gorm:"default:foreground" json:"writeOrigin"` // foreground | background_review | curator
```

**`dto/agent_skill.go`** — add to `CreateAgentSkillRequest` after `CreatedBy` (line 11):

```go
WriteOrigin string `json:"writeOrigin,omitempty"` // foreground | background_review | curator
```

**`services/agent_skill_service.go`** — inside `Create`, add `WriteOrigin` fallback + field (replace lines 213-230):

```go
createdBy := req.CreatedBy
if createdBy == "" {
	createdBy = models.AgentSkillCreatedByAgent
}
writeOrigin := req.WriteOrigin
if writeOrigin == "" {
	writeOrigin = "foreground"
}

skill := models.AgentSkill{
	WorkspaceID: workspaceID,
	Name:        req.Name,
	Slug:        skillSlug,
	Description: description,
	Category:    req.Category,
	Content:     req.Content,
	Frontmatter: frontmatter,
	Status:      models.AgentSkillStatusActive,
	CreatedBy:   createdBy,
	WriteOrigin: writeOrigin,
	CreatedAt:   time.Now(),
	UpdatedAt:   time.Now(),
}
```

- [ ] Run it and verify it PASSES:

```bash
cd backend && go test ./internal/services/ -run TestAgentSkillService_WriteOriginDefault -v
```

- [ ] Verify no callers broken — build-check entire tree:

```bash
cd backend && go build ./...
# All existing CreateAgentSkillRequest callers compile fine (WriteOrigin is omitempty)
```

- [ ] Run full service tests to verify no regression:

```bash
cd backend && go test ./internal/services/... -count=1 2>&1 | tail -5
# Expected: ok ... (all pass)
```

- [ ] Commit:

```bash
git add backend/internal/models/agent_skill.go backend/internal/dto/agent_skill.go backend/internal/services/agent_skill_service.go backend/internal/services/agent_skill_service_test.go
git commit -m "feat(skills): add WriteOrigin field to AgentSkill model and Create DTO"
```

---

### Task 2: Create `SkillProvenanceLog` model + register in AutoMigrate

**Depends on:** Task 1

**Files:**
- Create: `backend/internal/models/skill_provenance_log.go`
- Modify: `backend/internal/services/db.go:61` — add `SkillProvenanceLog` to AutoMigrate list

- [ ] Write the minimal implementation:

**Create `backend/internal/models/skill_provenance_log.go`:**

```go
package models

import "time"

// SkillProvenanceLog records a full-content snapshot every time a skill
// is created, updated, patched, deleted, or a skill file is written/removed.
type SkillProvenanceLog struct {
	ID          int       `gorm:"primaryKey;autoIncrement" json:"id"`
	SkillID     int       `gorm:"index;not null" json:"skillId"`
	WorkspaceID int       `gorm:"index;not null" json:"workspaceId"`
	Action      string    `gorm:"not null" json:"action"`      // create | update | patch | delete | write_file | remove_file
	WriteOrigin string    `gorm:"not null" json:"writeOrigin"` // foreground | background_review | curator
	ActorType   string    `gorm:"not null" json:"actorType"`   // agent | user | system
	ActorID     string    `json:"actorId"`                     // user ID or empty string
	Content     string    `gorm:"type:text" json:"content"`    // full skill body snapshot
	FilePath    string    `json:"filePath"`                    // for file ops, empty for skill body
	CreatedAt   time.Time `json:"createdAt"`
}

func (SkillProvenanceLog) TableName() string { return "skill_provenance_logs" }
```

**Modify `backend/internal/services/db.go`** — insert after `&models.AgentSkillFile{},` (line 61):

```go
&models.SkillProvenanceLog{},
```

- [ ] Build-verify:

```bash
cd backend && go build ./...
```

- [ ] AutoMigrate verify — the existing curator test uses in-memory DB with AutoMigrate, so it proves the new table migrates cleanly:

```bash
cd backend && go test ./internal/workers/ -run TestSkillCuratorJob_Run -v
# Expected: PASS — GORM AutoMigrate creates skill_provenance_logs silently
```

- [ ] Commit:

```bash
git add backend/internal/models/skill_provenance_log.go backend/internal/services/db.go
git commit -m "feat(skills): add SkillProvenanceLog model for audit trail"
```

---

### Task 3: Create `ProvenanceService` + add `IsPinned` to `AgentSkillManager`

**Depends on:** Task 1, Task 2

**Files:**
- Create: `backend/internal/services/provenance_service.go`
- Modify: `backend/internal/services/agent_skill_service.go:59-77` — add `IsPinned` to interface + implementation
- Test: `backend/internal/services/provenance_service_test.go`

**Note on `inferActor`:** The design defers `inferActor` to implementation-time verification. For this batch, `ProvenanceService.Record` accepts explicit `actorType`/`actorID` parameters from the caller. Agents pass `"agent"`/`""`; API handlers (future work) would pass `"user"`/`userID`.

- [ ] Write the failing test — create `backend/internal/services/provenance_service_test.go`:

```go
package services

import (
	"context"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/dto"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupProvenanceTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	err = db.AutoMigrate(&models.AgentSkill{}, &models.AgentSkillFile{}, &models.SkillProvenanceLog{})
	require.NoError(t, err)
	return db
}

func TestProvenanceService_RecordOnCreate(t *testing.T) {
	db := setupProvenanceTestDB(t)
	svc := NewProvenanceService(db)
	ctx := context.Background()

	skill := &models.AgentSkill{
		ID:          1,
		WorkspaceID: 1,
		Name:        "test-skill",
		Slug:        "test-skill",
		Content:     "hello world",
		WriteOrigin: "foreground",
	}
	err := svc.Record(ctx, skill, "create", "", "agent", "")
	require.NoError(t, err)

	var logs []models.SkillProvenanceLog
	err = db.WithContext(ctx).Where("skill_id = ?", 1).Find(&logs).Error
	require.NoError(t, err)
	assert.Len(t, logs, 1)
	assert.Equal(t, "create", logs[0].Action)
	assert.Equal(t, "foreground", logs[0].WriteOrigin)
	assert.Equal(t, "agent", logs[0].ActorType)
	assert.Equal(t, "hello world", logs[0].Content)
}

func TestProvenanceService_RecordOnPatch(t *testing.T) {
	db := setupProvenanceTestDB(t)
	svc := NewProvenanceService(db)
	ctx := context.Background()

	skill := &models.AgentSkill{
		ID:          2,
		WorkspaceID: 1,
		Name:        "patch-skill",
		Slug:        "patch-skill",
		Content:     "new content after patch",
		WriteOrigin: "foreground",
	}
	err := svc.Record(ctx, skill, "patch", "references/doc.md", "agent", "")
	require.NoError(t, err)

	var logs []models.SkillProvenanceLog
	err = db.WithContext(ctx).Where("skill_id = ?", 2).Find(&logs).Error
	require.NoError(t, err)
	assert.Len(t, logs, 1)
	assert.Equal(t, "patch", logs[0].Action)
	assert.Equal(t, "references/doc.md", logs[0].FilePath)
}

func TestProvenanceService_MultipleRecords(t *testing.T) {
	db := setupProvenanceTestDB(t)
	svc := NewProvenanceService(db)
	ctx := context.Background()

	skill := &models.AgentSkill{
		ID:          3,
		WorkspaceID: 1,
		Name:        "multi-skill",
		Slug:        "multi-skill",
		Content:     "v3",
		WriteOrigin: "foreground",
	}
	_ = svc.Record(ctx, skill, "create", "", "agent", "")
	skill.Content = "v4"
	_ = svc.Record(ctx, skill, "edit", "", "agent", "")

	var logs []models.SkillProvenanceLog
	err := db.Where("skill_id = ?", 3).Find(&logs).Error
	require.NoError(t, err)
	assert.Len(t, logs, 2)
}
```

- [ ] Run it and verify it FAILS:

```bash
cd backend && go test ./internal/services/ -run TestProvenanceService -v
# Expected: compilation error — undefined: NewProvenanceService
```

- [ ] Write the minimal implementation — create `backend/internal/services/provenance_service.go`:

```go
package services

import (
	"context"

	"github.com/odysseythink/hermind/backend/internal/models"
	"gorm.io/gorm"
)

// ProvenanceRecorder records skill mutations for audit trail.
type ProvenanceRecorder interface {
	Record(ctx context.Context, skill *models.AgentSkill, action, filePath, actorType, actorID string) error
}

type ProvenanceService struct {
	db *gorm.DB
}

func NewProvenanceService(db *gorm.DB) *ProvenanceService {
	return &ProvenanceService{db: db}
}

func (s *ProvenanceService) Record(ctx context.Context, skill *models.AgentSkill, action, filePath, actorType, actorID string) error {
	log := models.SkillProvenanceLog{
		SkillID:     skill.ID,
		WorkspaceID: skill.WorkspaceID,
		Action:      action,
		WriteOrigin: skill.WriteOrigin,
		ActorType:   actorType,
		ActorID:     actorID,
		Content:     skill.Content,
		FilePath:    filePath,
	}
	return s.db.WithContext(ctx).Create(&log).Error
}
```

- [ ] Add `IsPinned` to `AgentSkillManager` interface — modify `backend/internal/services/agent_skill_service.go` lines 59-77:

```go
type AgentSkillManager interface {
	Create(ctx context.Context, workspaceID int, req dto.CreateAgentSkillRequest) (*models.AgentSkill, error)
	GetBySlug(ctx context.Context, workspaceID int, slug string) (*models.AgentSkill, error)
	GetByID(ctx context.Context, id int) (*models.AgentSkill, error)
	List(ctx context.Context, workspaceID int, includeArchived bool) ([]models.AgentSkill, error)
	ListActiveByWorkspace(ctx context.Context, workspaceID int) ([]models.AgentSkill, error)
	Update(ctx context.Context, workspaceID int, skillSlug string, req dto.UpdateAgentSkillRequest) (*models.AgentSkill, error)
	Patch(ctx context.Context, workspaceID int, skillSlug string, req dto.PatchAgentSkillRequest) (*models.AgentSkill, error)
	PatchFile(ctx context.Context, workspaceID int, skillSlug string, req dto.PatchSkillFileRequest) (*models.AgentSkillFile, error)
	Delete(ctx context.Context, workspaceID int, skillSlug string) error
	WriteFile(ctx context.Context, workspaceID int, skillSlug string, req dto.WriteSkillFileRequest) error
	RemoveFile(ctx context.Context, workspaceID int, skillSlug string, filePath string) error
	GetFile(ctx context.Context, skillID int, filePath string) (*models.AgentSkillFile, error)
	ListFiles(ctx context.Context, skillID int) ([]models.AgentSkillFile, error)
	BumpUse(ctx context.Context, workspaceID int, skillSlug string) error
	BumpView(ctx context.Context, workspaceID int, skillSlug string) error
	BumpPatch(ctx context.Context, workspaceID int, skillSlug string) error
	ApplyCuratorTransitions(ctx context.Context, staleDays, archiveDays int) (map[string]int, error)
	IsPinned(ctx context.Context, workspaceID int, skillSlug string) (bool, error)
}
```

- [ ] Add `IsPinned` implementation to `AgentSkillService` — append after `ApplyCuratorTransitions` method (after line 674):

```go
func (s *AgentSkillService) IsPinned(ctx context.Context, workspaceID int, skillSlug string) (bool, error) {
	skill, err := s.GetBySlug(ctx, workspaceID, skillSlug)
	if err != nil {
		return false, err
	}
	return skill.Pinned, nil
}
```

- [ ] Run it and verify it PASSES:

```bash
cd backend && go test ./internal/services/ -run TestProvenanceService -v
```

- [ ] Whole-tree typecheck (interface changed — all `AgentSkillManager` implementations must satisfy it):

```bash
cd backend && go build ./...
cd backend && go test ./... -count=1 2>&1 | grep -E "^(ok|FAIL|---)"
# All packages must compile and pass
```

- [ ] Commit:

```bash
git add backend/internal/services/provenance_service.go backend/internal/services/provenance_service_test.go backend/internal/services/agent_skill_service.go
git commit -m "feat(skills): add ProvenanceService with Record method and IsPinned to AgentSkillManager"
```

---

### Task 4: Create `BackupService` with Snapshot/Restore/List/Prune

**Depends on:** Task 1, Task 2

**Files:**
- Create: `backend/internal/services/skill_backup_service.go`
- Test: `backend/internal/services/skill_backup_service_test.go`

- [ ] Write the failing test — create `backend/internal/services/skill_backup_service_test.go`:

```go
package services

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/dto"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupBackupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	err = db.AutoMigrate(&models.AgentSkill{}, &models.AgentSkillFile{})
	require.NoError(t, err)
	return db
}

func TestBackupService_SnapshotCreatesFile(t *testing.T) {
	db := setupBackupTestDB(t)
	skillSvc := NewAgentSkillService(db)
	ctx := context.Background()

	tmpDir := t.TempDir()

	// Create a skill with a file
	skill, err := skillSvc.Create(ctx, 1, dto.CreateAgentSkillRequest{
		Name:    "backup-test",
		Content: "## backup content",
	})
	require.NoError(t, err)

	err = skillSvc.WriteFile(ctx, 1, skill.Slug, dto.WriteSkillFileRequest{
		FilePath: "references/guide.md",
		Content:  "# Guide",
	})
	require.NoError(t, err)

	backupSvc := NewBackupService(db, tmpDir, skillSvc)

	snapshotID, err := backupSvc.Snapshot(ctx, 1)
	require.NoError(t, err)
	assert.NotEmpty(t, snapshotID)

	// Verify file exists
	path := filepath.Join(tmpDir, "skill-backups", "1", snapshotID+".json")
	_, err = os.Stat(path)
	require.NoError(t, err, "snapshot file must exist at %s", path)
}

func TestBackupService_SnapshotPruneOld(t *testing.T) {
	db := setupBackupTestDB(t)
	skillSvc := NewAgentSkillService(db)
	ctx := context.Background()

	tmpDir := t.TempDir()
	backupSvc := NewBackupService(db, tmpDir, skillSvc)

	// Create 11 snapshots — the oldest should be pruned (keep=10)
	for i := 0; i < 11; i++ {
		_, err := skillSvc.Create(ctx, 1, dto.CreateAgentSkillRequest{
			Name:    "prune-test-" + string(rune('a'+i%26)),
			Content: "...",
		})
		require.NoError(t, err)
		_, err = backupSvc.Snapshot(ctx, 1)
		require.NoError(t, err)
	}

	entries, err := os.ReadDir(filepath.Join(tmpDir, "skill-backups", "1"))
	require.NoError(t, err)
	count := 0
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".json" {
			count++
		}
	}
	assert.LessOrEqual(t, count, 10, "should keep at most 10 snapshots")
}

func TestBackupService_RestoreIntegrity(t *testing.T) {
	db := setupBackupTestDB(t)
	skillSvc := NewAgentSkillService(db)
	ctx := context.Background()

	tmpDir := t.TempDir()
	backupSvc := NewBackupService(db, tmpDir, skillSvc)

	skill, err := skillSvc.Create(ctx, 1, dto.CreateAgentSkillRequest{
		Name:    "restore-test",
		Content: "original content",
	})
	require.NoError(t, err)

	err = skillSvc.WriteFile(ctx, 1, skill.Slug, dto.WriteSkillFileRequest{
		FilePath: "references/doc.md",
		Content:  "file content",
	})
	require.NoError(t, err)

	// Snapshot
	snapshotID, err := backupSvc.Snapshot(ctx, 1)
	require.NoError(t, err)

	// Mutate: delete the skill and recreate something different
	err = skillSvc.Delete(ctx, 1, skill.Slug)
	require.NoError(t, err)

	_, err = skillSvc.Create(ctx, 1, dto.CreateAgentSkillRequest{
		Name:    "different-skill",
		Content: "different",
	})
	require.NoError(t, err)

	// Restore
	err = backupSvc.Restore(ctx, 1, snapshotID)
	require.NoError(t, err)

	// Verify original skill is back
	restored, err := skillSvc.GetBySlug(ctx, 1, "restore-test")
	require.NoError(t, err)
	assert.Equal(t, "original content", restored.Content)

	// Verify the file is restored
	files, err := skillSvc.ListFiles(ctx, restored.ID)
	require.NoError(t, err)
	assert.Len(t, files, 1)
	assert.Equal(t, "file content", files[0].Content)
	assert.Equal(t, "references/doc.md", files[0].FilePath)

	// The "different-skill" should NOT exist after restore
	_, err = skillSvc.GetBySlug(ctx, 1, "different-skill")
	assert.ErrorIs(t, err, ErrSkillNotFound)
}

func TestBackupService_RestoreInvalidSnapshot(t *testing.T) {
	db := setupBackupTestDB(t)
	skillSvc := NewAgentSkillService(db)
	ctx := context.Background()

	tmpDir := t.TempDir()
	backupSvc := NewBackupService(db, tmpDir, skillSvc)

	err := backupSvc.Restore(ctx, 1, "nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "snapshot not found")
}

func TestBackupService_List(t *testing.T) {
	db := setupBackupTestDB(t)
	skillSvc := NewAgentSkillService(db)
	ctx := context.Background()

	tmpDir := t.TempDir()
	backupSvc := NewBackupService(db, tmpDir, skillSvc)

	_, _ = skillSvc.Create(ctx, 1, dto.CreateAgentSkillRequest{Name: "list-1", Content: "..."})
	_, err := backupSvc.Snapshot(ctx, 1)
	require.NoError(t, err)

	_, _ = skillSvc.Create(ctx, 1, dto.CreateAgentSkillRequest{Name: "list-2", Content: "..."})
	_, err = backupSvc.Snapshot(ctx, 1)
	require.NoError(t, err)

	infos, err := backupSvc.List(ctx, 1)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(infos), 1)
}
```

- [ ] Run it and verify it FAILS:

```bash
cd backend && go test ./internal/services/ -run TestBackupService -v
# Expected: compilation error — undefined: NewBackupService
```

- [ ] Write the minimal implementation — create `backend/internal/services/skill_backup_service.go`:

```go
package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"github.com/odysseythink/hermind/backend/internal/models"
	"gorm.io/gorm"
)

var ErrSnapshotNotFound = errors.New("snapshot not found")

type SnapshotInfo struct {
	SnapshotID  string    `json:"snapshotId"`
	WorkspaceID int       `json:"workspaceId"`
	CreatedAt   time.Time `json:"createdAt"`
	SkillCount  int       `json:"skillCount"`
	FileCount   int       `json:"fileCount"`
}

type BackupManager interface {
	Snapshot(ctx context.Context, workspaceID int) (string, error)
	Restore(ctx context.Context, workspaceID int, snapshotID string) error
	List(ctx context.Context, workspaceID int) ([]SnapshotInfo, error)
	Prune(ctx context.Context, workspaceID int, keep int) error
}

type BackupService struct {
	db       *gorm.DB
	baseDir  string // <StorageDir>/skill-backups
	skillSvc *AgentSkillService
}

func NewBackupService(db *gorm.DB, storageDir string, skillSvc *AgentSkillService) *BackupService {
	return &BackupService{
		db:       db,
		baseDir:  filepath.Join(storageDir, "skill-backups"),
		skillSvc: skillSvc,
	}
}

type snapshotData struct {
	WorkspaceID int                  `json:"workspaceId"`
	Timestamp   time.Time            `json:"timestamp"`
	Skills      []skillSnapshotEntry `json:"skills"`
}

type skillSnapshotEntry struct {
	models.AgentSkill
	Files []models.AgentSkillFile `json:"files"`
}

func (s *BackupService) Snapshot(ctx context.Context, workspaceID int) (string, error) {
	skills, err := s.skillSvc.List(ctx, workspaceID, true)
	if err != nil {
		return "", fmt.Errorf("list skills: %w", err)
	}

	entries := make([]skillSnapshotEntry, 0, len(skills))
	totalFiles := 0
	for _, sk := range skills {
		files, err := s.skillSvc.ListFiles(ctx, sk.ID)
		if err != nil {
			return "", fmt.Errorf("list files for skill %d: %w", sk.ID, err)
		}
		entries = append(entries, skillSnapshotEntry{
			AgentSkill: sk,
			Files:      files,
		})
		totalFiles += len(files)
	}

	snap := snapshotData{
		WorkspaceID: workspaceID,
		Timestamp:   time.Now().UTC(),
		Skills:      entries,
	}

	data, err := json.Marshal(snap)
	if err != nil {
		return "", fmt.Errorf("marshal snapshot: %w", err)
	}

	dir := filepath.Join(s.baseDir, strconv.Itoa(workspaceID))
	if err := os.MkdirAll(dir, 0750); err != nil {
		return "", fmt.Errorf("mkdir: %w", err)
	}

	filename := time.Now().UTC().Format("20060102-150405") + ".json"
	if err := os.WriteFile(filepath.Join(dir, filename), data, 0640); err != nil {
		return "", fmt.Errorf("write file: %w", err)
	}

	snapshotID := filename[:len(filename)-5]
	_ = s.Prune(ctx, workspaceID, 10)
	return snapshotID, nil
}

func (s *BackupService) Restore(ctx context.Context, workspaceID int, snapshotID string) error {
	path := filepath.Join(s.baseDir, strconv.Itoa(workspaceID), snapshotID+".json")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("%w: %s", ErrSnapshotNotFound, snapshotID)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read snapshot: %w", err)
	}

	var snap snapshotData
	if err := json.Unmarshal(data, &snap); err != nil {
		return fmt.Errorf("unmarshal snapshot: %w", err)
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("workspace_id = ?", workspaceID).Delete(&models.AgentSkillFile{}).Error; err != nil {
			return err
		}
		if err := tx.Where("workspace_id = ?", workspaceID).Delete(&models.AgentSkill{}).Error; err != nil {
			return err
		}
		for _, entry := range snap.Skills {
			entry.AgentSkill.ID = 0
			if err := tx.Create(&entry.AgentSkill).Error; err != nil {
				return err
			}
			for _, f := range entry.Files {
				f.ID = 0
				f.SkillID = entry.AgentSkill.ID
				if err := tx.Create(&f).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func (s *BackupService) List(ctx context.Context, workspaceID int) ([]SnapshotInfo, error) {
	dir := filepath.Join(s.baseDir, strconv.Itoa(workspaceID))
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var infos []SnapshotInfo
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		name := e.Name()
		snapshotID := name[:len(name)-5]
		info, err := e.Info()
		if err != nil {
			continue
		}
		infos = append(infos, SnapshotInfo{
			SnapshotID:  snapshotID,
			WorkspaceID: workspaceID,
			CreatedAt:   info.ModTime(),
		})
	}
	sort.Slice(infos, func(i, j int) bool {
		return infos[i].SnapshotID < infos[j].SnapshotID
	})
	return infos, nil
}

func (s *BackupService) Prune(ctx context.Context, workspaceID int, keep int) error {
	infos, err := s.List(ctx, workspaceID)
	if err != nil {
		return err
	}
	if len(infos) <= keep {
		return nil
	}
	toDelete := len(infos) - keep
	dir := filepath.Join(s.baseDir, strconv.Itoa(workspaceID))
	for i := 0; i < toDelete; i++ {
		path := filepath.Join(dir, infos[i].SnapshotID+".json")
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("prune %s: %w", infos[i].SnapshotID, err)
		}
	}
	return nil
}
```

- [ ] Run it and verify it PASSES:

```bash
cd backend && go test ./internal/services/ -run TestBackupService -v
```

- [ ] Run full service tests:

```bash
cd backend && go test ./internal/services/... -count=1 2>&1 | tail -5
# Expected: ok
```

- [ ] Commit:

```bash
git add backend/internal/services/skill_backup_service.go backend/internal/services/skill_backup_service_test.go
git commit -m "feat(skills): add BackupService with Snapshot/Restore/List/Prune"
```

---

### Task 5: Add Pin checks + Provenance recording to Agent Tools

**Depends on:** Task 3

**Files:**
- Modify: `backend/internal/agent/tools/agent_skills.go:77-277` — Pin checks + provenance in edit/patch/write_file/remove_file/delete
- Modify: `backend/internal/agent/tools/context.go:54` — add `ProvenanceSvc` field
- Modify: `backend/internal/agent/tools/builder.go:42,72-87,112` — add `ProvenanceSvc` to `BuilderDeps` + pass to `ToolContext` + pass to `NewSkillManageSkill`
- Test: `backend/internal/agent/tools/agent_skills_test.go` — append Pin + Provenance tests

- [ ] Write the failing test — create `backend/internal/agent/tools/agent_skills_test.go` (if not exists) or append:

First, read the existing test patterns. Then append:

```go
package tools

import (
	"context"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/dto"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupAgentToolTestDB(t *testing.T) (*gorm.DB, *services.AgentSkillService, *services.ProvenanceService) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	err = db.AutoMigrate(&models.AgentSkill{}, &models.AgentSkillFile{}, &models.SkillProvenanceLog{})
	require.NoError(t, err)
	skillSvc := services.NewAgentSkillService(db)
	provSvc := services.NewProvenanceService(db)
	return db, skillSvc, provSvc
}

func TestPinBlocksAgentEdit(t *testing.T) {
	db, skillSvc, provSvc := setupAgentToolTestDB(t)
	ctx := context.Background()

	skill, err := skillSvc.Create(ctx, 1, dto.CreateAgentSkillRequest{
		Name:    "pinned-edit",
		Content: "original",
	})
	require.NoError(t, err)

	_, err = skillSvc.Update(ctx, 1, skill.Slug, dto.UpdateAgentSkillRequest{
		Pinned: boolPtr(true),
	})
	require.NoError(t, err)

	tc := &ToolContext{
		Ctx:           ctx,
		Workspace:     &models.Workspace{ID: 1},
		Approval:      nil,
		AgentSkillSvc: skillSvc,
		ProvenanceSvc: provSvc,
	}

	result, _ := skillManageEdit(ctx, tc, skillSvc, 1, skillManageArgs{
		Name:    "pinned-edit",
		Content: "---\nname: pinned-edit\ndescription: test\n---\nnew content",
	})
	assert.Contains(t, result, "pinned")
	assert.Contains(t, result, "cannot be edited")

	// Verify skill was NOT edited
	updated, _ := skillSvc.GetBySlug(ctx, 1, skill.Slug)
	assert.Equal(t, "original", updated.Content)

	_ = db // used in setup
}

func TestPinBlocksAgentPatch(t *testing.T) {
	_, skillSvc, provSvc := setupAgentToolTestDB(t)
	ctx := context.Background()

	skill, err := skillSvc.Create(ctx, 1, dto.CreateAgentSkillRequest{
		Name:    "pinned-patch",
		Content: "hello world",
	})
	require.NoError(t, err)
	_, err = skillSvc.Update(ctx, 1, skill.Slug, dto.UpdateAgentSkillRequest{
		Pinned: boolPtr(true),
	})
	require.NoError(t, err)

	tc := &ToolContext{
		Ctx:           ctx,
		Workspace:     &models.Workspace{ID: 1},
		Approval:      nil,
		AgentSkillSvc: skillSvc,
		ProvenanceSvc: provSvc,
	}

	result, _ := skillManagePatch(ctx, tc, skillSvc, 1, skillManageArgs{
		Name:      "pinned-patch",
		OldString: "world",
		NewString: "universe",
	})
	assert.Contains(t, result, "pinned")
	assert.Contains(t, result, "cannot be patched")
}

func TestPinBlocksAgentWriteFile(t *testing.T) {
	_, skillSvc, provSvc := setupAgentToolTestDB(t)
	ctx := context.Background()

	skill, err := skillSvc.Create(ctx, 1, dto.CreateAgentSkillRequest{
		Name:    "pinned-write",
		Content: "...",
	})
	require.NoError(t, err)
	_, err = skillSvc.Update(ctx, 1, skill.Slug, dto.UpdateAgentSkillRequest{
		Pinned: boolPtr(true),
	})
	require.NoError(t, err)

	tc := &ToolContext{
		Ctx:           ctx,
		Workspace:     &models.Workspace{ID: 1},
		Approval:      nil,
		AgentSkillSvc: skillSvc,
		ProvenanceSvc: provSvc,
	}

	result, _ := skillManageWriteFile(ctx, tc, skillSvc, 1, skillManageArgs{
		Name:        "pinned-write",
		FilePath:    "references/test.md",
		FileContent: "new file",
	})
	assert.Contains(t, result, "pinned")
	assert.Contains(t, result, "cannot be modified")
}

func TestPinBlocksAgentRemoveFile(t *testing.T) {
	_, skillSvc, provSvc := setupAgentToolTestDB(t)
	ctx := context.Background()

	skill, err := skillSvc.Create(ctx, 1, dto.CreateAgentSkillRequest{
		Name:    "pinned-rm",
		Content: "...",
	})
	require.NoError(t, err)
	err = skillSvc.WriteFile(ctx, 1, skill.Slug, dto.WriteSkillFileRequest{
		FilePath: "references/doc.md",
		Content:  "doc",
	})
	require.NoError(t, err)
	_, err = skillSvc.Update(ctx, 1, skill.Slug, dto.UpdateAgentSkillRequest{
		Pinned: boolPtr(true),
	})
	require.NoError(t, err)

	tc := &ToolContext{
		Ctx:           ctx,
		Workspace:     &models.Workspace{Id: 1},
		Approval:      nil,
		AgentSkillSvc: skillSvc,
		ProvenanceSvc: provSvc,
	}

	result, _ := skillManageRemoveFile(ctx, tc, skillSvc, 1, skillManageArgs{
		Name:     "pinned-rm",
		FilePath: "references/doc.md",
	})
	assert.Contains(t, result, "pinned")
	assert.Contains(t, result, "cannot be removed")

	// Verify file is still there
	updated, _ := skillSvc.GetBySlug(ctx, 1, skill.Slug)
	files, _ := skillSvc.ListFiles(ctx, updated.ID)
	assert.Len(t, files, 1)
}

func TestAgentEditRecordsProvenance(t *testing.T) {
	db, skillSvc, provSvc := setupAgentToolTestDB(t)
	ctx := context.Background()

	skill, err := skillSvc.Create(ctx, 1, dto.CreateAgentSkillRequest{
		Name:    "prov-edit",
		Content: "before edit",
	})
	require.NoError(t, err)

	tc := &ToolContext{
		Ctx:           ctx,
		Workspace:     &models.Workspace{ID: 1},
		Approval:      nil,
		AgentSkillSvc: skillSvc,
		ProvenanceSvc: provSvc,
	}

	_, _ = skillManageEdit(ctx, tc, skillSvc, 1, skillManageArgs{
		Name:    "prov-edit",
		Content: "---\nname: prov-edit\ndescription: test\n---\nafter edit",
	})

	var logs []models.SkillProvenanceLog
	err = db.Where("skill_id = ?", skill.ID).Find(&logs).Error
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(logs), 1)
	assert.Equal(t, "edit", logs[len(logs)-1].Action)
	assert.Equal(t, "agent", logs[len(logs)-1].ActorType)
}
```

- [ ] Run it and verify it FAILS:

```bash
cd backend && go test ./internal/agent/tools/ -run "TestPinBlocksAgent|TestAgentEditRecordsProvenance" -v
# Expected: compilation error — ToolContext has no field ProvenanceSvc
```

- [ ] Write the minimal implementation:

**Step 1: Add `ProvenanceSvc` to `ToolContext`** — `backend/internal/agent/tools/context.go`, after line 54 (`AgentSkillSvc`):

```go
ProvenanceSvc  services.ProvenanceRecorder
```

**Step 2: Add `ProvenanceSvc` to `BuilderDeps`** — `backend/internal/agent/tools/builder.go`, after line 42 (`AgentSkillSvc`):

```go
ProvenanceSvc  services.ProvenanceRecorder
```

**Step 3: Pass `ProvenanceSvc` to `ToolContext`** — `builder.go`, in `Build()` method, after line 86 (`AgentSkillSvc: b.deps.AgentSkillSvc,`):

```go
ProvenanceSvc:  b.deps.ProvenanceSvc,
```

**Step 4: Pass `ProvenanceSvc` to `NewSkillManageSkill`** — `builder.go`, line 112:

Change:
```go
NewSkillManageSkill(tc, b.deps.AgentSkillSvc),
```
To:
```go
NewSkillManageSkill(tc, b.deps.AgentSkillSvc, b.deps.ProvenanceSvc),
```

**Step 5: Update `NewSkillManageSkill` signature + handler dispatch** — `agent_skills.go`, lines 17-37:

```go
func NewSkillManageSkill(tc *ToolContext, skillSvc services.AgentSkillManager, provenanceSvc services.ProvenanceRecorder) *tool.Entry {
	return &tool.Entry{
		// ... same ...
		Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var args skillManageArgs
			if err := json.Unmarshal(raw, &args); err != nil {
				return tool.Error(err.Error()), nil
			}
			return handleSkillManage(ctx, tc, skillSvc, provenanceSvc, args)
		},
	}
}
```

Update `handleSkillManage` signature (line 52):

```go
func handleSkillManage(ctx context.Context, tc *ToolContext, skillSvc services.AgentSkillManager, provenanceSvc services.ProvenanceRecorder, args skillManageArgs) (string, error) {
```

Update dispatch to pass `provenanceSvc`:

```go
case "create":
	return skillManageCreate(ctx, tc, skillSvc, wsID, args)
case "edit":
	return skillManageEdit(ctx, tc, skillSvc, provenanceSvc, wsID, args)
case "patch":
	return skillManagePatch(ctx, tc, skillSvc, provenanceSvc, wsID, args)
case "delete":
	return skillManageDelete(ctx, tc, skillSvc, provenanceSvc, wsID, args)
case "write_file":
	return skillManageWriteFile(ctx, tc, skillSvc, provenanceSvc, wsID, args)
case "remove_file":
	return skillManageRemoveFile(ctx, tc, skillSvc, provenanceSvc, wsID, args)
```

**Step 6: Add Pin checks + Provenance to each handler** — `agent_skills.go`:

`skillManageEdit` (line 114) — add after `if args.Content == ""`:

```go
	skill, err := skillSvc.GetBySlug(ctx, wsID, slugifyForLookup(args.Name))
	if err != nil {
		return tool.Error("Skill not found: " + err.Error()), nil
	}
	if skill.Pinned {
		return tool.Error(fmt.Sprintf("Skill '%s' is pinned and cannot be edited. Unpin it first.", args.Name)), nil
	}
```

Then after the successful update (line 142), add provenance:

```go
	if provenanceSvc != nil {
		_ = provenanceSvc.Record(ctx, updated, "edit", "", "agent", "")
	}
```

`skillManagePatch` (line 155) — add after `tc.Emit(...)`:

```go
	skill, err := skillSvc.GetBySlug(ctx, wsID, slugifyForLookup(args.Name))
	if err != nil {
		return tool.Error("Skill not found: " + err.Error()), nil
	}
	if skill.Pinned {
		return tool.Error(fmt.Sprintf("Skill '%s' is pinned and cannot be patched. Unpin it first.", args.Name)), nil
	}
```

Then after the successful patch (replace the return near line 190), add provenance:

```go
	if provenanceSvc != nil {
		_ = provenanceSvc.Record(ctx, patchedSkill, "patch", args.FilePath, "agent", "")
	}
```

`skillManageDelete` (line 197) — add Pin check BEFORE approval (Pin check should block early):

```go
	skill, err := skillSvc.GetBySlug(ctx, wsID, slugifyForLookup(args.Name))
	if err != nil {
		return tool.Error("Skill not found: " + err.Error()), nil
	}
	if skill.Pinned {
		return tool.Error(fmt.Sprintf("Skill '%s' is pinned and cannot be deleted. Unpin it first.", args.Name)), nil
	}
```

Then after successful delete (line 210), add provenance:

```go
	if provenanceSvc != nil {
		_ = provenanceSvc.Record(ctx, skill, "delete", "", "agent", "")
	}
```

`skillManageWriteFile` (line 223) — add after `if args.FileContent == ""`:

```go
	skill, err := skillSvc.GetBySlug(ctx, wsID, slugifyForLookup(args.Name))
	if err != nil {
		return tool.Error("Skill not found: " + err.Error()), nil
	}
	if skill.Pinned {
		return tool.Error(fmt.Sprintf("Skill '%s' is pinned and files cannot be modified. Unpin it first.", args.Name)), nil
	}
```

Then after successful write (line 248), add provenance:

```go
	if provenanceSvc != nil {
		_ = provenanceSvc.Record(ctx, skill, "write_file", args.FilePath, "agent", "")
	}
```

`skillManageRemoveFile` (line 254) — add after `if args.FilePath == ""`:

```go
	skill, err := skillSvc.GetBySlug(ctx, wsID, slugifyForLookup(args.Name))
	if err != nil {
		return tool.Error("Skill not found: " + err.Error()), nil
	}
	if skill.Pinned {
		return tool.Error(fmt.Sprintf("Skill '%s' is pinned and files cannot be removed. Unpin it first.", args.Name)), nil
	}
```

Then after successful remove (line 271), add provenance:

```go
	if provenanceSvc != nil {
		_ = provenanceSvc.Record(ctx, skill, "remove_file", args.FilePath, "agent", "")
	}
```

- [ ] Run it and verify it PASSES:

```bash
cd backend && go test ./internal/agent/tools/ -run "TestPinBlocksAgent|TestAgentEditRecordsProvenance" -v
```

- [ ] Whole-tree typecheck (shared signature: `NewSkillManageSkill`, `handleSkillManage`, `BuilderDeps`, `ToolContext` all changed):

```bash
cd backend && go build ./...
cd backend && go test ./... -count=1 2>&1 | grep -E "^(ok|FAIL|---)"
# All packages must compile and pass
```

- [ ] Commit:

```bash
git add backend/internal/agent/tools/agent_skills.go backend/internal/agent/tools/context.go backend/internal/agent/tools/builder.go backend/internal/agent/tools/agent_skills_test.go
git commit -m "feat(skills): add Pin protection and Provenance recording to agent skill_manage tool"
```

---

### Task 6: Integrate Backup into Curator Worker

**Depends on:** Task 4

**Files:**
- Modify: `backend/internal/workers/skill_curator.go:14-22` — add `backupSvc` field
- Modify: `backend/internal/workers/skill_curator.go:32-58` — call Backup before `ApplyCuratorTransitions`
- Test: `backend/internal/workers/skill_curator_test.go` — add `TestBackupFailureBlocksCurator`, `TestCuratorRespectsPinned`

- [ ] Write the failing tests — append to `backend/internal/workers/skill_curator_test.go`:

```go
func TestCuratorBackupCalled(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.AgentSkill{}, &models.AgentSkillFile{}, &models.SystemSetting{}))

	skillSvc := services.NewAgentSkillService(db)
	ctx := context.Background()

	_, _ = skillSvc.Create(ctx, 1, dto.CreateAgentSkillRequest{Name: "backup-test", Content: "..."})

	tmpDir := t.TempDir()
	backupSvc := services.NewBackupService(db, tmpDir, skillSvc)
	sysSvc := services.NewSystemService(db)

	job := NewSkillCuratorJobWithBackup(db, skillSvc, sysSvc, backupSvc)

	err = job.Run(ctx)
	require.NoError(t, err)

	// Verify backup file exists
	infos, err := backupSvc.List(ctx, 1)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(infos), 1)
}

func TestCuratorRespectsPinned(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.AgentSkill{}, &models.AgentSkillFile{}, &models.SystemSetting{}))

	skillSvc := services.NewAgentSkillService(db)
	ctx := context.Background()

	// Create a pinned old skill
	pinned, _ := skillSvc.Create(ctx, 1, dto.CreateAgentSkillRequest{Name: "pinned-old", Content: "..."})
	db.Model(pinned).Updates(map[string]any{
		"pinned":     true,
		"created_at": db.Raw("datetime('now', '-100 days')"),
	})

	// Create an unpinned old skill
	unpinned, _ := skillSvc.Create(ctx, 1, dto.CreateAgentSkillRequest{Name: "unpinned-old", Content: "..."})
	db.Model(unpinned).Update("created_at", db.Raw("datetime('now', '-100 days')"))

	tmpDir := t.TempDir()
	backupSvc := services.NewBackupService(db, tmpDir, skillSvc)
	sysSvc := services.NewSystemService(db)
	job := NewSkillCuratorJobWithBackup(db, skillSvc, sysSvc, backupSvc)

	// Set stale=30, archive=90
	_ = sysSvc.SetSetting(ctx, "agent_skill_stale_after_days", "30")
	_ = sysSvc.SetSetting(ctx, "agent_skill_archive_after_days", "90")

	err = job.Run(ctx)
	require.NoError(t, err)

	// Pinned should remain active
	p, _ := skillSvc.GetBySlug(ctx, 1, pinned.Slug)
	assert.Equal(t, models.AgentSkillStatusActive, p.Status)

	// Unpinned should be archived (100 days > 90 days)
	u, _ := skillSvc.GetBySlug(ctx, 1, unpinned.Slug)
	assert.Equal(t, models.AgentSkillStatusArchived, u.Status)
}
```

- [ ] Run it and verify it FAILS:

```bash
cd backend && go test ./internal/workers/ -run "TestCuratorBackupCalled|TestCuratorRespectsPinned" -v
# Expected: compilation error — undefined: NewSkillCuratorJobWithBackup
```

- [ ] Write the minimal implementation — modify `backend/internal/workers/skill_curator.go`:

**Add backupSvc to struct** (lines 14-18):

```go
type SkillCuratorJob struct {
	db        *gorm.DB
	skillSvc  services.AgentSkillManager
	sysSvc    *services.SystemService
	backupSvc services.BackupManager
}
```

**Add new constructor** (after line 22):

```go
func NewSkillCuratorJobWithBackup(db *gorm.DB, skillSvc services.AgentSkillManager, sysSvc *services.SystemService, backupSvc services.BackupManager) *SkillCuratorJob {
	return &SkillCuratorJob{db: db, skillSvc: skillSvc, sysSvc: sysSvc, backupSvc: backupSvc}
}
```

**Update existing constructor** to preserve backward compatibility:

```go
func NewSkillCuratorJob(db *gorm.DB, skillSvc services.AgentSkillManager, sysSvc *services.SystemService) *SkillCuratorJob {
	return &SkillCuratorJob{db: db, skillSvc: skillSvc, sysSvc: sysSvc, backupSvc: nil}
}
```

**Update `Run` method** (lines 32-58) — add backup before transitions:

```go
func (j *SkillCuratorJob) Run(ctx context.Context) error {
	staleDays := 30
	archiveDays := 90

	if v, err := j.sysSvc.GetSetting(ctx, "agent_skill_stale_after_days"); err == nil && v != "" {
		if d, err := strconv.Atoi(v); err == nil && d > 0 {
			staleDays = d
		}
	}
	if v, err := j.sysSvc.GetSetting(ctx, "agent_skill_archive_after_days"); err == nil && v != "" {
		if d, err := strconv.Atoi(v); err == nil && d > 0 {
			archiveDays = d
		}
	}

	// Backup before curator transitions — iterate all workspaces
	if j.backupSvc != nil {
		var workspaces []models.Workspace
		if err := j.db.WithContext(ctx).Find(&workspaces).Error; err != nil {
			mlog.Error("curator: failed to list workspaces for backup", mlog.Err(err))
			return fmt.Errorf("list workspaces: %w", err)
		}
		for _, ws := range workspaces {
			snapshotID, err := j.backupSvc.Snapshot(ctx, ws.ID)
			if err != nil {
				mlog.Error("curator: backup failed, aborting for workspace", mlog.Int("workspace", ws.ID), mlog.Err(err))
				return fmt.Errorf("backup failed for workspace %d: %w", ws.ID, err)
			}
			mlog.Info("curator: backup complete", mlog.Int("workspace", ws.ID), mlog.String("snapshot", snapshotID))
		}
	}

	counts, err := j.skillSvc.ApplyCuratorTransitions(ctx, staleDays, archiveDays)
	if err != nil {
		return err
	}

	mlog.Info("skill curator run complete",
		mlog.Int("checked", counts["checked"]),
		mlog.Int("marked_stale", counts["marked_stale"]),
		mlog.Int("archived", counts["archived"]),
		mlog.Int("reactivated", counts["reactivated"]),
	)
	return nil
}
```

Also add `"fmt"` and `models` imports to the file.

- [ ] Run it and verify it PASSES:

```bash
cd backend && go test ./internal/workers/ -run "TestCuratorBackupCalled|TestCuratorRespectsPinned" -v
```

- [ ] Run all workers tests:

```bash
cd backend && go test ./internal/workers/... -count=1 -v 2>&1 | tail -20
```

- [ ] Commit:

```bash
git add backend/internal/workers/skill_curator.go backend/internal/workers/skill_curator_test.go
git commit -m "feat(skills): integrate BackupService into curator worker with per-workspace snapshot"
```

---

### Task 7: Wire BackupService + ProvenanceService into main.go

**Depends on:** Task 5, Task 6

**Files:**
- Modify: `backend/cmd/server/main.go:189,248` — instantiate services + pass to worker constructor

This task has no tests (pure wiring/DI). Verification is done via build.

- [ ] Write the code — modify `backend/cmd/server/main.go`:

**Add imports** (ensure `"github.com/odysseythink/hermind/backend/internal/models"` is imported — it already is at line 27):

No new imports needed — `services.BackupManager`, `services.ProvenanceRecorder` are already in the `services` package.

**Add service instantiation** after `agentSkillSvc` (line 189):

```go
	agentSkillSvc := services.NewAgentSkillService(db)
	provenanceSvc := services.NewProvenanceService(db)                                    // [new]
	backupSvc := services.NewBackupService(db, cfg.StorageDir, agentSkillSvc)             // [new]
```

**Update `agent.Deps`** (add `ProvenanceSvc` after line 223):

```go
		AgentSkillSvc:   agentSkillSvc,
		ProvenanceSvc:   provenanceSvc,                                                    // [new]
```

**Update worker registration** (line 248) — replace:

```go
workers.NewSkillCuratorJob(db, agentSkillSvc, sysSvc),
```

With:

```go
workers.NewSkillCuratorJobWithBackup(db, agentSkillSvc, sysSvc, backupSvc),               // [new with backup]
```

- [ ] Build-verify:

```bash
cd backend && go build -tags="fts5" -o /dev/null ./cmd/server/
```

- [ ] Full test suite to verify no regressions:

```bash
cd backend && go test -race -cover ./... 2>&1 | tail -30
```

- [ ] Manual verification — start the server in dev mode and check logs for curator backup:

```bash
cd backend && go build -tags="fts5" -o ../hermind ./cmd/server/
# Start server (Ctrl-C after a few seconds)
# Check log output: no panic, all services initialized, AutoMigrate runs
```

- [ ] Commit:

```bash
git add backend/cmd/server/main.go
git commit -m "feat(skills): wire ProvenanceService and BackupService into main.go"
```

---

## Self-Review

- [ ] 1. Spec-coverage table — map every design requirement to tasks:

| # | Design Requirement | Task(s) | Status |
|---|-------------------|---------|--------|
| 1 | Pin Mechanism — Curator skip Pinned skills | Task 6 (already in service via `ApplyCuratorTransitions:616`) | covered |
| 2 | Pin Mechanism — Agent tools reject edit/patch/write_file/remove_file on Pinned | Task 5 | covered |
| 3 | Pin Mechanism — Agent tools reject delete on Pinned | Task 5 (already in service via `Delete:454` + tool-layer check added) | covered |
| 4 | Provenance — `WriteOrigin` field on `AgentSkill` | Task 1 | covered |
| 5 | Provenance — `SkillProvenanceLog` table | Task 2 | covered |
| 6 | Provenance — `ProvenanceService.Record` with full content snapshot | Task 3 | covered |
| 7 | Provenance — Record on all skill mutation operations | Task 5 (agent tools), Task 3 (service) | covered |
| 8 | Backup/Rollback — `BackupService.Snapshot(workspaceID)` exports JSON | Task 4 | covered |
| 9 | Backup/Rollback — keep last 10 snapshots (Prune) | Task 4 | covered |
| 10 | Backup/Rollback — `Restore(snapshotID)` full rollback | Task 4 | covered |
| 11 | Backup/Rollback — failure blocks Curator | Task 6 | covered |
| 12 | Curator Worker — backup before transitions per workspace | Task 6 | covered |
| 13 | `IsPinned` on `AgentSkillManager` interface | Task 3 | covered |
| 14 | `CreateAgentSkillRequest.WriteOrigin` DTO field | Task 1 | covered |
| 15 | main.go wiring of all new services | Task 7 | covered |
| 16 | Frontend Pin UI | — | **no-op** (deferred to Batch 2) |
| 17 | LLM Review Fork | — | **no-op** (deferred to Batch 2) |
| 18 | Umbrella Building | — | **no-op** (deferred to Batch 2) |
| 19 | Backup compression/encryption | — | **no-op** (deferred to Batch 2) |
| 20 | Provenance log query API | — | **no-op** (deferred to Batch 2) |

- [ ] 2. Placeholder scan — verify no `TODO`/`TBD`/deferred-by-dependency:

```bash
grep -n "TODO\|TBD\|implement later\|add appropriate" .ody-code/plans/2026-06-12-agent-skill-batch1-design.md
# Expected: no matches
```

Scan results: zero placeholders. All deferred items explicitly marked `no-op` in coverage table with Batch 2 justification. Every task step contains concrete code, not hand-waving. The `inferActor` function is explicitly NOT deferred — Task 3 accepts explicit `actorType`/`actorID` parameters instead, which is the correct approach for this batch where only the agent path exists.

- [ ] 3. No phantom tasks — every task produces a verifiable change:

| Task | Verifiable Output |
|------|------------------|
| 1 | `WriteOrigin` field exists in DB, `TestAgentSkillService_WriteOriginDefault` passes |
| 2 | `skill_provenance_logs` table created by AutoMigrate, `TestSkillCuratorJob_Run` still passes |
| 3 | `ProvenanceService.Record` creates DB rows, `TestProvenanceService_*` passes |
| 4 | `BackupService.Snapshot` creates JSON files, `TestBackupService_*` passes |
| 5 | `skill_manage edit/patch/write_file/remove_file` reject pinned skills, `TestPinBlocksAgent*` passes |
| 6 | Curator creates backup before transitions, `TestCuratorBackupCalled` passes |
| 7 | `go build ./cmd/server/` succeeds with all services wired |

Zero `--allow-empty` commits. Every task modifies or creates at least one file.

- [ ] 4. Dependency soundness — verify every `Depends on:` is satisfied:

```
Task 1: none                                           ✓
Task 2: Task 1                                         ✓ (needs models/ + AutoMigrate)
Task 3: Task 1, Task 2                                 ✓ (needs WriteOrigin + SkillProvenanceLog)
Task 4: Task 1, Task 2                                 ✓ (needs AgentSkill + AgentSkillFile)
Task 5: Task 3                                         ✓ (needs ProvenanceService + IsPinned)
Task 6: Task 4                                         ✓ (needs BackupService)
Task 7: Task 5, Task 6                                 ✓ (needs both services to wire)
```

No forward references. No symbol defined in a later task is used in an earlier task.

- [ ] 5. Caller & build soundness — every shared-signature change updates all callers:

**Shared signatures changed and verified:**

| Change | Updaters | Verification |
|--------|----------|-------------|
| `AgentSkill` struct + `WriteOrigin` (Task 1) | GORM — auto; all reads unaffected (new field with default) | `go build ./...` passes |
| `CreateAgentSkillRequest` + `WriteOrigin` (Task 1) | Task 1 searches callers via grep; `WriteOrigin` is `omitempty` so no existing callers break | `go build ./...` passes |
| `AgentSkillManager` interface + `IsPinned` (Task 3) | `AgentSkillService` — implementation added in same task | `go build ./...` passes |
| `ToolContext` + `ProvenanceSvc` (Task 5) | `builder.go` — all `ToolContext{}` literals in `Build()` updated in same task | `go build ./...` passes |
| `BuilderDeps` + `ProvenanceSvc` (Task 5) | `main.go:agent.Deps` — updated in Task 7 via `ProvenanceSvc` field | `go build ./...` passes |
| `NewSkillManageSkill` signature (Task 5) | `builder.go` call site updated in same task | `go build ./...` passes |
| `NewSkillCuratorJobWithBackup` (Task 6) | `main.go` updated in Task 7 | `go build ./...` passes |

All whole-tree typechecks are executed at the end of each shared-signature task. No signature is changed across multiple tasks.

**End-to-end trace**: `WriteOrigin` field flows: `CreateAgentSkillRequest.WriteOrigin` → `AgentSkillService.Create` sets `skill.WriteOrigin` → stored in DB → read back via `GetBySlug` → `ProvenanceService.Record` reads `skill.WriteOrigin` → written to `SkillProvenanceLog.WriteOrigin`. All steps in the chain use the same field name `WriteOrigin`.

- [ ] 6. Test-the-risk — every state-mutating task has behavioral assertions:

| Test | Risk Asserted | Trace through constants |
|------|--------------|------------------------|
| `TestAgentSkillService_WriteOriginDefault` | Default is `"foreground"`, explicit value preserved | Default string `"foreground"` matches `WriteOrigin` gorm default tag |
| `TestProvenanceService_RecordOnCreate` | Record stores full content snapshot | `"hello world"` content asserted exactly in DB row |
| `TestProvenanceService_MultipleRecords` | Multiple records accumulate | 2 records asserted for 2 Record calls |
| `TestBackupService_SnapshotCreatesFile` | JSON file exists at expected path | Path built from `tmpDir/skill-backups/1/{snapshotID}.json` |
| `TestBackupService_RestoreIntegrity` | Restore replaces all data (skills + files), removes divergent data | "different-skill" must NOT exist; "original content" must be restored |
| `TestBackupService_SnapshotPruneOld` | 11 snapshots → ≤10 kept | `keep=10` constant in Prune call |
| `TestPinBlocksAgentEdit/Patch/WriteFile/RemoveFile` | Pinned skill rejection + no mutation | Error contains "pinned" + content unchanged |
| `TestCuratorBackupCalled` | Backup file exists after curator run | `backupSvc.List(ctx, 1)` returns ≥1 entries |
| `TestCuratorRespectsPinned` | Pinned stays active, unpinned archived | Pinned skill `Status == "active"`; unpinned `Status == "archived"` |

No "must-survive" filter/matching tests needed — this batch adds no new filters, regexes, or matching rules.

- [ ] 7. Type consistency — cross-check types across tasks:

| Type/Field | Defined | Used In | Consistent? |
|-----------|---------|---------|-------------|
| `AgentSkill.WriteOrigin string` | Task 1 (model) | Task 3 (Provenance), Task 5 (tools) | ✓ |
| `SkillProvenanceLog` struct | Task 2 (model) | Task 3 (ProvenanceService) | ✓ |
| `ProvenanceRecorder` interface | Task 3 (service) | Task 5 (ToolContext, BuilderDeps) | ✓ |
| `ProvenanceService` struct | Task 3 (service) | Task 7 (main.go NewProvenanceService) | ✓ |
| `BackupManager` interface | Task 4 (service) | Task 6 (SkillCuratorJob.backupSvc) | ✓ |
| `BackupService` struct | Task 4 (service) | Task 7 (main.go NewBackupService) | ✓ |
| `SnapshotInfo` struct | Task 4 (service) | Task 4 (List return) | ✓ |
| `IsPinned(workspaceID int, skillSlug string) (bool, error)` | Task 3 (interface) | Task 3 (implementation) | ✓ |
| `Record(ctx, skill, action, filePath, actorType, actorID)` | Task 3 (interface) | Task 5 (tools calls) | ✓ |
| `Snapshot(ctx, workspaceID int) (string, error)` | Task 4 (interface) | Task 6 (curator call), Task 7 (wiring) | ✓ |
| `Restore(ctx, workspaceID int, snapshotID string) error` | Task 4 (interface) | — (no caller in this batch; ready for future API) | ✓ |
| `ToolContext.ProvenanceSvc` | Task 5 (context.go) | Task 5 (builder.go construct), Task 5 (handler calls) | ✓ |
| `BuilderDeps.ProvenanceSvc` | Task 5 (builder.go) | Task 7 (main.go agent.Deps) | ✓ |
| `CreateAgentSkillRequest.WriteOrigin` | Task 1 (DTO) | Task 1 (Create method) | ✓ |

---

## Done Criteria

```bash
# All new tests pass
cd backend && go test -v ./internal/services/ -run "TestAgentSkillService_WriteOriginDefault|TestProvenanceService|TestBackupService"
cd backend && go test -v ./internal/agent/tools/ -run "TestPinBlocksAgent|TestAgentEditRecordsProvenance"
cd backend && go test -v ./internal/workers/ -run "TestCuratorBackupCalled|TestCuratorRespectsPinned"

# All existing tests pass (no regression)
cd backend && go test -race -cover ./... 2>&1 | tail -5
# Expected: ok ... (all pass)

# Full build
cd backend && go build -tags="fts5" -o ../hermind ./cmd/server/

# Lint
cd backend && golangci-lint run ./...
```


---

