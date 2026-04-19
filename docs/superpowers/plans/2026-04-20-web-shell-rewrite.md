# Web Shell Rewrite Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the IM-only Vite/React web UI with a seven-group shell (Models, Gateway, Memory, Skills, Runtime, Advanced, Observability). Gateway wraps existing IM editor unchanged; other six groups render a Coming-soon placeholder. Hash URL evolves to `#<group>/<sub>` with legacy `#<platform-key>` auto-migration.

**Architecture:** Pure helpers first (`shell/groups.ts`, `shell/hash.ts`, `shell/summaries.tsx`), then state extensions (shell slice + dirty selectors + localStorage persistence), then presentational components (TopBar, GroupSection, ComingSoonPanel, EmptyState), then Gateway group components (GatewaySidebar, GatewayApplyButton, GatewayPanel), then Sidebar + ContentPanel, finally App.tsx integration + Footer cleanup + smoke docs.

**Tech Stack:** TypeScript 5, React 18, Vite 5, Vitest 2, jsdom 25, @testing-library/react (added in Task 1), pnpm 10. No new runtime dependencies.

**Spec:** `docs/superpowers/specs/2026-04-20-web-shell-rewrite-design.md`.

**Branch:** Work on `main` directly (this repo is trunk-based; see prior IM stages).

**Always run from repo root** unless a task says otherwise.

**Running tests:** `cd web && pnpm test` (one-shot) or `cd web && pnpm test:watch` (watch mode).

---

## Task 1: Add @testing-library dev dependencies

**Files:**
- Modify: `web/package.json`
- Modify: `web/pnpm-lock.yaml` (auto via pnpm)
- Create: `web/src/test/setup.ts`
- Modify: `web/vitest.config.ts`

**Why:** Current tests only cover pure TS (state, schemas). Adding 10 new UI components means we need `@testing-library/react` for rendering + `@testing-library/user-event` for interactions + `@testing-library/jest-dom` for readable matchers.

- [ ] **Step 1: Install dev dependencies**

Run from the repo root:

```bash
cd web && pnpm add -D @testing-library/react@^16.1.0 @testing-library/user-event@^14.5.2 @testing-library/jest-dom@^6.6.3
```

Expected: `pnpm-lock.yaml` updated, three packages added under `devDependencies`.

- [ ] **Step 2: Create test setup file**

Create `web/src/test/setup.ts`:

```ts
import '@testing-library/jest-dom/vitest';
import { afterEach } from 'vitest';
import { cleanup } from '@testing-library/react';

afterEach(() => {
  cleanup();
});
```

- [ ] **Step 3: Wire setup into vitest.config.ts**

Read current file, then add a `setupFiles` entry. Example final state (adjust to match whatever exists):

```ts
import { defineConfig } from 'vitest/config';
import react from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [react()],
  test: {
    environment: 'jsdom',
    setupFiles: ['./src/test/setup.ts'],
    globals: false,
  },
});
```

- [ ] **Step 4: Smoke-test the setup**

Create a throwaway test `web/src/test/setup.test.ts`:

```ts
import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { createElement } from 'react';

describe('testing-library setup', () => {
  it('renders a DOM element', () => {
    render(createElement('h1', null, 'hello'));
    expect(screen.getByText('hello')).toBeInTheDocument();
  });
});
```

Run: `cd web && pnpm test -- setup`
Expected: PASS (1 test).

- [ ] **Step 5: Remove the smoke file and commit**

```bash
rm web/src/test/setup.test.ts
cd .. && git add web/package.json web/pnpm-lock.yaml web/src/test/setup.ts web/vitest.config.ts
git commit -m "chore(web): add @testing-library for component tests"
```

---

## Task 2: Group metadata (shell/groups.ts)

**Files:**
- Create: `web/src/shell/groups.ts`
- Create: `web/src/shell/groups.test.ts`

**Why:** Single source of truth for group ids, labels, planned stages, and config-field mappings. Sidebar, ContentPanel, and summaries all read from this table.

- [ ] **Step 1: Write the failing test**

Create `web/src/shell/groups.test.ts`:

```ts
import { describe, it, expect } from 'vitest';
import { GROUP_IDS, GROUPS, findGroup, type GroupId } from './groups';

describe('GROUPS table', () => {
  it('contains exactly 7 groups', () => {
    expect(GROUPS).toHaveLength(7);
  });

  it('has the expected ids in fixed display order', () => {
    const ids: GroupId[] = GROUPS.map(g => g.id);
    expect(ids).toEqual([
      'models',
      'gateway',
      'memory',
      'skills',
      'runtime',
      'advanced',
      'observability',
    ]);
  });

  it('has no duplicate ids', () => {
    const ids = GROUPS.map(g => g.id);
    expect(new Set(ids).size).toBe(ids.length);
  });

  it('every group has label, plannedStage, configKeys, description, bullets', () => {
    for (const g of GROUPS) {
      expect(g.label).toBeTruthy();
      expect(g.plannedStage).toBeTruthy();
      expect(Array.isArray(g.configKeys)).toBe(true);
      expect(g.configKeys.length).toBeGreaterThan(0);
      expect(g.description).toBeTruthy();
      expect(Array.isArray(g.bullets)).toBe(true);
      expect(g.bullets.length).toBeGreaterThan(0);
    }
  });

  it('GROUP_IDS set matches GROUPS entries', () => {
    expect(Array.from(GROUP_IDS).sort()).toEqual(GROUPS.map(g => g.id).sort());
  });
});

describe('findGroup', () => {
  it('returns the matching group for a known id', () => {
    expect(findGroup('gateway').label).toBe('Gateway');
  });

  it('throws for an unknown id', () => {
    expect(() => findGroup('bogus' as GroupId)).toThrow();
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd web && pnpm test -- groups
```

Expected: FAIL — module `./groups` not found.

- [ ] **Step 3: Implement groups.ts**

Create `web/src/shell/groups.ts`:

```ts
export type GroupId =
  | 'models'
  | 'gateway'
  | 'memory'
  | 'skills'
  | 'runtime'
  | 'advanced'
  | 'observability';

export interface GroupDef {
  id: GroupId;
  label: string;
  plannedStage: string;
  configKeys: readonly string[];
  description: string;
  bullets: readonly string[];
}

export const GROUPS: readonly GroupDef[] = [
  {
    id: 'models',
    label: 'Models',
    plannedStage: '3 & 4',
    configKeys: ['model', 'providers', 'fallback_providers'],
    description: 'Default model and provider configuration.',
    bullets: [
      'Default model selection',
      'Provider configs (OpenAI, Anthropic, local, …)',
      'Fallback providers',
      'Per-provider fetch-models button',
    ],
  },
  {
    id: 'gateway',
    label: 'Gateway',
    plannedStage: 'done',
    configKeys: ['gateway'],
    description: 'Messaging platform instances (Feishu, DingTalk, WeChat, …).',
    bullets: ['Per-platform instance configuration', 'Secret handling', 'Connection test'],
  },
  {
    id: 'memory',
    label: 'Memory',
    plannedStage: '5',
    configKeys: ['memory'],
    description: 'Long-term memory backend configuration.',
    bullets: [
      'Backend selection (RetainDB, OpenViking, Byterover, Honcho, Mem0, …)',
      'Per-backend credentials and endpoints',
      'Enable/disable toggle',
    ],
  },
  {
    id: 'skills',
    label: 'Skills',
    plannedStage: '6',
    configKeys: ['skills'],
    description: 'Skill enable/disable and per-platform overrides.',
    bullets: [
      'Global disabled list',
      'Per-platform overrides (CLI, gateway, cron)',
      'Auto-discovered skill catalog',
    ],
  },
  {
    id: 'runtime',
    label: 'Runtime',
    plannedStage: '3',
    configKeys: ['agent', 'auxiliary', 'terminal', 'storage'],
    description: 'Agent prompt, auxiliary config, terminal, and storage.',
    bullets: [
      'Agent system prompt',
      'Auxiliary model',
      'Terminal config',
      'Storage backend',
    ],
  },
  {
    id: 'advanced',
    label: 'Advanced',
    plannedStage: '7',
    configKeys: ['mcp', 'browser', 'cron'],
    description: 'MCP servers, browser automation, and scheduled jobs.',
    bullets: ['MCP server list', 'Browser (Browserbase / Camofox) config', 'Cron jobs'],
  },
  {
    id: 'observability',
    label: 'Observability',
    plannedStage: '3',
    configKeys: ['logging', 'metrics', 'tracing'],
    description: 'Logging level, metrics, and tracing.',
    bullets: ['Logging level and output', 'Metrics exporter', 'Tracing exporter'],
  },
] as const;

export const GROUP_IDS: ReadonlySet<GroupId> = new Set(GROUPS.map(g => g.id));

export function findGroup(id: GroupId): GroupDef {
  const g = GROUPS.find(x => x.id === id);
  if (!g) throw new Error(`unknown group id: ${id}`);
  return g;
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd web && pnpm test -- groups
```

Expected: PASS (6 tests).

- [ ] **Step 5: Commit**

```bash
git add web/src/shell/groups.ts web/src/shell/groups.test.ts
git commit -m "feat(web/shell): GROUPS metadata table + findGroup helper"
```

---

## Task 3: Hash routing (shell/hash.ts)

**Files:**
- Create: `web/src/shell/hash.ts`
- Create: `web/src/shell/hash.test.ts`

**Why:** All hash parsing / stringifying / legacy migration lives here so state.ts and App.tsx don't sprinkle string manipulation.

- [ ] **Step 1: Write the failing test**

Create `web/src/shell/hash.test.ts`:

```ts
import { describe, it, expect } from 'vitest';
import { parseHash, stringifyHash, migrateLegacyHash } from './hash';

describe('parseHash', () => {
  it('returns null/null for an empty hash', () => {
    expect(parseHash('')).toEqual({ group: null, sub: null });
    expect(parseHash('#')).toEqual({ group: null, sub: null });
  });

  it('parses #<group> with no sub', () => {
    expect(parseHash('#models')).toEqual({ group: 'models', sub: null });
    expect(parseHash('#gateway')).toEqual({ group: 'gateway', sub: null });
  });

  it('parses #<group>/<sub>', () => {
    expect(parseHash('#gateway/feishu-bot-main')).toEqual({
      group: 'gateway',
      sub: 'feishu-bot-main',
    });
  });

  it('decodes percent-encoded sub keys', () => {
    expect(parseHash('#gateway/' + encodeURIComponent('key with/special chars'))).toEqual({
      group: 'gateway',
      sub: 'key with/special chars',
    });
  });

  it('returns null/null for unknown group names', () => {
    expect(parseHash('#bogus')).toEqual({ group: null, sub: null });
    expect(parseHash('#bogus/whatever')).toEqual({ group: null, sub: null });
  });

  it('tolerates malformed percent encoding by passing through', () => {
    // '%' alone is invalid percent-encoding; decodeURIComponent throws.
    expect(parseHash('#gateway/%-raw')).toEqual({ group: 'gateway', sub: '%-raw' });
  });
});

describe('stringifyHash', () => {
  it('returns empty for null group', () => {
    expect(stringifyHash(null, null)).toBe('');
    expect(stringifyHash(null, 'ignored')).toBe('');
  });

  it('builds #<group> when sub is null', () => {
    expect(stringifyHash('models', null)).toBe('#models');
  });

  it('builds #<group>/<sub> and encodes the sub', () => {
    expect(stringifyHash('gateway', 'feishu-bot-main')).toBe('#gateway/feishu-bot-main');
    expect(stringifyHash('gateway', 'key with/special')).toBe(
      '#gateway/' + encodeURIComponent('key with/special'),
    );
  });

  it('round-trips with parseHash', () => {
    const h = stringifyHash('gateway', 'weird/key with%');
    expect(parseHash(h)).toEqual({ group: 'gateway', sub: 'weird/key with%' });
  });
});

describe('migrateLegacyHash', () => {
  const platforms = ['feishu-bot-main', 'dingtalk-alerts'];

  it('returns null when hash is empty', () => {
    expect(migrateLegacyHash('', platforms)).toBeNull();
  });

  it('returns null when hash already matches a known group', () => {
    expect(migrateLegacyHash('#gateway/anything', platforms)).toBeNull();
    expect(migrateLegacyHash('#models', platforms)).toBeNull();
  });

  it('migrates a bare legacy key that exists in platforms', () => {
    expect(migrateLegacyHash('#feishu-bot-main', platforms)).toBe('#gateway/feishu-bot-main');
  });

  it('migrates percent-encoded legacy keys', () => {
    const legacy = '#' + encodeURIComponent('feishu-bot-main');
    expect(migrateLegacyHash(legacy, platforms)).toBe('#gateway/feishu-bot-main');
  });

  it('returns null for unknown bare keys', () => {
    expect(migrateLegacyHash('#never-existed', platforms)).toBeNull();
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd web && pnpm test -- hash
```

