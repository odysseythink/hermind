# Agent Skill System Phase 3: Curator LLM Review + UsageSidecar + Audit Logging

> **For agentic workers:** REQUIRED SUB-SKILL: Use gpowers:subagent-driven-development (recommended) or gpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 构建智能 Curator 服务，集成 LLM 审查能力、UsageSidecar 持久化、审计日志、报告生成和 REST API，完成技能系统增强的最后阶段。

**Architecture:** `CuratorService` 作为核心编排器，Phase A 执行时间驱动转换（active→stale→archived），Phase B 调用 Pantheon `LLMProvider.Complete()` 进行 LLM 审查，三层信号归并后更新技能状态，所有操作写入 `CuratorAuditLog` 和文件系统报告。`SkillCuratorJob` 改为遍历 workspace 调用 `CuratorService.Run()`。

**Tech Stack:** Go 1.26, Gin, GORM, Pantheon SDK (`github.com/odysseythink/pantheon/core`), `gopkg.in/yaml.v3`

---

## File Structure

### 新建文件

| 文件 | 责任 |
|------|------|
| `backend/internal/models/curator_audit_log.go` | `CuratorAuditLog` GORM 模型 + `CuratorAuditAction` 常量 |
| `backend/internal/services/curator_service.go` | `CuratorService` 主入口、`CuratorRunResult`、时间驱动转换 |
| `backend/internal/services/curator_llm.go` | LLM 审查 Prompt 构建器、YAML 响应解析器、LLM 调用封装 |
| `backend/internal/services/curator_report.go` | 报告生成：`run.json`（机器可读）+ `REPORT.md`（人类可读） |
| `backend/internal/services/curator_audit.go` | 审计日志写入：DB `CuratorAuditLog` + `mlog` 结构化日志 |

### 修改文件

| 文件 | 责任 |
|------|------|
| `backend/internal/models/agent_skill.go` | 添加 `ParseUsageSidecar()` 和 `SaveUsageSidecar()` 方法 |
| `backend/internal/services/agent_skill_service.go` | 添加 `GetBySlug` 扩展（如需要）、`List` 支持 includeArchived 确认 |
| `backend/internal/workers/skill_curator.go` | 重构：注入 `CuratorService`，遍历 workspace 调用 `Run()` |
| `backend/internal/handlers/agent_skills.go` | 添加 `GET /workspace/:slug/agent-skills/curator-report` 端点 |
| `backend/cmd/server/main.go` | 构造 `CuratorService`，注入 `LLMProvider` 和配置 |

---

## Dependency Overview

```
Task 17 (UsageSidecar) ──┐
Task 18 (AuditLog model) ─┤
                          ├──► Task 19 (Prompt builder)
                          │      └──► Task 20 (Response parser)
                          │             └──► Task 21 (Three-signal reconciliation)
                          │                    └──► Task 22 (Report generation)
                          │                           ├──► Task 23 (Worker integration)
                          │                           └──► Task 24 (REST API)
                          └──► Task 16 (CuratorService scaffold)
                                   └──► Task 23 (Worker integration)

Task 25 (Integration tests) ──► depends on all above
```

**可并行任务：**
- Task 17 和 Task 18 完全独立，可并行。
- Task 16 依赖 Task 17 完成（`applyTimeDrivenTransitions` 需要 `ParseUsageSidecar` 中的 `AgentCreated` 标记）。

---

## Risks & Open Questions

| 风险 | 影响 | 缓解 |
|------|------|------|
| Pantheon `LLMProvider.Complete()` 在大量技能文本上超时 | Curator 运行失败 | Prompt 截断：每个技能内容最多 500 字符；设置 60s 上下文超时 |
| LLM 输出 YAML 格式不合法 | 解析失败，无法归并 | YAML 解析失败时记录错误并降级为仅时间驱动；添加 `gopkg.in/yaml.v3` 严格解析 |
| `CuratorService` 需要 workspace 级别的 LLM 配置（模型、温度等） | 当前 `LLMProvider` 是全局单例 | V1 使用全局 `LLMProvider`；未来可通过 `SystemSetting` 添加 curator 专用模型配置 |
| Worker 遍历所有 workspace 时，某一 workspace 的 Curator 失败不应阻断其他 workspace | 部分 workspace 未处理 | `SkillCuratorJob.Run` 对每个 workspace 的错误单独捕获并记录，不提前返回 |

---

## Phase 3 Tasks

### Task 16: CuratorService scaffolding + time-driven transitions

**Depends on:** Task 17

**Files:**
- Create: `backend/internal/services/curator_service.go`
- Modify: `backend/internal/services/agent_skill_service.go:58-77` (interface) + `~600` (implementation)
- Test: `backend/internal/services/curator_service_test.go`

- [ ] **Step 1: Extend `AgentSkillManager` interface with `UpdateStatus`**

在 `backend/internal/services/agent_skill_service.go` 的 `AgentSkillManager` 接口中添加：

```go
UpdateStatus(ctx context.Context, skillID int, status string) error
```

同时在 `AgentSkillService` 中实现：

```go
func (s *AgentSkillService) UpdateStatus(ctx context.Context, skillID int, status string) error {
    return s.db.WithContext(ctx).Model(&models.AgentSkill{}).Where("id = ?", skillID).
        Updates(map[string]any{"status": status, "updated_at": time.Now()}).Error
}
```

- [ ] **Step 2: Write the failing test for `CuratorService.applyTimeDrivenTransitions`**

```go
// backend/internal/services/curator_service_test.go
package services

import (
    "context"
    "testing"
    "time"

    "github.com/odysseythink/hermind/backend/internal/models"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "gorm.io/driver/sqlite"
    "gorm.io/gorm"
)

func TestCuratorService_applyTimeDrivenTransitions(t *testing.T) {
    db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
    require.NoError(t, err)
    require.NoError(t, db.AutoMigrate(&models.AgentSkill{}, &models.Workspace{}))

    ctx := context.Background()
    ws := &models.Workspace{Name: "test", Slug: "test"}
    require.NoError(t, db.Create(ws).Error)

    now := time.Now()
    // active skill, 100 days old, not pinned, agent-created
    skill := &models.AgentSkill{
        WorkspaceID: ws.ID,
        Name:        "old-skill",
        Slug:        "old-skill",
        Status:      models.AgentSkillStatusActive,
        CreatedBy:   "agent",
        CreatedAt:   now.AddDate(0, 0, -100),
    }
    require.NoError(t, db.Create(skill).Error)

    skillSvc := NewAgentSkillService(db, nil, "")
    curator := NewCuratorService(skillSvc, nil, nil, "")
    result := &CuratorRunResult{Timestamp: now}

    err = curator.applyTimeDrivenTransitions(ctx, ws.ID, result)
    require.NoError(t, err)
    assert.Equal(t, 1, result.Checked)
    assert.Equal(t, 1, result.Archived)

    var updated models.AgentSkill
    require.NoError(t, db.First(&updated, skill.ID).Error)
    assert.Equal(t, models.AgentSkillStatusArchived, updated.Status)
}
```

Run: `cd backend && go test ./internal/services/ -run TestCuratorService_applyTimeDrivenTransitions -v`
Expected: FAIL — `CuratorService`, `CuratorRunResult`, `NewCuratorService` 未定义

- [ ] **Step 3: Implement `CuratorService` scaffold and time-driven logic**

