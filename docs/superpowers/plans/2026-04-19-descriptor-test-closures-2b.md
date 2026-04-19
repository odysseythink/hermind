# Descriptor Test Closures (Stage 2b) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Populate the `Test` closure on 12 of the 19 platform descriptors so `POST /api/platforms/{key}/test` actually probes the platform's auth/identity endpoint. Leave the 7 outbound-only webhook descriptors with `Test == nil` (they keep returning 501 — no non-destructive probe exists).

**Architecture:** Each probed descriptor gets a package-private `test<Type>(ctx, args..., baseURL)` function in its `descriptor_<type>.go` file. The closure hardcodes the real base URL; tests call the same function with an `httptest.NewServer` URL (or a direct `net.Listen` for the TCP-listen types). A shared `httpProbe` helper centralizes the request/status handling.

**Tech Stack:** Go 1.22 `net/http`, `net/http/httptest` for HTTP stubs, `net/smtp` for the email probe, `net` for TCP-listen probes.

**Probes by type:**

| Type | Probe |
|---|---|
| telegram | `GET <api>/bot<token>/getMe` |
| discord_bot | `GET <api>/users/@me` with `Authorization: Bot <token>` |
| slack_events | `POST <api>/api/auth.test` with `Authorization: Bearer <bot_token>` |
| whatsapp | `GET <api>/v20.0/<phone_id>` with `Authorization: Bearer <access_token>` |
| matrix | `GET <home_server>/_matrix/client/v3/account/whoami` with bearer |
| homeassistant | `GET <base_url>/api/` with bearer |
| mattermost_bot | `GET <base_url>/api/v4/users/me` with bearer |
| signal | `GET <base_url>/v1/about` (no auth) |
| sms | `GET https://api.twilio.com/2010-04-01/Accounts/<sid>.json` with basic auth |
| email | `net.Dial` SMTP host:port → EHLO → AUTH LOGIN (if creds) → QUIT |
| api_server | `net.Listen` on addr + immediate close |
| acp | `net.Listen` on addr + immediate close |

**Explicitly out of scope:** slack, discord, mattermost, feishu, dingtalk, wecom, webhook — these are outbound-only webhook posters with no non-destructive probe. `Test` stays nil; `/api/platforms/{key}/test` returns 501 per Stage 2a.

**Source of truth:** `docs/superpowers/specs/2026-04-19-web-im-config-design.md` §4 "Test implementation sketch per type" and the API contract in §5 (test endpoint returns `{ok: true}` when the probe succeeds).

---

## File Structure

**Create:**

- `gateway/platforms/testprobe.go` — shared `httpProbe(ctx, method, url, headers) error` helper.
- `gateway/platforms/testprobe_test.go` — tests for the helper.
- Per-platform tests are appended to each `descriptor_<type>_test.go` that doesn't yet exist — create the test file alongside the first Test addition.

**Modify** (12 descriptor files, one Test closure + one private probe fn each):

- `gateway/platforms/descriptor_telegram.go`
- `gateway/platforms/descriptor_discord_bot.go`
- `gateway/platforms/descriptor_slack_events.go`
- `gateway/platforms/descriptor_whatsapp.go`
- `gateway/platforms/descriptor_matrix.go`
- `gateway/platforms/descriptor_homeassistant.go`
- `gateway/platforms/descriptor_mattermost_bot.go`
- `gateway/platforms/descriptor_signal.go`
- `gateway/platforms/descriptor_sms.go`
- `gateway/platforms/descriptor_email.go`
- `gateway/platforms/descriptor_api_server.go`
- `gateway/platforms/descriptor_acp.go`

**Untouched:** the 7 outbound-webhook descriptors (`descriptor_{slack,discord,mattermost,feishu,dingtalk,wecom,webhook}.go`).

---

## Task 1: Shared `httpProbe` helper

**Files:**
- Create: `gateway/platforms/testprobe.go`
- Create: `gateway/platforms/testprobe_test.go`

- [ ] **Step 1: Write the failing tests**

Create `gateway/platforms/testprobe_test.go`:

```go
package platforms

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTPProbe_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	if err := httpProbe(context.Background(), "GET", srv.URL, nil); err != nil {
		t.Errorf("unexpected: %v", err)
	}
}

func TestHTTPProbe_ForwardsHeaders(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	err := httpProbe(context.Background(), "GET", srv.URL, map[string]string{
		"Authorization": "Bearer abc",
	})
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	if gotAuth != "Bearer abc" {
		t.Errorf("Authorization = %q, want %q", gotAuth, "Bearer abc")
	}
}

func TestHTTPProbe_NonSuccessStatusReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid token"}`))
	}))
	defer srv.Close()
	err := httpProbe(context.Background(), "GET", srv.URL, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("error should mention status: %v", err)
	}
}

func TestHTTPProbe_RespectsContextCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancel

	if err := httpProbe(ctx, "GET", srv.URL, nil); err == nil {
		t.Fatal("expected error for canceled context")
	}
}
```

- [ ] **Step 2: Run the tests — expect FAIL**

Run: `(cd <worktree> && go test ./gateway/platforms/ -run TestHTTPProbe -v)`

Expected: FAIL — `undefined: httpProbe`.

- [ ] **Step 3: Implement `testprobe.go`**

Create `gateway/platforms/testprobe.go`:

```go
package platforms

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// httpProbe sends an HTTP request and returns nil on 2xx status, or a
// user-facing error with a short body excerpt otherwise. Headers may be
// nil. Uses http.DefaultClient; the caller's ctx controls cancellation
// and deadline.
//
// Descriptor Test closures wrap this helper with platform-specific
// URL + header assembly. The /api/platforms/{key}/test handler surfaces
// the returned error.Error() verbatim as the `error` field of the JSON
// response body.
func httpProbe(ctx context.Context, method, url string, headers map[string]string) error {
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	snippet := strings.TrimSpace(string(body))
	if snippet == "" {
		return fmt.Errorf("probe failed: status %d", resp.StatusCode)
	}
	return fmt.Errorf("probe failed: status %d: %s", resp.StatusCode, snippet)
}
```

- [ ] **Step 4: Run the tests — confirm PASS**

Run: `(cd <worktree> && go test ./gateway/platforms/ -run TestHTTPProbe -v)`

Expected: all 4 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add gateway/platforms/testprobe.go gateway/platforms/testprobe_test.go
git commit -m "$(cat <<'EOF'
feat(gateway/platforms): add shared httpProbe helper

Centralises the "HTTP GET/POST and map status to error" logic used by
every descriptor Test closure landed in this stage. Respects the
caller's context for cancellation/deadline and truncates the error
body snippet to 512 bytes so users see useful feedback without a wall
of HTML.
EOF
)"
```

