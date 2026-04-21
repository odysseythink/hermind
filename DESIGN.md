# Design System — hermind

> **For agentic workers and humans alike.** Read this before making any visual
> or UI change. All token values, component patterns, and aesthetic decisions
> live here. Do not deviate without explicit user approval.

## Product Context

- **What this is:** A self-hosted, open-source AI agent framework. Users chat
  with an agent that runs tools (bash, web_search, web_fetch, MCP servers,
  memory) and ships results back. Agent can run locally, behind a platform
  gateway (Feishu, DingTalk, WeChat), or as a cron job.
- **Who it's for:** Developers and operators. People who read `config.yaml`
  for fun.
- **Space / industry:** Agentic developer tooling — peers are Aider, Cursor,
  Zed, Continue, Claude Code.
- **Project type:** Web app with two top-level modes (`#/chat` and
  `#/settings`). Chat is the conversational surface; settings is a dense
  control panel for providers, gateway platforms, MCP servers, cron, skills,
  memory, and observability.

## Memorable thing

> **"Built for tinkerers, not product managers."**

Every design decision below serves this. Where a choice would appeal to a
product manager (polish, generous whitespace, rounded-everything, pastel
palettes, "welcome to our product" onboarding modals) and the opposite choice
would appeal to a tinkerer (density, visible mechanism, monospace where
precision matters, terminal-adjacent palette), we pick the tinkerer choice.

## Aesthetic Direction

- **Direction:** Industrial / utilitarian with terminal accents. Think
  oscilloscope UI, sysadmin console, `htop` if it went to design school.
  Between Zed IDE and an honest terminal emulator. Not brutalist (too raw).
  Not luxury-refined (too serif, too generous).
- **Decoration level:** Minimal. Typography, grid, and the amber accent do
  all the work. No gradients, no glass, no drop shadows — 1px borders only.
- **Mood:** Precise, dense, restful at 2am, legible at a glance. The product
  feels like a control surface, not a brochure.

## Typography

Three fonts. All free. All distinctive. **Never `system-ui`, `Inter`,
`Roboto`, `Arial`, `Helvetica`, `Open Sans`, `Lato`, `Montserrat`, `Poppins`,
or `Space Grotesk`.**

- **Display / headings / branding:** `JetBrains Mono` — weight 600, tight
  letter-spacing (`.02em`), often uppercase. Monospace headings are the
  tinkerer signal. Every heading, every section title.
- **Body / UI / chat messages:** `IBM Plex Sans` — weight 400 default, 500
  for emphasis, 600 for buttons. Humanist sans with quirks. Reads well at
  small sizes. Google Fonts.
- **Data / code / session IDs / token counts / timestamps / config keys:**
  `JetBrains Mono` — weight 400-500 with `tabular-nums`. Columns don't dance.

### Loading

Load via Google Fonts `<link>` with `preconnect`:

```html
<link rel="preconnect" href="https://fonts.googleapis.com">
<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
<link href="https://fonts.googleapis.com/css2?family=IBM+Plex+Sans:wght@400;500;600;700&family=JetBrains+Mono:wght@400;500;600;700&display=swap" rel="stylesheet">
```

### Scale (tight, functional)

| Token | Size | Line-height | Usage |
|---|---|---|---|
| `--fs-xs`    | 11px | 1.3 | status bar, session timestamps, smallest metadata |
| `--fs-sm`    | 12px | 1.4 | labels, help text, tool_call internals |
| `--fs-base`  | 13px | 1.5 | body, chat messages (compact on purpose) |
| `--fs-md`    | 14px | 1.5 | emphasized body, section labels |
| `--fs-lg`    | 16px | 1.4 | section titles |
| `--fs-xl`    | 20px | 1.3 | page h2 |
| `--fs-2xl`   | 28px | 1.2 | page h1, landing hero |

Body at 13px is deliberate. Tinkerers sit at a 27" monitor, not a phone.

## Color

### Approach