```go
// backend/internal/services/curator_service.go
package services

import (
    "context"
    "fmt"
    "time"

    "github.com/odysseythink/hermind/backend/internal/config"
    "github.com/odysseythink/hermind/backend/internal/models"
    "github.com/odysseythink/hermind/backend/internal/providers"
    "github.com/odysseythink/mlog"
)

// CuratorRunResult aggregates the outcome of a single Curator run.
type CuratorRunResult struct {
    Timestamp      time.Time       `json:"timestamp"`
    Checked        int             `json:"checked"`
    MarkedStale    int             `json:"marked_stale"`
    Archived       int             `json:"archived"`
    Reactivated    int             `json:"reactivated"`
    Merged         int             `json:"merged"`
    Errors         []string        `json:"errors,omitempty"`
    Consolidations []Consolidation `json:"consolidations,omitempty"`
    Prunings       []Pruning       `json:"prunings,omitempty"`
    Absorbed       []Absorbed      `json:"absorbed,omitempty"`
}

// CuratorService orchestrates skill lifecycle review.
type CuratorService struct {
    skillSvc   AgentSkillManager
    llmProv    providers.LLMProvider
    cfg        *config.Config
    storageDir string
    audit      AuditWriter
}

// NewCuratorService creates a new CuratorService.
func NewCuratorService(skillSvc AgentSkillManager, llmProv providers.LLMProvider, cfg *config.Config, storageDir string) *CuratorService {
    return &CuratorService{
        skillSvc:   skillSvc,
        llmProv:    llmProv,
        cfg:        cfg,
        storageDir: storageDir,
        audit:      &noopAuditWriter{},
    }
}

// SetAuditWriter replaces the default no-op audit writer.
func (c *CuratorService) SetAuditWriter(a AuditWriter) {
    c.audit = a
}

// Run executes the full curator pipeline for a workspace.
func (c *CuratorService) Run(ctx context.Context, wsID int) (*CuratorRunResult, error) {
    result := &CuratorRunResult{Timestamp: time.Now()}

    // Phase A: time-driven transitions
    if err := c.applyTimeDrivenTransitions(ctx, wsID, result); err != nil {
        mlog.Warn("curator time-driven phase failed", mlog.Err(err), mlog.Int("workspace", wsID))
        result.Errors = append(result.Errors, fmt.Sprintf("time-driven: %v", err))
    }

    // Phase B: LLM review (wired in Task 21)
    // if c.llmProv != nil && c.cfg != nil && c.isLLMReviewEnabled() { ... }

    return result, nil
}

func (c *CuratorService) applyTimeDrivenTransitions(ctx context.Context, wsID int, result *CuratorRunResult) error {
    skills, err := c.skillSvc.List(ctx, wsID, true)
    if err != nil {
        return err
    }

    now := time.Now()
    staleCutoff := now.AddDate(0, 0, -30)
    archiveCutoff := now.AddDate(0, 0, -90)

    for _, skill := range skills {
        if skill.Pinned {
            continue
        }

        sidecar, _ := skill.ParseUsageSidecar()
        if sidecar != nil && !sidecar.AgentCreated {
            // Also check legacy CreatedBy field
            if skill.CreatedBy != "agent" {
                continue
            }
        } else if sidecar == nil && skill.CreatedBy != "agent" {
            continue
        }

        result.Checked++

        anchor := skill.CreatedAt
        if skill.LastUsedAt != nil && skill.LastUsedAt.After(anchor) {
            anchor = *skill.LastUsedAt
        }
        if skill.LastViewedAt != nil && skill.LastViewedAt.After(anchor) {
            anchor = *skill.LastViewedAt
        }
        if skill.LastPatchedAt != nil && skill.LastPatchedAt.After(anchor) {
            anchor = *skill.LastPatchedAt
        }

        switch skill.Status {
        case models.AgentSkillStatusActive:
            if anchor.Before(archiveCutoff) {
                if err := c.skillSvc.UpdateStatus(ctx, skill.ID, models.AgentSkillStatusArchived); err != nil {
                    return err
                }
                result.Archived++
                c.audit.Write(ctx, wsID, skill.ID, skill.Slug, "archive", "90 days inactive", "")
            } else if anchor.Before(staleCutoff) {
                if err := c.skillSvc.UpdateStatus(ctx, skill.ID, models.AgentSkillStatusStale); err != nil {
                    return err
                }
                result.MarkedStale++
                c.audit.Write(ctx, wsID, skill.ID, skill.Slug, "mark_stale", "30 days inactive", "")
            }
        case models.AgentSkillStatusStale:
            if anchor.Before(archiveCutoff) {
                if err := c.skillSvc.UpdateStatus(ctx, skill.ID, models.AgentSkillStatusArchived); err != nil {
                    return err
                }
                result.Archived++
                c.audit.Write(ctx, wsID, skill.ID, skill.Slug, "archive", "90 days inactive", "")
            } else if anchor.After(staleCutoff) {
                if err := c.skillSvc.UpdateStatus(ctx, skill.ID, models.AgentSkillStatusActive); err != nil {
                    return err
                }
                result.Reactivated++
                c.audit.Write(ctx, wsID, skill.ID, skill.Slug, "reactivate", "activity resumed", "")
            }
        }
    }
    return nil
}

type noopAuditWriter struct{}

func (n *noopAuditWriter) Write(ctx context.Context, workspaceID, skillID int, skillSlug, action, reason, details string) {}
```

- [ ] **Step 4: Run tests**

Run: `cd backend && go test ./internal/services/ -run TestCuratorService_applyTimeDrivenTransitions -v`
Expected: PASS

Run: `cd backend && go vet ./...`
Expected: pass (no stale callers — `UpdateStatus` is a new method, no existing callers)

- [ ] **Step 5: Commit**

```bash
git add backend/internal/services/curator_service.go backend/internal/services/curator_service_test.go backend/internal/services/agent_skill_service.go
git commit -m "feat(curator): scaffold CuratorService + time-driven transitions"
```

---

### Task 17: UsageSidecar persistence

**Depends on:** none

**Files:**
- Modify: `backend/internal/models/agent_skill.go`
- Test: `backend/internal/models/agent_skill_test.go`（追加测试到现有文件，或新建 `agent_skill_sidecar_test.go`）

- [ ] **Step 1: Write the failing test**

```go
// backend/internal/models/agent_skill_test.go
func TestAgentSkill_ParseUsageSidecar(t *testing.T) {
    skill := &AgentSkill{
        UsageSidecar: `{"useCount":5,"viewCount":10,"agentCreated":true,"state":"active"}`,
    }
    sidecar, err := skill.ParseUsageSidecar()
    require.NoError(t, err)
    assert.Equal(t, 5, sidecar.UseCount)
    assert.Equal(t, 10, sidecar.ViewCount)
    assert.True(t, sidecar.AgentCreated)
    assert.Equal(t, "active", sidecar.State)

    // empty column returns nil, no error
    empty := &AgentSkill{UsageSidecar: ""}
    s, err := empty.ParseUsageSidecar()
    require.NoError(t, err)
    assert.Nil(t, s)
}

func TestAgentSkill_SaveUsageSidecar(t *testing.T) {
    skill := &AgentSkill{}
    sidecar := &SkillUsageSidecar{
        UseCount:     3,
        ViewCount:    7,
        AgentCreated: true,
        State:        "stale",
    }
    err := skill.SaveUsageSidecar(sidecar)
    require.NoError(t, err)
    parsed, err := skill.ParseUsageSidecar()
    require.NoError(t, err)
    assert.Equal(t, 3, parsed.UseCount)
    assert.Equal(t, "stale", parsed.State)
}
```

Run: `cd backend && go test ./internal/models/ -run TestAgentSkill_ParseUsageSidecar -v`
Expected: FAIL — `SkillUsageSidecar`, `ParseUsageSidecar`, `SaveUsageSidecar` 未定义

- [ ] **Step 2: Implement `SkillUsageSidecar` and parser/saver**

在 `backend/internal/models/agent_skill.go` 中追加：

