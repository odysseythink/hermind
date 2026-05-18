# Design: Port AnythingLLM Chat UI to Hermind

## Overview

Port the right-side chat interface from AnythingLLM v1.12.1 into Hermind's `web/` frontend, replacing the current `ChatWorkspace` conversation UI. The left sidebar (workspace/thread list) is **not** ported — only the chat area inside the red box.

## Goals

1. Replace Hermind's current chat empty-state and input area with AnythingLLM-style UI.
2. Support dual-mode layout: centered empty-state vs. bottom-fixed input when messages exist.
3. Add model picker, attach/mention/tools buttons, and a tools menu.
4. Keep changes scoped to `web/src/components/chat/` and reuse existing Hermind APIs.
5. Maintain CSS Modules styling (no Tailwind).

## Non-Goals

- Do NOT port AnythingLLM's left sidebar, workspace system, or agent model.
- Do NOT add "创建代理" / "编辑工作区" / "上传文件" quick-action buttons.
- Do NOT introduce new build dependencies or theming systems.

---

## Layout: Dual-Mode Switch

`ChatWorkspace` switches layout based on `isEmpty`:

```
isEmpty = messages.length === 0 && !streamingDraft && streamingToolCalls.length === 0
```

### Empty Mode (isEmpty = true)

```
┌────────────────────────────┐
│ [ModelPicker]              │  ← top-left pill showing model name
│                            │
│                            │
│      今天我能帮您什么？      │  ← vertically centered greeting
│                            │
│    ┌──────────────────┐    │
│    │  输入消息...      │    │  ← rounded large prompt input
│    │  + @ 工具    [↑] │    │  ← button row + send button
│    └──────────────────┘    │
│                            │
│   [建议1] [建议2] [建议3]   │  ← suggestion chips (existing)
└────────────────────────────┘
```

- `ChatWorkspace` uses `display: flex; flex-direction: column; align-items: center; justify-content: center;`
- `ModelPicker` is absolutely positioned at top-left.
- `PromptInput` receives `centered={true}` prop. Its outer wrapper participates in the centered flex layout.
- `ChatHistory` is NOT rendered.

### Chat Mode (isEmpty = false)

```
┌────────────────────────────┐
│ [ModelPicker]              │  ← top-left pill
│ ────────────────────────── │
│ 用户：你好                  │
│ AI：你好！很高兴为您服务    │  ← ChatHistory (scrollable)
│ ...                        │
│ ────────────────────────── │
│ ┌────────────────────────┐ │
│ │ 输入消息...    + @ 工具 [↑]│  ← PromptInput (bottom-fixed)
│ └────────────────────────┘ │
└────────────────────────────┘
```

- Restores existing layout: `ChatHistory` fills the scrollable area above, `PromptInput` sits at the bottom.
- `PromptInput` receives `centered={false}`.
- `ChatHistory` no longer renders `EmptyState`.

---

## Component Inventory

### New Components

| Component | Path | Responsibility |
|-----------|------|----------------|
| **ModelPicker** | `components/chat/ModelPicker.tsx` | Top-left pill showing current model name. Click to expand dropdown for provider/model selection. Reads from `config.providers`. |
| **AttachButton** | `components/chat/AttachButton.tsx` | "+" button. Triggers file picker. Reuses `AttachmentUploader` logic. |
| **MentionButton** | `components/chat/MentionButton.tsx` | "@" button. Placeholder — inserts `@` into textarea and focuses. |
| **ToolsButton** | `components/chat/ToolsButton.tsx` | "工具" pill button. Toggles `ToolsMenu` visibility. |
| **ToolsMenu** | `components/chat/ToolsMenu.tsx` | Popover menu below the "工具" button. Contains three grouped sections. |

### Modified Components

| Component | Changes |
|-----------|---------|
| **ChatWorkspace** | Add `isEmpty` branch. Empty mode: centered flex layout with `ModelPicker` + greeting + `PromptInput(centered)` + suggestion chips. Chat mode: existing layout. |
| **PromptInput** | Restyle to rounded large input box (border-radius 20px). Add bottom button row: left side `AttachButton` + `MentionButton` + `ToolsButton`; right side send button. Add `centered?: boolean` prop for layout participation. |
| **ChatHistory** | Remove `EmptyState` rendering. Now only renders message list + streaming reply + scroll-to-bottom button. |
| **EmptyState** | Greeting text and suggestion chips moved into `ChatWorkspace`'s empty-mode branch. Component can be removed or kept as a thin wrapper. |

---

## Tools Menu Content

The `ToolsMenu` popover has three grouped sections:

### 1. MCP 工具
- **Source**: `GET /api/tools`
- **Current backend status**: Stub — returns `[]`.
- **Frontend behavior**: Display tools in a scrollable list. If empty, show "暂无可用工具". Frontend data structures should be ready for when the backend populates this endpoint.

