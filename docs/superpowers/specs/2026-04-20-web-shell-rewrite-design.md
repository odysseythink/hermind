# Web Shell Rewrite — Stage 1 Design

**Date:** 2026-04-20
**Topic:** Multi-section config UI shell (first of an 8-stage web-config expansion)
**Status:** Approved

## Context

The current Vite/React web UI at `web/` (served via `hermind web` as the
embedded `api/webroot/`) only renders editors for `gateway.platforms`
(the 19 IM platforms shipped in the prior five stages). The other 13
top-level `config.Config` fields — `Model`, `Providers`,
`FallbackProviders`, `Agent`, `Auxiliary`, `Terminal`, `Storage`, `MCP`,
`Memory`, `Browser`, `Cron`, `Logging`, `Metrics`, `Tracing`, `Skills` —
have no presence in the new UI. Users who want to edit those fields
currently must drop to the legacy `hermind config --web` editor at
`cli/ui/webconfig/`, which is a separate server with a separate HTML
bundle.

The goal of this 8-stage initiative is to bring full config editing into
the new `api/` + `web/` stack and eventually sunset
`cli/ui/webconfig/`. Stage 1 (this document) covers the shell / navigation
rewrite only; no new config editors ship in stage 1.

## Sub-project Sequence (for context)

1. **Stage 1 — Shell / navigation rewrite** (this spec)
2. Stage 2 — Schema infrastructure for non-platform sections
3. Stage 3 — Simple sections (Model / Agent / Terminal / Storage / Logging / Metrics / Tracing)
4. Stage 4 — Providers editor (multi-type + "fetch models" button)
5. Stage 5 — Memory editor (multi-backend branching)
6. Stage 6 — Skills editor (enable/disable + platform overrides)
7. Stage 7 — MCP / Browser / Cron
8. Stage 8 — Sunset `cli/ui/webconfig/`, flip `./bin/hermind` default command

Each sub-project produces its own spec → plan → implementation.

## Scope

### In Scope

- New shell: TopBar + collapsible-group sidebar + content panel
- Seven top-level groups: Models, Gateway, Memory, Skills, Runtime,
  Advanced, Observability
- **Gateway** group wraps the existing IM editor with zero functional
  changes
- Other six groups render a "Coming soon — stage N" placeholder with a
  read-only config preview and a CLI escape hatch
- TopBar: keep global **Save** (→ `PUT /api/config`); remove
  **Save and Apply**
- Gateway panel: add a local **Apply** button (→
  `POST /api/platforms/apply`); the previous TopBar Apply is moved here
- Hash URL scheme: `#<group>` and `#<group>/<sub>`
- Deep-link compat: legacy `#<platform-key>` hashes migrate automatically
  to `#gateway/<platform-key>` if the key matches an existing instance
- Collapse state of groups is persisted to
  `localStorage['hermind.shell.expandedGroups']`, default
  `['gateway']`
- Test coverage for new shell components and hash routing; existing IM
  tests continue to pass

### Out of Scope

- Any non-platform config editors (deferred to stages 3–7)
- Schema infrastructure for generic sections (stage 2)
- Deletion of `cli/ui/webconfig/` (stage 8)
- Changes to the `hermind` default subcommand (stage 8)
- Sidebar search or filtering
- Keyboard shortcuts for group switching
- Per-group reducers or state splitting

## Layout

### TopBar (~48 px)

- Left: `hermind` brand label + version
- Right: **Save** button with dirty badge, e.g. `Save · 3 changes`
- No **Save and Apply** button

### Sidebar (~220 px)

Seven groups in this fixed order:

1. **Models** (maps to `config.Model`, `config.Providers`, `config.FallbackProviders`)
2. **Gateway** (maps to `config.Gateway`)
3. **Memory** (maps to `config.Memory`)
4. **Skills** (maps to `config.Skills`)
5. **Runtime** (maps to `config.Agent`, `config.Auxiliary`, `config.Terminal`, `config.Storage`)
6. **Advanced** (maps to `config.MCP`, `config.Browser`, `config.Cron`)
7. **Observability** (maps to `config.Logging`, `config.Metrics`, `config.Tracing`)

Group header: ▸/▾ arrow + label text. No icons.

Default state: only **Gateway** is expanded (it is the only group with
substantive content in stage 1).

When expanded:

- **Gateway**: lists each IM instance + a `+ new instance` button
  (functionally identical to the current Sidebar)
- Any other group: shows a single dimmed row `Coming soon — stage N`

Active-state highlighting reuses existing `.item.active` styles.

Collapse state is persisted to
`localStorage['hermind.shell.expandedGroups']` as a JSON array of group
IDs.

### Content Panel (flex 1)

- Top row: breadcrumb (e.g. `Gateway · dingtalk-alerts`) + the group's
  optional Apply button
