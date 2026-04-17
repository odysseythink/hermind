# Design: literal API keys + per-provider model fetch

**Status:** Approved
**Date:** 2026-04-17

## Goal

Two related quality-of-life improvements to the web config editor:

1. **Drop `env:VAR` indirection.** API keys (and other secrets) stored in
   `~/.hermind/config.yaml` are literal strings, not `env:VARNAME`
   references that get resolved at load time. Users paste real keys into
   the web UI and they land directly in the YAML.
2. **Per-provider "Get" button.** In the Providers section of the web
   UI, each card has a small trailing `Get` button next to the `model`
   input. When the card's `api_key` is non-empty and the provider type
   supports model listing, the button is active; clicking it queries the
   provider's `/models` endpoint and populates a `<datalist>` attached
   to the `model` field so the user can pick (or keep typing) a model
   ID.

## Breaking change

Configs that still use `api_key: env:OPENAI_API_KEY` will no longer be
resolved. The raw string `"env:OPENAI_API_KEY"` will be sent to the
provider and the provider will 401. Users must paste the real key (the
web UI makes this easy — open the Providers section, paste, Save). The
CHANGELOG entry flags this explicitly. No deprecation window, no
auto-migration — this is a developer tool, loopback-only, single-user;
the cost of a sharp break is low.

## Non-goals / out of scope

- Model capability metadata (context window, pricing, modality). v1
  returns model IDs only.
- Caching fetched model lists between sessions — each click of `Get`
  hits the live API.
- Pagination UI — browsers handle `<datalist>` with a few hundred
  options fine (OpenRouter returns ~300).
- Listing support for Zhipu (JWT rotation) and Wenxin (ACCESS_TOKEN
  flow) — add when someone needs it.
- Secret managers (1Password CLI, `pass`, macOS Keychain). If the user
  wants indirection again, that's a later spec.
- TUI parity — the TUI still uses its own edit flow; this spec only
  touches the web editor.

## Scope

Three packages change.

| File | Change |
|------|--------|
| `config/loader.go` | Delete `expandEnvVars` and its caller |
| `config/loader_test.go` | Remove env-expansion tests; add one literal-key round-trip test |
| `provider/provider.go` | Add optional `ModelLister` interface |
| `provider/openaicompat/list_models.go` (new) | `Client.ListModels(ctx)` → `GET {base}/models` with Bearer auth |
| `provider/openaicompat/list_models_test.go` (new) | Happy + failure cases |
| `provider/anthropic/list_models.go` (new) | `Provider.ListModels(ctx)` → `GET {base}/v1/models` with `x-api-key` + `anthropic-version` |
| `provider/anthropic/list_models_test.go` (new) | Happy + failure cases |
| `cli/ui/webconfig/handlers.go` | New `handleProvidersModels`; add `providersModelsUnsupported` allowlist |
| `cli/ui/webconfig/server.go` | Route `/api/providers/models` |
| `cli/ui/webconfig/server_test.go` | Three tests for the new endpoint |
| `cli/ui/webconfig/web/app.js` | Get button wiring, datalist, live api_key reactivity, flush-on-click |
| `cli/ui/webconfig/web/app.css` | `.get-models-btn`, `.inline-error`, row layout for model + button |
| `README.md` (and any doc that mentions `env:`) | Update / remove |

Untouched: TUI editor (`cli/ui/config/`), Go tests outside the above, every provider package that delegates to `openaicompat` (they inherit `ModelLister` for free).

## Design

### Part 1: remove `env:` substitution

`config/loader.go:expandEnvVars` handles five substitution sites —
primary providers' `api_key`, fallback providers' `api_key`,
`terminal.modal_token`, `terminal.daytona_token`, and `mcp.servers[].env`
values. All five go. The function is deleted along with its single
caller inside `Load()`. Legacy configs are not rewritten; users edit
manually (or via the web UI for provider API keys).

`config/loader_test.go` currently has tests named something like
`TestLoadWithEnvVar*` that exercise the expansion. Those get deleted.
Add one minimal test `TestLoadPreservesLiteralApiKey` that loads a YAML
with `api_key: env:FOO` and asserts the config holds the literal string
`"env:FOO"` (no expansion).

Update any `README.md`, `docs/`, or `CHANGELOG.md` reference to
`env:VAR` substitution. CHANGELOG entry:

> **Breaking:** `env:VAR` references in `api_key`, `terminal.modal_token`,
> `terminal.daytona_token`, and `mcp.servers[].env` are no longer
> resolved. Paste literal values. Use the web editor (`hermind config
> --web`) for API keys.

### Part 2: `ModelLister` interface

New optional interface in `provider/provider.go`:

```go
// ModelLister is an optional capability for providers that expose a
// models-listing endpoint. Webconfig consumers check
// `lister, ok := p.(ModelLister)` before offering model discovery —
// providers without a `/models` endpoint simply do not implement it.
type ModelLister interface {
    ListModels(ctx context.Context) ([]string, error)
}
```

