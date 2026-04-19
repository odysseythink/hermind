# Web IM Gateway Configuration — Design Spec

**Date:** 2026-04-19
**Status:** Draft — awaiting user review
**Scope:** Replace the placeholder at `api/webroot/` with a functional web UI that configures all 19 gateway adapters currently wired in `cli/gateway.go::buildPlatform` (messaging platforms plus the generic webhook / API-server / ACP transports).

---

## 1. Goals

- Let a user add, edit, enable/disable, test and apply gateway platform configurations from a browser — without hand-editing YAML or restarting the hermind process.
- Cover every adapter the current factory exposes (19 types).
- Keep the single user / local-host / Bearer-token security model. No multi-user, no RBAC.

## 2. Non-goals

- Configuring anything outside `gateway.platforms.*` (model, storage, memory, MCP, skills, etc. — those stay in `cli/ui/webconfig/`).
- i18n. UI is English only.
- Live-reload on every keystroke. Users save explicitly, then apply explicitly.
- Remote access hardening (TLS, CSRF, origin checks) beyond what the existing REST server already has.
- `telegram_user` (MTProto) and `mattermost_ws` — defined in code but not wired into the factory today. Out of scope; follow-up.

## 3. Architecture overview

```
hermind/
├── gateway/platforms/
│   ├── registry.go              (new)  FieldKind, FieldSpec, Descriptor, Register/Get/All
│   ├── descriptor_<type>.go     (new)  one per adapter (19 of them); init() registers itself
│   └── <adapter>.go             (existing, untouched except for one init() call)
├── cli/gateway.go               (edit) buildPlatform switch → registry lookup; unknown type → hard error
├── api/
│   ├── server.go                (edit) mount 4 new routes
│   ├── handlers_platforms.go    (new)  schema / reveal / test / apply
│   ├── handlers_config.go       (edit) redact on GET, merge on PUT
│   └── webroot/                 (edit) Vite build output: index.html + assets/
└── web/                         (new)  Vite + React 18 + TS source project
    ├── package.json, vite.config.ts, tsconfig.json, index.html
    ├── src/{main,App}.tsx
    ├── src/api/{client,types}.ts
    ├── src/components/{TopBar,Sidebar,Editor,Footer,FieldList,SecretInput,…}.tsx
    ├── src/pages/PlatformEditor.tsx
    └── src/styles/theme.css
```

**Build flow:** `make web` → `cd web && pnpm install && pnpm build` → `rsync` `web/dist/` into `api/webroot/`. `api/webroot/` contents are committed; `web/dist/` is gitignored. CI asserts the two are in sync (`make web && git diff --quiet api/webroot/`).

**Why `web/` is separate from `api/webroot/`:** `//go:embed` rules stay trivial when `api/webroot/` contains only built artifacts. Mixing sources there would require fragile exclude patterns.

**Dev loop:** `pnpm --dir web dev` starts Vite on :5173 with `/api` proxied to hermind; token supplied via `.env.local`'s `VITE_HERMIND_TOKEN`.

## 4. Platform registry (Go)

New types in `gateway/platforms/registry.go`:

```go
type FieldKind int
const (
    FieldString FieldKind = iota
    FieldInt
    FieldBool
    FieldSecret
    FieldEnum
)

type FieldSpec struct {
    Name     string   // YAML key, e.g. "bot_token"
    Label    string   // UI label
    Help     string   // optional one-line hint
    Kind     FieldKind
    Required bool
    Default  any
    Enum     []string // only for FieldEnum
}

type Descriptor struct {
    Type        string       // stable id, e.g. "telegram"
    DisplayName string
    Summary     string
    Fields      []FieldSpec
    Build       func(opts map[string]string) (Platform, error)
    Test        func(ctx context.Context, opts map[string]string) error
}

func Register(d Descriptor)
func Get(typ string) (Descriptor, bool)
func All() []Descriptor // sorted by Type
```

Each adapter gets a `descriptor_<type>.go` whose `init()` registers itself. `cli/gateway.go::buildPlatform` collapses to:

```go
d, ok := platforms.Get(cfg.Type)
if !ok { return nil, fmt.Errorf("unknown platform type %q", cfg.Type) }
return d.Build(cfg.Options)
```

