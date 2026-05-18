# AnythingLLM Chat UI Port — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use gpowers:subagent-driven-development (recommended) or gpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Port AnythingLLM's right-side chat interface into Hermind's `web/` frontend, replacing the current `ChatWorkspace` empty-state and input area with a dual-mode layout (centered empty-state vs. bottom-fixed chat).

**Architecture:** Add small presentational components (`ModelPicker`, `MentionButton`, `ToolsButton`, `ToolsMenu`) and refactor three existing components (`PromptInput`, `ChatHistory`, `ChatWorkspace`). `ChatWorkspace` switches between empty mode (centered greeting + `PromptInput`) and chat mode (scrollable history + bottom `PromptInput`). No new dependencies; reuse existing `apiFetch`, CSS Modules, and i18n.

**Tech Stack:** React 18, CSS Modules, Vite, Vitest, i18next, Zod

---

## File Structure

### New Files

| File | Responsibility |
|------|---------------|
| `web/src/components/chat/ModelPicker.tsx` | Top-left pill showing current model name |
| `web/src/components/chat/ModelPicker.module.css` | Pill styling (rounded, transparent bg) |
| `web/src/components/chat/MentionButton.tsx` | "@" circular button; inserts `@` into textarea |
| `web/src/components/chat/MentionButton.module.css` | 28px circular button styling |
| `web/src/components/chat/ToolsButton.tsx` | "工具" pill button; toggles active state |
| `web/src/components/chat/ToolsButton.module.css` | Pill button styling + active tint |
| `web/src/components/chat/ToolsMenu.tsx` | Popover with MCP Tools / Preset Commands / Skills sections |
| `web/src/components/chat/ToolsMenu.module.css` | Popover shadow, sections, item hover |
| `web/src/components/chat/ToolsMenu.test.tsx` | Tests for rendering sections, empty states, selection |
| `web/src/components/chat/ModelPicker.test.tsx` | Tests for displaying model name |

### Modified Files

| File | Change |
|------|--------|
| `web/src/api/schemas.ts` | Add `ToolSchema` + `ToolsResponseSchema` |
| `web/src/locales/en/ui.json` | Add `chat.greeting`, `chat.noTools` |
| `web/src/locales/zh-CN/ui.json` | Add `chat.greeting`, `chat.noTools` |
| `web/src/components/chat/PromptInput.tsx` | Rounded wrapper, bottom button row (`Attach`+`Mention`+`Tools`+`Send`), i18n placeholder, internal `ToolsMenu` |
| `web/src/components/chat/PromptInput.module.css` | Rounded box (`border-radius: 20px`), transparent textarea, button row layout |
| `web/src/components/chat/PromptInput.test.tsx` | Update placeholder assertion; test button presence |
| `web/src/components/chat/ChatHistory.tsx` | Remove `EmptyState` import and rendering; remove `suggestions` + `onSuggestionClick` props |
| `web/src/components/chat/ChatHistory.test.tsx` | Remove EmptyState test; keep message-rendering test |
| `web/src/components/chat/EmptyState.tsx` | Use i18n `t('chat.greeting')`; remove outer flex layout wrapper |
| `web/src/components/chat/EmptyState.module.css` | Keep only `.greeting`, `.suggestions`, `.suggestionBtn` |
| `web/src/components/chat/EmptyState.test.tsx` | Update to query by i18n key or test-id |
| `web/src/components/chat/ChatWorkspace.tsx` | Dual-mode layout (`isEmpty` branch); fetch `/api/status` for `current_model`; render `ModelPicker`; pass `suggestions` to `PromptInput` |
| `web/src/components/chat/ChatWorkspace.module.css` | `.workspace` gains `position: relative`; add `.emptyMode`, `.chatMode`, `.promptWrapper` |

---

## Task 1: Add Missing Frontend Types and Translations

**Files:**
- Modify: `web/src/api/schemas.ts`
- Modify: `web/src/locales/en/ui.json`
- Modify: `web/src/locales/zh-CN/ui.json`

- [ ] **Step 1: Add `ToolSchema` and `ToolsResponseSchema` to `schemas.ts`**

Append after `TTSResponseSchema` (end of file):

```ts
export const ToolSchema = z.object({
  name: z.string(),
  description: z.string().optional(),
});
export type Tool = z.infer<typeof ToolSchema>;

export const ToolsResponseSchema = z.object({
  tools: z.array(ToolSchema),
});
export type ToolsResponse = z.infer<typeof ToolsResponseSchema>;
```

- [ ] **Step 2: Add i18n keys**

In `web/src/locales/en/ui.json`, add under existing `"chat.send"`:
```json
  "chat.greeting": "What can I help you with today?",
  "chat.noTools": "No tools available",
```

In `web/src/locales/zh-CN/ui.json`, add:
```json
  "chat.greeting": "今天我能帮您什么？",
  "chat.noTools": "暂无可用工具",
```

- [ ] **Step 3: Commit**

```bash
git add web/src/api/schemas.ts web/src/locales/en/ui.json web/src/locales/zh-CN/ui.json
git commit -m "feat(chat): add Tool schema and i18n keys for new chat UI"
```

---

## Task 2: Create ModelPicker Component

**Files:**
- Create: `web/src/components/chat/ModelPicker.tsx`
- Create: `web/src/components/chat/ModelPicker.module.css`
- Create: `web/src/components/chat/ModelPicker.test.tsx`

- [ ] **Step 1: Write `ModelPicker.tsx`**