---

## Task 2: Telegram probe

**Files:**
- Modify: `gateway/platforms/descriptor_telegram.go`
- Create: `gateway/platforms/descriptor_telegram_test.go`

- [ ] **Step 1: Write the failing test**

Create `gateway/platforms/descriptor_telegram_test.go`:

```go
package platforms

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTelegram_Success(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"ok":true,"result":{"username":"testbot"}}`))
	}))
	defer srv.Close()

	err := testTelegram(context.Background(), "12345:abcdef", srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := "/bot12345:abcdef/getMe"; gotPath != want {
		t.Errorf("path = %q, want %q", gotPath, want)
	}
}

func TestTelegram_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"ok":false,"description":"Unauthorized"}`, http.StatusUnauthorized)
	}))
	defer srv.Close()

	err := testTelegram(context.Background(), "bad", srv.URL)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestTelegram_ClosureIsRegistered(t *testing.T) {
	d, ok := Get("telegram")
	if !ok || d.Test == nil {
		t.Fatal("telegram descriptor missing Test closure")
	}
}
```

- [ ] **Step 2: Run — expect FAIL**

Run: `(cd <worktree> && go test ./gateway/platforms/ -run TestTelegram -v)`

Expected: FAIL — `undefined: testTelegram`.

- [ ] **Step 3: Add the probe + closure to `descriptor_telegram.go`**

Read the current file, then replace its body with:

```go
package platforms

import (
	"context"
	"fmt"

	"github.com/odysseythink/hermind/gateway"
)

func init() {
	Register(Descriptor{
		Type:        "telegram",
		DisplayName: "Telegram Bot",
		Summary:     "Telegram Bot API — long-polling adapter.",
		Fields: []FieldSpec{
			{Name: "token", Label: "Bot Token", Kind: FieldSecret, Required: true,
				Help: "From @BotFather."},
		},
		Build: func(opts map[string]string) (gateway.Platform, error) {
			return NewTelegram(opts["token"]), nil
		},
		Test: func(ctx context.Context, opts map[string]string) error {
			return testTelegram(ctx, opts["token"], "https://api.telegram.org")
		},
	})
}

func testTelegram(ctx context.Context, token, baseURL string) error {
	if token == "" {
		return fmt.Errorf("telegram: token is empty")
	}
	return httpProbe(ctx, "GET", baseURL+"/bot"+token+"/getMe", nil)
}
```

- [ ] **Step 4: Run — confirm PASS**

Run: `(cd <worktree> && go test ./gateway/platforms/ -run 'TestTelegram|TestHTTPProbe' -v)`

Expected: all PASS. The `TestTelegram_ClosureIsRegistered` case confirms registry integration.

- [ ] **Step 5: Commit**

```bash
git add gateway/platforms/descriptor_telegram.go gateway/platforms/descriptor_telegram_test.go
git commit -m "feat(gateway/platforms): add telegram descriptor Test closure"
```

---

## Task 3: Bearer-auth HTTP probes (6 types)

Six descriptors share the shape "GET some endpoint with an Authorization header". This task adds probes + closures + tests for all six in one commit.

**Files:**
- Modify: `gateway/platforms/descriptor_discord_bot.go`
- Modify: `gateway/platforms/descriptor_slack_events.go`
- Modify: `gateway/platforms/descriptor_whatsapp.go`
- Modify: `gateway/platforms/descriptor_matrix.go`
- Modify: `gateway/platforms/descriptor_homeassistant.go`
- Modify: `gateway/platforms/descriptor_mattermost_bot.go`
- Create: `gateway/platforms/descriptor_discord_bot_test.go`
- Create: `gateway/platforms/descriptor_slack_events_test.go`
- Create: `gateway/platforms/descriptor_whatsapp_test.go`
- Create: `gateway/platforms/descriptor_matrix_test.go`
- Create: `gateway/platforms/descriptor_homeassistant_test.go`
- Create: `gateway/platforms/descriptor_mattermost_bot_test.go`

Each probe function lives in the descriptor file and follows the same shape as `testTelegram`. Six separate test files, each one written by lifting the `TestTelegram_Success` and `TestTelegram_Unauthorized` patterns and adjusting the URL path + auth header.

- [ ] **Step 1: Update `descriptor_discord_bot.go`**

Replace the file with:

```go
package platforms

import (
	"context"
	"fmt"

	"github.com/odysseythink/hermind/gateway"
)

func init() {
	Register(Descriptor{
		Type:        "discord_bot",
		DisplayName: "Discord Bot (REST poll)",
		Summary:     "Polls a single channel for new messages and replies in-thread.",
		Fields: []FieldSpec{
			{Name: "token", Label: "Bot Token", Kind: FieldSecret, Required: true},
			{Name: "channel_id", Label: "Channel ID", Kind: FieldString, Required: true,
				Help: "Numeric Discord channel snowflake."},
		},
		Build: func(opts map[string]string) (gateway.Platform, error) {
			return NewDiscordBot(opts["token"], opts["channel_id"]), nil
		},
		Test: func(ctx context.Context, opts map[string]string) error {
			return testDiscordBot(ctx, opts["token"], "https://discord.com/api/v10")
		},
	})
}

func testDiscordBot(ctx context.Context, token, baseURL string) error {
	if token == "" {
		return fmt.Errorf("discord_bot: token is empty")
	}
	return httpProbe(ctx, "GET", baseURL+"/users/@me", map[string]string{
		"Authorization": "Bot " + token,
	})
}
```

