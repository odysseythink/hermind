# Telegram HTTP Proxy — Design

**Goal:** Let a hermind operator route Telegram Bot API traffic through an HTTP, HTTPS, or SOCKS5 proxy so the platform works in mainland China (and any other network where `api.telegram.org` is blocked at the IP/TLS layer).

**Why now:** Telegram is unreachable from mainland China. The existing `telegram_doh.go` DNS-over-HTTPS fallback handles DNS-level blocks but does nothing when the Telegram IPs themselves are firewalled (which is the China case). Without a proxy, every Chinese operator has to run hermind inside a VPN'd shell — awkward for anything hermind-embedded (long-running gateway processes, systemd units). An optional proxy field on the Telegram descriptor makes the platform work in-process.

**Scope:** Telegram platform only. No other platforms, no LLM providers, no global config. If future stages want a system-wide proxy config, they build on the pattern this change establishes.

---

## What this does NOT ship

- **No global `proxy:` config.** The top-level Config struct is unchanged. Users who want every platform proxied configure each one individually.
- **No proxy support for other gateway platforms** (Discord, WhatsApp, Slack, Matrix, etc). They're also blocked in China, but we scope this change tightly to prove the pattern. Follow-up stages extend.
- **No proxy support for LLM providers** (Anthropic, OpenAI, Bedrock, etc). Same reasoning. Users in China run an external VPN or `HTTPS_PROXY` shell var for LLM calls until a later stage lands.
- **No `HTTPS_PROXY`/`HTTP_PROXY` environment variable fallback.** If the telegram descriptor's `proxy` field is empty, hermind connects directly. No surprise behavior from inherited shell state.
- **No per-call proxy selection.** Every Telegram outbound request goes through the configured proxy (or not). No allowlist, no per-endpoint override.
- **No proxy authentication UI.** Users embed credentials in the URL if needed (`socks5://user:pass@host:port`). Separate `proxy_username` / `proxy_password` fields are out of scope — the 95% case in China is localhost, passwordless.
- **No integration test with a real SOCKS5 server.** Unit tests cover transport wiring. End-to-end proxy verification happens through the smoke flow.
- **No shared `internal/httpx` package.** The proxy-client builder lives as a package-private helper inside `gateway/platforms/telegram.go`. If a second platform needs the same logic, extract then; YAGNI now.

---

## YAML config surface

`proxy` is a new optional field on the Telegram platform descriptor, alongside the existing `token` field.

```yaml
gateway:
  platforms:
    my_telegram:
      type: telegram
      options:
        token: "123456:AAHb..."
        proxy: socks5://127.0.0.1:7890
```

### Semantics

- **Absent or empty string** → direct connection. Current behavior, no change.
- **Non-empty** → must parse as a URL with scheme in `{http, https, socks5}`. Any other scheme (ftp, socks4, socks5h, bogus) is a config error.
- **URL may contain embedded credentials** (`socks5://user:pass@host:port`). Credentials are parsed by `net/url` and passed to the respective dialer. No separate fields.
- **Same proxy applies to every Telegram outbound request**: polling `getUpdates`, sending `sendMessage`, the startup reachability `Test`, and the DoH fallback path in `telegram_doh.go`.

### Error reporting

- Invalid proxy URL at startup → `hermind gateway` logs `telegram: invalid proxy "<value>": <parse error>` and refuses to start the platform. Other platforms boot normally.
- Invalid scheme → `telegram: unsupported proxy scheme "ftp" (want http/https/socks5)`.
- Proxy unreachable at runtime → standard `net.OpError` surfaces through whichever Telegram method hit it (polling loop, send, etc.). Operator debugs via hermind logs.

---

## Implementation

Three files touched, no new packages, no new dependencies beyond the standard library plus `golang.org/x/net/proxy` for SOCKS5 (already an indirect dependency via `aws-sdk-go-v2`).

### `gateway/platforms/telegram.go`

`NewTelegram` grows a `proxyURL` parameter AND starts returning an error so invalid proxy URLs fail loud at construction. Two package-private helpers: `newTelegramTransport` builds the `http.RoundTripper` (shared with `DoHTransport`), `newTelegramClient` wraps it with a timeout for normal API calls.