```tsx
import styles from './ModelPicker.module.css';

interface Props {
  modelName: string;
}

export default function ModelPicker({ modelName }: Props) {
  return (
    <div className={styles.picker}>
      <span className={styles.modelName}>{modelName || '—'}</span>
    </div>
  );
}
```

- [ ] **Step 2: Write `ModelPicker.module.css`**

```css
.picker {
  display: inline-flex;
  align-items: center;
  gap: var(--space-2);
  padding: var(--space-2) var(--space-3);
  border-radius: 999px;
  background: transparent;
  border: 1px solid var(--border);
  color: var(--muted);
  font-size: var(--fs-sm);
  cursor: default;
  transition: background var(--t-short);
}

.picker:hover {
  background: var(--hover-tint);
}

.modelName {
  font-weight: 500;
}
```

- [ ] **Step 3: Write `ModelPicker.test.tsx`**

```tsx
import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import ModelPicker from './ModelPicker';

describe('ModelPicker', () => {
  it('renders model name', () => {
    render(<ModelPicker modelName="gpt-4" />);
    expect(screen.getByText('gpt-4')).toBeInTheDocument();
  });

  it('renders em-dash when no model name', () => {
    render(<ModelPicker modelName="" />);
    expect(screen.getByText('—')).toBeInTheDocument();
  });
});
```

- [ ] **Step 4: Run tests**

```bash
cd web && npx vitest run src/components/chat/ModelPicker.test.tsx
```
Expected: 2 passes

- [ ] **Step 5: Commit**

```bash
git add web/src/components/chat/ModelPicker.tsx web/src/components/chat/ModelPicker.module.css web/src/components/chat/ModelPicker.test.tsx
git commit -m "feat(chat): add ModelPicker component"
```

---

## Task 3: Create MentionButton Component

**Files:**
- Create: `web/src/components/chat/MentionButton.tsx`
- Create: `web/src/components/chat/MentionButton.module.css`

- [ ] **Step 1: Write `MentionButton.tsx`**

```tsx
import styles from './MentionButton.module.css';

interface Props {
  onClick: () => void;
  disabled?: boolean;
}

export default function MentionButton({ onClick, disabled }: Props) {
  return (
    <button
      type="button"
      className={styles.btn}
      onClick={onClick}
      disabled={disabled}
      aria-label="Mention"
    >
      @
    </button>
  );
}
```

- [ ] **Step 2: Write `MentionButton.module.css`**

```css
.btn {
  width: 28px;
  height: 28px;
  border-radius: 50%;
  border: none;
  background: transparent;
  color: var(--muted);
  font-size: var(--fs-md);
  font-weight: 600;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  transition: background var(--t-short), color var(--t-short);
}

.btn:hover:not(:disabled) {
  background: var(--hover-tint);
  color: var(--text);
}

.btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}
```

- [ ] **Step 3: Commit**

```bash
git add web/src/components/chat/MentionButton.tsx web/src/components/chat/MentionButton.module.css
git commit -m "feat(chat): add MentionButton component"
```

---

## Task 4: Create ToolsButton Component

**Files:**
- Create: `web/src/components/chat/ToolsButton.tsx`
- Create: `web/src/components/chat/ToolsButton.module.css`

- [ ] **Step 1: Write `ToolsButton.tsx`**

```tsx
import styles from './ToolsButton.module.css';

interface Props {
  onClick: () => void;
  active?: boolean;
  disabled?: boolean;
}

export default function ToolsButton({ onClick, active, disabled }: Props) {
  return (
    <button
      type="button"
      className={`${styles.btn} ${active ? styles.active : ''}`}
      onClick={onClick}
      disabled={disabled}
      aria-label="Tools"
    >
      工具
    </button>
  );
}
```

- [ ] **Step 2: Write `ToolsButton.module.css`**

```css
.btn {
  padding: 2px 10px;
  border-radius: 999px;
  border: none;
  background: transparent;
  color: var(--muted);
  font-size: var(--fs-sm);
  cursor: pointer;
  transition: background var(--t-short), color var(--t-short);
}

.btn:hover:not(:disabled) {
  background: var(--hover-tint);
  color: var(--text);
}

.btn.active {
  background: var(--active-tint);
  color: var(--accent);
}

.btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}
```

- [ ] **Step 3: Commit**

```bash
git add web/src/components/chat/ToolsButton.tsx web/src/components/chat/ToolsButton.module.css
git commit -m "feat(chat): add ToolsButton component"
```

---

## Task 5: Create ToolsMenu Component

**Files:**
- Create: `web/src/components/chat/ToolsMenu.tsx`
- Create: `web/src/components/chat/ToolsMenu.module.css`
- Create: `web/src/components/chat/ToolsMenu.test.tsx`

- [ ] **Step 1: Write `ToolsMenu.tsx`**