- [ ] **Step 2: Create `descriptor_discord_bot_test.go`**

```go
package platforms

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDiscordBot_Success(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		if r.URL.Path != "/users/@me" {
			http.Error(w, "wrong path", http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`{"id":"1"}`))
	}))
	defer srv.Close()

	if err := testDiscordBot(context.Background(), "tok", srv.URL); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if gotAuth != "Bot tok" {
		t.Errorf("Authorization = %q, want %q", gotAuth, "Bot tok")
	}
}

func TestDiscordBot_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"message":"401: Unauthorized"}`, http.StatusUnauthorized)
	}))
	defer srv.Close()

	if err := testDiscordBot(context.Background(), "bad", srv.URL); err == nil {
		t.Fatal("expected error")
	}
}
```

- [ ] **Step 3: Update `descriptor_slack_events.go`**

Replace:

```go
package platforms

import (
	"context"
	"fmt"

	"github.com/odysseythink/hermind/gateway"
)

func init() {
	Register(Descriptor{
		Type:        "slack_events",
		DisplayName: "Slack (Events API)",
		Summary:     "Bidirectional Slack integration via the Events API + chat.postMessage.",
		Fields: []FieldSpec{
			{Name: "addr", Label: "Listen Address", Kind: FieldString, Required: true,
				Help: `e.g. ":8082". Must match the URL Slack posts events to.`},
			{Name: "bot_token", Label: "Bot Token", Kind: FieldSecret, Required: true,
				Help: `"xoxb-..." — from Slack app OAuth settings.`},
		},
		Build: func(opts map[string]string) (gateway.Platform, error) {
			return NewSlackEvents(opts["addr"], opts["bot_token"]), nil
		},
		Test: func(ctx context.Context, opts map[string]string) error {
			return testSlackEvents(ctx, opts["bot_token"], "https://slack.com")
		},
	})
}

func testSlackEvents(ctx context.Context, botToken, baseURL string) error {
	if botToken == "" {
		return fmt.Errorf("slack_events: bot_token is empty")
	}
	return httpProbe(ctx, "POST", baseURL+"/api/auth.test", map[string]string{
		"Authorization": "Bearer " + botToken,
	})
}
```

- [ ] **Step 4: Create `descriptor_slack_events_test.go`**

```go
package platforms

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSlackEvents_Success(t *testing.T) {
	var gotMethod, gotPath, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath, gotAuth = r.Method, r.URL.Path, r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	if err := testSlackEvents(context.Background(), "xoxb-abc", srv.URL); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if gotMethod != "POST" || gotPath != "/api/auth.test" || gotAuth != "Bearer xoxb-abc" {
		t.Errorf("got method=%q path=%q auth=%q", gotMethod, gotPath, gotAuth)
	}
}

func TestSlackEvents_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"ok":false,"error":"invalid_auth"}`, http.StatusUnauthorized)
	}))
	defer srv.Close()

	if err := testSlackEvents(context.Background(), "bad", srv.URL); err == nil {
		t.Fatal("expected error")
	}
}
```

- [ ] **Step 5: Update `descriptor_whatsapp.go`**

Replace:

```go
package platforms

import (
	"context"
	"fmt"

	"github.com/odysseythink/hermind/gateway"
)

func init() {
	Register(Descriptor{
		Type:        "whatsapp",
		DisplayName: "WhatsApp Cloud API",
		Summary:     "Meta's WhatsApp Cloud API, outbound only.",
		Fields: []FieldSpec{
			{Name: "phone_id", Label: "Phone Number ID", Kind: FieldString, Required: true},
			{Name: "access_token", Label: "Access Token", Kind: FieldSecret, Required: true},
		},
		Build: func(opts map[string]string) (gateway.Platform, error) {
			return NewWhatsApp(opts["phone_id"], opts["access_token"]), nil
		},
		Test: func(ctx context.Context, opts map[string]string) error {
			return testWhatsApp(ctx, opts["phone_id"], opts["access_token"], "https://graph.facebook.com")
		},
	})
}

func testWhatsApp(ctx context.Context, phoneID, accessToken, baseURL string) error {
	if phoneID == "" || accessToken == "" {
		return fmt.Errorf("whatsapp: phone_id and access_token are required")
	}
	return httpProbe(ctx, "GET", baseURL+"/v20.0/"+phoneID, map[string]string{
		"Authorization": "Bearer " + accessToken,
	})
}
```

- [ ] **Step 6: Create `descriptor_whatsapp_test.go`**

```go
package platforms

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWhatsApp_Success(t *testing.T) {
	var gotPath, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotAuth = r.URL.Path, r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"id":"42"}`))
	}))
	defer srv.Close()

	if err := testWhatsApp(context.Background(), "42", "abc", srv.URL); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if gotPath != "/v20.0/42" || gotAuth != "Bearer abc" {
		t.Errorf("path=%q auth=%q", gotPath, gotAuth)
	}
}

func TestWhatsApp_MissingCreds(t *testing.T) {
	if err := testWhatsApp(context.Background(), "", "abc", "http://unused"); err == nil {
		t.Error("expected error for empty phone_id")
	}
	if err := testWhatsApp(context.Background(), "42", "", "http://unused"); err == nil {
		t.Error("expected error for empty access_token")
	}
}
```

- [ ] **Step 7: Update `descriptor_matrix.go`**

Replace:

```go
package platforms

import (
	"context"
	"fmt"
	"strings"

	"github.com/odysseythink/hermind/gateway"
)