```go
func NewTelegram(token, proxyURL string) (*Telegram, error) {
    client, err := newTelegramClient(proxyURL, 60*time.Second)
    if err != nil {
        return nil, err
    }
    return &Telegram{token: token, client: client}, nil
}

// newTelegramTransport returns an http.RoundTripper routed through proxyURL.
// Empty proxyURL → http.DefaultTransport. Supported schemes: http, https, socks5.
// Shared between the main Telegram client and DoHTransport's fallback path.
func newTelegramTransport(proxyURL string) (http.RoundTripper, error) {
    if proxyURL == "" {
        return http.DefaultTransport, nil
    }
    u, err := url.Parse(proxyURL)
    if err != nil {
        return nil, fmt.Errorf("telegram: invalid proxy %q: %w", proxyURL, err)
    }
    switch u.Scheme {
    case "http", "https":
        return &http.Transport{Proxy: http.ProxyURL(u)}, nil
    case "socks5":
        dialer, err := proxy.FromURL(u, proxy.Direct)
        if err != nil {
            return nil, fmt.Errorf("telegram: socks5 dial: %w", err)
        }
        cd, ok := dialer.(proxy.ContextDialer)
        if !ok {
            return nil, fmt.Errorf("telegram: socks5 dialer does not support context")
        }
        return &http.Transport{DialContext: cd.DialContext}, nil
    default:
        return nil, fmt.Errorf("telegram: unsupported proxy scheme %q (want http/https/socks5)", u.Scheme)
    }
}

// newTelegramClient wraps newTelegramTransport with the standard timeout.
func newTelegramClient(proxyURL string, timeout time.Duration) (*http.Client, error) {
    t, err := newTelegramTransport(proxyURL)
    if err != nil {
        return nil, err
    }
    return &http.Client{Transport: t, Timeout: timeout}, nil
}
```

### `gateway/platforms/telegram_doh.go`

`DoHTransport` currently builds its own `&http.Transport{...}` inside `tryFallbackIP` at line 157. That transport also needs to honor the proxy or DoH-on-proxy users break. `DoHTransport` gains a `proxyURL string` field populated by its constructor; `tryFallbackIP` calls `newTelegramTransport(d.proxyURL)` instead of building a raw `&http.Transport{}` directly.

### `gateway/platforms/descriptor_telegram.go`

Add the field and thread proxy through `Build` and `Test`:

```go
Fields: []FieldSpec{
    {Name: "token", Label: "Bot Token", Kind: FieldSecret, Required: true,
        Help: "From @BotFather."},
    {Name: "proxy", Label: "Proxy URL", Kind: FieldString, Required: false,
        Help: "Optional. http://, https://, or socks5://. Leave blank for direct connection. Required in mainland China."},
},
Build: func(opts map[string]string) (gateway.Platform, error) {
    return NewTelegram(opts["token"], opts["proxy"])
},
Test: func(ctx context.Context, opts map[string]string) error {
    return testTelegram(ctx, opts["token"], opts["proxy"], "https://api.telegram.org")
},
```

`testTelegram` signature grows a `proxyURL` parameter. Inside, instead of calling `httpProbe` (which uses `http.DefaultClient`), it builds a dedicated proxy-aware client via `newTelegramClient` and issues the `GET /bot<token>/getMe` request itself — ~8 lines of code, no touch to `testprobe.go`, no risk to other platforms' tests.

---

## Testing

### Unit tests — `gateway/platforms/telegram_test.go` (new file)

Four tests, zero network, zero real proxy.

1. **`TestNewTelegramClient_Direct`** — empty `proxyURL` → returned client's `Transport` is nil (falls back to `http.DefaultTransport`). Confirms no proxy behavior when unset.
2. **`TestNewTelegramClient_HTTP`** — `http://127.0.0.1:8080` → the returned client's transport has a non-nil `Proxy` func; calling it with a test `*http.Request` returns the parsed proxy URL.
3. **`TestNewTelegramClient_SOCKS5`** — `socks5://127.0.0.1:1080` → returned client's transport has a non-nil `DialContext` func (the SOCKS5 dialer chain). No actual connect.
4. **`TestNewTelegramClient_InvalidScheme`** — `ftp://host:21` → error matches `unsupported proxy scheme`.