Expected: FAIL — module `./hash` not found.

- [ ] **Step 3: Implement hash.ts**

Create `web/src/shell/hash.ts`:

```ts
import { GROUP_IDS, type GroupId } from './groups';

export interface ParsedHash {
  group: GroupId | null;
  sub: string | null;
}

function safeDecode(raw: string): string {
  try {
    return decodeURIComponent(raw);
  } catch {
    return raw;
  }
}

export function parseHash(hash: string): ParsedHash {
  const raw = hash.replace(/^#/, '');
  if (!raw) return { group: null, sub: null };
  const slash = raw.indexOf('/');
  const groupPart = slash === -1 ? raw : raw.substring(0, slash);
  if (!GROUP_IDS.has(groupPart as GroupId)) return { group: null, sub: null };
  const subPart = slash === -1 ? null : raw.substring(slash + 1);
  const sub = subPart ? safeDecode(subPart) : null;
  return { group: groupPart as GroupId, sub };
}

export function stringifyHash(group: GroupId | null, sub: string | null): string {
  if (!group) return '';
  if (!sub) return '#' + group;
  return '#' + group + '/' + encodeURIComponent(sub);
}

export function migrateLegacyHash(
  hash: string,
  knownPlatformKeys: readonly string[],
): string | null {
  const raw = hash.replace(/^#/, '');
  if (!raw) return null;
  const slash = raw.indexOf('/');
  const groupPart = slash === -1 ? raw : raw.substring(0, slash);
  if (GROUP_IDS.has(groupPart as GroupId)) return null;
  const legacyKey = safeDecode(raw);
  if (knownPlatformKeys.includes(legacyKey)) {
    return stringifyHash('gateway', legacyKey);
  }
  return null;
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd web && pnpm test -- hash
```

Expected: PASS (all parseHash / stringifyHash / migrateLegacyHash cases).

- [ ] **Step 5: Commit**

```bash
git add web/src/shell/hash.ts web/src/shell/hash.test.ts
git commit -m "feat(web/shell): hash parse/stringify + legacy #<key> migration"
```

---

## Task 4: Per-group summaries (shell/summaries.tsx)

**Files:**
- Create: `web/src/shell/summaries.tsx`
- Create: `web/src/shell/summaries.test.tsx`

**Why:** Each placeholder group shows a small read-only preview of the relevant config slice. Summaries are hardcoded per group (not schema-driven; that's stage 2's job).

- [ ] **Step 1: Write the failing test**

Create `web/src/shell/summaries.test.tsx`:

```tsx
import { describe, it, expect } from 'vitest';
import { render } from '@testing-library/react';
import type { Config } from '../api/schemas';
import { summaryFor } from './summaries';

const empty: Config = {};

describe('summaryFor', () => {
  it('returns null for the gateway group', () => {
    expect(summaryFor('gateway', empty)).toBeNull();
  });

  it('renders a models summary with default model + provider counts', () => {
    const cfg = {
      model: 'claude-opus-4-7',
      providers: { openai: {}, anthropic: {} },
      fallback_providers: [{ type: 'openai' }],
    } as unknown as Config;
    const { container } = render(<>{summaryFor('models', cfg)}</>);
    expect(container.textContent).toContain('claude-opus-4-7');
    expect(container.textContent).toContain('2'); // providers count
    expect(container.textContent).toContain('1'); // fallback count
  });

  it('renders a memory summary with backend + enabled flag', () => {
    const cfg = {
      memory: { enabled: true, retain_db: { endpoint: 'x' } },
    } as unknown as Config;
    const { container } = render(<>{summaryFor('memory', cfg)}</>);
    expect(container.textContent).toContain('retain_db');
    expect(container.textContent).toMatch(/yes/i);
  });

  it('renders a skills summary with disabled + override counts', () => {
    const cfg = {
      skills: {
        disabled: ['a', 'b', 'c'],
        platform_disabled: { cli: ['x'] },
      },
    } as unknown as Config;
    const { container } = render(<>{summaryFor('skills', cfg)}</>);
    expect(container.textContent).toContain('3');
    expect(container.textContent).toContain('1');
  });

  it('renders a runtime summary with storage kind + agent prompt presence', () => {
    const cfg = {
      agent: { prompt: 'custom prompt' },
      storage: { kind: 'sqlite' },
    } as unknown as Config;
    const { container } = render(<>{summaryFor('runtime', cfg)}</>);
    expect(container.textContent).toContain('sqlite');
    expect(container.textContent).toContain('custom');
  });

  it('renders an advanced summary with MCP + cron counts', () => {
    const cfg = {
      mcp: { servers: { a: {}, b: {} } },
      cron: { jobs: [{ name: 'x' }, { name: 'y' }, { name: 'z' }] },
    } as unknown as Config;
    const { container } = render(<>{summaryFor('advanced', cfg)}</>);
    expect(container.textContent).toContain('2');
    expect(container.textContent).toContain('3');
  });

  it('renders an observability summary with log level + enabled flags', () => {
    const cfg = {
      logging: { level: 'debug' },
      metrics: { enabled: true },
      tracing: { enabled: false },
    } as unknown as Config;
    const { container } = render(<>{summaryFor('observability', cfg)}</>);
    expect(container.textContent).toContain('debug');
    expect(container.textContent).toMatch(/on/i);
    expect(container.textContent).toMatch(/off/i);
  });

  it('renders gracefully on an empty config for every placeholder group', () => {
    for (const id of ['models', 'memory', 'skills', 'runtime', 'advanced', 'observability'] as const) {
      const { container } = render(<>{summaryFor(id, empty)}</>);
      expect(container.textContent).toBeTruthy();
    }
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd web && pnpm test -- summaries
```

Expected: FAIL — module `./summaries` not found.

- [ ] **Step 3: Implement summaries.tsx**

Create `web/src/shell/summaries.tsx`:

```tsx
import type { ReactNode } from 'react';
import type { Config } from '../api/schemas';
import type { GroupId } from './groups';

type AnyRec = Record<string, unknown>;

function asRecord(v: unknown): AnyRec {
  return v && typeof v === 'object' ? (v as AnyRec) : {};
}

function countKeys(v: unknown): number {
  return Object.keys(asRecord(v)).length;
}

function SummaryRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="summary-row">
      <span className="summary-label">{label}</span>
      <span className="summary-value">{value}</span>
    </div>
  );
}

function modelsSummary(cfg: Config): ReactNode {
  const c = cfg as unknown as AnyRec;
  const model = typeof c.model === 'string' ? c.model : '(unset)';
  const providers = countKeys(c.providers);
  const fallbacks = Array.isArray(c.fallback_providers) ? c.fallback_providers.length : 0;
  return (
    <>
      <SummaryRow label="Default model" value={model} />
      <SummaryRow label="Providers" value={`${providers} configured`} />
      <SummaryRow label="Fallbacks" value={`${fallbacks} configured`} />
    </>
  );
}

function memorySummary(cfg: Config): ReactNode {
  const m = asRecord((cfg as unknown as AnyRec).memory);
  const enabled = m.enabled === true;
  const backend = Object.keys(m).find(k => k !== 'enabled') ?? '(none)';
  return (
    <>
      <SummaryRow label="Backend" value={backend} />
      <SummaryRow label="Enabled" value={enabled ? 'yes' : 'no'} />
    </>
  );
}

function skillsSummary(cfg: Config): ReactNode {
  const s = asRecord((cfg as unknown as AnyRec).skills);
  const disabled = Array.isArray(s.disabled) ? s.disabled.length : 0;
  const overrides = countKeys(s.platform_disabled);
  return (
    <>
      <SummaryRow label="Globally disabled" value={String(disabled)} />
      <SummaryRow label="Platform overrides" value={String(overrides)} />
    </>
  );
}

function runtimeSummary(cfg: Config): ReactNode {
  const c = cfg as unknown as AnyRec;
  const agent = asRecord(c.agent);
  const storage = asRecord(c.storage);
  const hasPrompt = typeof agent.prompt === 'string' && agent.prompt.length > 0;
  const storageKind = typeof storage.kind === 'string' ? storage.kind : '(default)';
  return (
    <>
      <SummaryRow label="Agent prompt" value={hasPrompt ? 'custom' : 'default'} />
      <SummaryRow label="Storage" value={storageKind} />
    </>
  );
}

function advancedSummary(cfg: Config): ReactNode {
  const c = cfg as unknown as AnyRec;
  const mcp = asRecord(c.mcp);
  const cron = asRecord(c.cron);
  const mcpServers = countKeys(mcp.servers);
  const cronJobs = Array.isArray(cron.jobs) ? cron.jobs.length : 0;
  return (
    <>
      <SummaryRow label="MCP servers" value={String(mcpServers)} />
      <SummaryRow label="Cron jobs" value={String(cronJobs)} />
    </>
  );
}

function observabilitySummary(cfg: Config): ReactNode {
  const c = cfg as unknown as AnyRec;
  const log = asRecord(c.logging);
  const met = asRecord(c.metrics);
  const trc = asRecord(c.tracing);
  const level = typeof log.level === 'string' ? log.level : '(default)';
  return (
    <>
      <SummaryRow label="Log level" value={level} />
      <SummaryRow label="Metrics" value={met.enabled === true ? 'on' : 'off'} />
      <SummaryRow label="Tracing" value={trc.enabled === true ? 'on' : 'off'} />
    </>
  );
}

const FNS: Record<Exclude<GroupId, 'gateway'>, (cfg: Config) => ReactNode> = {
  models: modelsSummary,
  memory: memorySummary,
  skills: skillsSummary,
  runtime: runtimeSummary,
  advanced: advancedSummary,
  observability: observabilitySummary,
};

export function summaryFor(group: GroupId, cfg: Config): ReactNode {
  if (group === 'gateway') return null;
  return FNS[group](cfg);
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd web && pnpm test -- summaries
```

Expected: PASS (all summaryFor cases).

- [ ] **Step 5: Commit**

```bash
git add web/src/shell/summaries.tsx web/src/shell/summaries.test.tsx
git commit -m "feat(web/shell): per-group config summary renderers"
```

---

## Task 5: State shell slice + actions + selectors

**Files:**
- Modify: `web/src/state.ts`
- Modify: `web/src/state.test.ts`

**Why:** The AppState gains a `shell` sub-state (activeGroup, activeSubKey, expandedGroups). Actions update the sub-state; selectors aggregate dirty info across groups.

- [ ] **Step 1: Write the failing tests (append to state.test.ts)**

Append these test blocks to `web/src/state.test.ts`:

```ts
import {
  dirtyGroups,
  groupDirty,
  reducer,
  totalDirtyCount,
  type AppState,
} from './state';
import type { GroupId } from './shell/groups';

function boot(state: AppState = initialState): AppState {
  return reducer(state, {
    type: 'boot/loaded',
    descriptors: emptyDescriptors,
    config: cfg({ a: { type: 't', enabled: true } }),
  });
}

describe('shell slice — initial state', () => {
  it('starts with activeGroup=null, activeSubKey=null, expandedGroups={gateway}', () => {
    expect(initialState.shell.activeGroup).toBeNull();
    expect(initialState.shell.activeSubKey).toBeNull();
    expect(initialState.shell.expandedGroups.has('gateway')).toBe(true);
    expect(initialState.shell.expandedGroups.size).toBe(1);
  });
});

describe('reducer — shell/selectGroup', () => {
  it('sets activeGroup and clears activeSubKey', () => {
    const s0 = boot();
    const s1 = reducer(s0, { type: 'shell/selectGroup', group: 'models' });
    expect(s1.shell.activeGroup).toBe('models');
    expect(s1.shell.activeSubKey).toBeNull();
  });

  it('null group clears both', () => {
    const s0 = reducer(boot(), { type: 'shell/selectGroup', group: 'gateway' });
    const s1 = reducer(s0, { type: 'shell/selectSub', key: 'a' });
    const s2 = reducer(s1, { type: 'shell/selectGroup', group: null });
    expect(s2.shell.activeGroup).toBeNull();
    expect(s2.shell.activeSubKey).toBeNull();
  });
});

describe('reducer — shell/selectSub', () => {
  it('sets activeSubKey', () => {
    const s0 = reducer(boot(), { type: 'shell/selectGroup', group: 'gateway' });
    const s1 = reducer(s0, { type: 'shell/selectSub', key: 'a' });
    expect(s1.shell.activeSubKey).toBe('a');
  });
});

describe('reducer — shell/toggleGroup', () => {
  it('adds an expanded group when absent', () => {
    const s1 = reducer(initialState, { type: 'shell/toggleGroup', group: 'models' });
    expect(s1.shell.expandedGroups.has('models')).toBe(true);
  });

  it('removes an expanded group when present', () => {
    const s1 = reducer(initialState, { type: 'shell/toggleGroup', group: 'gateway' });
    expect(s1.shell.expandedGroups.has('gateway')).toBe(false);
  });
});

describe('groupDirty', () => {
  it('returns false for a pristine state', () => {
    const s0 = boot();
    for (const id of [
      'models',
      'gateway',
      'memory',
      'skills',
      'runtime',
      'advanced',
      'observability',
    ] as const satisfies readonly GroupId[]) {
      expect(groupDirty(s0, id)).toBe(false);
    }
  });

  it('returns true for the gateway group after an edit', () => {
    const s0 = boot();
    const s1 = reducer(s0, {
      type: 'edit/field',
      key: 'a',
      field: 'token',
      value: 'x',
    });
    expect(groupDirty(s1, 'gateway')).toBe(true);
    expect(groupDirty(s1, 'models')).toBe(false);
  });
});

describe('dirtyGroups + totalDirtyCount', () => {
  it('lists only gateway as dirty after an edit', () => {
    const s0 = boot();
    const s1 = reducer(s0, {
      type: 'edit/field',
      key: 'a',
      field: 'token',
      value: 'x',
    });
    expect(dirtyGroups(s1)).toEqual(new Set<GroupId>(['gateway']));
  });

  it('totalDirtyCount equals dirtyCount (sum of IM instance diffs) in stage 1', () => {
    const s0 = boot();
    const s1 = reducer(s0, {
      type: 'edit/field',
      key: 'a',
      field: 'token',
      value: 'x',
    });
    expect(totalDirtyCount(s1)).toBe(1);
  });
});
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd web && pnpm test -- state
```

Expected: FAIL — `shell/selectGroup` action type not recognized; exports `groupDirty`, `dirtyGroups`, `totalDirtyCount` not found.

- [ ] **Step 3: Extend state.ts — types and initialState**

Modify `web/src/state.ts`. At the top, import `GroupId`:

```ts
import type { GroupId } from './shell/groups';
```

Inside the file, replace the `AppState` interface with:

```ts
export interface ShellSliceState {
  activeGroup: GroupId | null;
  activeSubKey: string | null;
  expandedGroups: Set<GroupId>;
}

export interface AppState {
  status: Status;
  descriptors: SchemaDescriptor[];
  config: Config;
  originalConfig: Config;
  /** Legacy field retained for existing IM code paths.
   *  Mirrors shell.activeSubKey when shell.activeGroup === 'gateway'. */
  selectedKey: string | null;
  flash: Flash | null;
  shell: ShellSliceState;
}
```

Replace `initialState`:

```ts
export const initialState: AppState = {
  status: 'booting',
  descriptors: [],
  config: {},
  originalConfig: {},
  selectedKey: null,
  flash: null,
  shell: {
    activeGroup: null,
    activeSubKey: null,
    expandedGroups: new Set<GroupId>(['gateway']),
  },
};
```

Extend the `Action` union:

```ts
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
  | { type: 'instance/delete'; key: string }
  | { type: 'instance/create'; key: string; platformType: string }
  | { type: 'shell/selectGroup'; group: GroupId | null }
  | { type: 'shell/selectSub'; key: string | null }
  | { type: 'shell/toggleGroup'; group: GroupId };
```

- [ ] **Step 4: Extend reducer cases**

In the `reducer` function's `switch` block, add these three cases before the closing `}`:

```ts
case 'shell/selectGroup': {
  const next = {
    ...state,
    shell: {
      ...state.shell,
      activeGroup: action.group,
      activeSubKey: null,
    },
  };
  // Keep legacy selectedKey in sync so the existing IM Editor path keeps working
  return { ...next, selectedKey: null };
}
case 'shell/selectSub':
  return {
    ...state,
    shell: { ...state.shell, activeSubKey: action.key },
    selectedKey:
      state.shell.activeGroup === 'gateway' ? action.key : state.selectedKey,
  };
case 'shell/toggleGroup': {
  const expanded = new Set(state.shell.expandedGroups);
  if (expanded.has(action.group)) expanded.delete(action.group);
  else expanded.add(action.group);
  return { ...state, shell: { ...state.shell, expandedGroups: expanded } };
}
```

- [ ] **Step 5: Add groupDirty / dirtyGroups / totalDirtyCount**

Append to the end of `state.ts`:

```ts
import { GROUPS, type GroupId as _GroupId } from './shell/groups';

/** groupDirty returns true if the config slice for the group differs from
 *  the originalConfig snapshot. Stage 1: only 'gateway' can be dirty. */
export function groupDirty(state: AppState, group: GroupId): boolean {
  if (group === 'gateway') {
    return dirtyCount(state) > 0;
  }
  // For non-gateway groups, compare the relevant configKeys shallowly.
  const def = GROUPS.find(g => g.id === group);
  if (!def) return false;
  const a = state.config as unknown as Record<string, unknown>;
  const b = state.originalConfig as unknown as Record<string, unknown>;
  for (const k of def.configKeys) {
    if (!deepEqual(a[k], b[k])) return true;
  }
  return false;
}

/** dirtyGroups returns the set of groups with unsaved changes. */
export function dirtyGroups(state: AppState): Set<GroupId> {
  const out = new Set<GroupId>();
  for (const g of GROUPS) {
    if (groupDirty(state, g.id)) out.add(g.id);
  }
  return out;
}

/** totalDirtyCount returns the total number of dirty sub-items across all
 *  groups. Stage 1: equals dirtyCount (the IM instance diff count); later
 *  stages may sum per-group sub-counts. */
export function totalDirtyCount(state: AppState): number {
  return dirtyCount(state);
}

function deepEqual(a: unknown, b: unknown): boolean {
  if (a === b) return true;
  if (a === null || b === null) return false;
  if (typeof a !== typeof b) return false;
  if (typeof a !== 'object') return false;
  if (Array.isArray(a) || Array.isArray(b)) {
    if (!Array.isArray(a) || !Array.isArray(b)) return false;
    if (a.length !== b.length) return false;
    for (let i = 0; i < a.length; i++) {
      if (!deepEqual(a[i], b[i])) return false;
    }
    return true;
  }
  const ao = a as Record<string, unknown>;
  const bo = b as Record<string, unknown>;
  const keys = new Set<string>([...Object.keys(ao), ...Object.keys(bo)]);
  for (const k of keys) {
    if (!deepEqual(ao[k], bo[k])) return false;
  }
  return true;
}
```

Note: the unused `_GroupId` import exists only to satisfy tsc re-exporting; remove the duplicate `import type { GroupId }` if TypeScript complains. The critical fact is that `GROUPS` is imported.

- [ ] **Step 6: Run tests to verify they pass**

```bash
cd web && pnpm test -- state
```

Expected: PASS (all existing state tests + 11 new shell/selector tests).

- [ ] **Step 7: Commit**

```bash
git add web/src/state.ts web/src/state.test.ts
git commit -m "feat(web/state): shell slice + dirty-group selectors"
```

---

## Task 6: localStorage persistence for expandedGroups

**Files:**
- Create: `web/src/shell/persistence.ts`
- Create: `web/src/shell/persistence.test.ts`
- Modify: `web/src/state.ts`

**Why:** User's collapse/expand state of each group should survive page reload. Keyed `hermind.shell.expandedGroups` → JSON array of group ids.

- [ ] **Step 1: Write the failing test**

Create `web/src/shell/persistence.test.ts`:

```ts
import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { loadExpandedGroups, saveExpandedGroups, STORAGE_KEY } from './persistence';

describe('loadExpandedGroups', () => {
  beforeEach(() => {
    localStorage.clear();
  });
  afterEach(() => {
    localStorage.clear();
  });

  it('returns the default set {gateway} when localStorage is empty', () => {
    const got = loadExpandedGroups();
    expect(Array.from(got).sort()).toEqual(['gateway']);
  });

  it('reads a valid persisted array', () => {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(['models', 'memory']));
    const got = loadExpandedGroups();
    expect(got.has('models')).toBe(true);
    expect(got.has('memory')).toBe(true);
    expect(got.has('gateway')).toBe(false);
  });

  it('ignores unknown group ids in the stored array', () => {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(['models', 'not-a-group']));
    const got = loadExpandedGroups();
    expect(got.has('models')).toBe(true);
    expect(got.size).toBe(1);
  });

  it('falls back to default on malformed JSON', () => {
    localStorage.setItem(STORAGE_KEY, '{not json');
    const got = loadExpandedGroups();
    expect(Array.from(got).sort()).toEqual(['gateway']);
  });

  it('falls back to default when the stored value is not an array', () => {
    localStorage.setItem(STORAGE_KEY, JSON.stringify({ gateway: true }));
    const got = loadExpandedGroups();
    expect(Array.from(got).sort()).toEqual(['gateway']);
  });
});

describe('saveExpandedGroups', () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it('writes a sorted JSON array of group ids', () => {
    saveExpandedGroups(new Set(['memory', 'models']));
    expect(localStorage.getItem(STORAGE_KEY)).toBe(JSON.stringify(['memory', 'models']));
  });

  it('writes an empty array for an empty set', () => {
    saveExpandedGroups(new Set());
    expect(localStorage.getItem(STORAGE_KEY)).toBe('[]');
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd web && pnpm test -- persistence
```

Expected: FAIL — module `./persistence` not found.

- [ ] **Step 3: Implement persistence.ts**

Create `web/src/shell/persistence.ts`:

```ts
import { GROUP_IDS, type GroupId } from './groups';

export const STORAGE_KEY = 'hermind.shell.expandedGroups';
const DEFAULT_EXPANDED: readonly GroupId[] = ['gateway'];

export function loadExpandedGroups(): Set<GroupId> {
  const raw = tryRead();
  if (raw === null) return new Set(DEFAULT_EXPANDED);
  let parsed: unknown;
  try {
    parsed = JSON.parse(raw);
  } catch {
    return new Set(DEFAULT_EXPANDED);
  }
  if (!Array.isArray(parsed)) return new Set(DEFAULT_EXPANDED);
  const out = new Set<GroupId>();
  for (const v of parsed) {
    if (typeof v === 'string' && GROUP_IDS.has(v as GroupId)) {
      out.add(v as GroupId);
    }
  }
  return out;
}

export function saveExpandedGroups(set: Set<GroupId>): void {
  const arr = Array.from(set).sort();
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(arr));
  } catch {
    // quota exceeded or storage disabled — silently drop
  }
}

function tryRead(): string | null {
  try {
    return localStorage.getItem(STORAGE_KEY);
  } catch {
    return null;
  }
}
```

- [ ] **Step 4: Run persistence tests to verify they pass**

```bash
cd web && pnpm test -- persistence
```