- Body: the group's panel (in stage 1, only Gateway has one;
  everything else renders `ComingSoonPanel`)

### Empty State

When `activeGroup` is null (first load with no hash), show a centered
"Select a configuration section" panel with seven small cards — one per
group — that jump to that group on click. Reuses the current empty-state
Editor styles.

### Visual

Reuses existing CSS variables. No new design tokens. Group-header and
sub-item styles are variations on the existing `.item` class.

## URL Scheme

```
#<group>                 open a group, no sub-item selected (e.g. #models)
#<group>/<sub>           open a group with a sub-item selected (e.g. #gateway/feishu-bot-main)
```

- Group keys are a fixed whitelist (`models`, `gateway`, `memory`,
  `skills`, `runtime`, `advanced`, `observability`) and are not
  URL-encoded.
- The `<sub>` segment is run through `encodeURIComponent` on write and
  `decodeURIComponent` on read, matching the current App.tsx behavior
  for IM instance keys that contain special characters.
- The `/` separator is literal, not encoded, for human readability.

### Parsing

| Hash value                     | Result                                                       |
|--------------------------------|--------------------------------------------------------------|
| empty / `#`                    | default state (Gateway expanded, no sub-item selected)       |
| `#gateway`                     | activeGroup=`gateway`, no sub                                |
| `#gateway/xxx`                 | activeGroup=`gateway`, sub=`xxx`                             |
| `#<other-group>`               | activeGroup=that group, no sub (renders ComingSoonPanel)     |
| `#<other-group>/xxx`           | same; `xxx` ignored                                          |
| `#feishu-bot-main` (legacy)    | if `feishu-bot-main` exists in `platforms`, redirect via `history.replaceState` to `#gateway/feishu-bot-main`; otherwise default state |
| `#anything-else-unknown`       | default state                                                |

### Writing

- Clicking a group or sub-item in the sidebar updates the hash via
  `history.replaceState` (not `pushState`) to avoid polluting browser
  history.
- Collapsing or expanding a group does **not** write the hash; it only
  writes localStorage.

## State Model

Extend the existing single-root reducer in `web/src/state.ts`:

```ts
interface AppState {
  // existing fields (preserved)
  config: Config;
  originalConfig: Config;           // snapshot after the last successful save
  descriptors: SchemaDescriptor[];  // IM platform descriptors
  flash: FlashMessage | null;
  // ... other existing fields

  // new
  shell: {
    activeGroup: GroupId | null;
    activeSubKey: string | null;    // used only when activeGroup === 'gateway'
    expandedGroups: Set<GroupId>;   // persisted to localStorage
  };
}
```

Rationale for a single reducer (vs. per-group reducers):

- Every group edits a slice of the same `config: Config` tree. There is
  one config, not seven.
- Save targets `PUT /api/config` once with the whole object. Global
  dirty state is simpler with a single root.
- New groups add `config`-slice edit actions, not new reducers.
- The current state.ts architecture is already this shape.

### Dirty Tracking

Augment the existing `instanceDirty(key)` selector (which compares a
single IM instance against `originalConfig`) with a group-level helper:

```ts
groupDirty(state, groupId): boolean
```

Implementation is a structural compare of the group's config-slice
against `originalConfig`. Stage 1 only has Gateway wired up, so in
practice only `groupDirty('gateway')` ever returns true. The helper
exists so future stages can plug in without reshaping the selector API.

### Save Behavior

- Save button label: `Save` when clean, `Save · N changes` when dirty
  (N = total dirty sub-items across all groups; stage 1 counts dirty IM
  instances).
- Click → `PUT /api/config` with the full `config`; on success,
  `originalConfig = config`, dirty flags clear, flash = "Saved."

### Apply Behavior (Gateway only in stage 1)

Gateway panel renders an Apply button in its breadcrumb row:

- Disabled when `groupDirty('gateway') === true` (you must save before
  applying; matches the current "don't apply stale disk config" rule).
- Enabled when the gateway slice is clean.
- Click → `POST /api/platforms/apply` (unchanged from current
  behavior).

Other groups add their own Apply semantics in later stages; stage 1
makes no commitment about their shape beyond "there is a place for the
button."

### New Actions

- `shell/selectGroup` — set `activeGroup`, clear `activeSubKey` unless the
  group is `gateway` and a sub is being selected simultaneously; sync
  hash.
- `shell/selectSub` — set `activeSubKey`; sync hash.
- `shell/toggleGroup` — flip the group's membership in
  `expandedGroups`; write localStorage.

### New Selectors

- `dirtyGroups(state): Set<GroupId>` — which groups have unsaved changes
- `totalDirtyCount(state): number` — summed dirty sub-item count across
  all groups (stage 1: dirty IM instance count)

