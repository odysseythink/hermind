# Editor + Fields (Stage 4a) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the Editor live. Users pick an instance, edit its fields with real inputs (text/number/bool/enum), toggle enabled, delete, and Save / Save & Apply. The dirty flag is driven by a structural diff against `originalConfig`. Selection persists across refresh via the URL hash.

**Architecture:** `state.ts` gains four new actions (`edit/field`, `instance/delete`, `instance/toggle_enabled`, `hash/select`) and a `dirtyCount` selector. `App.tsx` wires Save + Save & Apply handlers that call `/api/config` and `/api/platforms/apply`, drives dirty detection, and syncs `state.selectedKey ↔ window.location.hash`. A new `components/fields/` directory houses four small input controls; `FieldList.tsx` dispatches by `FieldKind` and feeds each control the current value + an onChange that dispatches `edit/field`. `Editor.tsx` renders the FieldList plus an enabled checkbox and a delete button in the header. `FieldSecret` renders as a plain text input in Stage 4a — Stage 4b replaces it with the reveal-capable `SecretInput`.

**Tech Stack:** React 18, TypeScript 5, zod (existing). No new dependencies.

**Source of truth:** `docs/superpowers/specs/2026-04-19-web-im-config-design.md` §8 "Frontend architecture" + §6 (secret round-trip) + §11 (stage 4 entry).

**Explicit scope cuts (deferred to Stage 4b):**
- **SecretInput with /reveal** — `FieldSecret` renders as an empty text input for now. Users can type a new value and save (backend preserves empty-string secrets), but can't see the stored value.
- **TestConnection button** — no UI trigger; users probe via `curl POST /api/platforms/<key>/test` manually.
- **NewInstanceDialog** — the Sidebar's "+ New instance" button is still a log-only stub. Users still create instances via `curl PUT /api/config`.

These cuts keep 4a focused on the edit-and-save backbone; 4b adds the side-effectful controls on top.

---

## File Structure

**Create:**

- `web/src/components/fields/TextInput.tsx`
- `web/src/components/fields/NumberInput.tsx`
- `web/src/components/fields/BoolToggle.tsx`
- `web/src/components/fields/EnumSelect.tsx`
- `web/src/components/fields/fields.module.css` (shared styling for the four controls + the `FieldList` wrapper)
- `web/src/components/FieldList.tsx`

**Modify:**

- `web/src/state.ts` — add 4 actions + dirty selector + helper for deep-config path updates.
- `web/src/App.tsx` — Save/Apply handlers, dirty count, hash persistence.
- `web/src/components/Editor.tsx` — replace stage placeholder with `FieldList`, add enabled toggle + delete button in the header.
- `web/src/components/Sidebar.tsx` — show a per-instance dirty indicator (amber dot when that key differs from originalConfig). Optional but cheap.
- `web/src/components/Editor.module.css` — style the header's enabled toggle + delete button.

**Untouched:**

- `web/src/api/{client,schemas}.ts` (already has everything 4a needs).
- `web/src/components/{TopBar,Footer,Sidebar}.module.css` (Sidebar only gets a JSX tweak, not CSS).

---

## Task 1: State extensions + dirty selector + path-set helper

**Files:**
- Modify: `web/src/state.ts`

- [ ] **Step 1: Overwrite `web/src/state.ts`**

