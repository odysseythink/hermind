# Batch 2 — Data Models + Service Interfaces

## 1. Data Models

### 1.1 CuratorReport

文件: `backend/internal/models/curator_report.go` [新增] [C:USER]

```go
type CuratorReport struct {
    ID            int       `gorm:"primaryKey;autoIncrement" json:"id"`
    WorkspaceID   int       `gorm:"index;not null" json:"workspaceId"`
    RunID         string    `gorm:"index;not null" json:"runId"`       // "20060102-150405"
    Mode          string    `gorm:"not null" json:"mode"`              // dry_run | live
    Status        string    `gorm:"not null" json:"status"`            // running | completed | failed
    SafetyLevel   string    `gorm:"not null" json:"safetyLevel"`       // high | medium | low
    ModelProvider string    `json:"modelProvider"`
    ModelID       string    `json:"modelID"`
    SnapshotID    string    `json:"snapshotId"`                        // [C:USER] 关联 Batch 1 Backup
    Summary       string    `gorm:"type:text" json:"summary"`
    DecisionsJSON string    `gorm:"type:text" json:"-"`
    Decisions     []CuratorReviewDecision `gorm:"-" json:"decisions"`
    Reasoning     string    `gorm:"type:text" json:"reasoning"`        // [C:USER] 完整 LLM 推理
    TokenUsage    int       `json:"tokenUsage"`
    CostUSD       float64   `json:"costUSD"`
    ErrorMessage  string    `json:"errorMessage,omitempty"`
    StartedAt     time.Time `json:"startedAt"`
    CompletedAt   *time.Time `json:"completedAt,omitempty"`
}

func (CuratorReport) TableName() string { return "curator_reports" }
```

### 1.2 CuratorReviewDecision

文件: `backend/internal/models/curator_report.go` [同文件] [C:USER]

```go
type CuratorReviewDecision struct {
    SkillID     int    `json:"skillId"`
    SkillName   string `json:"skillName"`
    SkillSlug   string `json:"skillSlug"`
    Action      string `json:"action"`                  // keep | patch | consolidate | archive
    TargetSkill string `json:"targetSkill,omitempty"`   // umbrella name for consolidate
    TargetSlug  string `json:"targetSlug,omitempty"`
    Reason      string `json:"reason"`
    Applied     bool   `json:"applied"`
    DryRun      bool   `json:"dryRun"`
}
```

### 1.3 AgentSkill 字段使用

沿用 Batch 1 新增字段 [C:USER]：

- `WriteOrigin`: 创建 umbrella 时设为 `"curator"`
- `CreatedBy`: 创建 umbrella 时设为 `"agent"`
- `Status`: 被合并的 skill 更新为 `"archived"`
- `Pinned`: Review 候选集过滤 `Pinned == false`

---

## 2. Service Interfaces

### 2.1 LLMReviewService

文件: `backend/internal/services/llm_review_service.go` [新增] [C:USER]

```go
type ReviewOptions struct {
    MaxIterations int
    DryRun        bool
    SafetyLevel   string            // high | medium | low
    ModelProvider string            // [C:USER] 独立 curator provider
    ModelID       string
    APIKey        string
}

type ReviewResult struct {
    Decisions  []models.CuratorReviewDecision
    Reasoning  string
    TokenUsage int
    CostUSD    float64
}

type LLMReviewService interface {
    Review(ctx context.Context, workspaceID int, opts ReviewOptions) (*ReviewResult, error)
}

type llmReviewService struct {
    db            *gorm.DB
    skillSvc      AgentSkillManager
    backupSvc     BackupManager
    runtime       ReviewAgentRuntime
    provenanceSvc ProvenanceRecorder
}
```

### 2.2 UmbrellaBuilder

文件: `backend/internal/services/umbrella_builder.go` [新增] [C:USER]

```go
type UmbrellaBuilder interface {
    Build(ctx context.Context, workspaceID int, decisions []models.CuratorReviewDecision) ([]models.CuratorReviewDecision, error)
}

type umbrellaBuilder struct {
    db       *gorm.DB
    skillSvc AgentSkillManager
}
```

### 2.3 CuratorReportService

文件: `backend/internal/services/curator_report_service.go` [新增] [C:USER]

```go
type CuratorReportService interface {
    Create(ctx context.Context, workspaceID int, mode, safetyLevel, snapshotID string) (*models.CuratorReport, error)
    Finalize(ctx context.Context, report *models.CuratorReport, result *ReviewResult, err error) error
    SaveFiles(ctx context.Context, report *models.CuratorReport) error
    List(ctx context.Context, workspaceID int, limit int) ([]models.CuratorReport, error)
    HasReports(ctx context.Context) (bool, error)
}

type curatorReportService struct {
    db         *gorm.DB
    storageDir string
}
```

### 2.4 ReviewAgentRuntime

文件: `backend/internal/agent/review_agent.go` [新增] [C:USER]

```go
type ReviewAgentRuntime interface {
    RunReview(ctx context.Context, workspaceID int, prompt string, modelSettings map[string]string) (*ReviewAgentResult, error)
}

type ReviewAgentResult struct {
    FinalResponse string
    ToolCalls     []ReviewAgentToolCall
    TokenUsage    int
}

type ReviewAgentToolCall struct {
    ToolName string
    Args     map[string]any
    Result   string
}
```

### 2.5 DTO 扩展

文件: `backend/internal/dto/agent_skill.go` [扩展] [C:USER]

```go
type CreateAgentSkillRequest struct {
    // ... existing fields ...
    WriteOrigin string `json:"writeOrigin"` // default "foreground"
}

type UpdateAgentSkillRequest struct {
    // ... existing fields ...
    WriteOrigin string `json:"writeOrigin"` // optional
}
```

---

## 3. Local Notes

### 3.1 数据库迁移

- `CuratorReport` 和 `CuratorReviewDecision` 通过 GORM AutoMigrate 自动创建
- `DecisionsJSON` 存储 JSON 字符串，由 `BeforeCreate`/`AfterFind` hook 与 `Decisions` 字段互转

### 3.2 索引设计

- `idx_curator_reports_workspace_run` on `(workspace_id, run_id)`
- `idx_curator_reports_status` on `status`
- `idx_curator_reports_started_at` on `started_at`

### 3.3 与 Batch 1 的依赖

- 依赖 `BackupManager`（Batch 1）做快照
- 依赖 `ProvenanceRecorder`（Batch 1）记录 curator 写入
- 依赖 `AgentSkill.Pinned` 和 `WriteOrigin`（Batch 1）

### 3.4 本 Part 风险

| Risk | Mitigation |
|------|-----------|
| `DecisionsJSON` 字段过大 | 单个 workspace skills 数量有限，且 archive 决策占空间小 |
| `ReviewAgentResult.TokenUsage` 无法从 Pantheon 直接获取 | 使用 `core.Response` 的 usage 字段或近似估算 |
