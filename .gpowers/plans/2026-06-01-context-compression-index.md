# Context Compression Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use gpowers:subagent-driven-development (recommended) or gpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Integrate Pantheon's context-compression engine into Hermind's Agent and Regular Chat paths, with persistent summaries, calibrated thresholds, and upstream Pantheon gap-filling.

**Architecture:** Reuse Pantheon's `agent/compression` 5-phase engine (no re-implementation). Hermind provides a thin adapter layer (`internal/agent/compression/`) for model-metadata lookup, factory wiring, persistence to `thread_compactions`, and tuned redaction patterns. Agent path auto-compresses per step via `WithContextEngine`; Chat path compresses explicitly before LLM invocation when threshold exceeded. Upstream Pantheon receives 8 gap-fill changes as an independent PR.

**Tech Stack:** Go 1.26, Gin, GORM (SQLite/Postgres), Pantheon SDK (`github.com/odysseythink/pantheon`), Gorilla WebSocket, React 18 + Vite.

---

## File Structure

### New files (hermind)

| Path | Responsibility |
|---|---|
| `backend/internal/models/thread_compaction.go` | GORM model for persisted compression summaries |
| `backend/internal/agent/compression/model_metadata.go` | Static `model→contextLength` map + `ContextLengthFor()` lookup |
| `backend/internal/agent/compression/factory.go` | `NewForAgent()` / `NewForChat()` constructors with path-specific thresholds |
| `backend/internal/agent/compression/persistence.go` | `CompactionStore` — `LoadLatest`, `Save`, `SeedForSession` |
| `backend/internal/agent/compression/redact_patterns.go` | Tuned regex set (removes bare-hex & email, keeps key/token rules) |
| `backend/internal/agent/compression/*_test.go` | Unit tests for each core file |
| `backend/internal/services/chat_service_compaction_test.go` | Chat-path behavioral tests (trigger, incremental read, soft-delete) |
| `backend/internal/agent/agent_compaction_test.go` | Agent-path e2e tests (trigger, persist, reconnect, fallback warning) |
| `backend/internal/handlers/compress_endpoint_test.go` | `POST /compress` endpoint tests |
| `backend/internal/services/thread_handoff_test.go` | Cross-thread handoff tests |
| `backend/internal/services/usage_calibration_test.go` | Real usage calibration tests |
| `backend/internal/services/workspace_compress_settings_test.go` | Per-workspace override priority tests |

### Modified files (hermind)

| Path | Responsibility |
|---|---|
| `backend/internal/models/workspace.go` | Add `CompressEnabled *bool`, `CompressThreshold *float64`, `CompressContextLen *int` |
| `backend/internal/models/workspace_thread.go` | Add `ParentThreadID *int` |
| `backend/internal/services/db.go` | AutoMigrate `ThreadCompaction`; seed `context_compress_enabled=false` default |
| `backend/internal/services/chat_service.go` | Incremental `buildChatHistory` (summary seed + `id>UpToChatID`); compression gate in `Stream`/`Complete`; constructor receives `CompactionStore` |
| `backend/internal/agent/session.go` | `newSession` accepts `*compression.DefaultCompressor`, stores on `Session` |
| `backend/internal/agent/handler.go` | `HandleWS` builds compressor (conditional), calls `SetPreviousSummary`, passes to `newSession`, wires `WithContextEngine` |
| `backend/internal/agent/runtime.go` | `RunAgentDirectly` same compressor wiring as `handler.go` |
| `backend/internal/services/thread_service.go` | `Create` copies parent-thread latest compaction as seed when `ParentThreadID!=nil` |
| `backend/internal/services/workspace_service.go` | `Update` maps compression override fields from DTO |
| `backend/internal/dto/workspace.go` | `UpdateWorkspaceRequest` + `CreateThreadRequest` add compression/parent fields |
| `backend/internal/handlers/workspace.go` | New `POST /api/workspaces/:slug/compress` handler (or new `compress.go`) |
| `backend/cmd/server/main.go` | Wire `CompactionStore` into `ChatService` and agent handler deps |
| `backend/go.mod` | Bump or `replace` Pantheon to version with upstream changes (or local path) |

### Modified files (pantheon — independent PR)

