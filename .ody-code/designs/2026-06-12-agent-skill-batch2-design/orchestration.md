# Batch 2 — Part 3: Curator Integration + Algorithms + Tests

> 范围：Curator Worker 编排流程、`LLMReviewService`、`UmbrellaBuilder`、`CuratorReportService` 的实现，以及它们与 Batch 1/Part 2 的集成与测试。

---

## 1. Scope In/Out

### 1.1 In

| # | 功能 | 说明 | 来源 |
|---|------|------|------|
| 1 | **LLMReviewService 实现** | 读取 curator 配置、构造 prompt、调用 ReviewAgentRuntime、解析 ToolCalls 为 `CuratorReviewDecision`、估算成本 | [C:USER] |
| 2 | **UmbrellaBuilder 实现** | 接收 consolidate/archive 决策，创建 umbrella skill，归档被吸收 skill，设置 `absorbed_into` | [C:USER] |
| 3 | **CuratorReportService 实现** | 创建/完结 DB 记录，生成 `run.json` 与 `REPORT.md` | [C:USER] |
| 4 | **SkillCuratorJob 编排** | 备份 → deterministic transitions → LLM Review → Umbrella Building → 报告落盘；支持 dry-run 与 safety level | [C:USER] |
| 5 | **Safety Level 策略** | high 回滚快照、medium 保留已应用、low 忽略错误继续 | [C:USER] |
| 6 | **Curator 默认关闭** | `agent_skill_curator_enabled` 默认 `false`，手动开启后才执行 | [C:USER] |

### 1.2 Out

| # | 功能 | 延后理由 |
|---|------|---------|
| 1 | **前端 Curator UI** | 前端独立任务，Batch 3 |
| 2 | **LLM 驱动的 umbrella 内容合成** | MVP 使用确定性模板合并；后续可用 auxiliary 模型生成更优 umbrella 内容 |
| 3 | **多 workspace 并行** | 先串行处理每个 workspace |
| 4 | **自动首次运行** | 用户要求默认关闭，首次手动触发 |
| 5 | **GEPA 自进化** | 独立仓库，非运行时内置 |

---

## 2. Architecture & Data Flow

```
SkillCuratorJob.Run(ctx)
        │
        ├── readConfig() ──► enabled / dryRun / safety / provider / model / apiKey / maxIter / stale / archive
        ├── if !enabled: return
        │
        ├── for each workspace:
        │       │
        │       ├── BackupService.Snapshot(wsID) ──► snapshotID
        │       ├── CuratorReportService.Create(wsID, mode, safety, snapshotID) ──► report
        │       │
        │       ├── SkillSvc.ApplyCuratorTransitions(staleDays, archiveDays)
        │       │
        │       ├── LLMReviewService.Review(wsID, opts)
        │       │       ├─ buildReviewPrompt(ws)
        │       │       ├─ ReviewAgentRuntime.RunReview(...)
        │       │       ├─ parseFinalResponse()
        │       │       ├─ parseToolCalls() ──► []CuratorReviewDecision
        │       │       └─ estimateCost() ──► costUSD
        │       │
        │       ├── UmbrellaBuilder.Build(wsID, decisions, dryRun)
        │       │       ├─ group archive decisions by target umbrella
        │       │       ├─ create umbrella skill (if missing)
        │       │       └─ archive absorbed skills + set absorbed_into
        │       │
        │       ├── CuratorReportService.Finalize(report, result, err)
        │       └── CuratorReportService.SaveFiles(report)
        │
        └── return nil / first error (logged, not propagated if partial)
```

数据变化：
- **Deterministic transitions**：`active → stale → archived`（Batch 1 已存在）。
- **LLM Review**：dry-run 不修改 DB；live 通过 `skill_manage` 直接修改 skill。
- **Umbrella Building**：创建 umbrella skill（`WriteOrigin="curator"`），将被吸收 skill 标记 `archived` 并写入 `absorbed_into`。
- **Report**：`CuratorReport` 表记录元数据；`<StorageDir>/curator-reports/<wsID>/<runID>/` 下保存 `run.json` + `REPORT.md`。

---

