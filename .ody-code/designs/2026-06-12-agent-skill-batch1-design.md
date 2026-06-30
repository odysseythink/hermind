# Agent Skill System — Batch 1 基础设施增强设计

> **审计级别**: Deep  
> **批次**: Batch 1（Pin Mechanism + Provenance + Backup/Rollback）  
> **方案**: B — 完整基础设施  
> **日期**: 2026-06-11

---

## Scope In/Out

### In

| # | 功能 | 范围 | 来源 |
|---|------|------|------|
| 1 | **Pin Mechanism** | Curator 跳过 Pinned 技能；Agent 工具（delete/edit/patch/write_file/remove_file）拒绝修改 Pinned 技能 | [C:USER] |
| 2 | **Provenance** | `AgentSkill` 新增 `WriteOrigin` 字段；新增 `SkillProvenanceLog` 表；所有 skill 修改操作自动记录完整内容快照 | [C:USER] |
| 3 | **Backup/Rollback** | `BackupService.Snapshot(workspaceID)` 导出 JSON；保留最近 10 个；失败阻断；`Restore(snapshotID)` 回滚 | [C:USER] |

### Out

| # | 功能 | 延后理由 |
|---|------|---------|
| 1 | 前端 Pin UI（toggle + filter） | 前端改动独立，可与 Batch 2 一起交付 | [C:DEFERRED] |
| 2 | LLM Review Fork | 属于 Batch 2 | [C:DEFERRED] |
| 3 | Umbrella Building | 属于 Batch 2 | [C:DEFERRED] |
| 4 | 审计日志查询 API | 如前端需要查看历史再添加 | [C:DEFERRED] |
| 5 | 备份压缩/加密 | 10 个 JSON 快照体积可控 | [C:DEFERRED] |

---

## Architecture

```
Curator Worker
├── BackupService.Snapshot(wsID) ──→ <StorageDir>/skill-backups/{wsID}/{timestamp}.json
│   [失败 → mlog.Error + return error 阻断]
└── SkillService.ApplyCuratorTransitions(wsID)
    └── if skill.Pinned { continue }

Agent Tools (skill_manage)
├── GetBySlug → if Pinned → return tool.Error
├── Execute operation
└── ProvenanceService.Record(ctx, skill, action, ...)

API Handlers
└── List/Get → 返回 Pinned + WriteOrigin

ProvenanceService
└── Record → GORM Create (SkillProvenanceLog)

BackupService
├── Snapshot → JSON file + Prune(keep=10)
└── Restore → JSON read + tx replace
```

---

## Assumptions & Unverified Items

| # | Assumption | Confidence | Impact if wrong | How to verify |
|---|-----------|------------|-----------------|---------------|

---

## Risk Register

| # | Risk | Likelihood | Impact | Mitigation |
|---|------|-----------|--------|-----------|

---

## Parts

本设计为单一文件（非 split），以下内容按顺序追加：

| # | Section | Status |
|---|---------|--------|
| 1 | Data Models & Interfaces | **done** |
| 2 | Algorithms & Call-sites | **done** |
| 3 | Error Table & Test Plan | **done** |
| 4 | Self-Review & Audit Gate | **done** |

---

## Part 1 — Data Models & Interfaces

### 1.1 AgentSkill 模型扩展

文件: `backend/internal/models/agent_skill.go` [C:USER]