func init() {
	Register(Descriptor{
		Type:        "matrix",
		DisplayName: "Matrix",
		Summary:     "Matrix client-server API (v3). Joins a single room.",
		Fields: []FieldSpec{
			{Name: "home_server", Label: "Home Server", Kind: FieldString, Required: true,
				Help: `e.g. "https://matrix.org".`},
			{Name: "access_token", Label: "Access Token", Kind: FieldSecret, Required: true},
			{Name: "room_id", Label: "Room ID", Kind: FieldString, Required: true,
				Help: `Internal room id, e.g. "!abcd:matrix.org".`},
		},
		Build: func(opts map[string]string) (gateway.Platform, error) {
			return NewMatrix(opts["home_server"], opts["access_token"], opts["room_id"]), nil
		},
		Test: func(ctx context.Context, opts map[string]string) error {
			return testMatrix(ctx, opts["home_server"], opts["access_token"])
		},
	})
}

func testMatrix(ctx context.Context, homeServer, accessToken string) error {
	if homeServer == "" || accessToken == "" {
		return fmt.Errorf("matrix: home_server and access_token are required")
	}
	base := strings.TrimRight(homeServer, "/")
	return httpProbe(ctx, "GET", base+"/_matrix/client/v3/account/whoami", map[string]string{
		"Authorization": "Bearer " + accessToken,
	})
}
```

- [ ] **Step 8: Create `descriptor_matrix_test.go`**

```go
package platforms

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMatrix_Success(t *testing.T) {
	var gotPath, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotAuth = r.URL.Path, r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"user_id":"@u:m"}`))
	}))
	defer srv.Close()

	if err := testMatrix(context.Background(), srv.URL, "tok"); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if gotPath != "/_matrix/client/v3/account/whoami" || gotAuth != "Bearer tok" {
		t.Errorf("path=%q auth=%q", gotPath, gotAuth)
	}
}

func TestMatrix_TrimsTrailingSlashOnHomeServer(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	if err := testMatrix(context.Background(), srv.URL+"/", "tok"); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if gotPath != "/_matrix/client/v3/account/whoami" {
		t.Errorf("path = %q (double slash?)", gotPath)
	}
}
```

- [ ] **Step 9: Update `descriptor_homeassistant.go`**

Replace:

```go
package platforms

import (
	"context"
	"fmt"
	"strings"

	"github.com/odysseythink/hermind/gateway"
)

func init() {
	Register(Descriptor{
		Type:        "homeassistant",
		DisplayName: "Home Assistant (Notify)",
		Summary:     "Calls a Home Assistant notify service.",
		Fields: []FieldSpec{
			{Name: "base_url", Label: "Base URL", Kind: FieldString, Required: true,
				Help: `e.g. "http://homeassistant.local:8123".`},
			{Name: "access_token", Label: "Long-Lived Access Token", Kind: FieldSecret, Required: true},
			{Name: "service", Label: "Notify Service", Kind: FieldString,
				Default: "notify",
				Help:    `Service under notify.*; e.g. "mobile_app_my_phone".`},
		},
		Build: func(opts map[string]string) (gateway.Platform, error) {
			svc := opts["service"]
			if svc == "" {
				svc = "notify"
			}
			return NewHomeAssistant(opts["base_url"], opts["access_token"], svc), nil
		},
		Test: func(ctx context.Context, opts map[string]string) error {
			return testHomeAssistant(ctx, opts["base_url"], opts["access_token"])
		},
	})
}

func testHomeAssistant(ctx context.Context, baseURL, accessToken string) error {
	if baseURL == "" || accessToken == "" {
		return fmt.Errorf("homeassistant: base_url and access_token are required")
	}
	base := strings.TrimRight(baseURL, "/")
	return httpProbe(ctx, "GET", base+"/api/", map[string]string{
		"Authorization": "Bearer " + accessToken,
	})
}
```

- [ ] **Step 10: Create `descriptor_homeassistant_test.go`**

```go
package platforms

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHomeAssistant_Success(t *testing.T) {
	var gotPath, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotAuth = r.URL.Path, r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"message":"API running."}`))
	}))
	defer srv.Close()

	if err := testHomeAssistant(context.Background(), srv.URL, "tok"); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if gotPath != "/api/" || gotAuth != "Bearer tok" {
		t.Errorf("path=%q auth=%q", gotPath, gotAuth)
	}
}
```

- [ ] **Step 11: Update `descriptor_mattermost_bot.go`**

Replace:

```go
package platforms

import (
	"context"
	"fmt"
	"strings"

	"github.com/odysseythink/hermind/gateway"
)

func init() {
	Register(Descriptor{
		Type:        "mattermost_bot",
		DisplayName: "Mattermost Bot (REST poll)",
		Summary:     "Polls a single channel for mentions and replies via the REST API.",
		Fields: []FieldSpec{
			{Name: "base_url", Label: "Server Base URL", Kind: FieldString, Required: true,
				Help: `e.g. "https://mm.example.com".`},
			{Name: "token", Label: "Personal Access Token", Kind: FieldSecret, Required: true},
			{Name: "channel_id", Label: "Channel ID", Kind: FieldString, Required: true},
		},
		Build: func(opts map[string]string) (gateway.Platform, error) {
			return NewMattermostBot(opts["base_url"], opts["token"], opts["channel_id"]), nil
		},
		Test: func(ctx context.Context, opts map[string]string) error {
			return testMattermostBot(ctx, opts["base_url"], opts["token"])
		},
	})
}

