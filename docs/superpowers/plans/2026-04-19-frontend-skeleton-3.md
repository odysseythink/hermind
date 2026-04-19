# Frontend Skeleton (Stage 3) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the placeholder `api/webroot/index.html` with a real Vite + React + TypeScript shell that loads config + platform schema on boot and renders the app chrome (TopBar / Sidebar / Footer) with an empty-state Editor pane. No actual field rendering yet — Stage 4 adds that.

**Architecture:** A new `web/` directory holds the Vite project; `make web` builds it and syncs `web/dist/` into `api/webroot/` (which remains the committed artifact the Go server embeds). Asset URLs use the `/ui/` prefix that the existing `handleStatic` route already strips. Token injection keeps using the `{{TOKEN}}` template substitution already implemented in `api/server.go::handleIndex`.

**Tech Stack:** Vite 5, React 18, TypeScript 5, zod for runtime schema validation. No UI framework, no router, no state library. CSS Modules for component styles + one global theme file with design tokens.

**Prerequisites for the implementer:** Node.js 20+ and pnpm 9+ available on PATH. If pnpm is missing, install via `corepack enable && corepack prepare pnpm@9 --activate` (ships with Node 20+). No global npm install needed.

**Source of truth:** `docs/superpowers/specs/2026-04-19-web-im-config-design.md` §8 "Frontend architecture" + §11 stage-3 entry.

**Explicit scope cuts (deferred to Stage 4 / 5):**
- No FieldList / SecretInput / TestConnection / NewInstanceDialog — Editor is empty-state only.
- No actual Save / Save&Apply wiring — buttons render but log-only on click.
- No CI `make web` enforcement gate — Stage 5.

---

## File Structure

**Create (new `web/` source tree):**

```
web/
├── package.json
├── tsconfig.json
├── tsconfig.node.json
├── vite.config.ts
├── index.html
├── .gitignore
├── .env.example
└── src/
    ├── main.tsx            Entry: renders <App/>.
    ├── App.tsx             Root: useReducer, shell layout.
    ├── state.ts            AppState type + reducer + actions.
    ├── api/
    │   ├── client.ts       apiFetch(path, init) with Bearer header.
    │   └── schemas.ts      zod schemas + inferred types for Config / Descriptor / Errors.
    ├── components/
    │   ├── TopBar.tsx      Brand + config path + status dot.
    │   ├── TopBar.module.css
    │   ├── Sidebar.tsx     Instance list + "New instance" button (stub).
    │   ├── Sidebar.module.css
    │   ├── Footer.tsx      Status text + Save + Save & Apply (stub).
    │   ├── Footer.module.css
    │   ├── Editor.tsx      Empty-state view.
    │   └── Editor.module.css
    └── styles/
        └── theme.css       CSS custom properties, prefers-color-scheme.
```

**Modify:**

