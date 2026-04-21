# Web Chat Frontend (Phase 2/3) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a React chat workspace to the existing web UI. Chat becomes the landing mode; the current seven config groups move into a Settings mode reached from a TopBar toggle. Consumes Phase 1's `POST /messages` / `POST /cancel` + the existing SSE stream.

**Architecture:** Two-mode shell selected by hash routing. Independent `chatState` reducer. SSE subscription in a hook. Markdown + GFM + shiki + KaTeX eager; Mermaid lazy-loaded. No attachments. Auth via existing `?t=<token>` query string (already accepted server-side).

**Tech Stack:** React 18, Zod, Vite, vitest + React Testing Library, `react-markdown`, `remark-gfm`, `remark-math`, `rehype-katex`, `katex`, `shiki`, `mermaid` (lazy).

**Spec:** `docs/superpowers/specs/2026-04-21-web-chat-frontend-design.md`

**Dependency:** Phase 1 (backend dispatch/cancel endpoints) must be merged before this plan ships to production. During implementation, tests stub the endpoints.

---

## File map

### Create

| Path | Responsibility |
|---|---|
| `web/src/state/chat.ts` | Chat reducer + action catalog + initial state |
| `web/src/state/chat.test.ts` | Reducer pure-function tests |
| `web/src/state/config.ts` | Move the current `web/src/state.ts` into a named module (rename + re-export). |
| `web/src/hooks/useChatStream.ts` | SSE subscription + event dispatch + token throttling |
| `web/src/hooks/useChatStream.test.ts` | |
| `web/src/hooks/useSessionList.ts` | GET /api/sessions + optimistic new-session |
| `web/src/hooks/useSessionList.test.ts` | |
| `web/src/components/chat/ChatWorkspace.tsx` | Root chat layout |
| `web/src/components/chat/ChatSidebar.tsx` | Left sidebar with session list + new button |
| `web/src/components/chat/NewChatButton.tsx` | |
| `web/src/components/chat/SessionList.tsx` | |
| `web/src/components/chat/SessionItem.tsx` | |
| `web/src/components/chat/ConversationHeader.tsx` | Title + ModelSelector |
| `web/src/components/chat/ModelSelector.tsx` | |
| `web/src/components/chat/MessageList.tsx` | Scroll container, stick-to-bottom |
| `web/src/components/chat/MessageBubble.tsx` | |
| `web/src/components/chat/MessageContent.tsx` | react-markdown pipeline |
| `web/src/components/chat/StreamingCursor.tsx` | |
| `web/src/components/chat/ToolCallCard.tsx` | |
| `web/src/components/chat/ComposerBar.tsx` | TextArea + send/stop buttons |
| `web/src/components/chat/SlashMenu.tsx` | Overlay when input starts with `/` |
| `web/src/components/chat/StopButton.tsx` | |
| `web/src/components/chat/markdown/CodeBlock.tsx` | Shiki highlight; delegates mermaid |
| `web/src/components/chat/markdown/MermaidBlock.tsx` | Lazy mermaid loader |
| `web/src/test/fakeEventSource.ts` | Minimal EventSource polyfill for vitest |

### Modify

| Path | Change |
|---|---|
| `web/src/App.tsx` | Add two-mode router; dispatch to chat or settings shell |
| `web/src/shell/hash.ts` | Parse `#/chat/:id` and `#/settings/:group`; migrate legacy hashes |
| `web/src/components/shell/Sidebar.tsx` | Rename file + default export to `SettingsSidebar` |
| `web/src/components/shell/ContentPanel.tsx` | Rename to `SettingsPanel` |
| `web/src/components/shell/TopBar.tsx` | Add Chat/Settings toggle |
| `web/src/api/schemas.ts` | Add message/session/stream-event zod schemas |
| `web/src/api/client.ts` | Add `apiPost` helper if missing |
| `web/package.json` | Add deps |
| `web/src/main.tsx` | Import `katex/dist/katex.min.css` |

### Do not touch

`web/src/components/groups/*` (config panels), `api/*` Go code (that's Phase 1's account), `cli/*` (Phase 3).

---

### Task 1: Install dependencies

**Files:**
- Modify: `web/package.json`, `web/pnpm-lock.yaml`

- [ ] **Step 1: Install**

```bash
cd web
pnpm add react-markdown remark-gfm remark-math rehype-katex katex shiki mermaid
```

- [ ] **Step 2: Verify**

```bash
pnpm list --depth 0 | grep -E 'react-markdown|remark-gfm|remark-math|rehype-katex|katex|shiki|mermaid'
```

Expected: all seven listed.

- [ ] **Step 3: Build + test**

```bash
pnpm build
pnpm test
```

Expected: both exit 0. Bundle size will grow; we'll check budgets in Task 24.

- [ ] **Step 4: Commit**

```bash
cd ..
git add web/package.json web/pnpm-lock.yaml
git commit -m "chore(web/deps): add markdown + shiki + katex + mermaid"
```

---

### Task 2: Import KaTeX CSS

**Files:**
- Modify: `web/src/main.tsx`

- [ ] **Step 1: Add the import**

At the top of `web/src/main.tsx`, after the existing CSS imports:

```ts
import 'katex/dist/katex.min.css';
```

- [ ] **Step 2: Build**

```bash
cd web && pnpm build
```

Expected: build succeeds; `dist/assets/*.css` now includes KaTeX styles.

- [ ] **Step 3: Commit**

```bash
cd .. && git add web/src/main.tsx
git commit -m "feat(web): load KaTeX stylesheet globally"
```

---

### Task 3: Extend hash.ts for two modes

**Files:**
- Modify: `web/src/shell/hash.ts`
- Modify: `web/src/shell/hash.test.ts`

- [ ] **Step 1: Read the current parser**

```bash
cat web/src/shell/hash.ts
```

Note the current `parseHash` signature. We'll extend it to return `{mode, groupId?, sessionId?}`.

- [ ] **Step 2: Write failing tests**

Append to `web/src/shell/hash.test.ts`:

```ts
import { describe, it, expect } from 'vitest';
import { parseHash, stringifyHash } from './hash';

describe('parseHash two-mode', () => {
  it('empty hash → chat mode, no sessionId', () => {
    expect(parseHash('')).toEqual({ mode: 'chat' });
  });
  it('#/chat → chat mode', () => {
    expect(parseHash('#/chat')).toEqual({ mode: 'chat' });
  });
  it('#/chat/abc → chat mode with sessionId', () => {
    expect(parseHash('#/chat/abc-123')).toEqual({ mode: 'chat', sessionId: 'abc-123' });
  });
  it('#/settings → settings mode, default group', () => {
    expect(parseHash('#/settings')).toEqual({ mode: 'settings', groupId: 'models' });
  });
  it('#/settings/gateway → settings mode + groupId', () => {
    expect(parseHash('#/settings/gateway')).toEqual({ mode: 'settings', groupId: 'gateway' });
  });
  it('legacy group hash #models → settings/models', () => {
    expect(parseHash('#models')).toEqual({ mode: 'settings', groupId: 'models' });
  });
});

describe('stringifyHash', () => {
  it('chat with no id', () => {
    expect(stringifyHash({ mode: 'chat' })).toBe('#/chat');
  });
  it('chat with id', () => {
    expect(stringifyHash({ mode: 'chat', sessionId: 'abc' })).toBe('#/chat/abc');
  });
  it('settings', () => {
    expect(stringifyHash({ mode: 'settings', groupId: 'memory' })).toBe('#/settings/memory');
  });
});
```

- [ ] **Step 3: Run — expect fail**

```bash
cd web && pnpm test -- hash
```

- [ ] **Step 4: Implement**

Replace `web/src/shell/hash.ts` content:

```ts
import type { GroupId } from './groups';

export type HashState =
  | { mode: 'chat'; sessionId?: string }
  | { mode: 'settings'; groupId: GroupId };

const LEGACY_GROUPS: GroupId[] = [
  'models',
  'gateway',
  'memory',
  'skills',
  'runtime',
  'advanced',
  'observability',
];

export function parseHash(raw: string): HashState {
  const h = (raw || '').replace(/^#\/?/, '');
  if (h === '' || h === 'chat') return { mode: 'chat' };
  if (h.startsWith('chat/')) {
    const id = h.slice('chat/'.length);
    return id ? { mode: 'chat', sessionId: id } : { mode: 'chat' };
  }
  if (h === 'settings') return { mode: 'settings', groupId: 'models' };
  if (h.startsWith('settings/')) {
    const g = h.slice('settings/'.length) as GroupId;
    return { mode: 'settings', groupId: LEGACY_GROUPS.includes(g) ? g : 'models' };
  }
  // Legacy: bare #models, #gateway, …
  if (LEGACY_GROUPS.includes(h as GroupId)) {
    return { mode: 'settings', groupId: h as GroupId };
  }
  return { mode: 'chat' };
}

export function stringifyHash(s: HashState): string {
  if (s.mode === 'chat') return s.sessionId ? `#/chat/${s.sessionId}` : '#/chat';
  return `#/settings/${s.groupId}`;
}