func testMattermostBot(ctx context.Context, baseURL, token string) error {
	if baseURL == "" || token == "" {
		return fmt.Errorf("mattermost_bot: base_url and token are required")
	}
	base := strings.TrimRight(baseURL, "/")
	return httpProbe(ctx, "GET", base+"/api/v4/users/me", map[string]string{
		"Authorization": "Bearer " + token,
	})
}
```

- [ ] **Step 12: Create `descriptor_mattermost_bot_test.go`**

```go
package platforms

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMattermostBot_Success(t *testing.T) {
	var gotPath, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotAuth = r.URL.Path, r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"id":"u1"}`))
	}))
	defer srv.Close()

	if err := testMattermostBot(context.Background(), srv.URL, "tok"); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if gotPath != "/api/v4/users/me" || gotAuth != "Bearer tok" {
		t.Errorf("path=%q auth=%q", gotPath, gotAuth)
	}
}
```

- [ ] **Step 13: Run all Stage 2b tests so far**

Run: `(cd <worktree> && go test ./gateway/platforms/ -run 'TestHTTPProbe|TestTelegram|TestDiscordBot|TestSlackEvents|TestWhatsApp|TestMatrix|TestHomeAssistant|TestMattermostBot' -v)`

Expected: all tests PASS.

- [ ] **Step 14: Commit**

```bash
git add gateway/platforms/descriptor_discord_bot.go gateway/platforms/descriptor_slack_events.go \
        gateway/platforms/descriptor_whatsapp.go gateway/platforms/descriptor_matrix.go \
        gateway/platforms/descriptor_homeassistant.go gateway/platforms/descriptor_mattermost_bot.go \
        gateway/platforms/descriptor_discord_bot_test.go gateway/platforms/descriptor_slack_events_test.go \
        gateway/platforms/descriptor_whatsapp_test.go gateway/platforms/descriptor_matrix_test.go \
        gateway/platforms/descriptor_homeassistant_test.go gateway/platforms/descriptor_mattermost_bot_test.go
git commit -m "$(cat <<'EOF'
feat(gateway/platforms): bearer-auth Test closures (6 platforms)

Adds httpProbe-driven Test closures for discord_bot, slack_events,
whatsapp, matrix, homeassistant, and mattermost_bot. Each probe hits
the adapter's identity or auth endpoint with the platform-specific
Authorization header (Bot/Bearer) and returns a descriptive error on
non-2xx status.
EOF
)"
```

---

## Task 4: Signal + Twilio SMS probes

Signal has no auth; SMS uses HTTP Basic auth with base64-encoded credentials.

**Files:**
- Modify: `gateway/platforms/descriptor_signal.go`
- Modify: `gateway/platforms/descriptor_sms.go`
- Create: `gateway/platforms/descriptor_signal_test.go`
- Create: `gateway/platforms/descriptor_sms_test.go`

- [ ] **Step 1: Update `descriptor_signal.go`**

Replace:

```go
package platforms

import (
	"context"
	"fmt"
	"strings"

	"github.com/odysseythink/hermind/gateway"
)

func init() {
	Register(Descriptor{
		Type:        "signal",
		DisplayName: "Signal (signal-cli REST)",
		Summary:     "Requires a running signal-cli REST API.",
		Fields: []FieldSpec{
			{Name: "base_url", Label: "signal-cli Base URL", Kind: FieldString, Required: true,
				Help: `e.g. "http://localhost:8080".`},
			{Name: "account", Label: "Account", Kind: FieldString, Required: true,
				Help: "Registered phone number in E.164 form."},
		},
		Build: func(opts map[string]string) (gateway.Platform, error) {
			return NewSignal(opts["base_url"], opts["account"]), nil
		},
		Test: func(ctx context.Context, opts map[string]string) error {
			return testSignal(ctx, opts["base_url"])
		},
	})
}

func testSignal(ctx context.Context, baseURL string) error {
	if baseURL == "" {
		return fmt.Errorf("signal: base_url is required")
	}
	base := strings.TrimRight(baseURL, "/")
	return httpProbe(ctx, "GET", base+"/v1/about", nil)
}
```

- [ ] **Step 2: Create `descriptor_signal_test.go`**

```go
package platforms

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSignal_Success(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"version":"0.13.0"}`))
	}))
	defer srv.Close()

	if err := testSignal(context.Background(), srv.URL); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if gotPath != "/v1/about" {
		t.Errorf("path = %q", gotPath)
	}
}

func TestSignal_MissingBaseURL(t *testing.T) {
	if err := testSignal(context.Background(), ""); err == nil {
		t.Error("expected error")
	}
}
```

- [ ] **Step 3: Update `descriptor_sms.go`**

Replace:

```go
package platforms

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/odysseythink/hermind/gateway"
)

func init() {
	Register(Descriptor{
		Type:        "sms",
		DisplayName: "SMS (Twilio)",
		Summary:     "Twilio REST API — outbound SMS only.",
		Fields: []FieldSpec{
			{Name: "account_sid", Label: "Account SID", Kind: FieldSecret, Required: true,
				Help: `Twilio AC... account identifier.`},
			{Name: "auth_token", Label: "Auth Token", Kind: FieldSecret, Required: true},
			{Name: "from", Label: "From Number", Kind: FieldString, Required: true,
				Help: "E.164 phone number registered with Twilio."},
			{Name: "to", Label: "To Number", Kind: FieldString, Required: true,
				Help: "Destination phone number in E.164."},
		},
		Build: func(opts map[string]string) (gateway.Platform, error) {
			return NewSMS(opts["account_sid"], opts["auth_token"], opts["from"], opts["to"]), nil
		},
		Test: func(ctx context.Context, opts map[string]string) error {
			return testSMS(ctx, opts["account_sid"], opts["auth_token"], "https://api.twilio.com")
		},
	})
}