```tsx
import { useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { apiFetch } from '../../api/client';
import { ToolsResponseSchema, SkillsResponseSchema } from '../../api/schemas';
import styles from './ToolsMenu.module.css';

interface Props {
  visible: boolean;
  onClose: () => void;
  onSelect: (text: string, sendNow?: boolean) => void;
  suggestions: string[];
}

export default function ToolsMenu({ visible, onClose, onSelect, suggestions }: Props) {
  const { t } = useTranslation('ui');
  const menuRef = useRef<HTMLDivElement>(null);
  const [tools, setTools] = useState<{ name: string; description?: string }[]>([]);
  const [skills, setSkills] = useState<{ name: string; description: string; enabled: boolean }[]>([]);

  useEffect(() => {
    if (!visible) return;
    Promise.all([
      apiFetch('/api/tools', { schema: ToolsResponseSchema }).catch(() => ({ tools: [] })),
      apiFetch('/api/skills', { schema: SkillsResponseSchema }).catch(() => ({ skills: [] })),
    ]).then(([toolsRes, skillsRes]) => {
      setTools(toolsRes.tools);
      setSkills(skillsRes.skills.filter((s) => s.enabled));
    });
  }, [visible]);

  useEffect(() => {
    if (!visible) return;
    const handleClick = (e: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
        onClose();
      }
    };
    document.addEventListener('mousedown', handleClick);
    return () => document.removeEventListener('mousedown', handleClick);
  }, [visible, onClose]);

  if (!visible) return null;

  return (
    <div className={styles.menu} ref={menuRef}>
      <div className={styles.section}>
        <div className={styles.sectionHeader}>MCP</div>
        {tools.length === 0 ? (
          <div className={styles.empty}>{t('chat.noTools')}</div>
        ) : (
          tools.map((tool) => (
            <button
              key={tool.name}
              className={styles.item}
              onClick={() => {
                onSelect(tool.name);
                onClose();
              }}
            >
              <span className={styles.itemName}>{tool.name}</span>
              {tool.description && <span className={styles.itemDesc}>{tool.description}</span>}
            </button>
          ))
        )}
      </div>

      {suggestions.length > 0 && (
        <div className={styles.section}>
          <div className={styles.sectionHeader}>Commands</div>
          {suggestions.map((s) => (
            <button
              key={s}
              className={styles.item}
              onClick={() => {
                onSelect(s, true);
                onClose();
              }}
            >
              <span className={styles.itemName}>{s}</span>
            </button>
          ))}
        </div>
      )}

      {skills.length > 0 && (
        <div className={styles.section}>
          <div className={styles.sectionHeader}>Skills</div>
          {skills.map((s) => (
            <button
              key={s.name}
              className={styles.item}
              onClick={() => {
                onSelect(`/${s.name}`);
                onClose();
              }}
            >
              <span className={styles.itemName}>{s.name}</span>
              {s.description && <span className={styles.itemDesc}>{s.description}</span>}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
```

- [ ] **Step 2: Write `ToolsMenu.module.css`**

```css
.menu {
  position: absolute;
  bottom: calc(100% + 8px);
  left: 0;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: var(--r-lg);
  box-shadow: 0 4px 24px rgba(0, 0, 0, 0.3);
  min-width: 240px;
  max-width: 320px;
  max-height: 400px;
  overflow-y: auto;
  z-index: 100;
  padding: var(--space-3);
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
}

.sectionHeader {
  color: var(--muted);
  font-size: var(--fs-xs);
  text-transform: uppercase;
  letter-spacing: 0.05em;
  margin-bottom: var(--space-2);
  font-weight: 600;
}

.item {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 2px;
  width: 100%;
  padding: var(--space-2) var(--space-3);
  background: none;
  border: none;
  border-radius: var(--r-md);
  color: var(--text);
  font-size: var(--fs-sm);
  text-align: left;
  cursor: pointer;
  transition: background var(--t-fast);
}

.item:hover {
  background: var(--hover-tint);
}

.itemName {
  font-weight: 500;
}

.itemDesc {
  color: var(--muted);
  font-size: var(--fs-xs);
}

.empty {
  color: var(--muted);
  font-size: var(--fs-sm);
  padding: var(--space-2) var(--space-3);
}
```

- [ ] **Step 3: Write `ToolsMenu.test.tsx`**

```tsx
import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import ToolsMenu from './ToolsMenu';

describe('ToolsMenu', () => {
  it('does not render when not visible', () => {
    render(<ToolsMenu visible={false} onClose={vi.fn()} onSelect={vi.fn()} suggestions={[]} />);
    expect(screen.queryByText('MCP')).not.toBeInTheDocument();
  });

  it('renders sections and handles selection', async () => {
    const user = userEvent.setup();
    const onSelect = vi.fn();
    render(
      <ToolsMenu
        visible={true}
        onClose={vi.fn()}
        onSelect={onSelect}
        suggestions={['Hello']}
      />
    );
    expect(screen.getByText('MCP')).toBeInTheDocument();
    expect(screen.getByText('Commands')).toBeInTheDocument();

    await user.click(screen.getByText('Hello'));
    expect(onSelect).toHaveBeenCalledWith('Hello', true);
  });
});
```

- [ ] **Step 4: Run tests**

```bash
cd web && npx vitest run src/components/chat/ToolsMenu.test.tsx
```
Expected: passes (ToolsMenu tests are isolated; API calls are caught by `.catch`)

- [ ] **Step 5: Commit**

```bash
git add web/src/components/chat/ToolsMenu.tsx web/src/components/chat/ToolsMenu.module.css web/src/components/chat/ToolsMenu.test.tsx
git commit -m "feat(chat): add ToolsMenu component"
```

---

## Task 6: Refactor PromptInput

**Files:**
- Modify: `web/src/components/chat/PromptInput.tsx`
- Modify: `web/src/components/chat/PromptInput.module.css`
- Modify: `web/src/components/chat/PromptInput.test.tsx`

- [ ] **Step 1: Rewrite `PromptInput.tsx`**

Replace the entire file with:

```tsx
import { useRef, useCallback, useState } from 'react';
import { useTranslation } from 'react-i18next';
import styles from './PromptInput.module.css';
import AttachmentUploader from './AttachmentUploader';
import MentionButton from './MentionButton';
import ToolsButton from './ToolsButton';
import ToolsMenu from './ToolsMenu';
import type { Attachment } from '../../state/chat';

interface Props {
  text: string;
  onTextChange: (text: string) => void;
  onSubmit: () => void;
  disabled?: boolean;
  attachments?: Attachment[];
  onAttachmentsAdd?: (attachments: Attachment[]) => void;
  onAttachmentRemove?: (id: string) => void;
  suggestions?: string[];
}

export default function PromptInput({
  text,
  onTextChange,
  onSubmit,
  disabled,
  attachments,
  onAttachmentsAdd,
  onAttachmentRemove,
  suggestions = [],
}: Props) {
  const { t } = useTranslation('ui');
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const [toolsOpen, setToolsOpen] = useState(false);

  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
      if (e.key === 'Enter' && !e.shiftKey) {
        e.preventDefault();
        onSubmit();
      }
    },
    [onSubmit],
  );

  const handleChange = useCallback(
    (e: React.ChangeEvent<HTMLTextAreaElement>) => {
      onTextChange(e.target.value);
      const el = e.target;
      el.style.height = 'auto';
      el.style.height = `${Math.min(el.scrollHeight, 200)}px`;
    },
    [onTextChange],
  );

  const handleMention = () => {
    const newText = text + '@';
    onTextChange(newText);
    textareaRef.current?.focus();
  };

  const handleToolsSelect = (selected: string, sendNow?: boolean) => {
    onTextChange(selected);
    if (sendNow) {
      onSubmit();
    } else {
      textareaRef.current?.focus();
    }
  };

  const placeholder = disabled ? t('chat.placeholderNoProvider') : t('chat.placeholder');

  return (
    <div className={styles.inputWrapper}>
      <textarea
        ref={textareaRef}
        className={styles.textarea}
        value={text}
        onChange={handleChange}
        onKeyDown={handleKeyDown}
        placeholder={placeholder}
        rows={1}
        disabled={disabled}
      />

      <div className={styles.buttonRow}>
        <div className={styles.leftButtons}>
          <AttachmentUploader onAttachmentsAdd={onAttachmentsAdd ?? (() => {})} />
          <MentionButton onClick={handleMention} disabled={disabled} />
          <div className={styles.toolsWrapper}>
            <ToolsButton
              onClick={() => setToolsOpen((v) => !v)}
              active={toolsOpen}
              disabled={disabled}
            />
            <ToolsMenu
              visible={toolsOpen}
              onClose={() => setToolsOpen(false)}
              onSelect={handleToolsSelect}
              suggestions={suggestions}
            />
          </div>
        </div>
        <button
          className={styles.sendBtn}
          onClick={onSubmit}
          disabled={disabled || !text.trim()}
          aria-label={t('chat.send')}
        >
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
            <line x1="22" y1="2" x2="11" y2="13" />
            <polygon points="22 2 15 22 11 13 2 9" />
          </svg>
        </button>
      </div>

      {attachments && attachments.length > 0 && (
        <div
          style={{
            display: 'flex',
            flexDirection: 'column',
            gap: '2px',
            maxHeight: '80px',
            overflowY: 'auto',
            marginTop: '8px',
          }}
        >
          {attachments.map((att) => (
            <div
              key={att.id}
              style={{
                display: 'flex',
                alignItems: 'center',
                gap: '4px',
                fontSize: '12px',
                color: 'var(--muted)',
              }}
            >
              <span
                style={{
                  overflow: 'hidden',
                  textOverflow: 'ellipsis',
                  whiteSpace: 'nowrap',
                  maxWidth: '150px',
                }}
              >
                {att.name}
              </span>
              {onAttachmentRemove && (
                <button
                  onClick={() => onAttachmentRemove(att.id)}
                  style={{
                    background: 'none',
                    border: 'none',
                    cursor: 'pointer',
                    color: 'var(--muted)',
                    fontSize: '14px',
                    lineHeight: 1,
                    padding: 0,
                  }}
                  aria-label={`Remove ${att.name}`}
                >
                  ×
                </button>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
```

- [ ] **Step 2: Rewrite `PromptInput.module.css`**

Replace the entire file with:

```css
.inputWrapper {
  background: var(--surface-2);
  border: 1px solid var(--border);
  border-radius: 20px;
  padding: var(--space-3) var(--space-4);
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
  transition: border-color var(--t-fast);
}

.inputWrapper:focus-within {
  border-color: var(--accent);
}

.textarea {
  background: transparent;
  border: none;
  outline: none;
  color: var(--text);
  font-size: var(--fs-base);
  resize: none;
  min-height: 24px;
  max-height: 200px;
  width: 100%;
  padding: 0;
  line-height: 1.5;
}

.textarea::placeholder {
  color: var(--muted);
}

.textarea:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.buttonRow {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-2);
}

.leftButtons {
  display: flex;
  align-items: center;
  gap: var(--space-1);
}

.toolsWrapper {
  position: relative;
}

.sendBtn {
  width: 32px;
  height: 32px;
  border-radius: 50%;
  background: var(--text);
  color: var(--accent-fg);
  border: none;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  transition: opacity var(--t-fast);
  flex-shrink: 0;
}

.sendBtn:hover:not(:disabled) {
  opacity: 0.9;
}

.sendBtn:disabled {
  background: var(--muted);
  color: var(--bg);
  opacity: 0.6;
  cursor: not-allowed;
}
```

- [ ] **Step 3: Update `PromptInput.test.tsx`**

Replace the entire file with:

```tsx
import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { useState } from 'react';
import userEvent from '@testing-library/user-event';
import PromptInput from './PromptInput';

function StatefulPromptInput({ onSubmit, onTextChangeSpy }: { onSubmit?: () => void; onTextChangeSpy?: (text: string) => void }) {
  const [text, setText] = useState('');
  return (
    <PromptInput
      text={text}
      onTextChange={(t) => {
        setText(t);
        onTextChangeSpy?.(t);
      }}
      onSubmit={onSubmit ?? vi.fn()}
    />
  );
}

describe('PromptInput', () => {
  it('renders textarea and send button', () => {
    render(<StatefulPromptInput />);
    expect(screen.getByPlaceholderText(/type a message/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/send/i)).toBeInTheDocument();
  });

  it('calls onTextChange when typing', async () => {
    const user = userEvent.setup();
    const onTextChange = vi.fn();
    render(<StatefulPromptInput onTextChangeSpy={onTextChange} />);
    await user.type(screen.getByPlaceholderText(/type a message/i), 'hi');
    expect(onTextChange).toHaveBeenCalledWith('hi');
  });

  it('calls onSubmit when send button clicked', async () => {
    const user = userEvent.setup();
    const onSubmit = vi.fn();
    render(<PromptInput text="hello" onTextChange={vi.fn()} onSubmit={onSubmit} />);
    await user.click(screen.getByLabelText(/send/i));
    expect(onSubmit).toHaveBeenCalled();
  });

  it('calls onSubmit on Enter without shift', async () => {
    const user = userEvent.setup();
    const onSubmit = vi.fn();
    render(<PromptInput text="hello" onTextChange={vi.fn()} onSubmit={onSubmit} />);
    await user.type(screen.getByPlaceholderText(/type a message/i), '{Enter}');
    expect(onSubmit).toHaveBeenCalled();
  });

  it('renders attach, mention, and tools buttons', () => {
    render(<StatefulPromptInput />);
    expect(screen.getByLabelText('Attach file')).toBeInTheDocument();
    expect(screen.getByLabelText('Mention')).toBeInTheDocument();
    expect(screen.getByLabelText('Tools')).toBeInTheDocument();
  });
});
```

- [ ] **Step 4: Run tests**

```bash
cd web && npx vitest run src/components/chat/PromptInput.test.tsx
```
Expected: 5 passes

- [ ] **Step 5: Commit**

```bash
git add web/src/components/chat/PromptInput.tsx web/src/components/chat/PromptInput.module.css web/src/components/chat/PromptInput.test.tsx
git commit -m "feat(chat): refactor PromptInput to rounded box with tool buttons"
```

---

## Task 7: Remove EmptyState from ChatHistory

**Files:**
- Modify: `web/src/components/chat/ChatHistory.tsx`
- Modify: `web/src/components/chat/ChatHistory.test.tsx`

- [ ] **Step 1: Rewrite `ChatHistory.tsx`**

```tsx
import { useRef, useEffect } from 'react';
import styles from './ChatHistory.module.css';
import { useScrollToBottom } from '../../hooks/useScrollToBottom';
import HistoricalMessage from './HistoricalMessage';
import PromptReply from './PromptReply';
import ScrollToBottomButton from './ScrollToBottomButton';
import type { ChatMessage, ToolCall } from '../../state/chat';

interface Props {
  messages: ChatMessage[];
  streamingDraft: string;
  streamingToolCalls: ToolCall[];
  onEdit?: (id: string, content: string) => void;
  onDelete?: (id: string) => void;
  onRegenerate?: (id: string) => void;
}

export default function ChatHistory({
  messages,
  streamingDraft,
  streamingToolCalls,
  onEdit,
  onDelete,
  onRegenerate,
}: Props) {
  const containerRef = useRef<HTMLDivElement>(null);
  const { isAtBottom, scrollToBottom } = useScrollToBottom(containerRef);

  useEffect(() => {
    if (isAtBottom) {
      scrollToBottom('auto');
    }
  }, [messages, streamingDraft, streamingToolCalls, isAtBottom, scrollToBottom]);

  return (
    <div className={styles.history} ref={containerRef}>
      <div className={styles.messages}>
        {messages.map((msg) => (
          <HistoricalMessage
            key={msg.id}
            message={msg}
            onEdit={onEdit}
            onDelete={onDelete}
            onRegenerate={onRegenerate}
          />
        ))}
        {(streamingDraft || streamingToolCalls.length > 0) && (
          <PromptReply draft={streamingDraft} toolCalls={streamingToolCalls} />
        )}
      </div>
      {!isAtBottom && <ScrollToBottomButton onClick={() => scrollToBottom('smooth')} />}
    </div>
  );
}
```

- [ ] **Step 2: Update `ChatHistory.test.tsx`**

```tsx
import { describe, it, expect, vi, beforeAll, afterAll } from 'vitest';
import { render, screen } from '@testing-library/react';
import ChatHistory from './ChatHistory';
import type { ChatMessage } from '../../state/chat';

describe('ChatHistory', () => {
  const originalScrollTo = HTMLElement.prototype.scrollTo;

  beforeAll(() => {
    HTMLElement.prototype.scrollTo = vi.fn();
  });

  afterAll(() => {
    HTMLElement.prototype.scrollTo = originalScrollTo;
  });

  it('renders messages when provided', () => {
    const messages: ChatMessage[] = [
      { id: '1', role: 'user', content: 'hello', timestamp: 0 },
      { id: '2', role: 'assistant', content: 'world', timestamp: 1 },
    ];
    render(
      <ChatHistory
        messages={messages}
        streamingDraft=""
        streamingToolCalls={[]}
      />
    );
    expect(screen.getByText('hello')).toBeInTheDocument();
    expect(screen.getByText('world')).toBeInTheDocument();
  });

  it('renders streaming draft', () => {
    render(
      <ChatHistory
        messages={[]}
        streamingDraft="thinking..."
        streamingToolCalls={[]}
      />
    );
    expect(screen.getByText('thinking...')).toBeInTheDocument();
  });
});
```