export function migrateLegacyHash(raw: string): string {
  const parsed = parseHash(raw);
  return stringifyHash(parsed);
}
```

- [ ] **Step 5: Run**

```bash
pnpm test -- hash
```

Expected: all new tests + existing ones PASS.

- [ ] **Step 6: Commit**

```bash
cd .. && git add web/src/shell/hash.ts web/src/shell/hash.test.ts
git commit -m "feat(web/shell): hash parses chat + settings two-mode"
```

---

### Task 4: Rename Sidebar → SettingsSidebar, ContentPanel → SettingsPanel

**Files:**
- Rename: `web/src/components/shell/Sidebar.tsx` → `SettingsSidebar.tsx`
- Rename: `web/src/components/shell/Sidebar.module.css` → `SettingsSidebar.module.css` (if exists)
- Rename: `web/src/components/shell/Sidebar.test.tsx` → `SettingsSidebar.test.tsx`
- Rename: `ContentPanel.tsx` → `SettingsPanel.tsx` (and test + css)
- Modify: `web/src/App.tsx` imports

- [ ] **Step 1: git mv**

```bash
cd web/src/components/shell
git mv Sidebar.tsx SettingsSidebar.tsx
git mv Sidebar.test.tsx SettingsSidebar.test.tsx
git mv Sidebar.module.css SettingsSidebar.module.css 2>/dev/null || true
git mv ContentPanel.tsx SettingsPanel.tsx
git mv ContentPanel.test.tsx SettingsPanel.test.tsx
```

- [ ] **Step 2: Update default exports / component names**

Inside `SettingsSidebar.tsx`, rename `export default function Sidebar` → `export default function SettingsSidebar`. Same for `SettingsPanel.tsx`.

- [ ] **Step 3: Fix all importers**

```bash
cd /Users/ranwei/workspace/go_work/hermes-agent-rewrite/hermind
grep -rn "from './shell/Sidebar'\|from './shell/ContentPanel'\|from '../shell/Sidebar'\|from '../shell/ContentPanel'" web/src
```

For each hit, update the path:
- `from '.../shell/Sidebar'` → `from '.../shell/SettingsSidebar'`
- `from '.../shell/ContentPanel'` → `from '.../shell/SettingsPanel'`

Same rename inside `Sidebar.module.css` imports.

- [ ] **Step 4: Run tests + type-check**

```bash
cd web && pnpm type-check && pnpm test
```

Expected: both pass. Renamed tests discover their components under new names.

- [ ] **Step 5: Commit**

```bash
cd ..
git add web
git commit -m "refactor(web/shell): rename Sidebar→SettingsSidebar, ContentPanel→SettingsPanel"
```

---

### Task 5: TopBar mode toggle

**Files:**
- Modify: `web/src/components/shell/TopBar.tsx`
- Modify: `web/src/components/shell/TopBar.test.tsx`

- [ ] **Step 1: Write failing test**

Append to TopBar.test.tsx:

```tsx
import { render, screen, fireEvent } from '@testing-library/react';
import TopBar from './TopBar';

it('shows Chat and Settings toggles; clicking Settings calls onModeChange', () => {
  const spy = vi.fn();
  render(<TopBar mode="chat" onModeChange={spy} dirtyCount={0} onApply={() => {}} />);
  const settings = screen.getByRole('button', { name: /settings/i });
  fireEvent.click(settings);
  expect(spy).toHaveBeenCalledWith('settings');
});
```

> If TopBar doesn't currently accept `mode` / `onModeChange`, the test documents the new contract.

- [ ] **Step 2: Extend TopBar**

Add `mode: 'chat' | 'settings'` and `onModeChange: (m: 'chat'|'settings') => void` props. In render:

```tsx
<div className={styles.modeToggle}>
  <button
    aria-pressed={mode === 'chat'}
    onClick={() => onModeChange('chat')}
  >Chat</button>
  <button
    aria-pressed={mode === 'settings'}
    onClick={() => onModeChange('settings')}
  >Settings</button>
</div>
```

Minimal CSS in TopBar.module.css:

```css
.modeToggle { display: flex; gap: 0.25rem; }
.modeToggle button[aria-pressed='true'] { font-weight: 600; }
```

- [ ] **Step 3: Run**

```bash
cd web && pnpm test -- TopBar
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
cd .. && git add web/src/components/shell/TopBar.tsx web/src/components/shell/TopBar.test.tsx web/src/components/shell/TopBar.module.css
git commit -m "feat(web/shell): TopBar chat/settings mode toggle"
```

---

### Task 6: API schemas + client POST helper

**Files:**
- Modify: `web/src/api/schemas.ts`
- Modify: `web/src/api/client.ts`

- [ ] **Step 1: Add schemas**

Append to `web/src/api/schemas.ts`:

```ts
export const MessageSubmitRequestSchema = z.object({
  text: z.string().min(1),
  model: z.string().optional(),
});
export type MessageSubmitRequest = z.infer<typeof MessageSubmitRequestSchema>;

export const MessageSubmitResponseSchema = z.object({
  session_id: z.string(),
  status: z.literal('accepted'),
});
export type MessageSubmitResponse = z.infer<typeof MessageSubmitResponseSchema>;

export const SessionSummarySchema = z.object({
  id: z.string(),
  title: z.string().optional(),
  updated_at: z.number().optional(),
});
export type SessionSummary = z.infer<typeof SessionSummarySchema>;

export const SessionsListResponseSchema = z.object({
  sessions: z.array(SessionSummarySchema),
  total: z.number().optional(),
});

export const ChatMessageSchema = z.object({
  id: z.string(),
  role: z.enum(['user', 'assistant', 'system', 'tool']),
  content: z.string(),
  timestamp: z.number().optional(),
  tool_calls: z.string().optional(),
});
export type ChatMessage = z.infer<typeof ChatMessageSchema>;

export const MessagesResponseSchema = z.object({
  messages: z.array(ChatMessageSchema),
  total: z.number().optional(),
});

// Stream events (from SSE). Discriminated union.
export const StreamEventSchema = z.object({
  type: z.string(),
  session_id: z.string(),
  data: z.unknown().optional(),
});
export type StreamEvent = z.infer<typeof StreamEventSchema>;
```

- [ ] **Step 2: Add apiPost helper**

In `web/src/api/client.ts`, add beside `apiFetch`:

```ts
export async function apiPost<T>(
  path: string,
  body: unknown,
  opts: { schema: z.ZodType<T>; signal?: AbortSignal } = { schema: z.any() as never }
): Promise<T> {
  const res = await fetch(path, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', ...authHeader() },
    body: JSON.stringify(body),
    signal: opts.signal,
  });
  if (!res.ok) {
    const text = await res.text();
    throw new ApiError(res.status, text);
  }
  const json = await res.json();
  return opts.schema.parse(json);
}

export async function apiPostNoBody(path: string, opts: { signal?: AbortSignal } = {}) {
  const res = await fetch(path, {
    method: 'POST',
    headers: authHeader(),
    signal: opts.signal,
  });
  if (!res.ok && res.status !== 204) {
    throw new ApiError(res.status, await res.text());
  }
}
```

(If `authHeader()` does not exist, extract it from the existing `apiFetch` function.)

- [ ] **Step 3: Build + type-check**

```bash
cd web && pnpm type-check && pnpm build
```

Expected: exit 0.

- [ ] **Step 4: Commit**

```bash
cd ..
git add web/src/api/schemas.ts web/src/api/client.ts
git commit -m "feat(web/api): schemas + POST helpers for chat endpoints"
```

---

### Task 7: Chat reducer + tests

**Files:**
- Create: `web/src/state/chat.ts`
- Create: `web/src/state/chat.test.ts`

- [ ] **Step 1: Write failing tests**

Create `web/src/state/chat.test.ts`:

```ts
import { describe, it, expect } from 'vitest';
import { chatReducer, initialChatState } from './chat';