`ListModels` returns model IDs only. Ordering is provider-defined
(typically the server's order preserved). An empty slice is a valid
result (provider with no models). Errors carry the underlying HTTP
status or transport error unwrapped so the UI can surface them.

#### Implementations

**`provider/openaicompat/list_models.go`:** `(*Client).ListModels(ctx)`
does `GET {BaseURL}/models` with the client's configured
`Authorization: Bearer {APIKey}` header (plus any `ExtraHeaders`). Parses
the OpenAI-standard shape `{"data":[{"id":"gpt-4"},...]}` via
`json.Decoder`, extracts `id` fields, returns the slice. Non-2xx
responses map to an error that includes status and up to 512 bytes of
body. Because DeepSeek, Kimi, Qwen, Minimax, OpenRouter, and
OpenAICompat-custom providers all build on this client, implementing
once here gives us six providers' listing support.

**`provider/anthropic/list_models.go`:** Same shape but headers are
`x-api-key: {APIKey}` and `anthropic-version: 2023-06-01`. Response
shape is also `{"data":[{"id":"claude-..."},...]}` — identical extractor
works. Returns IDs.

**Zhipu and Wenxin:** no implementation. Their JWT rotation / ACCESS_TOKEN
flows mean a drop-in `ListModels` would be non-trivial and nobody is
asking for it yet.

Context timeout: implementations respect the caller's `ctx`. The HTTP
request uses `http.NewRequestWithContext(ctx, ...)`. Callers set their
own timeout (see Part 3).

### Part 3: `/api/providers/models` endpoint

New handler in `cli/ui/webconfig/handlers.go`:

```
POST /api/providers/models
Body: {"key": "<provider-entry-key>"}
```

Handler flow:

1. **Method guard** — accept only `POST`, else 405.
2. **Origin check** — `isLocalOrigin(r)` (same defense as `/api/reveal`);
   403 on failure. Extra paranoia since we're about to use live creds.
3. **Key validation** — reuse existing `validProviderKey`; 400 if bad.
4. **Read config from doc** — fetch `providers.<key>.provider`,
   `providers.<key>.base_url`, `providers.<key>.api_key`, and
   `providers.<key>.model` via `s.doc.Get`. These come from the
   *in-memory* doc, which is what the user is currently editing —
   crucial so a freshly-pasted key can be tested before Save.
5. **Build `config.ProviderConfig`** with those four fields and call
   `factory.New(cfg)` (from `provider/factory`) to instantiate the
   provider.
6. **Type-assert** — `lister, ok := p.(provider.ModelLister)`. If `!ok`,
   return 400 with body `"provider type \"%s\" does not support model
   listing"`. This is the defense-in-depth counterpart to the
   frontend's hidden-button hardcoded set.
7. **Call `lister.ListModels(ctx)`** with a 10s timeout derived from the
   request context. On error: 502 with the error string as body (the
   frontend surfaces it via `.inline-error`). On success: 200 with
   `{"models": [...]}` JSON.

Route registration in `server.go`:

```go
mux.HandleFunc("/api/providers/models", s.handleProvidersModels)
```

### Part 4: frontend UX

#### Layout

Each provider card's `model` row becomes a flex container with the
input on the left and a `Get` button on the right:

```
API key    [ •••••••••••••••  ] [Show]
Model      [ gpt-4             ] [Get]
```

The input has `list="models-<escaped-key>"`. A sibling empty
`<datalist id="models-<escaped-key>"></datalist>` is created in the DOM
(invisible, browsers handle the dropdown rendering).

#### Button states (driven by CSS + class toggles)

| Condition | Visual |
|---|---|
| Provider type is in the client-side unsupported set (`zhipu`, `wenxin`) | Button hidden (not rendered) |
| `api_key` input is empty | Button disabled, color `--muted`, `cursor: default` |
| `api_key` input is non-empty | Button enabled, color `--accent`, `cursor: pointer`, hover shows underline |
| Fetch in flight | Button text = `Loading…`, `disabled` attribute set |
| Fetch succeeded | Datalist populated with fetched IDs; button text becomes `Got <N>` for 1000ms via `setTimeout`, then reverts to `Get` |
| Fetch failed | `<span class="inline-error">` rendered under the model row with the server's error string; button reverts to `Get` immediately |

Styling parallels `.reveal-btn`: transparent bg, 12px font, no border,
muted→accent color transition on 120ms. `.inline-error`: `color:
var(--error)`, `font-size: 13px`, `margin-top: 4px`.

#### Live reactivity

The api_key input gets an `oninput` handler (in addition to the existing
`onchange` which still fires persist on blur). `oninput` fires on every
keystroke and just toggles the Get button's `disabled` attribute and
color class — no server roundtrip. This makes the button activate
"live" as the user types, matching `当api key不为空时，获取按钮变亮`.

#### Flush-on-click

When the Get button is clicked, the handler:

1. Calls `document.activeElement?.blur()` to force any pending `onchange`
   on the currently focused input to fire. This triggers the existing
   `persistProviderField` path for whichever field the user was editing.
2. **Does not iterate and re-persist every input on the card.** The
   `api_key` input's displayed value may be the `"••••"` mask sentinel
   returned by `/api/providers` GET; blindly persisting that would
   overwrite the real key in the in-memory doc with four dot characters.
   The safe path is to rely on the browser's native
   `blur → onchange → persistProviderField` chain: only fields the user
   actually edited fire `onchange` with a non-sentinel value.
3. Fetches `POST /api/providers/models` with `{key}`.
4. On success, clears the datalist's existing `<option>` children and
   appends one `<option value="...">` per returned model ID.
5. On error, renders `.inline-error` under the row.

**Race window.** Between `blur()` and the fetch, the browser has
dispatched `onchange` but `persistProviderField`'s POST may not have
round-tripped. On loopback this is ~a few ms; a user clicking Get
immediately after typing may see a stale-credentials error (e.g., 401)
and retry. We consider this acceptable and prefer it to the api_key
overwrite hazard of the alternative.

#### Unsupported-provider hardcoded set

```js
const UNSUPPORTED_LIST_MODELS = new Set(['zhipu', 'wenxin']);
```

Checked at render time against `p.provider` (the current value of the
provider-type field on the card). If the user changes the provider type
to an unsupported one, the card re-renders via the existing
`renderForm()` call, and the Get button disappears for that card.

## Error / edge cases

- **Empty provider type.** Frontend skips the hardcoded-unsupported
  check (empty string is not in the set) and lets the server respond
  with 400 "unsupported". User sees `.inline-error` "provider type
  \"\" does not support model listing". Reasonable.
- **Invalid base_url / DNS failure.** `openaicompat.ListModels` returns
  the transport error (`*url.Error`). Server returns 502 with that
  string. UI shows it.
- **401 from provider.** Server returns 502 with the body fragment from
  the provider. UI shows `"provider returned 401: {..}"`. User realizes
  the key is wrong.
- **Very large response (OpenRouter).** `<datalist>` handles a few
  hundred options without pagination. No pruning.
- **Concurrent Get clicks.** The button is disabled while a fetch is
  in flight, so only one is ever outstanding per card.
- **Get clicked with stale `model` value.** The current `model` text is
  preserved as the input's value; the user can continue to type or pick
  a new ID from the datalist.

## Testing

### Unit tests

- **`config/loader_test.go`** — remove all env-expansion tests. Add
  `TestLoadPreservesLiteralApiKey` asserting that
  `providers.x.api_key: "env:FOO"` loads as the literal 9-byte string,
  not expanded.
- **`provider/openaicompat/list_models_test.go`** — two cases:
  happy path (`httptest.NewServer` returning canned
  `{"data":[{"id":"a"},{"id":"b"}]}` → expect `["a","b"]`), failure
  (server returns 500 → expect error containing status).
- **`provider/anthropic/list_models_test.go`** — same two cases with
  anthropic-style response and header assertions (request must carry
  `x-api-key` and `anthropic-version`).
- **`cli/ui/webconfig/server_test.go`** — three new tests:
  - `TestProvidersModelsHappyPath`: seeds `providers.test.provider =
    openaicompat`, `base_url = <httptest URL>`, `api_key = "sk"`; asserts
    `POST /api/providers/models {"key":"test"}` returns
    `{"models":["a","b"]}`.
  - `TestProvidersModelsUnsupportedType`: seeds `provider = zhipu`;
    asserts 400.
  - `TestProvidersModelsOriginCheck`: same happy-path setup but the
    request carries `Origin: https://evil.com`; asserts 403.

### Manual smoke

1. `./bin/hermind config --web`
2. Providers → pick an existing card with a valid api_key → Get button
   shows amber. Click → datalist populates. Type in model input —
   autocomplete suggestions appear.
3. Clear the api_key → Get grays out live as you backspace.
4. Change provider type to `zhipu` → Get disappears.
5. Delete api_key and click into model input and press `Get` shortcut
   (n/a, there's no shortcut) — N/A.
6. Stale-key failure: paste a wrong key, click Get → `.inline-error`
   appears under the row with `401` message.

## Migration

Existing configs with `env:VAR` values are not touched on upgrade. On
the first provider call after upgrade, the provider 401s with the
literal string. User opens the web editor, pastes the real key, Saves.
For `terminal.*_token` and `mcp.servers[].env`, users edit the YAML by
hand (no web surface). CHANGELOG callout is the only communication.

## Rollout

Two commits, logically separable for reviewer focus:

1. `refactor(config): drop env:VAR api-key indirection` — `loader.go`,
   `loader_test.go`, doc updates. Breaking change flag.
2. `feat(web-config): per-provider Get models button` — provider
   `ModelLister`, openaicompat + anthropic implementations, new
   endpoint, frontend button + datalist.

Commit 1 first so the breaking-change commit stands on its own in the
log. Commit 2 builds on top.

No feature flag, no gradual rollout — this is an embedded-asset web UI
served by the CLI. Next build ships both changes automatically.