## 3. Interfaces & Types

### 3.1 复用接口（定义见 `core.md`）

- `LLMReviewService` / `ReviewOptions` / `ReviewResult`：见 `core.md` §2.1
- `UmbrellaBuilder`：见 `core.md` §2.2
- `CuratorReportService`：见 `core.md` §2.3
- `ReviewAgentRuntime` / `ReviewAgentResult` / `ReviewAgentToolCall`：见 `core.md` §2.4

### 3.2 LLMReviewService 实现

文件：`backend/internal/services/llm_review_service.go` [新增] [C:USER]

```go
type llmReviewService struct {
    db       *gorm.DB
    skillSvc services.AgentSkillManager
    backupSvc services.BackupManager
    runtime   agent.ReviewAgentRuntime
    provenanceSvc services.ProvenanceRecorder
}

// NewLLMReviewService 构造 LLMReviewService。
func NewLLMReviewService(
    db *gorm.DB,
    skillSvc services.AgentSkillManager,
    backupSvc services.BackupManager,
    runtime agent.ReviewAgentRuntime,
    provenanceSvc services.ProvenanceRecorder,
) LLMReviewService
```

### 3.3 UmbrellaBuilder 实现

文件：`backend/internal/services/umbrella_builder.go` [新增] [C:USER]

```go
type umbrellaBuilder struct {
    db       *gorm.DB
    skillSvc services.AgentSkillManager
}

// NewUmbrellaBuilder 构造 UmbrellaBuilder。
func NewUmbrellaBuilder(db *gorm.DB, skillSvc services.AgentSkillManager) UmbrellaBuilder
```

### 3.4 CuratorReportService 实现

文件：`backend/internal/services/curator_report_service.go` [新增] [C:USER]

```go
type curatorReportService struct {
    db         *gorm.DB
    storageDir string
}

// NewCuratorReportService 构造 CuratorReportService。
func NewCuratorReportService(db *gorm.DB, storageDir string) CuratorReportService
```

### 3.5 SkillCuratorJob 扩展

文件：`backend/internal/workers/skill_curator.go` [扩展] [C:USER]

```go
type SkillCuratorJob struct {
    db            *gorm.DB
    skillSvc      services.AgentSkillManager
    sysSvc        *services.SystemService
    backupSvc     services.BackupManager
    llmReviewSvc  services.LLMReviewService
    umbrellaBuilder services.UmbrellaBuilder
    reportSvc     services.CuratorReportService
}

// NewSkillCuratorJobWithReview 完整构造函数。
func NewSkillCuratorJobWithReview(
    db *gorm.DB,
    skillSvc services.AgentSkillManager,
    sysSvc *services.SystemService,
    backupSvc services.BackupManager,
    llmReviewSvc services.LLMReviewService,
    umbrellaBuilder services.UmbrellaBuilder,
    reportSvc services.CuratorReportService,
) *SkillCuratorJob
```

### 3.6 数据模型补充

由于 umbrella 需要持久化“被吸收到哪个 umbrella”，`AgentSkill` 模型需新增字段 [C:USER]：

```go
// backend/internal/models/agent_skill.go
 type AgentSkill struct {
     ...
     AbsorbedInto string `json:"absorbedInto"` // 仅当 status=archived 且被合并时填充
 }
```

并在 `dto.UpdateAgentSkillRequest` 中同步增加 `AbsorbedInto string` 以便通过服务层更新 [C:INFERRED]。

---

## 4. Algorithms

### 4.1 `LLMReviewService.Review`