describe('chatReducer', () => {
  it('session/select switches activeSessionId', () => {
    const s = chatReducer(initialChatState, {
      type: 'chat/session/select',
      sessionId: 'abc',
    });
    expect(s.activeSessionId).toBe('abc');
  });

  it('session/created prepends to sessions and activates', () => {
    const s = chatReducer(initialChatState, {
      type: 'chat/session/created',
      id: 'new-1',
      title: 'New conversation',
    });
    expect(s.sessions[0]?.id).toBe('new-1');
    expect(s.activeSessionId).toBe('new-1');
  });

  it('stream/start adds optimistic user message and sets running', () => {
    const s = chatReducer(
      { ...initialChatState, activeSessionId: 's1' },
      { type: 'chat/stream/start', sessionId: 's1', userText: 'hi' }
    );
    expect(s.streaming.status).toBe('running');
    expect(s.messagesBySession.s1[0].role).toBe('user');
    expect(s.messagesBySession.s1[0].content).toBe('hi');
  });

  it('stream/token appends to assistantDraft', () => {
    let s = chatReducer(initialChatState, {
      type: 'chat/stream/start', sessionId: 's1', userText: 'hi',
    });
    s = chatReducer(s, { type: 'chat/stream/token', delta: 'Hel' });
    s = chatReducer(s, { type: 'chat/stream/token', delta: 'lo' });
    expect(s.streaming.assistantDraft).toBe('Hello');
  });

  it('stream/complete promotes draft to messagesBySession', () => {
    let s = chatReducer(initialChatState, {
      type: 'chat/stream/start', sessionId: 's1', userText: 'hi',
    });
    s = chatReducer(s, { type: 'chat/stream/token', delta: 'Hi' });
    s = chatReducer(s, {
      type: 'chat/stream/complete', text: 'Hi', messageId: 'm1',
    });
    const msgs = s.messagesBySession.s1;
    expect(msgs.at(-1)).toMatchObject({ role: 'assistant', content: 'Hi', id: 'm1' });
    expect(s.streaming.status).toBe('idle');
    expect(s.streaming.assistantDraft).toBe('');
  });

  it('stream/cancelled keeps draft with truncated flag', () => {
    let s = chatReducer(initialChatState, {
      type: 'chat/stream/start', sessionId: 's1', userText: 'hi',
    });
    s = chatReducer(s, { type: 'chat/stream/token', delta: 'partial' });
    s = chatReducer(s, { type: 'chat/stream/cancelled' });
    const last = s.messagesBySession.s1.at(-1);
    expect(last?.role).toBe('assistant');
    expect(last?.content).toBe('partial');
    expect(last?.truncated).toBe(true);
    expect(s.streaming.status).toBe('idle');
  });

  it('stream/error keeps draft with truncated flag + error', () => {
    let s = chatReducer(initialChatState, {
      type: 'chat/stream/start', sessionId: 's1', userText: 'hi',
    });
    s = chatReducer(s, { type: 'chat/stream/error', message: 'boom' });
    expect(s.streaming.status).toBe('error');
    expect(s.streaming.error).toBe('boom');
  });

  it('stream/rollbackUserMessage undoes start', () => {
    let s = chatReducer(initialChatState, {
      type: 'chat/stream/start', sessionId: 's1', userText: 'hi',
    });
    s = chatReducer(s, { type: 'chat/stream/rollbackUserMessage', sessionId: 's1' });
    expect(s.messagesBySession.s1 ?? []).toHaveLength(0);
    expect(s.streaming.status).toBe('idle');
  });

  it('composer/setText + setModel', () => {
    let s = chatReducer(initialChatState, { type: 'chat/composer/setText', text: 'hello' });
    expect(s.composer.text).toBe('hello');
    s = chatReducer(s, { type: 'chat/composer/setModel', model: 'claude' });
    expect(s.composer.selectedModel).toBe('claude');
  });
});
```

- [ ] **Step 2: Run — expect fail**

```bash
cd web && pnpm test -- state/chat
```

- [ ] **Step 3: Implement**

Create `web/src/state/chat.ts`:

```ts
export type ChatMessage = {
  id: string;
  role: 'user' | 'assistant' | 'system' | 'tool';
  content: string;
  timestamp: number;
  toolCalls?: ToolCallSnapshot[];
  truncated?: true;
};

export type ToolCallSnapshot = {
  id: string;
  name: string;
  input: unknown;
  result?: string;
  state: 'running' | 'done' | 'error';
};

export type SessionSummary = {
  id: string;
  title: string;
  updatedAt: number;
};

export type ChatState = {
  activeSessionId: string | null;
  sessions: SessionSummary[];
  messagesBySession: Record<string, ChatMessage[]>;
  streaming: {
    sessionId: string | null;
    assistantDraft: string;
    toolCalls: ToolCallSnapshot[];
    status: 'idle' | 'running' | 'cancelling' | 'error';
    error: string | null;
  };
  composer: {
    text: string;
    selectedModel: string;
  };
};

export const initialChatState: ChatState = {
  activeSessionId: null,
  sessions: [],
  messagesBySession: {},
  streaming: {
    sessionId: null,
    assistantDraft: '',
    toolCalls: [],
    status: 'idle',
    error: null,
  },
  composer: { text: '', selectedModel: '' },
};

export type ChatAction =
  | { type: 'chat/session/select'; sessionId: string }
  | { type: 'chat/session/created'; id: string; title: string }
  | { type: 'chat/session/listLoaded'; sessions: SessionSummary[] }
  | { type: 'chat/messages/loaded'; sessionId: string; messages: ChatMessage[] }
  | { type: 'chat/stream/start'; sessionId: string; userText: string }
  | { type: 'chat/stream/rollbackUserMessage'; sessionId: string }
  | { type: 'chat/stream/token'; delta: string }
  | { type: 'chat/stream/toolCall'; call: ToolCallSnapshot }
  | { type: 'chat/stream/toolResult'; id: string; result: string }
  | { type: 'chat/stream/complete'; text: string; messageId: string }
  | { type: 'chat/stream/cancelled' }
  | { type: 'chat/stream/error'; message: string }
  | { type: 'chat/composer/setText'; text: string }
  | { type: 'chat/composer/setModel'; model: string };

export function chatReducer(state: ChatState, action: ChatAction): ChatState {
  switch (action.type) {
    case 'chat/session/select':
      return { ...state, activeSessionId: action.sessionId };

    case 'chat/session/created':
      return {
        ...state,
        sessions: [
          { id: action.id, title: action.title, updatedAt: Date.now() },
          ...state.sessions,
        ],
        activeSessionId: action.id,
      };

    case 'chat/session/listLoaded':
      return { ...state, sessions: action.sessions };

    case 'chat/messages/loaded':
      return {
        ...state,
        messagesBySession: {
          ...state.messagesBySession,
          [action.sessionId]: action.messages,
        },
      };

    case 'chat/stream/start': {
      const existing = state.messagesBySession[action.sessionId] ?? [];
      const userMsg: ChatMessage = {
        id: `draft-user-${Date.now()}`,
        role: 'user',
        content: action.userText,
        timestamp: Date.now(),
      };
      return {
        ...state,
        messagesBySession: {
          ...state.messagesBySession,
          [action.sessionId]: [...existing, userMsg],
        },
        streaming: {
          sessionId: action.sessionId,
          assistantDraft: '',
          toolCalls: [],
          status: 'running',
          error: null,
        },
      };
    }

    case 'chat/stream/rollbackUserMessage': {
      const existing = state.messagesBySession[action.sessionId] ?? [];
      // Drop the last user message and any draft-id assistant trailing
      const trimmed = existing.slice();
      while (trimmed.length > 0 && trimmed.at(-1)?.id.startsWith('draft-')) {
        trimmed.pop();
      }
      return {
        ...state,
        messagesBySession: {
          ...state.messagesBySession,
          [action.sessionId]: trimmed,
        },
        streaming: { ...initialChatState.streaming },
      };
    }

    case 'chat/stream/token':
      return {
        ...state,
        streaming: {
          ...state.streaming,
          assistantDraft: state.streaming.assistantDraft + action.delta,
        },
      };

    case 'chat/stream/toolCall':
      return {
        ...state,
        streaming: {
          ...state.streaming,
          toolCalls: [...state.streaming.toolCalls, action.call],
        },
      };

    case 'chat/stream/toolResult':
      return {
        ...state,
        streaming: {
          ...state.streaming,
          toolCalls: state.streaming.toolCalls.map((c) =>
            c.id === action.id ? { ...c, result: action.result, state: 'done' } : c
          ),
        },
      };

    case 'chat/stream/complete': {
      const sid = state.streaming.sessionId;
      if (!sid) return state;
      const existing = state.messagesBySession[sid] ?? [];
      const assistantMsg: ChatMessage = {
        id: action.messageId,
        role: 'assistant',
        content: action.text,
        timestamp: Date.now(),
        toolCalls: state.streaming.toolCalls,
      };
      return {
        ...state,
        messagesBySession: {
          ...state.messagesBySession,
          [sid]: [...existing, assistantMsg],
        },
        streaming: { ...initialChatState.streaming },
      };
    }

    case 'chat/stream/cancelled': {
      const sid = state.streaming.sessionId;
      if (!sid) return state;
      const existing = state.messagesBySession[sid] ?? [];
      const assistantMsg: ChatMessage = {
        id: `draft-assistant-cancelled-${Date.now()}`,
        role: 'assistant',
        content: state.streaming.assistantDraft,
        timestamp: Date.now(),
        toolCalls: state.streaming.toolCalls,
        truncated: true,
      };
      return {
        ...state,
        messagesBySession: {
          ...state.messagesBySession,
          [sid]: [...existing, assistantMsg],
        },
        streaming: { ...initialChatState.streaming },
      };
    }

    case 'chat/stream/error': {
      return {
        ...state,
        streaming: {
          ...state.streaming,
          status: 'error',
          error: action.message,
        },
      };
    }

    case 'chat/composer/setText':
      return { ...state, composer: { ...state.composer, text: action.text } };

    case 'chat/composer/setModel':
      return { ...state, composer: { ...state.composer, selectedModel: action.model } };

    default:
      return state;
  }
}
```

- [ ] **Step 4: Run**

```bash
pnpm test -- state/chat
```

Expected: all nine tests PASS.

- [ ] **Step 5: Commit**

```bash
cd ..
git add web/src/state/chat.ts web/src/state/chat.test.ts
git commit -m "feat(web/state): chat reducer + action catalog + tests"
```

---

### Task 8: Fake EventSource + useSessionList hook

**Files:**
- Create: `web/src/test/fakeEventSource.ts`
- Create: `web/src/hooks/useSessionList.ts`
- Create: `web/src/hooks/useSessionList.test.ts`

- [ ] **Step 1: Fake EventSource**

Create `web/src/test/fakeEventSource.ts`:

```ts
/// <reference lib="dom" />
type Handler = (ev: MessageEvent) => void;

