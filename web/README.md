# hermind web

React + TypeScript frontend for `hermind web` (the REST-backed config UI).
The built bundle lives in `api/webroot/` and is embedded into the Go
binary via `//go:embed` — `api/webroot/` is the source of truth the
server serves, not `web/dist/`.

## Dev loop

```bash
# one-time: make sure pnpm is available via Corepack (ships with Node 20+)
corepack enable && corepack prepare pnpm@10.29.3 --activate

# install deps
cd web && pnpm install

# copy the env template and fill in a token from `hermind web` stdout
cp .env.example .env.local
echo "VITE_HERMIND_TOKEN=paste-the-token-here" > .env.local

# in one terminal: run hermind web on :9119
hermind web --addr=127.0.0.1:9119 --no-browser

# in another: run the Vite dev server on :5173, proxying /api → :9119
cd web && pnpm dev
```

Open <http://localhost:5173>. Edit any file under `web/src/` — Vite
hot-reloads.

## Build + deploy loop

```bash
# from repo root
make web
```

This runs `pnpm install` + `pnpm build`, then copies `web/dist/`
into `api/webroot/`. Commit **both** the source change under
`web/src/` and the rebuilt `api/webroot/` files — CI rejects PRs
where the two are out of sync.

## Scripts

| Command           | What it does                                     |
|-------------------|--------------------------------------------------|
| `pnpm dev`        | Vite dev server on :5173 with /api proxy         |
| `pnpm build`      | tsc --noEmit + vite build into `dist/`           |
| `pnpm type-check` | tsc --noEmit (no build)                          |
| `pnpm test`       | vitest run (one-shot)                            |
| `pnpm test:watch` | vitest in watch mode                             |
| `pnpm lint`       | ESLint over `src/**/*.{ts,tsx}`                  |

From repo root:

| Command          | What it does                                                      |
|------------------|-------------------------------------------------------------------|
| `make web`       | `pnpm install` + build + sync `web/dist/` → `api/webroot/`        |
| `make web-dev`   | Vite dev server (auto-copies `.env.example` → `.env.local`)       |
| `make web-test`  | vitest run                                                        |
| `make web-lint`  | ESLint                                                            |
| `make web-check` | The composite gate CI runs — install → type-check → test → lint → build → sync assertion |
| `make web-clean` | Remove `web/node_modules` + `web/dist` (leaves `api/webroot/`)    |

## Env vars

| Name                  | Scope          | Notes                                                      |
|-----------------------|----------------|------------------------------------------------------------|
| `VITE_HERMIND_TOKEN`  | dev only       | Read by `web/.env.local`. Overrides `window.HERMIND.token`.|
| `window.HERMIND.token`| production     | Injected by `api/server.go::handleIndex` template sub.     |

## Layout

```
web/
├── src/
│   ├── api/          fetch wrapper + zod schemas for every REST DTO
│   ├── components/   TopBar / Sidebar / Footer / Editor / Dialog / Fields
│   ├── styles/       global theme tokens + app-shell grid
│   ├── App.tsx       root component: reducer, boot fetch, routing
│   ├── state.ts      AppState, Actions, reducer, selectors
│   └── main.tsx      React mount
├── eslint.config.js  flat-config lint rules
├── vitest.config.ts  jsdom + include src/**/*.test.{ts,tsx}
├── vite.config.ts    base=/ui/, /api proxy for dev, dist/ output
├── package.json      devDeps only — React + zod are the sole runtime deps
└── tsconfig*.json    strict TS, ES2022, JSX react-jsx
```

## Troubleshooting

- **"Boot failed: HTTP 401" screen on first load** — the token isn't
  making it through. In dev: ensure `VITE_HERMIND_TOKEN` matches the
  token `hermind web` prints on stdout. In prod: the `{{TOKEN}}`
  substitution in `api/server.go::handleIndex` should fill it — if
  that literal is still on the page, the Go server didn't template
  the response.

- **"unknown platform key" on Show** — the instance isn't on disk
  yet. Save first; then the Show button will fire `/reveal`.

- **api/webroot/ diff doesn't match CI** — run `make web` locally
  before pushing. The `make web-check` target is the same gate CI
  runs, but catches it 30 seconds earlier.