```go
// SkillUsageSidecar mirrors the .usage.json sidecar for Curator LLM review.
type SkillUsageSidecar struct {
    UseCount      int        `json:"useCount"`
    ViewCount     int        `json:"viewCount"`
    PatchCount    int        `json:"patchCount"`
    LastUsedAt    *time.Time `json:"lastUsedAt"`
    LastViewedAt  *time.Time `json:"lastViewedAt"`
    LastPatchedAt *time.Time `json:"lastPatchedAt"`
    AgentCreated  bool       `json:"agentCreated"`
    State         string     `json:"state"`
}

// ParseUsageSidecar unmarshals the JSON text column.
func (s *AgentSkill) ParseUsageSidecar() (*SkillUsageSidecar, error) {
    if strings.TrimSpace(s.UsageSidecar) == "" {
        return nil, nil
    }
    var sc SkillUsageSidecar
    if err := json.Unmarshal([]byte(s.UsageSidecar), &sc); err != nil {
        return nil, err
    }
    return &sc, nil
}

// SaveUsageSidecar marshals the sidecar into the JSON text column.
func (s *AgentSkill) SaveUsageSidecar(sc *SkillUsageSidecar) error {
    if sc == nil {
        s.UsageSidecar = ""
        return nil
    }
    b, err := json.Marshal(sc)
    if err != nil {
        return err
    }
    s.UsageSidecar = string(b)
    return nil
}
```

确保 `agent_skill.go` 文件头部有 `encoding/json` 和 `strings` import（Phase 1 已添加 JSON 解析器，应已有）。

- [ ] **Step 3: Run tests**

Run: `cd backend && go test ./internal/models/ -run TestAgentSkill_ParseUsageSidecar -v && go test ./internal/models/ -run TestAgentSkill_SaveUsageSidecar -v`
Expected: PASS

Run: `cd backend && go vet ./...`
Expected: pass

- [ ] **Step 4: Commit**

```bash
git add backend/internal/models/agent_skill.go backend/internal/models/agent_skill_test.go
git commit -m "feat(skill): UsageSidecar JSON persistence"
```

---

### Task 18: CuratorAuditLog model + write path

**Depends on:** none

**Files:**
- Create: `backend/internal/models/curator_audit_log.go`
- Create: `backend/internal/services/curator_audit.go`
- Test: `backend/internal/services/curator_audit_test.go`

- [ ] **Step 1: Write the failing test**

```go
// backend/internal/services/curator_audit_test.go
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

func TestDBAuditWriter_Write(t *testing.T) {
    db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
    require.NoError(t, err)
    require.NoError(t, db.AutoMigrate(&models.CuratorAuditLog{}))

    writer := NewDBAuditWriter(db)
    ctx := context.Background()

    err = writer.Write(ctx, 1, 42, "test-skill", "archive", "90 days inactive", `{"prev":"active"}`)
    require.NoError(t, err)

    var logs []models.CuratorAuditLog
    require.NoError(t, db.Find(&logs).Error)
    require.Len(t, logs, 1)
    assert.Equal(t, "test-skill", logs[0].SkillSlug)
    assert.Equal(t, "archive", logs[0].Action)
}
```

Run: `cd backend && go test ./internal/services/ -run TestDBAuditWriter_Write -v`
Expected: FAIL — `CuratorAuditLog`, `DBAuditWriter`, `NewDBAuditWriter`, `AuditWriter` 未定义

- [ ] **Step 2: Implement model and audit writer**

```go
// backend/internal/models/curator_audit_log.go
package models

import "time"

// CuratorAuditLog records every action taken by the Curator.
type CuratorAuditLog struct {
    ID          int       `gorm:"primaryKey;autoIncrement" json:"id"`
    WorkspaceID int       `gorm:"index:idx_curator_ws" json:"workspaceId"`
    SkillID     int       `gorm:"index:idx_curator_skill" json:"skillId"`
    SkillSlug   string    `gorm:"index:idx_curator_slug" json:"skillSlug"`
    Action      string    `json:"action"` // archive, mark_stale, reactivate, merge, model_review
    Reason      string    `json:"reason"`
    Details     string    `gorm:"type:text" json:"details"` // free-form JSON or text
    CreatedAt   time.Time `json:"createdAt"`
}
```

```go
// backend/internal/services/curator_audit.go
package services

import (
    "context"

    "github.com/odysseythink/hermind/backend/internal/models"
    "github.com/odysseythink/mlog"
    "gorm.io/gorm"
)

// AuditWriter persists Curator decisions.
type AuditWriter interface {
    Write(ctx context.Context, workspaceID, skillID int, skillSlug, action, reason, details string)
}

// DBAuditWriter writes audit entries to the database and structured logs.
type DBAuditWriter struct {
    db *gorm.DB
}

// NewDBAuditWriter creates a new database-backed audit writer.
func NewDBAuditWriter(db *gorm.DB) *DBAuditWriter {
    return &DBAuditWriter{db: db}
}

// Write persists the audit entry.
func (w *DBAuditWriter) Write(ctx context.Context, workspaceID, skillID int, skillSlug, action, reason, details string) {
    entry := &models.CuratorAuditLog{
        WorkspaceID: workspaceID,
        SkillID:     skillID,
        SkillSlug:   skillSlug,
        Action:      action,
        Reason:      reason,
        Details:     details,
    }
    if err := w.db.WithContext(ctx).Create(entry).Error; err != nil {
        mlog.Warn("curator audit log failed", mlog.Err(err), mlog.String("skill", skillSlug), mlog.String("action", action))
    }
    mlog.Info("curator action", mlog.String("skill", skillSlug), mlog.String("action", action), mlog.String("reason", reason))
}
```

- [ ] **Step 3: Run tests**

Run: `cd backend && go test ./internal/services/ -run TestDBAuditWriter_Write -v`
Expected: PASS

Run: `cd backend && go vet ./...`
Expected: pass

- [ ] **Step 4: Commit**

```bash
git add backend/internal/models/curator_audit_log.go backend/internal/services/curator_audit.go backend/internal/services/curator_audit_test.go
git commit -m "feat(curator): CuratorAuditLog model + DB audit writer"
```

---

### Task 19: LLM review prompt builder

**Depends on:** Task 16, Task 17

**Files:**
- Create: `backend/internal/services/curator_llm.go`
- Test: `backend/internal/services/curator_llm_test.go`

- [ ] **Step 1: Write the failing test**

```go
// backend/internal/services/curator_llm_test.go
package services

import (
    "strings"
    "testing"

    "github.com/odysseythink/hermind/backend/internal/models"
    "github.com/stretchr/testify/assert"
)

func TestBuildReviewPrompt(t *testing.T) {
    skills := []models.AgentSkill{
        {Slug: "skill-a", Name: "Skill A", Content: "Content of skill A"},
        {Slug: "skill-b", Name: "Skill B", Content: "Content of skill B"},
    }
    prompt := buildReviewPrompt(skills)
    assert.Contains(t, prompt, "Skill Curator")
    assert.Contains(t, prompt, "skill-a")
    assert.Contains(t, prompt, "skill-b")
    assert.Contains(t, prompt, "consolidations:")
    assert.Contains(t, prompt, "prunings:")
    assert.Contains(t, prompt, "absorbed:")
    assert.Contains(t, prompt, "DO NOT delete any skill")
}
```

Run: `cd backend && go test ./internal/services/ -run TestBuildReviewPrompt -v`
Expected: FAIL — `buildReviewPrompt` 未定义

- [ ] **Step 2: Implement prompt builder**

