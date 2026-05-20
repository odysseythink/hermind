# Filesystem Config Page — AnythingLLM Style Port

**Date:** 2026-05-20  
**Scope:** Frontend only (`web/src/components/groups/skills/detail-renderers/filesystem/`)  
**Backend impact:** None (reuses existing `config.tools.settings.filesystem` storage)

---

## 1. Goal

Redesign the `FilesystemConfig` detail renderer in hermind's settings page to match the visual style and interaction pattern of AnythingLLM's filesystem skill configuration panel.

## 2. Design Overview

The `FilesystemConfig` component renders inside hermind's existing settings two-pane layout (left list, right detail). The redesign keeps this layout but transforms the **right detail panel** into a full AnythingLLM-style filesystem skill panel.

### Visual structure (top to bottom)

1. **Header** — "📁 文件系统访问" + master toggle switch
2. **Banner image** — `/filesystem-banner.png` (adaptive width, rounded corners)
3. **Warning banner** — Orange alert about filesystem risks
4. **Description** — One-line description of what filesystem access does
5. **Tab navigation** — Two tabs: "Available Tools" / "Permissions"
6. **Tab content** — Dynamic content based on active tab

**Master switch behavior:** When disabled, all content below the header is dimmed (`opacity: 0.5`, `pointer-events: none`) but remains visible.

---

## 3. Tab 1 — Available Tools

Displays filesystem sub-tools grouped by read/write operations, each with an individual toggle switch.

### Read Actions (阅读操作)

| Icon | Name | Description |
|------|------|-------------|
| 📄 | 读取文件 | 读取文件内容（包括文本、代码、PDF、图像等） |
| 📑 | 读取多个文件 | 同时读取多个文件 |
| 📂 | 目录 | 列出文件夹中的文件和目录 |
| 🔍 | 搜索文件 | 按文件名或内容搜索文件 |
| ℹ️ | 获取文件信息 | 获取有关文件的详细元数据 |

### Write Actions (编写操作)

Group header carries an orange warning icon (⚠️).

| Icon | Name | Description |
|------|------|-------------|
| 💾 | 创建文本文件 | 创建新的文本文件，或覆盖现有的文本文件 |
| ✏️ | 编辑文件 | 对文本文件进行基于行的编辑 |
| 📂 | 创建目录 | 创建新的目录 |
| 📋 | 复制文件 | 复制文件和目录 |
| ↔️ | 移动/重命名文件 | 移动或重命名文件和目录 |

### Card styling

- **Read action cards:** `background: var(--surface-2)`, `border: 1px solid var(--border)`, `border-radius: var(--r-lg)`
- **Write action cards:** `background: rgba(234, 88, 12, 0.06)`, `border: 1px solid rgba(234, 88, 12, 0.25)`, same radius
- **Disabled state:** `opacity: 0.45`
- Switch: existing `Switch` component (28×14 px)

---

## 4. Tab 2 — Permissions

A managed directory list with add/remove controls.

### Layout

```
定义文件系统代理可以访问哪些文件夹。代理只能在这些文件夹及其子目录中读取、写入和搜索。

┌────────────────────────────────────────────────────┐
│ 📁 Name                                    [+] [−] │
├────────────────────────────────────────────────────┤
│ 📁 /home/user/projects                             │
│ 📁 /tmp                                            │
│ 📁                                                 │  ← new empty row (editing)
└────────────────────────────────────────────────────┘
```

### Interactions

- **Directory rows:** Each row shows a folder icon (📁) and the path. Rows are rendered as lightweight inline input fields that look like plain text by default; on focus they show a standard input border.
- **[+] button:** Appends a new empty input row at the bottom of the list and auto-focuses it.
- **[−] button:** Removes the **currently selected** row. If no row is selected, removes the **last** row.
- **Selection:** Clicking a row selects it (highlighted background or left border accent).
- **Auto-cleanup:** An empty row that loses focus is automatically removed.
- **Persistence:** The directory list is serialized to a newline-delimited string stored at `config.tools.settings.filesystem.allowed_directories` (same backend contract as today).