| Path | Responsibility |
|---|---|
| `pantheon/agent/compression/state.go` | Add `PreviousSummary()`/`SetPreviousSummary()`/`LastFallbackUsed()` accessors; implement 600s third cooldown tier; implement fallback-model retry in `generateSummaryWithFallback` |
| `pantheon/agent/compression/config.go` | Add `RedactPatterns []*regexp.Regexp` field |
| `pantheon/agent/compression/summary.go` | Add per-message 6000-char truncation in `renderTranscript`; accept injectable redact patterns; use `summaryPrefix` + end marker in assembled output |
| `pantheon/agent/compression/prune.go` | Per-tool summary templates (`terminal`, `browser_navigate`, `create_files`, `web_scraping`, `session_search` + generic fallback); accept injectable redact patterns |
| `pantheon/agent/compression/assemble.go` | Prefix summary with `summaryPrefix`; append end marker when summary lands as user role |
| `pantheon/agent/compression/compressor.go` | Ensure `UpdateFromResponse` usage is surfaced to callers |
| `pantheon/agent/stream.go` / `pantheon/agent/agent.go` | Call `contextEngine.UpdateFromResponse(resp.Usage)` after each step |

---

## Dependency Overview

```
Phase 0 (pantheon-upstream)  ─────────────────────────────────────────────┐
  [independent PR, can land before or after hermind side]                   │
                                                                           │
Phase 1 (hermind-models)                                                   │
  Task M1  ThreadCompaction model                                          │
  Task M2  Workspace + WorkspaceThread new fields                          │
  Task M3  AutoMigrate + SystemSetting defaults                            │
       │                                                                   │
       ▼                                                                   │
Phase 2 (hermind-core)                                                     │
  Task C1  model_metadata.go + test                                        │
  Task C2  redact_patterns.go + test                                       │
  Task C3  persistence.go (CompactionStore) + test                         │
  Task C4  factory.go + test                                               │
       │                                                                   │
       ├──────────────────────────┬──────────────────────────┐             │
       ▼                          ▼                          ▼             │
Phase 3 (hermind-chat-path)   Phase 4 (hermind-agent-path)   │             │
  Task H1  buildChatHistory     Task A1  session/handler/    │             │
           incremental read              runtime compressor  │             │
  Task H2  compression gate     Task A2  step-end persist    │             │
           in Stream/Complete           + WS event            │             │
  Task H3  ChatService ctor     Task A3  Shared-signature    │             │
           + whole-tree fix              ripple (main.go)     │             │
       │                          │                          │             │
       └──────────────┬───────────┘                          │             │
                      ▼                                      │             │
               Phase 5 (hermind-extensions) ◄────────────────┘             │
                 Task E1  POST /compress endpoint                          │
                 Task E2  Cross-thread handoff                             │
                 Task E3  Real usage calibration (Chat)                    │
                 Task E4  Telemetry + observability wiring                 │
                      │                                                     │
                      ▼                                                     │
               Phase 6 (hermind-frontend)                                   │
                 Task F1  Workspace settings UI (React)                     │
```

**Parallelizable phases:**
- Phase 0 (pantheon-upstream) runs fully in parallel with Phases 1–5.
- Phase 1 (models) and Phase 0 can start immediately.
- Phase 6 (frontend) only needs Phase 5's backend API surface; can start once `POST /compress` and workspace-update DTOs are stable.

---

## Risks & Open Questions

| # | Risk | Assumption | Impact if wrong |
|---|---|---|---|
| 1 | Chat soft-delete (`include=false`) hides history from UI | Frontend chat list queries do **not** filter on `include` | Users lose visible history; mitigation: verify UI query before soft-delete goes live |
| 2 | Pantheon upstream PR delayed | All 8 upstream changes are backward-compatible additions | Hermind can integrate with current Pantheon version and gain features incrementally via version bump |
| 3 | `thread_id=nil` default workspace session leaks across users | `workspace_id` fully isolates sessions in multi-user mode | History leakage across workspaces; mitigation: queries always include `workspace_id` |
| 4 | `modelContextLength` map misses unknown models | Conservative default 8192 is safe | Premature compression on large-context models; mitigation: workspace-level `CompressContextLen` override |
| 5 | Agent path `PreviousSummary` accessor not yet in released Pantheon | We `replace` to local Pantheon workspace during development | Agent summary persistence is blocked; plan includes typed-shim fallback (memory-only) if accessor unavailable |
| 6 | V1 scope inflated by 5 extension items | Each extension is independently shippable | Schedule slip; mitigation: strict phase ordering — core chat+agent paths first, extensions incrementally |