```
function Review(ctx, workspaceID, opts):
    ws := db.First(&Workspace{}, workspaceID)
    if ws not found: return error

    skills := skillSvc.List(ctx, workspaceID, false)  // active + stale, not archived
    candidates := filter(skills, s -> s.CreatedBy == "agent" && !s.Pinned && s.Status != "archived")
    summary := buildCandidateSummary(candidates)
    prompt := buildReviewPrompt(ws, summary)

    settings := map[string]string{
        "curator_llm_provider":   opts.ModelProvider,
        "curator_llm_model":      opts.ModelID,
        "curator_llm_api_key":    opts.APIKey,
        "curator_dry_run":        strconv.FormatBool(opts.DryRun),
        "curator_max_iterations": strconv.Itoa(opts.MaxIterations),
        "curator_safety_level":   opts.SafetyLevel,
    }

    agentResult, err := runtime.RunReview(ctx, workspaceID, prompt, settings)
    if err != nil:
        return nil, err

    reasoning, incomplete := parseFinalResponse(agentResult.FinalResponse)
    decisions := parseToolCalls(agentResult.ToolCalls, opts.DryRun)
    cost := estimateCost(opts.ModelProvider, opts.ModelID, agentResult.TokenUsage)

    if incomplete:
        reasoning = "[INCOMPLETE] " + reasoning

    return &ReviewResult{
        Decisions:  decisions,
        Reasoning:  reasoning,
        TokenUsage: agentResult.TokenUsage,
        CostUSD:    cost,
    }, nil
```

### 4.2 `buildCandidateSummary`

```
function buildCandidateSummary(skills):
    lines := []
    for s in skills:
        lines.append(fmt.Sprintf("- %s (category=%s, status=%s, description=%s)",
            s.Name, s.Category, s.Status, s.Description))
    return strings.Join(lines, "\n")
```

### 4.3 `parseFinalResponse`

```
function parseFinalResponse(raw):
    raw = strings.TrimSpace(raw)
    type finalJSON struct {
        Reasoning      string `json:"reasoning"`
        ReviewComplete bool   `json:"review_complete"`
    }
    var out finalJSON
    if json.Unmarshal([]byte(raw), &out) == nil:
        return out.Reasoning, out.ReviewComplete
    // 回退：把原始文本当 reasoning，视为 incomplete
    return raw, false
```

### 4.4 `parseToolCalls`

```
function parseToolCalls(toolCalls, dryRun):
    decisions := []
    for call in toolCalls:
        if call.ToolName != "skill_manage": continue

        args := parseSkillManageArgs(call.Args)
        if args == nil: continue

        action := ""
        target := ""
        switch args.Action:
        case "create":
            action = "consolidate"      // review agent 只能为 umbrella 调用 create
            target = args.Name
        case "patch", "edit", "write_file", "remove_file":
            action = "patch"
        case "delete":
            action = "archive"
            target = args.AbsorbedInto
        default:
            continue
        end switch

        decisions.append(models.CuratorReviewDecision{
            SkillName:   args.Name,
            SkillSlug:   slugifyForLookup(args.Name),
            Action:      action,
            TargetSkill: target,
            TargetSlug:  slugifyForLookup(target),
            Reason:      args.Reason,
            Applied:     !dryRun,
            DryRun:      dryRun,
        })
    return decisions
```

### 4.5 `estimateCost`

```
// 每 1M tokens 价格（美元），按 input+output 综合估算
var pricingMap = map[string]float64{
    "openai/gpt-4o-mini": 0.30,
    "openai/gpt-4o":      5.00,
    "anthropic/claude-3-haiku": 0.50,  // 示例占位
    "ollama/default":     0.00,
}

function estimateCost(provider, model, tokenUsage):
    if tokenUsage <= 0: return 0
    key := provider + "/" + model
    price := pricingMap[key]
    if price == 0:
        price = pricingMap[provider + "/default"]
    if price == 0:
        price = pricingMap["openai/gpt-4o-mini"]  // 保守 fallback
    return float64(tokenUsage) * price / 1_000_000.0
```

### 4.6 `UmbrellaBuilder.Build`