export class FakeEventSource {
  static instances: FakeEventSource[] = [];
  readonly url: string;
  readyState = 0;
  onopen: (() => void) | null = null;
  onerror: (() => void) | null = null;
  onmessage: Handler | null = null;

  constructor(url: string) {
    this.url = url;
    FakeEventSource.instances.push(this);
    queueMicrotask(() => {
      this.readyState = 1;
      this.onopen?.();
    });
  }

  addEventListener(event: 'message' | 'open' | 'error', h: Handler | (() => void)) {
    if (event === 'message') this.onmessage = h as Handler;
    if (event === 'open') this.onopen = h as () => void;
    if (event === 'error') this.onerror = h as () => void;
  }

  dispatchMessage(data: unknown) {
    const ev = new MessageEvent('message', { data: JSON.stringify(data) });
    this.onmessage?.(ev);
  }

  close() {
    this.readyState = 2;
  }

  static install() {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    (globalThis as any).EventSource = FakeEventSource;
  }

  static reset() {
    FakeEventSource.instances = [];
  }
}
```

- [ ] **Step 2: Write hook test**

Create `web/src/hooks/useSessionList.test.ts`:

```ts
import { describe, it, expect, beforeEach, vi } from 'vitest';
import { renderHook, waitFor, act } from '@testing-library/react';
import { useSessionList } from './useSessionList';

beforeEach(() => {
  vi.restoreAllMocks();
});

describe('useSessionList', () => {
  it('loads sessions via GET /api/sessions', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(
      new Response(
        JSON.stringify({ sessions: [{ id: 's1', title: 'First' }] }),
        { status: 200, headers: { 'content-type': 'application/json' } }
      )
    );
    const { result } = renderHook(() => useSessionList());
    await waitFor(() => expect(result.current.sessions.length).toBe(1));
    expect(result.current.sessions[0].id).toBe('s1');
  });

  it('handles fetch errors gracefully', async () => {
    vi.spyOn(globalThis, 'fetch').mockRejectedValueOnce(new Error('network'));
    const { result } = renderHook(() => useSessionList());
    await waitFor(() => expect(result.current.error).toBeTruthy());
  });

  it('newSession generates uuid and prepends', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(
      new Response(JSON.stringify({ sessions: [] }), { status: 200 })
    );
    const { result } = renderHook(() => useSessionList());
    await waitFor(() => expect(result.current.sessions.length).toBe(0));
    let created = '';
    act(() => { created = result.current.newSession(); });
    expect(result.current.sessions[0].id).toBe(created);
    expect(created).toMatch(/^[0-9a-f-]{36}$/);
  });
});
```

- [ ] **Step 3: Run — expect fail**

```bash
cd web && pnpm test -- useSessionList
```

- [ ] **Step 4: Implement hook**

Create `web/src/hooks/useSessionList.ts`:

```ts
import { useCallback, useEffect, useState } from 'react';
import { apiFetch } from '../api/client';
import { SessionsListResponseSchema, type SessionSummary } from '../api/schemas';

export function useSessionList() {
  const [sessions, setSessions] = useState<SessionSummary[]>([]);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const ctrl = new AbortController();
    apiFetch('/api/sessions?limit=50', {
      schema: SessionsListResponseSchema,
      signal: ctrl.signal,
    })
      .then((r) => setSessions(r.sessions))
      .catch((e) => setError(e instanceof Error ? e.message : 'load failed'));
    return () => ctrl.abort();
  }, []);

  const newSession = useCallback(() => {
    const id = crypto.randomUUID();
    setSessions((prev) => [{ id, title: 'New conversation', updated_at: Date.now() }, ...prev]);
    return id;
  }, []);

  return { sessions, error, newSession };
}
```

> `apiFetch` may already accept a zod schema; if its return type differs, adjust the call to match.

- [ ] **Step 5: Run**

```bash
pnpm test -- useSessionList
```

Expected: all three pass.

- [ ] **Step 6: Commit**

```bash
cd ..
git add web/src/test/fakeEventSource.ts web/src/hooks/useSessionList.ts web/src/hooks/useSessionList.test.ts
git commit -m "feat(web/hooks): useSessionList + FakeEventSource test helper"
```

---

### Task 9: useChatStream hook

**Files:**
- Create: `web/src/hooks/useChatStream.ts`
- Create: `web/src/hooks/useChatStream.test.ts`

- [ ] **Step 1: Write tests**

```ts
import { describe, it, expect, beforeEach } from 'vitest';
import { renderHook, act, waitFor } from '@testing-library/react';
import { FakeEventSource } from '../test/fakeEventSource';
import { useChatStream } from './useChatStream';

beforeEach(() => {
  FakeEventSource.install();
  FakeEventSource.reset();
});

describe('useChatStream', () => {
  it('subscribes on mount, closes on unmount', () => {
    const dispatch = vi.fn();
    const { unmount } = renderHook(() => useChatStream('s1', dispatch));
    expect(FakeEventSource.instances.length).toBe(1);
    unmount();
    expect(FakeEventSource.instances[0].readyState).toBe(2);
  });

  it('reconnects on activeSessionId change', () => {
    const dispatch = vi.fn();
    const { rerender } = renderHook(({ id }) => useChatStream(id, dispatch), {
      initialProps: { id: 's1' as string | null },
    });
    expect(FakeEventSource.instances.length).toBe(1);
    rerender({ id: 's2' });
    expect(FakeEventSource.instances.length).toBe(2);
    expect(FakeEventSource.instances[0].readyState).toBe(2);
  });

  it('dispatches stream/token on token event', async () => {
    const dispatch = vi.fn();
    renderHook(() => useChatStream('s1', dispatch));
    await waitFor(() => expect(FakeEventSource.instances[0]?.readyState).toBe(1));
    act(() => {
      FakeEventSource.instances[0].dispatchMessage({
        type: 'token', session_id: 's1', data: { text: 'Hi' },
      });
    });
    // Throttled dispatch; give rAF a tick.
    await new Promise((r) => requestAnimationFrame(() => r(null)));
    expect(dispatch).toHaveBeenCalledWith(
      expect.objectContaining({ type: 'chat/stream/token', delta: 'Hi' })
    );
  });

  it('filters events for stale session_id', () => {
    const dispatch = vi.fn();
    renderHook(() => useChatStream('s1', dispatch));
    act(() => {
      FakeEventSource.instances[0].dispatchMessage({
        type: 'token', session_id: 'OTHER', data: { text: 'stale' },
      });
    });
    expect(dispatch).not.toHaveBeenCalled();
  });
});
```

- [ ] **Step 2: Run — fail**

```bash
cd web && pnpm test -- useChatStream
```

- [ ] **Step 3: Implement**

Create `web/src/hooks/useChatStream.ts`:

```ts
import { useEffect, useRef } from 'react';
import type { ChatAction } from '../state/chat';

type Dispatch = (a: ChatAction) => void;

