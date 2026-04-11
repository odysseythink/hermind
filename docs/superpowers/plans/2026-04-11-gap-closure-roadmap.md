# Hermes-Agent-Go Gap Closure Roadmap

**Date:** 2026-04-11
**Status:** Planning
**Context:** Plans 1-9 shipped the foundation + feature matrix through the eighth external memory provider and the tenth gateway platform. A horizontal comparison against `hermes-agent-2026.4.8` places the Go port at ~35-45% feature coverage. This roadmap captures the remaining gaps as 14 coherent follow-up plans, ordered by business value and dependency.

---

## Guiding Principles

1. **No single mega-plan.** Each entry below is sized to be written + executed in one focused session (~500-1500 LoC, 4-8 tasks).
2. **Prefer the path that unlocks the most user-visible features.** Skills marketplace and CLI UX rank above low-level cleanups.
3. **Trust existing plumbing.** Many items just need a new file under `tool/` or `cli/` that wires a Python script into the existing Go skeleton.
4. **Zero or minimal new deps.** We've kept the module under a dozen direct deps; target plans avoid WebSocket / proto / gRPC libraries unless strictly necessary.
5. **Deferred hard items stay deferred.** Things like the MTProto user account, full OTLP proto export, and the RL training harness are explicitly NOT in this roadmap — they each need a dedicated session.

---

## Phase Summary

| Phase | Plans | Theme | Approx LoC | Unlocks |
|---|---|---|---|---|
| **10 — Skills core** | 10a, 10b, 10c | Skills marketplace mechanism | ~2000 | `hermes skills` + `/slash` commands + skill registry |
| **11 — CLI UX** | 11a, 11b, 11c | Onboarding + maintenance | ~1800 | `setup`, `auth`, `doctor`, `profiles`, `models`, `config` wizard |
| **12 — Agent depth** | 12a, 12b | Agent features Python ships | ~1500 | trajectory, insights, title_generator, usage_pricing, smart routing, credential pool, redact |
| **13 — Multimedia tools** | 13a, 13b | image / tts / stt / voice | ~1200 | vision generation, speech synthesis, transcription |
| **14 — Security tools** | 14 | defensive toolset | ~800 | osv_check, url_safety, website_policy, mcp_oauth |
| **15 — Agent meta tools** | 15 | todo / clarify / checkpoint / session_search / approval | ~900 | richer agent self-management |
| **16 — Gateway depth** | 16a, 16b | hooks, mirror, pairing, sticker_cache, channel_directory | ~1500 | full Python Gateway parity (minus MTProto) |
| **17 — ACP full** | 17 | full ACP server + permissions + events + registry | ~900 | true ACP compliance instead of the Plan 8 stub |
| **18 — Cron persistence** | 18 | crontab-spec grammar + DB-backed job history | ~700 | feature-parity with Python cron |
| **19 — Packaging** | 19 | Dockerfile, nix flake, `hermes upgrade` | ~400 | full distribution coverage |

**Aggregate:** ~12000 additional LoC across **20 plans**, each individually executable. Completing Phase 10 + 11 alone lifts the Go port from ~40% → ~65% coverage and closes the two highest-impact gaps.

---

## Phase 10: Skills Marketplace

The Python reference's **biggest** differentiator is the `skills/` tree and the runtime that loads them as dynamic tool packs + slash commands. It is 0% covered in Go today.

### Plan 10a — Skill runtime core
- Add `skills/` Go package: `Skill` struct (name, description, path, commands, tools), `Loader` that walks a `skills_home` directory and parses `skill.yaml` / `skill.json`, `Registry` keyed by name.
- Skill files live in `~/.hermes/skills/<category>/<name>/` matching the Python layout.
- No network — pure local filesystem walk + YAML decode.
- Deliverable: `hermes skills list` prints discovered skills.

### Plan 10b — Slash command dispatcher
- Extend the REPL so an input starting with `/` is parsed as a command name and routed to a registered handler from the skill registry.
- Built-in slash commands: `/help`, `/skills`, `/reset`, `/memory`, `/model`, `/profile`, `/doctor`.
- Skills may register additional `/foo` handlers that shell out to a subagent run with a pre-filled prompt.
- Deliverable: typing `/reset` in the REPL clears the current session.

