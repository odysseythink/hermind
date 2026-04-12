# Dependency-Heavy Feature Phases: Sequencing Design

**Date:** 2026-04-12
**Status:** Approved
**Approach:** Dependency clustering — group by ecosystem to minimize context-switching

## Context

The hermes-agent-rewrite Go port has shipped 19 phases covering core agent loop, 21 platform gateways, 15+ tools, 8+ memory providers, cron, ACP, and CLI. The remaining features require heavy external dependencies that conflict with the project's stdlib-first philosophy. This spec sequences those features into 6 phases ordered by dependency ecosystem affinity.

**Eliminated item:** `batch_runner.py` / `mini_swe_runner.py` are covered by the existing cron/scheduler system — no separate implementation needed.

## Constraints

- Solo developer — minimize context-switching between dependency ecosystems
- No hard deadline — quality over speed, clear ordering so the next task is always obvious
- Total new Go dependencies target: 3 (websocket, protobuf, gotd/td)

## Phase Sequence

| Phase | Item | New Go Deps | Effort Character |
|-------|------|-------------|------------------|
| 1 | Landing page | 0 | Light, creative, decoupled |
| 2 | WebSocket (Discord + Mattermost) | 1 | Medium, shared pattern |
| 3 | OTLP/HTTP trace export | 1 | Medium, clean interface |
| 4 | MTProto (Telegram user accounts) | 1 | Heavy, specialized protocol |
| 5 | Camofox browser | 0 | Light, HTTP client only |
| 6 | RL training | 0 Go deps | Medium, subprocess management |

---

## Phase 1: Landing Page (Plain HTML/CSS/JS)

**Goal:** Marketing site for hermes-agent. No build step, no Node.js.

**Scope:**
- Port and adapt the existing `landingpage/` from the Python reference (24KB HTML, 15KB JS, 23KB CSS) into a `website/` directory in the Go repo
- Update content to reflect Go rewrite features (21 platforms, 8+ LLM providers, cron, ACP, etc.)
- Add placeholder `/docs` link for future documentation site
- Serve from GitHub Pages or any static host

**Not in scope:**
- Docusaurus documentation site (the Python reference has a 3.6MB Docusaurus setup — defer to a future phase)
- Any Go code changes

**Dependencies added:** 0

**Deliverable:** `website/` directory with `index.html`, `style.css`, `script.js`, static assets. Deployable to GitHub Pages.

---

## Phase 2: WebSocket (Discord Gateway + Mattermost Streaming)

**Goal:** Upgrade two outbound-only REST adapters into fully bidirectional real-time gateways using a single WebSocket dependency.

### Current State

- `discord_bot.go` (76 lines) — outbound-only REST, `Run()` is `<-ctx.Done()`
- `mattermost_bot.go` (71 lines) — outbound-only REST, `Run()` is `<-ctx.Done()`
- Both implement `Platform` interface: `Name()`, `Run(ctx, handler)`, `SendReply(ctx, out)`

### WebSocket Library

Use `nhooyr.io/websocket` — stdlib-compatible, no CGO, supports both client and server, context-native API. Preferred over `gorilla/websocket` which is in maintenance mode. Single dependency for both adapters.

### Shared Infrastructure: `internal/wsconn`

Extract a small WebSocket helper used by both adapters:
- Connect with configurable URL and headers
- Read JSON frames in a loop
- Write JSON frames (thread-safe)
- Heartbeat/ping-pong at configurable interval
- Reconnect with exponential backoff (base 2s, max 60s, jitter)
- Context-based shutdown

### Discord Gateway Adapter

- WebSocket to `wss://gateway.discord.gg/?v=10&encoding=json`
- Identify payload with bot token on connect
- Heartbeat at server-requested interval (`hello.heartbeat_interval`)
- Resume on disconnect (session_id + sequence tracking)
- Receive `MESSAGE_CREATE` dispatch events → extract user/channel/text → dispatch to `MessageHandler`
- Keep existing REST `SendReply` unchanged

### Mattermost WebSocket Adapter

- WebSocket to `{base_url}/api/v4/websocket`
- Auth challenge-response with bearer token
- Receive `posted` events → parse JSON post payload → dispatch to `MessageHandler`
- Reconnect with exponential backoff (base 2s, max 60s, jitter ±20%)
- Keep existing REST `SendReply` unchanged

### Not in Scope