```go
type AgentSkill struct {
    ID            int        `gorm:"primaryKey;autoIncrement" json:"id"`
    WorkspaceID   int        `gorm:"index:idx_ws_slug,unique;not null" json:"workspaceId"`
    Name          string     `gorm:"not null" json:"name"`
    Slug          string     `gorm:"index:idx_ws_slug,unique;not null" json:"slug"`
    Description   string     `json:"description"`
    Category      string     `json:"category"`
    Content       string     `gorm:"type:text" json:"content"`
    Frontmatter   string     `gorm:"type:text" json:"frontmatter"`
    Status        string     `gorm:"default:active" json:"status"`      // active | stale | archived
    Pinned        bool       `gorm:"default:false" json:"pinned"`       // [已有] Curator/Agent 不得修改
    UseCount      int        `gorm:"default:0" json:"useCount"`
    ViewCount     int        `gorm:"default:0" json:"viewCount"`
    PatchCount    int        `gorm:"default:0" json:"patchCount"`
    LastUsedAt    *time.Time `json:"lastUsedAt"`
    LastViewedAt  *time.Time `json:"lastViewedAt"`
    LastPatchedAt *time.Time `json:"lastPatchedAt"`
    CreatedBy     string     `gorm:"default:agent" json:"createdBy"`    // "agent" | "user"
    WriteOrigin   string     `gorm:"default:foreground" json:"writeOrigin"` // [新增] foreground | background_review | curator [C:USER]
    CreatedAt     time.Time  `json:"createdAt"`
    UpdatedAt     time.Time  `json:"updatedAt"`
}
```

### 1.2 SkillProvenanceLog 新模型

文件: `backend/internal/models/skill_provenance_log.go` [C:USER]

```go
type SkillProvenanceLog struct {
    ID          int       `gorm:"primaryKey;autoIncrement" json:"id"`
    SkillID     int       `gorm:"index;not null" json:"skillId"`
    WorkspaceID int       `gorm:"index;not null" json:"workspaceId"`
    Action      string    `gorm:"not null" json:"action"`           // create | update | patch | delete | write_file | remove_file
    WriteOrigin string    `gorm:"not null" json:"writeOrigin"`      // foreground | background_review | curator
    ActorType   string    `gorm:"not null" json:"actorType"`        // agent | user | system
    ActorID     string    `json:"actorId"`                          // user ID or empty string
    Content     string    `gorm:"type:text" json:"content"`         // [C:USER] 完整内容快照
    FilePath    string    `json:"filePath"`                         // for file ops, empty for skill body
    CreatedAt   time.Time `json:"createdAt"`
}

func (SkillProvenanceLog) TableName() string { return "skill_provenance_logs" }
```

### 1.3 AgentSkillManager 接口扩展

文件: `backend/internal/services/agent_skill_service.go` [C:USER]

```go
type AgentSkillManager interface {
    // ... 现有方法 ...
    IsPinned(ctx context.Context, workspaceID int, skillSlug string) (bool, error) // [新增]
}
```

### 1.4 ProvenanceRecorder 接口

文件: `backend/internal/services/provenance_service.go` [新增] [C:USER]

```go
type ProvenanceRecorder interface {
    Record(ctx context.Context, skill *models.AgentSkill, action, filePath string, beforeContent *string) error
}

type ProvenanceService struct {
    db *gorm.DB
}

func NewProvenanceService(db *gorm.DB) *ProvenanceService
func (s *ProvenanceService) Record(ctx context.Context, skill *models.AgentSkill, action, filePath string, beforeContent *string) error
```

### 1.5 BackupManager 接口

文件: `backend/internal/services/skill_backup_service.go` [新增] [C:USER]

```go
type SnapshotInfo struct {
    SnapshotID  string    `json:"snapshotId"`   // timestamp string, e.g. "20260611-143052"
    WorkspaceID int       `json:"workspaceId"`
    CreatedAt   time.Time `json:"createdAt"`
    SkillCount  int       `json:"skillCount"`
    FileCount   int       `json:"fileCount"`
}

type BackupManager interface {
    Snapshot(ctx context.Context, workspaceID int) (string, error)   // returns snapshotID
    Restore(ctx context.Context, workspaceID int, snapshotID string) error
    List(ctx context.Context, workspaceID int) ([]SnapshotInfo, error)
    Prune(ctx context.Context, workspaceID int, keep int) error
}

type BackupService struct {
    db         *gorm.DB
    storageDir string // <StorageDir>/skill-backups
}

func NewBackupService(db *gorm.DB, storageDir string) *BackupService
func (s *BackupService) Snapshot(ctx context.Context, workspaceID int) (string, error)
func (s *BackupService) Restore(ctx context.Context, workspaceID int, snapshotID string) error
func (s *BackupService) List(ctx context.Context, workspaceID int) ([]SnapshotInfo, error)
func (s *BackupService) Prune(ctx context.Context, workspaceID int, keep int) error
```