---

## Spec Coverage

Map every design-document section to the implementing sub-plan file and task(s).

| Design § | Requirement | Sub-plan File | Task(s) | Status |
|---|---|---|---|---|
| §1.1 | Agent path `WithContextEngine` | `hermind-agent-path.md` | A1 | pending |
| §1.1 | Regular Chat path compression | `hermind-chat-path.md` | H1–H2 | pending |
| §1.1 | Persistence `thread_compactions` | `hermind-models.md` + `hermind-core.md` | M1, C3 | pending |
| §1.1 | Calibration `UpdateModel` + ctxLen | `hermind-core.md` | C1, C4 | pending |
| §1.1 | Redaction tuning | `hermind-core.md` | C2 | pending |
| §1.1 | Global + per-workspace switch | `hermind-models.md` + `hermind-chat-path.md` + `hermind-agent-path.md` | M2, H2, A1 | pending |
| §1.1 | `/compress` endpoint | `hermind-extensions.md` | E1 | pending |
| §1.1 | Cross-thread handoff | `hermind-extensions.md` | E2 | pending |
| §1.1 | Real usage calibration | `hermind-extensions.md` | E3 | pending |
| §1.1 | Per-workspace UI | `hermind-frontend.md` | F1 | pending |
| §1.1 | 600s cooldown tier | `pantheon-upstream.md` | P5 | pending |
| §5 | Config defaults (Agent 0.50 / Chat 0.75) | `hermind-core.md` | C4 | pending |
| §6 | Model context length map | `hermind-core.md` | C1 | pending |
| §7 | Redact patterns (no bare-hex/email) | `hermind-core.md` | C2 | pending |
| §8 | Degradation & error layers | `hermind-chat-path.md` + `hermind-agent-path.md` | H2, A2 | pending |
| §9 | Observability (WS events, logs, telemetry) | `hermind-extensions.md` | E4 | pending |
| §10 | Upstream Pantheon 8 changes | `pantheon-upstream.md` | P1–P8 | pending |
| §11.1 | `buildChatHistory` incremental read | `hermind-chat-path.md` | H1 | pending |
| §11.2 | Chat compression gate | `hermind-chat-path.md` | H2 | pending |
| §11.3 | Agent wiring | `hermind-agent-path.md` | A1 | pending |
| §12 | API endpoints | `hermind-extensions.md` | E1 | pending |
| §18.1 | Manual `/compress` | `hermind-extensions.md` | E1 | pending |
| §18.2 | Thread handoff + MemoryProvider | `hermind-extensions.md` | E2 | pending |
| §18.3 | Usage calibration | `hermind-extensions.md` | E3 | pending |
| §18.4 | Workspace settings UI | `hermind-frontend.md` | F1 | pending |
| §18.5 | 600s cooldown | `pantheon-upstream.md` | P5 | pending |

---

## Parts (generate one per invocation, in order)

> ▶ To generate the next `pending` part: run `/compact`, then re-invoke the **`/writing-plans`** slash command. Do NOT type "continue" — it skips the rule reload and batch-generates everything.

| # | File | Scope | Status |
|---|---|---|---|
| 1 | `2026-06-01-context-compression-pantheon-upstream.md` | 8 upstream Pantheon gap-fill changes | done |
| 2 | `2026-06-01-context-compression-hermind-models.md` | GORM models + AutoMigrate + system defaults | done |
| 3 | `2026-06-01-context-compression-hermind-core.md` | Adapter layer: metadata, factory, persistence, redact + tests | done |
| 4 | `2026-06-01-context-compression-hermind-chat-path.md` | Chat path: incremental buildChatHistory + compression gate | done |
| 5 | `2026-06-01-context-compression-hermind-agent-path.md` | Agent path: WithContextEngine wiring + step-end persistence | done |
| 6 | `2026-06-01-context-compression-hermind-extensions.md` | /compress endpoint, handoff, usage calibration, telemetry | done |
| 7 | `2026-06-01-context-compression-hermind-frontend.md` | React workspace settings UI for compression | done |