Expected: PASS (7 tests).

- [ ] **Step 5: Wire persistence into state.ts initial state + toggle action**

In `web/src/state.ts`:

1. Add import at the top:
```ts
import { loadExpandedGroups, saveExpandedGroups } from './shell/persistence';
```

2. Replace `initialState` to use `loadExpandedGroups()`:

```ts
export const initialState: AppState = {
  status: 'booting',
  descriptors: [],
  config: {},
  originalConfig: {},
  selectedKey: null,
  flash: null,
  shell: {
    activeGroup: null,
    activeSubKey: null,
    expandedGroups: loadExpandedGroups(),
  },
};
```

3. In the reducer's `shell/toggleGroup` case, call `saveExpandedGroups` as a side effect AFTER computing the next set:

```ts
case 'shell/toggleGroup': {
  const expanded = new Set(state.shell.expandedGroups);
  if (expanded.has(action.group)) expanded.delete(action.group);
  else expanded.add(action.group);
  saveExpandedGroups(expanded);
  return { ...state, shell: { ...state.shell, expandedGroups: expanded } };
}
```

- [ ] **Step 6: Add a state integration test for persistence**

Append to `web/src/state.test.ts`:

```ts
import { STORAGE_KEY } from './shell/persistence';

describe('reducer — shell/toggleGroup persistence', () => {
  beforeEach(() => localStorage.clear());

  it('writes to localStorage on toggle', () => {
    const s1 = reducer(initialState, { type: 'shell/toggleGroup', group: 'models' });
    expect(JSON.parse(localStorage.getItem(STORAGE_KEY) ?? '[]')).toContain('models');
    expect(s1.shell.expandedGroups.has('models')).toBe(true);
  });
});
```

Add `import { beforeEach } from 'vitest';` to the existing imports in the test file if it isn't already imported.

- [ ] **Step 7: Run all tests to verify**

```bash
cd web && pnpm test
```

Expected: PASS (all previous tests + 7 persistence + 1 new state test).

- [ ] **Step 8: Commit**

```bash
git add web/src/shell/persistence.ts web/src/shell/persistence.test.ts web/src/state.ts web/src/state.test.ts
git commit -m "feat(web/shell): persist expandedGroups to localStorage"
```

---

## Task 7: TopBar rewrite (global Save button)

**Files:**
- Create: `web/src/components/shell/TopBar.tsx`
- Create: `web/src/components/shell/TopBar.module.css`
- Create: `web/src/components/shell/TopBar.test.tsx`
- Delete: `web/src/components/TopBar.tsx` (done during commit)
- Delete: `web/src/components/TopBar.module.css` (done during commit)