---

## Part 2 — Algorithms & Call-sites

### 2.1 BackupService.Snapshot

```
func Snapshot(ctx, workspaceID int) -> (string, error):
    1. skills ← skillSvc.List(ctx, workspaceID, true) // include archived
    2. totalFiles ← 0
    3. for each skill in skills:
         files ← skillSvc.ListFiles(ctx, skill.ID)
         skill.Files = files
         totalFiles += len(files)
    4. snapshot ← {
         WorkspaceID: workspaceID,
         Timestamp:   time.Now().UTC(),
         Skills:      skills,
       }
    5. dir ← filepath.Join(storageDir, "skill-backups", strconv.Itoa(workspaceID))
    6. os.MkdirAll(dir, 0750)
    7. filename ← time.Now().UTC().Format("20060102-150405") + ".json"
    8. data ← json.Marshal(snapshot)
    9. os.WriteFile(filepath.Join(dir, filename), data, 0640)
    10. Prune(workspaceID, keep=10)
    11. return filename[:len(filename)-5], nil // snapshotID = "20060102-150405"
```

### 2.2 BackupService.Restore

```
func Restore(ctx, workspaceID int, snapshotID string) -> error:
    1. path ← filepath.Join(storageDir, "skill-backups", strconv.Itoa(workspaceID), snapshotID+".json")
    2. if not os.Exists(path): return ErrSnapshotNotFound
    3. data ← os.ReadFile(path)
    4. json.Unmarshal(data, &snapshot)
    5. tx ← db.Begin()
    6. tx.Where("workspace_id = ?", workspaceID).Delete(AgentSkillFile{})
    7. tx.Where("workspace_id = ?", workspaceID).Delete(AgentSkill{})
    8. for each skill in snapshot.Skills:
         skill.ID = 0 // reset PK for insert
         tx.Create(&skill)
         for each file in skill.Files:
             file.ID = 0
             file.SkillID = skill.ID
             tx.Create(&file)
    9. tx.Commit()
    10. return nil
```

### 2.3 ProvenanceService.Record

```
func Record(ctx, skill *AgentSkill, action, filePath string, beforeContent *string) -> error:
    1. actorType, actorID ← inferActor(ctx)
       // [C:INFERRED] 从 gin.Context 读取 "user"（handler 路径）
       // 或从 tool context 推断 "agent"（Agent 工具路径）
    2. log ← SkillProvenanceLog{
         SkillID:     skill.ID,
         WorkspaceID: skill.WorkspaceID,
         Action:      action,
         WriteOrigin: skill.WriteOrigin,
         ActorType:   actorType,
         ActorID:     actorID,
         Content:     skill.Content, // [C:USER] 完整快照
         FilePath:    filePath,
         CreatedAt:   time.Now(),
       }
    3. return db.WithContext(ctx).Create(&log).Error
       // [C:INFERRED] 失败时由调用方 mlog.Warn，不阻断主操作
```

### 2.4 Curator Worker 集成

文件: `backend/internal/workers/skill_curator.go:32`

```go
func (j *SkillCuratorJob) Run(ctx context.Context) error {
    staleDays, archiveDays := readSettings()
    
    // [C:INFERRED] 遍历所有 workspace
    var workspaces []models.Workspace
    j.db.WithContext(ctx).Find(&workspaces)
    
    for _, ws := range workspaces {
        // Backup 先执行，失败阻断
        snapshotID, err := j.backupSvc.Snapshot(ctx, ws.ID)
        if err != nil {
            mlog.Error("backup failed ws=%d, aborting curator", ws.ID)
            return fmt.Errorf("backup failed for workspace %d: %w", ws.ID, err)
        }
        mlog.Info("curator backup complete", mlog.Int("workspace", ws.ID), mlog.String("snapshot", snapshotID))
        
        counts, err := j.skillSvc.ApplyCuratorTransitions(ctx, staleDays, archiveDays)
        // ApplyCuratorTransitions 内部已检查 Pinned
        // ...
    }
    return nil
}
```