- `api/webroot/index.html` — replaced by the built artifact every `make web` run.
- `api/webroot/assets/` — populated by the build.
- `api/server.go` — no code changes needed (handleIndex's `{{TOKEN}}` substitution still works on the built index.html because we'll reference `{{TOKEN}}` in `web/index.html`'s source).
- `Makefile` — adds `web` + `web-install` + `web-dev` targets.
- `.gitignore` — adds `web/node_modules`, `web/dist`, `web/.env.local`, `.DS_Store`.

**Untouched:**

- Every backend file (`cli/`, `gateway/`, `config/`).
- The Stage 2a/2b handlers.

---

## Task 1: Vite + React + TS scaffold + Makefile integration

**Files:**
- Create: `web/package.json`, `web/tsconfig.json`, `web/tsconfig.node.json`, `web/vite.config.ts`, `web/index.html`, `web/.gitignore`, `web/.env.example`, `web/src/main.tsx`, `web/src/App.tsx`, `web/src/styles/theme.css`
- Modify: `Makefile`, `.gitignore`
- Modify: `api/webroot/index.html` (replaced by the first build)

This task produces a minimal working app: "hermind web" with no content beyond a placeholder heading. The point is to prove the toolchain builds, embeds, and serves.

- [ ] **Step 1: Create `web/package.json`**

```json
{
  "name": "hermind-web",
  "version": "0.0.0",
  "private": true,
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "tsc --noEmit && vite build",
    "type-check": "tsc --noEmit",
    "lint": "eslint 'src/**/*.{ts,tsx}'"
  },
  "dependencies": {
    "react": "^18.3.1",
    "react-dom": "^18.3.1",
    "zod": "^3.23.8"
  },
  "devDependencies": {
    "@types/react": "^18.3.3",
    "@types/react-dom": "^18.3.0",
    "@vitejs/plugin-react": "^4.3.1",
    "typescript": "^5.5.4",
    "vite": "^5.4.1"
  },
  "packageManager": "pnpm@9.7.0"
}
```

- [ ] **Step 2: Create `web/tsconfig.json`**

```json
{
  "compilerOptions": {
    "target": "ES2022",
    "useDefineForClassFields": true,
    "lib": ["ES2022", "DOM", "DOM.Iterable"],
    "module": "ESNext",
    "skipLibCheck": true,
    "moduleResolution": "bundler",
    "allowImportingTsExtensions": true,
    "resolveJsonModule": true,
    "isolatedModules": true,
    "noEmit": true,
    "jsx": "react-jsx",
    "strict": true,
    "noUnusedLocals": true,
    "noUnusedParameters": true,
    "noFallthroughCasesInSwitch": true
  },
  "include": ["src"],
  "references": [{ "path": "./tsconfig.node.json" }]
}
```

- [ ] **Step 3: Create `web/tsconfig.node.json`**

```json
{
  "compilerOptions": {
    "composite": true,
    "skipLibCheck": true,
    "module": "ESNext",
    "moduleResolution": "bundler",
    "allowSyntheticDefaultImports": true,
    "strict": true
  },
  "include": ["vite.config.ts"]
}
```

- [ ] **Step 4: Create `web/vite.config.ts`**

```ts
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [react()],
  base: '/ui/',
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    sourcemap: false,
    // Keep filename hashing so cache-busting works after every make web.
    rollupOptions: {
      output: {
        entryFileNames: 'assets/[name]-[hash].js',
        chunkFileNames: 'assets/[name]-[hash].js',
        assetFileNames: 'assets/[name]-[hash][extname]',
      },
    },
  },
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:9119',
        changeOrigin: true,
      },
    },
  },
});
```

- [ ] **Step 5: Create `web/index.html`**

```html
<!doctype html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="color-scheme" content="light dark">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>hermind</title>
  <script>
    // Populated by api/server.go's handleIndex template substitution.
    // In dev (vite) it stays "{{TOKEN}}" — read VITE_HERMIND_TOKEN from
    // .env.local instead (see src/api/client.ts).
    window.HERMIND = { token: "{{TOKEN}}" };
  </script>
</head>
<body>
  <div id="root"></div>
  <script type="module" src="/src/main.tsx"></script>
</body>
</html>
```

- [ ] **Step 6: Create `web/.gitignore`**

```
node_modules
dist
.env.local
.DS_Store
```

- [ ] **Step 7: Create `web/.env.example`**

```
# Copy to .env.local for `pnpm dev`. The web server (api) injects this
# value via handleIndex for the built artifact; dev mode reads it here.
VITE_HERMIND_TOKEN=
```

- [ ] **Step 8: Create `web/src/styles/theme.css`**

Minimal tokens for Stage 3 — expanded in Task 2.

```css
:root {
  --bg: #ffffff;
  --text: #111827;
  --muted: #6b7280;
  --accent: #FFB800;
  --border: #e5e7eb;
  --font-sans: ui-sans-serif, system-ui, -apple-system, "SF Pro Text",
               "Segoe UI", "PingFang SC", "Microsoft YaHei", sans-serif;
}

@media (prefers-color-scheme: dark) {
  :root {
    --bg: #0b0d11;
    --text: #e6e8eb;
    --muted: #8892a1;
    --border: #2a2f38;
  }
}

* { box-sizing: border-box; }
html, body { margin: 0; padding: 0; height: 100%; }
body {
  font-family: var(--font-sans);
  font-size: 14px;
  line-height: 1.5;
  color: var(--text);
  background: var(--bg);
}
#root { height: 100%; }
```

- [ ] **Step 9: Create `web/src/main.tsx`**

```tsx
import React from 'react';
import { createRoot } from 'react-dom/client';
import './styles/theme.css';
import App from './App';

const rootElem = document.getElementById('root');
if (!rootElem) {
  throw new Error('hermind: #root element missing');
}
createRoot(rootElem).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
);
```

- [ ] **Step 10: Create `web/src/App.tsx`**

Placeholder-only version for Task 1. Later tasks replace this.

```tsx
export default function App() {
  return (
    <main style={{ padding: '2rem', fontFamily: 'var(--font-sans)' }}>
      <h1>hermind</h1>
      <p>Stage 3 scaffolding. Shell components arrive next.</p>
    </main>
  );
}
```

- [ ] **Step 11: Extend the root `.gitignore`**

Append to `/Users/ranwei/workspace/go_work/hermes-agent-rewrite/hermind/.gitignore`:

```
web/node_modules
web/dist
web/.env.local
.DS_Store
```

- [ ] **Step 12: Extend the root `Makefile`**

Append these targets (keep existing targets intact):

```makefile
.PHONY: web web-install web-dev web-clean

# pnpm install inside web/ — safe to re-run.
web-install:
	@command -v pnpm >/dev/null 2>&1 || { \
	  echo "error: pnpm not found. With Node 20+: corepack enable && corepack prepare pnpm@9 --activate"; \
	  exit 1; \
	}
	cd web && pnpm install --frozen-lockfile || cd web && pnpm install

# Build web bundle then sync into api/webroot/.
# The committed api/webroot/ contents are the source of truth the Go
# server embeds — rebuild + sync after every frontend change.
web: web-install
	cd web && pnpm build
	# Replace api/webroot/ contents with the freshly built dist/.
	# Preserve only the directory itself.
	find api/webroot -mindepth 1 -delete
	cp -R web/dist/. api/webroot/

# Start the Vite dev server on :5173 proxying /api to hermind on :9119.
# Run `hermind web --addr=127.0.0.1:9119` in another terminal.
web-dev: web-install
	@test -f web/.env.local || cp web/.env.example web/.env.local
	cd web && pnpm dev

# Wipe build artefacts. Does NOT touch api/webroot/ (that's committed).
web-clean:
	rm -rf web/node_modules web/dist
```

Also update the existing `.PHONY:` line at the top so it mentions the new targets — actually keep the new `.PHONY` declaration above separate so it's a clean append.

- [ ] **Step 13: Install dependencies + run the first build**

Run, from the worktree root:

```bash
make web
```

Expected flow:
1. `pnpm install` resolves ~200 MB of deps into `web/node_modules/`. Progress messages.
2. `vite build` emits `web/dist/index.html` + `web/dist/assets/index-<hash>.js` + `web/dist/assets/index-<hash>.css`.
3. `find … -delete` empties `api/webroot/`.
4. `cp -R web/dist/. api/webroot/` repopulates it with the build output.

After the command finishes, `ls api/webroot/` should show `index.html` + `assets/`.

- [ ] **Step 14: Smoke-test the build via hermind web**

Launch in one shell:

```bash
go build -o bin/hermind ./cmd/hermind
HOME=/tmp/e2e-home bin/hermind web --addr=127.0.0.1:0 --no-browser --exit-after=30s &
```

Wait a second, scrape the port from stdout, then:

```bash
curl -s http://127.0.0.1:<port>/ | grep -E 'id="root"|hermind'
```

Expected: response HTML contains `<div id="root"></div>` and `window.HERMIND = { token: "<actual-token>" }` (the `{{TOKEN}}` substitution happened). Also verify an asset reference:

```bash
curl -s http://127.0.0.1:<port>/ | grep -oE '/ui/assets/[^"]+'
```

Expected: at least one `/ui/assets/index-<hash>.js` reference. Fetch one and confirm it isn't 404:

```bash
curl -s -o /dev/null -w "%{http_code}\n" http://127.0.0.1:<port>/ui/assets/<paste-a-real-filename-from-previous-step>
```

Expected: `200`.

Kill the background hermind (`kill %1`).

- [ ] **Step 15: Commit**

Stage the new `web/` tree, the rewritten `api/webroot/` artifacts, the Makefile, and the gitignore updates:

```bash
git add web/ api/webroot/ Makefile .gitignore
git commit -m "$(cat <<'EOF'
feat(web): add Vite + React + TypeScript scaffold

Bootstraps a `web/` source tree (package.json, tsconfig, vite.config,
entry point) that `make web` builds into `web/dist/` and syncs into
api/webroot/. The committed api/webroot/ contents are the source of
truth the Go server embeds via //go:embed — rerun `make web` after any
frontend change and commit the updated artifacts.

Token injection keeps working on the built index.html thanks to the
existing {{TOKEN}} template substitution in api/server.go::handleIndex.

Vite dev server runs on :5173 and proxies /api/* to hermind on :9119;
copy web/.env.example → web/.env.local and set VITE_HERMIND_TOKEN for
dev-mode auth.

Stage 3 follow-on tasks replace the placeholder App.tsx with the
TopBar / Sidebar / Footer shell + empty-state Editor.
EOF
)"
```

---

## Task 2: Design tokens + layout CSS grid

**Files:**
- Modify: `web/src/styles/theme.css`

Expands Task 1's minimal theme.css into the full token set that the shell components will reference.

- [ ] **Step 1: Overwrite `web/src/styles/theme.css`**

```css
/* Design tokens — light mode (default). */
:root {
  --bg: #ffffff;
  --surface: #f8fafc;
  --border: #e5e7eb;
  --text: #111827;
  --muted: #6b7280;
  --accent: #FFB800;
  --accent-fg: #111827;
  --focus: rgba(255, 184, 0, .35);
  --success: #22c55e;
  --error: #ef4444;
  --active-tint: rgba(255, 184, 0, .08);
  --hover-tint: rgba(127, 127, 127, .06);

  --font-sans: ui-sans-serif, system-ui, -apple-system, "SF Pro Text",
               "Segoe UI", "PingFang SC", "Microsoft YaHei", sans-serif;
  --font-mono: ui-monospace, "SF Mono", Menlo, "Liberation Mono",
               "Consolas", monospace;
}

/* Dark mode — engaged when the OS prefers dark. */
@media (prefers-color-scheme: dark) {
  :root {
    --bg: #0b0d11;
    --surface: #14171c;
    --border: #2a2f38;
    --text: #e6e8eb;
    --muted: #8892a1;
    --accent: #FFB800;
    --accent-fg: #0b0d11;
    --focus: rgba(255, 184, 0, .5);
    --success: #22c55e;
    --error: #f87171;
    --active-tint: rgba(255, 184, 0, .10);
    --hover-tint: rgba(255, 255, 255, .04);
  }
}

/* Reset + base layout. Three-row, two-column app shell. */
* { box-sizing: border-box; }
html, body { margin: 0; padding: 0; height: 100%; }
body {
  font-family: var(--font-sans);
  font-size: 14px;
  line-height: 1.5;
  color: var(--text);
  background: var(--bg);
}
#root { height: 100%; }

.app-shell {
  display: grid;
  grid-template-columns: 240px 1fr;
  grid-template-rows: 48px 1fr 48px;
  grid-template-areas:
    "top  top"
    "side main"
    "foot foot";
  height: 100vh;
  min-height: 0; /* so main can scroll */
}

.app-shell > header { grid-area: top; }
.app-shell > aside  { grid-area: side; overflow-y: auto; }
.app-shell > main   { grid-area: main; overflow-y: auto; min-width: 0; }
.app-shell > footer { grid-area: foot; }
```

- [ ] **Step 2: Rebuild + smoke-verify**

```bash
make web
```

Expected: build succeeds. No type errors (theme.css is not typed).

- [ ] **Step 3: Commit**

```bash
git add web/src/styles/theme.css api/webroot/
git commit -m "$(cat <<'EOF'
feat(web): flesh out design tokens + app-shell grid

Expands the starter theme.css into the full token set that the
TopBar / Sidebar / Footer components in subsequent tasks consume.
The .app-shell class is a 3-row / 2-column CSS Grid that the root
App component applies — header / sidebar / main / footer land in
named grid areas with no manual positioning math.
EOF
)"
```

---

## Task 3: API client + zod schemas

**Files:**
- Create: `web/src/api/client.ts`
- Create: `web/src/api/schemas.ts`

- [ ] **Step 1: Create `web/src/api/schemas.ts`**

```ts
import { z } from 'zod';

// FieldKind strings produced by gateway/platforms.FieldKind.String().
export const FieldKindSchema = z.enum([
  'unknown', 'string', 'int', 'bool', 'secret', 'enum',
]);
export type FieldKind = z.infer<typeof FieldKindSchema>;

export const SchemaFieldSchema = z.object({
  name: z.string(),
  label: z.string(),
  help: z.string().optional(),
  kind: FieldKindSchema,
  required: z.boolean().optional(),
  default: z.unknown().optional(),
  enum: z.array(z.string()).optional(),
});
export type SchemaField = z.infer<typeof SchemaFieldSchema>;

export const SchemaDescriptorSchema = z.object({
  type: z.string(),
  display_name: z.string(),
  summary: z.string().optional(),
  fields: z.array(SchemaFieldSchema),
});
export type SchemaDescriptor = z.infer<typeof SchemaDescriptorSchema>;

export const PlatformsSchemaResponseSchema = z.object({
  descriptors: z.array(SchemaDescriptorSchema),
});
export type PlatformsSchemaResponse = z.infer<typeof PlatformsSchemaResponseSchema>;

// Config is shaped like the backend Config struct, but we only model
// the gateway.platforms subtree explicitly — everything else is kept
// as-is in the draft object so PUT round-trips the unknown fields.
export const PlatformInstanceSchema = z.object({
  enabled: z.boolean().optional(),
  type: z.string(),
  options: z.record(z.string(), z.string()).optional(),
});
export type PlatformInstance = z.infer<typeof PlatformInstanceSchema>;

export const ConfigSchema = z.object({
  gateway: z.object({
    platforms: z.record(z.string(), PlatformInstanceSchema).optional(),
  }).optional(),
}).catchall(z.unknown());
export type Config = z.infer<typeof ConfigSchema>;

export const ConfigResponseSchema = z.object({ config: ConfigSchema });

export const ApplyResultSchema = z.object({
  ok: z.boolean(),
  restarted: z.array(z.string()).optional(),
  errors: z.record(z.string(), z.string()).optional(),
  took_ms: z.number(),
  error: z.string().optional(),
});
export type ApplyResult = z.infer<typeof ApplyResultSchema>;
```

- [ ] **Step 2: Create `web/src/api/client.ts`**

```ts
import { z } from 'zod';

/**
 * Token resolution order:
 *  1. VITE_HERMIND_TOKEN env (Vite dev mode, via web/.env.local).
 *  2. window.HERMIND.token (prod — injected by api/server.go::handleIndex).
 */
export function resolveToken(): string {
  const envTok = import.meta.env.VITE_HERMIND_TOKEN as string | undefined;
  if (envTok && envTok.length > 0) return envTok;
  const globalTok = (window as unknown as { HERMIND?: { token?: string } }).HERMIND?.token;
  if (globalTok && globalTok.length > 0 && globalTok !== '{{TOKEN}}') {
    return globalTok;
  }
  return '';
}

/** Thrown for any non-2xx response; carries the decoded JSON error if present. */
export class ApiError extends Error {
  constructor(public status: number, public body: unknown) {
    super(`api: ${status}`);
  }
}

/**
 * apiFetch sends a JSON request to hermind with the Bearer token
 * automatically attached. The response is parsed as JSON and passed
 * through the optional zod schema — callers get a typed value or a
 * thrown ApiError. 401/403 responses throw ApiError with status set
 * so the caller can surface a "token invalid" banner.
 */
export async function apiFetch<T>(
  path: string,
  opts: {
    method?: string;
    body?: unknown;
    schema?: z.ZodType<T>;
    signal?: AbortSignal;
  } = {},
): Promise<T> {
  const token = resolveToken();
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
  };
  if (token) headers['Authorization'] = `Bearer ${token}`;

  const res = await fetch(path, {
    method: opts.method ?? 'GET',
    headers,
    body: opts.body !== undefined ? JSON.stringify(opts.body) : undefined,
    signal: opts.signal,
  });

  const ctype = res.headers.get('content-type') ?? '';
  const parsed = ctype.includes('application/json') ? await res.json() : await res.text();

  if (!res.ok) {
    throw new ApiError(res.status, parsed);
  }

  if (opts.schema) {
    return opts.schema.parse(parsed);
  }
  return parsed as T;
}
```

- [ ] **Step 3: Build + type-check**

```bash
cd web && pnpm type-check
```

Expected: exit 0, no type errors.

- [ ] **Step 4: Commit**

```bash
git add web/src/api/
git commit -m "$(cat <<'EOF'
feat(web/api): apiFetch client + zod schemas

Thin Bearer-authenticated fetch wrapper (resolveToken picks from
VITE_HERMIND_TOKEN in dev, window.HERMIND.token in prod). zod
schemas mirror the backend DTO shapes so GET responses are
validated at the boundary and give typed values downstream.
ApiError carries status + body for the 401 / 404 / 409 mapping the
UI will eventually surface.
EOF
)"
```

---

## Task 4: AppState reducer + data-fetching lifecycle

**Files:**
- Create: `web/src/state.ts`
- Modify: `web/src/App.tsx`

- [ ] **Step 1: Create `web/src/state.ts`**

```ts
import type { Config, SchemaDescriptor } from './api/schemas';

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
  | { type: 'apply/done'; error?: string };

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
        : { ...state, status: 'ready', flash: { kind: 'ok', msg: 'Saved.' } };
    case 'apply/start':
      return { ...state, status: 'applying', flash: null };
    case 'apply/done':
      return action.error
        ? { ...state, status: 'ready', flash: { kind: 'err', msg: action.error } }
        : { ...state, status: 'ready', flash: { kind: 'ok', msg: 'Applied.' } };
  }
}

/** listInstances returns keys in the current config.gateway.platforms map, sorted. */
export function listInstances(state: AppState): string[] {
  const plats = state.config.gateway?.platforms ?? {};
  return Object.keys(plats).sort();
}
```

- [ ] **Step 2: Overwrite `web/src/App.tsx`**

```tsx
import { useEffect, useReducer } from 'react';
import { apiFetch } from './api/client';
import {
  ConfigResponseSchema,
  PlatformsSchemaResponseSchema,
} from './api/schemas';
import { initialState, reducer } from './state';

export default function App() {
  const [state, dispatch] = useReducer(reducer, initialState);

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
  return (
    <div>
      <p style={{ padding: '2rem' }}>
        Boot ok — {state.descriptors.length} descriptors,{' '}
        {Object.keys(state.config.gateway?.platforms ?? {}).length} instances.
      </p>
    </div>
  );
}
```

- [ ] **Step 3: Build**

```bash
make web
```

Expected: build succeeds, no type errors.

- [ ] **Step 4: Smoke-test against a running hermind web**

```bash
go build -o bin/hermind ./cmd/hermind
HOME=/tmp/e2e-home bin/hermind web --addr=127.0.0.1:9119 --no-browser --exit-after=60s &
sleep 2
curl -s http://127.0.0.1:9119/ > /tmp/index.html
TOK=$(grep -oE 'token: "[^"]+"' /tmp/hermind-e2e.log | sed 's/token: "\(.*\)"/\1/' | head -1)
# If the above log parsing is fragile, read the token directly from the
# server stdout or from your earlier e2e home config.
```

In a browser, open `http://127.0.0.1:9119/?t=<token>` — the page should render "Boot ok — 19 descriptors, 0 instances." (or whatever count the current config has).

If the browser shows a blank page or errors in the devtools Network tab, inspect:

- `/api/platforms/schema` response — should be a 200 JSON with 19 descriptors.
- `/api/config` response — should be a 200 JSON with the full config.

Kill the background hermind.

- [ ] **Step 5: Commit**

```bash
git add web/src/state.ts web/src/App.tsx api/webroot/
git commit -m "$(cat <<'EOF'
feat(web): AppState reducer + boot-time data load

Single useReducer owns the whole app state (descriptors + config +
selectedKey + flash). On mount, App fetches /api/platforms/schema and
/api/config in parallel, parses them via zod, and transitions
booting → ready. Boot errors render an inline error message; success
currently shows a placeholder counter — Task 5 replaces the body with
the real shell layout.
EOF
)"
```

---

## Task 5: Shell components (TopBar + Sidebar + Footer)

**Files:**
- Create: `web/src/components/TopBar.tsx`, `TopBar.module.css`
- Create: `web/src/components/Sidebar.tsx`, `Sidebar.module.css`
- Create: `web/src/components/Footer.tsx`, `Footer.module.css`
- Modify: `web/src/App.tsx`

- [ ] **Step 1: Create `web/src/components/TopBar.tsx`**

```tsx
import styles from './TopBar.module.css';

export interface TopBarProps {
  dirtyCount: number;
  status: 'booting' | 'ready' | 'saving' | 'applying' | 'error';
}

export default function TopBar({ dirtyCount, status }: TopBarProps) {
  const dotClass =
    status === 'saving' || status === 'applying'
      ? styles.dotBusy
      : dirtyCount > 0
        ? styles.dotDirty
        : styles.dotIdle;
  const msg =
    status === 'saving'
      ? 'Saving…'
      : status === 'applying'
        ? 'Applying…'
        : dirtyCount > 0
          ? `${dirtyCount} unsaved change${dirtyCount === 1 ? '' : 's'}`
          : 'All saved';
  return (
    <header className={styles.topbar}>
      <div className={styles.brand}>
        <span className={styles.logo}>⬡</span>
        <span className={styles.title}>hermind</span>
      </div>
      <span className={styles.spacer} />
      <span className={styles.status}>
        <span className={dotClass} />
        {msg}
      </span>
    </header>
  );
}
```

- [ ] **Step 2: Create `web/src/components/TopBar.module.css`**

```css
.topbar {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 0 16px;
  border-bottom: 1px solid var(--border);
  background: var(--bg);
  height: 48px;
}
.brand { display: flex; align-items: center; gap: 8px; }
.logo  { color: var(--accent); font-size: 16px; line-height: 1; }
.title { font-size: 15px; font-weight: 500; }
.spacer { flex: 1; }
.status {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  color: var(--muted);
}
.dotIdle, .dotDirty, .dotBusy {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  display: inline-block;
}
.dotIdle  { background: transparent; border: 1px solid var(--border); }
.dotDirty { background: var(--accent); }
.dotBusy  { background: var(--muted); animation: pulse 1s infinite; }
@keyframes pulse {
  0%, 100% { opacity: 1; }
  50%      { opacity: 0.4; }
}
```

- [ ] **Step 3: Create `web/src/components/Sidebar.tsx`**

```tsx
import styles from './Sidebar.module.css';
import type { SchemaDescriptor } from '../api/schemas';

export interface SidebarProps {
  instances: Array<{ key: string; type: string; enabled: boolean }>;
  selectedKey: string | null;
  descriptors: SchemaDescriptor[];
  onSelect: (key: string) => void;
  onNewInstance: () => void;
}

export default function Sidebar({
  instances,
  selectedKey,
  descriptors,
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
          <span className={styles.itemKey}>{inst.key}</span>
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

- [ ] **Step 4: Create `web/src/components/Sidebar.module.css`**

```css
.sidebar {
  background: var(--surface);
  border-right: 1px solid var(--border);
  padding: 12px 8px;
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.label {
  font-size: 11px;
  text-transform: uppercase;
  color: var(--muted);
  padding: 6px 10px;
  letter-spacing: 0.5px;
}
.empty {
  padding: 10px 12px;
  font-size: 13px;
  color: var(--muted);
  font-style: italic;
}
.item {
  appearance: none;
  background: transparent;
  border: 0;
  text-align: left;
  font: inherit;
  color: var(--text);
  padding: 10px 12px;
  border-radius: 6px;
  cursor: pointer;
  display: flex;
  flex-direction: column;
  gap: 2px;
  position: relative;
}
.item:hover { background: var(--hover-tint); }
.active {
  background: var(--active-tint);
  font-weight: 500;
}
.active::before {
  content: "";
  position: absolute;
  left: 0;
  top: 8px;
  bottom: 8px;
  width: 2px;
  background: var(--accent);
  border-radius: 2px;
}
.dimmed { opacity: 0.6; }
.itemKey  { font-size: 14px; }
.itemType { font-size: 12px; color: var(--muted); display: inline-flex; align-items: center; gap: 6px; }
.offBadge {
  font-size: 10px;
  border: 1px solid var(--border);
  border-radius: 3px;
  padding: 1px 4px;
}
.newBtn {
  appearance: none;
  background: transparent;
  color: var(--accent);
  border: 1px dashed var(--border);
  border-radius: 6px;
  padding: 8px 12px;
  font: inherit;
  font-size: 13px;
  cursor: pointer;
  margin-top: 8px;
}
.newBtn:hover { background: var(--hover-tint); }
```

- [ ] **Step 5: Create `web/src/components/Footer.tsx`**

```tsx
import styles from './Footer.module.css';
import type { Flash } from '../state';

export interface FooterProps {
  dirtyCount: number;
  flash: Flash | null;
  busy: boolean;
  onSave: () => void;
  onSaveAndApply: () => void;
}

export default function Footer({
  dirtyCount,
  flash,
  busy,
  onSave,
  onSaveAndApply,
}: FooterProps) {
  const flashClass =
    flash?.kind === 'err' ? styles.flashErr : flash?.kind === 'ok' ? styles.flashOk : '';
  const label = flash?.msg ?? (dirtyCount > 0 ? `${dirtyCount} unsaved` : '');
  return (
    <footer className={styles.footer}>
      <span className={`${styles.status} ${flashClass}`}>{label}</span>
      <span className={styles.spacer} />
      <button
        type="button"
        className={`${styles.btn} ${styles.secondary}`}
        onClick={onSave}
        disabled={busy || dirtyCount === 0}
      >
        Save
      </button>
      <button
        type="button"
        className={`${styles.btn} ${styles.primary}`}
        onClick={onSaveAndApply}
        disabled={busy || dirtyCount === 0}
      >
        Save &amp; Apply
      </button>
    </footer>
  );
}
```

- [ ] **Step 6: Create `web/src/components/Footer.module.css`**

```css
.footer {
  display: flex;
  align-items: center;
  padding: 0 16px;
  background: var(--surface);
  border-top: 1px solid var(--border);
  gap: 12px;
  height: 48px;
}
.status {
  font-size: 13px;
  color: var(--text);
}
.flashOk  { color: var(--success); }
.flashErr { color: var(--error); }
.spacer { flex: 1; }
.btn {
  height: 32px;
  padding: 0 14px;
  font-size: 14px;
  font-family: inherit;
  border-radius: 6px;
  cursor: pointer;
  transition: filter 120ms ease;
}
.btn:disabled { cursor: not-allowed; opacity: 0.5; }
.secondary {
  background: transparent;
  color: var(--text);
  border: 1px solid var(--border);
}
.secondary:hover:not(:disabled) { background: var(--hover-tint); }
.primary {
  background: var(--accent);
  color: var(--accent-fg);
  border: 0;
  font-weight: 500;
}
.primary:hover:not(:disabled) { filter: brightness(0.95); }
```

- [ ] **Step 7: Update `web/src/App.tsx` to render the shell**

Replace the previous App.tsx with:

```tsx
import { useEffect, useMemo, useReducer } from 'react';
import { apiFetch } from './api/client';
import {
  ConfigResponseSchema,
  PlatformsSchemaResponseSchema,
} from './api/schemas';
import { initialState, listInstances, reducer } from './state';
import TopBar from './components/TopBar';
import Sidebar from './components/Sidebar';
import Footer from './components/Footer';

export default function App() {
  const [state, dispatch] = useReducer(reducer, initialState);

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

  const instances = useMemo(() => {
    const plats = state.config.gateway?.platforms ?? {};
    return listInstances(state).map(key => ({
      key,
      type: plats[key]?.type ?? '',
      enabled: plats[key]?.enabled ?? false,
    }));
  }, [state]);

  // Dirty count: Stage 3 doesn't yet have field-level edits, so this
  // is a placeholder 0. Stage 4 computes a real structural diff.
  const dirtyCount = 0;
  const busy = state.status === 'saving' || state.status === 'applying';

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

  return (
    <div className="app-shell">
      <TopBar dirtyCount={dirtyCount} status={state.status} />
      <Sidebar
        instances={instances}
        selectedKey={state.selectedKey}
        descriptors={state.descriptors}
        onSelect={key => dispatch({ type: 'select', key })}
        onNewInstance={() => console.log('TODO: new instance (Stage 4)')}
      />
      <main>
        {state.selectedKey
          ? <div style={{ padding: '2rem' }}>Editor for {state.selectedKey} — fields land in Stage 4.</div>
          : <div style={{ padding: '2rem', color: 'var(--muted)' }}>
              Select an instance from the sidebar, or click + New instance.
            </div>}
      </main>
      <Footer
        dirtyCount={dirtyCount}
        flash={state.flash}
        busy={busy}
        onSave={() => console.log('TODO: save (Stage 4)')}
        onSaveAndApply={() => console.log('TODO: save & apply (Stage 4)')}
      />
    </div>
  );
}
```

- [ ] **Step 8: Build + type-check**

```bash
make web
```

Expected: clean build. `api/webroot/` now holds the new bundle.

- [ ] **Step 9: Smoke-test in a browser**

Same routine as Task 4. Open `http://127.0.0.1:9119/?t=<token>`. Expected visual:

1. 48 px topbar with the `⬡ hermind` brand on the left and "All saved" on the right (dot is transparent/bordered).
2. 240 px sidebar with "Messaging Platforms" label and "No instances configured." (or the keys from your config).
3. Main pane says "Select an instance from the sidebar, or click + New instance."
4. 48 px footer at the bottom with disabled Save / Save & Apply buttons (dirtyCount is 0).
5. Light mode by default, dark mode if your OS prefers dark.

If the page is unstyled or layout is broken, inspect whether `theme.css` is loaded: the devtools Sources panel should show `main-<hash>.css` under `/ui/assets/`.

- [ ] **Step 10: Commit**

```bash
git add web/src/ api/webroot/
git commit -m "$(cat <<'EOF'
feat(web): TopBar + Sidebar + Footer shell components

Replaces the placeholder App body with a full three-row / two-column
grid shell driven by design tokens. Sidebar lists config.gateway.platforms
entries in sort order with selection state and a "New instance" stub
button. Footer has Save / Save & Apply buttons that are currently
disabled (dirtyCount is a placeholder 0 until Stage 4 computes a real
diff). TopBar shows a status dot that reflects saving / applying /
idle states wired through the reducer.
EOF
)"
```

---

## Task 6: Empty Editor component

**Files:**
- Create: `web/src/components/Editor.tsx`, `Editor.module.css`
- Modify: `web/src/App.tsx`

Moves the inline main-pane JSX in App.tsx into a dedicated `Editor` component, with a proper empty-state panel styled to match the Stage 4 wireframe (Editor will grow to include field list + Test button).

- [ ] **Step 1: Create `web/src/components/Editor.tsx`**

```tsx
import styles from './Editor.module.css';
import type { PlatformInstance, SchemaDescriptor } from '../api/schemas';

export interface EditorProps {
  selectedKey: string | null;
  instance: PlatformInstance | null;
  descriptor: SchemaDescriptor | null;
}

export default function Editor({ selectedKey, instance, descriptor }: EditorProps) {
  if (!selectedKey || !instance) {
    return (
      <div className={styles.wrapper}>
        <div className={styles.emptyCard}>
          <h2 className={styles.emptyTitle}>No instance selected</h2>
          <p className={styles.emptyBody}>
            Pick an instance from the sidebar, or click <em>+ New instance</em>
            to create one. Field editors land in Stage 4.
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
        </header>
        {descriptor.summary && (
          <p className={styles.summary}>{descriptor.summary}</p>
        )}
        <div className={styles.stagePlaceholder}>
          Field editors land in Stage 4 —
          this descriptor has {descriptor.fields.length} field
          {descriptor.fields.length === 1 ? '' : 's'} to render.
        </div>
      </section>
    </div>
  );
}
```

- [ ] **Step 2: Create `web/src/components/Editor.module.css`**

```css
.wrapper {
  padding: 32px 40px;
  background: var(--bg);
}
.panel {
  max-width: 720px;
  margin: 0 auto;
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 24px;
  background: var(--bg);
}
.panelHeader {
  display: flex;
  align-items: baseline;
  gap: 12px;
  margin-bottom: 12px;
}
.title {
  font-size: 17px;
  font-weight: 500;
  margin: 0;
}
.typeTag {
  font-size: 12px;
  color: var(--muted);
  font-family: var(--font-mono);
}
.summary {
  color: var(--muted);
  font-size: 13px;
  margin: 0 0 20px;
}
.stagePlaceholder {
  padding: 16px;
  border: 1px dashed var(--border);
  border-radius: 6px;
  background: var(--surface);
  color: var(--muted);
  font-size: 13px;
}

.emptyCard {
  max-width: 520px;
  margin: 64px auto;
  padding: 24px;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--bg);
  text-align: center;
}
.emptyTitle {
  margin: 0 0 8px;
  font-size: 16px;
  font-weight: 500;
}
.emptyBody {
  margin: 0;
  color: var(--muted);
  font-size: 13px;
}
```

- [ ] **Step 3: Update `web/src/App.tsx` to use `Editor`**

Replace the `<main>` block with:

```tsx
      <main>
        <Editor
          selectedKey={state.selectedKey}
          instance={
            state.selectedKey
              ? state.config.gateway?.platforms?.[state.selectedKey] ?? null
              : null
          }
          descriptor={
            state.selectedKey && state.config.gateway?.platforms?.[state.selectedKey]
              ? state.descriptors.find(
                  d => d.type === state.config.gateway!.platforms![state.selectedKey!]!.type,
                ) ?? null
              : null
          }
        />
      </main>
```

Also add the import near the top of `App.tsx`:

```tsx
import Editor from './components/Editor';
```

- [ ] **Step 4: Build**

```bash
make web
```

Expected: clean.

- [ ] **Step 5: Smoke-test**

1. With no instances → Main pane shows an empty-state card centered in the main area.
2. Seed one via curl PUT (use the Stage 2 smoke flow) and refresh — Sidebar should show the instance, clicking it should swap Main to a bordered panel with the instance key + type display name.

- [ ] **Step 6: Commit**

```bash
git add web/src/components/Editor.tsx web/src/components/Editor.module.css web/src/App.tsx api/webroot/
git commit -m "$(cat <<'EOF'
feat(web): empty-state Editor component

Moves the main-pane rendering into a dedicated Editor component that
handles three states: no-selection (centered empty card), unknown-type
(warning message), and selected-valid (panel with header + placeholder
stage). The panel's shape anticipates Stage 4's FieldList + Test
button landing in the .stagePlaceholder slot.
EOF
)"
```

---

## Task 7: Final verification

No code changes.

- [ ] **Step 1: Full build from scratch**

```bash
(cd <worktree> && make web-clean && make web)
```

Expected: install + build succeed, `api/webroot/` ends with `index.html` + `assets/`.

- [ ] **Step 2: Go build + test**

```bash
(cd <worktree> && go build ./... && go test ./api/... ./cli/... ./gateway/...)
```

Expected: clean. The embed picks up the new assets.

- [ ] **Step 3: Manual browser walkthrough**

Follow the Stage 2 E2E recipe to launch hermind web. Load `http://127.0.0.1:9119/?t=<token>` and confirm:

1. TopBar renders with brand + status chip.
2. Sidebar shows "No instances configured." for an empty config (or the seeded keys).
3. Main pane shows the empty-state card.
4. Footer has both buttons disabled (dirtyCount 0).
5. Light / dark swap on OS theme change.
6. Seeding a telegram instance via `curl -X PUT /api/config … ` and refreshing the page shows the instance in the sidebar. Clicking it swaps the main pane to the placeholder panel.

- [ ] **Step 4: Commit history sanity**

```bash
(cd <worktree> && git log --oneline <stage-3-base>..HEAD)
```

Expected: 6 feat commits (one per task), no stray commits outside `web/` + `Makefile` + `.gitignore` + `api/webroot/`.

---

## Rollback

`git reset --hard <commit-before-task-1>` on the feature branch. The placeholder `api/webroot/index.html` returns via the reset; no on-disk state elsewhere.

## Known scope cuts (for Stage 4)

1. **Save / Save & Apply buttons are log-only.** Real PUT /api/config + POST /api/platforms/apply wiring lands in Stage 4 once dirty-tracking is in place.
2. **No dirty detection.** Without a structural diff against `originalConfig`, the footer always shows 0 unsaved changes; `Editor` can't mark specific instances as modified yet.
3. **No FieldList, SecretInput, TestConnection, NewInstanceDialog.** Editor shows the stage-placeholder panel only.
4. **No test infrastructure.** Stage 5 adds vitest + the `make web` CI gate; for now we rely on `tsc --noEmit` + manual browser smoke.
5. **Hash in asset filenames forces frequent api/webroot/ diffs.** Every code change flips the hash, so expect larger diffs in the committed artifacts. Stage 5 can revisit whether to move the build into CI to avoid committing these.