```go
// backend/internal/services/curator_llm.go
package services

import (
    "fmt"
    "strings"

    "github.com/odysseythink/hermind/backend/internal/models"
)

// Consolidation represents a model suggestion to merge one skill into another.
type Consolidation struct {
    Skill  string `yaml:"skill"`
    Into   string `yaml:"into"`
    Reason string `yaml:"reason"`
}

// Pruning represents a model suggestion to archive a skill.
type Pruning struct {
    Skill  string `yaml:"skill"`
    Action string `yaml:"action"`
    Reason string `yaml:"reason"`
}

// Absorbed represents a model-declared absorption relationship.
type Absorbed struct {
    Skill        string `yaml:"skill"`
    AbsorbedInto string `yaml:"absorbed_into"`
}

// buildReviewPrompt constructs the umbrella-merge prompt for the Curator LLM.
func buildReviewPrompt(skills []models.AgentSkill) string {
    var b strings.Builder
    b.WriteString("You are the Skill Curator. Review the following agent-created skills and decide how to consolidate them.\n\n")
    b.WriteString("Hard rules:\n")
    b.WriteString("1. DO NOT touch bundled or user-created skills.\n")
    b.WriteString("2. DO NOT delete any skill. Archiving is the maximum destructive action.\n")
    b.WriteString("3. DO NOT touch pinned skills.\n")
    b.WriteString("4. DO NOT use usage counters as a reason to skip consolidation.\n\n")
    b.WriteString("Output format (YAML inside triple backticks):\n")
    b.WriteString("```yaml\n")
    b.WriteString("consolidations:\n")
    b.WriteString("  - skill: <slug>\n")
    b.WriteString("    into: <umbrella-slug>\n")
    b.WriteString("    reason: <why>\n")
    b.WriteString("prunings:\n")
    b.WriteString("  - skill: <slug>\n")
    b.WriteString("    action: archive\n")
    b.WriteString("    reason: <why>\n")
    b.WriteString("absorbed:\n")
    b.WriteString("  - skill: <slug>\n")
    b.WriteString("    absorbed_into: <umbrella-slug>\n")
    b.WriteString("```\n\n")
    b.WriteString("Skills to review:\n")
    for _, s := range skills {
        preview := s.Content
        if len(preview) > 500 {
            preview = preview[:500] + "..."
        }
        b.WriteString(fmt.Sprintf("--- %s ---\n%s\n\n", s.Slug, preview))
    }
    return b.String()
}
```

- [ ] **Step 3: Run tests**

Run: `cd backend && go test ./internal/services/ -run TestBuildReviewPrompt -v`
Expected: PASS

Run: `cd backend && go vet ./...`
Expected: pass

- [ ] **Step 4: Commit**

```bash
git add backend/internal/services/curator_llm.go backend/internal/services/curator_llm_test.go
git commit -m "feat(curator): LLM review prompt builder"
```

---

### Task 20: LLM review response parser

**Depends on:** Task 19

**Files:**
- Modify: `backend/internal/services/curator_llm.go`
- Test: `backend/internal/services/curator_llm_test.go`（追加测试）

- [ ] **Step 1: Write the failing test**

```go
func TestParseReviewResponse(t *testing.T) {
    raw := `
Some preamble text.

` + "```yaml\n" + `
consolidations:
  - skill: skill-a
    into: skill-b
    reason: overlapping purpose
prunings:
  - skill: skill-c
    action: archive
    reason: obsolete
absorbed:
  - skill: skill-d
    absorbed_into: skill-e
` + "```\n" + `
Some trailing text.
`

    cons, prun, abs, err := parseReviewResponse(raw)
    require.NoError(t, err)
    require.Len(t, cons, 1)
    assert.Equal(t, "skill-a", cons[0].Skill)
    assert.Equal(t, "skill-b", cons[0].Into)
    require.Len(t, prun, 1)
    assert.Equal(t, "skill-c", prun[0].Skill)
    require.Len(t, abs, 1)
    assert.Equal(t, "skill-d", abs[0].Skill)
}

func TestParseReviewResponse_InvalidYAML(t *testing.T) {
    _, _, _, err := parseReviewResponse("not yaml at all")
    assert.Error(t, err)
}
```

Run: `cd backend && go test ./internal/services/ -run TestParseReviewResponse -v`
Expected: FAIL — `parseReviewResponse` 未定义

- [ ] **Step 2: Implement response parser**

在 `backend/internal/services/curator_llm.go` 中追加：

```go
import (
    "fmt"
    "regexp"
    "strings"

    "gopkg.in/yaml.v3"
)

// parseReviewResponse extracts YAML blocks from the LLM response.
func parseReviewResponse(response string) ([]Consolidation, []Pruning, []Absorbed, error) {
    // Extract content inside triple-backtick yaml blocks
    re := regexp.MustCompile("(?s)```yaml\\n(.*?)\\n```")
    matches := re.FindAllStringSubmatch(response, -1)

    if len(matches) == 0 {
        return nil, nil, nil, fmt.Errorf("no yaml block found in response")
    }

    var allCons []Consolidation
    var allPrun []Pruning
    var allAbs []Absorbed

    for _, m := range matches {
        block := strings.TrimSpace(m[1])
        var doc struct {
            Consolidations []Consolidation `yaml:"consolidations"`
            Prunings       []Pruning       `yaml:"prunings"`
            Absorbed       []Absorbed      `yaml:"absorbed"`
        }
        if err := yaml.Unmarshal([]byte(block), &doc); err != nil {
            // Try to continue with other blocks; log warning via caller
            continue
        }
        allCons = append(allCons, doc.Consolidations...)
        allPrun = append(allPrun, doc.Prunings...)
        allAbs = append(allAbs, doc.Absorbed...)
    }

    if len(allCons) == 0 && len(allPrun) == 0 && len(allAbs) == 0 {
        return nil, nil, nil, fmt.Errorf("no valid curator directives found")
    }

    return allCons, allPrun, allAbs, nil
}
```

- [ ] **Step 3: Run tests**

Run: `cd backend && go test ./internal/services/ -run TestParseReviewResponse -v && go test ./internal/services/ -run TestParseReviewResponse_InvalidYAML -v`
Expected: PASS

Run: `cd backend && go vet ./...`
Expected: pass

- [ ] **Step 4: Commit**

```bash
git add backend/internal/services/curator_llm.go backend/internal/services/curator_llm_test.go
git commit -m "feat(curator): LLM review response parser"
```

---

### Task 21: Three-signal reconciliation

**Depends on:** Task 18, Task 20

**Files:**
- Modify: `backend/internal/services/curator_service.go`
- Test: `backend/internal/services/curator_reconcile_test.go`

- [ ] **Step 1: Write the failing test**

```go
// backend/internal/services/curator_reconcile_test.go
package services

import (
    "context"
    "testing"
    "time"

    "github.com/odysseythink/hermind/backend/internal/models"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "gorm.io/driver/sqlite"
    "gorm.io/gorm"
)

