# Custom Tool & MCP Detail Renderers — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use gpowers:subagent-driven-development (recommended) or gpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the monolithic `renderToolDetail`/`renderMcpDetail` with an extensible registry of custom detail components, an enhanced generic fallback renderer, and a rich `browser_control` configuration panel as the first example.

**Architecture:** Front-end component-slot registries (`toolDetailRegistry`, `mcpDetailRegistry`) map tool names / MCP keys to React components. Unregistered items fall back to enhanced generic renderers (`ToolDetailFallback`, `McpDetailFallback`). All custom components share unified Props interfaces and write config through the existing `onSectionField` callback.

**Tech Stack:** React 18, TypeScript, CSS Modules, Vitest, Testing Library, Vite.

---

## File Structure

```
web/src/components/groups/skills/
├── SkillToolsConfigPage.tsx                          (refactored)
├── SkillToolsConfigPage.module.css                   (add missing .configRow styles)
├── detail-renderers/
│   ├── registry.ts                                   (new)
│   ├── types.ts                                      (new)
│   ├── ToolDetailFallback.tsx                        (new)
│   ├── ToolDetailFallback.module.css                 (new)
│   ├── McpDetailFallback.tsx                         (new)
│   ├── McpDetailFallback.module.css                  (new)
│   ├── browser/
│   │   ├── BrowserControlConfig.tsx                  (new)
│   │   └── BrowserControlConfig.module.css           (new)
│   ├── registry.test.ts                              (new)
│   ├── ToolDetailFallback.test.tsx                   (new)
│   └── browser/BrowserControlConfig.test.tsx         (new)
```

---

## Dependencies

All new files depend only on existing project code:
- `web/src/api/schemas.ts` — `ConfigField`, `ConfigPredicate`
- `web/src/api/client.ts` — `apiFetch`
- `web/src/components/fields/Switch.tsx` — toggle switch
- `web/src/components/groups/skills/SkillToolsConfigPage.module.css` — shared detail panel layout classes

No new npm packages are required.

---

### Task 1: Create Shared Types (`detail-renderers/types.ts`)

**Files:**
- **Create:** `web/src/components/groups/skills/detail-renderers/types.ts`

