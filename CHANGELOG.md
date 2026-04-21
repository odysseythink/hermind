# Changelog

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
