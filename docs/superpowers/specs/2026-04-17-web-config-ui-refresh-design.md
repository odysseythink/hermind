# Design: web config UI visual refresh

**Status:** Approved
**Date:** 2026-04-17

## Goal

Replace the bare-bones styling in `cli/ui/webconfig/web/` with a clean
developer-tool aesthetic (Linear/Vercel style) that respects light and
dark OS preferences. No JavaScript logic changes — the existing schema
fetch, per-field persist, save, reveal, and shutdown calls keep working
as-is. This is a pure CSS/HTML rewrite anchored to the TUI brand color
(`#FFB800` amber) so the web and terminal surfaces feel like one product.

## Non-goals

- Search across sections
- Per-section icons (would require an icon set — stays zero-dependency)
- Keyboard shortcuts
- Internationalization — existing English labels stay; no new strings
- Animations beyond `transition: 120ms ease` for hover/focus
- Backend or API changes (`cli/ui/webconfig/{server.go,handlers.go}` untouched)
- New tests — `server_test.go` already covers API behavior and doesn't
  inspect frontend rendering

## Scope

Three files in `cli/ui/webconfig/web/`:

| File | Before | After |
|------|--------|-------|
| `index.html` | 18 lines — sidebar + main + footer | ~40 lines — adds header bar, status dot element, structured section panels |
| `app.css` | 12 lines, unstyled | ~200 lines — CSS custom properties, light+dark mode, typography, layout, components |
| `app.js` | 110 lines — keep as-is | 110 lines, minor tweaks only to emit the new status-dot classes |

Optional tweak in `app.js`: the existing `status()` function writes plain
text. The refresh asks it to also toggle a small CSS class on the status
element (`idle|unsaved|saved|error`) so the colored dot reflects state.
Two lines. Not logic change.

## Palette

Defined as CSS custom properties on `:root`, overridden inside
`@media (prefers-color-scheme: dark)`.

**Light:**
- `--bg`: `#ffffff`
- `--surface`: `#f8fafc` (sidebar, footer)
- `--border`: `#e5e7eb`
- `--text`: `#111827`
- `--muted`: `#6b7280`
- `--accent`: `#FFB800` (TUI parity)
- `--accent-fg`: `#111827` (amber needs dark text for contrast)
- `--focus`: `rgba(255,184,0,.35)`
- `--success`: `#22c55e`
- `--error`: `#ef4444`

**Dark:**
- `--bg`: `#0b0d11`
- `--surface`: `#14171c`
- `--border`: `#2a2f38` (near the TUI's `#3B4252`)
- `--text`: `#e6e8eb`
- `--muted`: `#8892a1`
- `--accent`: `#FFB800`
- `--accent-fg`: `#0b0d11`
- `--focus`: `rgba(255,184,0,.5)`
- `--success`: `#22c55e`
- `--error`: `#f87171`

## Layout

```
┌─────────────────────────────────────────────────┐
│ ⬡ hermind config  ~/.hermind/config.yaml   [dot]│  header (48px)
├──────────────────┬──────────────────────────────┤
│ Sidebar          │  Main                        │
│ · General        │  ┌ Section title ──────────┐ │
│ · Providers      │  │ muted subtitle          │ │
│ · Skills         │  ├─────────────────────────┤ │
│ · Storage        │  │ Label                   │ │
│ · Terminal       │  │ [ input              ] │ │
│ · …              │  │ help text               │ │
│                  │  │                         │ │
│                  │  │ ... more fields         │ │
│                  │  └─────────────────────────┘ │
├──────────────────┴──────────────────────────────┤
│ [Save] [Save & Exit]              status · saved│  footer (48px)
└─────────────────────────────────────────────────┘
```

### Regions

- **Header (new):** 48px tall with a bottom `--border`. Left: `⬡ hermind
  config` in medium weight. Middle: config file path rendered in
  monospace + `--muted`, selectable. Right: status dot (same element as
  footer status; reflects unsaved/saved/error).
- **Sidebar:** 240px wide, `--surface` background. Section items get 8px
  border-radius, 10px vertical / 12px horizontal padding. Active item:
  2px amber left border + `rgba(255,184,0,0.08)` tinted fill (same value
  in light and dark). Hover (non-active): `rgba(127,127,127,0.06)` tinted
  fill without the strip. No icons (out of scope).
- **Main:** max-width 720px, centered inside the remaining width, 40px
  horizontal padding, 32px top padding. Each section renders as one
  panel (1px `--border`, 8px radius, 24px padding, 32px gap between
  panels). Section title: 16px medium; optional subtitle 13px `--muted`.
- **Form rows:** label stacked above input (not side-by-side). Works on
  narrow windows and is easier to scan than the current fixed 18rem
  label column. Label: 14px medium `--text`. Help below input: 13px
  `--muted`, 8px top margin.
- **Inputs:** full-width inside the panel, 38px tall, 8px radius, 1px
  `--border`, 12px horizontal padding, 14px text. Focus: 2px `--focus`
  ring (via `box-shadow`) + `--accent` border. Number inputs use
  `font-variant-numeric: tabular-nums`.
- **Secret field:** Show/Hide button replaces the emoji. Text button:
  `--muted` color, no background, 12px font, hover = underline. Sits
  inside the input's right padding (absolute-positioned inside a wrapper
  span) so it doesn't widen the row.