**Context:** Both custom components and fallback renderers must receive config data through a single, predictable interface. We pass the **entire** `config` tree (not just the tool's own subsection) because built-in tools often read/write other sections. Example: `browser_control` writes its API key to `browser_extension.api_key`.

- [ ] **Step 1: Write the types file**

```ts
import type { ConfigField } from '../../../api/schemas';

export interface ToolDetailProps {
  name: string;
  description?: string;
  toolset?: string;
  enabled: boolean;
  settings_schema?: ConfigField[];
  onToggle: (nextEnabled: boolean) => void;
  config?: Record<string, unknown>;
  onSectionField: (sectionKey: string, field: string, value: unknown) => void;
}

export interface McpDetailProps {
  key: string;
  command: string;
  enabled: boolean;
  onToggle: (nextEnabled: boolean) => void;
  serverConfig: Record<string, unknown>;
  onServerChange: (next: Record<string, unknown>) => void;
}
```

- [ ] **Step 2: Commit**

```bash
git add web/src/components/groups/skills/detail-renderers/types.ts
git commit -m "feat(detail-renderers): shared ToolDetailProps and McpDetailProps interfaces"
```

---

### Task 2: Create Registries (`detail-renderers/registry.ts`)

**Files:**
- **Create:** `web/src/components/groups/skills/detail-renderers/registry.ts`
- **Test:** `web/src/components/groups/skills/detail-renderers/registry.test.ts`

**Context:** Registration is explicit — every custom component is imported and listed. No magic strings, no dynamic imports. Unregistered names automatically fall back to the generic renderer.

- [ ] **Step 1: Write the registry file**

```ts
import type { ToolDetailProps, McpDetailProps } from './types';
import BrowserControlConfig from './browser/BrowserControlConfig';

export const toolDetailRegistry: Record<string, React.FC<ToolDetailProps>> = {
  browser_control: BrowserControlConfig,
};

export const mcpDetailRegistry: Record<string, React.FC<McpDetailProps>> = {};
```

- [ ] **Step 2: Write the failing test**

```ts
import { describe, it, expect } from 'vitest';
import { toolDetailRegistry, mcpDetailRegistry } from './registry';

describe('toolDetailRegistry', () => {
  it('contains browser_control mapped to a component', () => {
    expect(toolDetailRegistry['browser_control']).toBeDefined();
    expect(typeof toolDetailRegistry['browser_control']).toBe('function');
  });

  it('does not contain an unregistered tool', () => {
    expect(toolDetailRegistry['nonexistent_tool']).toBeUndefined();
  });
});

describe('mcpDetailRegistry', () => {
  it('is initially empty', () => {
    expect(Object.keys(mcpDetailRegistry)).toHaveLength(0);
  });
});
```

- [ ] **Step 3: Run test to verify it fails**

```bash
cd web && npx vitest run src/components/groups/skills/detail-renderers/registry.test.ts
```

Expected: FAIL — `BrowserControlConfig` does not exist yet.

- [ ] **Step 4: Create a stub `BrowserControlConfig.tsx` so the test passes**

```tsx
import type { ToolDetailProps } from '../types';

export default function BrowserControlConfig(_props: ToolDetailProps) {
  return <div data-testid="browser-control-config">Browser Control Config</div>;
}
```

- [ ] **Step 5: Run test to verify it passes**

```bash
cd web && npx vitest run src/components/groups/skills/detail-renderers/registry.test.ts
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add web/src/components/groups/skills/detail-renderers/registry.ts \
        web/src/components/groups/skills/detail-renderers/registry.test.ts \
        web/src/components/groups/skills/detail-renderers/browser/BrowserControlConfig.tsx
git commit -m "feat(detail-renderers): explicit component registries for tools and MCP"
```

---

### Task 3: Create Enhanced Tool Fallback Renderer (`detail-renderers/ToolDetailFallback.tsx`)

**Files:**
- **Create:** `web/src/components/groups/skills/detail-renderers/ToolDetailFallback.tsx`
- **Create:** `web/src/components/groups/skills/detail-renderers/ToolDetailFallback.module.css`
- **Test:** `web/src/components/groups/skills/detail-renderers/ToolDetailFallback.test.tsx`

**Context:** The current `renderSchemaFields` inside `SkillToolsConfigPage.tsx` only supports `bool`, `int`, `secret`, and plain `text`. It also lacks styling (`.configRow` is referenced but never defined in the CSS module). `ToolDetailFallback` fixes all of this: it supports every `ConfigFieldKind`, groups fields by an optional `group` property, evaluates `visible_when` predicates, and writes generic tool settings correctly through `tools.settings.<toolName>`.

**Key behavioral fix:** The current inline `setSettingValue` has `// Generic: would need to read current tools.settings, merge, and write back // For now unsupported`. `ToolDetailFallback` implements this correctly.

- [ ] **Step 1: Write the CSS module**

```css
/* ToolDetailFallback.module.css */

.configRow {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-4);
  padding: var(--space-3) 0;
  border-bottom: 1px solid var(--border);
}

.configRow:last-child {
  border-bottom: none;
}

.label {
  font-family: var(--font-mono);
  font-size: var(--fs-sm);
  font-weight: 600;
  color: var(--text);
}

.help {
  font-size: var(--fs-xs);
  color: var(--muted);
  margin-top: var(--space-1);
}

.groupTitle {
  font-family: var(--font-mono);
  font-size: var(--fs-sm);
  font-weight: 600;
  letter-spacing: 0.05em;
  text-transform: uppercase;
  margin: var(--space-5) 0 var(--space-3);
  padding-top: var(--space-4);
  border-top: 1px solid var(--border);
}

.groupTitle:first-child {
  margin-top: 0;
  padding-top: 0;
  border-top: none;
}

.noSettings {
  color: var(--muted);
  font-size: var(--fs-sm);
}

.input {
  background: var(--bg);
  border: 1px solid var(--border);
  border-radius: var(--r-sm);
  color: var(--text);
  font-family: var(--font-mono);
  font-size: var(--fs-sm);
  padding: var(--space-1) var(--space-2);
  width: 280px;
  transition: border-color var(--t-fast) ease-out, box-shadow var(--t-fast) ease-out;
}

.input:focus {
  outline: none;
  border-color: var(--accent);
  box-shadow: 0 0 0 2px var(--focus);
}

.textarea {
  background: var(--bg);
  border: 1px solid var(--border);
  border-radius: var(--r-sm);
  color: var(--text);
  font-family: var(--font-mono);
  font-size: var(--fs-sm);
  padding: var(--space-1) var(--space-2);
  width: 280px;
  height: 80px;
  resize: vertical;
  transition: border-color var(--t-fast) ease-out, box-shadow var(--t-fast) ease-out;
}

.textarea:focus {
  outline: none;
  border-color: var(--accent);
  box-shadow: 0 0 0 2px var(--focus);
}

.numberInput {
  background: var(--bg);
  border: 1px solid var(--border);
  border-radius: var(--r-sm);
  color: var(--text);
  font-family: var(--font-mono);
  font-size: var(--fs-sm);
  padding: var(--space-1) var(--space-2);
  width: 80px;
  text-align: right;
  font-variant-numeric: tabular-nums;
  transition: border-color var(--t-fast) ease-out, box-shadow var(--t-fast) ease-out;
}

.numberInput:focus {
  outline: none;
  border-color: var(--accent);
  box-shadow: 0 0 0 2px var(--focus);
}

.select {
  background: var(--bg);
  border: 1px solid var(--border);
  border-radius: var(--r-sm);
  color: var(--text);
  font-family: var(--font-mono);
  font-size: var(--fs-sm);
  padding: var(--space-1) var(--space-2);
  width: 280px;
  cursor: pointer;
  transition: border-color var(--t-fast) ease-out, box-shadow var(--t-fast) ease-out;
}

.select:focus {
  outline: none;
  border-color: var(--accent);
  box-shadow: 0 0 0 2px var(--focus);
}

.multiSelect {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
}

.checkboxRow {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  font-size: var(--fs-sm);
  cursor: pointer;
}

.checkboxRow input[type='checkbox'] {
  accent-color: var(--accent);
  cursor: pointer;
}
```

- [ ] **Step 2: Write the component**

```tsx
import { useMemo } from 'react';
import pageStyles from '../SkillToolsConfigPage.module.css';
import styles from './ToolDetailFallback.module.css';
import Switch from '../../fields/Switch';
import type { ConfigField, ConfigPredicate } from '../../../api/schemas';
import type { ToolDetailProps } from './types';

function asBool(v: unknown): boolean {
  if (typeof v === 'boolean') return v;
  if (typeof v === 'string') return v === 'true';
  return false;
}

function asString(v: unknown): string {
  if (v === undefined || v === null) return '';
  return typeof v === 'string' ? v : String(v);
}

function parseIntField(raw: string): number {
  if (raw === '') return 0;
  const n = Number(raw);
  return Number.isFinite(n) ? n : 0;
}

function parseFloatField(raw: string): number {
  if (raw === '') return 0;
  const n = Number(raw);
  return Number.isFinite(n) ? n : 0;
}

function getToolSettingValue(
  toolName: string,
  fieldName: string,
  config?: Record<string, unknown>,
): unknown {
  const settings = ((config?.tools as Record<string, unknown> | undefined)?.settings as
    | Record<string, Record<string, unknown>>
    | undefined);
  return settings?.[toolName]?.[fieldName];
}

function setToolSettingValue(
  toolName: string,
  fieldName: string,
  value: unknown,
  config: Record<string, unknown> | undefined,
  onSectionField?: (sectionKey: string, field: string, value: unknown) => void,
) {
  if (!onSectionField) return;
  const toolsCfg = (config?.tools as Record<string, unknown> | undefined) ?? {};
  const settings = (toolsCfg.settings as Record<string, Record<string, unknown>> | undefined) ?? {};
  const nextToolSettings = { ...(settings[toolName] ?? {}), [fieldName]: value };
  const nextSettings = { ...settings, [toolName]: nextToolSettings };
  onSectionField('tools', 'settings', nextSettings);
}

function evaluatePredicate(
  predicate: ConfigPredicate | undefined,
  values: Record<string, unknown>,
): boolean {
  if (!predicate) return true;
  const val = values[predicate.field];
  if (predicate.equals !== undefined) {
    return val === predicate.equals;
  }
  if (predicate.in !== undefined) {
    return predicate.in.includes(val);
  }
  return true;
}

function groupBy<T>(arr: T[], keyFn: (item: T) => string): Record<string, T[]> {
  const groups: Record<string, T[]> = {};
  for (const item of arr) {
    const key = keyFn(item);
    if (!groups[key]) groups[key] = [];
    groups[key].push(item);
  }
  return groups;
}

export default function ToolDetailFallback({
  name,
  description,
  toolset,
  enabled,
  settings_schema,
  onToggle,
  config,
  onSectionField,
}: ToolDetailProps) {
  const values = useMemo(() => {
    const v: Record<string, unknown> = {};
    if (settings_schema) {
      for (const field of settings_schema) {
        v[field.name] = getToolSettingValue(name, field.name, config);
      }
    }
    return v;
  }, [name, settings_schema, config]);

  const groups = useMemo(() => {
    if (!settings_schema) return {};
    return groupBy(settings_schema, (f) => f.group || 'General');
  }, [settings_schema]);

  const groupOrder = useMemo(() => {
    const keys = Object.keys(groups);
    const generalIdx = keys.indexOf('General');
    if (generalIdx > 0) {
      keys.splice(generalIdx, 1);
      keys.unshift('General');
    }
    return keys;
  }, [groups]);

  return (
    <div className={pageStyles.detailContent}>
      <div className={pageStyles.detailHeader}>
        <h2 className={pageStyles.detailTitle}>
          {name}
          {toolset && (
            <span style={{ fontSize: 'var(--fs-sm)', color: 'var(--muted)', marginLeft: 'var(--space-2)' }}>
              ({toolset})
            </span>
          )}
        </h2>
        <Switch checked={enabled} onChange={onToggle} ariaLabel={`Enable ${name}`} />
      </div>
      {description && <div className={pageStyles.detailDesc}>{description}</div>}
      <div className={pageStyles.configSection}>
        {settings_schema && settings_schema.length > 0 ? (
          groupOrder.map((groupName) => {
            const fields = groups[groupName].filter((f) => evaluatePredicate(f.visible_when, values));
            if (fields.length === 0) return null;
            return (
              <div key={groupName}>
                {groupName !== 'General' && <h3 className={styles.groupTitle}>{groupName}</h3>}
                {fields.map((field) => (
                  <SchemaFieldRow
                    key={field.name}
                    field={field}
                    toolName={name}
                    value={values[field.name]}
                    config={config}
                    onSectionField={onSectionField}
                  />
                ))}
              </div>
            );
          })
        ) : (
          <p className={styles.noSettings}>此工具暂无配置项。</p>
        )}
      </div>
    </div>
  );
}

function SchemaFieldRow({
  field,
  toolName,
  value,
  config,
  onSectionField,
}: {
  field: ConfigField;
  toolName: string;
  value: unknown;
  config: Record<string, unknown> | undefined;
  onSectionField?: (sectionKey: string, field: string, value: unknown) => void;
}) {
  const label = field.label || field.name;
  const help = field.help || '';

  const handleChange = (next: unknown) => {
    setToolSettingValue(toolName, field.name, next, config, onSectionField);
  };

  let input: React.ReactNode;

  if (field.kind === 'bool') {
    input = <Switch checked={asBool(value)} onChange={handleChange} ariaLabel={label} />;
  } else if (field.kind === 'int') {
    input = (
      <input
        type="number"
        className={styles.numberInput}
        value={asString(value)}
        onChange={(e) => handleChange(parseIntField(e.currentTarget.value))}
        aria-label={label}
      />
    );
  } else if (field.kind === 'float') {
    input = (
      <input
        type="number"
        step="any"
        className={styles.numberInput}
        value={asString(value)}
        onChange={(e) => handleChange(parseFloatField(e.currentTarget.value))}
        aria-label={label}
      />
    );
  } else if (field.kind === 'secret') {
    input = (
      <input
        type="password"
        className={styles.input}
        value={asString(value)}
        onChange={(e) => handleChange(e.currentTarget.value)}
        placeholder={field.default as string}
        aria-label={label}
      />
    );
  } else if (field.kind === 'enum') {
    const choices = field.enum ?? [];
    input = (
      <select
        className={styles.select}
        value={asString(value)}
        onChange={(e) => handleChange(e.currentTarget.value)}
        aria-label={label}
      >
        {!field.required && <option value="">—</option>}
        {choices.map((c) => (
          <option key={c} value={c}>
            {c}
          </option>
        ))}
      </select>
    );
  } else if (field.kind === 'text') {
    input = (
      <textarea
        className={styles.textarea}
        value={asString(value)}
        onChange={(e) => handleChange(e.currentTarget.value)}
        placeholder={field.default as string}
        aria-label={label}
      />
    );
  } else if (field.kind === 'multiselect') {
    const current = Array.isArray(value) ? value : [];
    const choices = field.enum ?? [];
    input = (
      <div className={styles.multiSelect}>
        {choices.map((c) => (
          <label key={c} className={styles.checkboxRow}>
            <input
              type="checkbox"
              checked={current.includes(c)}
              onChange={(e) => {
                const next = e.currentTarget.checked
                  ? [...current, c]
                  : current.filter((x: string) => x !== c);
                handleChange(next);
              }}
            />
            <span>{c}</span>
          </label>
        ))}
      </div>
    );
  } else {
    input = (
      <input
        type="text"
        className={styles.input}
        value={asString(value)}
        onChange={(e) => handleChange(e.currentTarget.value)}
        placeholder={field.default as string}
        aria-label={label}
      />
    );
  }

  return (
    <div className={styles.configRow}>
      <div>
        <div className={styles.label}>{label}</div>
        {help && <div className={styles.help}>{help}</div>}
      </div>
      {input}
    </div>
  );
}
```

- [ ] **Step 3: Write the test**

```tsx
import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import ToolDetailFallback from './ToolDetailFallback';
import type { ConfigField } from '../../../api/schemas';

const baseProps = {
  name: 'test_tool',
  enabled: true,
  onToggle: vi.fn(),
  onSectionField: vi.fn(),
};

describe('ToolDetailFallback', () => {
  it('renders name and toggle', () => {
    render(<ToolDetailFallback {...baseProps} description="A test tool" />);
    expect(screen.getByText('test_tool')).toBeInTheDocument();
    expect(screen.getByText('A test tool')).toBeInTheDocument();
    expect(screen.getByRole('switch')).toHaveAttribute('aria-checked', 'true');
  });

  it('dispatches onToggle when switch is clicked', () => {
    render(<ToolDetailFallback {...baseProps} />);
    fireEvent.click(screen.getByRole('switch'));
    expect(baseProps.onToggle).toHaveBeenCalledWith(false);
  });

  it('renders "no settings" when schema is empty', () => {
    render(<ToolDetailFallback {...baseProps} settings_schema={[]} />);
    expect(screen.getByText(/此工具暂无配置项/)).toBeInTheDocument();
  });

  it('renders string input and dispatches on change', () => {
    const schema: ConfigField[] = [
      { name: 'host', label: 'Host', kind: 'string', help: 'Server host' },
    ];
    render(<ToolDetailFallback {...baseProps} settings_schema={schema} config={{}} />);
    expect(screen.getByText('Host')).toBeInTheDocument();
    const input = screen.getByLabelText('Host');
    fireEvent.change(input, { target: { value: 'example.com' } });
    expect(baseProps.onSectionField).toHaveBeenCalledWith('tools', 'settings', {
      test_tool: { host: 'example.com' },
    });
  });

  it('renders bool switch and dispatches on change', () => {
    const schema: ConfigField[] = [
      { name: 'verbose', label: 'Verbose', kind: 'bool' },
    ];
    render(<ToolDetailFallback {...baseProps} settings_schema={schema} config={{}} />);
    const sw = screen.getByRole('switch', { name: 'Verbose' });
    fireEvent.click(sw);
    expect(baseProps.onSectionField).toHaveBeenCalledWith('tools', 'settings', {
      test_tool: { verbose: true },
    });
  });

  it('groups fields by group property', () => {
    const schema: ConfigField[] = [
      { name: 'a', label: 'A', kind: 'string', group: 'Network' },
      { name: 'b', label: 'B', kind: 'string', group: 'Auth' },
    ];
    render(<ToolDetailFallback {...baseProps} settings_schema={schema} config={{}} />);
    expect(screen.getByText('Network')).toBeInTheDocument();
    expect(screen.getByText('Auth')).toBeInTheDocument();
  });

  it('hides fields when visible_when predicate is not met', () => {
    const schema: ConfigField[] = [
      { name: 'use_proxy', label: 'Use proxy', kind: 'bool' },
      { name: 'proxy_url', label: 'Proxy URL', kind: 'string', visible_when: { field: 'use_proxy', equals: true } },
    ];
    render(
      <ToolDetailFallback
        {...baseProps}
        settings_schema={schema}
        config={{ tools: { settings: { test_tool: { use_proxy: false } } } }}
      />,
    );
    expect(screen.getByText('Use proxy')).toBeInTheDocument();
    expect(screen.queryByText('Proxy URL')).not.toBeInTheDocument();
  });

  it('shows hidden field when visible_when predicate is met', () => {
    const schema: ConfigField[] = [
      { name: 'use_proxy', label: 'Use proxy', kind: 'bool' },
      { name: 'proxy_url', label: 'Proxy URL', kind: 'string', visible_when: { field: 'use_proxy', equals: true } },
    ];
    render(
      <ToolDetailFallback
        {...baseProps}
        settings_schema={schema}
        config={{ tools: { settings: { test_tool: { use_proxy: true } } } }}
      />,
    );
    expect(screen.getByText('Proxy URL')).toBeInTheDocument();
  });
});
```

- [ ] **Step 4: Run tests**

```bash
cd web && npx vitest run src/components/groups/skills/detail-renderers/ToolDetailFallback.test.tsx
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/src/components/groups/skills/detail-renderers/ToolDetailFallback.tsx \
        web/src/components/groups/skills/detail-renderers/ToolDetailFallback.module.css \
        web/src/components/groups/skills/detail-renderers/ToolDetailFallback.test.tsx
git commit -m "feat(detail-renderers): enhanced generic ToolDetailFallback with grouping, visible_when, and all field kinds"
```

---

### Task 4: Create MCP Fallback Renderer (`detail-renderers/McpDetailFallback.tsx`)

**Files:**
- **Create:** `web/src/components/groups/skills/detail-renderers/McpDetailFallback.tsx`
- **Create:** `web/src/components/groups/skills/detail-renderers/McpDetailFallback.module.css`

**Context:** The current `renderMcpDetail` is nearly empty. The fallback shows the server name, command, enable switch, and a message directing users to the Advanced page.

- [ ] **Step 1: Write the component**

```tsx
import pageStyles from '../SkillToolsConfigPage.module.css';
import styles from './McpDetailFallback.module.css';
import Switch from '../../fields/Switch';
import type { McpDetailProps } from './types';

export default function McpDetailFallback({
  key,
  command,
  enabled,
  onToggle,
}: McpDetailProps) {
  return (
    <div className={pageStyles.detailContent}>
      <div className={pageStyles.detailHeader}>
        <h2 className={pageStyles.detailTitle}>{key}</h2>
        <Switch checked={enabled} onChange={onToggle} ariaLabel={`Enable ${key}`} />
      </div>
      {command && <div className={pageStyles.detailDesc}>{command}</div>}
      <div className={pageStyles.configSection}>
        <p className={styles.noSettings}>MCP 服务器配置请在 Advanced 页面管理。</p>
      </div>
    </div>
  );
}
```

- [ ] **Step 2: Write the CSS module**

```css
.noSettings {
  color: var(--muted);
  font-size: var(--fs-sm);
}
```

- [ ] **Step 3: Commit**

```bash
git add web/src/components/groups/skills/detail-renderers/McpDetailFallback.tsx \
        web/src/components/groups/skills/detail-renderers/McpDetailFallback.module.css
git commit -m "feat(detail-renderers): McpDetailFallback for unregistered MCP servers"
```

---

### Task 5: Implement `browser_control` Custom Panel (`detail-renderers/browser/BrowserControlConfig.tsx`)

**Files:**
- **Create:** `web/src/components/groups/skills/detail-renderers/browser/BrowserControlConfig.tsx`
- **Create:** `web/src/components/groups/skills/detail-renderers/browser/BrowserControlConfig.module.css`
- **Test:** `web/src/components/groups/skills/detail-renderers/browser/BrowserControlConfig.test.tsx`

**Context:** This is the primary UX improvement. Instead of two generic form fields, `browser_control` gets a dedicated panel with connection status, a test button, API key input with copy, and an install guide.

**Data flow:**
- API Key writes to `onSectionField('browser_extension', 'api_key', value)`.
- Enable toggle writes to `onSectionField('browser_extension', 'enabled', value)` (the parent `toggleTool` already handles this, but the custom component receives `onToggle` which maps to the same action).

Actually, looking at the current `SkillToolsConfigPage.tsx`, `toggleTool` writes to `tools.disabled`. But `browser_control` also has its own `enabled` field in `browser_extension`. The current `getSettingValue`/`setSettingValue` special-casing writes to `browser_extension` directly.

For the custom component, the `onToggle` prop will be wired by the parent to toggle the tool in `tools.disabled`. The API key will be wired to `browser_extension.api_key`. This matches the existing behavior.

- [ ] **Step 1: Write the CSS module**

```css
.statusCard {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  padding: var(--space-4);
  border-radius: var(--r-md);
  border: 1px solid var(--border);
  margin-bottom: var(--space-6);
}

.statusConnected {
  background: rgba(34, 197, 94, 0.08);
  border-color: rgba(34, 197, 94, 0.3);
}

.statusError {
  background: rgba(239, 68, 68, 0.08);
  border-color: rgba(239, 68, 68, 0.3);
}

.statusChecking {
  background: rgba(234, 179, 8, 0.08);
  border-color: rgba(234, 179, 8, 0.3);
}

.statusUnknown {
  background: var(--surface);
  border-color: var(--border);
}

.statusIcon {
  font-size: var(--fs-lg);
}

.statusTitle {
  font-family: var(--font-mono);
  font-size: var(--fs-sm);
  font-weight: 600;
}

.statusMeta {
  font-size: var(--fs-xs);
  color: var(--muted);
  margin-top: var(--space-1);
}

.configRow {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-4);
  padding: var(--space-3) 0;
}

.label {
  font-family: var(--font-mono);
  font-size: var(--fs-sm);
  font-weight: 600;
  color: var(--text);
}

.help {
  font-size: var(--fs-xs);
  color: var(--muted);
  margin-top: var(--space-1);
}

.keyInputRow {
  display: flex;
  align-items: center;
  gap: var(--space-2);
}

.keyInput {
  background: var(--bg);
  border: 1px solid var(--border);
  border-radius: var(--r-sm);
  color: var(--text);
  font-family: var(--font-mono);
  font-size: var(--fs-sm);
  padding: var(--space-1) var(--space-2);
  width: 260px;
  transition: border-color var(--t-fast) ease-out, box-shadow var(--t-fast) ease-out;
}

.keyInput:focus {
  outline: none;
  border-color: var(--accent);
  box-shadow: 0 0 0 2px var(--focus);
}

.iconBtn {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: var(--r-sm);
  padding: var(--space-1) var(--space-2);
  cursor: pointer;
  font-size: var(--fs-sm);
  transition: background var(--t-fast);
}

.iconBtn:hover {
  background: var(--hover-tint);
}

.actionBar {
  display: flex;
  gap: var(--space-3);
  margin-top: var(--space-5);
  padding-top: var(--space-4);
  border-top: 1px solid var(--border);
}

.primaryBtn {
  background: var(--accent);
  color: var(--accent-contrast, #fff);
  border: none;
  border-radius: var(--r-sm);
  padding: var(--space-2) var(--space-4);
  font-family: var(--font-mono);
  font-size: var(--fs-sm);
  cursor: pointer;
  transition: opacity var(--t-fast);
}

.primaryBtn:hover:not(:disabled) {
  opacity: 0.9;
}

.primaryBtn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.installGuide {
  margin-top: var(--space-5);
  padding: var(--space-4);
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: var(--r-md);
}

.installGuide h4 {
  font-family: var(--font-mono);
  font-size: var(--fs-sm);
  font-weight: 600;
  margin: 0 0 var(--space-3);
}

.installGuide ol {
  margin: 0;
  padding-left: var(--space-5);
  font-size: var(--fs-sm);
  color: var(--muted);
  line-height: 1.7;
}
```

- [ ] **Step 2: Write the component**

```tsx
import { useState, useCallback } from 'react';
import pageStyles from '../../SkillToolsConfigPage.module.css';
import styles from './BrowserControlConfig.module.css';
import Switch from '../../../fields/Switch';
import type { ToolDetailProps } from '../types';

function asString(v: unknown): string {
  if (v === undefined || v === null) return '';
  return typeof v === 'string' ? v : String(v);
}

type ConnectionStatus =
  | { state: 'unknown' }
  | { state: 'checking' }
  | { state: 'connected'; version: string }
  | { state: 'error'; message: string };

export default function BrowserControlConfig({
  name,
  description,
  toolset,
  enabled,
  onToggle,
  config,
  onSectionField,
}: ToolDetailProps) {
  const [status, setStatus] = useState<ConnectionStatus>({ state: 'unknown' });

  const apiKey = asString(
    (config?.browser_extension as Record<string, unknown> | undefined)?.api_key,
  );

  const handleTestConnection = useCallback(async () => {
    setStatus({ state: 'checking' });
    try {
      const resp = await fetch('/api/browser-extension/check');
      const data = (await resp.json()) as {
        connected?: boolean;
        version?: string;
        error?: string;
      };
      if (data.connected) {
        setStatus({ state: 'connected', version: data.version || 'unknown' });
      } else {
        setStatus({ state: 'error', message: data.error || 'Extension not responding' });
      }
    } catch (err) {
      setStatus({
        state: 'error',
        message: err instanceof Error ? err.message : 'Network error',
      });
    }
  }, []);

  const handleKeyChange = (value: string) => {
    onSectionField('browser_extension', 'api_key', value);
  };

  const handleCopyKey = async () => {
    if (!apiKey) return;
    try {
      await navigator.clipboard.writeText(apiKey);
    } catch {
      // ignore
    }
  };

  const renderStatusCard = () => {
    switch (status.state) {
      case 'connected':
        return (
          <div className={`${styles.statusCard} ${styles.statusConnected}`} data-testid="status-connected">
            <div className={styles.statusIcon}>🟢</div>
            <div>
              <div className={styles.statusTitle}>已连接</div>
              <div className={styles.statusMeta}>Extension version {status.version}</div>
            </div>
          </div>
        );
      case 'error':
        return (
          <div className={`${styles.statusCard} ${styles.statusError}`} data-testid="status-error">
            <div className={styles.statusIcon}>🔴</div>
            <div>
              <div className={styles.statusTitle}>未连接</div>
              <div className={styles.statusMeta}>{status.message}</div>
            </div>
          </div>
        );
      case 'checking':
        return (
          <div className={`${styles.statusCard} ${styles.statusChecking}`} data-testid="status-checking">
            <div className={styles.statusIcon}>⏳</div>
            <div>
              <div className={styles.statusTitle}>检测中...</div>
            </div>
          </div>
        );
      default:
        return (
          <div className={`${styles.statusCard} ${styles.statusUnknown}`} data-testid="status-unknown">
            <div className={styles.statusIcon}>⚪</div>
            <div>
              <div className={styles.statusTitle}>状态未知</div>
              <div className={styles.statusMeta}>点击「测试连接」检查扩展状态</div>
            </div>
          </div>
        );
    }
  };

  return (
    <div className={pageStyles.detailContent}>
      <div className={pageStyles.detailHeader}>
        <h2 className={pageStyles.detailTitle}>
          <span className={pageStyles.detailEmoji}>🌐</span>
          {name}
          {toolset && (
            <span style={{ fontSize: 'var(--fs-sm)', color: 'var(--muted)', marginLeft: 'var(--space-2)' }}>
              ({toolset})
            </span>
          )}
        </h2>
        <Switch checked={enabled} onChange={onToggle} ariaLabel={`Enable ${name}`} />
      </div>
      {description && <div className={pageStyles.detailDesc}>{description}</div>}

      {renderStatusCard()}

      <div className={pageStyles.configSection}>
        <h3>认证</h3>
        <div className={styles.configRow}>
          <div>
            <div className={styles.label}>Extension API Key</div>
            <div className={styles.help}>浏览器扩展通过此密钥与 Hermind 建立安全通信</div>
          </div>
          <div className={styles.keyInputRow}>
            <input
              type="password"
              className={styles.keyInput}
              value={apiKey}
              onChange={(e) => handleKeyChange(e.currentTarget.value)}
              placeholder="输入或生成 API Key"
              aria-label="Extension API Key"
              data-testid="api-key-input"
            />
            <button type="button" className={styles.iconBtn} onClick={handleCopyKey} title="复制">
              📋
            </button>
          </div>
        </div>
      </div>

      <div className={styles.actionBar}>
        <button
          type="button"
          className={styles.primaryBtn}
          onClick={handleTestConnection}
          disabled={status.state === 'checking'}
          data-testid="test-connection-btn"
        >
          {status.state === 'checking' ? '检测中...' : '测试连接'}
        </button>
      </div>

      {(status.state === 'error' || status.state === 'unknown') && (
        <div className={styles.installGuide} data-testid="install-guide">
          <h4>安装浏览器扩展</h4>
          <ol>
            <li>下载并解压浏览器扩展包</li>
            <li>打开 Chrome 扩展管理页面（chrome://extensions/）</li>
            <li>开启「开发者模式」</li>
            <li>点击「加载已解压的扩展程序」，选择解压后的文件夹</li>
            <li>在扩展设置中粘贴上方的 API Key</li>
          </ol>
        </div>
      )}
    </div>
  );
}
```

- [ ] **Step 3: Write the test**

```tsx
import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import BrowserControlConfig from './BrowserControlConfig';

const baseProps = {
  name: 'browser_control',
  description: 'Control the browser via extension',
  enabled: true,
  onToggle: vi.fn(),
  onSectionField: vi.fn(),
  config: {},
};

describe('BrowserControlConfig', () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('renders header with emoji and toggle', () => {
    render(<BrowserControlConfig {...baseProps} />);
    expect(screen.getByText('browser_control')).toBeInTheDocument();
    expect(screen.getByRole('switch')).toHaveAttribute('aria-checked', 'true');
  });

  it('dispatches onToggle when switch is clicked', () => {
    render(<BrowserControlConfig {...baseProps} />);
    fireEvent.click(screen.getByRole('switch'));
    expect(baseProps.onToggle).toHaveBeenCalledWith(false);
  });

  it('shows api key from browser_extension config', () => {
    render(
      <BrowserControlConfig
        {...baseProps}
        config={{ browser_extension: { api_key: 'secret123' } }}
      />,
    );
    const input = screen.getByTestId('api-key-input') as HTMLInputElement;
    expect(input.value).toBe('secret123');
  });

  it('dispatches onSectionField to browser_extension.api_key on change', () => {
    render(<BrowserControlConfig {...baseProps} />);
    const input = screen.getByTestId('api-key-input');
    fireEvent.change(input, { target: { value: 'newkey' } });
    expect(baseProps.onSectionField).toHaveBeenCalledWith('browser_extension', 'api_key', 'newkey');
  });

  it('shows unknown status initially', () => {
    render(<BrowserControlConfig {...baseProps} />);
    expect(screen.getByTestId('status-unknown')).toBeInTheDocument();
  });

  it('updates status on test connection success', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ connected: true, version: '1.2.0' }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    );
    render(<BrowserControlConfig {...baseProps} />);
    fireEvent.click(screen.getByTestId('test-connection-btn'));
    await waitFor(() => expect(screen.getByTestId('status-connected')).toBeInTheDocument());
    expect(screen.getByText('Extension version 1.2.0')).toBeInTheDocument();
  });

  it('updates status on test connection failure', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ connected: false, error: 'Extension not installed' }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    );
    render(<BrowserControlConfig {...baseProps} />);
    fireEvent.click(screen.getByTestId('test-connection-btn'));
    await waitFor(() => expect(screen.getByTestId('status-error')).toBeInTheDocument());
    expect(screen.getByText('Extension not installed')).toBeInTheDocument();
  });

  it('shows install guide when connection fails', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ connected: false, error: 'Extension not installed' }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    );
    render(<BrowserControlConfig {...baseProps} />);
    fireEvent.click(screen.getByTestId('test-connection-btn'));
    await waitFor(() => expect(screen.getByTestId('status-error')).toBeInTheDocument());
    expect(screen.getByTestId('install-guide')).toBeInTheDocument();
  });
});
```

- [ ] **Step 4: Run tests**

```bash
cd web && npx vitest run src/components/groups/skills/detail-renderers/browser/BrowserControlConfig.test.tsx
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/src/components/groups/skills/detail-renderers/browser/
git commit -m "feat(detail-renderers): custom BrowserControlConfig with status, test, and install guide"
```

---

### Task 6: Refactor `SkillToolsConfigPage.tsx` to Use Registry Dispatch

**Files:**
- **Modify:** `web/src/components/groups/skills/SkillToolsConfigPage.tsx`
- **Modify:** `web/src/components/groups/skills/SkillToolsConfigPage.module.css`
- **Test:** `web/src/components/groups/skills/SkillsSection.test.tsx` (update existing)

**Context:** This is the integration step. We remove the inline `renderToolDetail`, `renderMcpDetail`, `renderSchemaFields`, `getSettingValue`, and `setSettingValue` functions. The detail panel JSX now looks up the registry and falls back to the generic renderers. `renderSkillDetail` is preserved but its internal schema rendering is inlined and simplified (no more `browser_control` special case).

**Important:** The current `SkillToolsConfigPage.module.css` is missing `.configRow`, `.label`, and `.help` classes that `renderSchemaFields` references. We add them now so that the preserved `renderSkillSchemaFields` (used by skills) also looks correct.

- [ ] **Step 1: Add missing styles to `SkillToolsConfigPage.module.css`**

Append to `web/src/components/groups/skills/SkillToolsConfigPage.module.css`:

```css
/* Schema field rows used by the preserved skill detail renderer */
.configRow {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-4);
  padding: var(--space-3) 0;
  border-bottom: 1px solid var(--border);
}

.configRow:last-child {
  border-bottom: none;
}

.label {
  font-family: var(--font-mono);
  font-size: var(--fs-sm);
  font-weight: 600;
  color: var(--text);
}

.help {
  font-size: var(--fs-xs);
  color: var(--muted);
  margin-top: var(--space-1);
}
```

- [ ] **Step 2: Refactor `SkillToolsConfigPage.tsx`**

Replace the file content with the refactored version. The exact diff is large; here are the critical changes:

**Add imports at the top:**

```tsx
import { toolDetailRegistry, mcpDetailRegistry } from './detail-renderers/registry';
import ToolDetailFallback from './detail-renderers/ToolDetailFallback';
import McpDetailFallback from './detail-renderers/McpDetailFallback';
```

**Replace the detail panel JSX (around line 319-321):**

```tsx
{selected?.type === 'skill' && renderSkillDetail(selected.name, skillsState, disabledSkillSet, toggleSkill, props.config, props.onSectionField)}
{selected?.type === 'tool' && (() => {
  if (toolsState.status !== 'ok') return null;
  const tl = toolsState.data.find(t => t.name === selected.name);
  if (!tl) return null;
  const enabled = !toolsDisabledSet.has(tl.name);
  const Renderer = toolDetailRegistry[tl.name] ?? ToolDetailFallback;
  return (
    <Renderer
      name={tl.name}
      description={tl.description}
      toolset={tl.toolset}
      enabled={enabled}
      settings_schema={tl.settings_schema}
      onToggle={(next) => toggleTool(tl.name, next)}
      config={props.config}
      onSectionField={props.onSectionField}
    />
  );
})()}
{selected?.type === 'mcp' && (() => {
  const mcp = mcpList.find(m => m.key === selected.name);
  if (!mcp) return null;
  const Renderer = mcpDetailRegistry[mcp.key] ?? McpDetailFallback;
  return (
    <Renderer
      key={mcp.key}
      command={mcp.command}
      enabled={mcp.enabled}
      onToggle={(next) => toggleMcp(mcp.key, next)}
      serverConfig={mcpServers?.[mcp.key] ?? {}}
      onServerChange={(next) => props.onSectionField?.('mcp', 'servers', { ...mcpServers, [mcp.key]: next })}
    />
  );
})()}
```

**Remove these functions entirely:**
- `renderToolDetail`
- `renderMcpDetail`
- `getSettingValue`
- `setSettingValue`
- `renderSchemaFields`

**Keep `renderSkillDetail` but rename its internal schema helper to `renderSkillSchemaFields` and remove the `browser_control` branch.**

The simplified `renderSkillSchemaFields` should look like this (keep it inside `SkillToolsConfigPage.tsx` because skills are out of scope for custom renderers):

```tsx
function renderSkillSchemaFields(
  skillName: string,
  schema: ConfigField[],
  config?: Record<string, unknown>,
  onSectionField?: (sectionKey: string, field: string, value: unknown) => void,
) {
  return schema.map(field => {
    const settings = ((config?.tools as Record<string, unknown> | undefined)?.settings as
      | Record<string, Record<string, unknown>>
      | undefined);
    const value = settings?.[skillName]?.[field.name];
    const label = field.label || field.name;
    const help = field.help || '';

    let input: React.ReactNode;
    if (field.kind === 'bool') {
      input = (
        <Switch
          checked={asBool(value)}
          onChange={(next) => {
            if (!onSectionField) return;
            const toolsCfg = (config?.tools as Record<string, unknown> | undefined) ?? {};
            const s = (toolsCfg.settings as Record<string, Record<string, unknown>> | undefined) ?? {};
            const nextSkill = { ...(s[skillName] ?? {}), [field.name]: next };
            onSectionField('tools', 'settings', { ...s, [skillName]: nextSkill });
          }}
          ariaLabel={label}
        />
      );
    } else if (field.kind === 'int') {
      input = (
        <input
          type="number"
          className={styles.numberInput}
          value={asString(value)}
          onChange={(e) => {
            if (!onSectionField) return;
            const n = parseIntField(e.currentTarget.value);
            const toolsCfg = (config?.tools as Record<string, unknown> | undefined) ?? {};
            const s = (toolsCfg.settings as Record<string, Record<string, unknown>> | undefined) ?? {};
            const nextSkill = { ...(s[skillName] ?? {}), [field.name]: n };
            onSectionField('tools', 'settings', { ...s, [skillName]: nextSkill });
          }}
          aria-label={label}
        />
      );
    } else if (field.kind === 'secret') {
      input = (
        <input
          type="password"
          className={styles.numberInput}
          style={{ width: '280px', fontFamily: 'var(--font-mono)' }}
          value={asString(value)}
          onChange={(e) => {
            if (!onSectionField) return;
            const toolsCfg = (config?.tools as Record<string, unknown> | undefined) ?? {};
            const s = (toolsCfg.settings as Record<string, Record<string, unknown>> | undefined) ?? {};
            const nextSkill = { ...(s[skillName] ?? {}), [field.name]: e.currentTarget.value };
            onSectionField('tools', 'settings', { ...s, [skillName]: nextSkill });
          }}
          placeholder={field.default as string}
          aria-label={label}
        />
      );
    } else {
      input = (
        <input
          type="text"
          className={styles.numberInput}
          style={{ width: '280px', fontFamily: 'var(--font-mono)' }}
          value={asString(value)}
          onChange={(e) => {
            if (!onSectionField) return;
            const toolsCfg = (config?.tools as Record<string, unknown> | undefined) ?? {};
            const s = (toolsCfg.settings as Record<string, Record<string, unknown>> | undefined) ?? {};
            const nextSkill = { ...(s[skillName] ?? {}), [field.name]: e.currentTarget.value };
            onSectionField('tools', 'settings', { ...s, [skillName]: nextSkill });
          }}
          placeholder={field.default as string}
          aria-label={label}
        />
      );
    }

    return (
      <div key={field.name} className={styles.configRow}>
        <div>
          <div className={styles.label}>{label}</div>
          {help && <div className={styles.help}>{help}</div>}
        </div>
        {input}
      </div>
    );
  });
}
```

- [ ] **Step 3: Update existing integration tests**

In `web/src/components/groups/skills/SkillsSection.test.tsx`, add a test that verifies clicking `browser_control` renders the custom component:

```tsx
it('renders BrowserControlConfig for browser_control tool', async () => {
  mockToolsApi([
    { name: 'browser_control', description: 'Browser control', toolset: 'browser', enabled: true, settings_schema: [] },
  ]);
  render(
    <SkillsSection
      section={skillsSection}
      value={{}}
      originalValue={{}}
      onField={vi.fn()}
      onSectionField={vi.fn()}
      config={{}}
    />,
  );
  await waitFor(() => screen.getByText('browser_control'));
  fireEvent.click(screen.getByText('browser_control'));
  await waitFor(() => expect(screen.getByTestId('browser-control-config')).toBeInTheDocument());
});
```

Also add a test for an unregistered tool falling back to the generic renderer:

```tsx
it('renders fallback for unregistered tools', async () => {
  mockToolsApi([
    { name: 'web_search', description: 'Search', toolset: 'web', enabled: true, settings_schema: [{ name: 'api_key', label: 'API Key', kind: 'secret' }] },
  ]);
  render(
    <SkillsSection
      section={skillsSection}
      value={{}}
      originalValue={{}}
      onField={vi.fn()}
      onSectionField={vi.fn()}
      config={{}}
    />,
  );
  await waitFor(() => screen.getByText('web_search'));
  fireEvent.click(screen.getByText('web_search'));
  await waitFor(() => expect(screen.getByLabelText('API Key')).toBeInTheDocument());
});
```

- [ ] **Step 4: Run all tests in the skills group**

```bash
cd web && npx vitest run src/components/groups/skills/
```

Expected: PASS (or pre-existing failures only).

- [ ] **Step 5: Commit**

```bash
git add web/src/components/groups/skills/SkillToolsConfigPage.tsx \
        web/src/components/groups/skills/SkillToolsConfigPage.module.css \
        web/src/components/groups/skills/SkillsSection.test.tsx
git commit -m "refactor(skills): registry dispatch for tool and MCP detail panels"
```

---

### Task 7: Full Test Suite & Type Check

**Files:**
- All files in `web/src/components/groups/skills/detail-renderers/`
- `web/src/components/groups/skills/SkillToolsConfigPage.tsx`

- [ ] **Step 1: Run the full web test suite**

```bash
cd web && npm run test:ci
```

Expected: All new tests pass. Any pre-existing failures should be noted but not caused by this change.

- [ ] **Step 2: Run TypeScript type check**

```bash
cd web && npm run type-check
```

Expected: No type errors.

- [ ] **Step 3: Run linter**

```bash
cd web && npm run lint
```

Expected: No lint errors in modified/created files.

- [ ] **Step 4: Commit**

```bash
git commit --allow-empty -m "chore: verify tests, types, and lint pass"
```

---

## Self-Review

### 1. Spec Coverage

| Spec Requirement | Implementing Task |
|---|---|
| Component slot registry for tools | Task 2 (`registry.ts`) |
| Component slot registry for MCP | Task 2 (`registry.ts`) |
| Unified Props interfaces | Task 1 (`types.ts`) |
| `browser_control` custom UI with status/test/install guide | Task 5 (`BrowserControlConfig.tsx`) |
| Enhanced generic tool fallback with all field kinds | Task 3 (`ToolDetailFallback.tsx`) |
| Field grouping in fallback | Task 3 (`ToolDetailFallback.tsx`) |
| `visible_when` support in fallback | Task 3 (`ToolDetailFallback.tsx`) |
| MCP fallback with basic info | Task 4 (`McpDetailFallback.tsx`) |
| Refactor `SkillToolsConfigPage.tsx` to use registries | Task 6 |
| Fix generic tool settings write path (`tools.settings.<tool>`) | Task 3 (`setToolSettingValue`) |
| Add missing CSS classes to `SkillToolsConfigPage.module.css` | Task 6 |
| Unit tests for registry | Task 2 |
| Unit tests for `ToolDetailFallback` | Task 3 |
| Unit tests for `BrowserControlConfig` | Task 5 |
| Integration tests updated | Task 6 |

**No gaps identified.**

### 2. Placeholder Scan

- No "TBD", "TODO", "implement later", "fill in details" found.
- No vague "add appropriate error handling" — all error handling is explicit (try/catch in `BrowserControlConfig`, defensive null checks in fallback).
- No "Similar to Task N" shortcuts — each task contains its own complete code.
- All referenced types (`ConfigField`, `ConfigPredicate`, `ToolDetailProps`, `McpDetailProps`) are defined in earlier tasks.

### 3. Type Consistency

- `ToolDetailProps` uses `settings_schema?: ConfigField[]` — used consistently in `registry.ts`, `ToolDetailFallback.tsx`, and `BrowserControlConfig.tsx`.
- `onSectionField` signature is `(sectionKey: string, field: string, value: unknown) => void` everywhere.
- `onToggle` signature is `(nextEnabled: boolean) => void` everywhere.
- `McpDetailProps` uses `key` (not `name`) to avoid shadowing the reserved keyword in plain JS, but the prop is named `key` in all usages.

No type mismatches found.