### 2.5 AgentSkillService.ApplyCuratorTransitions（Pin 检查）

文件: `backend/internal/services/agent_skill_service.go:594`

```go
// 在循环内部，line ~616:
for _, skill := range batch {
    counts["checked"]++
    if skill.Pinned {
        continue // [C:USER] Pinned 技能跳过所有自动转换
    }
    // ... 原有状态转换逻辑
}
```

### 2.6 Agent Tools Pin 检查

文件: `backend/internal/agent/tools/agent_skills.go`

在每个破坏性操作前插入 Pin 检查：

```go
// skillManageDelete (line ~197)
func skillManageDelete(...):
    skill, err := skillSvc.GetBySlug(ctx, wsID, slugifyForLookup(args.Name))
    if err != nil { return tool.Error(...) }
    if skill.Pinned {
        return tool.Error(fmt.Sprintf("Skill '%s' is pinned and cannot be deleted. Unpin it first.", args.Name)), nil
    }
    // ... 继续删除逻辑
    _ = provenanceSvc.Record(ctx, skill, "delete", "", &skill.Content)

// skillManageEdit (line ~114)
func skillManageEdit(...):
    skill, err := skillSvc.GetBySlug(ctx, wsID, slugifyForLookup(args.Name))
    if err != nil { return tool.Error(...) }
    if skill.Pinned {
        return tool.Error(fmt.Sprintf("Skill '%s' is pinned and cannot be edited. Unpin it first.", args.Name)), nil
    }
    beforeContent := skill.Content
    // ... 继续编辑逻辑
    _ = provenanceSvc.Record(ctx, skill, "edit", "", &beforeContent)

// skillManagePatch (line ~155)
func skillManagePatch(...):
    skill, err := skillSvc.GetBySlug(ctx, wsID, slugifyForLookup(args.Name))
    if err != nil { return tool.Error(...) }
    if skill.Pinned {
        return tool.Error(fmt.Sprintf("Skill '%s' is pinned and cannot be patched. Unpin it first.", args.Name)), nil
    }
    beforeContent := skill.Content
    // ... 继续 patch 逻辑
    _ = provenanceSvc.Record(ctx, skill, "patch", args.FilePath, &beforeContent)

// skillManageWriteFile (line ~223)
func skillManageWriteFile(...):
    skill, err := skillSvc.GetBySlug(ctx, wsID, slugifyForLookup(args.Name))
    if err != nil { return tool.Error(...) }
    if skill.Pinned {
        return tool.Error(fmt.Sprintf("Skill '%s' is pinned and files cannot be modified. Unpin it first.", args.Name)), nil
    }
    // ... 继续写文件逻辑
    _ = provenanceSvc.Record(ctx, skill, "write_file", args.FilePath, nil)

// skillManageRemoveFile (line ~254)
func skillManageRemoveFile(...):
    skill, err := skillSvc.GetBySlug(ctx, wsID, slugifyForLookup(args.Name))
    if err != nil { return tool.Error(...) }
    if skill.Pinned {
        return tool.Error(fmt.Sprintf("Skill '%s' is pinned and files cannot be removed. Unpin it first.", args.Name)), nil
    }
    // ... 继续删文件逻辑
    _ = provenanceSvc.Record(ctx, skill, "remove_file", args.FilePath, nil)
```

### 2.7 AgentSkillService.Create（WriteOrigin 设置）

文件: `backend/internal/services/agent_skill_service.go:184`

```go
skill := models.AgentSkill{
    // ...
    WriteOrigin: req.WriteOrigin, // [C:USER] 调用方传入，默认 "foreground"
    // ...
}
```

### 2.8 DTO 扩展

文件: `backend/internal/dto/agent_skill.go` [新增字段]