- **Footer:** 48px tall, `--surface` background, top `--border`. Buttons
  right-aligned (was left). Status indicator left-aligned: colored dot
  (6px circle) + 13px text.
- **Buttons:**
  - **Save (secondary):** 32px tall, 1px `--border`, transparent bg,
    `--text` color, 8px radius, 14px horizontal padding. Hover: `--surface`.
  - **Save & Exit (primary):** same dimensions, `--accent` bg, `--accent-fg`
    text, no border. Hover: slightly darker via `filter: brightness(0.95)`.

### Status dot

`<span id="status"><span class="dot"></span><span class="msg"></span></span>`.
The dot has 4 states via CSS class on the parent:

| State | Dot color | Text |
|-------|-----------|------|
| `idle` (default) | hidden | empty |
| `unsaved` | `--accent` | "unsaved changes" |
| `saved` | `--success` | "saved — restart hermind to apply" |
| `error` | `--error` | actual error string from the server |

`app.js` keeps calling `status(s)` but also sets a class on the parent
based on which message it's writing.

## Typography

```css
--font-sans: ui-sans-serif, system-ui, -apple-system,
             "SF Pro Text", "Segoe UI", "PingFang SC",
             "Microsoft YaHei", sans-serif;
--font-mono: ui-monospace, "SF Mono", Menlo, "Liberation Mono",
             "Consolas", monospace;
```

Base `14px` / line-height `1.5`. Header title `15px` medium. Section
title `16px` medium. Help text `13px`. Path in the header uses
`--font-mono` at `13px`.

## Error / empty states

- **Save failure OR per-field persist failure:** status flips to `error`
  (red dot + server message from the response body, or `"save failed"` /
  `"error: <body>"` as `app.js` currently constructs). Footer gets a
  CSS class `.flash-error` for 1 second (set, then `setTimeout(..., 1000)`
  clears it). That class applies
  `box-shadow: inset 0 0 0 1px var(--error)` so the visual flash is a
  pure CSS transition — no keyframe animation needed.
- **List fields (`kind === 6`):** current "(edit via YAML or TUI)" text
  stays, but rendered in `--muted` italic inside a neutral background.

## Testing

- No new Go tests (server behavior unchanged).
- Manual smoke:
  1. Launch `./bin/hermind config --web` → confirm the header renders,
     sidebar has the amber active indicator, and section panels have
     visible borders.
  2. Toggle OS theme (macOS: ⌃⌘Space → color scheme) → confirm CSS
     custom properties flip without reload (CSS `prefers-color-scheme`
     reacts live).
  3. Edit any field → status dot turns amber with "unsaved changes".
  4. Click Save → dot turns green with "saved — restart…".
  5. Click the Show/Hide button on a secret field → text becomes
     visible/masked correctly.
  6. Resize browser down to ~800px width → sidebar + main layout
     remains usable (no wrapping bugs, no horizontal scroll).

## Rollout

Single commit. No migration, no feature flag. Users who load the page
after the next `hermind` build get the new styling automatically (the
frontend is embedded via `embed.FS` in `server.go`).

## Out of scope / future work

- Mobile layout (sidebar collapse to hamburger). Config UI is primarily
  a desktop tool; defer until someone asks.
- An actual logo in the header (the `⬡` is a Unicode hex glyph
  placeholder — good enough for v1).
- Auto-save on blur (current explicit Save button is clearer and matches
  the TUI editor's save-on-exit semantics).
- i18n — current labels come from Go side and stay English for now.