- Discord voice channels, Opus codec, RTP (the Python reference has ~200 lines of voice capture — separate future phase if ever needed)
- Discord slash commands / interactions (can be added incrementally)
- Mattermost file uploads (existing REST covers text)

**Dependencies added:** 1 (WebSocket library)

**Deliverables:**
- `internal/wsconn/` — shared WebSocket connection helper
- `gateway/platforms/discord_gateway.go` — replaces or extends `discord_bot.go`
- `gateway/platforms/mattermost_ws.go` — replaces or extends `mattermost_bot.go`

---

## Phase 3: Protobuf + OTLP/HTTP Trace Export

**Goal:** Implement an OTLP/HTTP exporter that plugs into the existing 2-method `Exporter` interface.

### Current State

- Tracing package (520 lines): `Span`, `Tracer`, `Exporter` interface
- Built-in exporters: Noop, Memory, JSONLines
- `Exporter` interface:
  ```go
  type Exporter interface {
      Export(span *Span)
      Shutdown(ctx context.Context) error
  }
  ```
- No protobuf anywhere in the project

### Protobuf Approach

Vendored minimal proto: only the OTLP trace proto definitions (~3 files from `opentelemetry-proto`), run `go generate` once, check in generated `.pb.go` files. No runtime `protoc` dependency for builds.

Required protos:
- `opentelemetry/proto/common/v1/common.proto`
- `opentelemetry/proto/resource/v1/resource.proto`
- `opentelemetry/proto/trace/v1/trace.proto`
- `opentelemetry/proto/collector/trace/v1/trace_service.proto`

### OTLPHTTPExporter

- Implements `Exporter` interface
- Converts internal `*Span` → OTLP `ExportTraceServiceRequest` protobuf
- Batches spans in memory, flushes on interval (5s) or buffer capacity (256 spans)
- POST to `{endpoint}/v1/traces` with `Content-Type: application/x-protobuf`
- Retry with exponential backoff on transient failures (429, 503)
- `Shutdown()` flushes remaining buffer and closes HTTP client
- Configurable: endpoint URL, batch size, flush interval, headers (for auth tokens)

### Not in Scope

- OTLP/gRPC transport (HTTP is sufficient, avoids gRPC dependency)
- Metrics export via OTLP (Prometheus `/metrics` endpoint already works)
- Log export via OTLP
- Span sampling or filtering (all spans exported; add later if volume is a problem)

**Dependencies added:** 1 (`google.golang.org/protobuf`)

**Deliverables:**
- `proto/` — vendored OTLP proto definitions + generated Go code
- `tracing/otlp_http.go` — OTLP/HTTP exporter implementation
- `tracing/otlp_http_test.go` — tests against a mock HTTP server

---

## Phase 4: MTProto (Telegram User Accounts)

**Goal:** Enable Telegram user account login alongside the existing bot API polling adapter.

### Important Context

The Python reference does NOT have a full MTProto implementation. Its `telegram_network.py` (248 lines) is only a DNS-over-HTTPS fallback transport for network-restricted environments — still uses Bot API tokens. This phase is greenfield.

### MTProto Library

Use `github.com/gotd/td` — actively maintained, pure Go, no CGO, handles TL serialization internally.

### New Adapter: `telegram_user.go`

- Implements `Platform` interface (Name/Run/SendReply)
- `Name()` returns `"telegram_user"` (distinct from `"telegram"` bot adapter)
- First-time auth flow: phone number → code → optional 2FA password
- Session file persistence (`~/.hermes/telegram_session.json` or similar) to avoid re-auth on restart
- `Run()`: connect to Telegram, receive updates via MTProto, dispatch `MESSAGE_CREATE`-equivalent events to `MessageHandler`
- `SendReply()`: send messages via `messages.sendMessage` MTProto RPC

### Existing Adapter Unchanged

`telegram.go` (bot polling, 139 lines) remains as-is. Users choose one or both via config:
```yaml
gateway:
  telegram:
    token: "bot123:ABC"        # bot adapter
  telegram_user:
    phone: "+1234567890"       # user adapter
```

### DNS Fallback Transport

Port the Python DoH fallback (248 lines) as an optional HTTP transport for both bot and user adapters:
- DNS-over-HTTPS via Google/Cloudflare to resolve alternative Telegram IPs
- Fallback IP pool from 149.154.160.0/20
- Sticky IP selection after successful connection
- Useful in network-restricted environments

