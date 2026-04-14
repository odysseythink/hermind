# Hermind Config UI — Design

**Date:** 2026-04-14
**Status:** Approved for implementation planning

## Problem

`hermind` configuration lives in `~/.hermind/config.yaml`. Today users edit it
by hand or run `hermind setup`, an interactive CLI wizard that covers only a
handful of fields. There is no way to discover or edit the rest of the
configuration surface (providers, memory, MCP, browser, compression, etc.)
without reading the `config.Config` Go struct.

Goal: give users a graphical way — both in-terminal and in-browser — to see
and edit the configuration that really matters for day-to-day use.

## Scope

### In scope

- Core runtime settings: `model`, `providers` (CRUD on each provider entry
  with `api_key`, `base_url`, `model`, `fallback`), `terminal.backend`,
  `storage.sqlite_path`, `agent.max_turns`, `agent.compression.*`.
- External integrations: `memory` (provider selection + per-provider keys),
  `mcp` (add/remove MCP servers), `browser` (browserbase / camofox).
- A first-run TUI that replaces the implicit "missing config" failure.
- A `hermind config` subcommand with `--web` flag for the browser variant.

### Out of scope

- `gateway.platforms.*` (Discord/Slack/Telegram/Matrix/...) — per-platform
  schemas are large and vary; advanced users keep editing YAML directly.
- `cron.jobs` — list of prompts is better authored in YAML.
- `logging` / `metrics` / `tracing` — leave as YAML-only knobs.
- Hot reload of already-running components (providers, storage, MCP). Save
  prompts the user to restart `hermind`.

## Architecture

```
┌─────────────────────────────┐       ┌──────────────────────────────┐
│   hermind config   (TUI)    │       │  hermind config --web  (Web) │
│   bubbletea screen          │       │  net/http + embed.FS         │
└──────────────┬──────────────┘       └──────────────┬───────────────┘
               │                                     │
               └─────────────┬───────────────────────┘
                             ▼
                ┌──────────────────────────┐
                │   config/editor package  │  ← new
                │  • Load(path) *Doc       │
                │  • Doc.Get/Set/Save      │
                │  • Doc.Schema() []Field  │
                └──────────────┬───────────┘
                               ▼
                     ~/.hermind/config.yaml
```

Both UIs are thin renderers over a shared `config/editor` package. That
package owns:

- The YAML AST (preserves comments, ordering, blank lines on save).
- The `Schema() []Field` description that drives form generation.
- File I/O (atomic tmpfile + rename).

## Components

### `config/editor` (new)

```go
type Doc struct { root *yaml.Node; path string }

func Load(path string) (*Doc, error)
func (d *Doc) Get(dotPath string) (string, bool)
func (d *Doc) Set(dotPath string, value any) error
func (d *Doc) SetBlock(dotPath string, yamlFragment string) error
func (d *Doc) Remove(dotPath string) error
func (d *Doc) Save() error

type Kind int
const (
    KindString Kind = iota
    KindInt
    KindFloat
    KindBool
    KindEnum
    KindSecret
    KindList
)

type Field struct {
    Path     string
    Label    string
    Help     string
    Kind     Kind
    Enum     []string
    Validate func(any) error
    Section  string
}

func Schema() []Field
```

Implementation notes:

- Built on `gopkg.in/yaml.v3` Node API (already a dependency — no new libs).
- `Load` with missing file returns a Doc initialized from the commented
  template that `hermind setup` renders today, so first-run users get the
  full provider catalog.
- `Set` locates leaf nodes by dotPath, updates the scalar, preserves line
  and column comments. Missing intermediate maps are created.
- `SetBlock` is the escape hatch for adding a new map entry (e.g. a new
  provider) where a scalar Set is insufficient.
- `Save` writes to `<path>.tmp`, `fsync`s, then `os.Rename` — atomic on
  POSIX; Windows falls back to copy+delete consistent with the existing
  upgrade.go pattern.

### `cli/ui/config` (new, TUI)

bubbletea Model with a three-pane layout: sections (left) / fields (right) /
help + keybinding bar (bottom).

Keys: `↑↓` field, `tab` section, `enter` edit, `esc` cancel, `s` save, `a`
add list item, `d` delete list item, `q` quit with dirty-check prompt.

Field editors are small sub-models dispatched on `Field.Kind`:

- `string/int/float` → `bubbles/textinput`
- `bool` → toggle
- `enum` → `bubbles/list`
- `secret` → `bubbles/textinput` with mask; toggle to reveal
- `list` → nested pane listing items, each reusing the primitive editors