export function useChatStream(sessionId: string | null, dispatch: Dispatch) {
  const tokenBufRef = useRef('');
  const rafPendingRef = useRef(false);

  useEffect(() => {
    if (!sessionId) return;
    const token = new URLSearchParams(window.location.search).get('t') ?? '';
    const es = new EventSource(
      `/api/sessions/${encodeURIComponent(sessionId)}/stream/sse?t=${encodeURIComponent(token)}`
    );

    function flushTokens() {
      rafPendingRef.current = false;
      if (tokenBufRef.current) {
        dispatch({ type: 'chat/stream/token', delta: tokenBufRef.current });
        tokenBufRef.current = '';
      }
    }

    es.onmessage = (ev) => {
      let parsed: { type?: string; session_id?: string; data?: Record<string, unknown> };
      try {
        parsed = JSON.parse(ev.data);
      } catch {
        return;
      }
      if (parsed.session_id && parsed.session_id !== sessionId) return;
      switch (parsed.type) {
        case 'token': {
          const d = parsed.data as { text?: string } | undefined;
          if (typeof d?.text === 'string') {
            tokenBufRef.current += d.text;
            if (!rafPendingRef.current) {
              rafPendingRef.current = true;
              requestAnimationFrame(flushTokens);
            }
          }
          break;
        }
        case 'tool_call': {
          const d = parsed.data as Record<string, unknown>;
          dispatch({
            type: 'chat/stream/toolCall',
            call: {
              id: String(d.id ?? d.tool_use_id ?? Date.now()),
              name: String(d.name ?? 'tool'),
              input: d.input ?? d,
              state: 'running',
            },
          });
          break;
        }
        case 'tool_result': {
          const d = parsed.data as { call?: { id?: string }; result?: string } | undefined;
          dispatch({
            type: 'chat/stream/toolResult',
            id: String(d?.call?.id ?? Date.now()),
            result: String(d?.result ?? ''),
          });
          break;
        }
        case 'message_complete': {
          flushTokens();
          const d = parsed.data as { assistant_text?: string; message_id?: string } | undefined;
          dispatch({
            type: 'chat/stream/complete',
            text: String(d?.assistant_text ?? ''),
            messageId: String(d?.message_id ?? `complete-${Date.now()}`),
          });
          break;
        }
        case 'status': {
          const d = parsed.data as { state?: string; error?: string } | undefined;
          if (d?.state === 'cancelled') {
            flushTokens();
            dispatch({ type: 'chat/stream/cancelled' });
          } else if (d?.state === 'error') {
            dispatch({ type: 'chat/stream/error', message: String(d.error ?? 'error') });
          }
          break;
        }
      }
    };

    return () => {
      es.close();
    };
  }, [sessionId, dispatch]);
}
```

- [ ] **Step 4: Run**

```bash
pnpm test -- useChatStream
```

Expected: all four tests PASS.

- [ ] **Step 5: Commit**

```bash
cd ..
git add web/src/hooks/useChatStream.ts web/src/hooks/useChatStream.test.ts
git commit -m "feat(web/hooks): useChatStream subscribes SSE + token throttle"
```

---

### Task 10: ChatWorkspace + ChatSidebar scaffolding

Wire the top-level components. Most child components are stubs for now, refined in later tasks.

**Files:**
- Create: `web/src/components/chat/ChatWorkspace.tsx`
- Create: `web/src/components/chat/ChatSidebar.tsx`
- Create: `web/src/components/chat/NewChatButton.tsx`
- Create: `web/src/components/chat/SessionList.tsx`
- Create: `web/src/components/chat/SessionItem.tsx`

- [ ] **Step 1: Scaffold files**

```tsx
// web/src/components/chat/NewChatButton.tsx
type Props = { onClick: () => void };
export default function NewChatButton({ onClick }: Props) {
  return <button type="button" onClick={onClick}>+ New conversation</button>;
}
```

```tsx
// web/src/components/chat/SessionItem.tsx
import type { SessionSummary } from '../../state/chat';
type Props = {
  session: SessionSummary;
  active: boolean;
  onClick: (id: string) => void;
};
export default function SessionItem({ session, active, onClick }: Props) {
  return (
    <li>
      <button
        type="button"
        aria-pressed={active}
        onClick={() => onClick(session.id)}
      >
        {session.title || 'New conversation'}
      </button>
    </li>
  );
}
```

```tsx
// web/src/components/chat/SessionList.tsx
import SessionItem from './SessionItem';
import type { SessionSummary } from '../../state/chat';
type Props = {
  sessions: SessionSummary[];
  activeId: string | null;
  onSelect: (id: string) => void;
};
export default function SessionList({ sessions, activeId, onSelect }: Props) {
  return (
    <ul>
      {sessions.map((s) => (
        <SessionItem key={s.id} session={s} active={s.id === activeId} onClick={onSelect} />
      ))}
    </ul>
  );
}
```

```tsx
// web/src/components/chat/ChatSidebar.tsx
import NewChatButton from './NewChatButton';
import SessionList from './SessionList';
import type { SessionSummary } from '../../state/chat';

type Props = {
  sessions: SessionSummary[];
  activeId: string | null;
  onSelect: (id: string) => void;
  onNew: () => void;
};

export default function ChatSidebar({ sessions, activeId, onSelect, onNew }: Props) {
  return (
    <aside>
      <NewChatButton onClick={onNew} />
      <SessionList sessions={sessions} activeId={activeId} onSelect={onSelect} />
    </aside>
  );
}
```

```tsx
// web/src/components/chat/ChatWorkspace.tsx
import { useReducer } from 'react';
import ChatSidebar from './ChatSidebar';
import { useSessionList } from '../../hooks/useSessionList';
import { chatReducer, initialChatState } from '../../state/chat';

type Props = {
  sessionId: string | null;
  onChangeSession: (id: string) => void;
};

export default function ChatWorkspace({ sessionId, onChangeSession }: Props) {
  const [state, dispatch] = useReducer(chatReducer, initialChatState);
  const { sessions, newSession } = useSessionList();

  return (
    <div style={{ display: 'flex', height: '100%' }}>
      <ChatSidebar
        sessions={sessions}
        activeId={sessionId}
        onSelect={onChangeSession}
        onNew={() => {
          const id = newSession();
          onChangeSession(id);
        }}
      />
      <main style={{ flex: 1, padding: '1rem' }}>
        <p>Chat main — session: {sessionId ?? '(none)'}</p>
        <p>Messages: {state.messagesBySession[sessionId ?? '']?.length ?? 0}</p>
      </main>
    </div>
  );
}
```

- [ ] **Step 2: Wire into App**

In `web/src/App.tsx`, after the existing `useEffect` + `dispatch` setup, add mode-based render (reference — adjust to match surrounding code):

```tsx
import { parseHash, stringifyHash } from './shell/hash';
import ChatWorkspace from './components/chat/ChatWorkspace';

// inside App:
const [hash, setHash] = useState(() => parseHash(window.location.hash));
useEffect(() => {
  const on = () => setHash(parseHash(window.location.hash));
  window.addEventListener('hashchange', on);
  return () => window.removeEventListener('hashchange', on);
}, []);

if (hash.mode === 'chat') {
  return (
    <>
      <TopBar mode="chat" onModeChange={(m) => {
        window.location.hash = stringifyHash(
          m === 'chat' ? { mode: 'chat' } : { mode: 'settings', groupId: 'models' }
        );
      }} dirtyCount={0} onApply={() => {}} />
      <ChatWorkspace
        sessionId={hash.sessionId ?? null}
        onChangeSession={(id) => {
          window.location.hash = stringifyHash({ mode: 'chat', sessionId: id });
        }}
      />
    </>
  );
}
// settings render path: existing SettingsSidebar + SettingsPanel
```

- [ ] **Step 3: Build**

```bash
cd web && pnpm type-check && pnpm build
```

Expected: exit 0.

- [ ] **Step 4: Commit**

```bash
cd ..
git add web/src
git commit -m "feat(web/chat): ChatWorkspace + sidebar scaffolding + App mode routing"
```

---

### Task 11: ModelSelector + ConversationHeader

**Files:**
- Create: `web/src/components/chat/ModelSelector.tsx`
- Create: `web/src/components/chat/ConversationHeader.tsx`

- [ ] **Step 1: Implement**

```tsx
// ModelSelector.tsx
type Props = {
  value: string;
  options: string[];
  onChange: (v: string) => void;
};
export default function ModelSelector({ value, options, onChange }: Props) {
  return (
    <select value={value} onChange={(e) => onChange(e.target.value)}>
      {options.map((o) => <option key={o} value={o}>{o}</option>)}
    </select>
  );
}
```

```tsx
// ConversationHeader.tsx
import ModelSelector from './ModelSelector';
type Props = {
  title: string;
  model: string;
  modelOptions: string[];
  onModelChange: (v: string) => void;
};
export default function ConversationHeader({ title, model, modelOptions, onModelChange }: Props) {
  return (
    <header style={{ display: 'flex', justifyContent: 'space-between', padding: '0.5rem' }}>
      <h2>{title}</h2>
      <ModelSelector value={model} options={modelOptions} onChange={onModelChange} />
    </header>
  );
}
```

- [ ] **Step 2: Hook up in ChatWorkspace**

Replace the `<main>` stub in `ChatWorkspace.tsx` with:

```tsx
<main style={{ flex: 1, display: 'flex', flexDirection: 'column' }}>
  <ConversationHeader
    title={sessions.find((s) => s.id === sessionId)?.title ?? 'New conversation'}
    model={state.composer.selectedModel}
    modelOptions={['', 'claude-opus-4-7', 'claude-sonnet-4-6', 'gpt-4']}
    onModelChange={(m) => dispatch({ type: 'chat/composer/setModel', model: m })}
  />
  {/* MessageList + ComposerBar added in later tasks */}
