# Editor + Fields (Stage 4b) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Finish the editor UX. `FieldSecret` gets a real SecretInput with Show/Hide backed by `/api/platforms/{key}/reveal`. The Editor gains a Test button that calls `/api/platforms/{key}/test` and renders ok/err inline. Sidebar's `+ New instance` button opens a modal that picks a type, validates a key, and creates an empty options blob.

**Architecture:** Three new components (`SecretInput`, `TestConnection`, `NewInstanceDialog`) plus the wiring in `App.tsx` / `Sidebar.tsx` / `Editor.tsx` / `FieldList.tsx`. A new reducer action `instance/create` handles the new-instance path. `SecretInput` owns a small local state for "revealed/masked" toggle and its cached plaintext; reveal calls are single-shot per Show click. `TestConnection` owns an inline busy/ok/err cell backed by a single `useState`. `NewInstanceDialog` renders as a full-viewport overlay with a native `<form>` and traps focus via `HTMLDialogElement`.

**Tech Stack:** React 18, TypeScript 5 (existing). Uses native `<dialog>` element for the modal — no focus-trap library needed. No new dependencies.

**Source of truth:** `docs/superpowers/specs/2026-04-19-web-im-config-design.md` §8 (Frontend architecture), §6 (Secret handling), §5 (REST endpoints: /reveal, /test).

**Scope cut (Stage 5):** No vitest / CI integration / README yet — all Stage 5.

---

## File Structure

**Create:**

- `web/src/components/fields/SecretInput.tsx` — masked input + Show/Hide button.
- `web/src/components/fields/SecretInput.module.css`
- `web/src/components/TestConnection.tsx` — button + inline result cell.
- `web/src/components/TestConnection.module.css`
- `web/src/components/NewInstanceDialog.tsx` — modal form (type picker + key input).
- `web/src/components/NewInstanceDialog.module.css`

**Modify:**

- `web/src/api/schemas.ts` — add `PlatformTestResponseSchema` + `RevealResponseSchema` (the client already reads matching shapes today but without schemas).
- `web/src/state.ts` — add `instance/create` action + `instance/deselect` (used when instance-create lands on a new key and we want to select it).
- `web/src/components/FieldList.tsx` — route `FieldSecret` to `SecretInput` (replace the Stage-4a fall-through).
- `web/src/components/Editor.tsx` — render TestConnection between FieldList and the danger zone. The Test button is disabled while the instance is dirty (ask user to save first).
- `web/src/components/Sidebar.tsx` — `onNewInstance` still a simple callback, but App.tsx now passes a real handler that opens the dialog.
- `web/src/App.tsx` — modal open/close state; dispatch `instance/create` on submit; pass `dialogOpen` / `onOpenDialog` / `onCloseDialog` props.

**Untouched:**

- Any backend (`api/`, `cli/`, `gateway/`).
- The four existing field controls (TextInput / NumberInput / BoolToggle / EnumSelect).
- `TopBar`, `Footer`, CSS theme tokens.

---

## Task 1: Schema additions

**Files:**
- Modify: `web/src/api/schemas.ts`

Zod schemas for the two responses we'll start parsing in later tasks. Both shapes are already served by Stage 2 endpoints (`api/dto.go`): `{ok: bool, error?: string}` for test, `{value: string}` for reveal.

- [ ] **Step 1: Append to `web/src/api/schemas.ts`**

Open the current file and append these two schema declarations **after** `ApplyResultSchema` (which is the current last export):

```ts
export const PlatformTestResponseSchema = z.object({
  ok: z.boolean(),
  error: z.string().optional(),
});
export type PlatformTestResponse = z.infer<typeof PlatformTestResponseSchema>;

export const RevealResponseSchema = z.object({
  value: z.string(),
});
export type RevealResponse = z.infer<typeof RevealResponseSchema>;
```

- [ ] **Step 2: type-check**

```bash
(cd /Users/ranwei/.config/superpowers/worktrees/hermind/feat-editor-fields-4b/web && pnpm type-check)
```

Expected: PASS.

- [ ] **Step 3: Build**

```bash
(cd /Users/ranwei/.config/superpowers/worktrees/hermind/feat-editor-fields-4b && make web)
```

Expected: clean; new hashes on `api/webroot/assets/*`.

- [ ] **Step 4: Commit**

Stage `web/src/api/schemas.ts` + rebuilt `api/webroot/`:

```bash
(cd /Users/ranwei/.config/superpowers/worktrees/hermind/feat-editor-fields-4b && git add web/src/api/schemas.ts api/webroot/)
```

Commit with exactly:

```
feat(web/api): zod schemas for test + reveal responses

PlatformTestResponseSchema and RevealResponseSchema mirror the
backend DTOs so the upcoming SecretInput + TestConnection components
can parse responses through apiFetch's generic schema slot.
```

---

## Task 2: SecretInput component + FieldList wiring

**Files:**
- Create: `web/src/components/fields/SecretInput.tsx`
- Create: `web/src/components/fields/SecretInput.module.css`
- Modify: `web/src/components/FieldList.tsx`

SecretInput renders a `<input type="password">` plus a Show/Hide button. Click "Show" fires `POST /api/platforms/{key}/reveal`; success flips the input to `type="text"` and populates the value from the response; Hide re-masks but keeps whatever the user typed. If the user has edited the field (value differs from the empty baseline), Show stays disabled — revealing a half-typed value is confusing and the backend would overwrite anyway on save.

### SecretInput props shape

```ts
interface SecretInputProps {
  field: SchemaField;
  value: string;
  instanceKey: string;      // needed for /reveal URL
  dirty: boolean;           // has the user typed into this field?
  onChange: (value: string) => void;
}
```

The `dirty` flag is driven by comparing the current options value to the originalConfig — Editor will pass it down.

- [ ] **Step 1: Create `web/src/components/fields/SecretInput.module.css`**