```
function Build(ctx, workspaceID, decisions, dryRun):
    groups := map[string][]CuratorReviewDecision{}
    for d in decisions:
        if d.Action == "archive" and d.TargetSkill != "":
            groups[d.TargetSkill] = append(groups[d.TargetSkill], d)

    for umbrellaName, absorbed := groups:
        umbrellaSlug := slugifyForLookup(umbrellaName)
        umbrella, err := skillSvc.GetBySlug(ctx, workspaceID, umbrellaSlug)

        if err != nil: // umbrella 不存在
            if dryRun:
                // 仅验证：记录一个未应用的 consolidate 决策
                appendDecision(decisions, consolidateDecision(umbrellaName, absorbed, false))
                continue
            // 创建 umbrella skill
            content := buildUmbrellaContent(umbrellaName, absorbed)
            frontmatter := fmt.Sprintf("name: %s\ndescription: Umbrella skill consolidating %d skills",
                umbrellaName, len(absorbed))
            skillSvc.Create(ctx, workspaceID, dto.CreateAgentSkillRequest{
                Name:        umbrellaName,
                Category:    absorbed[0].Category,  // 取首个 category，或为空
                Content:     content,
                Frontmatter: frontmatter,
                CreatedBy:   models.AgentSkillCreatedByAgent,
                WriteOrigin: "curator",
            })
            appendDecision(decisions, consolidateDecision(umbrellaName, absorbed, true))
        else if !dryRun:
            // umbrella 已存在：可选追加内容（MVP 跳过，仅记录）
            appendDecision(decisions, consolidateDecision(umbrellaName, absorbed, true))
        else:
            appendDecision(decisions, consolidateDecision(umbrellaName, absorbed, false))

        // 归档被吸收的技能
        for d in absorbed:
            if dryRun:
                d.Applied = false
                continue
            skill, err := skillSvc.GetBySlug(ctx, workspaceID, d.SkillSlug)
            if err != nil or skill.Pinned: continue
            skillSvc.Update(ctx, workspaceID, d.SkillSlug, dto.UpdateAgentSkillRequest{
                Status:       models.AgentSkillStatusArchived,
                AbsorbedInto: umbrellaName,
            })
            d.Applied = true

    return decisions, nil
```

### 4.7 `buildUmbrellaContent`

```
function buildUmbrellaContent(umbrellaName, absorbedDecisions):
    parts := []
    parts.append(fmt.Sprintf("# %s\n\nThis umbrella skill consolidates the following skills:\n", umbrellaName))
    for d in absorbedDecisions:
        parts.append(fmt.Sprintf("- %s: %s", d.SkillName, d.Reason))
    parts.append("\n## Merged guidance\n\n")
    for d in absorbedDecisions:
        parts.append(fmt.Sprintf("### From %s\n%s\n", d.SkillName, d.Reason))
    return strings.Join(parts, "\n")
```

### 4.8 `CuratorReportService.Create`

```
function Create(ctx, workspaceID, mode, safetyLevel, snapshotID):
    runID := time.Now().UTC().Format("20060102-150405")
    report := &models.CuratorReport{
        WorkspaceID: workspaceID,
        RunID:       runID,
        Mode:        mode,
        Status:      "running",
        SafetyLevel: safetyLevel,
        SnapshotID:  snapshotID,
        StartedAt:   time.Now().UTC(),
    }
    if err := db.WithContext(ctx).Create(report).Error; err != nil:
        return nil, err
    return report, nil
```

### 4.9 `CuratorReportService.Finalize`

```
function Finalize(ctx, report, result, runErr):
    report.Status = "completed"
    report.ErrorMessage = ""

    if runErr != nil:
        if report.SafetyLevel == "high" or report.SafetyLevel == "medium":
            report.Status = "failed"
        report.ErrorMessage = runErr.Error()

    if result != nil:
        report.Decisions = result.Decisions
        report.DecisionsJSON = marshalJSON(result.Decisions)
        report.Reasoning = result.Reasoning
        report.TokenUsage = result.TokenUsage
        report.CostUSD = result.CostUSD
        report.Summary = generateSummary(report)
        if strings.HasPrefix(result.Reasoning, "[INCOMPLETE]"):
            report.Summary = "Incomplete review. " + report.Summary

    report.CompletedAt = ptr(time.Now().UTC())
    return db.WithContext(ctx).Save(report).Error
```

### 4.10 `CuratorReportService.SaveFiles`