func testSMS(ctx context.Context, sid, token, baseURL string) error {
	if sid == "" || token == "" {
		return fmt.Errorf("sms: account_sid and auth_token are required")
	}
	cred := base64.StdEncoding.EncodeToString([]byte(sid + ":" + token))
	return httpProbe(ctx, "GET", baseURL+"/2010-04-01/Accounts/"+sid+".json", map[string]string{
		"Authorization": "Basic " + cred,
	})
}
```

- [ ] **Step 4: Create `descriptor_sms_test.go`**

```go
package platforms

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSMS_Success(t *testing.T) {
	var gotPath, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotAuth = r.URL.Path, r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"sid":"ACabc","status":"active"}`))
	}))
	defer srv.Close()

	if err := testSMS(context.Background(), "ACabc", "tok", srv.URL); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if gotPath != "/2010-04-01/Accounts/ACabc.json" {
		t.Errorf("path = %q", gotPath)
	}
	if !strings.HasPrefix(gotAuth, "Basic ") {
		t.Fatalf("Authorization = %q, want Basic …", gotAuth)
	}
	decoded, _ := base64.StdEncoding.DecodeString(strings.TrimPrefix(gotAuth, "Basic "))
	if string(decoded) != "ACabc:tok" {
		t.Errorf("decoded creds = %q, want %q", decoded, "ACabc:tok")
	}
}

func TestSMS_MissingCreds(t *testing.T) {
	if err := testSMS(context.Background(), "", "tok", "http://unused"); err == nil {
		t.Error("expected error for empty sid")
	}
	if err := testSMS(context.Background(), "AC", "", "http://unused"); err == nil {
		t.Error("expected error for empty token")
	}
}
```

- [ ] **Step 5: Run the new tests**

Run: `(cd <worktree> && go test ./gateway/platforms/ -run 'TestSignal|TestSMS' -v)`

Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add gateway/platforms/descriptor_signal.go gateway/platforms/descriptor_signal_test.go \
        gateway/platforms/descriptor_sms.go gateway/platforms/descriptor_sms_test.go
git commit -m "$(cat <<'EOF'
feat(gateway/platforms): signal + sms Test closures

Signal probes GET /v1/about on the signal-cli REST server (no auth).
SMS probes GET /2010-04-01/Accounts/<sid>.json on Twilio with HTTP
Basic auth derived from the account_sid + auth_token pair.
EOF
)"
```

---

## Task 5: Email (SMTP) probe

Dial the SMTP host, send EHLO, run AUTH LOGIN if credentials are provided, then QUIT. No message is sent.

**Files:**
- Modify: `gateway/platforms/descriptor_email.go`
- Create: `gateway/platforms/descriptor_email_test.go`

- [ ] **Step 1: Write the failing test**

Create `gateway/platforms/descriptor_email_test.go`:

```go
package platforms

import (
	"bufio"
	"context"
	"net"
	"strings"
	"testing"
	"time"
)

// startFakeSMTP starts a tiny SMTP server that announces EHLO, accepts
// AUTH LOGIN (ignoring the credentials), and replies 221 to QUIT. It
// returns the listener's "host:port" so tests can point testEmail at it.
func startFakeSMTP(t *testing.T, rejectAuth bool) (hostPort string, close func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
		rw := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))
		_, _ = rw.WriteString("220 fake.example.com ESMTP ready\r\n")
		_ = rw.Flush()
		for {
			line, err := rw.ReadString('\n')
			if err != nil {
				return
			}
			line = strings.TrimRight(line, "\r\n")
			switch {
			case strings.HasPrefix(line, "EHLO"), strings.HasPrefix(line, "HELO"):
				_, _ = rw.WriteString("250-fake.example.com hello\r\n")
				_, _ = rw.WriteString("250 AUTH LOGIN\r\n")
				_ = rw.Flush()
			case strings.HasPrefix(line, "AUTH LOGIN"):
				if rejectAuth {
					_, _ = rw.WriteString("535 5.7.8 authentication failed\r\n")
					_ = rw.Flush()
					return
				}
				_, _ = rw.WriteString("334 VXNlcm5hbWU6\r\n") // "Username:"
				_ = rw.Flush()
				_, _ = rw.ReadString('\n') // user b64
				_, _ = rw.WriteString("334 UGFzc3dvcmQ6\r\n") // "Password:"
				_ = rw.Flush()
				_, _ = rw.ReadString('\n') // pass b64
				_, _ = rw.WriteString("235 2.7.0 Authentication succeeded\r\n")
				_ = rw.Flush()
			case strings.HasPrefix(line, "QUIT"):
				_, _ = rw.WriteString("221 2.0.0 Bye\r\n")
				_ = rw.Flush()
				return
			case strings.HasPrefix(line, "NOOP"):
				_, _ = rw.WriteString("250 2.0.0 OK\r\n")
				_ = rw.Flush()
			default:
				_, _ = rw.WriteString("502 5.5.1 unknown\r\n")
				_ = rw.Flush()
			}
		}
	}()
	return ln.Addr().String(), func() { _ = ln.Close(); <-done }
}