```css
.wrap {
  display: block;
  margin: 16px 0;
}
.label {
  display: block;
  font-size: 14px;
  font-weight: 500;
  color: var(--text);
  margin-bottom: 6px;
}
.required {
  color: var(--error);
  margin-left: 4px;
}
.inputRow {
  position: relative;
}
.input {
  width: 100%;
  height: 36px;
  padding: 0 60px 0 12px;
  font-size: 14px;
  font-family: var(--font-mono);
  color: var(--text);
  background: var(--bg);
  border: 1px solid var(--border);
  border-radius: 6px;
  transition: border-color 120ms ease, box-shadow 120ms ease;
}
.input:focus {
  outline: none;
  border-color: var(--accent);
  box-shadow: 0 0 0 2px var(--focus);
}
.revealBtn {
  position: absolute;
  right: 8px;
  top: 50%;
  transform: translateY(-50%);
  appearance: none;
  background: transparent;
  border: 0;
  padding: 0 4px;
  font: inherit;
  font-size: 12px;
  color: var(--muted);
  cursor: pointer;
}
.revealBtn:hover:not(:disabled) { text-decoration: underline; }
.revealBtn:disabled { cursor: not-allowed; opacity: 0.5; }
.help {
  display: block;
  font-size: 12px;
  color: var(--muted);
  margin-top: 6px;
}
.error {
  display: block;
  font-size: 12px;
  color: var(--error);
  margin-top: 6px;
}
```

- [ ] **Step 2: Create `web/src/components/fields/SecretInput.tsx`**

```tsx
import { useState } from 'react';
import styles from './SecretInput.module.css';
import type { SchemaField } from '../../api/schemas';
import { RevealResponseSchema } from '../../api/schemas';
import { apiFetch, ApiError } from '../../api/client';

export interface SecretInputProps {
  field: SchemaField;
  value: string;
  instanceKey: string;
  dirty: boolean;
  onChange: (value: string) => void;
}

export default function SecretInput({
  field,
  value,
  instanceKey,
  dirty,
  onChange,
}: SecretInputProps) {
  const [revealed, setRevealed] = useState(false);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  async function onToggle() {
    if (revealed) {
      setRevealed(false);
      return;
    }
    // Fetch the real value and switch to text input.
    setBusy(true);
    setErr(null);
    try {
      const res = await apiFetch(
        `/api/platforms/${encodeURIComponent(instanceKey)}/reveal`,
        {
          method: 'POST',
          body: { field: field.name },
          schema: RevealResponseSchema,
        },
      );
      onChange(res.value);
      setRevealed(true);
    } catch (e) {
      setErr(toMsg(e));
    } finally {
      setBusy(false);
    }
  }

  // Show button disabled when: the user has already typed something
  // (revealing would overwrite their draft) OR the instance is unsaved
  // brand-new (nothing on disk to reveal yet — handled by the dirty flag
  // propagated from the Editor when the instance is dirty at all).
  const showDisabled = busy || dirty;

  return (
    <label className={styles.wrap}>
      <span className={styles.label}>
        {field.label}
        {field.required && <span className={styles.required}>*</span>}
      </span>
      <span className={styles.inputRow}>
        <input
          type={revealed ? 'text' : 'password'}
          className={styles.input}
          value={value}
          placeholder="•••"
          onChange={e => {
            // Typing into the field always hides any revealed state —
            // the user is editing, not viewing.
            setRevealed(false);
            onChange(e.currentTarget.value);
          }}
        />
        <button
          type="button"
          className={styles.revealBtn}
          onClick={onToggle}
          disabled={showDisabled}
          title={dirty ? 'Save changes before revealing the stored value' : undefined}
        >
          {busy ? '…' : revealed ? 'Hide' : 'Show'}
        </button>
      </span>
      {err && <span className={styles.error}>{err}</span>}
      {field.help && !err && <span className={styles.help}>{field.help}</span>}
    </label>
  );
}

function toMsg(e: unknown): string {
  if (e instanceof ApiError) {
    if (typeof e.body === 'object' && e.body !== null && 'error' in e.body) {
      const m = (e.body as { error?: unknown }).error;
      if (typeof m === 'string') return m;
    }
    return `HTTP ${e.status}`;
  }
  return e instanceof Error ? e.message : String(e);
}
```

- [ ] **Step 3: Modify `web/src/components/FieldList.tsx`**

Change the `secret` case to route to SecretInput, and make FieldList accept the two new props needed to feed SecretInput.

Overwrite `web/src/components/FieldList.tsx` with:

```tsx
import type { SchemaDescriptor } from '../api/schemas';
import TextInput from './fields/TextInput';
import NumberInput from './fields/NumberInput';
import BoolToggle from './fields/BoolToggle';
import EnumSelect from './fields/EnumSelect';
import SecretInput from './fields/SecretInput';

export interface FieldListProps {
  descriptor: SchemaDescriptor;
  options: Record<string, string>;
  originalOptions: Record<string, string>;
  instanceKey: string;
  onChange: (field: string, value: string) => void;
}

export default function FieldList({
  descriptor,
  options,
  originalOptions,
  instanceKey,
  onChange,
}: FieldListProps) {
  return (
    <div>
      {descriptor.fields.map(field => {
        const value = options[field.name] ?? '';
        const originalValue = originalOptions[field.name] ?? '';
        const onFieldChange = (v: string) => onChange(field.name, v);
        switch (field.kind) {
          case 'int':
            return <NumberInput key={field.name} field={field} value={value} onChange={onFieldChange} />;
          case 'bool':
            return <BoolToggle key={field.name} field={field} value={value} onChange={onFieldChange} />;
          case 'enum':
            return <EnumSelect key={field.name} field={field} value={value} onChange={onFieldChange} />;
          case 'secret':
            return (
              <SecretInput
                key={field.name}
                field={field}
                value={value}
                instanceKey={instanceKey}
                dirty={value !== originalValue}
                onChange={onFieldChange}
              />
            );
          case 'string':
          default:
            return <TextInput key={field.name} field={field} value={value} onChange={onFieldChange} />;
        }
      })}
    </div>
  );
}
```