### Not in Scope

- Voice calls, video calls, secret chats
- File/media upload beyond text (add incrementally)
- Group admin features, channel management
- Userbot automation features (this is for receiving/sending messages, not scripting)

**Dependencies added:** 1 (`github.com/gotd/td`)

**Deliverables:**
- `gateway/platforms/telegram_user.go` — MTProto user account adapter
- `gateway/platforms/telegram_doh.go` — DNS-over-HTTPS fallback transport (shared by both adapters)

---

## Phase 5: Camofox Browser (Local Anti-Detection)

**Goal:** Add a local Camofox browser provider alongside the existing Browserbase cloud provider.

### Current State

- Browser tool package (522 lines) with clean `Provider` interface:
  ```go
  type Provider interface {
      Name() string
      IsConfigured() bool
      CreateSession(ctx context.Context) (*Session, error)
      CloseSession(ctx context.Context, id string) error
      LiveURL(ctx context.Context, id string) (string, error)
  }
  ```
- Only Browserbase implemented (cloud, 140 lines)
- Sessions return CDP WebSocket URL for downstream tools/MCP

### New Provider: `camofox.go`

- Implements existing `Provider` interface
- HTTP REST client to self-hosted Camofox server (default `http://localhost:9377`)
- `Name()` → `"camofox"`
- `IsConfigured()` → check if `browser.camofox.base_url` is set in config
- `CreateSession()` → POST to Camofox API, return session with CDP connect URL
- `CloseSession()` → DELETE/close session via API
- `LiveURL()` → return VNC debug URL if available
- Managed persistence option: reuse browser profiles per user ID, matching Python's `browser.camofox.managed_persistence` config

### Not in Scope

- Bundling or managing the Camofox server process from Go (user runs it via `docker run` or `npm start`)
- Accessibility snapshot pagination (add if needed)
- Anti-detection tuning (Camofox server handles this)

**Dependencies added:** 0 (pure HTTP client calls to external REST API)

**Deliverable:** `tool/browser/camofox.go` — Camofox provider implementation

---

## Phase 6: RL Training (Go↔Python Interop)

**Goal:** Go management layer for RL training, delegating actual training execution to the Python/Tinker-Atropos infrastructure.

### Design Decision

Go is a management layer, not a training runtime. The ML ecosystem is Python-native. Rewriting training logic in Go gains nothing.

### New CLI Subcommand: `hermes rl`

| Subcommand | What it does |
|------------|-------------|
| `hermes rl list` | Scan `tinker-atropos/environments/` for BaseEnv subclasses via regex |
| `hermes rl config` | Read/write YAML config with locked infra settings + editable hyperparameters |
| `hermes rl start` | Launch Python training subprocess with managed lifecycle |
| `hermes rl status` | Query WandB API (HTTP) for run metrics |
| `hermes rl stop` | Signal subprocess termination (SIGTERM → SIGKILL timeout) |

### Tool Functions

Same 7 functions from Python registered in the tool system, callable by the agent:
- `rl_list_environments`
- `rl_select_environment`
- `rl_get_current_config`
- `rl_edit_config`
- `rl_start_training`
- `rl_check_status`
- `rl_stop_training`

### Subprocess Management

- Launch via `os/exec.CommandContext` with Python entrypoint
- Stdout/stderr piped to structured logs (slog)
- Graceful shutdown: SIGTERM, wait 30s, SIGKILL
- PID tracking persisted to SQLite for status checks across restarts

### Config Management

- Locked infrastructure settings (tokenizer, rollout server, max workers) — not editable via CLI
- Editable hyperparameters (LoRA rank, learning rate, checkpoint interval) — validated before training starts
- Config stored as YAML alongside tinker-atropos directory

### Not in Scope

- Porting actual training loops to Go
- Bundling Python dependencies (user must have Python + tinker-atropos installed)
- WandB write operations (read-only status queries only)
- GPU management or CUDA setup

**Dependencies added:** 0 Go dependencies. External requirement: Python environment with tinker-atropos, atroposlib, wandb installed.

**Deliverables:**
- `cli/rl.go` — CLI subcommand group
- `tool/rl/` — 7 tool functions for agent use
- `rl/manager.go` — subprocess lifecycle management
- `rl/config.go` — config read/write/validate
- `rl/wandb.go` — WandB HTTP status queries