```
function SaveFiles(ctx, report):
    dir := filepath.Join(storageDir, "curator-reports",
                         strconv.Itoa(report.WorkspaceID), report.RunID)
    os.MkdirAll(dir, 0750)

    runJSON := map[string]any{
        "run_id":        report.RunID,
        "workspace_id":  report.WorkspaceID,
        "mode":          report.Mode,
        "status":        report.Status,
        "safety_level":  report.SafetyLevel,
        "model_provider": report.ModelProvider,
        "model_id":      report.ModelID,
        "snapshot_id":   report.SnapshotID,
        "started_at":    report.StartedAt,
        "completed_at":  report.CompletedAt,
        "token_usage":   report.TokenUsage,
        "cost_usd":      report.CostUSD,
        "summary":       report.Summary,
        "reasoning":     report.Reasoning,
        "decisions":     report.Decisions,
        "error":         report.ErrorMessage,
    }
    writeJSON(filepath.Join(dir, "run.json"), runJSON)
    writeFile(filepath.Join(dir, "REPORT.md"), buildReportMarkdown(report))
    return nil
```

### 4.11 `buildReportMarkdown`

```
function buildReportMarkdown(report):
    md := fmt.Sprintf("# Curator Report — %s\n\n", report.RunID)
    md += fmt.Sprintf("- Workspace: %d\n", report.WorkspaceID)
    md += fmt.Sprintf("- Mode: %s\n", report.Mode)
    md += fmt.Sprintf("- Safety Level: %s\n", report.SafetyLevel)
    md += fmt.Sprintf("- Model: %s / %s\n", report.ModelProvider, report.ModelID)
    md += fmt.Sprintf("- Token Usage: %d (est. $%.6f)\n\n", report.TokenUsage, report.CostUSD)
    md += "## Summary\n\n" + report.Summary + "\n\n"
    md += "## Reasoning\n\n" + report.Reasoning + "\n\n"
    md += "## Decisions\n\n| Skill | Action | Target | Reason | Applied |\n|---|---|---|---|---|\n"
    for d in report.Decisions:
        md += fmt.Sprintf("| %s | %s | %s | %s | %v |\n",
            d.SkillName, d.Action, d.TargetSkill, d.Reason, d.Applied)
    if report.ErrorMessage != "":
        md += "\n## Error\n\n" + report.ErrorMessage + "\n"
    return md
```

### 4.12 `SkillCuratorJob.Run`

```
function Run(ctx):
    staleDays, archiveDays := readLifecycleSettings(ctx, sysSvc)
    dryRun := sysSvc.GetSetting(ctx, "agent_skill_curator_dry_run") == "true"
    safety := sysSvc.GetSetting(ctx, "curator_safety_level")
    if safety == "": safety = "medium"

    provider := sysSvc.GetSetting(ctx, "curator_llm_provider")
    model := sysSvc.GetSetting(ctx, "curator_llm_model")
    apiKey := sysSvc.GetSetting(ctx, "curator_llm_api_key")
    maxIter := parseInt(sysSvc.GetSetting(ctx, "curator_max_iterations"), 8)

    mode := "live"
    if dryRun: mode = "dry_run"

    workspaces := []models.Workspace{}
    db.WithContext(ctx).Find(&workspaces)

    for ws in workspaces:
        snapshotID, err := backupSvc.Snapshot(ctx, ws.ID)
        if err != nil:
            logError("curator snapshot failed", ws.ID, err)
            if safety == "high": return err
            continue

        report, _ := reportSvc.Create(ctx, ws.ID, mode, safety, snapshotID)
        var reviewResult *ReviewResult
        var runErr error

        // 1. deterministic transitions
        _, runErr = skillSvc.ApplyCuratorTransitions(ctx, staleDays, archiveDays)
        if runErr != nil and safety == "high":
            _ = reportSvc.Finalize(ctx, report, nil, runErr)
            _ = reportSvc.SaveFiles(ctx, report)
            restoreSnapshot(ctx, backupSvc, ws.ID, snapshotID, dryRun)
            continue

        // 2. LLM review
        if runErr == nil:
            opts := ReviewOptions{
                MaxIterations: maxIter,
                DryRun:        dryRun,
                SafetyLevel:   safety,
                ModelProvider: provider,
                ModelID:       model,
                APIKey:        apiKey,
            }
            reviewResult, runErr = llmReviewSvc.Review(ctx, ws.ID, opts)
            if runErr != nil and safety == "high":
                _ = reportSvc.Finalize(ctx, report, reviewResult, runErr)
                _ = reportSvc.SaveFiles(ctx, report)
                restoreSnapshot(ctx, backupSvc, ws.ID, snapshotID, dryRun)
                continue
            }
        }

        // 3. Umbrella building
        if runErr == nil and reviewResult != nil:
            finalDecisions, ubErr := umbrellaBuilder.Build(ctx, ws.ID, reviewResult.Decisions, dryRun)
            if ubErr != nil:
                runErr = ubErr
            else:
                reviewResult.Decisions = finalDecisions
            if runErr != nil and safety == "high":
                _ = reportSvc.Finalize(ctx, report, reviewResult, runErr)
                _ = reportSvc.SaveFiles(ctx, report)
                restoreSnapshot(ctx, backupSvc, ws.ID, snapshotID, dryRun)
                continue
            }
        }

        // 4. Finalize & save files
        _ = reportSvc.Finalize(ctx, report, reviewResult, runErr)
        _ = reportSvc.SaveFiles(ctx, report)

    return nil
```