Callers of `FieldList` must now pass `originalOptions` + `instanceKey`. The Editor update lands in Task 4.

- [ ] **Step 4: `pnpm type-check` — expect intermediate errors**

```bash
(cd /Users/ranwei/.config/superpowers/worktrees/hermind/feat-editor-fields-4b/web && pnpm type-check 2>&1 | head -15)
```

Expected: errors in `Editor.tsx` about FieldList not accepting the old prop shape. This is intentional — Task 4 fixes Editor. Do NOT patch Editor here.

- [ ] **Step 5: Commit**

Stage the three files:

```bash
(cd /Users/ranwei/.config/superpowers/worktrees/hermind/feat-editor-fields-4b && git add web/src/components/fields/SecretInput.tsx web/src/components/fields/SecretInput.module.css web/src/components/FieldList.tsx)
```

Verify staging — exactly 3 files:

```bash
(cd /Users/ranwei/.config/superpowers/worktrees/hermind/feat-editor-fields-4b && git status --short)
```

Commit with exactly:

```
feat(web/fields): SecretInput with Show/Hide backed by /reveal

Masked password input with a Show button that POSTs
/api/platforms/{key}/reveal to fetch the stored plaintext, flips
the input to type=text, and caches the value through onChange so
the reducer has it. Hide re-masks without re-fetching. Show is
disabled while the instance is dirty — the on-disk value would be
overwritten by save anyway, so revealing a stale snapshot is
confusing. Error from /reveal renders in place of the help text.

Pairs with a FieldList prop change: callers now pass
originalOptions + instanceKey so SecretInput can decide dirtiness
per-field. Temporarily breaks type-check until Editor updates in
task 4.
```

---

## Task 3: TestConnection component

**Files:**
- Create: `web/src/components/TestConnection.tsx`
- Create: `web/src/components/TestConnection.module.css`

Button + inline result. Disabled when instance is dirty (test endpoint reads disk-backed options, not in-memory draft — testing a dirty instance would probe the previous saved state, not what the user is editing).

- [ ] **Step 1: Create `web/src/components/TestConnection.module.css`**

```css
.wrap {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-top: 8px;
}
.btn {
  appearance: none;
  padding: 8px 14px;
  background: transparent;
  color: var(--text);
  border: 1px solid var(--border);
  border-radius: 6px;
  font: inherit;
  font-size: 13px;
  cursor: pointer;
}
.btn:hover:not(:disabled) { background: var(--hover-tint); }
.btn:disabled { cursor: not-allowed; opacity: 0.5; }
.result {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
}
.resultOk { color: var(--success); }
.resultErr { color: var(--error); }
.resultWarn { color: var(--muted); }
.dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  display: inline-block;
}
.dotOk   { background: var(--success); }
.dotErr  { background: var(--error); }
.dotWarn { background: var(--muted); }
```

- [ ] **Step 2: Create `web/src/components/TestConnection.tsx`**

```tsx
import { useState } from 'react';
import styles from './TestConnection.module.css';
import { apiFetch, ApiError } from '../api/client';
import { PlatformTestResponseSchema } from '../api/schemas';

export interface TestConnectionProps {
  instanceKey: string;
  dirty: boolean;
}

type Result =
  | { kind: 'idle' }
  | { kind: 'busy' }
  | { kind: 'ok' }
  | { kind: 'err'; msg: string }
  | { kind: 'warn'; msg: string }; // 501 + similar: probe not implemented

export default function TestConnection({ instanceKey, dirty }: TestConnectionProps) {
  const [result, setResult] = useState<Result>({ kind: 'idle' });

  async function runProbe() {
    setResult({ kind: 'busy' });
    try {
      const res = await apiFetch(
        `/api/platforms/${encodeURIComponent(instanceKey)}/test`,
        { method: 'POST', schema: PlatformTestResponseSchema },
      );
      if (res.ok) {
        setResult({ kind: 'ok' });
      } else {
        setResult({ kind: 'err', msg: res.error ?? 'probe failed' });
      }
    } catch (e) {
      if (e instanceof ApiError && e.status === 501) {
        setResult({
          kind: 'warn',
          msg: 'no probe for this platform type',
        });
        return;
      }
      setResult({ kind: 'err', msg: toMsg(e) });
    }
  }

  return (
    <div className={styles.wrap}>
      <button
        type="button"
        className={styles.btn}
        onClick={runProbe}
        disabled={result.kind === 'busy' || dirty}
        title={dirty ? 'Save changes first — probe uses on-disk config' : undefined}
      >
        {result.kind === 'busy' ? 'Testing…' : 'Test connection'}
      </button>
      {renderResult(result)}
    </div>
  );
}

function renderResult(r: Result) {
  if (r.kind === 'idle' || r.kind === 'busy') return null;
  if (r.kind === 'ok') {
    return (
      <span className={`${styles.result} ${styles.resultOk}`}>
        <span className={`${styles.dot} ${styles.dotOk}`} />
        connected
      </span>
    );
  }
  if (r.kind === 'warn') {
    return (
      <span className={`${styles.result} ${styles.resultWarn}`}>
        <span className={`${styles.dot} ${styles.dotWarn}`} />
        {r.msg}
      </span>
    );
  }
  return (
    <span className={`${styles.result} ${styles.resultErr}`}>
      <span className={`${styles.dot} ${styles.dotErr}`} />
      {r.msg}
    </span>
  );
}

function toMsg(e: unknown): string {
  if (e instanceof ApiError) {
    if (typeof e.body === 'object' && e.body !== null && 'error' in e.body) {
      const m = (e.body as { error?: unknown }).error;
      if (typeof m === 'string') return m;
    }
    return `HTTP ${e.status}`;
  }
  return e instanceof Error ? e.message : String(e);
}
```

