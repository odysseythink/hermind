# Plan 6c.2: Remaining Memory Providers Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans.

**Goal:** Port the remaining four external memory providers from the Python reference tree so all eight memprovider adapters are available. The reference tree has no `ex-machina` plugin (earlier conversation mentioned it speculatively), so only four adapters remain.

| Provider | Transport | Tools |
|---|---|---|
| **retaindb** | HTTP `POST /v1/memory` + `/v1/memory/search` | `retaindb_save`, `retaindb_search` |
| **openviking** | HTTP `POST /api/v1/search/find` + `/api/v1/sessions/{id}/messages` | `openviking_find`, `openviking_append` |
| **byterover** | Local `brv` CLI via `os/exec` | `brv_query`, `brv_curate`, `brv_status` |
| **holographic** | Local SQLite (reuses existing `storage.SearchMemories`) | `holographic_remember`, `holographic_recall` |

**Architecture:** Each provider is a new file in `tool/memory/memprovider/`. HTTP ones follow the same shape as Honcho/Mem0/Supermemory. Byterover shells out via `os/exec.CommandContext`. Holographic wraps the existing `storage.Storage.SaveMemory` / `SearchMemories` with a distinct pair of tool names so it doesn't collide with `memory_save` / `memory_search`.

**Tech Stack:** Go 1.25 stdlib (`net/http`, `os/exec`), existing `httpJSON`, `storage.Storage`. No new deps.

---

## Task 1: Config blocks

- [ ] Add to `config/config.go`:

```go
// RetainDBConfig holds the RetainDB provider configuration.
type RetainDBConfig struct {
	BaseURL string `yaml:"base_url,omitempty"`
	APIKey  string `yaml:"api_key,omitempty"`
	Project string `yaml:"project,omitempty"`
	UserID  string `yaml:"user_id,omitempty"`
}

// OpenVikingConfig holds the OpenViking provider configuration.
type OpenVikingConfig struct {
	Endpoint string `yaml:"endpoint,omitempty"`
	APIKey   string `yaml:"api_key,omitempty"`
}

// ByteroverConfig holds the Byterover CLI wrapper configuration.
// Byterover is driven by a local `brv` CLI; this config only records
// an optional explicit path to the binary.
type ByteroverConfig struct {
	BrvPath string `yaml:"brv_path,omitempty"`
	Cwd     string `yaml:"cwd,omitempty"`
}

// HolographicConfig is a placeholder — the holographic provider uses
// the shared SQLite storage so there is no backend URL or key.
type HolographicConfig struct{}
```

Add the fields to `MemoryConfig` so `provider: retaindb|openviking|byterover|holographic` picks them up.

- [ ] `go test ./config/...` — PASS.
- [ ] Commit `feat(config): add RetainDB, OpenViking, Byterover, Holographic memory configs`.

---

## Task 2: retaindb provider

- [ ] Create `tool/memory/memprovider/retaindb.go` implementing `Provider` with two tools (`retaindb_save`, `retaindb_search`) backed by `httpJSON`.
- [ ] Create `tool/memory/memprovider/retaindb_test.go` driving an `httptest.Server`.
- [ ] Run tests — PASS.
- [ ] Commit `feat(tool/memory): add RetainDB HTTP provider`.

---

## Task 3: openviking provider

- [ ] Create `tool/memory/memprovider/openviking.go` implementing `Provider` with two tools (`openviking_find`, `openviking_append`). The provider maintains an internal session id (generated on Initialize) and uses it for `/api/v1/sessions/{id}/messages`.
- [ ] Create `tool/memory/memprovider/openviking_test.go` against `httptest.Server`.
- [ ] Run tests — PASS.
- [ ] Commit `feat(tool/memory): add OpenViking HTTP provider`.

---

## Task 4: byterover CLI provider

- [ ] Create `tool/memory/memprovider/byterover.go` that locates `brv` via `exec.LookPath` (or `cfg.BrvPath`) and runs `brv query`, `brv curate`, `brv status` subcommands via `exec.CommandContext`.
- [ ] Each tool captures stdout/stderr and returns them in a JSON result. If `brv` isn't found, `IsAvailable` returns false — but since we don't have `IsAvailable` in the `Provider` interface yet, we instead return an error from `Initialize` which the CLI logs as a skip.
- [ ] Create `tool/memory/memprovider/byterover_test.go` that substitutes the command via a package-level `execCommand` function variable (so the test can point at an echo script).
- [ ] Run tests — PASS.
- [ ] Commit `feat(tool/memory): add Byterover CLI wrapper provider`.

---

## Task 5: holographic provider (reuses storage)

- [ ] Create `tool/memory/memprovider/holographic.go`:

```go
type Holographic struct {
    store     storage.Storage
    sessionID string
}

func NewHolographic(s storage.Storage) *Holographic { return &Holographic{store: s} }
```

`RegisterTools` registers `holographic_remember` (wraps `SaveMemory`) and `holographic_recall` (wraps `SearchMemories`). `SyncTurn` stores the turn as a memory with category=`conversation`. Shutdown is a no-op.

- [ ] Factory's `holographic` case passes the already-open `storage.Storage` from the CLI into `NewHolographic`. Since the existing factory signature only takes `config.MemoryConfig`, we add a second optional argument: `New(cfg, opts ...FactoryOption)` with an option for injecting the storage. CLI wiring passes `WithStorage(app.Storage)`.
- [ ] Create `tool/memory/memprovider/holographic_test.go` using an in-memory sqlite store.
- [ ] Run tests — PASS.
- [ ] Commit `feat(tool/memory): add Holographic local memory provider on top of existing storage`.

---

## Task 6: CLI wiring

- [ ] Update `cli/repl.go`'s `memprovider.New(app.Config.Memory)` call to `memprovider.New(app.Config.Memory, memprovider.WithStorage(app.Storage))`.
- [ ] Run `go test ./...` — PASS.
- [ ] Commit `feat(cli): pass storage to memprovider factory so holographic works`.

---

## Verification Checklist

- [ ] All four providers register their tools when selected
- [ ] Unknown provider names still error out cleanly via the factory
- [ ] `go test ./...` passes