### Plan 10c — Skill tool injection
- When a skill is "active" (selected for a session), its declared tool file names in `skill.yaml` are registered into the current tool registry alongside the built-ins.
- Skill-provided prompts get injected into the system prompt builder.
- `hermes skills enable <name>` / `disable <name>` persists the choice in the profile.
- Deliverable: enabling a skill exposes its tools to the model for the next turn.

**Out of scope for Phase 10:** remote skill sync (`skills_hub`, `skills_sync`) — becomes Plan 10d if needed.

---

## Phase 11: CLI UX Completion

Bring the `hermes <subcommand>` surface closer to the Python CLI so onboarding doesn't require hand-editing YAML.

### Plan 11a — Setup wizard + doctor
- `hermes setup`: interactive TUI that walks through provider selection, API key entry, terminal backend choice, storage path, and writes `~/.hermes/config.yaml`.
- `hermes doctor`: runs health checks — config file exists and parses, provider credentials resolve, storage opens, MCP servers start, tool registry populates, terminal backend executes a `true` command.
- Both reuse existing `config`, `provider/factory`, `storage/sqlite`, `tool/mcp`, `tool/terminal` packages.

### Plan 11b — Auth + model management
- `hermes auth <provider>`: store API keys in a keyring-style local credential file (`~/.hermes/credentials.yaml`, 0600 perms).
- `hermes models list`: query provider.ModelInfo for each configured provider and print the catalog.
- `hermes model switch <model-id>`: rewrite `model:` in config.yaml.
- `hermes auth revoke <provider>`: wipe the key entry.

### Plan 11c — Profiles + plugins
- `hermes profile list/create/switch/delete`: each profile is a separate subdirectory under `~/.hermes/profiles/<name>/` with its own config + storage + memory.
- Environment variable `HERMES_PROFILE` overrides the active profile.
- `hermes plugins list/enable/disable`: enumerate installed skills (Phase 10 output) and flip their activation.
- Depends on Plan 10 for skills; can ship in parallel for profiles.

---

## Phase 12: Agent Depth

Features the Python agent uses internally but which the Go port doesn't have yet. These are what make a long session feel "intelligent" instead of a dumb prompt loop.

### Plan 12a — Trajectory + insights + title
- `agent/trajectory.go`: records (tool call → result → assistant block) tuples, dumps JSON-lines on session end to `~/.hermes/trajectories/<session-id>.jsonl`.
- `agent/insights.go`: aggregates totals — tool call counts, duration histograms, most-used tools, errors.
- `agent/title.go`: calls the aux provider once with "summarize this conversation in 5 words" after N messages, updates `sessions.title`.

### Plan 12b — Usage pricing + smart routing + credential pool + redact
- `agent/pricing.go`: multiplies per-token usage by a per-model rate table, persists to the existing sessions cost columns.
- `agent/smart_routing.go`: a simple rule table (task-kind → preferred model) that can swap the model before the provider call based on detected intent (code, math, creative, etc.).
- `agent/credential_pool.go`: round-robin across multiple API keys for the same provider to survive rate limits.
- `agent/redact.go`: regex + structural pass that scrubs keys/tokens/emails from messages before logging or writing trajectories.

---

## Phase 13: Multimedia Tools

The Python reference ships rich multimedia support. Each sub-item below adds ~200-400 LoC and can land independently.

### Plan 13a — Image generation + transcription
- `tool/image/`: new `image_generate` tool hitting OpenAI `/v1/images/generations`, Anthropic `vision:generate` (or Stability fallback). Saves to `~/.hermes/cache/images/` and returns the path.
- `tool/transcribe/`: new `transcribe_audio` tool hitting OpenAI Whisper `/v1/audio/transcriptions` with a local file path.

### Plan 13b — TTS + voice mode
- `tool/tts/`: new `speak` tool using OpenAI TTS `/v1/audio/speech` (model: `tts-1`) that saves MP3 or plays via `afplay`/`aplay`.
- `cli voice`: optional voice-input REPL mode — press space, record, transcribe, submit. Gated behind a `--voice` flag. Uses the same transcribe tool under the hood.

---

## Phase 14: Security Tools

Defensive-only tools that protect the agent and its outputs. Everything here is fail-closed.