- [ ] **Step 3: type-check**

Only the isolated file should compile; the Editor-caller errors from Task 2 are still present but no new errors should appear.

```bash
(cd /Users/ranwei/.config/superpowers/worktrees/hermind/feat-editor-fields-4b/web && pnpm type-check 2>&1 | grep -c "src/components/TestConnection" | xargs -I{} echo "TestConnection errors: {}")
```

Expected: `TestConnection errors: 0`.

- [ ] **Step 4: Commit**

```bash
(cd /Users/ranwei/.config/superpowers/worktrees/hermind/feat-editor-fields-4b && git add web/src/components/TestConnection.tsx web/src/components/TestConnection.module.css)
```

Commit with exactly:

```
feat(web/components): TestConnection button + inline result

Single-button pane sitting below the FieldList. Click POSTs
/api/platforms/{key}/test, which delegates to descriptor.Test and
returns {ok, error}. 501 (descriptor has no Test closure — the 7
outbound-webhook types) becomes a grey "no probe" chip rather than
a red error. The button is disabled while the instance is dirty —
probe reads the on-disk options, not the in-memory draft.
```

---

## Task 4: Editor integration — SecretInput options + TestConnection slot

**Files:**
- Modify: `web/src/components/Editor.tsx`
- Modify: `web/src/components/Editor.module.css`

Editor needs to pass `originalOptions` and `instanceKey` to FieldList, and render `TestConnection` between the FieldList and the danger zone. Since "dirty" applies to the instance as a whole (for the Test button) AND per-field (for SecretInput), Editor computes both from its new `originalInstance` prop.

- [ ] **Step 1: Overwrite `web/src/components/Editor.tsx`**

```tsx
import styles from './Editor.module.css';
import type { PlatformInstance, SchemaDescriptor } from '../api/schemas';
import FieldList from './FieldList';
import TestConnection from './TestConnection';

export interface EditorProps {
  selectedKey: string | null;
  instance: PlatformInstance | null;
  originalInstance: PlatformInstance | null;
  descriptor: SchemaDescriptor | null;
  onField: (field: string, value: string) => void;
  onToggleEnabled: (enabled: boolean) => void;
  onDelete: () => void;
}

export default function Editor({
  selectedKey,
  instance,
  originalInstance,
  descriptor,
  onField,
  onToggleEnabled,
  onDelete,
}: EditorProps) {
  if (!selectedKey || !instance) {
    return (
      <div className={styles.wrapper}>
        <div className={styles.emptyCard}>
          <h2 className={styles.emptyTitle}>No instance selected</h2>
          <p className={styles.emptyBody}>
            Pick an instance from the sidebar, or click <em>+ New instance</em>
            to create one.
          </p>
        </div>
      </div>
    );
  }
  if (!descriptor) {
    return (
      <div className={styles.wrapper}>
        <div className={styles.emptyCard}>
          <h2 className={styles.emptyTitle}>Unknown platform type</h2>
          <p className={styles.emptyBody}>
            {selectedKey} is configured as type <code>{instance.type}</code>,
            which has no registered descriptor. Update the YAML directly or
            delete this instance.
          </p>
          <button
            type="button"
            className={styles.deleteBtn}
            onClick={() => {
              if (window.confirm(`Delete instance "${selectedKey}"?`)) onDelete();
            }}
          >
            Delete instance
          </button>
        </div>
      </div>
    );
  }

  const originalOptions = originalInstance?.options ?? {};
  const options = instance.options ?? {};
  // Instance is dirty iff any field value or enabled differs from the
  // original. New instances (no originalInstance) are dirty by construction.
  const instanceIsDirty =
    !originalInstance ||
    (originalInstance.enabled ?? false) !== (instance.enabled ?? false) ||
    optionsDiffer(options, originalOptions);

  return (
    <div className={styles.wrapper}>
      <section className={styles.panel}>
        <header className={styles.panelHeader}>
          <h2 className={styles.title}>{selectedKey}</h2>
          <span className={styles.typeTag}>{descriptor.display_name}</span>
          <span className={styles.headerSpacer} />
          <label className={styles.enabledToggle}>
            <input
              type="checkbox"
              checked={instance.enabled ?? false}
              onChange={e => onToggleEnabled(e.currentTarget.checked)}
            />
            Enabled
          </label>
        </header>
        {descriptor.summary && (
          <p className={styles.summary}>{descriptor.summary}</p>
        )}
        <FieldList
          descriptor={descriptor}
          options={options}
          originalOptions={originalOptions}
          instanceKey={selectedKey}
          onChange={onField}
        />
        <TestConnection instanceKey={selectedKey} dirty={instanceIsDirty} />
        <div className={styles.dangerZone}>
          <button
            type="button"
            className={styles.deleteBtn}
            onClick={() => {
              if (window.confirm(`Delete instance "${selectedKey}"?`)) onDelete();
            }}
          >
            Delete instance
          </button>
        </div>
      </section>
    </div>
  );
}

function optionsDiffer(a: Record<string, string>, b: Record<string, string>): boolean {
  const keys = new Set<string>([...Object.keys(a), ...Object.keys(b)]);
  for (const k of keys) {
    if ((a[k] ?? '') !== (b[k] ?? '')) return true;
  }
  return false;
}
```

- [ ] **Step 2: `web/src/components/Editor.module.css` is unchanged**

No CSS additions needed for this task — TestConnection brings its own module. Verify with `git diff` that `Editor.module.css` is not touched.

- [ ] **Step 3: type-check — expect App.tsx errors only**

```bash
(cd /Users/ranwei/.config/superpowers/worktrees/hermind/feat-editor-fields-4b/web && pnpm type-check 2>&1 | head -10)
```

Expected: errors about App.tsx not passing `originalInstance` to Editor. Task 5 closes this.

- [ ] **Step 4: Commit**

```bash
(cd /Users/ranwei/.config/superpowers/worktrees/hermind/feat-editor-fields-4b && git add web/src/components/Editor.tsx)
```