### Styling

- List container: `border: 1px solid var(--border)`, `border-radius: var(--r-md)`, `background: var(--surface)`
- Header row: `padding: var(--space-2) var(--space-3)`, bottom border separator, muted text
- Directory row: `padding: var(--space-2) var(--space-3)`, bottom border separator, selectable
- Selected row: `background: var(--accent-bg)` or left accent border
- [+] / [−] buttons: `width: 24px; height: 24px`, `border: 1px solid var(--border)`, `background: var(--surface-2)`, hover tint

---

## 5. Component Architecture

All components live inside `FilesystemConfig.tsx` (no new files).

```
FilesystemConfig
├── Header (title + master Switch)
├── BannerImage
├── WarningBanner
├── DescriptionBlock
├── TabNav
│   └── TabButton × 2
├── TabContent
│   ├── AvailableToolsTab
│   │   ├── ReadGroup (SubtoolCard × 5)
│   │   └── WriteGroup (SubtoolCard × 5)
│   └── PermissionsTab
│       ├── DirectoryListHeader (Name + [+] [−])
│       └── DirectoryList (DirectoryRow[])
└── SubtoolCard (read/write variants)
```

### State management

- `activeTab: 'tools' | 'permissions'` — local UI state (not persisted)
- `selectedDirIndex: number | null` — which directory row is selected
- Sub-tool booleans and `allowed_directories` continue to read/write through the existing `config.tools.settings.filesystem` path via `onSectionField('tools', 'settings', ...)`.

### Helper functions

- `parseDirs(raw: string): string[]` — split by newline, trim, filter empty
- `serializeDirs(dirs: string[]): string` — filter empty, join by newline

---

## 6. Data Flow

No backend changes. Existing data contract:

| Field | Type | Storage path |
|-------|------|--------------|
| `allowed_directories` | newline-delimited string | `config.tools.settings.filesystem.allowed_directories` |
| Sub-tool toggles | boolean per tool | `config.tools.settings.filesystem.<tool_name>` |
| Master disable | boolean (via disabled list) | `config.tools.disabled[]` (contains `"filesystem"` when off) |

The Permissions tab converts the newline string to a string array for UI editing, then serializes back on every change.

---

## 7. Styling Summary

| Element | Key styles |
|---------|-----------|
| Banner image | `width: 100%; max-height: 180px; object-fit: cover; border-radius: var(--r-lg)` |
| Warning banner | Existing `rgba(210, 153, 34, 0.08)` background + orange text |
| Tab nav | Reuse `.tabs` / `.tab` / `.active` from `SkillToolsConfigPage.module.css` |
| Read card | `background: var(--surface-2); border-color: var(--border)` |
| Write card | `background: rgba(234, 88, 12, 0.06); border-color: rgba(234, 88, 12, 0.25)` |
| Disabled card | `opacity: 0.45` |
| Directory list | `border: 1px solid var(--border); border-radius: var(--r-md)` |
| Add/Remove buttons | `24×24 px`, bordered, surface background |

All colors use CSS custom properties so light/dark mode switches automatically.

---

## 8. Testing Plan

1. **Manual visual test:** Open settings → Skills → filesystem. Verify banner loads, tabs switch, sub-tool toggles work, directory list add/remove/edit functions correctly.
2. **Regression:** Toggle master switch off/on; confirm other tools/skills unaffected.
3. **Theme test:** Toggle OS light/dark mode; verify write-action orange tint and directory list render correctly in both themes.
4. **Vitest:** `FilesystemConfig` has no existing unit test file; aligned with current project pattern, no new tests required for this frontend-only UI change.

---

## 9. Files Changed

| File | Action |
|------|--------|
| `web/src/components/groups/skills/detail-renderers/filesystem/FilesystemConfig.tsx` | Rewrite JSX structure and logic |
| `web/src/components/groups/skills/detail-renderers/filesystem/FilesystemConfig.module.css` | Rewrite styles |
| `web/public/filesystem-banner.png` | Add static asset |
