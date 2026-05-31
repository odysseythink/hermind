# Hermind — Agent Guide

> This file is written for AI coding agents. Assume the reader knows nothing about the project.

## Project Overview

Hermind is a RAG (Retrieval-Augmented Generation) and AI-agent platform. It provides a web UI and API for managing workspaces, documents, chat threads, and autonomous AI agents that can use tools, search knowledge bases, and execute workflows.

The project is a **Go rewrite** of a Node.js/Express backend. The frontend React application was preserved with zero changes; only the backend was replaced. The API contract remains 100 % compatible with the original Node backend so the frontend build can be dropped in unchanged.

### High-level Architecture

- **Backend**: Go 1.26 + Gin web framework
- **Frontend**: React 18 + Vite (embedded into the Go binary as static files)
- **Database**: SQLite (default) or PostgreSQL via GORM
- **Vector DBs**: LanceDB (default), PGVector, Pinecone, Chroma, Weaviate, Qdrant, Milvus, Zilliz, Astra
- **LLM / Embedding / Agent SDK**: Pantheon (`github.com/odysseythink/pantheon`)
- **Logging**: mlog (`github.com/odysseythink/mlog`) — structured JSON, glog-compatible

The server is a single binary (`hermind`) that serves the React SPA on root paths and REST API under `/api`.

## Directory Layout

```
hermind/
├── backend/
│   ├── cmd/server/
│   │   ├── main.go              # Entry point: load config, init DB, start Gin, register routes
│   │   └── frontend_embed.go    # //go:embed for the built frontend dist
│   ├── internal/
│   │   ├── agent/               # Agent runtime, WebSocket sessions, tool calling, flows
│   │   ├── chunker/             # Text chunking for RAG
│   │   ├── collector/           # Document collection / ingestion
│   │   ├── config/              # Environment-variable based configuration
│   │   ├── dto/                 # Request/response DTOs
│   │   ├── embedder/            # Embedding provider abstraction
│   │   ├── handlers/            # HTTP handlers (REST + OpenAI-compatible API)
│   │   ├── mcp/                 # MCP (Model Context Protocol) hypervisor
│   │   ├── middleware/          # Auth, CORS, workspace validation
│   │   ├── models/              # GORM models (replicas of original Prisma schema)
│   │   ├── providers/           # LLM provider abstraction via Pantheon
│   │   ├── reranker/            # Result reranking (Cohere, etc.)
│   │   ├── services/            # Business logic layer
│   │   ├── tts/                 # Text-to-speech providers
│   │   ├── vectordb/            # Vector database abstraction
│   │   └── workers/             # Background job framework (cron-based)
│   ├── pkg/utils/               # Shared utilities (encryption, logging helpers)
│   ├── storage/                 # Default SQLite DB, encryption keys, logs
│   └── tests/integration/       # Integration test suite
├── frontend/
│   ├── src/
│   │   ├── components/          # Reusable React components
│   │   ├── pages/               # Top-level page views
│   │   ├── hooks/               # Custom React hooks
│   │   ├── locales/             # i18n translations
│   │   ├── models/              # Frontend data models
│   │   └── utils/               # Frontend utilities
│   ├── vite.config.js           # Vite configuration
│   ├── eslint.config.js         # ESLint flat config
│   └── package.json             # Yarn-based dependencies
├── .github/workflows/           # CI: Go tests, lint, goreleaser, desktop builds
└── .gpowers/                    # Design docs & ADRs (not shipped)
```

## Technology Stack

| Layer | Technology |
|-------|------------|
| Language | Go 1.26 |
| Web Framework | Gin |
| ORM | GORM (drivers: sqlite, postgres) |
| AI SDK | Pantheon (LLM + embedding + agent + tool calling) |
| Vector DBs | LanceDB, PGVector, Pinecone, Chroma, Weaviate, Qdrant, Milvus, Zilliz, Astra |
| LLM Providers | OpenAI, Azure, Anthropic, Gemini, Ollama, LMStudio, LocalAI, TogetherAI, Fireworks, and more |
| Auth | JWT (cookie / Bearer), API keys, OAuth (Outlook) |
| Scheduler | robfig/cron/v3 |
| WebSocket | Gorilla WebSocket |
| Frontend | React 18, Vite, TailwindCSS |
| Package Manager (frontend) | Yarn |
| Logging | mlog |

## Build and Run

All build commands are run from `backend/`:

```bash
cd backend

# Development server (logs to stderr, no file logging)
make dev

# Build frontend + Go server binary (output: ../hermind)
make build

# Run all Go tests
make test

# Lint Go code
make lint
```

### Important Build Details

- The Go build **requires** the build tag `fts5` because SQLite full-text search is used:
  ```bash
  go build -tags="fts5" -o ../hermind ./cmd/server/
  ```