```ts
import type { Config, PlatformInstance, SchemaDescriptor } from './api/schemas';

export type Status = 'booting' | 'ready' | 'saving' | 'applying' | 'error';

export interface Flash {
  kind: 'ok' | 'err';
  msg: string;
}

export interface AppState {
  status: Status;
  descriptors: SchemaDescriptor[];
  config: Config;
  originalConfig: Config;
  selectedKey: string | null;
  flash: Flash | null;
}

export type Action =
  | { type: 'boot/loaded'; descriptors: SchemaDescriptor[]; config: Config }
  | { type: 'boot/failed'; error: string }
  | { type: 'select'; key: string | null }
  | { type: 'flash'; flash: Flash | null }
  | { type: 'save/start' }
  | { type: 'save/done'; error?: string }
  | { type: 'apply/start' }
  | { type: 'apply/done'; error?: string }
  | { type: 'edit/field'; key: string; field: string; value: string }
  | { type: 'edit/enabled'; key: string; enabled: boolean }
  | { type: 'instance/delete'; key: string };

export const initialState: AppState = {
  status: 'booting',
  descriptors: [],
  config: {},
  originalConfig: {},
  selectedKey: null,
  flash: null,
};

export function reducer(state: AppState, action: Action): AppState {
  switch (action.type) {
    case 'boot/loaded':
      return {
        ...state,
        status: 'ready',
        descriptors: action.descriptors,
        config: action.config,
        originalConfig: action.config,
      };
    case 'boot/failed':
      return {
        ...state,
        status: 'error',
        flash: { kind: 'err', msg: action.error },
      };
    case 'select':
      return { ...state, selectedKey: action.key };
    case 'flash':
      return { ...state, flash: action.flash };
    case 'save/start':
      return { ...state, status: 'saving', flash: null };
    case 'save/done':
      return action.error
        ? { ...state, status: 'ready', flash: { kind: 'err', msg: action.error } }
        : {
            ...state,
            status: 'ready',
            originalConfig: state.config,
            flash: { kind: 'ok', msg: 'Saved.' },
          };
    case 'apply/start':
      return { ...state, status: 'applying', flash: null };
    case 'apply/done':
      return action.error
        ? { ...state, status: 'ready', flash: { kind: 'err', msg: action.error } }
        : { ...state, status: 'ready', flash: { kind: 'ok', msg: 'Applied.' } };
    case 'edit/field':
      return { ...state, config: setField(state.config, action.key, action.field, action.value) };
    case 'edit/enabled':
      return { ...state, config: setEnabled(state.config, action.key, action.enabled) };
    case 'instance/delete':
      return {
        ...state,
        config: deleteInstance(state.config, action.key),
        selectedKey: state.selectedKey === action.key ? null : state.selectedKey,
      };
  }
}

/** listInstances returns keys in the current config.gateway.platforms map, sorted. */
export function listInstances(state: AppState): string[] {
  const plats = state.config.gateway?.platforms ?? {};
  return Object.keys(plats).sort();
}

/** dirtyCount returns how many instance keys differ between config and
 * originalConfig. Added keys count. Deleted keys count. Any mutation
 * inside a surviving key counts as one. */
export function dirtyCount(state: AppState): number {
  const a = state.config.gateway?.platforms ?? {};
  const b = state.originalConfig.gateway?.platforms ?? {};
  const keys = new Set<string>([...Object.keys(a), ...Object.keys(b)]);
  let n = 0;
  for (const k of keys) {
    if (!shallowEqualInstance(a[k], b[k])) n++;
  }
  return n;
}

/** instanceDirty returns true when a single key differs between the
 * current config and the snapshot. Used by the Sidebar to render a
 * per-instance unsaved indicator. */
export function instanceDirty(state: AppState, key: string): boolean {
  const a = state.config.gateway?.platforms?.[key];
  const b = state.originalConfig.gateway?.platforms?.[key];
  return !shallowEqualInstance(a, b);
}

function shallowEqualInstance(
  a: PlatformInstance | undefined,
  b: PlatformInstance | undefined,
): boolean {
  if (a === b) return true;
  if (!a || !b) return false;
  if (a.type !== b.type) return false;
  if ((a.enabled ?? false) !== (b.enabled ?? false)) return false;
  const ao = a.options ?? {};
  const bo = b.options ?? {};
  const keys = new Set<string>([...Object.keys(ao), ...Object.keys(bo)]);
  for (const k of keys) {
    if ((ao[k] ?? '') !== (bo[k] ?? '')) return false;
  }
  return true;
}

function setField(config: Config, key: string, field: string, value: string): Config {
  const plats = { ...(config.gateway?.platforms ?? {}) };
  const prev = plats[key];
  if (!prev) return config;
  const opts = { ...(prev.options ?? {}), [field]: value };
  plats[key] = { ...prev, options: opts };
  return { ...config, gateway: { ...(config.gateway ?? {}), platforms: plats } };
}

function setEnabled(config: Config, key: string, enabled: boolean): Config {
  const plats = { ...(config.gateway?.platforms ?? {}) };
  const prev = plats[key];
  if (!prev) return config;
  plats[key] = { ...prev, enabled };
  return { ...config, gateway: { ...(config.gateway ?? {}), platforms: plats } };
}

function deleteInstance(config: Config, key: string): Config {
  const plats = { ...(config.gateway?.platforms ?? {}) };
  if (!(key in plats)) return config;
  delete plats[key];
  return { ...config, gateway: { ...(config.gateway ?? {}), platforms: plats } };
}
```

