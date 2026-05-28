# Scheduled Jobs — Design

**Date**: 2026-05-28
**Status**: Draft
**Authors**: ranwei + Claude
**Companion docs**:
- `2026-05-28-phase2-quick-wins-design.md`
- `2026-05-28-memories-system-design.md`
- `2026-05-28-webpush-design.md` (downstream consumer)
- Source: `.gpowers/reports/server-collector-vs-go-server-gap-analysis.md` §10

## 1. Purpose

Let users schedule agent runs via cron expressions, with persisted job definitions, per-run history (status/result/error), at-most-one-in-flight dedup per job, kill semantics, and orphan recovery. Match anything-llm's user-facing surface (3 endpoints + continueInThread) without forking subprocesses.

## 2. Scope

### In
- 2 tables (`scheduled_jobs`, `scheduled_job_runs`)
- `JobScheduler` service: dynamic cron registration on top of existing `workers.Manager`; concurrency-limited goroutine pool; per-job kill via cancelFuncs; boot-time orphan recovery
- 7 endpoints (CRUD + trigger + kill + continue + tools-catalog + runs-list)
- `EphemeralAgentRunner` reuse with auto-approve callback override
- Event-log row on completion (downstream consumer = Design D WebPush)
- continueInThread: dedicated "Scheduled Jobs" workspace + new thread per continue

### Out
- WebPush — Design D
- Multi-run-per-job concurrency (anything-llm is also single-in-flight)
- Job ownership / per-user job isolation (Phase 4 — anything-llm doesn't have it either)
- UI work (frontend is anything-llm React app; design is backend-only)
- Subprocess isolation (decision recorded in §3.2 below)

## 3. Architectural decisions

### 3.1 Tables — exact mirror of anything-llm

```go
// backend/internal/models/scheduled_job.go
type ScheduledJob struct {
    ID        int    `gorm:"primaryKey;autoIncrement" json:"id"`
    Name      string `gorm:"not null" json:"name"`
    Prompt    string `gorm:"type:text;not null" json:"prompt"`
    Tools     string `gorm:"type:text" json:"tools"` // JSON array; "" = all defaults
    Schedule  string `gorm:"not null" json:"schedule"` // cron expression
    Enabled   bool   `gorm:"default:true" json:"enabled"`
    LastRunAt *time.Time `json:"lastRunAt,omitempty"`
    NextRunAt *time.Time `json:"nextRunAt,omitempty"`
    CreatedAt time.Time `gorm:"autoCreateTime" json:"createdAt"`
    UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updatedAt"`
}