Restrained. One accent (amber). Everything else is warm near-black + warm
grays. Amber appears only on: branding dot, focus ring, primary CTA, active
state, running indicators, streaming cursor.

### Dark mode (default)

| Token | Value | Role |
|---|---|---|
| `--bg`          | `#0a0b0d` | Page background — warm near-black with slight amber cast. NOT `#000`. NOT cool `#0d1117`. |
| `--surface`     | `#14161a` | Sidebar, composer, cards, inputs |
| `--surface-2`   | `#1d2027` | Raised panels, tool_call cards, dropdowns |
| `--border`      | `#2a2e36` | Visible 1px seams everywhere |
| `--text`        | `#e8e6e3` | Primary text — warm off-white |
| `--muted`       | `#8a8680` | Secondary text — warm gray, never cool |
| `--accent`      | `#FFB800` | VT220 phosphor amber — the one piece of soul |
| `--accent-2`    | `#FF8A00` | Hover / active amber |
| `--accent-bg`   | `rgba(255,184,0,.08)` | Subtle fill for active states |
| `--focus`       | `rgba(255,184,0,.35)` | 2px focus ring |
| `--success`     | `#7ee787` | Muted emerald |
| `--error`       | `#ff6b6b` | Muted coral |
| `--warning`     | `#d29922` | Muted honey (coexists with amber by being duller) |

### Light mode (supported)

| Token | Value | Role |
|---|---|---|
| `--bg`          | `#fafaf7` | Warm off-white, NOT `#fff` |
| `--surface`     | `#f0ede4` | Cream |
| `--surface-2`   | `#e8e4d6` | Raised cream |
| `--border`      | `#d9d4c8` | Warm gray |
| `--text`        | `#1a1817` | Warm near-black |
| `--muted`       | `#5c584f` | Warm gray |
| `--accent`      | `#FFB800` | Unchanged |
| `--accent-2`    | `#E89F00` | Hover / active |
| `--accent-bg`   | `rgba(255,184,0,.14)` | Subtle fill |
| `--focus`       | `rgba(255,184,0,.45)` | 2px focus ring |
| `--success`     | `#2f7d32` | Forest |
| `--error`       | `#c9302c` | Oxblood |
| `--warning`     | `#b37b04` | Mustard |

## Spacing

4px base unit, compact density.

| Token | Value | Usage |
|---|---|---|
| `--space-1`  | 4px  | hairline padding, badge internals |
| `--space-2`  | 8px  | input internal padding, tight rows |
| `--space-3`  | 12px | card padding default, panel padding |
| `--space-4`  | 16px | section gap |
| `--space-5`  | 20px | between sub-sections |
| `--space-6`  | 24px | panel gap |
| `--space-8`  | 32px | page edge gutter |
| `--space-12` | 48px | hero only, vertical breathing room |

## Layout

- **Approach:** Grid-disciplined with dense density.
- **Primary shell:** 48px top bar + content region. Chat workspace is a
  three-pane split (14rem sidebar + main conversation). Settings is a
  15rem sidebar + scrolling right panel.
- **Panel structure:** 1px borders between every major panel. Terminals
  use borders, not whitespace.
- **Max content width:** 1200px for marketing/landing flows (not used
  yet). App shells fill the viewport.
- **Radius:**

| Token | Value | Usage |
|---|---|---|
| `--r-none` | 0    | borders that should feel flush |
| `--r-sm`   | 2px  | buttons, inputs, most surfaces |
| `--r-md`   | 4px  | cards, tool_call, dialogs |
| `--r-lg`   | 8px  | upper bound — use only when strictly needed |

We don't use `9999px` pill shapes. No avatars, no rounded tabs.

## Motion

- **Approach:** Minimal-functional. Only transitions that communicate
  state change. No hero parallax, no scroll-driven animation, no entrance
  choreography.
- **Durations:**

| Token | Value | Usage |
|---|---|---|
| `--t-fast`   | 100ms | focus ring fade-in |
| `--t-short`  | 150ms | tooltip appear |
| `--t-medium` | 200ms | accordion expand, toggle slide |
| `--t-long`   | 400ms | phosphor pulse (amber running indicator) |