Commit with exactly:

```
feat(web/editor): wire SecretInput options + TestConnection slot

Editor now computes per-field dirtiness (via originalOptions) and
instance-level dirtiness (via originalInstance enabled + options
diff) and feeds both down: FieldList gets originalOptions, Test
Connection gets the instance-level flag so its Show and Test
buttons disable correctly. Temporarily breaks type-check until
App.tsx passes originalInstance (Task 5).
```

---

## Task 5: App.tsx — originalInstance prop + NewInstanceDialog host state

**Files:**
- Modify: `web/src/App.tsx`

App now looks up the originalInstance alongside the instance and passes both down. This task also sets up the NewInstanceDialog host state (`newDialogOpen`) and a placeholder callback — the dialog itself lands in Task 6.

- [ ] **Step 1: Overwrite `web/src/App.tsx`**

```tsx
import { useCallback, useEffect, useMemo, useReducer, useState } from 'react';
import { apiFetch, ApiError } from './api/client';
import {
  ApplyResultSchema,
  ConfigResponseSchema,
  PlatformsSchemaResponseSchema,
} from './api/schemas';
import {
  dirtyCount as selectDirtyCount,
  initialState,
  instanceDirty,
  listInstances,
  reducer,
} from './state';
import TopBar from './components/TopBar';
import Sidebar from './components/Sidebar';
import Footer from './components/Footer';
import Editor from './components/Editor';

export default function App() {
  const [state, dispatch] = useReducer(reducer, initialState);
  const [newDialogOpen, setNewDialogOpen] = useState(false);

  useEffect(() => {
    const ctrl = new AbortController();
    (async () => {
      try {
        const [schema, cfg] = await Promise.all([
          apiFetch('/api/platforms/schema', {
            schema: PlatformsSchemaResponseSchema,
            signal: ctrl.signal,
          }),
          apiFetch('/api/config', {
            schema: ConfigResponseSchema,
            signal: ctrl.signal,
          }),
        ]);
        dispatch({
          type: 'boot/loaded',
          descriptors: schema.descriptors,
          config: cfg.config,
        });
      } catch (err) {
        if (ctrl.signal.aborted) return;
        const msg = err instanceof Error ? err.message : 'boot failed';
        dispatch({ type: 'boot/failed', error: msg });
      }
    })();
    return () => ctrl.abort();
  }, []);

  useEffect(() => {
    if (state.status === 'booting') return;
    const encoded = state.selectedKey ? encodeURIComponent(state.selectedKey) : '';
    const wanted = encoded ? '#' + encoded : '';
    if (window.location.hash !== wanted) {
      if (encoded) {
        window.location.hash = encoded;
      } else if (window.location.hash) {
        history.replaceState(null, '', window.location.pathname + window.location.search);
      }
    }
  }, [state.selectedKey, state.status]);

  useEffect(() => {
    if (state.status !== 'ready' || state.selectedKey !== null) return;
    const raw = window.location.hash.replace(/^#/, '');
    let fromHash = '';
    try {
      fromHash = decodeURIComponent(raw);
    } catch {
      fromHash = raw;
    }
    if (fromHash && state.config.gateway?.platforms?.[fromHash]) {
      dispatch({ type: 'select', key: fromHash });
    }
  }, [state.status, state.selectedKey, state.config.gateway?.platforms]);

  const instances = useMemo(() => {
    const plats = state.config.gateway?.platforms ?? {};
    return listInstances(state).map(key => ({
      key,
      type: plats[key]?.type ?? '',
      enabled: plats[key]?.enabled ?? false,
    }));
  }, [state.config.gateway?.platforms]);

  const dirtyKeys = useMemo(() => {
    const a = state.config.gateway?.platforms ?? {};
    const b = state.originalConfig.gateway?.platforms ?? {};
    const keys = new Set([...Object.keys(a), ...Object.keys(b)]);
    const out = new Set<string>();
    for (const k of keys) {
      if (instanceDirty(state, k)) out.add(k);
    }
    return out;
  }, [state]);

  const dirty = selectDirtyCount(state);
  const busy = state.status === 'saving' || state.status === 'applying';

  const onSave = useCallback(async () => {
    dispatch({ type: 'save/start' });
    try {
      await apiFetch('/api/config', {
        method: 'PUT',
        body: { config: state.config },
      });
      dispatch({ type: 'save/done' });
    } catch (err) {
      const msg = toErrMsg(err);
      dispatch({ type: 'save/done', error: msg });
    }
  }, [state.config]);

  const onSaveAndApply = useCallback(async () => {
    dispatch({ type: 'save/start' });
    try {
      await apiFetch('/api/config', {
        method: 'PUT',
        body: { config: state.config },
      });
      dispatch({ type: 'save/done' });
    } catch (err) {
      dispatch({ type: 'save/done', error: toErrMsg(err) });
      return;
    }
    dispatch({ type: 'apply/start' });
    try {
      const res = await apiFetch('/api/platforms/apply', {
        method: 'POST',
        schema: ApplyResultSchema,
      });
      if (res.ok) {
        dispatch({ type: 'apply/done' });
      } else {
        dispatch({ type: 'apply/done', error: res.error ?? 'apply failed' });
      }
    } catch (err) {
      dispatch({ type: 'apply/done', error: toErrMsg(err) });
    }
  }, [state.config]);

  if (state.status === 'booting') {
    return <div style={{ padding: '2rem' }}>Loading…</div>;
  }
  if (state.status === 'error' && state.descriptors.length === 0) {
    return (
      <div style={{ padding: '2rem', color: 'var(--error)' }}>
        Boot failed: {state.flash?.msg ?? 'unknown error'}
      </div>
    );
  }

  const selectedInstance = state.selectedKey
    ? state.config.gateway?.platforms?.[state.selectedKey] ?? null
    : null;
  const selectedOriginal = state.selectedKey
    ? state.originalConfig.gateway?.platforms?.[state.selectedKey] ?? null
    : null;
  const selectedDescriptor = selectedInstance
    ? state.descriptors.find(d => d.type === selectedInstance.type) ?? null
    : null;

  return (
    <div className="app-shell">
      <TopBar dirtyCount={dirty} status={state.status} />
      <Sidebar
        instances={instances}
        selectedKey={state.selectedKey}
        descriptors={state.descriptors}
        dirtyKeys={dirtyKeys}
        onSelect={key => dispatch({ type: 'select', key })}
        onNewInstance={() => setNewDialogOpen(true)}
      />
      <main>
        <Editor
          selectedKey={state.selectedKey}
          instance={selectedInstance}
          originalInstance={selectedOriginal}
          descriptor={selectedDescriptor}
          onField={(field, value) =>
            state.selectedKey &&
            dispatch({ type: 'edit/field', key: state.selectedKey, field, value })
          }
          onToggleEnabled={enabled =>
            state.selectedKey &&
            dispatch({ type: 'edit/enabled', key: state.selectedKey, enabled })
          }
          onDelete={() =>
            state.selectedKey &&
            dispatch({ type: 'instance/delete', key: state.selectedKey })
          }
        />
      </main>
      <Footer
        dirtyCount={dirty}
        flash={state.flash}
        busy={busy}
        onSave={onSave}
        onSaveAndApply={onSaveAndApply}
      />
      {newDialogOpen && (
        <div style={{ position: 'fixed', inset: 0, background: 'rgba(0,0,0,0.5)' }}>
          {/* Placeholder — NewInstanceDialog lands in task 6 */}
          <button
            type="button"
            onClick={() => setNewDialogOpen(false)}
            style={{ position: 'absolute', top: 16, right: 16 }}
          >
            close
          </button>
          <p style={{ color: 'white', padding: '2rem' }}>
            NewInstanceDialog coming in task 6.
          </p>
        </div>
      )}
    </div>
  );
}

function toErrMsg(err: unknown): string {
  if (err instanceof ApiError) {
    if (typeof err.body === 'object' && err.body !== null && 'error' in err.body) {
      const e = (err.body as { error?: unknown }).error;
      if (typeof e === 'string') return e;
    }
    return `HTTP ${err.status}`;
  }
  return err instanceof Error ? err.message : String(err);
}
```