Previously an unknown type was logged and skipped; it now fails the gateway build. This is a small behavior change covered by a test.

**Descriptor coverage (MVP):** every type currently in `buildPlatform` — 19 total. Grouped for orientation:

- Bidirectional IM (7): `telegram`, `discord_bot`, `slack_events`, `matrix`, `signal`, `whatsapp`, `mattermost_bot`.
- Generic transports (3): `webhook`, `api_server`, `acp`.
- Outbound-only webhook adapters (6, all take a single `webhook_url`): `slack`, `discord`, `mattermost`, `feishu`, `dingtalk`, `wecom`.
- Out-of-band notification (3): `email`, `sms`, `homeassistant`.

Covering all 19 avoids a regression from the "unknown type → hard error" change: existing YAMLs referencing any legacy type continue to load.

**`Test` implementation sketch per type:**

- HTTP platforms (telegram/discord/slack/whatsapp/homeassistant/webhook): call the platform's own identity/auth probe (`getMe`, `auth.test`, etc.).
- Email (SMTP): dial + EHLO + AUTH LOGIN, no message sent.
- SMS (Twilio): GET `/Accounts/<sid>.json`.
- Matrix: `GET /_matrix/client/v3/account/whoami`.
- Signal: `GET /v1/about` on the configured base URL.
- api_server: open+close a listener on `addr` to prove the port binds.

All `Test` calls use a 10s context deadline.

## 5. REST API surface

| Method | Path | Auth | Purpose |
|---|---|---|---|
| GET | `/api/platforms/schema` | bearer | Returns all descriptors |
| GET | `/api/config` | bearer | **Changed**: secret values returned as `""` |
| PUT | `/api/config` | bearer | **Changed**: secret values received as `""` retain prior value |
| POST | `/api/platforms/{key}/reveal` | bearer | Returns plaintext for one secret field of one instance |
| POST | `/api/platforms/{key}/test` | bearer | Runs descriptor.Test against the stored Options for that key |
| POST | `/api/platforms/apply` | bearer | Stop → rebuild → Start the gateway subsystem |

**Payload shapes:**

`GET /api/platforms/schema`
```json
{"descriptors":[
  {"type":"telegram","display_name":"Telegram Bot","summary":"...",
   "fields":[{"name":"token","label":"Bot Token","kind":"secret","required":true}]},
  ...
]}
```

`POST /api/platforms/{key}/reveal`
```json
// request
{"field":"token"}
// response 200
{"value":"123:ABC..."}
// response 400
{"error":"no such field"}
```
Only fields with `Kind == FieldSecret` may be revealed. Any other field returns 400. The key must already exist under `gateway.platforms` — newly-created-but-unsaved keys are not revealable (the frontend hides Show in that state).

`POST /api/platforms/{key}/test`
```json
// response on both success and failure uses 200; frontend reads body
{"ok":true}
{"ok":false,"error":"unauthorized: check token"}
```
The server uses the Options currently stored in memory (post-PUT). Request body must be empty — no ad-hoc credential passing.

`POST /api/platforms/apply`
```json
{"ok":true,"restarted":["tg_main","slack_ops"],"errors":{},"took_ms":342}
{"ok":false,"error":"failed to stop gateway: context deadline exceeded"}
```
A `sync.Mutex` serializes concurrent apply requests; a second concurrent caller gets `409 Conflict`. Per-platform stop timeout is 5s.

**Auth for new endpoints:** all go through the existing `NewAuthMiddleware`; nothing added to the public allowlist. Frontend always uses `Authorization: Bearer <token>` header. The `?t=` query-param path stays in place for backward-compatible manual probing but is not used by the app.

**Error codes:**

- 400 — invalid field/key, YAML parse failure, malformed body.
- 401/403 — missing or wrong token (middleware).
- 404 — unknown `{key}`.
- 409 — apply in progress.
- 500 — internal error.

All new endpoints return JSON; success responses carry data fields, failure responses carry `error` string. Never mix the two forms in one response.

## 6. Secret handling

**On GET `/api/config`:** `handleConfigGet` marshals the config, then walks `gateway.platforms`. For each descriptor field with `Kind == FieldSecret`, the corresponding key under `Options` is overwritten with `""`. No placeholder — explicit empty string avoids the "user round-trips `••••` back to disk" footgun.