### 2. 预设命令
- **Source**: `GET /api/suggestions`
- **Content**: Returns an array of suggestion strings (e.g. "What can you help me with?").
- **Behavior**: Display as a list. Clicking an item sets the input text and triggers send.

### 3. Skills
- **Source**: `GET /api/skills`
- **Content**: Returns `{ name, description, enabled }` array.
- **Behavior**: Display enabled skills with name + description. Clicking inserts a skill invocation prompt into the textarea (exact format TBD, start with just the skill name).

---

## Styling (CSS Modules)

Hermind uses CSS Modules with CSS custom properties. No Tailwind is introduced.

| Element | Rules |
|---------|-------|
| Chat workspace container | `background: var(--bg)` |
| Rounded input box | `background: var(--surface-2)`; `border-radius: 20px`; `border: 1px solid var(--border)` |
| Textarea inside input | `background: transparent`; `resize: none`; `outline: none`; placeholder color `var(--muted)` |
| Button row buttons | Circular or pill-shaped; `hover: background var(--hover-tint)`; transition `var(--t-short)` |
| Send button (active) | `background: var(--text)`; `color: var(--accent-fg)` |
| Send button (disabled) | `background: var(--muted)`; `color: var(--bg)` |
| ModelPicker pill | `border-radius: 999px`; transparent bg; hover `var(--hover-tint)`; text `var(--muted)` |
| ToolsMenu popover | `background: var(--surface)`; `border: 1px solid var(--border)`; `border-radius: var(--r-lg)`; shadow; max-height with overflow-y |
| Section header in ToolsMenu | `color: var(--muted)`; uppercase; small font size |

---

## Data Flow

```
App.tsx
  └─ ChatWorkspace
       ├─ isEmpty ? Empty Mode : Chat Mode
       │
       ├─ ModelPicker ──→ reads config.providers (passed from App state)
       ├─ Greeting + SuggestionCards ──→ onClick → handleSend(text)
       ├─ PromptInput(centered={isEmpty})
       │    ├─ AttachButton ──→ file upload → onAttachmentsAdd
       │    ├─ MentionButton ──→ inserts "@" + focuses textarea
       │    ├─ ToolsButton ──→ toggles ToolsMenu
       │    │    └─ ToolsMenu
       │    │         ├─ MCP Tools (GET /api/tools)
       │    │         ├─ Preset Commands (GET /api/suggestions)
       │    │         └─ Skills (GET /api/skills)
       │    │              └─ onSelect → sets PromptInput text
       │    └─ SendButton ──→ onSubmit
       └─ ChatHistory ──→ message list only
```

All existing callbacks (`handleSend`, `handleStop`, `handleEdit`, `handleDelete`, `handleRegenerate`) remain unchanged in signature.

---

## Error Handling

| Scenario | Handling |
|----------|----------|
| `GET /api/tools` fails or returns empty | ToolsMenu shows "暂无可用工具" in MCP section. No user-visible error toast. |
| `GET /api/suggestions` fails | Preset Commands section is hidden. No toast. |
| `GET /api/skills` fails | Skills section is hidden. No toast. |
| File upload fails | Reuse existing Toast component with `t('chat.errorUpload')` or similar. |
| Send message fails | Reuse existing rollback + Toast mechanism. |

---

## Testing

| Component | What to test |
|-----------|--------------|
| `PromptInput` | Text input, Enter to submit, Shift+Enter for newline, attach button click, mention button inserts "@", tools button toggles menu, send button disabled when empty |
| `ToolsMenu` | Renders three sections when data loaded, handles empty states, clicking an item calls `onSelect`, closes on outside click |
| `ModelPicker` | Displays model name, expands/collapses dropdown, calls onChange when model selected |
| `ChatWorkspace` | Renders empty mode when no messages, switches to chat mode after first message, renders ModelPicker in both modes |
| Integration | Sending a suggestion chip from empty mode transitions to chat mode correctly |

---

## API Reference

| Endpoint | Method | Response | Used By |
|----------|--------|----------|---------|
| `/api/tools` | GET | `{ tools: [] }` | ToolsMenu → MCP Tools section |
| `/api/suggestions` | GET | `{ suggestions: string[] }` | ToolsMenu → Preset Commands section |
| `/api/skills` | GET | `{ skills: { name, description, enabled }[] }` | ToolsMenu → Skills section |
| `/api/config` | GET | Existing config shape | ModelPicker reads `providers` |

---

## Open Questions / Future Work

1. **ModelPicker model selection**: Currently Hermind's active model is determined by backend config. The picker may need a new API or use the existing config PUT mechanism to switch models.
2. **MCP Tools population**: `/api/tools` is a stub. The backend needs to expose the actual MCP tool registry when ready.
3. **Skill invocation format**: Clicking a skill in ToolsMenu should insert a proper invocation string (e.g., `/skill-name` or `@skill-name`). The exact syntax should align with Hermind's agent/skill system.
4. **Mention (`@`) functionality**: Currently a placeholder. Future work can implement agent/workspace mention similar to AnythingLLM.