- [ ] **Step 2: type-check + build**

```bash
(cd /Users/ranwei/.config/superpowers/worktrees/hermind/feat-editor-fields-4b/web && pnpm type-check)
(cd /Users/ranwei/.config/superpowers/worktrees/hermind/feat-editor-fields-4b && make web)
```

Expected: type-check clean, build succeeds. This closes the intermediate gaps from Tasks 2 and 4.

- [ ] **Step 3: Commit**

Stage App.tsx + rebuilt webroot:

```bash
(cd /Users/ranwei/.config/superpowers/worktrees/hermind/feat-editor-fields-4b && git add web/src/App.tsx api/webroot/)
```

Commit with exactly:

```
feat(web/app): pass originalInstance to Editor + host dialog state

Editor now needs originalInstance to compute per-field + instance
dirtiness for SecretInput.Show and TestConnection respectively.
Also hosts a newDialogOpen flag — the full NewInstanceDialog lands
in task 6; a minimal overlay stub lives here for now so the
Sidebar "+ New instance" button has somewhere to click.
```

---

## Task 6: NewInstanceDialog component + instance/create reducer action

**Files:**
- Modify: `web/src/state.ts`
- Create: `web/src/components/NewInstanceDialog.tsx`
- Create: `web/src/components/NewInstanceDialog.module.css`
- Modify: `web/src/App.tsx`

The dialog uses the browser's `<dialog>` element for the modal surface — it handles focus trapping, ESC-to-close, and backdrop rendering for free. The form collects a type (select from descriptors) + a key (regex-validated) and submits via the `instance/create` reducer action.

**Validation rules for the instance key:**
- Matches `^[a-z][a-z0-9_]*$`.
- Not already present in `config.gateway.platforms`.

- [ ] **Step 1: Modify `web/src/state.ts`**

Add the `instance/create` action variant. Open the current file and make two changes:

A) In the `Action` union (after `| { type: 'instance/delete'; key: string }`), append:

```ts
  | { type: 'instance/create'; key: string; platformType: string };
```

B) In the `reducer` switch, after the `case 'instance/delete':` block, add:

```ts
    case 'instance/create': {
      const plats = { ...(state.config.gateway?.platforms ?? {}) };
      plats[action.key] = {
        enabled: true,
        type: action.platformType,
        options: {},
      };
      return {
        ...state,
        config: { ...state.config, gateway: { ...(state.config.gateway ?? {}), platforms: plats } },
        selectedKey: action.key,
      };
    }
```

The curly braces around the case body are required because we declare `const plats` with block scope.

- [ ] **Step 2: Create `web/src/components/NewInstanceDialog.module.css`**