### Plan 14 — osv_check + url_safety + website_policy + mcp_oauth
- `tool/security/osv.go`: `osv_check` tool queries `https://api.osv.dev/v1/query` with a package name + version and returns CVEs.
- `tool/security/url_safety.go`: `url_check` inspects a URL against Google Safe Browsing (or a local blocklist if no API key).
- `tool/security/website_policy.go`: middleware used by `web_fetch` / browser tools that enforces an allowlist/denylist from config.
- `tool/mcp/oauth.go`: optional OAuth 2.1 client-credentials flow for MCP servers that require it.

---

## Phase 15: Agent Meta Tools

Tools that change how the model manages its own work — small but high leverage.

### Plan 15 — todo + clarify + checkpoint + session_search + send_message + approval
- `tool/meta/todo.go`: structured TODO list stored in the current session (existing storage).
- `tool/meta/clarify.go`: lets the model request user clarification instead of hallucinating ("clarify" pauses the REPL and waits for input).
- `tool/meta/checkpoint.go`: snapshot file tree + session history to `~/.hermes/checkpoints/` and restore.
- `tool/meta/session_search.go`: FTS over the existing `messages` / `memories` tables (already indexed; just needs a handler).
- `tool/meta/send_message.go`: post a reply via any configured gateway platform from within a tool call.
- `tool/meta/approval.go`: gate destructive tool calls behind an interactive yes/no — similar to git pre-commit hook UX.

---

## Phase 16: Gateway Depth

Everything in `hermes-agent-2026.4.8/gateway/` outside the platform adapters.

### Plan 16a — hooks + builtin_hooks + channel_directory
- `gateway/hooks.go`: pre-dispatch and post-dispatch hook interfaces. Hooks can mutate incoming/outgoing messages, drop them, or emit metrics.
- `gateway/builtin_hooks/`: rate limit, profanity filter, command allowlist, user-ban.
- `gateway/channels.go`: `/channel-directory` style listing + config UI for multi-channel setups.

### Plan 16b — mirror + pairing + sticker_cache
- `gateway/mirror.go`: cross-post a message from one platform to another (e.g., Telegram → Discord copy).
- `gateway/pairing.go`: one-time pairing tokens for linking a platform user to a hermes profile.
- `gateway/sticker_cache.go`: deduplicates + caches sticker / image attachments so providers see them by hash instead of URL.

**Explicit out-of-scope:** `telegram_network` (MTProto user account) and `stream_consumer.py` (the latter is a Python-specific asyncio glue).

---

## Phase 17: ACP Full Protocol

The current `gateway/platforms/acp.go` is a one-endpoint stub. The full Python `acp_adapter/` is a 9-file implementation with permissions, events, and a public `.well-known` registry.

### Plan 17 — Full ACP compliance
- `gateway/acp/server.go`: full ACP HTTP surface (create session, stream events, list tools, execute tool).
- `gateway/acp/permissions.go`: permission model (read, write, delete, execute) per client.
- `gateway/acp/events.go`: server-sent events for tool progress and stream deltas.
- `gateway/acp/tools.go`: ACP-native tool envelope translation.
- `gateway/acp/registry/`: `.well-known/agent.json` served over the same HTTP server.
- Replaces the old `gateway/platforms/acp.go` with a thin wrapper that delegates.

---

## Phase 18: Cron Persistence + Grammar

Python's `cron/scheduler.py` supports crontab strings and persists job state to the DB. The Go version is interval-only and in-memory.

### Plan 18 — crontab grammar + DB-backed jobs
- `cron/crontab.go`: parser for the standard 5-field syntax (`* * * * *`) and `@hourly` / `@daily` shortcuts. Keep the existing `every Nm` shim as a synonym.
- `cron/storage.go`: a new `cron_runs` SQLite table keyed by (job_name, started_at) with duration, status, error. Integrates with existing `storage/sqlite`.
- CLI: `hermes cron list`, `hermes cron run <name>` (manual trigger), `hermes cron history <name>`.

---

## Phase 19: Packaging Completion