- [ ] **Step 2: type-check**

```bash
(cd /Users/ranwei/.config/superpowers/worktrees/hermind/feat-editor-fields-4a/web && pnpm type-check)
```

Expected: PASS.

- [ ] **Step 3: Build**

```bash
(cd /Users/ranwei/.config/superpowers/worktrees/hermind/feat-editor-fields-4a && make web)
```

Expected: clean.

- [ ] **Step 4: Commit**

Stage `web/src/state.ts` and the rebuilt `api/webroot/`:

```bash
(cd /Users/ranwei/.config/superpowers/worktrees/hermind/feat-editor-fields-4a && git add web/src/state.ts api/webroot/)
```

Commit with exactly:

```
feat(web/state): dirty diff + edit actions

Extends the reducer with three mutation actions (edit/field,
edit/enabled, instance/delete) and two selectors (dirtyCount,
instanceDirty) for the Footer's unsaved-changes label and the
Sidebar's per-instance dot. Structural diff is shallow-equality
over {type, enabled, options[]} per instance — good enough for
the small config payloads we round-trip here.
```

---

## Task 2: App.tsx — Save/Apply wiring + hash persistence

**Files:**
- Modify: `web/src/App.tsx`

- [ ] **Step 1: Overwrite `web/src/App.tsx`**

```tsx
import { useCallback, useEffect, useMemo, useReducer } from 'react';
import { apiFetch, ApiError } from './api/client';
import {
  ApplyResultSchema,
  ConfigResponseSchema,
  PlatformsSchemaResponseSchema,
} from './api/schemas';
import {
  dirtyCount as selectDirtyCount,
  initialState,
  listInstances,
  reducer,
} from './state';
import TopBar from './components/TopBar';
import Sidebar from './components/Sidebar';
import Footer from './components/Footer';
import Editor from './components/Editor';

export default function App() {
  const [state, dispatch] = useReducer(reducer, initialState);

  // Boot: schema + config in parallel.
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

  // Hash persistence: read once on first ready, then write on every
  // selectedKey change. We skip the write while booting so we don't
  // clobber an incoming hash.
  useEffect(() => {
    if (state.status === 'booting') return;
    const wanted = '#' + (state.selectedKey ?? '');
    if (window.location.hash !== wanted) {
      if (state.selectedKey) {
        window.location.hash = state.selectedKey;
      } else if (window.location.hash) {
        history.replaceState(null, '', window.location.pathname + window.location.search);
      }
    }
  }, [state.selectedKey, state.status]);

  useEffect(() => {
    if (state.status !== 'ready' || state.selectedKey !== null) return;
    const fromHash = window.location.hash.replace(/^#/, '');
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
        dirtyKeys={useMemo(
          () => collectDirtyKeys(state),
          [state.config.gateway?.platforms, state.originalConfig.gateway?.platforms],
        )}
        onSelect={key => dispatch({ type: 'select', key })}
        onNewInstance={() => console.log('TODO: new instance (Stage 4b)')}
      />
      <main>
        <Editor
          selectedKey={state.selectedKey}
          instance={selectedInstance}
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

function collectDirtyKeys(state: {
  config: { gateway?: { platforms?: Record<string, unknown> } };
  originalConfig: { gateway?: { platforms?: Record<string, unknown> } };
}): Set<string> {
  const a = state.config.gateway?.platforms ?? {};
  const b = state.originalConfig.gateway?.platforms ?? {};
  const keys = new Set<string>([...Object.keys(a), ...Object.keys(b)]);
  const out = new Set<string>();
  for (const k of keys) {
    // Re-use shallow equality — simple JSON stringify is good enough
    // for a sidebar dot render; state.ts's instanceDirty is the
    // authoritative path but importing it here would double-traverse.
    if (JSON.stringify(a[k]) !== JSON.stringify(b[k])) out.add(k);
  }
  return out;
}
```