```css
.dialog {
  border: 0;
  border-radius: 8px;
  padding: 0;
  background: var(--bg);
  color: var(--text);
  max-width: 440px;
  width: 100%;
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.3);
}
.dialog::backdrop {
  background: rgba(0, 0, 0, 0.4);
}
.header {
  padding: 16px 20px;
  border-bottom: 1px solid var(--border);
  display: flex;
  align-items: center;
}
.title {
  margin: 0;
  font-size: 15px;
  font-weight: 500;
}
.spacer { flex: 1; }
.close {
  appearance: none;
  background: transparent;
  border: 0;
  font-size: 18px;
  color: var(--muted);
  cursor: pointer;
  padding: 0;
}
.close:hover { color: var(--text); }
.body {
  padding: 20px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.field { display: flex; flex-direction: column; gap: 6px; }
.label { font-size: 13px; font-weight: 500; }
.hint  { font-size: 12px; color: var(--muted); }
.input,
.select {
  width: 100%;
  height: 36px;
  padding: 0 12px;
  font-size: 14px;
  font-family: inherit;
  color: var(--text);
  background: var(--bg);
  border: 1px solid var(--border);
  border-radius: 6px;
}
.input:focus, .select:focus {
  outline: none;
  border-color: var(--accent);
  box-shadow: 0 0 0 2px var(--focus);
}
.error {
  font-size: 12px;
  color: var(--error);
}
.footer {
  padding: 16px 20px;
  border-top: 1px solid var(--border);
  display: flex;
  gap: 8px;
}
.footerSpacer { flex: 1; }
.btn {
  height: 32px;
  padding: 0 14px;
  font: inherit;
  font-size: 13px;
  border-radius: 6px;
  cursor: pointer;
}
.btn:disabled { opacity: 0.5; cursor: not-allowed; }
.secondary {
  background: transparent;
  border: 1px solid var(--border);
  color: var(--text);
}
.secondary:hover:not(:disabled) { background: var(--hover-tint); }
.primary {
  background: var(--accent);
  border: 0;
  color: var(--accent-fg);
  font-weight: 500;
}
.primary:hover:not(:disabled) { filter: brightness(0.95); }
```

- [ ] **Step 3: Create `web/src/components/NewInstanceDialog.tsx`**

```tsx
import { useEffect, useRef, useState } from 'react';
import styles from './NewInstanceDialog.module.css';
import type { SchemaDescriptor } from '../api/schemas';

export interface NewInstanceDialogProps {
  descriptors: SchemaDescriptor[];
  existingKeys: Set<string>;
  onCancel: () => void;
  onCreate: (key: string, platformType: string) => void;
}

const KEY_REGEX = /^[a-z][a-z0-9_]*$/;

export default function NewInstanceDialog({
  descriptors,
  existingKeys,
  onCancel,
  onCreate,
}: NewInstanceDialogProps) {
  const dialogRef = useRef<HTMLDialogElement>(null);
  const [key, setKey] = useState('');
  const [platformType, setPlatformType] = useState(
    descriptors[0]?.type ?? '',
  );
  const [keyError, setKeyError] = useState<string | null>(null);

  useEffect(() => {
    dialogRef.current?.showModal();
  }, []);

  function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    const trimmed = key.trim();
    if (!trimmed) {
      setKeyError('Instance key is required.');
      return;
    }
    if (!KEY_REGEX.test(trimmed)) {
      setKeyError('Use lowercase letters, digits, underscore. Must start with a letter.');
      return;
    }
    if (existingKeys.has(trimmed)) {
      setKeyError(`An instance named "${trimmed}" already exists.`);
      return;
    }
    if (!platformType) {
      setKeyError('Pick a platform type.');
      return;
    }
    onCreate(trimmed, platformType);
  }

  return (
    <dialog
      ref={dialogRef}
      className={styles.dialog}
      onCancel={e => {
        e.preventDefault();
        onCancel();
      }}
      onClose={() => onCancel()}
    >
      <form onSubmit={onSubmit}>
        <header className={styles.header}>
          <h2 className={styles.title}>New instance</h2>
          <span className={styles.spacer} />
          <button
            type="button"
            className={styles.close}
            onClick={onCancel}
            aria-label="Close"
          >
            ✕
          </button>
        </header>
        <div className={styles.body}>
          <div className={styles.field}>
            <label className={styles.label} htmlFor="new-instance-type">
              Platform type
            </label>
            <select
              id="new-instance-type"
              className={styles.select}
              value={platformType}
              onChange={e => setPlatformType(e.currentTarget.value)}
            >
              {descriptors.map(d => (
                <option key={d.type} value={d.type}>
                  {d.display_name} ({d.type})
                </option>
              ))}
            </select>
          </div>
          <div className={styles.field}>
            <label className={styles.label} htmlFor="new-instance-key">
              Instance key
            </label>
            <input
              id="new-instance-key"
              className={styles.input}
              value={key}
              placeholder="tg_main"
              autoFocus
              onChange={e => {
                setKey(e.currentTarget.value);
                setKeyError(null);
              }}
            />
            <span className={styles.hint}>
              Identifier under <code>gateway.platforms.*</code>. Lowercase, underscores.
              Immutable after creation.
            </span>
            {keyError && <span className={styles.error}>{keyError}</span>}
          </div>
        </div>
        <footer className={styles.footer}>
          <button
            type="button"
            className={`${styles.btn} ${styles.secondary}`}
            onClick={onCancel}
          >
            Cancel
          </button>
          <span className={styles.footerSpacer} />
          <button
            type="submit"
            className={`${styles.btn} ${styles.primary}`}
          >
            Create
          </button>
        </footer>
      </form>
    </dialog>
  );
}
```

- [ ] **Step 4: Modify `web/src/App.tsx` — replace the stub overlay**

In the previous App.tsx, the end of the `return (...)` block has a placeholder:

```tsx
      {newDialogOpen && (
        <div style={{ position: 'fixed', inset: 0, background: 'rgba(0,0,0,0.5)' }}>
          {/* Placeholder — NewInstanceDialog lands in task 6 */}
          <button
            type="button"
            onClick={() => setNewDialogOpen(false)}
            style={{ position: 'absolute', top: 16, right: 16 }}
          >
            close
          </button>
          <p style={{ color: 'white', padding: '2rem' }}>
            NewInstanceDialog coming in task 6.
          </p>
        </div>
      )}
```

Replace with:

```tsx
      {newDialogOpen && (
        <NewInstanceDialog
          descriptors={state.descriptors}
          existingKeys={new Set(Object.keys(state.config.gateway?.platforms ?? {}))}
          onCancel={() => setNewDialogOpen(false)}
          onCreate={(key, platformType) => {
            dispatch({ type: 'instance/create', key, platformType });
            setNewDialogOpen(false);
          }}
        />
      )}
```

Add the import near the other component imports:

```tsx
import NewInstanceDialog from './components/NewInstanceDialog';
```

- [ ] **Step 5: type-check + build**

```bash
(cd /Users/ranwei/.config/superpowers/worktrees/hermind/feat-editor-fields-4b/web && pnpm type-check)
(cd /Users/ranwei/.config/superpowers/worktrees/hermind/feat-editor-fields-4b && make web)
```

Expected: type-check clean, build succeeds, api/webroot refreshed.

- [ ] **Step 6: Commit**

Stage the 4 files + rebuilt webroot:

```bash
(cd /Users/ranwei/.config/superpowers/worktrees/hermind/feat-editor-fields-4b && git add web/src/state.ts web/src/components/NewInstanceDialog.tsx web/src/components/NewInstanceDialog.module.css web/src/App.tsx api/webroot/)
```

Commit with exactly:

```
feat(web/dialog): NewInstanceDialog + instance/create action

Modal picks a type from descriptors and validates a user-entered
key (^[a-z][a-z0-9_]*$, not already in use). Submit dispatches
instance/create which seeds an enabled entry with an empty options
blob and selects the new key. The dialog uses the native <dialog>
element so focus trap + ESC dismiss + backdrop all work without a
library.
```

---

## Task 7: Final E2E + build sweep

No code changes — manual + scripted verification.

- [ ] **Step 1: Full build + Go tests**

```bash
(cd /Users/ranwei/.config/superpowers/worktrees/hermind/feat-editor-fields-4b && make web-clean && make web && go test ./api/... ./cli/... ./gateway/... 2>&1 | tail -5)
```

Expected: clean build, Go tests green.

- [ ] **Step 2: Boot hermind web**

```bash
mkdir -p /tmp/e2e-home/.hermind
cat > /tmp/e2e-home/.hermind/config.yaml <<'EOF'
model: claude-sonnet-4-5-20250929
providers:
  anthropic:
    provider: anthropic
    api_key: test-placeholder
storage:
  driver: sqlite
  sqlite_path: /tmp/e2e-home/.hermind/hermind.db
EOF

(cd /Users/ranwei/.config/superpowers/worktrees/hermind/feat-editor-fields-4b && go build -o bin/hermind ./cmd/hermind)

HOME=/tmp/e2e-home /Users/ranwei/.config/superpowers/worktrees/hermind/feat-editor-fields-4b/bin/hermind web --addr=127.0.0.1:9119 --no-browser --exit-after=300s > /tmp/hermind-s4b-e2e.log 2>&1 &
sleep 2
TOK=$(grep "^token:" /tmp/hermind-s4b-e2e.log | awk '{print $2}')
echo "URL: http://127.0.0.1:9119/?t=$TOK"
```

- [ ] **Step 3: Browser walkthrough**

Open the URL. Verify:

1. Sidebar is empty ("No instances configured"). Main pane shows the no-selection card. Footer buttons disabled.
2. Click `+ New instance`. The modal opens, platform type select defaults to `acp` (alphabetical first), key input is focused.
3. Try `ACP_bot` as key — validation fails ("lowercase letters, digits, underscore"). Try `acp bot` — same. Type `acp_bot`, pick Telegram Bot from the dropdown, click Create.
4. Dialog closes. Sidebar shows `acp_bot` with the amber dirty dot. Main pane shows the Telegram editor with an empty Bot Token field. Footer shows "1 unsaved change" and Save/Apply enabled.
5. Click the `Test connection` button. Response should be `connected` for the few seconds the probe takes, OR an error — but NOT disabled, since a brand-new instance is dirty and will actually gets disabled. (This is the correct behavior — the test endpoint reads on-disk config, which doesn't yet have `acp_bot`.)
6. Click `Save`. Footer flashes green. Sidebar dot clears.
7. Now click `Test connection`. With a bogus empty token Telegram returns 404 — the result cell shows a red dot + `probe failed: status 404: ...`.
8. Type `123:abcdef` into the Bot Token field. Click `Show`. Button becomes disabled (instance is dirty). Save first. Click `Show` again — the input flips to `text` and displays `123:abcdef`. Click `Hide` — it re-masks.
9. Seed a different type to verify the 501 path:
   ```bash
   curl -s -X PUT -H "Authorization: Bearer $TOK" -H "Content-Type: application/json" \
     -d '{"config":{"gateway":{"platforms":{"acp_bot":{"enabled":true,"type":"telegram","options":{"token":"123:abcdef"}},"slack_stub":{"enabled":true,"type":"slack","options":{"webhook_url":"https://example.com"}}}}}}' \
     http://127.0.0.1:9119/api/config
   ```
   Refresh the page. Select `slack_stub`, click Test — the result should read grey "no probe for this platform type" (501 → warn).
10. Click Save & Apply. Footer flashes "Applied." TopBar status dot pulses briefly during the transition.

- [ ] **Step 4: Kill the server + commit history sanity**

```bash
kill %1 2>/dev/null || true
(cd /Users/ranwei/.config/superpowers/worktrees/hermind/feat-editor-fields-4b && git log --oneline <stage-4b-base>..HEAD)
```

Expected: 6 feat/chore commits, one per task.

---

## Rollback

`git reset --hard <commit-before-task-1>` on the branch. No backend state touched. The app reverts to the Stage 4a edit-and-save functionality with no SecretInput / TestConnection / NewInstanceDialog.

## Known scope cuts (Stage 5)

- **No vitest unit tests.** Dirty diff, secret preserve, reducer actions all uncovered by automated tests. Stage 5.
- **No ESLint config.** Stage 5.
- **No CI `make web` sync assertion.** A PR that edits `web/` without syncing `api/webroot/` still merges dirty. Stage 5.
- **No README for the dev loop.** Stage 5.
- **`<dialog>` fallback for old browsers** — we rely on native support (Safari 15.4+, Firefox 98+, Chrome 37+). All modern browsers covered; no polyfill.