func TestEmail_SuccessWithAuth(t *testing.T) {
	hostPort, stop := startFakeSMTP(t, false)
	defer stop()
	host, port, _ := net.SplitHostPort(hostPort)

	if err := testEmail(context.Background(), host, port, "u", "p"); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestEmail_SuccessNoAuth(t *testing.T) {
	hostPort, stop := startFakeSMTP(t, false)
	defer stop()
	host, port, _ := net.SplitHostPort(hostPort)

	// Empty username/password means "don't AUTH".
	if err := testEmail(context.Background(), host, port, "", ""); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestEmail_AuthRejected(t *testing.T) {
	hostPort, stop := startFakeSMTP(t, true)
	defer stop()
	host, port, _ := net.SplitHostPort(hostPort)

	if err := testEmail(context.Background(), host, port, "u", "p"); err == nil {
		t.Fatal("expected auth error")
	}
}

func TestEmail_MissingHost(t *testing.T) {
	if err := testEmail(context.Background(), "", "587", "", ""); err == nil {
		t.Error("expected error for empty host")
	}
}
```

- [ ] **Step 2: Run — expect FAIL**

Run: `(cd <worktree> && go test ./gateway/platforms/ -run TestEmail -v)`

Expected: FAIL — `undefined: testEmail`.

- [ ] **Step 3: Update `descriptor_email.go`**

Replace:

```go
package platforms

import (
	"context"
	"fmt"
	"net"
	"net/smtp"

	"github.com/odysseythink/hermind/gateway"
)

func init() {
	Register(Descriptor{
		Type:        "email",
		DisplayName: "Email (SMTP)",
		Summary:     "Sends outbound messages via an SMTP server.",
		Fields: []FieldSpec{
			{Name: "host", Label: "SMTP Host", Kind: FieldString, Required: true,
				Help: `e.g. "smtp.example.com".`},
			{Name: "port", Label: "SMTP Port", Kind: FieldString,
				Default: "587",
				Help:    `Submission port; typically 587 for STARTTLS, 465 for implicit TLS.`},
			{Name: "username", Label: "Username", Kind: FieldString},
			{Name: "password", Label: "Password", Kind: FieldSecret},
			{Name: "from", Label: "From", Kind: FieldString, Required: true,
				Help: "Sender address; must be allowed by the SMTP server."},
			{Name: "to", Label: "To", Kind: FieldString, Required: true,
				Help: "Comma-separated recipient addresses."},
		},
		Build: func(opts map[string]string) (gateway.Platform, error) {
			port := opts["port"]
			if port == "" {
				port = "587"
			}
			return NewEmail(
				opts["host"], port,
				opts["username"], opts["password"],
				opts["from"], opts["to"],
			), nil
		},
		Test: func(ctx context.Context, opts map[string]string) error {
			port := opts["port"]
			if port == "" {
				port = "587"
			}
			return testEmail(ctx, opts["host"], port, opts["username"], opts["password"])
		},
	})
}

// testEmail dials the SMTP submission port, runs EHLO, attempts
// AUTH LOGIN when credentials are supplied, and hangs up. No message
// is sent. Respects ctx for dial deadline.
func testEmail(ctx context.Context, host, port, user, pass string) error {
	if host == "" {
		return fmt.Errorf("email: host is required")
	}
	d := net.Dialer{}
	conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(host, port))
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return fmt.Errorf("smtp client: %w", err)
	}
	defer client.Quit()

	if err := client.Hello("hermind.test"); err != nil {
		return fmt.Errorf("ehlo: %w", err)
	}
	if user == "" && pass == "" {
		return nil
	}
	auth := smtp.PlainAuth("", user, pass, host)
	if err := client.Auth(auth); err != nil {
		// Fall back to AUTH LOGIN for servers that don't advertise PLAIN.
		if err2 := client.Auth(loginAuth(user, pass)); err2 != nil {
			return fmt.Errorf("auth: %w", err)
		}
	}
	return nil
}

// loginAuth returns an smtp.Auth implementation that performs the
// AUTH LOGIN exchange (not RFC 4954 SASL PLAIN). Some servers only
// advertise LOGIN, so we use this as a fallback.
func loginAuth(user, pass string) smtp.Auth { return &authLogin{user: user, pass: pass} }

type authLogin struct{ user, pass string }

func (a *authLogin) Start(_ *smtp.ServerInfo) (string, []byte, error) {
	return "LOGIN", nil, nil
}
func (a *authLogin) Next(fromServer []byte, more bool) ([]byte, error) {
	if !more {
		return nil, nil
	}
	switch string(fromServer) {
	case "Username:":
		return []byte(a.user), nil
	case "Password:":
		return []byte(a.pass), nil
	}
	return nil, nil
}
```

- [ ] **Step 4: Run — confirm PASS**

Run: `(cd <worktree> && go test ./gateway/platforms/ -run TestEmail -v)`

Expected: all 4 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add gateway/platforms/descriptor_email.go gateway/platforms/descriptor_email_test.go
git commit -m "$(cat <<'EOF'
feat(gateway/platforms): email SMTP Test closure

Dials host:port, runs EHLO, attempts PLAIN auth with a LOGIN fallback,
then QUITs without sending a message. Empty username+password means
"check connectivity only". Respects ctx for the dial deadline.
EOF
)"
```

---

## Task 6: api_server + ACP TCP-listen probes

Both verify the configured `addr` can be bound by Listen then immediately released. No network traffic outside the local host.

**Files:**
- Modify: `gateway/platforms/descriptor_api_server.go`
- Modify: `gateway/platforms/descriptor_acp.go`
- Create: `gateway/platforms/descriptor_api_server_test.go`
- Create: `gateway/platforms/descriptor_acp_test.go`

- [ ] **Step 1: Update `descriptor_api_server.go`**

Replace:

```go
package platforms

import (
	"context"
	"fmt"
	"net"

	"github.com/odysseythink/hermind/gateway"
)

func init() {
	Register(Descriptor{
		Type:        "api_server",
		DisplayName: "Generic API Server",
		Summary:     "Accepts inbound messages via HTTP POST; emits outbound via callback.",
		Fields: []FieldSpec{
			{Name: "addr", Label: "Listen Address", Kind: FieldString,
				Default: ":8080",
				Help:    `e.g. ":8080".`},
		},
		Build: func(opts map[string]string) (gateway.Platform, error) {
			addr := opts["addr"]
			if addr == "" {
				addr = ":8080"
			}
			return NewAPIServer(addr), nil
		},
		Test: func(ctx context.Context, opts map[string]string) error {
			addr := opts["addr"]
			if addr == "" {
				addr = ":8080"
			}
			return testListen(ctx, addr)
		},
	})
}

// testListen opens a TCP listener on addr and closes it immediately.
// A successful bind proves the address is syntactically valid and
// not already in use.
func testListen(ctx context.Context, addr string) error {
	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	return ln.Close()
}
```

- [ ] **Step 2: Create `descriptor_api_server_test.go`**