Note: `Editor` and `Sidebar` component signatures change in Tasks 4 / 5. Those tasks update both files so this App.tsx compiles at the end of Task 5; until then `pnpm type-check` will complain about the new props, which is expected.

- [ ] **Step 2: `pnpm type-check` expected to FAIL**

```bash
(cd /Users/ranwei/.config/superpowers/worktrees/hermind/feat-editor-fields-4a/web && pnpm type-check)
```

Expected: errors in `App.tsx` about `Editor` / `Sidebar` not accepting the new props (`onField`, `onToggleEnabled`, `onDelete`, `dirtyKeys`). DO NOT fix by patching App.tsx — the props are intentional and land in Tasks 4 and 5. Commit the state/App in a "work in progress, compiles after Task 5" commit.

- [ ] **Step 3: Commit**

```bash
(cd /Users/ranwei/.config/superpowers/worktrees/hermind/feat-editor-fields-4a && git add web/src/App.tsx)
```

Note: `api/webroot/` is NOT re-synced here because the build fails. That's intentional — the tree compiles after Task 5.

Commit with exactly:

```
feat(web/app): wire Save / Save & Apply + hash-persisted selection

Save calls PUT /api/config; Save & Apply calls PUT then POST
/api/platforms/apply and surfaces the result.ok flag plus any
result.error string as a footer flash. Selected-key reads from
location.hash on first ready and writes back on every change.
Temporarily does not type-check — the new Editor / Sidebar props
(onField / onToggleEnabled / onDelete / dirtyKeys) land in tasks
4 and 5. Intentional; intermediate commits inside a single-stage
branch are fine.
```

---

## Task 3: Simple field controls (Text / Number / Bool / Enum)

**Files:**
- Create: `web/src/components/fields/TextInput.tsx`
- Create: `web/src/components/fields/NumberInput.tsx`
- Create: `web/src/components/fields/BoolToggle.tsx`
- Create: `web/src/components/fields/EnumSelect.tsx`
- Create: `web/src/components/fields/fields.module.css`

Each control exposes the same prop shape so `FieldList` can dispatch uniformly: `{ field: SchemaField; value: string; onChange(value: string): void }`. Numbers and bools serialize to strings at the edit boundary — the Config's options map is always `Record<string, string>`, so the reducer stores the string form and the render converts back.

- [ ] **Step 1: Create `web/src/components/fields/fields.module.css`**

```css
.row {
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
.help {
  display: block;
  font-size: 12px;
  color: var(--muted);
  margin-top: 6px;
}
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
  transition: border-color 120ms ease, box-shadow 120ms ease;
}
.input:focus,
.select:focus {
  outline: none;
  border-color: var(--accent);
  box-shadow: 0 0 0 2px var(--focus);
}
.number {
  font-variant-numeric: tabular-nums;
}
.toggleRow {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 14px;
  margin: 16px 0;
}
.toggleRow input[type='checkbox'] {
  width: 16px;
  height: 16px;
  accent-color: var(--accent);
  cursor: pointer;
}
```