- `make build` first builds the frontend (`yarn install && yarn build`), copies `frontend/dist` into `backend/cmd/server/frontend/dist`, renames `_index.html` to `index.html`, then compiles the Go binary with the embedded filesystem.
- The default server port is **3001**. The frontend dev server runs on **3000**.
- Default storage directory is `./storage` relative to the working directory.

### Frontend Development (standalone)

```bash
cd frontend
yarn install
yarn dev      # Vite dev server on localhost:3000
yarn build    # Production build
yarn lint     # ESLint with auto-fix
```

### Browser Extension Development

```bash
# Build the browser extension (React + Vite)
make build-extension

# Build everything (frontend + server + extension)
make build-all
```

## Configuration

Configuration is driven entirely by environment variables. See `backend/internal/config/config.go` for the full struct. Key variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `SERVER_PORT` | `3001` | HTTP server port |
| `STORAGE_DIR` | `./storage` | Local data directory (DB, keys, logs) |
| `JWT_SECRET` | `dev-secret-change-me` | JWT signing secret |
| `DATABASE_URL` | *(empty)* | PostgreSQL DSN; if empty, SQLite is used |
| `VECTOR_DB` | `lancedb` | Vector database backend |
| `LLM_PROVIDER` | `openai` | Default LLM provider |
| `LLM_MODEL` | `gpt-4o-mini` | Default LLM model |
| `EMBEDDING_ENGINE` | `openai` | Default embedding provider |
| `DEBUG_MODE` | `false` | Enable Gin debug mode |
| `MULTI_USER_MODE` | `false` | Enable multi-user auth |

## Code Organization Rules

### Backend (`backend/internal/`)

