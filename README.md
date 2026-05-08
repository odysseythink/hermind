# hermind

A Go port of the hermes AI agent framework. Single-binary, instance-bound, single-conversation per directory. Ships a web UI, REST API, SSE streaming, multi-provider LLM routing, skills, memory replay, and an Anthropic-compatible `/v1/messages` proxy.

> **Status.** Pre-1.0 (`v0.3.x`). Schema and CLI surface are still moving. See [CHANGELOG.md](CHANGELOG.md).

[中文版本](README.zh-CN.md) · [Design notes](DESIGN.md) · [Changelog](CHANGELOG.md)

---

## What it does

hermind runs an LLM agent locally. Point it at any current working directory and the directory becomes a hermind instance — its config, conversation history, skills, and trajectories all live in `./.hermind/`. One directory, one conversation, one persistent state.

The agent loop is provider-agnostic. Configure one of the supported providers (Anthropic, OpenAI, Bedrock, OpenRouter, Qwen, Kimi, DeepSeek, Minimax, Wenxin, Copilot) as the primary, optionally another as auxiliary, and hermind handles tool use, multi-turn reasoning, compression, fallback chaining, and structured trajectory logging.

Run it as:

- A **chat web app** in your browser via `hermind web`
- A **CLI agent** for one-off prompts via `hermind run`
- A **background worker** that polls scheduled prompts (`hermind cron`) or IM platforms (Telegram, Feishu via `hermind listen`)
- A **drop-in Anthropic API proxy** so Claude Code, Cursor, and the official SDK can talk to whatever provider you have configured (set `proxy.enabled: true` then point `ANTHROPIC_BASE_URL` at hermind)

## Quick start

```bash
# Build
go build -o bin/hermind ./cmd/hermind

# Configure a provider (writes ./.hermind/config.yaml)
./bin/hermind auth set anthropic sk-ant-...

# Start the web UI (binds to 127.0.0.1 on a random port in [30000,40000))
./bin/hermind web
```

Open the printed URL in a browser. First run scaffolds `./.hermind/config.yaml` with sensible defaults; everything else is editable from the Settings panel.

For a one-shot CLI run:

```bash
./bin/hermind run "summarize the changes in the last 5 commits"
```

## Capabilities

**Multi-provider LLM routing.** First-class support for Anthropic, OpenAI, AWS Bedrock, GitHub Copilot, OpenRouter, plus several Chinese-market providers (Qwen, Kimi, DeepSeek, Minimax, Wenxin, Zhipu). Configurable fallback chain when a provider returns rate-limit or quota errors.

**Skills.** Hot-loadable skill packages from `<instance>/skills/`. The agent auto-injects up to N retrieved skills per turn (configurable). Optionally extracts new skill snippets from each conversation (`auto_extract`). Memory reinforcement signals decay over skill-library generations (`generation_half_life`).

**Memory replay buffer.** `hermind bench replay {generate,run,judge,report}` re-runs real historical user turns from `state.db` against the current configuration in an isolated temp sqlite. Three judge modes (`none` / `pairwise` / `rubric+pairwise`); two extraction modes (`cold` / `contextual`). Replay never mutates the operator's `state.db`.

**Anthropic `/v1/messages` proxy.** Opt-in transport-layer proxy. Hermind translates Anthropic Messages requests into its internal provider abstraction, dispatches to whichever LLM you have configured, and translates the response back — including SSE streaming and tool-use blocks. Useful when you want Claude-Code-class clients to drive a non-Anthropic model.

**User-presence framework.** Three-state (Unknown / Absent / Present) gate for background workers. HTTP-idle source (votes Absent after N seconds without inbound requests) and sleep-window source (votes Absent during a configured local-clock window) ship today; pluggable for future keyboard-idle and calendar-busy signals.

**MCP servers.** Configure any number of MCP servers under Settings → MCP. Tool calls are dispatched through the conversation engine like first-class tools.

**Browser automation.** Browserbase or Camofox provider for real-browser tool use.

**Cron.** YAML-defined scheduled prompts (`every 5m`, `every 1h`, etc.). Each cron run is ephemeral — it logs to its own trajectory file and never pollutes the main conversation.

**Multi-platform IM gateway.** Telegram, Feishu adapters (more in flight). Configure under Settings → IM Channels; hermind long-polls or webhooks each platform, routes incoming messages through the conversation engine, replies in-thread.

**Bench harness.** `hermind bench` runs synthetic A/B evaluations across config presets with deterministic dataset generation. Replay (above) shares the same runner via a pluggable `Item` interface.

**Web UI.** React + Vite SPA bundled into the binary. Settings panel is descriptor-driven — every field in `config.yaml` is editable from the browser, with live `visible_when` gating, dotted-path nested forms, and i18n (English + Simplified Chinese).

## Project layout

| Path | What lives there |
|------|------------------|
| `cmd/hermind` | Single binary entrypoint |
| `cli/` | All cobra subcommands (`web`, `run`, `bench`, `replay`, `skills`, `cron`, `listen`, …) |
| `agent/` | Conversation engine, batch runner, idle consolidator, presence framework |
| `provider/` | LLM provider implementations (anthropic / openai / bedrock / qwen / …) |
| `tool/` | Built-in tool implementations (file, web, browser, mcp, memory, terminal, …) |
| `skills/` | Skill loader and registry |
| `replay/` | Memory replay buffer (dataset generation, runner, judge, report) |
| `benchmark/` | Synthetic A/B benchmark harness |
| `gateway/` | Multi-platform IM adapters |
| `mcp/` | MCP server + client integration |
| `storage/` | SQLite + Postgres backends for `state.db` |
| `config/` | Config loader + descriptor registry that drives the web UI schema |
| `api/` | HTTP server, REST endpoints, SSE streaming, embedded web assets |
| `web/` | React + Vite frontend (built into `api/webroot/` at release time) |

## Configuration

Config lives at `./.hermind/config.yaml`. Override the location with `HERMIND_HOME=/some/dir`. Every field has a corresponding entry in the web UI's Settings panel — there's no need to edit YAML directly for routine changes.

Sensitive values (API keys, tokens) can be provided either inline in `config.yaml` or via environment variables (`ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, `BROWSERBASE_API_KEY`, etc.). Env vars take precedence at load time.

## Development

```bash
# Backend
go build ./...
go test ./...

# Frontend
cd web && npm install && npm run dev   # vite dev server with /api proxy to 127.0.0.1:9119
cd web && npm test -- --run            # vitest run-once

# Build the embedded SPA
cd web && npm run build                # writes to api/webroot/

# Regenerate the config-schema fixture (after adding/changing descriptors)
go test -tags fixture ./api -run TestDumpSchemaFixture
```

The `Makefile` aggregates common workflows. `flake.nix` provides a Nix-based dev shell.

## License

See LICENSE in the repository root.