### Descriptor test — `gateway/platforms/descriptor_telegram_test.go` (new file or extend existing)

One test pinning the schema: the `telegram` descriptor exposes `proxy` as `FieldString`, `Required: false`. Prevents accidental removal in future edits.

### Integration — deferred

An in-process SOCKS5 server (`armon/go-socks5` — ~30 lines of setup) roundtripping a request from `Telegram.client` through the proxy into an `httptest.Server` would prove end-to-end wiring. Skipped for this stage. If a bug surfaces, add it then; unit tests give enough signal to ship.

---

## UI behavior

Zero new frontend code. Stage 4b's `/api/config/schema` already walks every registered descriptor's fields and emits them over the API. Adding `proxy` as a `FieldString` on the Telegram descriptor automatically surfaces it as a text input in the web Gateway UI, next to the existing `token` field.

The field renders as a plain text input (not a secret). The 95% case in China is `socks5://127.0.0.1:7890` with no credentials — nothing sensitive about that. If a user embeds credentials in the URL (`socks5://user:pass@host:port`), the value is still plaintext in the UI and config; users who want auth-field masking get a follow-up stage (scope this change narrow).

---

## Smoke flow

Add to `docs/smoke/gateway.md` (or create the file):

```
## Telegram proxy

1. Start a local SOCKS5 proxy on 127.0.0.1:7890
   (Clash, mihomo, v2rayN, Shadowsocks — any of them).
2. Configure a Telegram platform in config.yaml:
     options:
       token: <your bot token>
       proxy: socks5://127.0.0.1:7890
3. `hermind gateway` — platform startup succeeds; no "connection timeout"
   error on the initial reachability probe.
4. POST to /api/platforms/my_telegram/test (via hermind web or curl) —
   response is {"ok": true}, routed through the proxy.
5. Send a message to the bot. Bot responds. Verify the polling loop works.
6. Kill the proxy process. Restart hermind. Startup probe fails with
   "connect: connection refused" (expected — cannot reach proxy).
7. Remove the proxy line from config. Direct connection resumes (works on
   any network without the Great Firewall).
```

---

## File manifest

**Created:**
- `gateway/platforms/telegram_test.go` — 4 unit tests for `newTelegramClient`

**Modified:**
- `gateway/platforms/telegram.go` — add `newTelegramTransport` + `newTelegramClient`; `NewTelegram` grows a `proxyURL` parameter
- `gateway/platforms/telegram_doh.go` — `DoHTransport` grows `proxyURL` field; `tryFallbackIP` routes fallback transport through it
- `gateway/platforms/descriptor_telegram.go` — add `proxy` FieldString; update `Build` + `Test` to thread it; inline a proxy-aware reachability probe in `testTelegram`
- `go.mod` — promote `golang.org/x/net/proxy` to a direct dependency (already pulled in indirectly)

**Possibly modified:**
- `gateway/platforms/descriptor_telegram_test.go` — if a test file exists, extend; otherwise create a minimal one pinning the new field

**Untouched:**
- `gateway/platforms/testprobe.go` — unchanged; `httpProbe` remains the default path for other platforms
- Every other platform descriptor + provider package
- `config/config.go`, `config/descriptor/` — no schema changes
- Web frontend — surfaces the new field via the existing descriptor-schema pipe

---

## Open questions carried to later stages

- **Generalization to a shared proxy helper.** If a second platform (Discord, WhatsApp) needs proxy support, extract the three-way switch into `internal/httpx` and migrate Telegram + the new platform together.
- **LLM provider proxy.** Anthropic + OpenAI-compat traffic is also China-blocked. When users file that issue, a follow-up stage adds `proxy` to `ProviderConfig` and threads it through `provider/factory`.
- **Global default + per-instance override.** The abandoned full-fleet design proposed `Config.Proxy` as a top-level default with per-instance overrides. If three or more platforms end up with proxy fields, revive that pattern rather than repeating the field-add dance.
- **Environment variable fallback.** Explicitly declined for this stage. If operators complain about duplicating `HTTPS_PROXY` into config, add `proxy_from_env: true` as an opt-in bool later.