- **handlers/** — HTTP handlers map 1:1 to route groups. Each file registers its own routes via a `Register*Routes` function. There are two auth planes:
  1. **Web UI auth** (cookie/session JWT) — uses `middleware.ValidatedRequest`
  2. **API key auth** — used by `/api/v1/*` and OpenAI-compatible endpoints
- **services/** — Business logic. Services accept `*gorm.DB` and `*config.Config` and are constructed in `main.go`. They should not depend on HTTP context directly.
- **models/** — GORM structs. Migrations are handled automatically by `services.AutoMigrate` on boot.
- **agent/** — The agent runtime owns WebSocket sessions, tool invocation, approval workflows, and the flow executor. It depends on Pantheon's `core.LanguageModel`.
- **mcp/** — The MCP hypervisor manages external MCP servers (stdio / HTTP / SSE transports), discovers tools, and routes tool calls with concurrency limits.
- **vectordb/** — Pure interface + implementations. `vectordb.VectorDatabase` is the contract. Add a new vector DB by implementing this interface and wiring it in `main.go`.
- **providers/** — LLM provider builders registered in a `providerRegistry` map. Pantheon is the underlying SDK.

### Frontend (`frontend/src/`)

- Pages live in `src/pages/`. Routing is handled by `react-router-dom`.
- Components live in `src/components/`.
- State/context providers are co-located with components (e.g., `AuthContext`, `ThemeProvider`).
- API calls are made directly to `/api/*` endpoints. In production the Go binary serves the SPA and proxies API requests internally.

## Testing Strategy

### Go Tests

- **Unit tests** are co-located: `*_test.go` files sit next to the code they test.
- **Integration tests** live in `backend/tests/integration/` and use a real HTTP server + SQLite test database.
- Run all tests:
  ```bash
  cd backend && go test -v ./...
  ```
- CI runs with the race detector:
  ```bash
  go test -race -cover ./...
  ```

### Frontend Tests

- There is currently no frontend test suite. The project relies on ESLint, Prettier, and the Vite build for correctness.

### CI / GitHub Actions

- **test** workflow — runs on `push`/`pull_request` to `main`:
  - Go build + test with race detector + `golangci-lint`
- **release** workflow — triggered on `v*` tags:
  - Runs tests then invokes `goreleaser`
- **Desktop Build** workflow — triggered on changes to `desktop/`, `api/`, `cmd/`:
  - Builds Qt desktop wrappers for macOS and Windows

## Code Style Guidelines

### Go

- Standard `gofmt` formatting.
- Use `golangci-lint` for linting (enforced in CI).
- Prefer explicit error handling; do not swallow errors.
- Use `mlog` for structured logging (not `log` or `fmt`).
- Context propagation: pass `context.Context` as the first argument to service methods.
- Pointer receivers for services and large structs.
- Tests should clean up after themselves; use `t.Cleanup` for temp resources.

### React / JavaScript

- ESLint flat config is in `frontend/eslint.config.js`.
- Prettier formatting is enforced as an ESLint rule (`prettier/prettier: error`).
- Unused imports are treated as errors (`unused-imports/no-unused-imports: error`).
- `react/react-in-jsx-scope` is off (React 18 automatic runtime).
- `react-hooks/exhaustive-deps` is off by design decision.

## Security Considerations

- **Encryption at rest**: Sensitive settings (OAuth secrets, API keys) are encrypted with AES-GCM. The AES key is derived from an RSA private key stored in `<storageDir>/encryption/private.pem`. If the key file is lost, encrypted secrets cannot be recovered.
- **JWT secrets**: `JWT_SECRET`, `SIG_KEY`, and `SIG_SALT` must be changed from defaults in production.
- **Auth modes**:
  - Single-user mode with no password bypasses auth entirely (`ValidatedRequest` sets an admin user automatically).
  - Multi-user mode enforces JWT validation.
- **Agent tool approval**: There is a global toggle (`tool_approval_required`) plus a per-skill whitelist. In the Go rewrite, the default behavior registers all 4 default skills unconditionally and relies on the global disable list.
- **SSRF guards**: The agent flow executor includes SSRF guards to prevent internal network probing via agent-initiated HTTP calls.
- **CORS**: Configured in middleware; be careful when widening allowed origins.

## Key Architectural Decisions

1. **Frontend Zero-Change Rewrite**: The React frontend was not modified during the backend rewrite. All API endpoints must preserve request/response shapes exactly.
2. **Pantheon SDK**: All LLM, embedding, and agent interactions go through Pantheon (`github.com/odysseythink/pantheon`). Do not add raw `openai-go` or `anthropic-go` dependencies; add a Pantheon builder instead.
3. **GORM AutoMigrate**: Schema changes are applied automatically on boot. There are no manual migration files.
4. **SQLite + FTS5**: The default database is SQLite with the `fts5` extension enabled via Go build tags. PostgreSQL is opt-in via `DATABASE_URL`.
5. **MCP Hypervisor**: External tools are loaded via MCP. The hypervisor manages process lifecycle, transport abstraction, and concurrency limits per server.
6. **Worker Framework**: Background jobs (cleanup, sync, embedding) are implemented as cron-scheduled jobs managed by `workers.Manager`, not a separate queue worker.

## Bug-Fixing Methodology

When investigating a bug and you are uncertain which hypothesis is correct, **prefer adding structured logs and validating against real runtime output** over making speculative code changes. Follow this sequence:

1. **Identify the decision point** — find the code path where multiple outcomes are possible (e.g., a conditional branch, an error that may or may not be returned, or a variable that could be `nil`).
2. **Add targeted `mlog` entries** — use `mlog.Info`, `mlog.Warn`, or `mlog.Error` (never `fmt.Println`) to emit the variable state, the branch taken, and any relevant IDs (request ID, workspace ID, thread ID).
3. **Reproduce the bug** — trigger the same scenario in the running server (`make dev` or the production binary).
4. **Collect the logs** — inspect the log output (stderr in dev mode, or `<storageDir>/logs/` in production). Use `grep` or `jq` to filter for the relevant log keys.
5. **Confirm or reject the hypothesis** — use the log evidence to decide which path is actually executing and what the data looks like at runtime.
6. **Remove or downgrade the debug logs** once the root cause is confirmed, unless they provide ongoing operational value.

> **Why this matters:** The codebase relies heavily on asynchronous flows (WebSocket sessions, background workers, MCP tool calls). Static code analysis often cannot predict which branch executes or what the intermediate state is. Runtime logs are the ground truth.

## Common Tasks for Agents

### Adding a New Vector Database

1. Implement `vectordb.VectorDatabase` in `backend/internal/vectordb/<name>.go`.
2. Wire it in `backend/cmd/server/main.go` inside the `switch cfg.VectorDB` block.
3. Add tests in `backend/internal/vectordb/<name>_test.go`.

### Adding a New LLM Provider

1. Add a builder function in `backend/internal/providers/builders.go` that returns a Pantheon `core.LanguageModel`.
2. Register it in the `providerRegistry` map.
3. Add any required config fields to `backend/internal/config/config.go`.

### Adding a New API Endpoint

1. Add the handler in the appropriate `backend/internal/handlers/*.go` file.
2. Register the route in the `Register*Routes` function.
3. Add tests in the corresponding `*_test.go` file.
4. If the endpoint is part of the API v1 surface, also add API-key auth middleware.

### Adding an Agent Skill

1. Implement the skill logic in `backend/internal/agent/` or as an MCP tool.
2. Register it in the agent runtime initialization.
3. Update the global disable list handling in `services.AgentSkillWhitelistService` if needed.

### Adding Browser Extension Features

1. Backend changes go under `backend/internal/handlers/browser_extension.go` and `backend/internal/services/browser_extension.go`.
2. Extension client changes go under `browser-extension/src/` and `browser-extension/public/`.
3. Build the extension with `make build-extension`.
4. Test the extension by loading `browser-extension/dist/` as an unpacked extension in Chrome.

## Notes

- The `.gpowers/` directory contains design documents and ADRs generated during planning. They are **not** shipped with the application and may drift from the actual code. Treat the source code as the source of truth.
- The old Node.js backend was deleted in commit `b29f337`. Do not search for `api/`, `cmd/`, or `web/` at the repository root — those directories no longer exist. The current backend is entirely under `backend/`.