**Why:** New TopBar owns the global **Save** button (moved from Footer) + the dirty-count badge. Save-and-Apply disappears (Apply moves to each group's panel).

- [ ] **Step 1: Write the failing test**

Create `web/src/components/shell/TopBar.test.tsx`:

```tsx
import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import TopBar from './TopBar';

describe('TopBar', () => {
  it('shows the brand label and "All saved" when clean', () => {
    render(<TopBar dirtyCount={0} status="ready" onSave={() => {}} />);
    expect(screen.getByText(/hermind/i)).toBeInTheDocument();
    expect(screen.getByText(/all saved/i)).toBeInTheDocument();
  });

  it('shows dirty count and enables Save when dirty', () => {
    render(<TopBar dirtyCount={3} status="ready" onSave={() => {}} />);
    const btn = screen.getByRole('button', { name: /save/i });
    expect(btn).toBeEnabled();
    expect(btn).toHaveTextContent(/3 changes/);
  });

  it('disables Save when clean', () => {
    render(<TopBar dirtyCount={0} status="ready" onSave={() => {}} />);
    expect(screen.getByRole('button', { name: /save/i })).toBeDisabled();
  });

  it('disables Save while status is saving', () => {
    render(<TopBar dirtyCount={1} status="saving" onSave={() => {}} />);
    expect(screen.getByRole('button', { name: /save/i })).toBeDisabled();
    expect(screen.getByText(/saving/i)).toBeInTheDocument();
  });

  it('calls onSave when the button is clicked', async () => {
    const onSave = vi.fn();
    render(<TopBar dirtyCount={1} status="ready" onSave={onSave} />);
    await userEvent.click(screen.getByRole('button', { name: /save/i }));
    expect(onSave).toHaveBeenCalledTimes(1);
  });

  it('does not render a Save-and-Apply button (regression guard)', () => {
    render(<TopBar dirtyCount={1} status="ready" onSave={() => {}} />);
    expect(screen.queryByText(/apply/i)).not.toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd web && pnpm test -- TopBar
```

Expected: FAIL — module `./TopBar` in `components/shell/` not found.

- [ ] **Step 3: Create TopBar.module.css**

Create `web/src/components/shell/TopBar.module.css`:

```css
.topbar {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 8px 16px;
  background: var(--bg-topbar, #161b22);
  border-bottom: 1px solid var(--border, #30363d);
  height: 48px;
  box-sizing: border-box;
}

.brand {
  display: flex;
  align-items: center;
  gap: 8px;
  font-weight: 600;
  color: var(--fg, #c9d1d9);
}

.logo {
  font-size: 18px;
  color: var(--accent, #58a6ff);
}

.spacer { flex: 1; }

.status {
  display: flex;
  align-items: center;
  gap: 6px;
  color: var(--fg-muted, #8b949e);
  font-size: 12px;
}

.dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
}

.dotIdle { background: var(--ok, #3fb950); }
.dotDirty { background: var(--warn, #d29922); }
.dotBusy { background: var(--accent, #58a6ff); animation: pulse 1s infinite; }

@keyframes pulse {
  0% { opacity: 1; }
  50% { opacity: 0.4; }
  100% { opacity: 1; }
}

.saveBtn {
  padding: 6px 14px;
  border-radius: 4px;
  border: 1px solid var(--border, #30363d);
  background: var(--accent, #1f6feb);
  color: white;
  font-weight: 500;
  cursor: pointer;
}

.saveBtn:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}
```

- [ ] **Step 4: Implement TopBar.tsx**

Create `web/src/components/shell/TopBar.tsx`:

```tsx
import styles from './TopBar.module.css';
import type { Status } from '../../state';

export interface TopBarProps {
  dirtyCount: number;
  status: Status;
  onSave: () => void;
}

export default function TopBar({ dirtyCount, status, onSave }: TopBarProps) {
  const busy = status === 'saving' || status === 'applying';
  const dotClass = busy
    ? styles.dotBusy
    : dirtyCount > 0
      ? styles.dotDirty
      : styles.dotIdle;
  const statusMsg = busy
    ? status === 'saving'
      ? 'Saving…'
      : 'Applying…'
    : dirtyCount > 0
      ? `${dirtyCount} unsaved change${dirtyCount === 1 ? '' : 's'}`
      : 'All saved';
  const saveLabel = dirtyCount > 0 ? `Save · ${dirtyCount} changes` : 'Save';
  return (
    <header className={styles.topbar}>
      <div className={styles.brand}>
        <span className={styles.logo}>⬡</span>
        <span>hermind</span>
      </div>
      <span className={styles.spacer} />
      <span className={styles.status}>
        <span className={`${styles.dot} ${dotClass}`} />
        {statusMsg}
      </span>
      <button
        type="button"
        className={styles.saveBtn}
        onClick={onSave}
        disabled={busy || dirtyCount === 0}
      >
        {saveLabel}
      </button>
    </header>
  );
}
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
cd web && pnpm test -- TopBar
```

Expected: PASS (6 tests).

- [ ] **Step 6: Delete old TopBar and commit**

```bash
rm web/src/components/TopBar.tsx web/src/components/TopBar.module.css
git add -A web/src/components/
git commit -m "feat(web/shell): new TopBar with global Save button"
```

Note: App.tsx still imports the old path; that will be fixed in Task 16. Build will break between Task 7 and Task 16 for the `import TopBar from './components/TopBar'` line. `pnpm test` continues to work because tests import by file. The plan accepts this transient broken state; the final integration task (16) restores the build.

---

## Task 8: ComingSoonPanel

**Files:**
- Create: `web/src/components/shell/ComingSoonPanel.tsx`
- Create: `web/src/components/shell/ComingSoonPanel.module.css`
- Create: `web/src/components/shell/ComingSoonPanel.test.tsx`

**Why:** The content panel for groups whose editors don't exist yet. Shows label + planned stage + bullet list + read-only summary + CLI escape.

- [ ] **Step 1: Write the failing test**

Create `web/src/components/shell/ComingSoonPanel.test.tsx`:

```tsx
import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import ComingSoonPanel from './ComingSoonPanel';
import type { Config } from '../../api/schemas';

const cfg: Config = {};

describe('ComingSoonPanel', () => {
  it('renders the group label as a heading', () => {
    render(<ComingSoonPanel group="models" config={cfg} />);
    expect(screen.getByRole('heading', { name: /models/i })).toBeInTheDocument();
  });

  it('displays the planned stage string', () => {
    render(<ComingSoonPanel group="models" config={cfg} />);
    expect(screen.getByText(/stage 3 & 4/i)).toBeInTheDocument();
  });

  it('renders the group bullets', () => {
    render(<ComingSoonPanel group="models" config={cfg} />);
    expect(screen.getByText(/default model selection/i)).toBeInTheDocument();
    expect(screen.getByText(/fallback providers/i)).toBeInTheDocument();
  });

  it('renders the summary for the group', () => {
    render(
      <ComingSoonPanel
        group="memory"
        config={{ memory: { enabled: true, retain_db: {} } } as unknown as Config}
      />,
    );
    expect(screen.getByText(/retain_db/i)).toBeInTheDocument();
    expect(screen.getByText(/yes/i)).toBeInTheDocument();
  });

  it('renders the Edit via CLI escape hatch', () => {
    render(<ComingSoonPanel group="models" config={cfg} />);
    expect(screen.getByText(/hermind config --web/i)).toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd web && pnpm test -- ComingSoon
```

Expected: FAIL — module `./ComingSoonPanel` not found.

- [ ] **Step 3: Create ComingSoonPanel.module.css**

Create `web/src/components/shell/ComingSoonPanel.module.css`:

```css
.panel {
  padding: 32px 40px;
  max-width: 720px;
}

.label {
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 1px;
  color: var(--fg-muted, #8b949e);
  margin-bottom: 8px;
}

.title {
  margin: 0 0 8px;
  font-size: 22px;
  color: var(--fg, #c9d1d9);
}

.stage {
  display: inline-block;
  padding: 2px 8px;
  border-radius: 10px;
  background: var(--bg-badge, #21262d);
  color: var(--fg-muted, #8b949e);
  font-size: 12px;
  margin-bottom: 16px;
}

.desc { color: var(--fg, #c9d1d9); margin: 8px 0 16px; }

.bullets { margin: 0 0 20px 18px; color: var(--fg-muted, #8b949e); }

.previewLabel {
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  color: var(--fg-muted, #8b949e);
  margin: 24px 0 6px;
}

.preview {
  border: 1px solid var(--border, #30363d);
  border-radius: 6px;
  padding: 12px;
  background: var(--bg-card, #0d1117);
}

.preview :global(.summary-row) {
  display: flex;
  justify-content: space-between;
  padding: 4px 0;
}

.preview :global(.summary-label) { color: var(--fg-muted, #8b949e); }
.preview :global(.summary-value) { color: var(--fg, #c9d1d9); font-family: monospace; }

.escape {
  margin-top: 24px;
  padding: 10px 12px;
  background: var(--bg-card, #0d1117);
  border: 1px dashed var(--border, #30363d);
  border-radius: 6px;
  color: var(--fg-muted, #8b949e);
  font-size: 13px;
}

.escape code {
  background: var(--bg-topbar, #161b22);
  padding: 2px 6px;
  border-radius: 3px;
  color: var(--fg, #c9d1d9);
}
```

- [ ] **Step 4: Implement ComingSoonPanel.tsx**

Create `web/src/components/shell/ComingSoonPanel.tsx`:

```tsx
import styles from './ComingSoonPanel.module.css';
import type { Config } from '../../api/schemas';
import { findGroup, type GroupId } from '../../shell/groups';
import { summaryFor } from '../../shell/summaries';

export interface ComingSoonPanelProps {
  group: GroupId;
  config: Config;
}

export default function ComingSoonPanel({ group, config }: ComingSoonPanelProps) {
  const def = findGroup(group);
  return (
    <section className={styles.panel} aria-label={`${def.label} — coming soon`}>
      <div className={styles.label}>{def.label}</div>
      <h2 className={styles.title}>Coming soon</h2>
      <span className={styles.stage}>Planned for stage {def.plannedStage}</span>
      <p className={styles.desc}>{def.description}</p>

      <div className={styles.label}>This section will cover</div>
      <ul className={styles.bullets}>
        {def.bullets.map(b => (
          <li key={b}>{b}</li>
        ))}
      </ul>

      <div className={styles.previewLabel}>Current config (read-only preview)</div>
      <div className={styles.preview}>{summaryFor(group, config)}</div>

      <div className={styles.escape}>
        Need to edit this now? Run <code>hermind config --web</code> for the legacy editor.
      </div>
    </section>
  );
}
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
cd web && pnpm test -- ComingSoon
```

Expected: PASS (5 tests).

- [ ] **Step 6: Commit**

```bash
git add web/src/components/shell/ComingSoonPanel.tsx web/src/components/shell/ComingSoonPanel.module.css web/src/components/shell/ComingSoonPanel.test.tsx
git commit -m "feat(web/shell): ComingSoonPanel with group summary + CLI escape"
```

---

## Task 9: EmptyState

**Files:**
- Create: `web/src/components/shell/EmptyState.tsx`
- Create: `web/src/components/shell/EmptyState.module.css`
- Create: `web/src/components/shell/EmptyState.test.tsx`

**Why:** Rendered when `activeGroup === null` (fresh load, no hash). Shows 7 group cards the user can click to jump in.

- [ ] **Step 1: Write the failing test**

Create `web/src/components/shell/EmptyState.test.tsx`:

```tsx
import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import EmptyState from './EmptyState';

describe('EmptyState', () => {
  it('renders a card for every group', () => {
    render(<EmptyState onSelectGroup={() => {}} />);
    for (const label of [
      'Models',
      'Gateway',
      'Memory',
      'Skills',
      'Runtime',
      'Advanced',
      'Observability',
    ]) {
      expect(screen.getByRole('button', { name: new RegExp(label, 'i') })).toBeInTheDocument();
    }
  });

  it('calls onSelectGroup with the right id when a card is clicked', async () => {
    const onSelectGroup = vi.fn();
    render(<EmptyState onSelectGroup={onSelectGroup} />);
    await userEvent.click(screen.getByRole('button', { name: /memory/i }));
    expect(onSelectGroup).toHaveBeenCalledWith('memory');
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd web && pnpm test -- EmptyState
```

Expected: FAIL — module `./EmptyState` not found.

- [ ] **Step 3: Create EmptyState.module.css**

Create `web/src/components/shell/EmptyState.module.css`:

```css
.empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 40px 24px;
  min-height: 60vh;
}

.title {
  font-size: 18px;
  color: var(--fg, #c9d1d9);
  margin: 0 0 24px;
}

.grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
  gap: 12px;
  width: 100%;
  max-width: 760px;
}

.card {
  text-align: left;
  padding: 14px 16px;
  background: var(--bg-card, #0d1117);
  border: 1px solid var(--border, #30363d);
  border-radius: 6px;
  color: var(--fg, #c9d1d9);
  cursor: pointer;
  font-family: inherit;
}

.card:hover { border-color: var(--accent, #58a6ff); }

.cardLabel { font-weight: 600; margin-bottom: 4px; }
.cardDesc { font-size: 12px; color: var(--fg-muted, #8b949e); }
.cardStage {
  display: inline-block;
  margin-top: 8px;
  padding: 1px 6px;
  font-size: 10px;
  border-radius: 8px;
  background: var(--bg-topbar, #161b22);
  color: var(--fg-muted, #8b949e);
}
```

- [ ] **Step 4: Implement EmptyState.tsx**

Create `web/src/components/shell/EmptyState.tsx`:

```tsx
import styles from './EmptyState.module.css';
import { GROUPS, type GroupId } from '../../shell/groups';

export interface EmptyStateProps {
  onSelectGroup: (id: GroupId) => void;
}

export default function EmptyState({ onSelectGroup }: EmptyStateProps) {
  return (
    <section className={styles.empty}>
      <h2 className={styles.title}>Select a configuration section</h2>
      <div className={styles.grid}>
        {GROUPS.map(g => (
          <button
            key={g.id}
            type="button"
            className={styles.card}
            onClick={() => onSelectGroup(g.id)}
          >
            <div className={styles.cardLabel}>{g.label}</div>
            <div className={styles.cardDesc}>{g.description}</div>
            <span className={styles.cardStage}>
              {g.plannedStage === 'done' ? 'available' : `stage ${g.plannedStage}`}
            </span>
          </button>
        ))}
      </div>
    </section>
  );
}
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
cd web && pnpm test -- EmptyState
```

Expected: PASS (2 tests).

- [ ] **Step 6: Commit**

```bash
git add web/src/components/shell/EmptyState.tsx web/src/components/shell/EmptyState.module.css web/src/components/shell/EmptyState.test.tsx
git commit -m "feat(web/shell): EmptyState — 7-group landing grid"
```

---

## Task 10: GatewaySidebar (lifted from current Sidebar)

**Files:**
- Create: `web/src/components/groups/gateway/GatewaySidebar.tsx`
- Create: `web/src/components/groups/gateway/GatewaySidebar.module.css`
- Create: `web/src/components/groups/gateway/GatewaySidebar.test.tsx`

**Why:** When the Gateway group is expanded in the main Sidebar, its children are the IM instances. That logic is currently in `components/Sidebar.tsx`; it moves here, keeping its existing look and behavior.

- [ ] **Step 1: Write the failing test**

Create `web/src/components/groups/gateway/GatewaySidebar.test.tsx`:

```tsx
import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import GatewaySidebar from './GatewaySidebar';
import type { SchemaDescriptor } from '../../../api/schemas';

const descriptors: SchemaDescriptor[] = [
  { type: 'feishu', display_name: 'Feishu Bot', fields: [] },
  { type: 'dingtalk', display_name: 'DingTalk', fields: [] },
] as unknown as SchemaDescriptor[];

const instances = [
  { key: 'feishu-main', type: 'feishu', enabled: true },
  { key: 'dt-alerts', type: 'dingtalk', enabled: false },
];

describe('GatewaySidebar', () => {
  it('lists every instance', () => {
    render(
      <GatewaySidebar
        instances={instances}
        selectedKey={null}
        descriptors={descriptors}
        dirtyKeys={new Set()}
        onSelect={() => {}}
        onNewInstance={() => {}}
      />,
    );
    expect(screen.getByText('feishu-main')).toBeInTheDocument();
    expect(screen.getByText('dt-alerts')).toBeInTheDocument();
  });

  it('shows an empty hint when there are no instances', () => {
    render(
      <GatewaySidebar
        instances={[]}
        selectedKey={null}
        descriptors={descriptors}
        dirtyKeys={new Set()}
        onSelect={() => {}}
        onNewInstance={() => {}}
      />,
    );
    expect(screen.getByText(/no instances configured/i)).toBeInTheDocument();
  });

  it('calls onSelect when an instance is clicked', async () => {
    const onSelect = vi.fn();
    render(
      <GatewaySidebar
        instances={instances}
        selectedKey={null}
        descriptors={descriptors}
        dirtyKeys={new Set()}
        onSelect={onSelect}
        onNewInstance={() => {}}
      />,
    );
    await userEvent.click(screen.getByText('feishu-main'));
    expect(onSelect).toHaveBeenCalledWith('feishu-main');
  });

  it('calls onNewInstance when "+ New" is clicked', async () => {
    const onNewInstance = vi.fn();
    render(
      <GatewaySidebar
        instances={instances}
        selectedKey={null}
        descriptors={descriptors}
        dirtyKeys={new Set()}
        onSelect={() => {}}
        onNewInstance={onNewInstance}
      />,
    );
    await userEvent.click(screen.getByRole('button', { name: /new instance/i }));
    expect(onNewInstance).toHaveBeenCalledTimes(1);
  });

  it('renders a dirty dot next to dirty keys', () => {
    render(
      <GatewaySidebar
        instances={instances}
        selectedKey={null}
        descriptors={descriptors}
        dirtyKeys={new Set(['feishu-main'])}
        onSelect={() => {}}
        onNewInstance={() => {}}
      />,
    );
    // The dirty dot has title="Unsaved changes" in the current Sidebar impl.
    expect(screen.getByTitle(/unsaved changes/i)).toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd web && pnpm test -- GatewaySidebar
```

Expected: FAIL — module `./GatewaySidebar` not found.

- [ ] **Step 3: Copy existing Sidebar.module.css into the group folder**

```bash
cp web/src/components/Sidebar.module.css web/src/components/groups/gateway/GatewaySidebar.module.css
```

- [ ] **Step 4: Implement GatewaySidebar.tsx (ported from components/Sidebar.tsx, no functional changes)**

Create `web/src/components/groups/gateway/GatewaySidebar.tsx`:

```tsx
import styles from './GatewaySidebar.module.css';
import type { SchemaDescriptor } from '../../../api/schemas';

export interface GatewaySidebarProps {
  instances: Array<{ key: string; type: string; enabled: boolean }>;
  selectedKey: string | null;
  descriptors: SchemaDescriptor[];
  dirtyKeys: Set<string>;
  onSelect: (key: string) => void;
  onNewInstance: () => void;
}

export default function GatewaySidebar({
  instances,
  selectedKey,
  descriptors,
  dirtyKeys,
  onSelect,
  onNewInstance,
}: GatewaySidebarProps) {
  const displayNames = new Map(descriptors.map(d => [d.type, d.display_name]));
  return (
    <div className={styles.sidebar}>
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
            {dirtyKeys.has(inst.key) && (
              <span className={styles.dirtyDot} title="Unsaved changes" />
            )}
          </span>
          <span className={styles.itemType}>
            {displayNames.get(inst.type) ?? inst.type}
            {!inst.enabled && <span className={styles.offBadge}>off</span>}
          </span>
        </button>
      ))}
      <button type="button" className={styles.newBtn} onClick={onNewInstance}>
        + New instance
      </button>
    </div>
  );
}
```

Note: the root element is now a `div` (not `aside` — the outer `aside` belongs to the parent Sidebar). The CSS file's top-level `.sidebar` rule will still target it but semantically this is nested.

- [ ] **Step 5: Run tests to verify they pass**

```bash
cd web && pnpm test -- GatewaySidebar
```

Expected: PASS (5 tests).

- [ ] **Step 6: Commit**

```bash
git add web/src/components/groups/gateway/GatewaySidebar.tsx web/src/components/groups/gateway/GatewaySidebar.module.css web/src/components/groups/gateway/GatewaySidebar.test.tsx
git commit -m "feat(web/gateway): lift GatewaySidebar from components/Sidebar"
```

---

## Task 11: GroupSection (collapsible group row)

**Files:**
- Create: `web/src/components/shell/GroupSection.tsx`
- Create: `web/src/components/shell/GroupSection.module.css`
- Create: `web/src/components/shell/GroupSection.test.tsx`

**Why:** One collapsible row for each group in the Sidebar. Header with ▸/▾ + label; children visible only when expanded.

- [ ] **Step 1: Write the failing test**

Create `web/src/components/shell/GroupSection.test.tsx`:

```tsx
import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import GroupSection from './GroupSection';

describe('GroupSection', () => {
  it('renders the group label', () => {
    render(
      <GroupSection
        group="models"
        expanded={false}
        active={false}
        onToggle={() => {}}
        onSelectGroup={() => {}}
      />,
    );
    expect(screen.getByText(/models/i)).toBeInTheDocument();
  });

  it('shows a right-arrow glyph when collapsed and down-arrow when expanded', () => {
    const { rerender } = render(
      <GroupSection
        group="models"
        expanded={false}
        active={false}
        onToggle={() => {}}
        onSelectGroup={() => {}}
      />,
    );
    expect(screen.getByText('▸')).toBeInTheDocument();
    rerender(
      <GroupSection
        group="models"
        expanded={true}
        active={false}
        onToggle={() => {}}
        onSelectGroup={() => {}}
      />,
    );
    expect(screen.getByText('▾')).toBeInTheDocument();
  });

  it('calls onToggle when the arrow is clicked', async () => {
    const onToggle = vi.fn();
    render(
      <GroupSection
        group="models"
        expanded={false}
        active={false}
        onToggle={onToggle}
        onSelectGroup={() => {}}
      />,
    );
    await userEvent.click(screen.getByRole('button', { name: /toggle/i }));
    expect(onToggle).toHaveBeenCalledTimes(1);
  });

  it('calls onSelectGroup when the label is clicked', async () => {
    const onSelectGroup = vi.fn();
    render(
      <GroupSection
        group="memory"
        expanded={false}
        active={false}
        onToggle={() => {}}
        onSelectGroup={onSelectGroup}
      />,
    );
    await userEvent.click(screen.getByText(/memory/i));
    expect(onSelectGroup).toHaveBeenCalledWith('memory');
  });

  it('shows children only when expanded', () => {
    const { rerender } = render(
      <GroupSection
        group="gateway"
        expanded={false}
        active={false}
        onToggle={() => {}}
        onSelectGroup={() => {}}
      >
        <div>child content</div>
      </GroupSection>,
    );
    expect(screen.queryByText(/child content/i)).not.toBeInTheDocument();
    rerender(
      <GroupSection
        group="gateway"
        expanded={true}
        active={false}
        onToggle={() => {}}
        onSelectGroup={() => {}}
      >
        <div>child content</div>
      </GroupSection>,
    );
    expect(screen.getByText(/child content/i)).toBeInTheDocument();
  });

  it('shows a dirty dot when dirty=true', () => {
    render(
      <GroupSection
        group="gateway"
        expanded={false}
        active={false}
        dirty
        onToggle={() => {}}
        onSelectGroup={() => {}}
      />,
    );
    expect(screen.getByTitle(/unsaved/i)).toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd web && pnpm test -- GroupSection
```

Expected: FAIL — module not found.

- [ ] **Step 3: Create GroupSection.module.css**

Create `web/src/components/shell/GroupSection.module.css`:

```css
.section { margin-bottom: 6px; }

.header {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 4px 6px;
  cursor: default;
}

.toggle {
  width: 16px;
  padding: 0;
  background: transparent;
  border: 0;
  color: var(--fg-muted, #8b949e);
  cursor: pointer;
  font-size: 11px;
}

.label {
  flex: 1;
  background: transparent;
  border: 0;
  text-align: left;
  color: var(--fg, #c9d1d9);
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  cursor: pointer;
  padding: 0;
}

.active { color: var(--accent, #58a6ff); }

.dirtyDot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--warn, #d29922);
}

.children { padding-left: 14px; }

.comingSoon {
  padding: 4px 8px;
  color: var(--fg-muted, #8b949e);
  font-size: 11px;
  font-style: italic;
}
```

- [ ] **Step 4: Implement GroupSection.tsx**

Create `web/src/components/shell/GroupSection.tsx`:

```tsx
import type { ReactNode } from 'react';
import styles from './GroupSection.module.css';
import { findGroup, type GroupId } from '../../shell/groups';

export interface GroupSectionProps {
  group: GroupId;
  expanded: boolean;
  active: boolean;
  dirty?: boolean;
  children?: ReactNode;
  onToggle: () => void;
  onSelectGroup: (id: GroupId) => void;
}

export default function GroupSection({
  group,
  expanded,
  active,
  dirty = false,
  children,
  onToggle,
  onSelectGroup,
}: GroupSectionProps) {
  const def = findGroup(group);
  const isGateway = group === 'gateway';
  return (
    <div className={styles.section}>
      <div className={styles.header}>
        <button
          type="button"
          aria-label={`toggle ${def.label}`}
          className={styles.toggle}
          onClick={onToggle}
        >
          {expanded ? '▾' : '▸'}
        </button>
        <button
          type="button"
          className={`${styles.label} ${active ? styles.active : ''}`}
          onClick={() => onSelectGroup(group)}
        >
          {def.label}
        </button>
        {dirty && <span className={styles.dirtyDot} title="Unsaved changes" />}
      </div>
      {expanded && (
        <div className={styles.children}>
          {isGateway
            ? children
            : <div className={styles.comingSoon}>Coming soon — stage {def.plannedStage}</div>}
        </div>
      )}
    </div>
  );
}
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
cd web && pnpm test -- GroupSection
```

Expected: PASS (6 tests).

- [ ] **Step 6: Commit**

```bash
git add web/src/components/shell/GroupSection.tsx web/src/components/shell/GroupSection.module.css web/src/components/shell/GroupSection.test.tsx
git commit -m "feat(web/shell): GroupSection — collapsible group row"
```

---

## Task 12: Sidebar rewrite (shell/Sidebar)

**Files:**
- Create: `web/src/components/shell/Sidebar.tsx`
- Create: `web/src/components/shell/Sidebar.module.css`
- Create: `web/src/components/shell/Sidebar.test.tsx`
- Delete (at commit): `web/src/components/Sidebar.tsx`
- Delete (at commit): `web/src/components/Sidebar.module.css`

**Why:** Combines 7 `GroupSection` instances. Gateway's `children` are the `GatewaySidebar`; others' children are the built-in "Coming soon — stage N" row.

- [ ] **Step 1: Write the failing test**

Create `web/src/components/shell/Sidebar.test.tsx`:

```tsx
import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type { ComponentProps } from 'react';
import Sidebar from './Sidebar';
import type { SchemaDescriptor } from '../../api/schemas';

const descriptors: SchemaDescriptor[] = [] as SchemaDescriptor[];

function baseProps(
  overrides: Partial<ComponentProps<typeof Sidebar>> = {},
): ComponentProps<typeof Sidebar> {
  return {
    activeGroup: null,
    activeSubKey: null,
    expandedGroups: new Set(['gateway']),
    dirtyGroups: new Set(),
    instances: [],
    selectedKey: null,
    descriptors,
    dirtyInstanceKeys: new Set<string>(),
    onSelectGroup: vi.fn(),
    onSelectSub: vi.fn(),
    onToggleGroup: vi.fn(),
    onNewInstance: vi.fn(),
    ...overrides,
  };
}

describe('Sidebar', () => {
  it('renders all 7 group labels', () => {
    render(<Sidebar {...baseProps()} />);
    for (const label of [
      'Models',
      'Gateway',
      'Memory',
      'Skills',
      'Runtime',
      'Advanced',
      'Observability',
    ]) {
      expect(screen.getByText(label)).toBeInTheDocument();
    }
  });

  it('shows instance children only under Gateway when expanded', () => {
    render(
      <Sidebar
        {...baseProps({
          instances: [{ key: 'feishu', type: 'feishu', enabled: true }],
          expandedGroups: new Set(['gateway']),
        })}
      />,
    );
    expect(screen.getByText('feishu')).toBeInTheDocument();
  });

  it('shows "Coming soon" rows under expanded non-Gateway groups', () => {
    render(
      <Sidebar
        {...baseProps({
          expandedGroups: new Set(['gateway', 'models']),
        })}
      />,
    );
    expect(screen.getByText(/coming soon — stage 3 & 4/i)).toBeInTheDocument();
  });

  it('calls onToggleGroup when an arrow is clicked', async () => {
    const onToggleGroup = vi.fn();
    render(<Sidebar {...baseProps({ onToggleGroup })} />);
    await userEvent.click(screen.getAllByRole('button', { name: /toggle models/i })[0]);
    expect(onToggleGroup).toHaveBeenCalledWith('models');
  });

  it('marks Gateway as dirty when dirtyGroups contains gateway', () => {
    render(<Sidebar {...baseProps({ dirtyGroups: new Set(['gateway']) })} />);
    expect(screen.getByTitle(/unsaved/i)).toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd web && pnpm test -- "shell/Sidebar"
```

Expected: FAIL — `shell/Sidebar` module not found.

- [ ] **Step 3: Create Sidebar.module.css**

Create `web/src/components/shell/Sidebar.module.css`:

```css
.sidebar {
  padding: 12px 8px;
  background: var(--bg-sidebar, #0d1117);
  border-right: 1px solid var(--border, #30363d);
  overflow-y: auto;
  min-width: 220px;
  max-width: 220px;
}
```

- [ ] **Step 4: Implement Sidebar.tsx**

Create `web/src/components/shell/Sidebar.tsx`:

```tsx
import styles from './Sidebar.module.css';
import { GROUPS, type GroupId } from '../../shell/groups';
import GroupSection from './GroupSection';
import GatewaySidebar from '../groups/gateway/GatewaySidebar';
import type { SchemaDescriptor } from '../../api/schemas';

export interface SidebarProps {
  activeGroup: GroupId | null;
  activeSubKey: string | null;
  expandedGroups: Set<GroupId>;
  dirtyGroups: Set<GroupId>;
  instances: Array<{ key: string; type: string; enabled: boolean }>;
  selectedKey: string | null;
  descriptors: SchemaDescriptor[];
  dirtyInstanceKeys: Set<string>;
  onSelectGroup: (id: GroupId) => void;
  onSelectSub: (key: string) => void;
  onToggleGroup: (id: GroupId) => void;
  onNewInstance: () => void;
}

export default function Sidebar(props: SidebarProps) {
  return (
    <aside className={styles.sidebar} aria-label="Configuration groups">
      {GROUPS.map(g => (
        <GroupSection
          key={g.id}
          group={g.id}
          expanded={props.expandedGroups.has(g.id)}
          active={props.activeGroup === g.id}
          dirty={props.dirtyGroups.has(g.id)}
          onToggle={() => props.onToggleGroup(g.id)}
          onSelectGroup={props.onSelectGroup}
        >
          {g.id === 'gateway' && (
            <GatewaySidebar
              instances={props.instances}
              selectedKey={props.selectedKey}
              descriptors={props.descriptors}
              dirtyKeys={props.dirtyInstanceKeys}
              onSelect={props.onSelectSub}
              onNewInstance={props.onNewInstance}
            />
          )}
        </GroupSection>
      ))}
    </aside>
  );
}
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
cd web && pnpm test -- "shell/Sidebar"
```

Expected: PASS (5 tests).

- [ ] **Step 6: Delete old Sidebar and commit**

```bash
rm web/src/components/Sidebar.tsx web/src/components/Sidebar.module.css
git add -A web/src/components/
git commit -m "feat(web/shell): new Sidebar with 7 collapsible groups"
```

---

## Task 13: GatewayApplyButton (extracted from old Footer)

**Files:**
- Create: `web/src/components/groups/gateway/GatewayApplyButton.tsx`
- Create: `web/src/components/groups/gateway/GatewayApplyButton.module.css`
- Create: `web/src/components/groups/gateway/GatewayApplyButton.test.tsx`

**Why:** Apply moves out of the old global Footer into the Gateway panel's breadcrumb row. Disabled when gateway slice is dirty or a request is in flight.

- [ ] **Step 1: Write the failing test**

Create `web/src/components/groups/gateway/GatewayApplyButton.test.tsx`:

```tsx
import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import GatewayApplyButton from './GatewayApplyButton';

describe('GatewayApplyButton', () => {
  it('is enabled when clean and idle', () => {
    render(<GatewayApplyButton dirty={false} busy={false} onApply={() => {}} />);
    expect(screen.getByRole('button', { name: /apply/i })).toBeEnabled();
  });

  it('is disabled when the gateway slice is dirty', () => {
    render(<GatewayApplyButton dirty={true} busy={false} onApply={() => {}} />);
    expect(screen.getByRole('button', { name: /apply/i })).toBeDisabled();
  });

  it('is disabled while busy', () => {
    render(<GatewayApplyButton dirty={false} busy={true} onApply={() => {}} />);
    expect(screen.getByRole('button', { name: /apply/i })).toBeDisabled();
  });

  it('calls onApply when clicked', async () => {
    const onApply = vi.fn();
    render(<GatewayApplyButton dirty={false} busy={false} onApply={onApply} />);
    await userEvent.click(screen.getByRole('button', { name: /apply/i }));
    expect(onApply).toHaveBeenCalledTimes(1);
  });

  it('shows a tooltip hint when disabled due to dirty', () => {
    render(<GatewayApplyButton dirty={true} busy={false} onApply={() => {}} />);
    expect(screen.getByRole('button', { name: /apply/i })).toHaveAttribute(
      'title',
      expect.stringMatching(/save first/i) as unknown as string,
    );
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd web && pnpm test -- GatewayApply
```

Expected: FAIL — module not found.

- [ ] **Step 3: Create GatewayApplyButton.module.css**

Create `web/src/components/groups/gateway/GatewayApplyButton.module.css`:

```css
.btn {
  padding: 4px 12px;
  border-radius: 4px;
  border: 1px solid var(--border, #30363d);
  background: var(--bg-card, #0d1117);
  color: var(--fg, #c9d1d9);
  cursor: pointer;
  font-size: 12px;
}

.btn:hover:not(:disabled) {
  border-color: var(--accent, #58a6ff);
  color: var(--accent, #58a6ff);
}

.btn:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}
```

- [ ] **Step 4: Implement GatewayApplyButton.tsx**

Create `web/src/components/groups/gateway/GatewayApplyButton.tsx`:

```tsx
import styles from './GatewayApplyButton.module.css';

export interface GatewayApplyButtonProps {
  dirty: boolean;
  busy: boolean;
  onApply: () => void;
}

export default function GatewayApplyButton({ dirty, busy, onApply }: GatewayApplyButtonProps) {
  const title = dirty
    ? 'Save first, then apply'
    : busy
      ? 'Busy…'
      : 'Restart gateway with current config';
  return (
    <button
      type="button"
      className={styles.btn}
      onClick={onApply}
      disabled={dirty || busy}
      title={title}
    >
      Apply
    </button>
  );
}
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
cd web && pnpm test -- GatewayApply
```

Expected: PASS (5 tests).

- [ ] **Step 6: Commit**

```bash
git add web/src/components/groups/gateway/GatewayApplyButton.tsx web/src/components/groups/gateway/GatewayApplyButton.module.css web/src/components/groups/gateway/GatewayApplyButton.test.tsx
git commit -m "feat(web/gateway): GatewayApplyButton — section-local apply"
```

---

## Task 14: GatewayPanel

**Files:**
- Create: `web/src/components/groups/gateway/GatewayPanel.tsx`
- Create: `web/src/components/groups/gateway/GatewayPanel.module.css`
- Create: `web/src/components/groups/gateway/GatewayPanel.test.tsx`

**Why:** The content panel for the Gateway group. Renders a breadcrumb + `GatewayApplyButton` + the existing `Editor` from `components/Editor.tsx` (unchanged).

- [ ] **Step 1: Write the failing test**

Create `web/src/components/groups/gateway/GatewayPanel.test.tsx`:

```tsx
import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import type { ComponentProps } from 'react';
import GatewayPanel from './GatewayPanel';

function makeProps(overrides: Partial<ComponentProps<typeof GatewayPanel>> = {}) {
  return {
    selectedKey: null,
    instance: null,
    originalInstance: null,
    descriptor: null,
    dirty: false,
    busy: false,
    onField: vi.fn(),
    onToggleEnabled: vi.fn(),
    onDelete: vi.fn(),
    onApply: vi.fn(),
    ...overrides,
  };
}

describe('GatewayPanel', () => {
  it('shows a breadcrumb with the selected key when one exists', () => {
    render(
      <GatewayPanel
        {...makeProps({
          selectedKey: 'feishu-main',
          instance: { type: 'feishu', enabled: true, options: {} },
        })}
      />,
    );
    expect(screen.getByText(/feishu-main/)).toBeInTheDocument();
  });

  it('shows a generic breadcrumb when no instance is selected', () => {
    render(<GatewayPanel {...makeProps()} />);
    expect(screen.getByText(/gateway/i)).toBeInTheDocument();
  });

  it('renders GatewayApplyButton', () => {
    render(<GatewayPanel {...makeProps()} />);
    expect(screen.getByRole('button', { name: /apply/i })).toBeInTheDocument();
  });

  it('passes dirty flag through to the apply button', () => {
    render(<GatewayPanel {...makeProps({ dirty: true })} />);
    expect(screen.getByRole('button', { name: /apply/i })).toBeDisabled();
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd web && pnpm test -- GatewayPanel
```

Expected: FAIL — module not found.

- [ ] **Step 3: Create GatewayPanel.module.css**

Create `web/src/components/groups/gateway/GatewayPanel.module.css`:

```css
.panel {
  display: flex;
  flex-direction: column;
  height: 100%;
}

.header {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 16px;
  border-bottom: 1px solid var(--border, #30363d);
}

.crumbs { flex: 1; font-size: 12px; color: var(--fg-muted, #8b949e); }
.crumbs strong { color: var(--fg, #c9d1d9); font-weight: 600; }

.body { flex: 1; overflow: auto; }
```

- [ ] **Step 4: Implement GatewayPanel.tsx**

Create `web/src/components/groups/gateway/GatewayPanel.tsx`:

```tsx
import styles from './GatewayPanel.module.css';
import type { PlatformInstance, SchemaDescriptor } from '../../../api/schemas';
import Editor from '../../Editor';
import GatewayApplyButton from './GatewayApplyButton';

export interface GatewayPanelProps {
  selectedKey: string | null;
  instance: PlatformInstance | null;
  originalInstance: PlatformInstance | null;
  descriptor: SchemaDescriptor | null;
  dirty: boolean;
  busy: boolean;
  onField: (field: string, value: string) => void;
  onToggleEnabled: (enabled: boolean) => void;
  onDelete: () => void;
  onApply: () => void;
}

export default function GatewayPanel({
  selectedKey,
  instance,
  originalInstance,
  descriptor,
  dirty,
  busy,
  onField,
  onToggleEnabled,
  onDelete,
  onApply,
}: GatewayPanelProps) {
  return (
    <section className={styles.panel} aria-label="Gateway configuration">
      <div className={styles.header}>
        <div className={styles.crumbs}>
          <strong>Gateway</strong>
          {selectedKey && <> · {selectedKey}</>}
        </div>
        <GatewayApplyButton dirty={dirty} busy={busy} onApply={onApply} />
      </div>
      <div className={styles.body}>
        <Editor
          selectedKey={selectedKey}
          instance={instance}
          originalInstance={originalInstance}
          descriptor={descriptor}
          onField={onField}
          onToggleEnabled={onToggleEnabled}
          onDelete={onDelete}
        />
      </div>
    </section>
  );
}
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
cd web && pnpm test -- GatewayPanel
```

Expected: PASS (4 tests).

- [ ] **Step 6: Commit**

```bash
git add web/src/components/groups/gateway/GatewayPanel.tsx web/src/components/groups/gateway/GatewayPanel.module.css web/src/components/groups/gateway/GatewayPanel.test.tsx
git commit -m "feat(web/gateway): GatewayPanel wraps existing Editor + apply"
```

---

## Task 15: ContentPanel (group-to-panel router)

**Files:**
- Create: `web/src/components/shell/ContentPanel.tsx`
- Create: `web/src/components/shell/ContentPanel.test.tsx`

**Why:** Single dispatch point: given `activeGroup`, render the right panel (GatewayPanel for gateway, ComingSoonPanel for everything else, EmptyState for null).

- [ ] **Step 1: Write the failing test**

Create `web/src/components/shell/ContentPanel.test.tsx`:

```tsx
import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import type { ComponentProps } from 'react';
import ContentPanel from './ContentPanel';
import type { Config } from '../../api/schemas';

const emptyCfg: Config = {};

function makeProps(
  overrides: Partial<ComponentProps<typeof ContentPanel>> = {},
): ComponentProps<typeof ContentPanel> {
  return {
    activeGroup: null,
    config: emptyCfg,
    // gateway-specific — defaults acceptable when activeGroup !== 'gateway'
    selectedKey: null,
    instance: null,
    originalInstance: null,
    descriptor: null,
    dirtyGateway: false,
    busy: false,
    onField: () => {},
    onToggleEnabled: () => {},
    onDelete: () => {},
    onApply: () => {},
    onSelectGroup: () => {},
    ...overrides,
  };
}

describe('ContentPanel', () => {
  it('renders EmptyState when activeGroup is null', () => {
    render(<ContentPanel {...makeProps()} />);
    expect(screen.getByText(/select a configuration section/i)).toBeInTheDocument();
  });

  it('renders GatewayPanel when activeGroup is gateway', () => {
    render(<ContentPanel {...makeProps({ activeGroup: 'gateway' })} />);
    expect(screen.getByRole('button', { name: /apply/i })).toBeInTheDocument();
  });

  it('renders ComingSoonPanel for every non-gateway group', () => {
    for (const id of ['models', 'memory', 'skills', 'runtime', 'advanced', 'observability'] as const) {
      const { unmount } = render(<ContentPanel {...makeProps({ activeGroup: id })} />);
      expect(screen.getByText(/coming soon/i)).toBeInTheDocument();
      unmount();
    }
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd web && pnpm test -- ContentPanel
```

Expected: FAIL — module not found.

- [ ] **Step 3: Implement ContentPanel.tsx**

Create `web/src/components/shell/ContentPanel.tsx`:

```tsx
import type { Config, PlatformInstance, SchemaDescriptor } from '../../api/schemas';
import { type GroupId } from '../../shell/groups';
import ComingSoonPanel from './ComingSoonPanel';
import EmptyState from './EmptyState';
import GatewayPanel from '../groups/gateway/GatewayPanel';

export interface ContentPanelProps {
  activeGroup: GroupId | null;
  config: Config;
  selectedKey: string | null;
  instance: PlatformInstance | null;
  originalInstance: PlatformInstance | null;
  descriptor: SchemaDescriptor | null;
  dirtyGateway: boolean;
  busy: boolean;
  onField: (field: string, value: string) => void;
  onToggleEnabled: (enabled: boolean) => void;
  onDelete: () => void;
  onApply: () => void;
  onSelectGroup: (id: GroupId) => void;
}

export default function ContentPanel(props: ContentPanelProps) {
  if (props.activeGroup === null) {
    return <EmptyState onSelectGroup={props.onSelectGroup} />;
  }
  if (props.activeGroup === 'gateway') {
    return (
      <GatewayPanel
        selectedKey={props.selectedKey}
        instance={props.instance}
        originalInstance={props.originalInstance}
        descriptor={props.descriptor}
        dirty={props.dirtyGateway}
        busy={props.busy}
        onField={props.onField}
        onToggleEnabled={props.onToggleEnabled}
        onDelete={props.onDelete}
        onApply={props.onApply}
      />
    );
  }
  return <ComingSoonPanel group={props.activeGroup} config={props.config} />;
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd web && pnpm test -- ContentPanel
```

Expected: PASS (3 tests).

- [ ] **Step 5: Commit**

```bash
git add web/src/components/shell/ContentPanel.tsx web/src/components/shell/ContentPanel.test.tsx
git commit -m "feat(web/shell): ContentPanel routes activeGroup to the right panel"
```

---

## Task 16: App.tsx integration + Footer cleanup

**Files:**
- Modify: `web/src/App.tsx` (major rewrite)
- Modify: `web/src/components/Footer.tsx` (drop Save + Save-and-Apply)
- Modify: `web/src/components/Footer.module.css` (keep only flash styles)

**Why:** Wire everything together. Replace the old IM-only shell with the new multi-group shell. Footer is reduced to a one-line flash strip (kept for continuity).

- [ ] **Step 1: Rewrite App.tsx**

Fully replace `web/src/App.tsx` with:

```tsx
import { useCallback, useEffect, useMemo, useReducer, useState } from 'react';
import { apiFetch, ApiError } from './api/client';
import {
  ApplyResultSchema,
  ConfigResponseSchema,
  PlatformsSchemaResponseSchema,
} from './api/schemas';
import {
  dirtyGroups as selectDirtyGroups,
  initialState,
  instanceDirty,
  listInstances,
  reducer,
  totalDirtyCount,
} from './state';
import { migrateLegacyHash, parseHash, stringifyHash } from './shell/hash';
import type { GroupId } from './shell/groups';
import TopBar from './components/shell/TopBar';
import Sidebar from './components/shell/Sidebar';
import ContentPanel from './components/shell/ContentPanel';
import Footer from './components/Footer';
import NewInstanceDialog from './components/NewInstanceDialog';

export default function App() {
  const [state, dispatch] = useReducer(reducer, initialState);
  const [newDialogOpen, setNewDialogOpen] = useState(false);

  // Boot: fetch schema + config
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

  // Resolve initial hash (including legacy migration) once config is available.
  useEffect(() => {
    if (state.status !== 'ready') return;
    if (state.shell.activeGroup !== null) return;
    const currentHash = window.location.hash;
    const platforms = Object.keys(state.config.gateway?.platforms ?? {});
    const migrated = migrateLegacyHash(currentHash, platforms);
    const effective = migrated ?? currentHash;
    if (migrated) {
      history.replaceState(null, '', window.location.pathname + window.location.search + migrated);
    }
    const parsed = parseHash(effective);
    if (parsed.group) {
      dispatch({ type: 'shell/selectGroup', group: parsed.group });
      if (parsed.sub && parsed.group === 'gateway') {
        dispatch({ type: 'shell/selectSub', key: parsed.sub });
      }
    }
    // If parsed.group is null, stay in EmptyState — no dispatch needed.
  }, [state.status, state.shell.activeGroup, state.config.gateway?.platforms]);

  // Sync hash whenever active group/sub changes.
  useEffect(() => {
    if (state.status === 'booting') return;
    const wanted = stringifyHash(state.shell.activeGroup, state.shell.activeSubKey);
    if (window.location.hash !== wanted) {
      if (wanted) {
        history.replaceState(null, '', window.location.pathname + window.location.search + wanted);
      } else if (window.location.hash) {
        history.replaceState(null, '', window.location.pathname + window.location.search);
      }
    }
  }, [state.shell.activeGroup, state.shell.activeSubKey, state.status]);

  const instances = useMemo(() => {
    const plats = state.config.gateway?.platforms ?? {};
    return listInstances(state).map(key => ({
      key,
      type: plats[key]?.type ?? '',
      enabled: plats[key]?.enabled ?? false,
    }));
  }, [state]);

  const dirtyInstanceKeys = useMemo(() => {
    const a = state.config.gateway?.platforms ?? {};
    const b = state.originalConfig.gateway?.platforms ?? {};
    const keys = new Set([...Object.keys(a), ...Object.keys(b)]);
    const out = new Set<string>();
    for (const k of keys) {
      if (instanceDirty(state, k)) out.add(k);
    }
    return out;
  }, [state]);

  const dirtyGroupIds = useMemo(() => selectDirtyGroups(state), [state]);
  const dirty = totalDirtyCount(state);
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
      dispatch({ type: 'save/done', error: toErrMsg(err) });
    }
  }, [state.config]);

  const onApplyGateway = useCallback(async () => {
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
  }, []);

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

  const selectedKey = state.shell.activeSubKey;
  const selectedInstance = selectedKey
    ? state.config.gateway?.platforms?.[selectedKey] ?? null
    : null;
  const selectedOriginal = selectedKey
    ? state.originalConfig.gateway?.platforms?.[selectedKey] ?? null
    : null;
  const selectedDescriptor = selectedInstance
    ? state.descriptors.find(d => d.type === selectedInstance.type) ?? null
    : null;

  return (
    <div className="app-shell">
      <TopBar dirtyCount={dirty} status={state.status} onSave={onSave} />
      <Sidebar
        activeGroup={state.shell.activeGroup}
        activeSubKey={state.shell.activeSubKey}
        expandedGroups={state.shell.expandedGroups}
        dirtyGroups={dirtyGroupIds}
        instances={instances}
        selectedKey={selectedKey}
        descriptors={state.descriptors}
        dirtyInstanceKeys={dirtyInstanceKeys}
        onSelectGroup={(id: GroupId) => dispatch({ type: 'shell/selectGroup', group: id })}
        onSelectSub={(key: string) => {
          dispatch({ type: 'shell/selectGroup', group: 'gateway' });
          dispatch({ type: 'shell/selectSub', key });
        }}
        onToggleGroup={(id: GroupId) => dispatch({ type: 'shell/toggleGroup', group: id })}
        onNewInstance={() => setNewDialogOpen(true)}
      />
      <main>
        <ContentPanel
          activeGroup={state.shell.activeGroup}
          config={state.config}
          selectedKey={selectedKey}
          instance={selectedInstance}
          originalInstance={selectedOriginal}
          descriptor={selectedDescriptor}
          dirtyGateway={dirtyGroupIds.has('gateway')}
          busy={busy}
          onField={(field, value) =>
            selectedKey &&
            dispatch({ type: 'edit/field', key: selectedKey, field, value })
          }
          onToggleEnabled={enabled =>
            selectedKey &&
            dispatch({ type: 'edit/enabled', key: selectedKey, enabled })
          }
          onDelete={() => selectedKey && dispatch({ type: 'instance/delete', key: selectedKey })}
          onApply={onApplyGateway}
          onSelectGroup={(id: GroupId) => dispatch({ type: 'shell/selectGroup', group: id })}
        />
      </main>
      <Footer flash={state.flash} />
      {newDialogOpen && (
        <NewInstanceDialog
          descriptors={state.descriptors}
          existingKeys={new Set(Object.keys(state.config.gateway?.platforms ?? {}))}
          onCancel={() => setNewDialogOpen(false)}
          onCreate={(key, platformType) => {
            dispatch({ type: 'instance/create', key, platformType });
            dispatch({ type: 'shell/selectGroup', group: 'gateway' });
            dispatch({ type: 'shell/selectSub', key });
            setNewDialogOpen(false);
          }}
        />
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

- [ ] **Step 2: Trim Footer.tsx to flash-only**

Replace `web/src/components/Footer.tsx` with:

```tsx
import styles from './Footer.module.css';
import type { Flash } from '../state';

export interface FooterProps {
  flash: Flash | null;
}

export default function Footer({ flash }: FooterProps) {
  if (!flash) {
    return <footer className={styles.footer} />;
  }
  const cls = flash.kind === 'err' ? styles.flashErr : styles.flashOk;
  return (
    <footer className={styles.footer}>
      <span className={cls}>{flash.msg}</span>
    </footer>
  );
}
```

Update `web/src/components/Footer.module.css` — keep the `.footer`, `.flashOk`, `.flashErr` rules; delete the button rules (`.btn`, `.primary`, `.secondary`, `.status`, `.spacer`). Final file (replace whole contents if unclear):

```css
.footer {
  display: flex;
  align-items: center;
  padding: 4px 16px;
  min-height: 24px;
  background: var(--bg-footer, #161b22);
  border-top: 1px solid var(--border, #30363d);
  font-size: 12px;
}

.flashOk { color: var(--ok, #3fb950); }
.flashErr { color: var(--error, #f85149); }
```

- [ ] **Step 3: Build + type-check**

```bash
cd web && pnpm type-check
```

Expected: PASS. If there are errors, they indicate a drift between an earlier task and the integration — fix before proceeding.

- [ ] **Step 4: Run the full test suite**

```bash
cd web && pnpm test
```

Expected: PASS (everything — shell tests, state tests, existing IM tests).

- [ ] **Step 5: Dev-run + manual smoke-test**

```bash
# in one shell
cd web && pnpm dev
# then open http://localhost:5173 in the browser
```

Manually verify:

- Sidebar shows all 7 group labels (Models, Gateway, Memory, Skills, Runtime, Advanced, Observability) with Gateway expanded.
- Click `▸` next to Models → row expands, shows "Coming soon — stage 3 & 4".
- Click the `Models` label → right panel shows ComingSoonPanel with planned stage 3 & 4 and a preview.
- Click `Gateway` label → right panel shows the Gateway panel with any existing instances.
- Select an instance → editor works exactly as before (field edits, toggle, delete).
- Edit a field → TopBar Save button enables and shows `Save · 1 changes`; Gateway's Apply button disables with tooltip "Save first, then apply".
- Click Save → flash "Saved." appears in the footer; Save re-disables; Apply re-enables.
- Click Gateway's Apply button → flash "Applied." appears.
- Visit `http://localhost:5173/#feishu-bot-main` (substitute a real instance key) → URL should auto-migrate to `#gateway/feishu-bot-main`; instance is selected.
- Reload the page → the expanded groups you left open are still open (localStorage).

- [ ] **Step 6: Commit**

```bash
git add web/src/App.tsx web/src/components/Footer.tsx web/src/components/Footer.module.css
git commit -m "feat(web): integrate new shell, drop TopBar/Footer Apply button"
```

---

## Task 17: Rebuild embedded assets + sync check + smoke doc

**Files:**
- Modify: `api/webroot/` (regenerated bundle)
- Modify: `docs/smoke/web-config.md`

**Why:** CI asserts `api/webroot/` matches the current Vite build. Smoke doc needs new test steps for the multi-group shell.

- [ ] **Step 1: Rebuild the web bundle and sync api/webroot/**

```bash
make web
```

Expected output: `pnpm build` succeeds, `api/webroot/` is refreshed.

- [ ] **Step 2: Re-run the CI gate**

```bash
make web-check
```

Expected: `type-check`, `pnpm test`, `pnpm lint`, `pnpm build`, `api/webroot/` sync assertion all PASS.

- [ ] **Step 3: Update docs/smoke/web-config.md**

Open `docs/smoke/web-config.md` and append a `## Stage 1 · Shell rewrite` section with these items (merge with existing structure; if the file doesn't have headings, use a bulleted list at the end):

```markdown
## Stage 1 · Shell rewrite

- Sidebar shows all seven groups: Models, Gateway, Memory, Skills, Runtime, Advanced, Observability.
- Gateway is expanded on first load; all others are collapsed.
- Clicking a non-Gateway group label shows a "Coming soon — stage N" panel with a read-only summary and an "Edit via CLI" note.
- Legacy deep link: visiting `/#feishu-bot-main` (with `feishu-bot-main` configured) auto-rewrites the URL to `/#gateway/feishu-bot-main` and selects that instance.
- Unknown legacy hashes fall back to the empty state.
- The TopBar Save button is disabled when clean and shows `Save · N changes` when dirty.
- There is no global `Save and Apply` button.
- Gateway panel has its own `Apply` button in the breadcrumb row.
- Apply is disabled while gateway slice is dirty; tooltip reads "Save first, then apply".
- Reload preserves the expanded/collapsed state of each group (localStorage key `hermind.shell.expandedGroups`).
- Empty state (no hash, no saved selection) shows a 7-card landing grid; clicking a card opens that group.
```

- [ ] **Step 4: Commit**

```bash
git add api/webroot/ docs/smoke/web-config.md
git commit -m "chore(web): rebuild api/webroot + smoke doc for shell rewrite"
```

---

## Completion checklist

Before calling stage 1 done, verify:

- [ ] `cd web && pnpm test` — all PASS
- [ ] `cd web && pnpm type-check` — PASS
- [ ] `cd web && pnpm lint` — zero warnings (the project has been at zero warnings; any new warning here is a regression to fix)
- [ ] `make web-check` — PASS (includes the `api/webroot/` sync assertion)
- [ ] Manual smoke test items in Task 16 Step 5 all pass
- [ ] `docs/smoke/web-config.md` has the new Stage 1 section
- [ ] No leftover files in `web/src/components/TopBar.*` or `web/src/components/Sidebar.*`
- [ ] Git status is clean (`git status --short` returns empty modulo other stages' in-progress work)

Once all boxes are checked, stage 1 is complete. Stage 2 (Schema infrastructure) can begin.