```go
type CreateAgentSkillRequest struct {
    // ... 现有字段 ...
    WriteOrigin string `json:"writeOrigin"` // [新增] 可选，默认 "foreground"
}

type UpdateAgentSkillRequest struct {
    // ... 现有字段 ...
    WriteOrigin string `json:"writeOrigin"` // [新增] 可选
}
```

### 2.9 调用点汇总表

| # | 文件路径 | 行号 | 变更内容 | 来源 |
|---|---------|------|---------|------|
| 1 | `models/agent_skill.go` | ~38 | 新增 `WriteOrigin string` | [C:USER] |
| 2 | `models/skill_provenance_log.go` | [新文件] | 新建 `SkillProvenanceLog` 模型 | [C:USER] |
| 3 | `services/agent_skill_service.go` | ~594 | `ApplyCuratorTransitions` 加 `if skill.Pinned { continue }` | [C:USER] |
| 4 | `services/agent_skill_service.go` | ~184 | `Create` 设置 `WriteOrigin` | [C:USER] |
| 5 | `services/provenance_service.go` | [新文件] | 新建 `ProvenanceService` | [C:USER] |
| 6 | `services/skill_backup_service.go` | [新文件] | 新建 `BackupService` | [C:USER] |
| 7 | `agent/tools/agent_skills.go` | ~197 | `skillManageDelete` 加 Pin 检查 + Provenance | [C:USER] |
| 8 | `agent/tools/agent_skills.go` | ~114 | `skillManageEdit` 加 Pin 检查 + Provenance | [C:USER] |
| 9 | `agent/tools/agent_skills.go` | ~155 | `skillManagePatch` 加 Pin 检查 + Provenance | [C:USER] |
| 10 | `agent/tools/agent_skills.go` | ~223 | `skillManageWriteFile` 加 Pin 检查 + Provenance | [C:USER] |
| 11 | `agent/tools/agent_skills.go` | ~254 | `skillManageRemoveFile` 加 Pin 检查 + Provenance | [C:USER] |
| 12 | `workers/skill_curator.go` | ~32 | `Run()` 加 Backup 前置 + workspace 遍历 | [C:INFERRED] |
| 13 | `dto/agent_skill.go` | [新增] | `CreateAgentSkillRequest` 加 `WriteOrigin` | [C:USER] |

---

## Part 3 — Error Table & Test Plan

### 3.1 Error & Degradation Table

| # | 错误类 | 触发场景 | 立即处理 | 降级路径 | 恢复条件 |
|---|--------|---------|---------|---------|---------|
| 1 | Backup 失败 | Snapshot 时磁盘满/权限不足 | 阻断 Curator Run，return error | 跳过当前 workspace，后续 workspace 不执行 | 清理磁盘/修复权限后下次 cron 重试 |
| 2 | Pin 检查拒绝 | Agent 尝试删除/编辑/ patch Pinned 技能 | 返回 `tool.Error("pinned, unpin first")` | Agent 收到拒绝，可向用户解释 | 用户通过前端/API 取消 Pin |
| 3 | Provenance 记录失败 | 数据库连接中断/超时 | `mlog.Warn`，不阻断主操作 | 丢失单条审计记录 | 数据库恢复后后续操作正常记录 |
| 4 | Restore 失败 | 备份文件损坏/被删除 | 事务回滚，return error | 现有数据不受影响 | 使用其他 snapshot 重试 |
| 5 | Prune 失败 | 文件系统权限不足 | `mlog.Warn`，不阻断 Snapshot | 备份数量可能暂时超限 | 修复权限后下次 Prune 清理 |
| 6 | Curator 遍历 workspace 失败 | 数据库查询错误 | `mlog.Error`，return error | 整个 Curator run 失败 | 数据库恢复后下次 cron 重试 |

### 3.2 Test Plan