## Placeholder Panels

Six groups (everything except Gateway) render
`<ComingSoonPanel group={groupId} plannedStage={N} />`:

- Breadcrumb: `<Group label> · Coming soon` (no Apply button)
- Body:
  - Large heading with group label
  - Subheading `Coming in stage N` (or `stage N & M` for groups
    spanning multiple stages)
  - Bulleted list of what that section will cover
  - A "Current config (read-only preview)" block: a small per-group
    summary function renders a human-readable snapshot of the relevant
    `config` slice, so the user can confirm their config is recognized
  - An "Edit via CLI" escape hatch: `hermind config --web`

Per-group planned-stage mapping:

| Group         | Stage(s) | Covers                                                 |
|---------------|----------|--------------------------------------------------------|
| Models        | 3 & 4    | `model`, `providers`, `fallback_providers`             |
| Memory        | 5        | `memory`                                               |
| Skills        | 6        | `skills`                                               |
| Runtime       | 3        | `agent`, `auxiliary`, `terminal`, `storage`            |
| Advanced      | 7        | `mcp`, `browser`, `cron`                               |
| Observability | 3        | `logging`, `metrics`, `tracing`                        |

The per-group summary functions live in `web/src/shell/summaries.ts` —
one exported function per group, each taking the full `Config` and
returning a `ReactNode` (typically a short `<dl>` of key facts).
Examples:

- Memory: `backend: retain_db, enabled: true`
- Observability: `logging.level: info, metrics.enabled: false`
- Skills: `3 disabled globally, 1 platform override`

The summary is deliberately minimal — it exists so the user can spot
misconfiguration at a glance, not replace an editor. Total implementation
is ~5–10 lines per group.

## Component Structure

```
web/src/
├── App.tsx                                  rewritten: shell host + hash router
├── state.ts                                 extended: shell slice + actions + selectors
├── api/                                     unchanged
│   ├── client.ts
│   └── schemas.ts
├── components/
│   ├── shell/                               new
│   │   ├── TopBar.tsx                       moved + trimmed: only Save remains
│   │   ├── TopBar.module.css
│   │   ├── Sidebar.tsx                      rewritten: 7 collapsible groups
│   │   ├── Sidebar.module.css
│   │   ├── GroupSection.tsx                 new: single-group header + expand
│   │   ├── ContentPanel.tsx                 new: routes activeGroup → panel
│   │   ├── EmptyState.tsx                   new: 7-card grid shown when nothing is selected
│   │   └── ComingSoonPanel.tsx              new: placeholder + summary + CLI escape
│   ├── groups/                              new
│   │   └── gateway/
│   │       ├── GatewayPanel.tsx             new: hosts existing IM Editor + apply
│   │       ├── GatewayApplyButton.tsx       new: extracted from TopBar
│   │       └── GatewaySidebar.tsx           new: instance list shown when Gateway is expanded
│   ├── Editor.tsx                           unchanged (IM instance editor)
│   ├── Editor.module.css                    unchanged
│   ├── FieldList.tsx                        unchanged
│   ├── NewInstanceDialog.tsx                unchanged
│   ├── TestConnection.tsx                   unchanged
│   ├── Footer.tsx                           unchanged
│   └── fields/                              unchanged
└── shell/                                   new
    ├── groups.ts                            single source of truth for group metadata
    ├── hash.ts                              hash parse/stringify + legacy migration
    └── summaries.ts                         per-group read-only summary functions
```

### Key Boundaries

1. **`groups.ts` is the single source of truth** for group metadata
   (id, label, planned stage, related config keys). Sidebar,
   ContentPanel, and summaries all read from it. Adding a new group
   later means editing this one file plus adding a matching
   `components/groups/<id>/` directory.
2. **`components/shell/` vs `components/groups/`**: `shell/` is the
   framework (TopBar / Sidebar / ContentPanel) and does not know which
   groups exist. `groups/` is per-group implementation, one directory
   per group id. `ContentPanel` looks up the active group's panel from
   a dispatch table; if no panel is registered, it falls back to
   `ComingSoonPanel`.
3. **IM Editor / FieldList / TestConnection are untouched**.
   `GatewayPanel` wraps them. This is how stage 1 guarantees
   zero-functional-regression for IM.
4. **`hash.ts` is the routing layer**. `state.ts` calls it for parse/
   stringify; `state.ts` does not build hash strings inline.

### Size Estimate

- ~10 new files, each under 150 lines
- `App.tsx` shrinks from ~180 to ~120 lines (state/routing pulled out)
- `Sidebar.tsx` largely rewritten
- `TopBar.tsx` small change (delete Save-and-Apply)
- `state.ts` small change (add shell slice)

## Testing

### Framework

