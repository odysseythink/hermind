# Changelog

## [0.2.0] - 2026-04-22

### Added

- **Per-session settings drawer**: a new gear button in the top-right of
  the conversation header opens a right-side drawer where you set the
  model and the system prompt for that session. Changes apply from the
  next turn. Includes draft state, Cancel/Save semantics, Esc to cancel,
  a conflict banner when another tab edits the same session, and a
  Tab/Shift+Tab focus trap so keyboard users cannot escape the dialog.
- **`config.agent.default_system_prompt`**: a new YAML field (rendered
  as a textarea in `/settings/agent`) that every new conversation
  inherits. The prompt is appended after the baked-in Hermind identity
  block, before any active-skill blocks.
- **`FieldText` descriptor kind**: multi-line string variant of
  `FieldString`. Wires through the `/api/config/schema` JSON, the TS
  `ConfigFieldKindSchema` enum, and a new `TextAreaInput` field
  component.
- **`PATCH /api/sessions/{id}`**: extended to accept optional `model`,
  `system_prompt`, and `title` pointer fields. Size caps:
  `MaxSessionTitleBytes=256`, `MaxSystemPromptBytes=32KB`,
  `MaxModelNameBytes=128`. Body is capped via `http.MaxBytesReader`
  before JSON decode. Emits a new `session_updated` SSE event on success
  so open tabs stay in sync.
- **`SessionSummary.system_prompt`** and a new `GET /api/sessions/{id}`
  hydrate the drawer with the current session's prompt.

### Changed

- **New sessions no longer splice the first user message into the
  stored `SystemPrompt`**. The stored value is the configured default
  only, frozen at session creation. `Title` derivation from the first
  message is unchanged. Existing session rows with the historical
  concatenation are not migrated.
- **`RunConversation` prefers `Session.Model`** over `RunOptions.Model`
  whenever the session row carries a non-empty value. This lets users
  switch models mid-conversation via PATCH and have the next turn honor
  the new choice.
- **Top-right model `<select>` replaced by a gear button** in
  `ConversationHeader`. Model is now a session attribute, not a
  composer-local ephemeral state.

### Removed

- **`model` field on `POST /api/sessions/{id}/messages`**. Model is set
  once per session via PATCH and read from the session row on every
  turn. `composer.selectedModel` state and the `chat/composer/setModel`
  reducer action are gone.
- **`ModelSelector` component**. Superseded by `SettingsButton` +
  `SessionSettingsDrawer`.

### Fixed

- **Session-id URL decoding on every `/api/sessions/{id}` endpoint**.
  chi's URL param returns the still-percent-encoded path segment, so
  session ids that contain `:` (all Telegram sessions use the format
  `telegram:<chat_id>`) were being looked up as
  `telegram%3A<chat_id>` in the database, returning 404 Not Found even
  when the row existed. Added a `sessionIDParam` helper that wraps
  `url.PathUnescape` and routed every session-id handler through it —
  GET, PATCH, messages, cancel, SSE, WS. This was a pre-existing bug
  surfaced for the first time when the new settings drawer tried to
  PATCH a Telegram session. SSE/WS subscribers using encoded URLs now
  also receive events correctly.

## Unreleased

### Added

- **Web chat frontend**: React chat workspace is now the default
  landing mode at `#/chat`. Config groups moved to `#/settings`,
  reached via a TopBar toggle. Features: session list sidebar with
  new-conversation button, conversation header with model selector,
  message list with markdown + KaTeX + shiki code highlighting +
  lazy-loaded Mermaid, streaming assistant bubble with
  token-throttled (rAF) updates, tool-call cards with expandable
  input/result, composer with Shift+Enter-newline and slash-menu
  (`/new`, `/clear`, `/settings`, `/model`), Stop button during
  streaming, error toasts for 409 busy / 503 no-provider, automatic
  message-history fetch on session select. Subscribes to the existing
  `/api/sessions/:id/stream/sse` channel via `useChatStream`.