### Plan 19 — Docker, nix, upgrade
- `Dockerfile` at repo root: multi-stage build, final image ~25 MB on alpine.
- `flake.nix` + `flake.lock`: nix derivation producing the same binary via nixpkgs Go toolchain.
- `hermes upgrade` subcommand: query GitHub Releases API, download the matching archive for the host arch, replace `os.Executable()` atomically, print new version. Reuses `scripts/install.sh` logic in Go.

---

## Execution Order (Recommended)

If we execute top-to-bottom, the port goes from ~40% → ~85% coverage. **Phases are independent** after 10a/11a, so they can also be parallelized across sessions.

```
Phase 10 (Skills)     ←  highest-value, unlocks slash commands + skill ecosystem
Phase 11 (CLI UX)     ←  onboarding fix; depends on 10 for `/help`, `/skills`
Phase 12 (Agent)      ←  makes long sessions feel smart (trajectory/insights/cost)
Phase 15 (Meta tools) ←  todo/clarify/checkpoint — cheap and highly visible in the REPL
Phase 18 (Cron)       ←  unblocks production cron use cases
Phase 16 (Gateway)    ←  after Phase 15 so we have send_message for cross-platform
Phase 17 (ACP)        ←  once gateway depth settles
Phase 13 (Multimedia) ←  relies on aux provider being stable
Phase 14 (Security)   ←  small, can slot in anywhere
Phase 19 (Packaging)  ←  last, because it's touch-and-forget
```

**Explicitly deferred (each needs its own dedicated multi-session project):**
- `telegram_network` — full MTProto stack
- OTLP/HTTP protobuf export — needs proto deps + OTel schema
- `rl_cli.py` + `tinker-atropos/` — RL training harness
- `landingpage/` + `website/` — product marketing site
- `batch_runner.py` / `mini_swe_runner.py` — research harnesses
- `browser_camofox` — stealth headless browser (needs a real browser process)
- `voice_mode` full UX (Phase 13b only covers single-shot TTS/STT)

---

## Decision Points

These are choices that should be made before executing the corresponding phase:

| Phase | Decision | Options |
|---|---|---|
| 10a | Skill file format | YAML (match Python) vs. TOML (idiomatic Go, tiny dep) |
| 11a | TUI library | Extend existing bubbletea install vs. plain stdin/stdout |
| 11b | Credential storage | Keyring (needs CGo / platform libs) vs. file-based 0600 |
| 13a | Image generation | OpenAI only vs. multi-provider fan-out |
| 13b | Audio playback | Shell out to platform tool vs. Go audio lib |
| 14 | URL safety source | Google Safe Browsing (needs key) vs. local blocklist |
| 16a | Hook registration | Static (compile-time) vs. dynamic (via skill registry) |
| 19 | Docker base | alpine (5 MB) vs. distroless vs. scratch |

---

## Tracking

As plans are executed, update this roadmap by ticking off the completed phase and linking to the commit SHA.

| Plan | Status | Commit | Notes |
|---|---|---|---|
| 10a — Skill runtime core | ⏳ planned | | |
| 10b — Slash dispatcher | ⏳ planned | | |
| 10c — Skill tool injection | ⏳ planned | | |
| 11a — Setup wizard + doctor | ⏳ planned | | |
| 11b — Auth + models | ⏳ planned | | |
| 11c — Profiles + plugins | ⏳ planned | depends on 10 | |
| 12a — Trajectory + insights + title | ⏳ planned | | |
| 12b — Pricing + routing + credpool + redact | ⏳ planned | | |
| 13a — Image gen + transcription | ⏳ planned | | |
| 13b — TTS + voice mode | ⏳ planned | | |
| 14 — Security tools | ⏳ planned | | |
| 15 — Meta tools | ⏳ planned | | |
| 16a — Hooks + channel directory | ⏳ planned | | |
| 16b — Mirror + pairing + sticker cache | ⏳ planned | | |
| 17 — ACP full protocol | ⏳ planned | | |
| 18 — Crontab grammar + DB history | ⏳ planned | | |
| 19 — Docker + nix + upgrade | ⏳ planned | | |

---

## Companion Files

When each phase begins, drop a full plan at `docs/superpowers/plans/2026-04-??-plan-<id>-<slug>.md` following the shape of the existing Plan 1-9b docs (Goal, Architecture, Tasks with checkboxes, Verification Checklist). This roadmap is the index; the individual plan files hold the TDD-grade implementation detail.