### 4.13 `restoreSnapshot`

```
function restoreSnapshot(ctx, backupSvc, workspaceID, snapshotID, dryRun):
    if dryRun: return  // dry-run 没有真实修改，无需回滚
    if backupSvc == nil or snapshotID == "": return
    if err := backupSvc.Restore(ctx, workspaceID, snapshotID); err != nil:
        logError("curator rollback failed", workspaceID, err)
```

### 4.14 `SkillCuratorJob.Enabled`

```
function Enabled(ctx):
    v, _ := sysSvc.GetSetting(ctx, "agent_skill_curator_enabled")
    return v == "true"   // 默认 false，用户手动开启
```

---

## 5. Call-Site Integration

### 5.1 新增文件：`backend/internal/services/llm_review_service.go`

实现 `LLMReviewService`。

### 5.2 新增文件：`backend/internal/services/umbrella_builder.go`

实现 `UmbrellaBuilder`。

### 5.3 新增文件：`backend/internal/services/curator_report_service.go`

实现 `CuratorReportService`。

### 5.4 扩展文件：`backend/internal/workers/skill_curator.go`

替换现有构造函数调用。`NewSkillCuratorJob` 仍可保留用于向后兼容（无 review 能力），主入口使用 `NewSkillCuratorJobWithReview`。

### 5.5 扩展文件：`backend/internal/models/agent_skill.go` 与 `backend/internal/dto/agent_skill.go`

新增 `AbsorbedInto string` 字段与对应 DTO 字段（见 §3.6）。

### 5.6 初始化：`backend/cmd/server/main.go`

```go
llmReviewSvc := services.NewLLMReviewService(db, agentSkillSvc, backupSvc, reviewRuntime, provenanceSvc)
umbrellaBuilder := services.NewUmbrellaBuilder(db, agentSkillSvc)
curatorReportSvc := services.NewCuratorReportService(db, cfg.StorageDir)

curatorJob := workers.NewSkillCuratorJobWithReview(
    db, agentSkillSvc, sysSvc, backupSvc, llmReviewSvc, umbrellaBuilder, curatorReportSvc,
)
workerManager.Register(curatorJob)
```

### 5.7 初始化：GORM AutoMigrate

`models.CuratorReport` 已加入 AutoMigrate（Batch 1/Part 1）。新增的 `AgentSkill.AbsorbedInto` 由 GORM 自动迁移。

---

## 6. Error Handling & Degradation

