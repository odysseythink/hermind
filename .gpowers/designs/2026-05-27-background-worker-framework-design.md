# Background Worker Framework Design

> backend background job scheduler: cron + lifecycle + graceful shutdown.
> Scope: general-purpose framework + initial 4 jobs migrated from Node.

---

## 1. Background & Goals

### Current State

- **Node server** uses `@mintplex-labs/bree` to run 3 recurring cron jobs + 1 on-demand embedding worker:
  - `cleanup-orphan-documents` — every 12hr
  - `cleanup-generated-files` — every 8hr
  - `sync-watched-documents` — every 1hr (conditional on `DocumentSyncQueue.enabled()`)
  - `embedding-worker` — spawned on-demand via IPC
- **Go server** has zero background job infrastructure. This gap is flagged as "high" in the capability gap analysis.

### Goals

1. Provide a general-purpose worker framework in `backend` that can run cron-scheduled and on-demand background jobs.
2. Migrate the 4 Node jobs as the first implementations.
3. Support graceful shutdown: stop scheduling, wait for running jobs, respect timeout.
4. Keep the design simple and Go-idiomatic (goroutines, not subprocesses).
5. Allow future jobs to be added with minimal boilerplate.

### Non-Goals

- Distributed job queues (Redis, RabbitMQ, etc.) — out of scope.
- Subprocess isolation for embedding worker — Go’s memory management and `recover()` are sufficient.
- Web UI for job management — not needed at this stage.

---

## 2. Architecture

### 2.1 Package Layout

```
backend/internal/workers/
├── manager.go              # WorkerManager: lifecycle, cron, graceful shutdown
├── job.go                  # Job interface + common utilities
├── cleanup_orphan.go       # Job: cleanup-orphan-documents
├── cleanup_generated.go    # Job: cleanup-generated-files
├── sync_watched.go         # Job: sync-watched-documents
├── embed_worker.go         # Job: embedding-worker (on-demand)
└── manager_test.go         # Unit tests for manager and job behaviour
```

### 2.2 Dependencies

- `github.com/robfig/cron/v3` — mature cron scheduler (54k+ stars, v3 stable).

### 2.3 Job Interface

```go
package workers

import "context"

// Job is the unified abstraction for a background task.
type Job interface {
    // Name returns the job identifier used in logs and monitoring.
    Name() string

    // Schedule returns a cron expression (e.g. "0 */12 * * *")
    // or a fixed interval (e.g. "@every 12h").
    // An empty string means the job is NOT auto-scheduled by cron
    // (e.g. on-demand jobs like embed-worker).
    Schedule() string

    // Enabled is checked before each execution.  If it returns false
    // the run is skipped for this cycle.
    Enabled(ctx context.Context) bool

    // Run executes the job body.  ctx carries a per-job timeout.
    Run(ctx context.Context) error
}
```

### 2.4 WorkerManager

```go
type Manager struct {
    cron   *cron.Cron
    jobs   []Job
    db     *gorm.DB
    cfg    *config.Config

    wg     sync.WaitGroup
    mu     sync.RWMutex
    state  State // booting | running | stopping | stopped
}

func NewManager(db *gorm.DB, cfg *config.Config) *Manager
func (m *Manager) Register(jobs ...Job)
func (m *Manager) Start() error
func (m *Manager) Stop(ctx context.Context) error
func (m *Manager) Trigger(name string) error // manual trigger for on-demand jobs
```

**Lifecycle**:

```
Boot (main.go)
  │
  ├─ NewManager(db, cfg)
  ├─ Register(Job1, Job2, Job3, Job4)
  └─ Start() ──► [cron starts, jobs scheduled]

Shutdown (SIGINT/SIGTERM)
  │
  ├─ srv.Shutdown(ctx)      // 1. stop accepting HTTP
  ├─ mcpHyp.PruneAll()      // 2. drain MCP children
  ├─ mgr.Stop(ctx)          // 3. stop cron, wait running jobs
  └─ agentRuntime.Shutdown(ctx) // 4. stop agent runtime
```

**Stop() internals**:

1. `cron.Stop()` — prevents new jobs from being scheduled.
2. `wg.Wait()` — blocks until all currently running jobs finish.
3. If `wg.Wait` returns before ctx timeout → clean exit.
4. If ctx times out → log warning and return; running goroutines receive `ctx.Done()` and should abort promptly.

**Panic recovery**: each job invocation is wrapped in `defer recover()` so a panicking job logs the error but does not crash the server.

---

## 3. Configuration

### 3.1 Env-Based Intervals & Timeouts

New fields in `internal/config/config.go`:

```go
// === Background Workers ===
WorkerCleanupOrphanInterval   string `env:"WORKER_CLEANUP_ORPHAN_INTERVAL"   envDefault:"0 */12 * * *"`
WorkerCleanupOrphanTimeout    string `env:"WORKER_CLEANUP_ORPHAN_TIMEOUT"    envDefault:"5m"`
WorkerCleanupOrphanEnabled    bool   `env:"WORKER_CLEANUP_ORPHAN_ENABLED"    envDefault:"true"`

WorkerCleanupGeneratedInterval string `env:"WORKER_CLEANUP_GENERATED_INTERVAL" envDefault:"0 */8 * * *"`
WorkerCleanupGeneratedTimeout  string `env:"WORKER_CLEANUP_GENERATED_TIMEOUT"  envDefault:"5m"`
WorkerCleanupGeneratedEnabled  bool   `env:"WORKER_CLEANUP_GENERATED_ENABLED"  envDefault:"true"`

WorkerSyncWatchedInterval      string `env:"WORKER_SYNC_WATCHED_INTERVAL"      envDefault:"0 * * * *"`
WorkerSyncWatchedTimeout       string `env:"WORKER_SYNC_WATCHED_TIMEOUT"       envDefault:"10m"`
WorkerSyncWatchedEnabled       bool   `env:"WORKER_SYNC_WATCHED_ENABLED"       envDefault:"true"`
```

### 3.2 Runtime Conditional Enablement

- `sync-watched-documents` queries the DB (`document_sync_queues` table) in its `Enabled()` method to decide whether there are stale documents to process.
- This mirrors Node behaviour where `DocumentSyncQueue.enabled()` gates the job registration.

---

## 4. Job Specifications

### 4.1 cleanup-orphan-documents

| Attribute | Value |
|-----------|-------|
| Schedule | `WORKER_CLEANUP_ORPHAN_INTERVAL` |
| Timeout | `WORKER_CLEANUP_ORPHAN_TIMEOUT` |
| Enabled | `WORKER_CLEANUP_ORPHAN_ENABLED` |

**Logic**:
1. Query DB for all records in `workspace_parsed_files`.
2. List files under `cfg.StorageDir`.
3. Delete files on disk that have no DB reference.
4. Log count of deleted / failed files.

**Node mapping**: `server/jobs/cleanup-orphan-documents.js`

### 4.2 cleanup-generated-files

| Attribute | Value |
|-----------|-------|
| Schedule | `WORKER_CLEANUP_GENERATED_INTERVAL` |
| Timeout | `WORKER_CLEANUP_GENERATED_TIMEOUT` |
| Enabled | `WORKER_CLEANUP_GENERATED_ENABLED` |

**Logic**:
1. Read `storageDir/outputs/` directory.
2. Scan `workspace_chats` with `include: true` and parse JSON content for file references.
3. Delete files not referenced by any active chat.
4. Match filename pattern `/^[a-z]+-[a-f0-9-]{36}(\.\w+)?$/i` (same as Node).

**Node mapping**: `server/jobs/cleanup-generated-files.js`

### 4.3 sync-watched-documents

| Attribute | Value |
|-----------|-------|
| Schedule | `WORKER_SYNC_WATCHED_INTERVAL` |
| Timeout | `WORKER_SYNC_WATCHED_TIMEOUT` |
| Enabled | `WORKER_SYNC_WATCHED_ENABLED` AND DB has stale queues |

**Logic**:
1. Query DB for stale `document_sync_queue` entries.
2. If none, exit early.
3. For each stale document:
   - Check collector API is online (`collector.Client`).
   - Re-fetch document content via collector.
   - Update DB record.
   - Re-run embedding + vector DB update.