| # | 测试文件 | 测试名 | 断言 |
|---|---------|--------|------|
| 1 | `services/agent_skill_service_test.go` | `TestPinBlocksCuratorStale` | Pinned skill 闲置 30 天后仍为 `active` |
| 2 | `services/agent_skill_service_test.go` | `TestPinBlocksCuratorArchive` | Pinned skill 闲置 90 天后仍为 `active` |
| 3 | `agent/tools/agent_skills_test.go` | `TestPinBlocksAgentDelete` | `skill_manage delete` pinned skill 返回包含 "pinned" 的 error |
| 4 | `agent/tools/agent_skills_test.go` | `TestPinBlocksAgentEdit` | `skill_manage edit` pinned skill 返回包含 "pinned" 的 error |
| 5 | `agent/tools/agent_skills_test.go` | `TestPinBlocksAgentPatch` | `skill_manage patch` pinned skill 返回包含 "pinned" 的 error |
| 6 | `services/skill_backup_service_test.go` | `TestSnapshotCreatesFile` | Snapshot 后 `<StorageDir>/skill-backups/{wsID}/*.json` 存在且可解析 |
| 7 | `services/skill_backup_service_test.go` | `TestSnapshotPruneOld` | 创建 11 个 snapshot 后，最旧的 JSON 文件被删除 |
| 8 | `services/skill_backup_service_test.go` | `TestRestoreIntegrity` | Restore 后 skills + files 数量/内容与 snapshot 一致 |
| 9 | `services/skill_backup_service_test.go` | `TestRestoreInvalidSnapshot` | 不存在的 snapshotID 返回 `ErrSnapshotNotFound` |
| 10 | `services/provenance_service_test.go` | `TestRecordOnCreate` | Create skill 后 `skill_provenance_logs` 有 1 条记录，Content 与 skill.Content 一致 |
| 11 | `services/provenance_service_test.go` | `TestRecordOnPatch` | Patch skill 后 `skill_provenance_logs` 有对应 action="patch" 的记录 |
| 12 | `workers/skill_curator_test.go` | `TestBackupFailureBlocksCurator` | Backup 失败时 `CuratorJob.Run` 返回 error，不调用 ApplyCuratorTransitions |
| 13 | `workers/skill_curator_test.go` | `TestCuratorRespectsPinned` | 混合 pinned/unpinned skills，仅 unpinned 被转换状态 |

### 3.3 Done Criteria

```bash
# 所有新增测试通过
cd backend && go test -v ./internal/services/... ./internal/agent/... ./internal/workers/...

# 现有测试不回归
cd backend && go test -race -cover ./...

# Lint 通过
cd backend && golangci-lint run
```

---

## Assumptions & Unverified Items

| # | Assumption | Confidence | Impact if wrong | How to verify |
|---|-----------|------------|-----------------|---------------|
| 1 | `AgentSkill.Pinned` 字段已在模型中且 GORM AutoMigrate 会自动添加新字段 | High | 无影响，字段已存在 | 已验证 `models/agent_skill.go:28` [C:USER] |
| 2 | `config.Config.StorageDir` 存在且可写，路径格式与现有 `<StorageDir>/hermind-fs` 一致 | High | Backup 无法写入文件系统 | 已验证 `config.go:20` [C:INFERRED] |
| 3 | `SystemService.GetSetting/SetSetting` 支持字符串键值存储，Curator 配置沿用此模式 | High | Curator 配置读取失败 | 已验证 `system_service.go:21,36` [C:INFERRED] |
| 4 | Curator Worker 遍历所有 workspace 时，`db.Find(&workspaces)` 不会 OOM（workspace 数量可控） | Medium | 内存占用过高 | 当前 workspace 表通常 <1000 条 [C:INFERRED] |
| 5 | `inferActor(ctx)` 可从 gin.Context 读取 user ID，从 tool context 识别 agent 调用 | Medium | Provenance ActorType/ActorID 记录错误 | 实现时验证 context 传递链 [C:INFERRED] |
| 6 | 备份 JSON 文件体积可控：单个 workspace 的 skills + files 总数据量 < 10MB | Medium | 磁盘占用过高 | 观察生产环境数据量 [C:INFERRED] |
| 7 | `AgentSkillFile` 在 JSON 序列化时可被完整表示（无外键循环引用） | High | Backup/Restore 数据丢失 | Restore 测试验证 [C:INFERRED] |
| 8 | GORM AutoMigrate 会自动创建 `skill_provenance_logs` 表 | High | Provenance 记录失败 | 启动日志验证 [C:INFERRED] |