</main>
```

- [ ] **Step 3: Build + test**

```bash
cd web && pnpm build
```

- [ ] **Step 4: Commit**

```bash
cd .. && git add web/src/components/chat
git commit -m "feat(web/chat): ConversationHeader + ModelSelector"
```

---

### Task 12: MessageBubble + MessageList

**Files:**
- Create: `web/src/components/chat/MessageBubble.tsx`
- Create: `web/src/components/chat/MessageList.tsx`
- Create: `web/src/components/chat/StreamingCursor.tsx`

- [ ] **Step 1: Create bubble + cursor**

```tsx
// StreamingCursor.tsx
export default function StreamingCursor() {
  return <span aria-hidden="true" style={{ opacity: 0.6 }}>▌</span>;
}
```

```tsx
// MessageBubble.tsx
import type { ChatMessage } from '../../state/chat';
type Props = { message: ChatMessage; streaming?: boolean };
export default function MessageBubble({ message, streaming }: Props) {
  const style = message.role === 'user'
    ? { textAlign: 'right' as const, padding: '0.5rem' }
    : { textAlign: 'left' as const, padding: '0.5rem' };
  return (
    <div style={style} data-role={message.role}>
      <pre style={{ whiteSpace: 'pre-wrap' }}>{message.content}</pre>
      {streaming && <StreamingCursor />}
      {message.truncated && <span style={{ color: '#a33', fontSize: '0.85em' }}> — interrupted</span>}
    </div>
  );
}
```

> Markdown rendering (MessageContent) replaces the `<pre>` in Task 14. For now plain text is fine.

```tsx
// MessageList.tsx
import { useEffect, useRef } from 'react';
import MessageBubble from './MessageBubble';
import StreamingCursor from './StreamingCursor';
import type { ChatMessage } from '../../state/chat';
type Props = {
  messages: ChatMessage[];
  streamingDraft: string;
  streamingSessionId: string | null;
  activeSessionId: string | null;
};
export default function MessageList({
  messages, streamingDraft, streamingSessionId, activeSessionId,
}: Props) {
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    ref.current?.scrollTo({ top: ref.current.scrollHeight });
  }, [messages, streamingDraft]);

  const showStreamingBubble = streamingSessionId === activeSessionId && streamingDraft;

  return (
    <div ref={ref} style={{ flex: 1, overflowY: 'auto' }}>
      {messages.map((m) => (
        <MessageBubble key={m.id} message={m} />
      ))}
      {showStreamingBubble && (
        <div data-role="assistant" style={{ padding: '0.5rem' }}>
          <pre style={{ whiteSpace: 'pre-wrap' }}>
            {streamingDraft}
            <StreamingCursor />
          </pre>
        </div>
      )}
    </div>
  );
}
```

- [ ] **Step 2: Wire into ChatWorkspace**

Between `<ConversationHeader>` and the (future) composer, add:

```tsx
<MessageList
  messages={state.messagesBySession[sessionId ?? ''] ?? []}
  streamingDraft={state.streaming.assistantDraft}
  streamingSessionId={state.streaming.sessionId}
  activeSessionId={sessionId}
/>
```

- [ ] **Step 3: Build**

```bash
cd web && pnpm build
```

- [ ] **Step 4: Commit**

```bash
cd .. && git add web/src/components/chat
git commit -m "feat(web/chat): MessageList + MessageBubble + StreamingCursor"
```

---

### Task 13: ComposerBar + StopButton + SendButton

**Files:**
- Create: `web/src/components/chat/ComposerBar.tsx`
- Create: `web/src/components/chat/StopButton.tsx`

- [ ] **Step 1: StopButton**

```tsx
// StopButton.tsx
type Props = { visible: boolean; onClick: () => void };
export default function StopButton({ visible, onClick }: Props) {
  if (!visible) return null;
  return <button type="button" onClick={onClick}>■ Stop</button>;
}
```

- [ ] **Step 2: ComposerBar**

```tsx
// ComposerBar.tsx
import { useCallback, useRef } from 'react';
import StopButton from './StopButton';

type Props = {
  text: string;
  onChangeText: (v: string) => void;
  onSend: () => void;
  onStop: () => void;
  disabled: boolean;       // true when no API key configured
  streaming: boolean;      // shows Stop instead of Send
};

export default function ComposerBar({
  text, onChangeText, onSend, onStop, disabled, streaming,
}: Props) {
  const inputRef = useRef<HTMLTextAreaElement>(null);
  const handleKey = useCallback((e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      if (text.trim()) onSend();
    }
  }, [text, onSend]);

  return (
    <div style={{ display: 'flex', gap: '0.5rem', padding: '0.5rem', borderTop: '1px solid #ccc' }}>
      <textarea
        ref={inputRef}
        value={text}
        onChange={(e) => onChangeText(e.target.value)}
        onKeyDown={handleKey}
        placeholder={disabled ? 'Configure a provider in Settings →' : 'Type a message. Shift+Enter for newline.'}
        disabled={disabled}
        rows={3}
        style={{ flex: 1, resize: 'vertical', maxHeight: '16rem' }}
      />
      {streaming ? (
        <StopButton visible onClick={onStop} />
      ) : (
        <button
          type="button"
          onClick={onSend}
          disabled={disabled || !text.trim()}
        >Send</button>
      )}
    </div>
  );
}
```

- [ ] **Step 3: Wire into ChatWorkspace**

At the bottom of the `<main>` element:

```tsx
<ComposerBar
  text={state.composer.text}
  onChangeText={(t) => dispatch({ type: 'chat/composer/setText', text: t })}
  onSend={() => handleSend()}
  onStop={() => handleStop()}
  disabled={false /* TODO: check config */}
  streaming={state.streaming.status === 'running'}
/>
```

And add `handleSend` / `handleStop` inside `ChatWorkspace`:

```tsx
import { apiPost, apiPostNoBody } from '../../api/client';
import { MessageSubmitResponseSchema } from '../../api/schemas';
import { useChatStream } from '../../hooks/useChatStream';

// inside component
useChatStream(sessionId, dispatch);

async function handleSend() {
  if (!sessionId) return;
  const text = state.composer.text.trim();
  if (!text) return;
  dispatch({ type: 'chat/composer/setText', text: '' });
  dispatch({ type: 'chat/stream/start', sessionId, userText: text });
  try {
    await apiPost(`/api/sessions/${encodeURIComponent(sessionId)}/messages`,
      { text, model: state.composer.selectedModel || undefined },
      { schema: MessageSubmitResponseSchema });
  } catch (err) {
    dispatch({ type: 'chat/stream/rollbackUserMessage', sessionId });
    console.error('send failed', err);
    // Toast handling in Task 18
  }
}

async function handleStop() {
  if (!sessionId) return;
  try {
    await apiPostNoBody(`/api/sessions/${encodeURIComponent(sessionId)}/cancel`);
  } catch (err) {
    console.warn('cancel failed', err);
  }
}
```

- [ ] **Step 4: Build**

```bash
cd web && pnpm build
```

- [ ] **Step 5: Commit**

```bash
cd .. && git add web/src/components/chat
git commit -m "feat(web/chat): ComposerBar + StopButton + send/stop wiring"
```

---

### Task 14: MessageContent — markdown + code highlight

**Files:**
- Create: `web/src/components/chat/MessageContent.tsx`
- Create: `web/src/components/chat/markdown/CodeBlock.tsx`

- [ ] **Step 1: CodeBlock with shiki**

```tsx
// web/src/components/chat/markdown/CodeBlock.tsx
import { useEffect, useState } from 'react';
import { codeToHtml } from 'shiki';
import MermaidBlock from './MermaidBlock';

type Props = {
  language: string;
  code: string;
};

export default function CodeBlock({ language, code }: Props) {
  const [html, setHtml] = useState<string | null>(null);
  useEffect(() => {
    if (language === 'mermaid') return;
    let cancelled = false;
    codeToHtml(code, { lang: language || 'text', theme: 'github-light' })
      .then((h) => { if (!cancelled) setHtml(h); })
      .catch(() => { if (!cancelled) setHtml(null); });
    return () => { cancelled = true; };
  }, [language, code]);

  if (language === 'mermaid') {
    return <MermaidBlock chart={code} />;
  }
  if (html) return <div dangerouslySetInnerHTML={{ __html: html }} />;
  return <pre><code>{code}</code></pre>;
}
```

- [ ] **Step 2: MermaidBlock (lazy)**

```tsx
// web/src/components/chat/markdown/MermaidBlock.tsx
import { useEffect, useRef } from 'react';