- **Easing:** `ease-out` for enter, `ease-in` for exit, `ease-in-out` for
  move.
- **Permitted personality:** (1) amber `box-shadow` focus glow — 2px,
  fades in over 100ms. (2) streaming cursor — 1s blink (`step-end`). (3)
  `running` badge dot — 1.2s pulse.

## Components (standard patterns)

### Buttons

- **Primary:** `--accent` background, `#0a0b0d` text, weight 600, 12px,
  2px radius. Hover → `--accent-2`. Focus ring.
- **Secondary:** transparent bg, `--border` border, `--text` color. Hover
  → `--accent-bg` fill.
- **Ghost:** transparent bg, no border, `--muted` color. Hover → `--text`.
- **No** gradient fills, no drop shadows, no pill shapes.

### Inputs

- Background: `--bg` (darker than surrounding `--surface` — the opposite
  of Material). Border: 1px `--border`. Padding: 8px 10px. Focus: border
  becomes `--accent`, 2px amber focus ring.
- Secret/password inputs: mono font, fixed `•••` placeholder.
- All numeric/id inputs: JetBrains Mono with `tabular-nums`.

### Badges

- 10px mono, uppercase, `.05em` letter-spacing, 2px radius, 1px border.
  No filled backgrounds unless `accent` variant. Use a tiny 6px dot for
  status; pulse on `running`.

### Dialogs

- Overlay: `rgba(0,0,0,.6)` in dark, `rgba(0,0,0,.25)` in light.
- Panel: `--surface-2`, 1px `--border`, 4px radius, centered.
- Header: mono, uppercase, 14px, bottom border.
- No fade/scale entrance. Just appear. (Terminals don't animate modals.)

### Cards (tool-call / session item / message bubble)

- `--surface` or `--surface-2` background with 1px `--border`.
- 2–4px radius.
- Active state: 2px left border in `--accent` + `--accent-bg` fill.
- Never use drop shadows for depth — use surface layering (`--surface`
  → `--surface-2`) and 1px borders.

### Message bubbles

- User: transparent background, 1px `--accent` border, right-aligned.
- Assistant: `--surface` background, 1px `--border`, left-aligned.
- Both: 2px radius (NOT a pill / NOT a bubble-shape). Sharp corners.
- Internal tool_call cards: `--surface-2`, collapsed by default except
  during active streaming.

### Tables / data density

- Mono font, `tabular-nums`.
- 1px borders between rows, no zebra stripes.
- Tight padding: 4px 8px.

## Anti-patterns (never ship these)

- `system-ui` / `-apple-system` as display or body font.
- Purple / violet gradients (the AI-slop signal).
- 3-column feature grid with icons inside colored circles.
- Centered-everything with uniform `margin: auto`.
- Rounded-bubble message containers (iMessage style).
- Gradient buttons as the primary CTA pattern.
- Drop shadows except a 1px border (which is not a shadow).
- Stock-photo hero sections with "people typing on laptops".
- Pastel backgrounds.
- Glass morphism / `backdrop-filter: blur`.
- Pill-shaped tabs or filters.
- "Built for developers" / "Designed for teams" marketing copy in the
  product.
- Any font on the blacklist: Papyrus, Comic Sans, Lobster, Impact,
  Raleway, Clash Display, Inter, Roboto, Arial, Helvetica, Open Sans,
  Lato, Montserrat, Poppins, Space Grotesk.

## Decisions Log

| Date | Decision | Rationale |
|---|---|---|
| 2026-04-21 | Initial design system created | Product moved from TUI to web in Plans 1-5. Existing ad-hoc `theme.css` used system-ui (the "I gave up on typography" signal) and cool grays. This system replaces it with JetBrains Mono + IBM Plex Sans + warm-near-black + VT220 amber, grounded in the memorable thing: "built for tinkerers, not product managers." |