**On PUT `/api/config`:** For each `gateway.platforms[key]`, iterate descriptor fields:

```text
for each FieldSecret f:
  incoming := newCfg.Platforms[key].Options[f.Name]
  current  := liveCfg.Platforms[key].Options[f.Name]
  if incoming == "":
      newCfg.Platforms[key].Options[f.Name] = current   # preserve
  else:
      # keep incoming (overwrite)
```

Non-secret fields are written as received, including empty strings — user intent respected.

**Clearing a secret** is not supported as a field operation — `""` always means "preserve". To remove a secret (and stop using a platform), delete the whole instance from the sidebar. Accepted UX limitation; the alternative would be a three-value scheme (`null` / `""` / string) and is not worth the complexity.

**Reveal** reads from **in-memory** config, not disk, so users see the value they just PUT (but possibly haven't Applied) rather than a stale disk value.

## 7. Apply lifecycle

1. Acquire `applyMu`; if already held, return 409.
2. Read current in-memory `Config.Gateway`.
3. Call `gateway.Gateway.Stop(ctx)` with a 5s per-platform deadline; timeouts are logged but do not abort the apply.
4. Rebuild a fresh `gateway.Gateway` using the existing `cli/gateway.go::BuildGateway` entry point.
5. `Start(ctx)`. Per-platform Start failures are collected in `errors[key]` but do not fail the whole apply. Other platforms keep running.
6. Release the mutex; respond with `{ok, restarted, errors, took_ms}`.

**Process scope:** only the gateway subsystem restarts. `agent.Engine`, storage, skills registry, and the REST server itself are unaffected.

**Failure modes:**

- Process dies mid-apply → next launch rebuilds from disk, no corruption.
- Partial start failure → those keys remain configured but not running; frontend marks them with a red dot derived from the Apply response.
- Config deserialization failure during apply → old gateway is already stopped. Frontend shows error and prompts the user to correct config and re-apply. Accepted risk for MVP; worst case is "run `hermind` again".
- Stop timeout (5s) on a wedged adapter → the stuck goroutine is left running and the new gateway is built alongside it. Over many apply cycles this could leak goroutines. Accepted for MVP; hardening tracked in §12.

## 8. Frontend architecture

**Stack:** Vite 5, React 18, TypeScript 5. No router, no state library, no UI framework. `zod` for runtime schema validation at the REST boundary. CSS Modules for scoping.

**App state** — single `useReducer` at the root:

```ts
type AppState = {
  token: string;                // from window.HERMIND.token
  descriptors: Descriptor[];    // GET /api/platforms/schema, one-shot
  config: Config;               // mutable draft
  originalConfig: Config;       // snapshot for dirty diff
  selectedKey: string | null;
  status: 'idle' | 'saving' | 'applying' | 'error';
  flash: { kind: 'ok' | 'err'; msg: string } | null;
};
```

Dirty detection is a structural diff between `config` and `originalConfig`; it drives the footer "N unsaved changes" text and the emphasis on the Save & Apply button.

**Component tree:**

```
App
├─ TopBar           brand / config path / global status dot
├─ Sidebar          instance list + "New instance" button
│   └─ NewInstanceDialog   pick type, enter key, create empty Options
├─ Editor           renders for selectedKey; empty state otherwise
│   ├─ EditorHeader       readonly key, type tag, enabled toggle, delete
│   ├─ FieldList          walks descriptor.fields
│   │   ├─ TextInput
│   │   ├─ NumberInput
│   │   ├─ BoolToggle
│   │   ├─ EnumSelect
│   │   └─ SecretInput    password input + Show/Hide, calls /reveal
│   └─ TestConnection     calls /test, renders ok/err bubble
└─ Footer           status msg + Save + Save & Apply
```

**Selection persistence:** selected key lives in `window.location.hash` so refresh keeps context.

**Key-naming rule:** new instance keys match `^[a-z][a-z0-9_]*$`. Keys are immutable after creation — rename = delete + recreate.

**Apply semantics at the UI:** one button, "Save & Apply" — it PUTs `/api/config` then POSTs `/api/platforms/apply`. A plain "Save" button exists for draft parking. Standalone Apply is not surfaced (reading the on-disk config can be done by the user via CLI in the edge case they need it).

**Visual style:** amber `#FFB800` accent, `prefers-color-scheme` light/dark, design tokens aligned with `docs/superpowers/specs/2026-04-17-web-config-ui-refresh-design.md` so both web surfaces look like hermind.

## 9. Error handling

| Category | Source | UI treatment |
|---|---|---|
| Field validation (Required / Enum / regex for keys) | Frontend + Go descriptor | Inline red border + hint, blocks Save |
| Persistence (PUT /api/config non-2xx) | Backend | Footer flashes red, full error in toast |
| Runtime (Test / Apply failures) | Backend | Red bubble next to the relevant button; Apply failures listed per-key in the sidebar |

**Specific risks:**

- **Revealing a key that doesn't exist on disk yet:** frontend hides Show until `originalConfig` contains the key.
- **Testing an unsaved key:** button disabled, tooltip "Save first".
- **External YAML edits:** not detected. Save overwrites. Documenting as a known limitation; a future `/api/config` `If-Unmodified-Since` could be added if users hit it.
- **Token invalidation (401):** top banner "Token invalid — reload the page from the hermind CLI". No auto-refresh.

## 10. Testing strategy

**Go unit:**

- `gateway/platforms/registry_test.go` — descriptor sanity (non-empty Type/Build/Test, unique field names, Required∧Default mutex).
- Per-descriptor `Test` stubbed against `http.RoundTripper`/SMTP fake for happy + sad paths. No real network.
- `api/handlers_platforms_test.go` — each endpoint: happy, 401, 404, reveal-rejects-non-secret, apply concurrency → 409.
- `api/handlers_config_test.go` — GET redaction, PUT empty-preserves, PUT non-empty-overwrites, round-trip preserves non-secret fields verbatim.
- `cli/gateway_test.go` — unknown type → error (regression guard for the silent-skip removal).

**Frontend unit (Vitest):**

- Diff logic: new key, deleted key, field-only change, enabled-only change.
- Serialization: unchanged secret fields PUT as `""` — correctness depends on backend merge (§6), not frontend masking. Test that the draft->payload function does not invent values.
- zod parse of descriptor responses catches schema drift.

**Manual smoke (documented in README under `web/README.md`):**

1. Start hermind, open the UI — empty sidebar.
2. New instance → pick telegram → key `tg_main` → Save. Sidebar shows entry.
3. Select `tg_main`, Show proves empty-secret handling, fill Bot Token, Save.
4. Show → plaintext visible. Test → green "connected as @x".
5. Save & Apply → footer shows applying → ok; hermind log shows `tg_main: started`.
6. Break the token, Save & Apply → `tg_main` red-dotted with error message.

**CI additions:**

- `make web && git diff --quiet api/webroot/` — rejects PRs that edit `web/` without syncing built output.
- `cd web && pnpm type-check && pnpm test && pnpm lint`.

## 11. Delivery sequence

Five independently-shippable PRs; each leaves hermind in a working state.

1. **Platform registry** (§4): registry, 19 descriptors, rewrite `buildPlatform`, tests. No REST/UI change.
2. **REST endpoints** (§5 §6 §7): schema / reveal / test / apply, plus config redact+merge. `curl` can drive the full flow.
3. **Frontend skeleton** (§8 TopBar/Sidebar/Footer with empty Editor): new Vite project, embed wiring, replaces placeholder.
4. **Editor + controls**: FieldList, SecretInput, TestConnection, NewInstanceDialog. Feature-complete.
5. **Polish & CI**: lint config, `make web` sync check, manual smoke script, README.

## 12. Open follow-ups (explicitly out of scope)

- `telegram_user` and `mattermost_ws` descriptors.
- Per-platform diff-based hot reload (instead of global gateway restart).
- Hardened Stop: replace the 5s soft timeout with cooperative cancellation + goroutine tracking so wedged adapters don't accumulate across repeated applies.
- Config file mtime / `If-Unmodified-Since` guard against external YAML edits.
- Playwright E2E.
- i18n.
- Multi-user auth / RBAC.
- Clearing secret fields through the UI (needs a three-value scheme; see §6).