```go
package platforms

import (
	"context"
	"net"
	"testing"
)

func TestAPIServer_Success(t *testing.T) {
	if err := testListen(context.Background(), "127.0.0.1:0"); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestAPIServer_PortInUse(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("setup listener: %v", err)
	}
	defer ln.Close()

	if err := testListen(context.Background(), ln.Addr().String()); err == nil {
		t.Error("expected bind conflict error")
	}
}

func TestAPIServer_InvalidAddr(t *testing.T) {
	if err := testListen(context.Background(), "not a real addr"); err == nil {
		t.Error("expected parse error")
	}
}
```

- [ ] **Step 3: Update `descriptor_acp.go`**

Replace:

```go
package platforms

import (
	"context"

	"github.com/odysseythink/hermind/gateway"
)

func init() {
	Register(Descriptor{
		Type:        "acp",
		DisplayName: "ACP Server",
		Summary:     "Agent Client Protocol HTTP server.",
		Fields: []FieldSpec{
			{Name: "addr", Label: "Listen Address", Kind: FieldString, Required: true,
				Help: `e.g. ":9000".`},
			{Name: "token", Label: "Shared Token", Kind: FieldSecret, Required: true,
				Help: "Clients must present this as a bearer token."},
		},
		Build: func(opts map[string]string) (gateway.Platform, error) {
			return NewACP(opts["addr"], opts["token"]), nil
		},
		Test: func(ctx context.Context, opts map[string]string) error {
			return testListen(ctx, opts["addr"])
		},
	})
}
```

(Reuses `testListen` from `descriptor_api_server.go` — same package, so no extra helper needed.)

- [ ] **Step 4: Create `descriptor_acp_test.go`**

```go
package platforms

import (
	"context"
	"testing"
)

func TestACP_ClosureDelegatesToTestListen(t *testing.T) {
	d, ok := Get("acp")
	if !ok || d.Test == nil {
		t.Fatal("acp descriptor missing Test closure")
	}
	err := d.Test(context.Background(), map[string]string{"addr": "127.0.0.1:0"})
	if err != nil {
		t.Errorf("unexpected: %v", err)
	}
}
```

- [ ] **Step 5: Run the new tests**

Run: `(cd <worktree> && go test ./gateway/platforms/ -run 'TestAPIServer|TestACP' -v)`

Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add gateway/platforms/descriptor_api_server.go gateway/platforms/descriptor_api_server_test.go \
        gateway/platforms/descriptor_acp.go gateway/platforms/descriptor_acp_test.go
git commit -m "$(cat <<'EOF'
feat(gateway/platforms): api_server + acp listen probes

Both descriptors share a testListen helper: net.Listen on addr then
immediate Close. Proves the address syntax is valid and the port is
free without opening any outbound network traffic.
EOF
)"
```

---

## Task 7: Final verification

**Files:** none modified — verification only.

- [ ] **Step 1: Full test sweep**

Run:

```bash
(cd <worktree> && go test ./gateway/platforms/...)
(cd <worktree> && go test ./cli/... ./api/... ./gateway/...)
```

Expected: all PASS.

- [ ] **Step 2: Vet + build**

Run:

```bash
(cd <worktree> && go vet ./gateway/platforms/... ./cli/... ./api/...)
(cd <worktree> && go build ./...)
```

Expected: vet clean, build exit 0.

- [ ] **Step 3: Count the probed descriptors**

Run:

```bash
(cd <worktree> && grep -c "Test: func" gateway/platforms/descriptor_*.go | grep ":1$" | cut -d: -f1 | wc -l)
```

Expected: `12` — exactly the 12 descriptors listed in this plan. If the number is 7 (only outbound webhooks) or 19 (everything), something is off — investigate before moving on.

- [ ] **Step 4: E2E via `curl` — /test actually runs a probe**

Follow the Stage 2a E2E recipe but now exercise the `/test` endpoint against a real adapter whose probe fails with a clear error:

```bash
# (with hermind web running on $URL + $TOK from Stage 2a)
curl -s -X PUT -H "Authorization: Bearer $TOK" -H "Content-Type: application/json" \
  -d '{"config":{"gateway":{"platforms":{"tg_test":{"enabled":true,"type":"telegram","options":{"token":"definitely-not-a-real-token"}}}}}}' \
  $URL/api/config

curl -s -X POST -H "Authorization: Bearer $TOK" $URL/api/platforms/tg_test/test
```

Expected shape (token is bogus, so probe fails with a Telegram 404):

```json
{"ok":false,"error":"probe failed: status 404: ..."}
```

If the response is `{"ok":true}` the bogus token was accepted — stop, something is wrong. If it's the 501 `{"error":"test not implemented..."}` from Stage 2a, the descriptor Test closure isn't registered — investigate.

- [ ] **Step 5: Final commit history check**

Run: `(cd <worktree> && git log --oneline <stage-2b-base>..HEAD)`

Expected: 6 feat/chore commits plus the `docs(plan)` commit at the base — one per task above, describing what it delivered. No stray commits touching non-descriptor files.

---

## Rollback

`git reset --hard <commit-before-task-1>` on the feature branch. No on-disk state, no migrations. A dropped Stage 2b simply puts the `/api/platforms/{key}/test` endpoint back to 501 for every type.

## Scope deltas worth flagging

1. **7 outbound-webhook types skipped.** slack, discord, mattermost, feishu, dingtalk, wecom, webhook keep `Test: nil`. Their `/test` endpoint stays 501. Documented in the plan goal; not a regression.
2. **API version strings are hardcoded.** `telegram` uses the path form (no version). `discord_bot` hardcodes `/api/v10`. `whatsapp` hardcodes `/v20.0`. If any platform bumps its API version this has to change. Accepted as Stage 2b scope — follow-up plan can make these configurable.
3. **No rate-limit awareness.** The probe hits the live platform once per user-initiated test. High-frequency clicking on the Stage 3 UI could trigger Telegram / Slack's rate limiter. Accepted; Stage 3 frontend will debounce the Test button.