| 错误类别 | 即时处理 | 降级路径 | 恢复条件 |
|---|---|---|---|
| Curator 未启用 | `Enabled()` 返回 false，跳过 | 无 | 用户设置 `agent_skill_curator_enabled=true` |
| 备份失败 | high: 返回错误；medium/low: 跳过该 workspace | 继续处理下一个 workspace | 修复存储/权限 |
| Deterministic transitions 失败 | high: 回滚快照并跳过；其余记录错误继续 | 后续 LLM review 仍可运行 | 修复 DB 问题 |
| LLM Review 失败/超时 | high: 回滚快照；medium: 记录错误，保留 transitions；low: 忽略 | 下次 cron 重试 | 检查模型配置/网络 |
| Review 输出 incomplete | 标记 summary `[INCOMPLETE]`，保留已执行决策 | high 视为失败并回滚 | 增大 max_iterations 或优化 prompt |
| Umbrella 构建失败 | high: 回滚快照；medium/low: 记录错误，保留 review 决策 | 手动修复冲突 skill 名 | 重试 |
| 单个决策失败（如 skill 被 Pin） | 跳过该 skill，继续后续 | 报告中标记未应用 | 用户 unpin |
| 上下文取消 | 中止当前 workspace，保留已保存 report | 其余 workspace 由 cron 下次处理 | 重试 |

---

## 7. Test Plan

新增/修改测试文件：

- `backend/internal/services/llm_review_service_test.go`
- `backend/internal/services/umbrella_builder_test.go`
- `backend/internal/services/curator_report_service_test.go`
- `backend/internal/workers/skill_curator_test.go`（扩展）

| # | 用例 | 断言 |
|---|------|------|
| 1 | `TestLLMReviewService_ParseDecisions` | mock runtime 返回 2 个 tool calls（patch + delete absorbed）；断言 decisions 长度=2，action 分别为 patch/archive，Applied 与 DryRun 一致 |
| 2 | `TestLLMReviewService_ParseFinalResponseJSON` | final response 为合法 JSON；断言 `Reasoning` 等于 JSON reasoning，且不以 `[INCOMPLETE]` 开头 |
| 3 | `TestLLMReviewService_ParseFinalResponseRaw` | final response 非 JSON；断言 `Reasoning` 等于 raw，且前缀为 `[INCOMPLETE]` |
| 4 | `TestLLMReviewService_EstimateCost` | provider/model=openai/gpt-4o-mini，tokenUsage=1_000_000；断言 CostUSD ≈ 0.30 |
| 5 | `TestUmbrellaBuilder_CreatesUmbrella` | 输入 2 个 archive 决策（target=umbrella）且无 umbrella；断言 DB 中新增 umbrella skill，被吸收 skill status=archived 且 absorbed_into=umbrella |
| 6 | `TestUmbrellaBuilder_DryRunNoWrite` | dryRun=true；断言无新增 skill，被吸收 skill 仍 active，返回决策 Applied=false |
| 7 | `TestUmbrellaBuilder_PinProtection` | 被吸收 skill pinned=true；断言 UmbrellaBuilder 跳过该 skill，不修改其状态 |
| 8 | `TestCuratorReportService_SaveFiles` | 创建 report 并 Finalize；断言 `<storageDir>/curator-reports/<wsID>/<runID>/run.json` 与 `REPORT.md` 存在，且 REPORT.md 包含 decisions 表格 |
| 9 | `TestCuratorReportService_List` | 创建 3 条 report；断言 List(wsID, 2) 返回 2 条，按时间倒序 |
| 10 | `TestSkillCuratorJob_DefaultDisabled` | 不设置 enabled；断言 `Enabled()==false`，Run 不调用 LLMReviewSvc |
| 11 | `TestSkillCuratorJob_DryRunProducesReport` | enabled=true + dry_run=true；断言 report.Status=completed，Mode=dry_run，无 skill 被真实修改 |
| 12 | `TestSkillCuratorJob_HighSafetyRollback` | enabled=true + safety=high，mock LLMReview 返回 error；断言 BackupService.Restore 被调用一次 |
| 13 | `TestSkillCuratorJob_MediumSafetyKeepsPartial` | enabled=true + safety=medium，UmbrellaBuilder 第二个决策失败；断言 report.Status=failed 或 completed_with_errors，之前已应用决策未被回滚 |

**Done criteria**：

```bash
cd backend && go test -tags=fts5 ./internal/services/... ./internal/workers/...
```

---

## 8. Assumptions & Unverified Items