Vitest + jsdom are already installed. Component tests require
`@testing-library/react` and `@testing-library/user-event`, which are
added as dev dependencies in the first task of the plan. No new
runtime dependencies.

### New Test Files

```
web/src/
├── shell/
│   ├── hash.test.ts
│   ├── groups.test.ts
│   └── summaries.test.ts
├── state.test.ts                              extended
└── components/
    ├── shell/
    │   ├── Sidebar.test.tsx
    │   ├── TopBar.test.tsx
    │   ├── ContentPanel.test.tsx
    │   └── ComingSoonPanel.test.tsx
    └── groups/gateway/
        └── GatewayPanel.test.tsx
```

### Core Test Cases

**`hash.test.ts`:**

- `parseHash('#gateway/feishu')` → `{ group: 'gateway', sub: 'feishu' }`
- `parseHash('#models')` → `{ group: 'models', sub: null }`
- `parseHash('#unknown')` → `{ group: null, sub: null }`
- `parseHash('#feishu-bot-main')` with `feishu-bot-main` present in
  `platforms` → migrates to `{ group: 'gateway', sub: 'feishu-bot-main' }`
- `parseHash('#feishu-bot-main')` with key absent →
  `{ group: null, sub: null }`
- Round-trip encode/decode for sub keys with `%`, `/`, spaces, dots

**`groups.test.ts`:**

- All 7 group ids present
- No duplicate ids
- Every group has `label`, `plannedStage`, `configKeys`
- `configKeys` reference known top-level `Config` fields (TypeScript
  plus runtime assertion)

**`summaries.test.ts`:**

- Each of the 6 placeholder groups renders a non-empty summary for a
  representative fixture `Config`
- Summaries render gracefully on a zero-value `Config` (no crashes)

**`Sidebar.test.tsx`:**

- Default state: only Gateway is expanded
- Click group header → toggles collapse; localStorage is written
- Reload (simulated by re-mounting) restores expanded state from
  localStorage
- Click instance → fires `onSelectSub` and updates hash
- Dirty dot shows next to instances with unsaved changes

**`TopBar.test.tsx`:**

- Save disabled when clean
- Save enabled and labeled `Save · N changes` when dirty
- Click → `PUT /api/config`; on success dirty clears
- Save-and-Apply button is absent (regression guard)

**`ContentPanel.test.tsx`:**

- `activeGroup === 'gateway'` → renders `GatewayPanel`
- `activeGroup === 'models'` → renders `ComingSoonPanel` with
  `plannedStage=3 & 4`
- `activeGroup === null` → renders `EmptyState`

**`GatewayPanel.test.tsx`:**

- Apply disabled when gateway slice is dirty
- Apply enabled when clean
- Click Apply → `POST /api/platforms/apply`
- Existing Editor / FieldList / TestConnection continue to work through
  the new wrapper (existing tests continue to pass)

**`ComingSoonPanel.test.tsx`:**

- Displays correct planned stage
- Renders the group's summary
- "Edit via CLI" text is visible

### Coverage Targets

- New code ≥ 90% line coverage (shell layer is mechanical, easy to
  cover)
- Existing IM code coverage does not drop; GatewayPanel wrapping must
  not bypass any existing test

### CI

`make web-check` already runs `pnpm test`. New tests are picked up
automatically. No CI changes.

### Smoke Test

Update `docs/smoke/web-config.md` to add:

- Sidebar shows all 7 groups on load
- Clicking Models shows "Coming soon — stage 3 & 4" with the current
  model listed in the preview
- Legacy hash `#feishu-bot-main` auto-migrates to
  `#gateway/feishu-bot-main`
- Save button's `N changes` counter matches the number of dirty IM
  instances

## Risks and Mitigations

- **Risk:** IM user sees a regression after shell rewrite.
  **Mitigation:** IM-facing components (`Editor`, `FieldList`,
  `TestConnection`, `NewInstanceDialog`, `fields/*`) are untouched.
  `GatewayPanel` only wraps them.
- **Risk:** Deep-link migration is brittle for edge cases.
  **Mitigation:** Conservative policy — only migrate if the key
  exists in the live `platforms` list at parse time; otherwise fall
  back to default state, no silent guessing.
- **Risk:** Dirty tracking diverges between global counter and
  per-group/per-instance UI.
  **Mitigation:** Single source — `totalDirtyCount(state)` —
  consumed by both TopBar and any future group badge. Tests assert
  the values match.
- **Risk:** Expanding all seven groups on a narrow viewport makes the
  sidebar scroll long.
  **Mitigation:** Default only Gateway expanded; no width resize in
  stage 1; deferred to a later polish pass.

## Approval

This spec was brainstormed and approved segment-by-segment before being
written. The next step is the implementation plan (via the
superpowers:writing-plans skill).