- [ ] **Step 2: Create `web/src/components/fields/TextInput.tsx`**

```tsx
import styles from './fields.module.css';
import type { SchemaField } from '../../api/schemas';

export interface FieldProps {
  field: SchemaField;
  value: string;
  onChange: (value: string) => void;
}

export default function TextInput({ field, value, onChange }: FieldProps) {
  return (
    <label className={styles.row}>
      <span className={styles.label}>
        {field.label}
        {field.required && <span className={styles.required}>*</span>}
      </span>
      <input
        type="text"
        className={styles.input}
        value={value}
        placeholder={field.default !== undefined ? String(field.default) : undefined}
        onChange={e => onChange(e.currentTarget.value)}
      />
      {field.help && <span className={styles.help}>{field.help}</span>}
    </label>
  );
}
```

- [ ] **Step 3: Create `web/src/components/fields/NumberInput.tsx`**

```tsx
import styles from './fields.module.css';
import type { FieldProps } from './TextInput';

export default function NumberInput({ field, value, onChange }: FieldProps) {
  return (
    <label className={styles.row}>
      <span className={styles.label}>
        {field.label}
        {field.required && <span className={styles.required}>*</span>}
      </span>
      <input
        type="number"
        className={`${styles.input} ${styles.number}`}
        value={value}
        placeholder={field.default !== undefined ? String(field.default) : undefined}
        onChange={e => onChange(e.currentTarget.value)}
      />
      {field.help && <span className={styles.help}>{field.help}</span>}
    </label>
  );
}
```

- [ ] **Step 4: Create `web/src/components/fields/BoolToggle.tsx`**

```tsx
import styles from './fields.module.css';
import type { FieldProps } from './TextInput';

export default function BoolToggle({ field, value, onChange }: FieldProps) {
  const checked = value === 'true';
  return (
    <label className={styles.toggleRow}>
      <input
        type="checkbox"
        checked={checked}
        onChange={e => onChange(e.currentTarget.checked ? 'true' : 'false')}
      />
      <span>
        {field.label}
        {field.required && <span className={styles.required}>*</span>}
      </span>
      {field.help && <span className={styles.help}>{field.help}</span>}
    </label>
  );
}
```

- [ ] **Step 5: Create `web/src/components/fields/EnumSelect.tsx`**

```tsx
import styles from './fields.module.css';
import type { FieldProps } from './TextInput';

export default function EnumSelect({ field, value, onChange }: FieldProps) {
  const choices = field.enum ?? [];
  return (
    <label className={styles.row}>
      <span className={styles.label}>
        {field.label}
        {field.required && <span className={styles.required}>*</span>}
      </span>
      <select
        className={styles.select}
        value={value}
        onChange={e => onChange(e.currentTarget.value)}
      >
        {!field.required && <option value="">—</option>}
        {choices.map(c => (
          <option key={c} value={c}>
            {c}
          </option>
        ))}
      </select>
      {field.help && <span className={styles.help}>{field.help}</span>}
    </label>
  );
}
```

- [ ] **Step 6: type-check the field controls in isolation**

```bash
(cd /Users/ranwei/.config/superpowers/worktrees/hermind/feat-editor-fields-4a/web && pnpm type-check)
```

Expected: the App.tsx errors from Task 2 still present, but no NEW errors from the field files. Visually scan the output — only complaints should be the pre-existing Editor / Sidebar prop mismatches.

- [ ] **Step 7: Commit**

```bash
(cd /Users/ranwei/.config/superpowers/worktrees/hermind/feat-editor-fields-4a && git add web/src/components/fields/)
```

Commit with exactly:

```
feat(web/fields): TextInput + NumberInput + BoolToggle + EnumSelect

Four string-valued form controls sharing a common FieldProps shape
(exported from TextInput for the other three to import). Numbers and
bools serialize into the options map as strings — the Config's
backing type is Record<string,string>, so keeping everything string
on the edit boundary avoids a second type layer. Required fields
get an amber asterisk; help text renders below each control.
```