func TestCuratorService_reconcileAndApply(t *testing.T) {
    db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
    require.NoError(t, err)
    require.NoError(t, db.AutoMigrate(&models.AgentSkill{}, &models.Workspace{}, &models.CuratorAuditLog{}))

    ctx := context.Background()
    ws := &models.Workspace{Name: "test", Slug: "test"}
    require.NoError(t, db.Create(ws).Error)

    now := time.Now()
    // target umbrella
    umbrella := &models.AgentSkill{WorkspaceID: ws.ID, Name: "Umbrella", Slug: "umbrella", Status: models.AgentSkillStatusActive, CreatedBy: "agent", CreatedAt: now}
    require.NoError(t, db.Create(umbrella).Error)
    // skill to be merged
    victim := &models.AgentSkill{WorkspaceID: ws.ID, Name: "Victim", Slug: "victim", Status: models.AgentSkillStatusActive, CreatedBy: "agent", CreatedAt: now}
    require.NoError(t, db.Create(victim).Error)
    // pinned skill should be immune
    pinned := &models.AgentSkill{WorkspaceID: ws.ID, Name: "Pinned", Slug: "pinned", Status: models.AgentSkillStatusActive, CreatedBy: "agent", CreatedAt: now, Pinned: true}
    require.NoError(t, db.Create(pinned).Error)

    skillSvc := NewAgentSkillService(db, nil, "")
    curator := NewCuratorService(skillSvc, nil, nil, "")
    curator.SetAuditWriter(NewDBAuditWriter(db))
    result := &CuratorRunResult{Timestamp: now}

    candidates := []models.AgentSkill{*umbrella, *victim, *pinned}
    consolidations := []Consolidation{{Skill: "victim", Into: "umbrella", Reason: "overlap"}}
    prunings := []Pruning{{Skill: "pinned", Action: "archive", Reason: "old"}}
    absorbed := []Absorbed{{Skill: "victim", AbsorbedInto: "umbrella"}}

    curator.reconcileAndApply(ctx, ws.ID, candidates, consolidations, prunings, absorbed, result)

    // victim merged -> archived
    var updatedVictim models.AgentSkill
    require.NoError(t, db.First(&updatedVictim, victim.ID).Error)
    assert.Equal(t, models.AgentSkillStatusArchived, updatedVictim.Status)
    assert.Equal(t, 1, result.Merged)

    // pinned immune -> still active
    var updatedPinned models.AgentSkill
    require.NoError(t, db.First(&updatedPinned, pinned.ID).Error)
    assert.Equal(t, models.AgentSkillStatusActive, updatedPinned.Status)

    // audit log should have entries
    var logs []models.CuratorAuditLog
    require.NoError(t, db.Find(&logs).Error)
    assert.GreaterOrEqual(t, len(logs), 1)
}
```

Run: `cd backend && go test ./internal/services/ -run TestCuratorService_reconcileAndApply -v`
Expected: FAIL — `reconcileAndApply`, `mergeSkill`, `archiveSkill` 未定义

- [ ] **Step 2: Implement reconciliation logic**

在 `backend/internal/services/curator_service.go` 中追加：

```go
import (
    "github.com/odysseythink/hermind/backend/internal/providers"
)

// applyLLMReview runs the LLM review phase.
func (c *CuratorService) applyLLMReview(ctx context.Context, wsID int, result *CuratorRunResult) error {
    if c.llmProv == nil {
        return fmt.Errorf("llm provider not configured")
    }

    skills, err := c.skillSvc.List(ctx, wsID, false)
    if err != nil {
        return err
    }

    var candidates []models.AgentSkill
    for _, s := range skills {
        if s.Pinned {
            continue
        }
        if s.Status != models.AgentSkillStatusActive && s.Status != models.AgentSkillStatusStale {
            continue
        }
        sidecar, _ := s.ParseUsageSidecar()
        if sidecar != nil && !sidecar.AgentCreated {
            continue
        }
        if sidecar == nil && s.CreatedBy != "agent" {
            continue
        }
        candidates = append(candidates, s)
    }

    if len(candidates) == 0 {
        return nil
    }

    prompt := buildReviewPrompt(candidates)
    messages := []core.Message{core.NewTextMessage(core.MESSAGE_ROLE_USER, prompt)}
    response, err := c.llmProv.Complete(ctx, messages, "", nil)
    if err != nil {
        return fmt.Errorf("llm complete failed: %w", err)
    }

    cons, prun, abs, err := parseReviewResponse(response)
    if err != nil {
        return fmt.Errorf("parse review response failed: %w", err)
    }

    c.reconcileAndApply(ctx, wsID, candidates, cons, prun, abs, result)
    return nil
}

func (c *CuratorService) reconcileAndApply(ctx context.Context, wsID int, candidates []models.AgentSkill,
    consolidations []Consolidation, prunings []Pruning, absorbed []Absorbed, result *CuratorRunResult) {

    candidateMap := make(map[string]*models.AgentSkill)
    for i := range candidates {
        candidateMap[candidates[i].Slug] = &candidates[i]
    }

    processed := make(map[string]bool)

    // Signal 1: absorbed_into (most authoritative)
    for _, a := range absorbed {
        if skill, ok := candidateMap[a.Skill]; ok && !skill.Pinned {
            c.mergeSkill(ctx, wsID, skill, a.AbsorbedInto, "model_declared_absorbed", result)
            processed[a.Skill] = true
        }
    }

    // Signal 2: consolidations
    for _, cons := range consolidations {
        if processed[cons.Skill] {
            continue
        }
        if skill, ok := candidateMap[cons.Skill]; ok && !skill.Pinned {
            if _, err := c.skillSvc.GetBySlug(ctx, wsID, cons.Into); err == nil {
                c.mergeSkill(ctx, wsID, skill, cons.Into, "model_consolidation: "+cons.Reason, result)
            } else {
                c.archiveSkill(ctx, wsID, skill, "consolidation_target_missing: "+cons.Into, result)
            }
            processed[cons.Skill] = true
        }
    }

    // Signal 3: prunings (archive suggestions)
    for _, p := range prunings {
        if processed[p.Skill] {
            continue
        }
        if skill, ok := candidateMap[p.Skill]; ok && !skill.Pinned {
            if p.Action == "archive" {
                c.archiveSkill(ctx, wsID, skill, "model_pruning: "+p.Reason, result)
            }
            processed[p.Skill] = true
        }
    }
}

func (c *CuratorService) mergeSkill(ctx context.Context, wsID int, skill *models.AgentSkill, intoSlug, reason string, result *CuratorRunResult) {
    if err := c.skillSvc.UpdateStatus(ctx, skill.ID, models.AgentSkillStatusArchived); err != nil {
        mlog.Warn("curator merge archive failed", mlog.Err(err), mlog.String("skill", skill.Slug))
        result.Errors = append(result.Errors, fmt.Sprintf("merge archive %s: %v", skill.Slug, err))
        return
    }
    result.Merged++
    c.audit.Write(ctx, wsID, skill.ID, skill.Slug, "merge", reason, fmt.Sprintf(`{"absorbed_into":"%s"}`, intoSlug))
}

func (c *CuratorService) archiveSkill(ctx context.Context, wsID int, skill *models.AgentSkill, reason string, result *CuratorRunResult) {
    if err := c.skillSvc.UpdateStatus(ctx, skill.ID, models.AgentSkillStatusArchived); err != nil {
        mlog.Warn("curator archive failed", mlog.Err(err), mlog.String("skill", skill.Slug))
        result.Errors = append(result.Errors, fmt.Sprintf("archive %s: %v", skill.Slug, err))
        return
    }
    result.Archived++
    c.audit.Write(ctx, wsID, skill.ID, skill.Slug, "archive", reason, "")
}
```

注意：需要在文件顶部添加 `github.com/odysseythink/pantheon/core` import（`core.Message`, `core.MESSAGE_ROLE_USER`）。但 `curator_service.go` 当前没有引入 `core`。如果 Pantheon 的 `core` 包路径不同，需要确认。根据已有代码，`core` 来自 `github.com/odysseythink/pantheon/core`。

- [ ] **Step 3: Wire `applyLLMReview` into `Run()`**

在 `curator_service.go` 的 `Run()` 方法中，取消 Phase B 的注释并启用：

```go
func (c *CuratorService) Run(ctx context.Context, wsID int) (*CuratorRunResult, error) {
    result := &CuratorRunResult{Timestamp: time.Now()}

    if err := c.applyTimeDrivenTransitions(ctx, wsID, result); err != nil {
        mlog.Warn("curator time-driven phase failed", mlog.Err(err), mlog.Int("workspace", wsID))
        result.Errors = append(result.Errors, fmt.Sprintf("time-driven: %v", err))
    }

    // Phase B: LLM review
    if c.llmProv != nil {
        if err := c.applyLLMReview(ctx, wsID, result); err != nil {
            mlog.Warn("curator LLM review failed, falling back to time-driven only",
                mlog.Err(err), mlog.Int("workspace", wsID))
            result.Errors = append(result.Errors, fmt.Sprintf("LLM review failed: %v", err))
        }
    }

    return result, nil
}
```

- [ ] **Step 4: Run tests**

Run: `cd backend && go test ./internal/services/ -run TestCuratorService_reconcileAndApply -v`
Expected: PASS

Run: `cd backend && go vet ./...`
Expected: pass

- [ ] **Step 5: Commit**

```bash
git add backend/internal/services/curator_service.go backend/internal/services/curator_reconcile_test.go
git commit -m "feat(curator): three-signal reconciliation + LLM review integration"
```

---

### Task 22: Curator report generation

**Depends on:** Task 21

**Files:**
- Create: `backend/internal/services/curator_report.go`
- Test: `backend/internal/services/curator_report_test.go`

- [ ] **Step 1: Write the failing test**

```go
// backend/internal/services/curator_report_test.go
package services