let mermaidPromise: Promise<typeof import('mermaid').default> | null = null;
function loadMermaid() {
  if (!mermaidPromise) {
    mermaidPromise = import('mermaid').then((m) => {
      m.default.initialize({ startOnLoad: false, theme: 'default' });
      return m.default;
    });
  }
  return mermaidPromise;
}

type Props = { chart: string };

export default function MermaidBlock({ chart }: Props) {
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    let cancelled = false;
    loadMermaid().then((m) => {
      if (cancelled || !ref.current) return;
      const id = `m-${Date.now()}-${Math.random().toString(36).slice(2, 7)}`;
      m.render(id, chart).then(({ svg }) => {
        if (!cancelled && ref.current) ref.current.innerHTML = svg;
      }).catch(() => {});
    });
    return () => { cancelled = true; };
  }, [chart]);
  return <div ref={ref} data-mermaid-container />;
}
```

- [ ] **Step 3: MessageContent**

```tsx
// web/src/components/chat/MessageContent.tsx
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import remarkMath from 'remark-math';
import rehypeKatex from 'rehype-katex';
import CodeBlock from './markdown/CodeBlock';

type Props = { content: string };

export default function MessageContent({ content }: Props) {
  return (
    <ReactMarkdown
      remarkPlugins={[remarkGfm, remarkMath]}
      rehypePlugins={[rehypeKatex]}
      components={{
        code({ inline, className, children }) {
          if (inline) return <code className={className}>{children}</code>;
          const lang = /language-(\w+)/.exec(className || '')?.[1] ?? 'text';
          const text = String(children).replace(/\n$/, '');
          return <CodeBlock language={lang} code={text} />;
        },
      }}
    >{content}</ReactMarkdown>
  );
}
```

- [ ] **Step 4: Replace the `<pre>` stub in MessageBubble**

In `MessageBubble.tsx`, replace `<pre>{message.content}</pre>` with `<MessageContent content={message.content} />`. Same for the streaming bubble in `MessageList.tsx`.

- [ ] **Step 5: Add smoke test**

Create `web/src/components/chat/MessageContent.test.tsx`:

```tsx
import { render, screen, waitFor } from '@testing-library/react';
import MessageContent from './MessageContent';

it('renders bold markdown', () => {
  render(<MessageContent content="**hello**" />);
  expect(screen.getByText('hello')).toHaveStyle({ fontWeight: 'bold' });
});

it('renders inline math as katex span', () => {
  const { container } = render(<MessageContent content="$x^2$" />);
  expect(container.querySelector('.katex')).not.toBeNull();
});

it('renders fenced code through shiki (async)', async () => {
  const { container } = render(<MessageContent content={'```python\nprint(1)\n```'} />);
  await waitFor(() => {
    expect(container.querySelector('pre') || container.querySelector('[class*="shiki"]')).not.toBeNull();
  });
});
```

- [ ] **Step 6: Run**

```bash
cd web && pnpm test -- MessageContent
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
cd ..
git add web/src/components/chat
git commit -m "feat(web/chat): MessageContent markdown + shiki + katex + lazy mermaid"
```

---

### Task 15: ToolCallCard

**Files:**
- Create: `web/src/components/chat/ToolCallCard.tsx`

- [ ] **Step 1: Implement**

```tsx
import { useState } from 'react';
import type { ToolCallSnapshot } from '../../state/chat';

type Props = { call: ToolCallSnapshot };

export default function ToolCallCard({ call }: Props) {
  const [open, setOpen] = useState(false);
  return (
    <div style={{ border: '1px solid #aaa', borderRadius: 4, padding: '0.25rem 0.5rem', margin: '0.25rem 0' }}>
      <button type="button" onClick={() => setOpen((o) => !o)}>
        {open ? '▼' : '▶'} tool: {call.name} ({call.state})
      </button>
      {open && (
        <div style={{ marginTop: '0.25rem' }}>
          <div><strong>input:</strong> <code>{JSON.stringify(call.input)}</code></div>
          {call.result && (
            <div><strong>result:</strong> <pre style={{ whiteSpace: 'pre-wrap' }}>{call.result}</pre></div>
          )}
        </div>
      )}
    </div>
  );
}
```

- [ ] **Step 2: Render in MessageBubble when present**

In `MessageBubble.tsx`, after `<MessageContent …>`:

```tsx
{message.toolCalls?.map((c) => <ToolCallCard key={c.id} call={c} />)}
```

Add the import for `ToolCallCard`.

- [ ] **Step 3: Also render streaming tool calls in MessageList**

In the `showStreamingBubble` branch of `MessageList.tsx`, add:

```tsx
{state.streaming.toolCalls.map((c) => <ToolCallCard key={c.id} call={c} />)}
```

(Note: requires passing `streamingToolCalls` as a prop. Update `MessageList` props + `ChatWorkspace` accordingly.)

- [ ] **Step 4: Build**

```bash
cd web && pnpm build
```

- [ ] **Step 5: Commit**

```bash
cd ..
git add web/src/components/chat
git commit -m "feat(web/chat): ToolCallCard render in bubbles + streaming"
```

---

### Task 16: SlashMenu

**Files:**
- Create: `web/src/components/chat/SlashMenu.tsx`
- Modify: `web/src/components/chat/ComposerBar.tsx`

- [ ] **Step 1: SlashMenu component**

```tsx
// SlashMenu.tsx
import { useEffect, useState } from 'react';

export type SlashCommand = {
  id: string;
  label: string;
  run: () => void;
};

type Props = { commands: SlashCommand[]; onClose: () => void };