---

## Task 4: FieldList dispatcher + Sidebar dirty indicator

**Files:**
- Create: `web/src/components/FieldList.tsx`
- Modify: `web/src/components/Sidebar.tsx`
- Modify: `web/src/components/Sidebar.module.css`

- [ ] **Step 1: Create `web/src/components/FieldList.tsx`**

```tsx
import type { SchemaDescriptor } from '../api/schemas';
import TextInput from './fields/TextInput';
import NumberInput from './fields/NumberInput';
import BoolToggle from './fields/BoolToggle';
import EnumSelect from './fields/EnumSelect';

export interface FieldListProps {
  descriptor: SchemaDescriptor;
  options: Record<string, string>;
  onChange: (field: string, value: string) => void;
}

export default function FieldList({ descriptor, options, onChange }: FieldListProps) {
  return (
    <div>
      {descriptor.fields.map(field => {
        const value = options[field.name] ?? '';
        const onFieldChange = (v: string) => onChange(field.name, v);
        switch (field.kind) {
          case 'int':
            return <NumberInput key={field.name} field={field} value={value} onChange={onFieldChange} />;
          case 'bool':
            return <BoolToggle key={field.name} field={field} value={value} onChange={onFieldChange} />;
          case 'enum':
            return <EnumSelect key={field.name} field={field} value={value} onChange={onFieldChange} />;
          case 'secret':
            // Stage 4a: secrets render as plain text inputs. Stage 4b
            // swaps in SecretInput with /reveal.
            return <TextInput key={field.name} field={field} value={value} onChange={onFieldChange} />;
          case 'string':
          default:
            return <TextInput key={field.name} field={field} value={value} onChange={onFieldChange} />;
        }
      })}
    </div>
  );
}
```

- [ ] **Step 2: Modify `web/src/components/Sidebar.tsx`**

Replace the file's content with:

```tsx
import styles from './Sidebar.module.css';
import type { SchemaDescriptor } from '../api/schemas';

export interface SidebarProps {
  instances: Array<{ key: string; type: string; enabled: boolean }>;
  selectedKey: string | null;
  descriptors: SchemaDescriptor[];
  dirtyKeys: Set<string>;
  onSelect: (key: string) => void;
  onNewInstance: () => void;
}

export default function Sidebar({
  instances,
  selectedKey,
  descriptors,
  dirtyKeys,
  onSelect,
  onNewInstance,
}: SidebarProps) {
  const displayNames = new Map(descriptors.map(d => [d.type, d.display_name]));
  return (
    <aside className={styles.sidebar}>
      <div className={styles.label}>Messaging Platforms</div>
      {instances.length === 0 && (
        <div className={styles.empty}>No instances configured.</div>
      )}
      {instances.map(inst => (
        <button
          key={inst.key}
          type="button"
          className={`${styles.item} ${inst.key === selectedKey ? styles.active : ''} ${!inst.enabled ? styles.dimmed : ''}`}
          onClick={() => onSelect(inst.key)}
        >
          <span className={styles.itemRow}>
            <span className={styles.itemKey}>{inst.key}</span>
            {dirtyKeys.has(inst.key) && <span className={styles.dirtyDot} title="Unsaved changes" />}
          </span>
          <span className={styles.itemType}>
            {displayNames.get(inst.type) ?? inst.type}
            {!inst.enabled && <span className={styles.offBadge}>off</span>}
          </span>
        </button>
      ))}
      <button
        type="button"
        className={styles.newBtn}
        onClick={onNewInstance}
      >
        + New instance
      </button>
    </aside>
  );
}
```

- [ ] **Step 3: Modify `web/src/components/Sidebar.module.css`**

Append these two rules at the end of the existing file:

```css
.itemRow {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}
.dirtyDot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--accent);
}
```

- [ ] **Step 4: type-check**

```bash
(cd /Users/ranwei/.config/superpowers/worktrees/hermind/feat-editor-fields-4a/web && pnpm type-check)
```