4. Log processed / failed counts.

**Node mapping**: `server/jobs/sync-watched-documents.js`

### 4.4 embed-worker (on-demand)

| Attribute | Value |
|-----------|-------|
| Schedule | `""` (no cron schedule) |
| Trigger | `Manager.Trigger("embed-worker")` |

**Logic**:
1. Accept a list of files + workspace info (via an internal queue/channel).
2. For each file: chunk → embed → write to vector DB.
3. Report progress via callback or future channel.
4. Recover from panic to prevent OOM from crashing the server.

**Node mapping**: `server/jobs/embedding-worker.js` (goroutine replaces subprocess).

---

## 5. Error Handling & Observability

### 5.1 Error Handling

- Job `Run()` returns `error`; Manager logs it with `mlog.Error`.
- A failed job does NOT block other jobs or prevent future scheduling.
- Panics are recovered inside Manager; error is logged, goroutine exits cleanly.

### 5.2 Logging

```go
mlog.Info("worker job started", mlog.String("job", name))
mlog.Info("worker job finished", mlog.String("job", name), mlog.Duration("duration", d))
mlog.Error("worker job failed", mlog.String("job", name), mlog.Err(err))
mlog.Error("worker job panicked", mlog.String("job", name), mlog.Any("recover", r))
```

### 5.3 State Transitions

Manager logs on every state change: `booting → running → stopping → stopped`.

---

## 6. Testing Strategy

| Test | Description |
|------|-------------|
| `TestManager_StartStop` | Start manager, verify cron entries registered, Stop waits for running job |
| `TestManager_PanicRecovery` | Job that panics should not crash manager |
| `TestManager_Trigger` | Manual trigger of on-demand job |
| `TestCleanupOrphanJob_Run` | Mock DB + temp dir, verify orphan files deleted |
| `TestCleanupGeneratedJob_Run` | Mock chat DB + temp outputs dir, verify unreferenced files deleted |
| `TestSyncWatchedJob_Enabled` | Returns false when no stale queues, true when stale queues exist |
| `TestEmbedWorkerJob_Run` | Mock embedder + vector DB, verify files processed |

---

## 7. main.go Integration

```go
// Boot
mgr := workers.NewManager(db, cfg)
mgr.Register(
    workers.NewCleanupOrphanJob(db, cfg),
    workers.NewCleanupGeneratedJob(db, cfg),
    workers.NewSyncWatchedJob(db, cfg, coll),
    workers.NewEmbedWorkerJob(db, cfg, emb, vectorDB),
)
if err := mgr.Start(); err != nil {
    mlog.Fatal("failed to start worker manager", mlog.Err(err))
}

// Shutdown
shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

srv.Shutdown(shutdownCtx)
mcpHyp.PruneAll()
if err := mgr.Stop(shutdownCtx); err != nil {
    mlog.Warning("worker manager stop timed out", mlog.Err(err))
}
agentRuntime.Shutdown(shutdownCtx)
```

---

## 8. Future Extensions (Out of Scope)

- Metrics endpoint exposing `workers_running_jobs`, `workers_job_duration_seconds`, etc.
- HTTP admin endpoint to list jobs, trigger manually, or view last-run status.
- Job retry with exponential backoff.
- Distributed locking so only one instance runs a job in a replicated deployment.

---

## 9. Decisions Log

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Scheduler library | `robfig/cron/v3` | De-facto standard, mature, supports cron expressions and `@every` intervals |
| Process isolation | No (goroutines only) | Go memory management is stable; `recover()` handles panics; Node’s subprocess model was mainly for V8 OOM isolation |
| Config source | Env for intervals/timeouts; DB for runtime conditions | Env = ops-friendly; DB = dynamic behaviour like `DocumentSyncQueue.enabled()` |
| Scope | General framework + 4 initial jobs | Framework-first so adding future jobs is ~20 lines |
| Graceful shutdown timeout | 30s global; per-job timeout from env | Node uses 1m/5m per job; 30s global aligns with existing server shutdown timeout |