// backend/internal/models/scheduled_job_run.go
type ScheduledJobRun struct {
    ID          int       `gorm:"primaryKey;autoIncrement" json:"id"`
    JobID       int       `gorm:"index;not null" json:"jobId"`
    Status      string    `gorm:"default:queued;index" json:"status"` // queued|running|completed|failed|timed_out
    Result      string    `gorm:"type:text" json:"result"` // JSON
    Error       string    `gorm:"type:text" json:"error"`
    StartedAt   time.Time `gorm:"autoCreateTime" json:"startedAt"`
    CompletedAt *time.Time `json:"completedAt,omitempty"`
    ReadAt      *time.Time `json:"readAt,omitempty"`
    Job         *ScheduledJob `gorm:"foreignKey:JobID;constraint:OnDelete:CASCADE" json:"job,omitempty"`
}
```

### 3.2 Runtime — goroutine + semaphore (no subprocess)

Decision: anything-llm uses a forked Node child per run because the Node event loop is single-threaded and a CPU-heavy agent run would starve all chats. Go's goroutine model already has true concurrency, so subprocess isolation buys very little here and costs a second binary + IPC + cross-platform fork headaches. We accept the trade-off that a panicking agent run will crash the server (mitigated by goroutine-level `recover` + standard panic→error funnel).

```go
// backend/internal/scheduler/job_scheduler.go
type JobScheduler struct {
    cron     *cron.Cron               // dedicated cron; lifecycle co-owned with workers.Manager
    sem      chan struct{}            // size = SCHEDULED_JOB_MAX_CONCURRENT (default 1)
    db       *gorm.DB

    mu           sync.RWMutex
    entries      map[int]cron.EntryID  // jobId → cron entry
    inflightCxl  map[int][]context.CancelFunc // jobId → cancelFns for active runs

    agentRunner  AgentRunner   // injected EphemeralAgent factory (interface for testability)
    timeout      time.Duration // SCHEDULED_JOB_TIMEOUT_MS, default 10min
}
```

The `cron` instance is independent of `workers.Manager` (different lifetime — Manager has static schedule-and-go; Scheduler needs runtime add/remove). Both `Start` from the same boot path; both `Stop` from the same shutdown path.

### 3.3 Dedup — DB transaction, not in-memory

Mirror anything-llm's `ScheduledJobRun.start`: BEGIN TXN → check `WHERE jobId=? AND status IN ('queued','running')` → if none, INSERT new row (status=queued) → COMMIT. SQLite serializes writes, so two concurrent enqueues for the same job cannot both succeed.

GORM:
```go
func (s *Scheduler) startRun(jobID int) (*ScheduledJobRun, error) {
    var run *ScheduledJobRun
    err := s.db.Transaction(func(tx *gorm.DB) error {
        var existing int64
        if err := tx.Model(&ScheduledJobRun{}).
            Where("job_id = ? AND status IN ?", jobID, []string{"queued","running"}).
            Count(&existing).Error; err != nil { return err }
        if existing > 0 { return errAlreadyInFlight }
        run = &ScheduledJobRun{JobID: jobID, Status: "queued"}
        return tx.Create(run).Error
    })
    if err == errAlreadyInFlight { return nil, nil }
    return run, err
}
```

### 3.4 Kill — context cancel + DB transition

Mirror anything-llm's two-step: (a) update DB row to `failed` with `error="Job killed by user"` and `readAt=now()` (`ScheduledJobRun.kill`); (b) call `cancelFn()` to unblock the run goroutine. Order is (a)-then-(b) so the goroutine's `defer` finds the row already in terminal state and is a no-op.

### 3.5 Auto-approve in scheduled context

When `EphemeralAgentRunner` is constructed from a scheduler context, its `ToolContext.Approval` is set to:
```go
func(ctx context.Context, tool string, args any, desc string) (bool, string) {
    eventlog.Append(ctx, "scheduled_job_tool_auto_approved", map[string]any{"tool": tool, "desc": desc})
    return true, "Auto-approved by scheduled job runner."
}
```
Every auto-approved tool call writes an audit row to `event_logs` for traceability.

### 3.6 Orphan recovery on boot

```go
func (s *Scheduler) Boot(ctx context.Context) error {
    // Step 1: any non-terminal runs from previous process = orphans
    s.db.Model(&ScheduledJobRun{}).
        Where("status IN ?", []string{"queued","running"}).
        Updates(map[string]any{
            "status": "failed",
            "error": "Server restarted during execution",
            "completed_at": time.Now(),
        })

    // Step 2: register enabled jobs in cron
    var jobs []ScheduledJob
    s.db.Where("enabled = ?", true).Find(&jobs)
    for _, j := range jobs {
        s.recomputeNextRunAt(j.ID)  // stale nextRunAt may have passed
        s.addCron(j)
    }
    return nil
}
```

## 4. Endpoints

| Method | Path | Body / Params | Returns | Auth |
|---|---|---|---|---|
| GET | `/scheduled-jobs/tools` | — | grouped tool catalog | admin |
| GET | `/scheduled-jobs` | — | `[]ScheduledJob` | admin |
| POST | `/scheduled-jobs` | `{name, prompt, tools[], schedule}` | created job | admin |
| PATCH | `/scheduled-jobs/:id` | partial fields | updated job | admin |
| DELETE | `/scheduled-jobs/:id` | — | `{success}` | admin |
| POST | `/scheduled-jobs/:id/trigger` | — | new run or 409 if in-flight | admin |
| GET | `/scheduled-jobs/:id/runs` | `?limit&offset` | `[]ScheduledJobRun` | admin |
| POST | `/scheduled-jobs/runs/:runId/kill` | — | `{success}` | admin |
| POST | `/scheduled-jobs/runs/:runId/read` | — | `{success}` | admin |
| POST | `/scheduled-jobs/runs/:runId/continue` | — | `{workspace, thread}` | admin |

`POST/PATCH/DELETE` operate against the live `JobScheduler` — after DB write, call `Scheduler.syncJob(id)` so cron registration follows the row.

## 5. Tools catalog

`/scheduled-jobs/tools` returns the same shape as anything-llm: `[{category, name, items:[{id, name, description, requiresSetup}]}, ...]`. Backend already knows its `Toolset` taxonomy via `tool.Entry.Toolset`. Implementation:
```go
func (s *Service) AvailableTools(ctx context.Context) []ToolCategory {
    // 1. Default skills always-available (rag-memory, doc-summarizer, web-scraping)
    // 2. Configurable single skills (web-browsing, sql-agent w/ requiresSetup)
    // 3. Sub-skill plugins (filesystem-agent, create-files-agent, gmail, gcal, outlook)
    //    derived from agent/tools/builder.go's registered Entries grouped by .Toolset
    // 4. MCP servers (from MCPCompatibilityLayer.servers())
    // 5. Imported skills (Hub plugins — empty until Community Hub lands)
}
```

`requiresSetup` is computed by calling into the existing oauth-status helpers (`GmailBridge.GetConfig()`, etc.).

## 6. continueInThread

```go
func (s *Service) ContinueInThread(ctx context.Context, runID int) (workspace *Workspace, thread *WorkspaceThread, err error) {
    // Get run + parent job (in one query)
    run, err := s.GetRunWithJob(ctx, runID)
    if err != nil { return nil, nil, err }

    // Parse run.Result for text + outputs
    var result struct {
        Text    string `json:"text"`
        Sources []any  `json:"sources,omitempty"`
        Outputs []any  `json:"outputs,omitempty"`
    }
    json.Unmarshal([]byte(run.Result), &result)
    if result.Text == "" { result.Text = "No response was generated." }

    // Upsert workspace by slug "scheduled-jobs"
    ws, err := s.workspaceSvc.Upsert(ctx, &Workspace{
        Slug: "scheduled-jobs", Name: "Scheduled Jobs", ChatMode: ptr("automatic"),
    })
    if err != nil { return nil, nil, err }

    // New thread + first chat row (run.Job.Prompt as user msg, result.Text as response)
    t, err := s.threadSvc.New(ctx, ws.ID)
    if err != nil { return nil, nil, err }

    s.chatSvc.Append(ctx, &WorkspaceChat{
        WorkspaceID: ws.ID,
        ThreadID:    &t.ID,
        Prompt:      run.Job.Prompt,
        Response:    mustMarshal(map[string]any{
            "text": result.Text, "sources": result.Sources,
            "outputs": result.Outputs, "type": "chat",
        }),
        Include: true,
    })

    return ws, t, nil
}
```

## 7. Configuration

| Env | Default | Purpose |
|---|---|---|
| `SCHEDULED_JOB_MAX_CONCURRENT` | `1` | semaphore size |
| `SCHEDULED_JOB_TIMEOUT_MS` | `600000` (10min) | per-run wall-clock cap |
| `SCHEDULED_JOB_MAX_ACTIVE` | `0` (unlimited) | hard cap on enabled jobs |

## 8. PR breakdown

**PR1 · schema + model (~150 LoC)**
- `models/scheduled_job.go`, `models/scheduled_job_run.go` with GORM tags
- AutoMigrate registration
- Trivial CRUD service (`services/scheduled_job.go`) — no scheduler yet
- Unit tests for model CRUD + dedup query

**PR2 · scheduler core (~400 LoC)**
- `scheduler/job_scheduler.go` — Boot/Stop/AddCron/RemoveCron/SyncJob/EnqueueRun/Kill
- `scheduler/run_executor.go` — `runOnce(ctx, jobID, runID)` that spawns EphemeralAgent with auto-approve callback, races against timeout, writes terminal state
- Tests: fake clock + in-memory db, exercise dedup, kill, timeout, orphan-on-boot

**PR3 · endpoints (~300 LoC)**
- `handlers/scheduled_job.go` — 9 routes + admin middleware
- Tools catalog handler with `requiresSetup` derivation
- Integration tests with httptest

**PR4 · continueInThread (~150 LoC)**
- `services/scheduled_job_continue.go` — workspace upsert + thread + chat append
- 1 endpoint
- Integration test

**PR5 · event-log hook (~50 LoC)**
- `scheduler/run_executor.go` on terminal state → eventLogSvc.LogEvent(ctx, "scheduled_job_completed"|"_failed", {jobId, runId, error?}, nil)
- Smoke test verifies the row gets written

## 9. Risk register

| # | Risk | Mitigation |
|---|------|------------|
| R1 | Agent panic crashes server | Goroutine-level `recover` → mark run failed with stack; standard pattern from `workers.Manager.wrapJob` |
| R2 | Library that ignores `ctx.Done()` keeps run alive after kill | Document this is a known limitation; agents that hot-loop should have internal cancellation hooks. Same trade-off as anything-llm minus subprocess SIGTERM |
| R3 | `nextRunAt` drift across server restarts | Boot-time `recomputeNextRunAt` for each enabled job; matches anything-llm behavior |
| R4 | Cron expression injection via API | Validate with `cron.ParseStandard` before INSERT; reject 400 |
| R5 | Job deletion race vs in-flight worker (worker writes to deleted row) | Cascade delete handled by FK; before deletion, call `kill` on any in-flight runs and wait `WaitForRun(jobID, 5s)` before issuing DELETE |
| R6 | Auto-approve in scheduled context skips manual approval gates — could expose dangerous tools | Audit each tool call to `event_logs` (R5 mitigation); admins can review. Future: per-job allowlist enforcement |

## 10. Done criteria

- `go test ./backend/...` green
- Integration test: create job with `*/2 * * * *` → wait 3min → assert 1+ runs in `completed` status
- Integration test: trigger job manually → during run, trigger again → 409
- Integration test: trigger job → kill → run row is `failed` with kill error
- Integration test: complete run → POST `/runs/:id/continue` → assert "Scheduled Jobs" workspace + new thread + 1 chat row exist
- Manual smoke: server restart with a `running` row → after boot the row is `failed` with "Server restarted" error
- CHANGELOG entry

## 11. Open questions

None at design time. All resolved during brainstorm:
- Runtime: goroutine + semaphore (no subprocess fork)
- continueInThread: ship with dedicated workspace
- WebPush: out — handled by Design D, scheduled-jobs only emits `event_logs` row
- Tool overrides JSON shape: same as anything-llm (`null=defaults, []=none, [ids]=specified`)
- Max active cap: env var, default unlimited