Expected: App.tsx's Sidebar errors are GONE now (we added `dirtyKeys`). Editor errors remain — those land in Task 5.

- [ ] **Step 5: Commit**

```bash
(cd /Users/ranwei/.config/superpowers/worktrees/hermind/feat-editor-fields-4a && git add web/src/components/FieldList.tsx web/src/components/Sidebar.tsx web/src/components/Sidebar.module.css)
```

Commit with exactly:

```
feat(web/components): FieldList dispatcher + Sidebar dirty dot

FieldList walks descriptor.fields and delegates to the kind-specific
control. secret currently falls through to TextInput — Stage 4b
swaps in SecretInput with /reveal. Sidebar gains a Set<string>
dirtyKeys prop so keys with unsaved changes show a small amber dot
next to their name.
```

---

## Task 5: Editor integration — enabled toggle, delete, FieldList slot

**Files:**
- Modify: `web/src/components/Editor.tsx`
- Modify: `web/src/components/Editor.module.css`

- [ ] **Step 1: Overwrite `web/src/components/Editor.tsx`**

```tsx
import styles from './Editor.module.css';
import type { PlatformInstance, SchemaDescriptor } from '../api/schemas';
import FieldList from './FieldList';

export interface EditorProps {
  selectedKey: string | null;
  instance: PlatformInstance | null;
  descriptor: SchemaDescriptor | null;
  onField: (field: string, value: string) => void;
  onToggleEnabled: (enabled: boolean) => void;
  onDelete: () => void;
}

export default function Editor({
  selectedKey,
  instance,
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
          options={instance.options ?? {}}
          onChange={onField}
        />
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
```

- [ ] **Step 2: Modify `web/src/components/Editor.module.css`**

Append the following at the end of the existing file:

```css
.headerSpacer {
  flex: 1;
}
.enabledToggle {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
}
.enabledToggle input[type='checkbox'] {
  width: 16px;
  height: 16px;
  accent-color: var(--accent);
  cursor: pointer;
}
.dangerZone {
  border-top: 1px solid var(--border);
  margin-top: 32px;
  padding-top: 16px;
}
.deleteBtn {
  appearance: none;
  background: transparent;
  color: var(--error);
  border: 1px solid var(--border);
  border-radius: 6px;
  padding: 6px 12px;
  font: inherit;
  font-size: 12px;
  cursor: pointer;
}
.deleteBtn:hover { background: var(--hover-tint); }
```

- [ ] **Step 3: type-check + build**

```bash
(cd /Users/ranwei/.config/superpowers/worktrees/hermind/feat-editor-fields-4a/web && pnpm type-check)
(cd /Users/ranwei/.config/superpowers/worktrees/hermind/feat-editor-fields-4a && make web)
```

Expected: both clean. All the App.tsx errors from Tasks 2–4 are now resolved because Editor takes the new props.

- [ ] **Step 4: Commit**

```bash
(cd /Users/ranwei/.config/superpowers/worktrees/hermind/feat-editor-fields-4a && git add web/src/components/Editor.tsx web/src/components/Editor.module.css api/webroot/)
```

Commit with exactly:

```
feat(web/editor): live FieldList + enabled toggle + delete

Editor now renders the descriptor's FieldList (four kind-aware
controls from components/fields/), an Enabled checkbox in the panel
header, and a Delete button in the danger zone at the bottom.
Unknown-type instances only expose the delete path. Confirmation
uses window.confirm — minimal viable UX; Stage 4b's NewInstanceDialog
can replace it with a custom modal if desired.
```

---

## Task 6: End-to-end smoke

No code changes — this is manual verification that Stage 4a's edit-save-apply cycle actually works.

- [ ] **Step 1: Full build + Go tests**

```bash
(cd /Users/ranwei/.config/superpowers/worktrees/hermind/feat-editor-fields-4a && make web && go test ./api/... ./cli/... ./gateway/... 2>&1 | tail -5)
```