- [ ] **Step 3: Run tests**

```bash
cd web && npx vitest run src/components/chat/ChatHistory.test.tsx
```
Expected: 2 passes

- [ ] **Step 4: Commit**

```bash
git add web/src/components/chat/ChatHistory.tsx web/src/components/chat/ChatHistory.test.tsx
git commit -m "refactor(chat): remove EmptyState from ChatHistory"
```

---

## Task 8: Refactor EmptyState (Simplify + i18n)

**Files:**
- Modify: `web/src/components/chat/EmptyState.tsx`
- Modify: `web/src/components/chat/EmptyState.module.css`
- Modify: `web/src/components/chat/EmptyState.test.tsx`

- [ ] **Step 1: Rewrite `EmptyState.tsx`**

```tsx
import { useTranslation } from 'react-i18next';
import styles from './EmptyState.module.css';

interface Props {
  suggestions: string[];
  onSuggestionClick: (text: string) => void;
}

export default function EmptyState({ suggestions, onSuggestionClick }: Props) {
  const { t } = useTranslation('ui');
  return (
    <>
      <h1 className={styles.greeting}>{t('chat.greeting')}</h1>
      {suggestions.length > 0 && (
        <div className={styles.suggestions}>
          {suggestions.map((text) => (
            <button
              key={text}
              className={styles.suggestionBtn}
              onClick={() => onSuggestionClick(text)}
            >
              {text}
            </button>
          ))}
        </div>
      )}
    </>
  );
}
```

- [ ] **Step 2: Rewrite `EmptyState.module.css`**

```css
.greeting {
  font-size: var(--fs-2xl);
  font-weight: 600;
  color: var(--text);
  text-align: center;
  margin: 0;
}

.suggestions {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  gap: var(--space-3);
  max-width: 600px;
}

.suggestionBtn {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: var(--r-lg);
  padding: var(--space-3) var(--space-4);
  color: var(--text);
  font-size: var(--fs-sm);
  cursor: pointer;
  transition: background var(--t-fast), border-color var(--t-fast);
}

.suggestionBtn:hover {
  background: var(--surface-2);
  border-color: var(--accent);
}
```

- [ ] **Step 3: Update `EmptyState.test.tsx`**

```tsx
import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import EmptyState from './EmptyState';

// Mock i18next
vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => (key === 'chat.greeting' ? 'Greeting Text' : key) }),
}));

describe('EmptyState', () => {
  it('renders greeting and suggestions', () => {
    render(<EmptyState suggestions={['What can you do?', 'Help me code']} onSuggestionClick={vi.fn()} />);
    expect(screen.getByText('Greeting Text')).toBeInTheDocument();
    expect(screen.getByText('What can you do?')).toBeInTheDocument();
    expect(screen.getByText('Help me code')).toBeInTheDocument();
  });

  it('calls onSuggestionClick when a suggestion is clicked', async () => {
    const user = userEvent.setup();
    const onClick = vi.fn();
    render(<EmptyState suggestions={['Test suggestion']} onSuggestionClick={onClick} />);
    await user.click(screen.getByText('Test suggestion'));
    expect(onClick).toHaveBeenCalledWith('Test suggestion');
  });
});
```

- [ ] **Step 4: Run tests**

```bash
cd web && npx vitest run src/components/chat/EmptyState.test.tsx
```
Expected: 2 passes

- [ ] **Step 5: Commit**

```bash
git add web/src/components/chat/EmptyState.tsx web/src/components/chat/EmptyState.module.css web/src/components/chat/EmptyState.test.tsx
git commit -m "refactor(chat): simplify EmptyState, use i18n greeting"
```

---

## Task 9: Refactor ChatWorkspace for Dual-Mode Layout

**Files:**
- Modify: `web/src/components/chat/ChatWorkspace.tsx`
- Modify: `web/src/components/chat/ChatWorkspace.module.css`

- [ ] **Step 1: Rewrite `ChatWorkspace.tsx`**

Replace imports and component body. Keep all existing handler logic (`handleSend`, `handleStop`, `handleEdit`, `handleDelete`, `handleRegenerate`, `handleSuggestionClick`). Only the `return` block and one new `useEffect` change.

Full file:

```tsx
import { useEffect, useReducer, useState, useTransition } from 'react';
import { useTranslation } from 'react-i18next';
import ConversationHeader from './ConversationHeader';
import ChatHistory from './ChatHistory';
import PromptInput from './PromptInput';
import Toast from './Toast';
import ModelPicker from './ModelPicker';
import EmptyState from './EmptyState';
import styles from './ChatWorkspace.module.css';
import { useChatStream } from '../../hooks/useChatStream';
import { chatReducer, initialChatState } from '../../state/chat';
import { apiFetch, apiPut, apiDelete, ApiError } from '../../api/client';
import {
  ConversationHistoryResponseSchema,
  SuggestionsResponseSchema,
  MetaResponseSchema,
} from '../../api/schemas';

type Props = {
  instanceRoot: string;
  providerConfigured?: boolean;
};

export default function ChatWorkspace({
  instanceRoot,
  providerConfigured = true,
}: Props) {
  const { t } = useTranslation('ui');
  const [state, dispatch] = useReducer(chatReducer, initialChatState);
  const [toast, setToast] = useState<string | null>(null);
  const [currentModel, setCurrentModel] = useState('');
  const [, startTransition] = useTransition();

  useChatStream(dispatch);

  useEffect(() => {
    const ctrl = new AbortController();
    apiFetch('/api/conversation', {
      schema: ConversationHistoryResponseSchema,
      signal: ctrl.signal,
    })
      .then((r) => {
        const messages = r.messages.map((m) => ({
          id: String(m.id),
          chatId: m.id,
          role: m.role,
          content: m.content,
          timestamp: m.timestamp,
        }));
        startTransition(() => {
          dispatch({ type: 'chat/history/loaded', messages });
        });
      })
      .catch(() => {
        /* empty history is fine */
      });
    return () => ctrl.abort();
  }, [startTransition]);

  useEffect(() => {
    const ctrl = new AbortController();
    apiFetch('/api/suggestions', {
      schema: SuggestionsResponseSchema,
      signal: ctrl.signal,
    })
      .then((r) => {
        startTransition(() => {
          dispatch({ type: 'chat/suggestions/loaded', suggestions: r.suggestions });
        });
      })
      .catch(() => {
        /* missing suggestions is fine */
      });
    return () => ctrl.abort();
  }, [startTransition]);

  useEffect(() => {
    const ctrl = new AbortController();
    apiFetch('/api/status', { schema: MetaResponseSchema, signal: ctrl.signal })
      .then((r) => setCurrentModel(r.current_model))
      .catch(() => {});
    return () => ctrl.abort();
  }, []);

  async function handleSend(overrideText?: string) {
    const text = (overrideText ?? state.composer.text).trim();
    if (!text) return;
    dispatch({ type: 'chat/composer/setText', text: '' });
    dispatch({ type: 'chat/stream/start', userText: text });
    try {
      await apiFetch('/api/conversation/messages', {
        method: 'POST',
        body: { user_message: text },
      });
    } catch (err) {
      dispatch({ type: 'chat/stream/rollbackUserMessage' });
      if (err instanceof ApiError) {
        if (err.status === 409) setToast(t('chat.errorBusy'));
        else if (err.status === 503) setToast(t('chat.errorNoProvider'));
        else setToast(t('chat.errorSendFailed', { msg: err.message }));
      } else {
        setToast(t('chat.errorSendFailed', { msg: err instanceof Error ? err.message : '' }));
      }
    }
  }

  async function handleStop() {
    try {
      await apiFetch('/api/conversation/cancel', { method: 'POST' });
    } catch (err) {
      console.warn('cancel failed', err);
    }
  }

  async function handleEdit(id: string, content: string) {
    const msg = state.messages.find((m) => m.id === id);
    if (!msg || msg.chatId === undefined) return;
    dispatch({ type: 'chat/message/edit', id, content });
    try {
      await apiPut(`/api/conversation/messages/${msg.chatId}`, { content });
    } catch (err) {
      setToast(t('chat.errorEditFailed', { msg: err instanceof Error ? err.message : '' }));
    }
  }

  async function handleDelete(id: string) {
    const msg = state.messages.find((m) => m.id === id);
    if (!msg || msg.chatId === undefined) return;
    dispatch({ type: 'chat/message/delete', id });
    try {
      await apiDelete(`/api/conversation/messages/${msg.chatId}`);
    } catch (err) {
      setToast(t('chat.errorDeleteFailed', { msg: err instanceof Error ? err.message : '' }));
    }
  }

  async function handleRegenerate(id: string) {
    const targetIndex = state.messages.findIndex((m) => m.id === id);
    if (targetIndex < 0) return;
    const msg = state.messages[targetIndex];
    if (!msg || msg.chatId === undefined) return;

    let precedingUserContent = '';
    for (let i = targetIndex - 1; i >= 0; i--) {
      if (state.messages[i].role === 'user') {
        precedingUserContent = state.messages[i].content;
        break;
      }
    }

    dispatch({ type: 'chat/message/regenerate', id });

    try {
      await apiDelete(`/api/conversation/messages/${msg.chatId}`);
    } catch (err) {
      console.warn('regenerate delete failed', err);
    }

    try {
      await apiFetch('/api/conversation/messages', {
        method: 'POST',
        body: { user_message: precedingUserContent },
      });
    } catch (err) {
      dispatch({ type: 'chat/stream/rollbackUserMessage' });
      if (err instanceof ApiError) {
        if (err.status === 409) setToast(t('chat.errorBusy'));
        else if (err.status === 503) setToast(t('chat.errorNoProvider'));
        else setToast(t('chat.errorSendFailed', { msg: err.message }));
      } else {
        setToast(t('chat.errorSendFailed', { msg: err instanceof Error ? err.message : '' }));
      }
    }
  }

  const handleSuggestionClick = (text: string) => {
    dispatch({ type: 'chat/composer/setText', text });
    handleSend(text);
  };

  const isEmpty =
    state.messages.length === 0 &&
    !state.streaming.assistantDraft &&
    state.streaming.toolCalls.length === 0;

  return (
    <div className={`${styles.workspace} ${isEmpty ? styles.emptyMode : styles.chatMode}`}>
      <div className={styles.modelPicker}>
        <ModelPicker modelName={currentModel} />
      </div>

      {isEmpty ? (
        <>
          <EmptyState
            suggestions={state.suggestions}
            onSuggestionClick={handleSuggestionClick}
          />
          <div className={styles.promptWrapper}>
            <PromptInput
              text={state.composer.text}
              onTextChange={(txt) => dispatch({ type: 'chat/composer/setText', text: txt })}
              onSubmit={handleSend}
              disabled={!providerConfigured || state.streaming.status === 'running'}
              suggestions={state.suggestions}
            />
          </div>
        </>
      ) : (
        <>
          <ConversationHeader
            instanceRoot={instanceRoot}
            onStop={handleStop}
            streaming={state.streaming.status === 'running'}
          />
          <ChatHistory
            messages={state.messages}
            streamingDraft={state.streaming.assistantDraft}
            streamingToolCalls={state.streaming.toolCalls}
            onEdit={handleEdit}
            onDelete={handleDelete}
            onRegenerate={handleRegenerate}
          />
          {state.streaming.status === 'error' && state.streaming.error && (
            <div role="alert" className={styles.errorBanner}>
              {state.streaming.error}
            </div>
          )}
          <PromptInput
            text={state.composer.text}
            onTextChange={(txt) => dispatch({ type: 'chat/composer/setText', text: txt })}
            onSubmit={handleSend}
            disabled={!providerConfigured || state.streaming.status === 'running'}
            suggestions={state.suggestions}
          />
        </>
      )}
      {toast && <Toast message={toast} onDismiss={() => setToast(null)} />}
    </div>
  );
}
```