---

## Risk Register

| # | Risk | Likelihood | Impact | Mitigation |
|---|------|-----------|--------|-----------|
| 1 | Pin 保护过于严格，Agent 无法修复自己创建但已 Pinned 的技能 | Medium | Agent 自改进循环受阻 | Pin 仅保护手动 Pin 的技能；Agent 创建的技能默认不 Pin | [C:USER] |
| 2 | Provenance 日志表无限增长 | Medium | 磁盘/性能问题 | 后续 Batch 添加日志保留策略（如保留 90 天） | [C:DEFERRED] |
| 3 | Backup 文件权限 0640 在多用户模式下可能导致其他用户无法读取 | Low | 权限问题 | 使用 0640（owner+group），与现有 storage 目录权限一致 | [C:INFERRED] |
| 4 | Restore 误操作覆盖用户手动修改 | Medium | 数据丢失 | Restore 操作需要管理员权限 + 操作前自动 Backup | [C:INFERRED] |
| 5 | Curator Backup 失败导致所有 workspace 的维护停滞 | Low | 技能库维护完全停止 | 按 workspace 独立 Backup，失败仅跳过该 workspace | [C:INFERRED] |

---

## Self-Review

### Security
- 检查了 Backup 文件权限（0640）和目录权限（0750），与现有 storage 目录一致
- 检查了 Provenance 日志记录完整内容快照的风险 — 已确认这是用户明确要求 [C:USER]，后续可通过日志保留策略控制
- 检查了 Pin 检查是否覆盖所有破坏性操作 — 已覆盖 delete/edit/patch/write_file/remove_file
- **发现**: Agent 工具中的 `skill_manage create` 未加 Pin 检查（正确，创建新技能不需要 Pin）

### Test
- 每个核心行为都有 must-pass 和 must-reject 测试用例
- `TestPinBlocksCuratorStale` vs `TestCuratorRespectsPinned` 覆盖了边界情况
- `TestRestoreInvalidSnapshot` 验证了 must-reject路径
- **修复**: 原始测试计划中缺少 `TestPinBlocksAgentWriteFile` 和 `TestPinBlocksAgentRemoveFile`，已补充到表 3.2

### Ops
- Backup Snapshot 在每次 Curator run 时执行，按 workspace 遍历
- 保留 10 个快照的策略避免了磁盘无限增长
- **发现**: 多 workspace 场景下，N 个 workspace × 10 个快照 = 最多 10N 个备份文件。当前 workspace 数量可控，风险低
- **修复**: Prune 应在 Snapshot 之后立即执行，而非依赖后续调用的 side effect

### Integration
- 验证了所有设计依赖的数据源/字段/钩子实际存在于代码中：
  - `Pinned bool` ✅ (`models/agent_skill.go:28`)
  - `CreatedBy string` ✅ (`models/agent_skill.go:35`)
  - `StorageDir string` ✅ (`config.go:20`)
  - `SystemService.GetSetting` ✅ (`system_service.go:21`)
  - `SkillCuratorJob` 结构体 ✅ (`workers/skill_curator.go:14`)
  - `skill_manage` 工具 ✅ (`agent/tools/agent_skills.go`)
- 设计落地位置与用户最初命名的代码位置一致，无 silent retargeting

### Scope
- 本设计仍为一个连贯的基础设施增强（Pin + Provenance + Backup），共享同一数据模型和服务层
- 三个功能之间有明确依赖关系但实现上紧密耦合，适合单一设计文件
- 无 scope creep：LLM Review、Umbrella Building、前端 UI 均已明确排除到 Scope Out