- Hash router: `#/chat[/:id]` and `#/settings/:group[/:sub]`; legacy
  `#<group>` and `#<group>/<sub>` hashes auto-canonicalize to
  `#/settings/...`. `parseHash` returns a discriminated
  `{mode,…}` union.

### Dependencies

- Added: `react-markdown`, `remark-gfm`, `remark-math`, `rehype-katex`,
  `katex`, `shiki`, `mermaid`. Bundle grew from ~350KB to ~1MB (~300KB
  gzipped) — mostly shiki grammars and KaTeX fonts. Dynamic imports
  for language grammars are a follow-up optimization if bundle budget
  becomes a concern.

- **Web chat backend**: `POST /api/sessions/{id}/messages` accepts a
  user message, spawns a per-request `agent.Engine` in a goroutine, and
  streams status/token/tool_call/tool_result/message_complete events
  through the existing `StreamHub` (SSE + WS). Returns 202 on accept,
  409 when the session is already running, 503 when no provider is
  configured. `POST /api/sessions/{id}/cancel` ctx-cancels the running
  engine (204 on success, 404 on not-running). An in-memory
  `SessionRegistry` tracks per-session cancel funcs. New
  `api/sessionrun` package hosts the reusable `Run(ctx, Deps, Request)`
  that both the web path and (eventually) other callers share.
  `cli.BuildEngineDeps` consolidates provider/tool/skills construction
  for the web path; TUI keeps its inlined copy until Plan 5 removes the
  TUI altogether.
- **Multi-provider `web_search`**: DuckDuckGo (keyless fallback),
  Tavily, and Brave Search joined Exa. Provider chosen via the new
  `web.search.provider` config field or auto-selected by priority
  (Tavily → Brave → Exa → DuckDuckGo). New top-level `web:` config
  section with `search.provider` and `search.providers.{tavily,brave,exa}.api_key`
  fields. Environment variables `EXA_API_KEY`, `TAVILY_API_KEY`, and
  `BRAVE_API_KEY` continue to work as fallback when the config field
  is empty. 60s in-memory LRU cache (max 128 entries) for repeated
  queries. New dep: `github.com/PuerkitoBio/goquery` (MIT).
- **Frontend i18n (中 / EN)**: Web UI now supports English and Simplified Chinese via `react-i18next`. Language toggle in TopBar (right of status); default follows `navigator.language`, manual choice persisted in `localStorage`. Descriptor labels/help from the backend are overlaid by per-locale JSON (`web/src/locales/{en,zh-CN}/descriptors.json`) with fallback to the backend English literal. CI guards translation completeness via a Go-generated fixture (`api/fixture_gen_test.go`, build tag `fixture`) plus a vitest completeness test.

### Known limitations

- Platform descriptor labels (`/api/platforms/schema`) are not yet covered by the fixture; platform-specific field labels fall back to backend English.
- Enum option values in dropdowns render as the raw canonical string (e.g. `browserbase`); only the field **label** is translated. Full enum-option translation is a follow-up.
- `CHANGELOG.md` and server-side error messages remain English.

### Breaking

- **TUI removed.** The bubbletea chat interface (`cli/ui/`) and
  bubbletea config editor (`cli/ui/config/`, `cli/ui/webconfig/`) are
  gone. `hermind` and `hermind run` now launch the web UI and open the
  browser (equivalent to `hermind web`). Configuration lives in the
  Settings panel of the web UI — the standalone `hermind config`
  subcommand is removed. Headless usage: `hermind web --no-browser`
  plus an SSH tunnel to the bound port.
  charmbracelet dependencies (bubbletea, bubbles, lipgloss, glamour)
  dropped from `go.mod`.

- **Feishu platform (`feishu`) switched from one-way bot webhook to
  self-built app over long-connection.** The `webhook_url` option is
  removed. Replace it with `app_id`, `app_secret`, `domain`, and
  optionally `encrypt_key` / `default_chat_id`. Recreate your Feishu bot
  as a self-built app in the Open Platform console (see
  `docs/smoke/feishu-app.md`). On startup, any `feishu` instance still
  carrying `webhook_url` without `app_id` will fail with a migration
  error.