import (
    "encoding/json"
    "os"
    "path/filepath"
    "testing"
    "time"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestSaveCuratorReport(t *testing.T) {
    tmpDir := t.TempDir()
    result := &CuratorRunResult{
        Timestamp:   time.Now(),
        Checked:     5,
        MarkedStale: 1,
        Archived:    2,
        Merged:      1,
    }

    dir, err := saveCuratorReport(tmpDir, 42, result)
    require.NoError(t, err)

    // Verify run.json
    jsonPath := filepath.Join(dir, "run.json")
    require.FileExists(t, jsonPath)
    b, err := os.ReadFile(jsonPath)
    require.NoError(t, err)
    var parsed CuratorRunResult
    require.NoError(t, json.Unmarshal(b, &parsed))
    assert.Equal(t, 5, parsed.Checked)

    // Verify REPORT.md
    mdPath := filepath.Join(dir, "REPORT.md")
    require.FileExists(t, mdPath)
    md, err := os.ReadFile(mdPath)
    require.NoError(t, err)
    assert.Contains(t, string(md), "## Curator Report")
    assert.Contains(t, string(md), "Checked: 5")
}
```

Run: `cd backend && go test ./internal/services/ -run TestSaveCuratorReport -v`
Expected: FAIL — `saveCuratorReport` 未定义

- [ ] **Step 2: Implement report generator**

```go
// backend/internal/services/curator_report.go
package services

import (
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
    "time"

    "github.com/odysseythink/mlog"
)

// saveCuratorReport writes the machine-readable and human-readable reports.
func saveCuratorReport(storageDir string, wsID int, result *CuratorRunResult) (string, error) {
    ts := result.Timestamp.Format("20060102-150405")
    dir := filepath.Join(storageDir, "logs", "curator", fmt.Sprintf("ws_%d", wsID), ts)
    if err := os.MkdirAll(dir, 0755); err != nil {
        return "", fmt.Errorf("mkdir report dir: %w", err)
    }

    // Machine-readable JSON
    jsonPath := filepath.Join(dir, "run.json")
    jsonData, err := json.MarshalIndent(result, "", "  ")
    if err != nil {
        return "", fmt.Errorf("marshal report json: %w", err)
    }
    if err := os.WriteFile(jsonPath, jsonData, 0644); err != nil {
        return "", fmt.Errorf("write report json: %w", err)
    }

    // Human-readable Markdown
    mdPath := filepath.Join(dir, "REPORT.md")
    md := generateReportMarkdown(result)
    if err := os.WriteFile(mdPath, []byte(md), 0644); err != nil {
        return "", fmt.Errorf("write report markdown: %w", err)
    }

    mlog.Info("curator report saved", mlog.String("dir", dir))
    return dir, nil
}

func generateReportMarkdown(r *CuratorRunResult) string {
    var b string
    b += fmt.Sprintf("# Curator Report — %s\n\n", r.Timestamp.Format(time.RFC3339))
    b += "## Summary\n\n"
    b += fmt.Sprintf("- **Checked:** %d\n", r.Checked)
    b += fmt.Sprintf("- **Marked Stale:** %d\n", r.MarkedStale)
    b += fmt.Sprintf("- **Archived:** %d\n", r.Archived)
    b += fmt.Sprintf("- **Reactivated:** %d\n", r.Reactivated)
    b += fmt.Sprintf("- **Merged:** %d\n", r.Merged)
    if len(r.Errors) > 0 {
        b += "\n## Errors\n\n"
        for _, e := range r.Errors {
            b += fmt.Sprintf("- %s\n", e)
        }
    }
    if len(r.Consolidations) > 0 {
        b += "\n## Consolidations\n\n"
        for _, c := range r.Consolidations {
            b += fmt.Sprintf("- `%s` → `%s` (%s)\n", c.Skill, c.Into, c.Reason)
        }
    }
    if len(r.Prunings) > 0 {
        b += "\n## Prunings\n\n"
        for _, p := range r.Prunings {
            b += fmt.Sprintf("- `%s` — %s (%s)\n", p.Skill, p.Action, p.Reason)
        }
    }
    if len(r.Absorbed) > 0 {
        b += "\n## Absorbed\n\n"
        for _, a := range r.Absorbed {
            b += fmt.Sprintf("- `%s` absorbed into `%s`\n", a.Skill, a.AbsorbedInto)
        }
    }
    return b
}
```

- [ ] **Step 3: Wire report saving into `CuratorService.Run()`**

在 `curator_service.go` 的 `Run()` 方法末尾，在 `return result, nil` 之前添加：

```go
    if _, err := saveCuratorReport(c.storageDir, wsID, result); err != nil {
        mlog.Warn("curator report save failed", mlog.Err(err), mlog.Int("workspace", wsID))
        result.Errors = append(result.Errors, fmt.Sprintf("report save: %v", err))
    }
```

- [ ] **Step 4: Run tests**

Run: `cd backend && go test ./internal/services/ -run TestSaveCuratorReport -v`
Expected: PASS

Run: `cd backend && go vet ./...`
Expected: pass

- [ ] **Step 5: Commit**

```bash
git add backend/internal/services/curator_report.go backend/internal/services/curator_report_test.go backend/internal/services/curator_service.go
git commit -m "feat(curator): report generation (JSON + Markdown)"
```

---

### Task 23: Worker integration (shared signature change)

**Depends on:** Task 16, Task 17, Task 18, Task 21, Task 22

**Files:**
- Modify: `backend/internal/workers/skill_curator.go`
- Modify: `backend/internal/workers/skill_curator_test.go`
- Modify: `backend/internal/workers/manager.go`
- Modify: `backend/cmd/server/main.go`

- [ ] **Step 1: Change `SkillCuratorJob` signature + update all callers**

修改 `backend/internal/workers/skill_curator.go`：

```go
type SkillCuratorJob struct {
    db         *gorm.DB
    skillSvc   services.AgentSkillManager
    sysSvc     *services.SystemService
    curatorSvc *services.CuratorService
}

func NewSkillCuratorJob(db *gorm.DB, skillSvc services.AgentSkillManager, sysSvc *services.SystemService, curatorSvc *services.CuratorService) *SkillCuratorJob {
    return &SkillCuratorJob{db: db, skillSvc: skillSvc, sysSvc: sysSvc, curatorSvc: curatorSvc}
}
```

重写 `Run` 方法：

```go
func (j *SkillCuratorJob) Run(ctx context.Context) error {
    // Fetch all workspace IDs
    var workspaceIDs []int
    if err := j.db.WithContext(ctx).Model(&models.Workspace{}).Pluck("id", &workspaceIDs).Error; err != nil {
        return err
    }

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

    var totalChecked, totalStale, totalArchived, totalReactivated int
    for _, wsID := range workspaceIDs {
        result, err := j.curatorSvc.Run(ctx, wsID)
        if err != nil {
            mlog.Warn("skill curator failed for workspace", mlog.Err(err), mlog.Int("workspace", wsID))
            continue // do not block other workspaces
        }
        totalChecked += result.Checked
        totalStale += result.MarkedStale
        totalArchived += result.Archived
        totalReactivated += result.Reactivated
    }

    mlog.Info("skill curator run complete",
        mlog.Int("workspaces", len(workspaceIDs)),
        mlog.Int("checked", totalChecked),
        mlog.Int("marked_stale", totalStale),
        mlog.Int("archived", totalArchived),
        mlog.Int("reactivated", totalReactivated),
    )
    return nil
}
```

- [ ] **Step 2: Find and update EVERY caller (prod + tests)**

Run: `grep -rn "NewSkillCuratorJob(" backend/`
Expected hits:
- `backend/cmd/server/main.go:248`
- `backend/internal/workers/skill_curator_test.go`（需要确认具体行号）

修改 `backend/cmd/server/main.go:248`：

```go
workers.NewSkillCuratorJob(db, agentSkillSvc, sysSvc, curatorSvc),
```

修改 `backend/internal/workers/skill_curator_test.go`：
更新 `NewSkillCuratorJob` 调用，传入 `curatorSvc`（如果测试使用旧接口 `ApplyCuratorTransitions`，需要重写测试以适应新的 `curatorSvc.Run` 方式；但由于 `curatorSvc` 内部调用 `skillSvc`，测试中可以直接构造 `CuratorService` 并传入 mock DB）。

同时，在 `backend/internal/workers/manager.go` 的 `wrapJob` 中的 timeout switch 添加 curator 专用超时：

```go
case "skill-curator":
    if d, err := time.ParseDuration(m.cfg.WorkerSkillCuratorTimeout); err == nil {
        timeout = d
    }
```

（注意：`config.Config` 需要新增 `WorkerSkillCuratorTimeout` 字段。如果该字段不存在，暂时使用 `5 * time.Minute` 默认即可。V1 可以先不加配置字段，直接在 switch 中不做特殊处理，因为默认 5m 已足够。为了简化，本 task 不修改 `config.Config`，仅添加 case 但无配置覆盖：）

```go
case "skill-curator":
    // default 5m is sufficient; future: m.cfg.WorkerSkillCuratorTimeout
```

或者更简洁：不修改 manager.go，因为默认 5m 对 Curator 来说足够了。

- [ ] **Step 3: Whole-tree typecheck (incl. tests) + targeted test**

Run: `cd backend && go vet ./...`
Expected: pass（包括 `skill_curator_test.go` 中的新签名）

Run: `cd backend && go test ./internal/workers/ -run TestSkillCuratorJob_Run -v`
Expected: PASS（测试需要适配新的调用方式；如果旧测试依赖 `ApplyCuratorTransitions` 的 mock，需要重写为使用真实 DB + CuratorService）

- [ ] **Step 4: Commit**

```bash
git add backend/internal/workers/skill_curator.go backend/internal/workers/skill_curator_test.go backend/cmd/server/main.go
git commit -m "feat(curator): wire CuratorService into SkillCuratorJob"
```

---

### Task 24: REST API for curator report

**Depends on:** Task 22

**Files:**
- Modify: `backend/internal/handlers/agent_skills.go`
- Modify: `backend/internal/handlers/agent_skills_test.go`
- Modify: `backend/cmd/server/main.go`（`RegisterAgentSkillsRoutes` 调用签名）

- [ ] **Step 1: Change `RegisterAgentSkillsRoutes` and `AgentSkillsHandler` signatures**

修改 `backend/internal/handlers/agent_skills.go`：

```go
type AgentSkillsHandler struct {
    skillSvc   services.AgentSkillManager
    storageDir string
}

func NewAgentSkillsHandler(skillSvc services.AgentSkillManager, storageDir string) *AgentSkillsHandler {
    return &AgentSkillsHandler{skillSvc: skillSvc, storageDir: storageDir}
}
```

在 `RegisterAgentSkillsRoutes` 中：

```go
func RegisterAgentSkillsRoutes(r *gin.RouterGroup, skillSvc services.AgentSkillManager, authSvc *services.AuthService, db *gorm.DB, storageDir string) {
    h := NewAgentSkillsHandler(skillSvc, storageDir)
    // ... existing routes ...
    group.GET("/curator-report", h.GetCuratorReport)
}
```

添加 handler 方法：

```go
func (h *AgentSkillsHandler) GetCuratorReport(c *gin.Context) {
    ws := c.MustGet("workspace").(*models.Workspace)

    // Find the most recent curator report directory for this workspace
    baseDir := filepath.Join(h.storageDir, "logs", "curator", fmt.Sprintf("ws_%d", ws.ID))
    entries, err := os.ReadDir(baseDir)
    if err != nil {
        if os.IsNotExist(err) {
            c.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "no curator reports found"})
            return
        }
        mlog.Error("read curator reports", mlog.Err(err))
        c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "internal server error"})
        return
    }

    var latest string
    for _, e := range entries {
        if e.IsDir() && e.Name() > latest {
            latest = e.Name()
        }
    }
    if latest == "" {
        c.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "no curator reports found"})
        return
    }

    jsonPath := filepath.Join(baseDir, latest, "run.json")
    data, err := os.ReadFile(jsonPath)
    if err != nil {
        mlog.Error("read curator report json", mlog.Err(err))
        c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "internal server error"})
        return
    }

    c.Data(http.StatusOK, "application/json", data)
}
```

确保文件顶部 import 包含 `fmt`, `os`, `path/filepath`。

- [ ] **Step 2: Find and update EVERY caller (prod + tests)**

Run: `grep -rn "RegisterAgentSkillsRoutes(" backend/`
Expected hits:
- `backend/cmd/server/main.go:324`
- `backend/internal/handlers/agent_skills_test.go`（如果有）
- `backend/internal/handlers/agent_skills.go:240`（内部 — 这是定义处）

更新 `backend/cmd/server/main.go:324`：

```go
handlers.RegisterAgentSkillsRoutes(api, agentSkillSvc, authSvc, db, cfg.StorageDir)
```

更新 `backend/internal/handlers/agent_skills_test.go` 中的 `NewAgentSkillsHandler` 调用（如果有）：

```go
h := NewAgentSkillsHandler(svc, t.TempDir())
```

以及 `RegisterAgentSkillsRoutes` 调用：

```go
handlers.RegisterAgentSkillsRoutes(r, svc, authSvc, db, t.TempDir())
```

- [ ] **Step 3: Write handler test**

```go
func TestAgentSkillsHandler_GetCuratorReport(t *testing.T) {
    tmpDir := t.TempDir()
    db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
    require.NoError(t, err)
    require.NoError(t, db.AutoMigrate(&models.Workspace{}, &models.AgentSkill{}))

    ws := &models.Workspace{Name: "test", Slug: "test-ws"}
    require.NoError(t, db.Create(ws).Error)

    // Seed a report
    reportDir := filepath.Join(tmpDir, "logs", "curator", fmt.Sprintf("ws_%d", ws.ID), "20260101-120000")
    require.NoError(t, os.MkdirAll(reportDir, 0755))
    require.NoError(t, os.WriteFile(filepath.Join(reportDir, "run.json"), []byte(`{"checked":3}`), 0644))

    h := NewAgentSkillsHandler(nil, tmpDir)
    c, _ := gin.CreateTestContext(httptest.NewRecorder())
    c.Set("workspace", ws)

    h.GetCuratorReport(c)
    assert.Equal(t, http.StatusOK, c.Writer.Status())
}
```

- [ ] **Step 4: Whole-tree typecheck + targeted test**

Run: `cd backend && go vet ./...`
Expected: pass

Run: `cd backend && go test ./internal/handlers/ -run TestAgentSkillsHandler_GetCuratorReport -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/internal/handlers/agent_skills.go backend/internal/handlers/agent_skills_test.go backend/cmd/server/main.go
git commit -m "feat(api): GET /workspace/:slug/agent-skills/curator-report endpoint"
```

---

### Task 25: Phase 3 integration tests

**Depends on:** Task 23, Task 24

**Files:**
- Create: `backend/tests/integration/curator_integration_test.go`

- [ ] **Step 1: Write integration test**

```go
// backend/tests/integration/curator_integration_test.go
package integration