Expected: clean, all Go tests green.

- [ ] **Step 2: Boot hermind web + curl-seed an instance**

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

(cd /Users/ranwei/.config/superpowers/worktrees/hermind/feat-editor-fields-4a && go build -o bin/hermind ./cmd/hermind)

HOME=/tmp/e2e-home /Users/ranwei/.config/superpowers/worktrees/hermind/feat-editor-fields-4a/bin/hermind web --addr=127.0.0.1:9119 --no-browser --exit-after=300s > /tmp/hermind-s4a-e2e.log 2>&1 &
sleep 2
TOK=$(grep "^token:" /tmp/hermind-s4a-e2e.log | awk '{print $2}')
echo "URL: http://127.0.0.1:9119/?t=$TOK"
```

Seed an instance via curl so the Editor has something to work with:

```bash
curl -s -X PUT -H "Authorization: Bearer $TOK" -H "Content-Type: application/json" \
  -d '{"config":{"gateway":{"platforms":{"tg_main":{"enabled":true,"type":"telegram","options":{"token":"seeded-token"}}}}}}' \
  http://127.0.0.1:9119/api/config
```

- [ ] **Step 3: Browser walkthrough**

Open the URL from step 2 in a browser. Verify:

1. Sidebar shows `tg_main` with no dirty dot. Select it.
2. Editor shows the telegram display name + one Bot Token text input (empty — GET redacted it).
3. Footer shows "All saved", Save buttons disabled.
4. Type `new-token-value` into the Bot Token field. Watch: Sidebar gains an amber dirty dot next to `tg_main`; TopBar and Footer both show "1 unsaved change"; Save and Save & Apply enable.
5. Click Save. Footer flashes green "Saved.", dirty indicators clear.
6. Verify the secret round-trip preserved the value:
   ```bash
   curl -s -X POST -H "Authorization: Bearer $TOK" -H "Content-Type: application/json" \
     -d '{"field":"token"}' http://127.0.0.1:9119/api/platforms/tg_main/reveal
   ```
   Expected: `{"value":"new-token-value"}`.
7. Toggle the Enabled checkbox off. Sidebar entry becomes dimmed with an "off" badge; Save button enables. Click Save. No errors.
8. Click Save & Apply. TopBar status dot pulses. Footer flashes green "Applied.". If the bogus token fails to connect, the restart still reports ok=true with per-key errors — confirm no red banner appears.
9. Edit the instance key via the URL: go to `http://127.0.0.1:9119/#tg_main`, refresh — the Editor should re-select `tg_main` automatically.
10. Click Delete in the danger zone, confirm the browser dialog. Sidebar empties, Editor shows the no-selection card.

Kill the hermind server when done:

```bash
kill %1 2>/dev/null || true
```

If any step breaks, stop and report. Don't ship a broken edit cycle.

- [ ] **Step 4: Commit history check**

```bash
(cd /Users/ranwei/.config/superpowers/worktrees/hermind/feat-editor-fields-4a && git log --oneline <stage-4a-base>..HEAD)
```

Expected: 5 commits (one per Task 1–5), no stray additions.

---

## Rollback

`git reset --hard <stage-4a-base>` on the feature branch. No backend state touched. The api/webroot/ bundle rolls back to the Stage 3 shell.

## Known scope cuts (Stage 4b follow-ups)

1. **SecretInput with /reveal.** Secret fields render as plain text inputs (value defaults to empty since GET redacts). Users can type new values and save; they cannot see what's stored. 4b adds the reveal button + masked password input.
2. **TestConnection button.** No UI trigger; use `curl POST /api/platforms/<key>/test` manually.
3. **NewInstanceDialog.** Sidebar "+ New instance" is still a log-only stub. Users seed instances via curl or the config.yaml.
4. **No inline validation messages** beyond HTML5 `type=number` / `required` affordances. Per-field error rendering (for the "`Required`" check + regex per key-name) lands with NewInstanceDialog.