| # | 假设 | 置信度 | 错误影响 | 验证方式 |
|---|------|--------|----------|----------|
| 1 | `AgentSkill` 可以增加 `AbsorbedInto` 字段而不破坏现有查询/API | 高 | 迁移失败或 JSON 序列化异常 | GORM AutoMigrate + 现有测试 |
| 2 | `dto.UpdateAgentSkillRequest` 增加 `AbsorbedInto` 后，service Update 会将其写入 DB | 高 | umbrella 归档不记录吸收目标 | 检查 `AgentSkillService.Update` 实现 |
| 3 | `skillSvc.List(ctx, wsID, false)` 返回 active + stale（不含 archived） | 高 | Review 候选集错误包含已归档 skill | 已读 `agent_skill_service.go:63` |
| 4 | `BackupService.Restore` 能完整回滚 Review Agent 造成的 skill 变更 | 高 | high safety 回滚后仍有残留 | 已有 Batch 1 测试 |
| 5 | 将 `skill_manage(create)` 统一映射为 `consolidate` 不会误标普通新建 | 中 | report 中 action 分类错误 | prompt 限制 review agent 只在 umbrella 场景调用 create |
| 6 | 价格表覆盖主要 provider/model；缺失时 fallback 到 gpt-4o-mini | 中 | 成本估算偏高/偏低 | 维护 pricingMap；允许用户后续贡献 |
| 7 | `<StorageDir>/curator-reports` 可写且不会暴露到 web root | 高 | 报告文件泄露 | 使用 0750/0640；静态文件路由仅服务 frontend/dist |
| 8 | `CuratorReport.DecisionsJSON` 由 GORM hook 与 `Decisions` 互转 | 高 | DB 不保存 decisions | 在 model hook 中实现（见 core.md §3.1） |
| 9 | `agent_skill_curator_enabled` 默认 false 的改动不会破坏现有期望默认开启的测试 | 中 | 现有 curator 测试失败 | 更新 `skill_curator_test.go` 中显式设置 enabled |

---

## 9. Risk Register

| # | 风险 | 可能性 | 影响 | 缓解措施 |
|---|------|--------|------|----------|
| 1 | Safety=high 时 restore 快照失败，导致无法回滚 | 低 | 错误变更残留 | restore 失败记 error；用户可手动从 `<storageDir>/skill-backups` 恢复 |
| 2 | UmbrellaBuilder 创建的 umbrella 名称与现有 skill 冲突 | 中 | 创建失败或覆盖 | 先 GetBySlug 检查；冲突时追加序号或跳过 |
| 3 | LLM Review 在 live 模式下删除/修改用户依赖的 skill | 中 | 业务逻辑受损 | Pin 保护 + deterministic transitions 先行 + high safety 可回滚 |
| 4 | 报告文件包含 skill 内容造成敏感信息泄露 | 低 | 数据泄露 | 报告目录与 storage 同权限；不对外提供 HTTP 访问 |
| 5 | 大 workspace 导致 Review 成本过高 | 中 | API 费用高 | 限制 max_iterations；候选集可先按 category 分批（未来优化） |
| 6 | `AbsorbedInto` 字段缺失导致 umbrella 关系无法追踪 | 中 | 报告不完整 | 本文件已明确补充该字段；实现时优先迁移 |

---

## 10. Local Notes

- **与 Part 2 的关系**：`LLMReviewService` 是 `ReviewAgentRuntime` 的唯一调用方；所有 prompt 构造、结果解析、成本估算集中在此处，便于后续调整。
- **与 Part 1/core.md 的关系**：`CuratorReportService` 实现 `core.md` §2.3 的接口；`UmbrellaBuilder` 实现 `core.md` §2.2。`AgentSkill.AbsorbedInto` 是 core.md 未覆盖但本批次必需的数据模型补充，已在 §3.6 说明。
- **向后兼容**：保留旧的 `NewSkillCuratorJob` / `NewSkillCuratorJobWithBackup` 构造函数，避免破坏未接入 Batch 2 的 main.go 路径；主路径使用新增的 `NewSkillCuratorJobWithReview`。
- **manual trigger**：用户可通过系统设置 API 将 `agent_skill_curator_enabled` 设为 `true`，或通过 worker manager 的 `Trigger("skill-curator")` 手动触发一次。