import (
    "context"
    "net/http"
    "net/http/httptest"
    "os"
    "path/filepath"
    "testing"
    "time"

    "github.com/gin-gonic/gin"
    "github.com/odysseythink/hermind/backend/internal/handlers"
    "github.com/odysseythink/hermind/backend/internal/models"
    "github.com/odysseythink/hermind/backend/internal/services"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "gorm.io/driver/sqlite"
    "gorm.io/gorm"
)

func TestCurator_EndToEnd(t *testing.T) {
    db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
    require.NoError(t, err)
    require.NoError(t, db.AutoMigrate(
        &models.Workspace{},
        &models.AgentSkill{},
        &models.CuratorAuditLog{},
        &models.SystemSetting{},
    ))

    ctx := context.Background()
    tmpDir := t.TempDir()

    ws := &models.Workspace{Name: "integration", Slug: "integration"}
    require.NoError(t, db.Create(ws).Error)

    now := time.Now()
    oldSkill := &models.AgentSkill{
        WorkspaceID: ws.ID,
        Name:        "Old",
        Slug:        "old",
        Status:      models.AgentSkillStatusActive,
        CreatedBy:   "agent",
        CreatedAt:   now.AddDate(0, 0, -100),
    }
    require.NoError(t, db.Create(oldSkill).Error)

    skillSvc := services.NewAgentSkillService(db, nil, tmpDir)
    curatorSvc := services.NewCuratorService(skillSvc, nil, nil, tmpDir)
    curatorSvc.SetAuditWriter(services.NewDBAuditWriter(db))

    // Run curator
    result, err := curatorSvc.Run(ctx, ws.ID)
    require.NoError(t, err)
    assert.Equal(t, 1, result.Checked)
    assert.Equal(t, 1, result.Archived)

    // Verify DB state
    var updated models.AgentSkill
    require.NoError(t, db.First(&updated, oldSkill.ID).Error)
    assert.Equal(t, models.AgentSkillStatusArchived, updated.Status)

    // Verify audit log
    var logs []models.CuratorAuditLog
    require.NoError(t, db.Find(&logs).Error)
    require.Len(t, logs, 1)
    assert.Equal(t, "archive", logs[0].Action)

    // Verify report files exist
    baseDir := filepath.Join(tmpDir, "logs", "curator", fmt.Sprintf("ws_%d", ws.ID))
    entries, err := os.ReadDir(baseDir)
    require.NoError(t, err)
    require.Len(t, entries, 1)
    require.FileExists(t, filepath.Join(baseDir, entries[0].Name(), "run.json"))
    require.FileExists(t, filepath.Join(baseDir, entries[0].Name(), "REPORT.md"))

    // Verify REST endpoint
    gin.SetMode(gin.TestMode)
    r := gin.New()
    authSvc := services.NewAuthService(db, &config.Config{JWTSecret: "test"}, nil)
    handlers.RegisterAgentSkillsRoutes(r.Group("/api"), skillSvc, authSvc, db, tmpDir)

    req, _ := http.NewRequest("GET", "/api/workspace/integration/agent-skills/curator-report", nil)
    w := httptest.NewRecorder()
    r.ServeHTTP(w, req)
    assert.Equal(t, http.StatusOK, w.Code)
    assert.Contains(t, w.Body.String(), `"checked":1`)
}
```

注意：需要在文件顶部添加缺失的 import，如 `fmt`（用于 `fmt.Sprintf`）以及 `config`。

- [ ] **Step 2: Run integration test**

Run: `cd backend && go test ./tests/integration/ -run TestCurator_EndToEnd -v`
Expected: PASS

- [ ] **Step 3: Run full backend test suite**

Run: `cd backend && go vet ./... && go test ./... -run=^$`
Expected: 全树编译通过（包括所有测试文件）

- [ ] **Step 4: Commit**

```bash
git add backend/tests/integration/curator_integration_test.go
git commit -m "test(integration): Phase 3 curator end-to-end"
```

---

## Self-Review

- [ ] **1. Spec coverage (build the table).**

| Spec section | Task(s) | Status |
|---|---|---|
| §6.1 CuratorService 主入口 | Task 16 | covered |
| §6.2 时间驱动转换 | Task 16 | covered |
| §6.3 LLM 审查 Prompt | Task 19 | covered |
| §6.4 三层信号归并 | Task 21 | covered |
| UsageSidecar 持久化 | Task 17 | covered |
| CuratorAuditLog 模型 | Task 18 | covered |
| 审计日志写入 | Task 18 | covered |
| 报告生成 | Task 22 | covered |
| Worker 集成 | Task 23 | covered |
| REST API 报告端点 | Task 24 | covered |
| §8 测试计划（Phase 3） | Task 25 | covered |

- [ ] **2. Placeholder scan:** 搜索计划中是否有 `TODO`/`TBD`、deferred-by-dependency 借口、dead-code 占位符。Phase 3 所有步骤均含完整代码块和明确命令，无占位。

- [ ] **3. No phantom tasks (binary):** 10 个任务均产生可验证的代码或测试变更。Task 16-25 各有明确的创建/修改文件列表。无空提交。

- [ ] **4. Dependency soundness:**
- Task 16 → Task 17（时间驱动需要 ParseUsageSidecar）
- Task 19 → Task 16, 17（Prompt builder 需要 CuratorService 和 UsageSidecar）
- Task 20 → Task 19（parser 依赖 builder 中的类型定义）
- Task 21 → Task 18, 20（reconciliation 需要 AuditWriter 和 parser 输出）
- Task 22 → Task 21（report 需要 result  populated by reconciliation）
- Task 23 → Task 16, 17, 18, 21, 22（Worker 集成需要完整 CuratorService + audit + report）
- Task 24 → Task 22（REST API 读取 report 目录）
- Task 25 → 所有前置任务
所有依赖均指向更早的任务。

- [ ] **5. Caller & build soundness:**
- **Task 16** 扩展 `AgentSkillManager` 接口添加 `UpdateStatus`：只有 `AgentSkillService` 实现 + `CuratorService` 调用。无其他外部调用者。`go vet ./...` 验证。
- **Task 23** 修改 `NewSkillCuratorJob` 签名：更新 `main.go` + `skill_curator_test.go`。`go vet ./...` 验证。
- **Task 24** 修改 `NewAgentSkillsHandler` 和 `RegisterAgentSkillsRoutes` 签名：更新 `main.go` + `agent_skills_test.go`。`go vet ./...` 验证。
- 无同一签名在多个任务中被重复修改。

- [ ] **6. Test-the-risk:**
- Task 16：状态变更测试（active → archived，基于时间边界）
- Task 17：序列化/反序列化状态测试
- Task 18：DB 写入行为测试（验证 CuratorAuditLog 行存在）
- Task 21：三层信号优先级 + pinned 免疫 + 目标缺失降级测试
- Task 23：Worker 运行后 DB + 文件系统状态验证
- Task 25：端到端集成测试

- [ ] **7. Type consistency:**
- `CuratorRunResult` 在 Task 16 定义，Task 21/22/23/25 使用相同字段名（`Checked`, `MarkedStale`, `Archived`, `Reactivated`, `Merged`, `Errors`, `Consolidations`, `Prunings`, `Absorbed`）
- `AuditWriter.Write` 签名在 Task 18 定义，Task 16（noop/noopAuditWriter）和 Task 21（DBAuditWriter）均一致使用
- `Consolidation`/`Pruning`/`Absorbed` 在 Task 19 定义，Task 20（parser 返回）、Task 21（reconcile 输入）、Task 22（report 输出）均使用相同类型