export default function SlashMenu({ commands, onClose }: Props) {
  const [idx, setIdx] = useState(0);
  useEffect(() => {
    const on = (e: KeyboardEvent) => {
      if (e.key === 'ArrowDown') { e.preventDefault(); setIdx((i) => Math.min(i + 1, commands.length - 1)); }
      else if (e.key === 'ArrowUp') { e.preventDefault(); setIdx((i) => Math.max(i - 1, 0)); }
      else if (e.key === 'Enter') { e.preventDefault(); commands[idx]?.run(); onClose(); }
      else if (e.key === 'Escape') { e.preventDefault(); onClose(); }
    };
    window.addEventListener('keydown', on);
    return () => window.removeEventListener('keydown', on);
  }, [commands, idx, onClose]);

  return (
    <ul style={{ position: 'absolute', bottom: '100%', background: 'white', border: '1px solid #ccc' }}>
      {commands.map((c, i) => (
        <li key={c.id} style={{ background: i === idx ? '#eef' : 'transparent', padding: '0.25rem 0.5rem' }}>
          /{c.label}
        </li>
      ))}
    </ul>
  );
}
```

- [ ] **Step 2: Wire into ComposerBar**

Inside `ComposerBar`, detect a leading `/` and render the menu. Example addition near the textarea:

```tsx
const slashOpen = text.startsWith('/');
// ... inside the JSX
{slashOpen && (
  <SlashMenu
    commands={[
      { id: 'new', label: 'new', run: () => onSlashCommand('new') },
      { id: 'clear', label: 'clear', run: () => onChangeText('') },
      { id: 'settings', label: 'settings', run: () => onSlashCommand('settings') },
      { id: 'model', label: 'model', run: () => onSlashCommand('model') },
    ]}
    onClose={() => onChangeText(text.replace(/^\//, ''))}
  />
)}
```

Add `onSlashCommand: (cmd: string) => void` to `ComposerBar` props. `ChatWorkspace` passes a handler that dispatches hash navigation for `'settings'`, triggers `newSession()` for `'new'`, etc.

- [ ] **Step 3: Build**

```bash
cd web && pnpm build
```

- [ ] **Step 4: Commit**

```bash
cd ..
git add web/src/components/chat
git commit -m "feat(web/chat): SlashMenu with /new /clear /settings /model"
```

---

### Task 17: Messages history fetch

When user selects a session that's not cached, fetch its messages.

**Files:**
- Modify: `web/src/components/chat/ChatWorkspace.tsx`
- Modify: `web/src/api/schemas.ts` (already has MessagesResponseSchema from Task 6)

- [ ] **Step 1: Add effect**

In `ChatWorkspace`, add:

```tsx
import { apiFetch } from '../../api/client';
import { MessagesResponseSchema } from '../../api/schemas';

useEffect(() => {
  if (!sessionId) return;
  if (state.messagesBySession[sessionId]) return; // cached
  const ctrl = new AbortController();
  apiFetch(`/api/sessions/${encodeURIComponent(sessionId)}/messages?limit=200`, {
    schema: MessagesResponseSchema, signal: ctrl.signal,
  }).then((r) => {
    dispatch({
      type: 'chat/messages/loaded',
      sessionId,
      messages: r.messages.map((m) => ({
        id: m.id,
        role: m.role,
        content: m.content,
        timestamp: m.timestamp ?? 0,
      })),
    });
  }).catch(() => {});
  return () => ctrl.abort();
}, [sessionId]);
```

- [ ] **Step 2: Build**

```bash
cd web && pnpm build
```

- [ ] **Step 3: Commit**

```bash
cd ..
git add web/src/components/chat/ChatWorkspace.tsx
git commit -m "feat(web/chat): fetch message history on session select"
```

---

### Task 18: Error toasts + busy / no-provider handling

Lightweight toast system since none exists; a `useState<string[]>` at the top-level App is fine.

**Files:**
- Modify: `web/src/components/chat/ChatWorkspace.tsx`
- Create: `web/src/components/chat/Toast.tsx`

- [ ] **Step 1: Toast component**

```tsx
// Toast.tsx
type Props = { message: string; onDismiss: () => void };
export default function Toast({ message, onDismiss }: Props) {
  return (
    <div role="alert" style={{ position: 'fixed', bottom: '1rem', right: '1rem', background: '#333', color: 'white', padding: '0.5rem 1rem', borderRadius: 4 }}>
      {message}
      <button type="button" onClick={onDismiss} style={{ marginLeft: '1rem', color: 'white' }}>×</button>
    </div>
  );
}
```

- [ ] **Step 2: Map error codes in handleSend**

Replace `handleSend`'s catch branch:

```tsx
} catch (err) {
  dispatch({ type: 'chat/stream/rollbackUserMessage', sessionId });
  if (err instanceof ApiError) {
    if (err.status === 409) setToast('Session is busy — wait for current response');
    else if (err.status === 503) setToast('Provider not configured — open Settings');
    else setToast('Send failed: ' + err.message);
  } else {
    setToast('Send failed');
  }
}
```

Add a local `const [toast, setToast] = useState<string | null>(null)` and render `{toast && <Toast message={toast} onDismiss={() => setToast(null)} />}` alongside the main layout.

Import `ApiError` from `../../api/client`.

- [ ] **Step 3: Build**

```bash
cd web && pnpm build
```

- [ ] **Step 4: Commit**

```bash
cd ..
git add web/src/components/chat
git commit -m "feat(web/chat): error toasts for 409/503/generic send failures"
```

---

### Task 19: Integration test via App.test.tsx

**Files:**
- Modify: `web/src/App.test.tsx`

- [ ] **Step 1: Add happy-path test**

Append:

```tsx
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { FakeEventSource } from './test/fakeEventSource';
import App from './App';

it('chat mode: sends message + receives streamed tokens + renders assistant message', async () => {
  FakeEventSource.install();
  FakeEventSource.reset();

  // Mocks: GET config, schemas, sessions; POST messages accepted
  const fetchMock = vi.spyOn(globalThis, 'fetch').mockImplementation(async (input) => {
    const url = typeof input === 'string' ? input : input.url;
    if (url.includes('/api/config/schema')) return new Response(JSON.stringify({ sections: [] }));
    if (url.includes('/api/platforms/schema')) return new Response(JSON.stringify({ descriptors: [] }));
    if (url.includes('/api/config')) return new Response(JSON.stringify({ config: {} }));
    if (url.includes('/api/sessions?limit=50')) return new Response(JSON.stringify({ sessions: [] }));
    if (url.endsWith('/messages?limit=200')) return new Response(JSON.stringify({ messages: [] }));
    if (url.endsWith('/messages') && input instanceof Request && input.method === 'POST') {
      return new Response(JSON.stringify({ session_id: 's1', status: 'accepted' }), { status: 202 });
    }
    return new Response('', { status: 404 });
  });

  window.location.hash = '#/chat/s1';
  render(<App />);

  const input = await screen.findByPlaceholderText(/Type a message/i);
  fireEvent.change(input, { target: { value: 'hi' } });
  fireEvent.keyDown(input, { key: 'Enter' });

  // Stream a token and completion
  await waitFor(() => expect(FakeEventSource.instances.length).toBeGreaterThan(0));
  const es = FakeEventSource.instances.at(-1)!;
  es.dispatchMessage({ type: 'status', session_id: 's1', data: { state: 'running' } });
  es.dispatchMessage({ type: 'token', session_id: 's1', data: { text: 'Hello' } });
  es.dispatchMessage({ type: 'message_complete', session_id: 's1', data: { assistant_text: 'Hello', message_id: 'm1' } });
  es.dispatchMessage({ type: 'status', session_id: 's1', data: { state: 'idle' } });

  await waitFor(() => expect(screen.getByText('Hello')).toBeInTheDocument());
  fetchMock.mockRestore();
});
```

- [ ] **Step 2: Run**

```bash
cd web && pnpm test -- App
```

Expected: PASS.

- [ ] **Step 3: Commit**

```bash
cd ..
git add web/src/App.test.tsx
git commit -m "test(web): end-to-end chat flow integration"
```

---

### Task 20: Rebuild bundle + copy to webroot

Follow existing convention: `web/dist/` built, then copied into `api/webroot/` for Go `//go:embed`.

- [ ] **Step 1: Build**

```bash
cd web && pnpm build
```

- [ ] **Step 2: Inspect bundle sizes**

```bash
ls -lh dist/assets
```

Record the largest JS and CSS sizes. Expected totals: JS ~ 700-900KB gzipped equivalent (raw ~2MB).

If any asset is >3MB raw, investigate — probably shiki with too many languages. Consider reducing `langs` list in `codeToHtml` options to just the user-expected set.

- [ ] **Step 3: Copy to webroot**

```bash
cd ..
rm -rf api/webroot/assets
mkdir -p api/webroot/assets
cp -r web/dist/index.html api/webroot/
cp -r web/dist/assets/* api/webroot/assets/
```

- [ ] **Step 4: Verify Go embed**

```bash
go build ./api
```

Expected: exit 0. `//go:embed webroot/*` picks up the new assets.

- [ ] **Step 5: Commit**

```bash
git add api/webroot
git commit -m "chore(web): rebuild + copy dist into api/webroot for embed"
```

---

### Task 21: CHANGELOG + smoke doc

**Files:**
- Modify: `CHANGELOG.md`
- Create: `docs/smoke/web-chat-frontend.md`

- [ ] **Step 1: CHANGELOG**

Under `## Unreleased`, add:

```markdown
### Added

- **Web chat workspace.** Opening the web UI now lands in a chat view
  with a session sidebar, message stream, and composer. Configuration
  moved to a Settings mode reached via the top-bar toggle. Markdown
  rendering covers GFM, code highlight (shiki), math (KaTeX), and
  lazy-loaded Mermaid. Assistant output streams token-by-token via SSE;
  a Stop button cancels in flight. 409 / 503 / send failures surface as
  toasts.
```

- [ ] **Step 2: Smoke doc**

Create `docs/smoke/web-chat-frontend.md`:

```markdown
# Web chat frontend smoke

1. Start: `hermind web`. Open the printed URL.
2. TopBar shows "Chat | Settings". Chat is highlighted.
3. Left sidebar has "+ New conversation" and an (initially empty) session list.
4. Click "+ New conversation" → URL hash becomes `#/chat/<uuid>`; sidebar shows the new session.
5. Type "hello" and press Enter. Expect:
   - Optimistic user bubble appears immediately.
   - Tokens stream into an assistant bubble.
   - "Stop" button appears while streaming; hides after completion.
6. Click the "Settings" toggle → URL becomes `#/settings/models`; sidebar shows the seven config groups.
7. Click "Chat" → returns to the chat workspace with the same session.
8. `/clear` in the composer → text box empties.
9. `/settings` → navigates to Settings mode.
10. With no API key (edit config to blank primary provider, reload):
    - Send returns a 503 toast referring to Settings.
11. Cancel while streaming → assistant bubble keeps a "— interrupted" mark; not persisted across reloads.
```

- [ ] **Step 3: Commit**

```bash
git add CHANGELOG.md docs/smoke/web-chat-frontend.md
git commit -m "docs: web chat frontend CHANGELOG + smoke flow"
```

---

## Self-review checklist

- [ ] `pnpm type-check` exits 0.
- [ ] `pnpm test` all green.
- [ ] `pnpm build` succeeds; `dist/index.html` exists.
- [ ] `api/webroot/` contains the latest bundle.
- [ ] `go build ./...` exits 0.
- [ ] `go test ./...` passes (no frontend changes touch Go tests; this just confirms embed still works).
- [ ] Manual: open `hermind web`, send a message, see streamed response.
- [ ] Toggle Chat ↔ Settings preserves state.
- [ ] No console errors in the browser on page load.