- [ ] **Step 2: Rewrite `ChatWorkspace.module.css`**

```css
.workspace {
  position: relative;
  height: 100%;
  min-height: 0;
  overflow: hidden;
  background: var(--bg);
}

.chatMode {
  display: grid;
  grid-template-rows: auto 1fr auto;
}

.emptyMode {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: var(--space-6);
  padding: var(--space-8);
}

.modelPicker {
  position: absolute;
  top: var(--space-3);
  left: var(--space-4);
  z-index: 10;
}

.promptWrapper {
  width: 100%;
  max-width: 640px;
}

.errorBanner {
  padding: var(--space-2) var(--space-4);
  background: var(--surface);
  border-top: 1px solid var(--error);
  color: var(--error);
  font-family: var(--font-mono);
  font-size: var(--fs-sm);
}
```

- [ ] **Step 3: Run all chat component tests**

```bash
cd web && npx vitest run src/components/chat/
```
Expected: All passes (PromptInput, ChatHistory, EmptyState, ModelPicker, ToolsMenu)

- [ ] **Step 4: Commit**

```bash
git add web/src/components/chat/ChatWorkspace.tsx web/src/components/chat/ChatWorkspace.module.css
git commit -m "feat(chat): dual-mode layout with ModelPicker and empty state"
```

---

## Task 10: Full Test Run and Integration Check

**Files:**
- Run: all frontend tests

- [ ] **Step 1: Run full frontend test suite**

```bash
cd web && npx vitest run
```
Expected: All tests pass. If any fail, fix the component or test.

- [ ] **Step 2: Type-check**

```bash
cd web && npx tsc --noEmit
```
Expected: No type errors.

- [ ] **Step 3: Build check**

```bash
cd web && npm run build
```
Expected: Build succeeds with no errors.

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "feat(chat): complete AnythingLLM chat UI port"
```

---

## Self-Review

### 1. Spec Coverage

| Design Doc Section | Implementing Task |
|---|---|
| Dual-mode layout (empty vs chat) | Task 9 |
| ModelPicker top-left pill | Task 2 + Task 9 |
| Rounded PromptInput with button row | Task 6 |
| Attach / Mention / Tools buttons | Tasks 3, 4, 6 |
| ToolsMenu with 3 sections | Task 5 |
| Remove EmptyState from ChatHistory | Task 7 |
| i18n greeting | Task 8 |
| Suggestion chips in empty mode | Task 9 (via EmptyState) |
| CSS Modules styling | All component tasks |

**Gaps:** None. Every design doc requirement has a corresponding task.

### 2. Placeholder Scan

- No "TBD", "TODO", "implement later", "fill in details" found.
- Every step contains actual code or exact commands.
- No vague instructions like "add appropriate error handling".

### 3. Type Consistency

- `PromptInput` props: `suggestions?: string[]` — consistent across Task 6 and Task 9.
- `ToolsMenu` props: `onSelect(text: string, sendNow?: boolean)` — consistent across Task 5 and Task 6.
- `MetaResponseSchema` has `current_model: string` — used in Task 9.
- `ToolsResponseSchema` / `SkillsResponseSchema` — used in Task 5.

### 4. Integration Notes

- `ChatWorkspace` now imports `ModelPicker`, `EmptyState`, `MetaResponseSchema` — these are all created in earlier tasks.
- `PromptInput` now imports `MentionButton`, `ToolsButton`, `ToolsMenu` — created in earlier tasks.
- `AttachmentUploader` is reused unchanged; its button styling may differ slightly from the new circular buttons, which is acceptable.
- The `ComposerBar` component exists in the codebase but is not used by `ChatWorkspace`; it is left untouched.

---

## Execution Handoff

**Plan complete and saved to `.gpowers/plans/2026-05-18-anythingllm-chat-ui-port.md`.**

**Two execution options:**

**1. Subagent-Driven (recommended)** — I dispatch a fresh subagent per task, review between tasks, fast iteration. Best for reliability and catching issues early.

**2. Inline Execution** — Execute tasks sequentially in this session, batching where safe, with checkpoints for review.

**Which approach would you like?**