Public entry point: `func Run(path string) error` and
`func RunFirstRun(path string) error`. The latter pre-seeds a Doc from the
template when no file exists.

### `cli/ui/webconfig` (new, Web)

Backend (`server.go`, ~150 LOC):

```
GET  /              embedded index.html
GET  /api/schema    editor.Schema() as JSON
GET  /api/config    current values (secrets masked)
POST /api/config    { "path": "...", "value": ... } → Doc.Set + Validate
POST /api/save      Doc.Save()
POST /api/reveal    { "path": "providers.x.api_key" } → plaintext (for 👁)
POST /api/shutdown  stops the server
```

Binds `127.0.0.1:7777` by default; `--port N` overrides. No authentication
(explicit design choice — deployment model is single-user local machine).
Port conflict is a hard error with an actionable message. After `ListenAndServe`
starts, the server opens the browser via `open` (darwin) / `xdg-open` (linux) /
`rundll32 url.dll,FileProtocolHandler` (windows).

Frontend (`web/` directory, embedded via `go:embed`):

- `index.html`, `app.css`, `app.js` — plain HTML/CSS/vanilla JS, no build
  step. Keeps `go build` as the only tool in the chain.
- Left-rail section nav mirrors the TUI structure so users learn once.
- Field renderers match the schema `Kind`: `<input>`, `<select>`, checkbox,
  `<input type=password>` + reveal button for secrets.
- List sections (providers, MCP servers) render as cards with "+ Add" and
  per-card "Remove".

### Top-level integration

`cli/app.go` — first-run detection:

```go
cfg, err := config.Load(defaultPath)
if errors.Is(err, os.ErrNotExist) {
    fmt.Println("No config found — launching first-run setup...")
    if err := uiconfig.RunFirstRun(defaultPath); err != nil { return nil, err }
    cfg, err = config.Load(defaultPath)
}
```

`cli/config.go` — new subcommand:

- `hermind config` → `uiconfig.Run(path)`
- `hermind config --web` → `webconfig.Serve(path, port)`
- `hermind config --web --port N` → override default 7777

The existing `hermind setup` command is kept for scriptable, non-TTY
initialization (Docker, CI). Its help text is updated to point
interactive users at `hermind config`.

## Data flow

1. User launches `hermind config` (or first-run).
2. `editor.Load(path)` parses YAML into `*Doc` (AST retained).
3. UI pulls `editor.Schema()` and renders sections + fields.
4. Each edit calls `Doc.Set(path, value)` after `Validate`. Failure shows
   inline error; the AST is not mutated.
5. `Save` writes AST back via `yaml.Node.Encode`, preserving comments.
6. UI shows "Saved. Restart hermind to apply."

## Error handling

- YAML parse failure on `Load` → the Doc exposes the error; first-run path
  refuses to overwrite; `config` subcommand offers "open in $EDITOR
  instead" fallback so a corrupt file is repairable.
- Validation failure → inline error, no mutation.
- Save failure (disk full, permission) → error banner; `dirty` flag stays.
- Web port conflict → process exits with message
  `config: port 7777 in use; try --port N`.

## Testing

- `config/editor`: unit tests for Get/Set/Save with comment-preservation
  fixtures; table-driven dotPath resolution; atomicity under simulated
  mid-save failure.
- TUI: `tea.NewProgram` + golden-file snapshots of the rendered frame
  after key sequences; assertions on `Doc` state after edits.
- Web backend: `httptest.Server`, JSON round-trips, schema drift check
  (Schema() result has a stable JSON snapshot).
- Frontend: manual smoke test. Vanilla JS + static forms do not justify
  E2E tooling.

## Implementation order

1. `config/editor` package + Schema + tests.
2. `cli/ui/config` TUI (MVP — zero extra deps).
3. `cli/app.go` first-run hook.
4. `cli/config.go` subcommand wiring (TUI only).
5. `cli/ui/webconfig` backend.
6. `cli/ui/webconfig/web/` frontend + `go:embed`.
7. `config --web` wiring + browser auto-open.
8. Documentation updates (README, `setup` help text).

Each step merges independently.

## Security

Web config page binds loopback-only and does not authenticate. Explicit
decision: hermind is a single-user local tool, and the threat of "another
process on the same machine reads the API key" is already present (the
config file itself is readable). Users concerned about this continue to
use the TUI. A future iteration can add a one-time URL token behind a
flag if the threat model changes.

## Open questions

None at time of writing.
