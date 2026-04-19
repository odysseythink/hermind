# Platform Registry Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the hand-rolled 19-case switch in `cli/gateway.go::buildPlatform` with a self-registering `gateway/platforms` descriptor registry — the single source of truth for every gateway platform type, its configurable fields, and how to instantiate it.

**Architecture:** Each adapter declares its `Descriptor` (type id, display name, field spec list, `Build` closure) in a new `descriptor_<type>.go` file alongside the adapter code. A package-level `init()` in each file calls `platforms.Register(Descriptor{...})`. `buildPlatform` collapses to a single `platforms.Get(type)` lookup that delegates to the descriptor's `Build`. This is **stage 1 of 5** in the spec at `docs/superpowers/specs/2026-04-19-web-im-config-design.md`; no REST or frontend change happens here.

**Tech Stack:** Go 1.22, `testing` package with table-driven tests, no new third-party deps.

**Out of scope for this plan** (covered in later stages): the `Test` closure on Descriptor is declared but left `nil` on every descriptor for now — the `/api/platforms/test` endpoint in stage 2 is what populates it. REST, frontend, and CI checks are stages 2–5.

**Source of truth for what must be covered:** the existing switch at `cli/gateway.go:140-200`. Every type listed there must have a descriptor after this plan runs, and `buildPlatform` must produce byte-equivalent platform instances for every type×valid-options input it accepted before.

---

## File Structure

**Create:**
- `gateway/platforms/registry.go` — the `FieldKind`, `FieldSpec`, `Descriptor` types and `Register` / `Get` / `All` helpers.
- `gateway/platforms/registry_test.go` — unit tests for the registry primitives and descriptor sanity invariants.
- `gateway/platforms/descriptor_parity_test.go` — the single guard test that enumerates every type the old switch accepted and asserts the registry produces a non-nil `gateway.Platform` for each.
- `gateway/platforms/descriptor_<type>.go` — 19 files, one per type. Each file contains a single `init()` that calls `Register`.

**Modify:**
- `cli/gateway.go` — replace the `buildPlatform` switch body (lines 140–200) with a registry lookup.

**Untouched in stage 1:**
- Every adapter implementation file (`telegram.go`, `discord_bot.go`, `chatbots.go`, …). They stay byte-identical. Descriptors live in separate files so a reader can review the registration without scrolling through adapter logic.

---

## Task 1: Registry primitives and types

**Files:**
- Create: `gateway/platforms/registry.go`
- Create: `gateway/platforms/registry_test.go`

- [ ] **Step 1: Write the failing test for Register + Get + All**

Create `gateway/platforms/registry_test.go`:

```go
package platforms

import (
	"context"
	"testing"

	"github.com/odysseythink/hermind/gateway"
)

// stubPlatform is a minimal gateway.Platform used only for registry tests.
type stubPlatform struct{ name string }

func (s *stubPlatform) Name() string                                        { return s.name }
func (s *stubPlatform) Run(ctx context.Context, h gateway.MessageHandler) error { <-ctx.Done(); return nil }
func (s *stubPlatform) SendReply(ctx context.Context, out gateway.OutgoingMessage) error { return nil }

func TestRegister_GetReturnsRegistered(t *testing.T) {
	resetRegistryForTest(t)

	d := Descriptor{
		Type:        "stub",
		DisplayName: "Stub",
		Build: func(opts map[string]string) (gateway.Platform, error) {
			return &stubPlatform{name: "stub"}, nil
		},
	}
	Register(d)

	got, ok := Get("stub")
	if !ok {
		t.Fatalf("Get(\"stub\"): ok=false, want true")
	}
	if got.Type != "stub" {
		t.Errorf("got.Type = %q, want %q", got.Type, "stub")
	}
}

func TestGet_MissingReturnsFalse(t *testing.T) {
	resetRegistryForTest(t)
	if _, ok := Get("does-not-exist"); ok {
		t.Fatal("Get of unknown type returned ok=true")
	}
}

func TestAll_ReturnsSortedByType(t *testing.T) {
	resetRegistryForTest(t)
	Register(Descriptor{Type: "zeta", Build: mustBuildStub("zeta")})
	Register(Descriptor{Type: "alpha", Build: mustBuildStub("alpha")})
	Register(Descriptor{Type: "mu", Build: mustBuildStub("mu")})

	got := All()
	if len(got) != 3 {
		t.Fatalf("len(All()) = %d, want 3", len(got))
	}
	want := []string{"alpha", "mu", "zeta"}
	for i, w := range want {
		if got[i].Type != w {
			t.Errorf("All()[%d].Type = %q, want %q", i, got[i].Type, w)
		}
	}
}

func TestRegister_DuplicateTypeOverwrites(t *testing.T) {
	resetRegistryForTest(t)
	Register(Descriptor{Type: "dup", DisplayName: "first", Build: mustBuildStub("dup")})
	Register(Descriptor{Type: "dup", DisplayName: "second", Build: mustBuildStub("dup")})

	got, _ := Get("dup")
	if got.DisplayName != "second" {
		t.Errorf("after overwrite, DisplayName = %q, want %q", got.DisplayName, "second")
	}
}

func mustBuildStub(name string) func(map[string]string) (gateway.Platform, error) {
	return func(map[string]string) (gateway.Platform, error) {
		return &stubPlatform{name: name}, nil
	}
}

// resetRegistryForTest swaps in a fresh map for the current test and
// restores the original after t finishes.
func resetRegistryForTest(t *testing.T) {
	t.Helper()
	saved := registry
	registry = map[string]Descriptor{}
	t.Cleanup(func() { registry = saved })
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./gateway/platforms/... -run 'TestRegister|TestGet|TestAll' -v`

Expected: FAIL — `undefined: Descriptor`, `undefined: Register`, `undefined: Get`, `undefined: All`, `undefined: registry`.

- [ ] **Step 3: Implement `registry.go`**

Create `gateway/platforms/registry.go`:

```go
// Package platforms hosts the gateway adapter registry and the
// descriptors that advertise each adapter's configurable fields.
//
// Each adapter ships a descriptor_<type>.go file whose init() calls
// Register. cli/gateway.go::buildPlatform looks up descriptors here
// instead of carrying a hand-rolled switch.
package platforms

import (
	"context"
	"sort"

	"github.com/odysseythink/hermind/gateway"
)

// FieldKind enumerates the value shapes a descriptor field can carry.
type FieldKind int

const (
	FieldString FieldKind = iota
	FieldInt
	FieldBool
	FieldSecret
	FieldEnum
)

// String returns a lowercase name suitable for JSON ("string",
// "secret", etc.). Stage 2 uses this for the schema endpoint.
func (k FieldKind) String() string {
	switch k {
	case FieldString:
		return "string"
	case FieldInt:
		return "int"
	case FieldBool:
		return "bool"
	case FieldSecret:
		return "secret"
	case FieldEnum:
		return "enum"
	}
	return "unknown"
}

// FieldSpec describes one configurable field of an adapter.
type FieldSpec struct {
	Name     string    // key under PlatformConfig.Options
	Label    string    // human-readable label
	Help     string    // optional one-line hint
	Kind     FieldKind
	Required bool
	Default  any       // nil when none
	Enum     []string  // only for FieldEnum
}

// Descriptor is the self-describing metadata for a gateway adapter.
//
// Build constructs a running adapter from its Options map. Test does a
// lightweight handshake for the /api/platforms/test endpoint; it may
// be nil until stage 2 populates it.
type Descriptor struct {
	Type        string
	DisplayName string
	Summary     string
	Fields      []FieldSpec
	Build       func(opts map[string]string) (gateway.Platform, error)
	Test        func(ctx context.Context, opts map[string]string) error
}

var registry = map[string]Descriptor{}

// Register installs d under d.Type, overwriting any prior entry with
// the same Type. Callers invoke this from init() in descriptor files.
func Register(d Descriptor) {
	registry[d.Type] = d
}

// Get returns the descriptor registered for typ. The second return
// value is false when typ is unknown.
func Get(typ string) (Descriptor, bool) {
	d, ok := registry[typ]
	return d, ok
}

// All returns every registered descriptor sorted by Type. Stage 2's
// /api/platforms/schema endpoint uses this to produce a deterministic
// JSON response.
func All() []Descriptor {
	out := make([]Descriptor, 0, len(registry))
	for _, d := range registry {
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Type < out[j].Type })
	return out
}
```

- [ ] **Step 4: Run the tests and make sure they pass**

Run: `go test ./gateway/platforms/... -run 'TestRegister|TestGet|TestAll' -v`

Expected: PASS for `TestRegister_GetReturnsRegistered`, `TestGet_MissingReturnsFalse`, `TestAll_ReturnsSortedByType`, `TestRegister_DuplicateTypeOverwrites`.

- [ ] **Step 5: Commit**

```bash
git add gateway/platforms/registry.go gateway/platforms/registry_test.go
git commit -m "$(cat <<'EOF'
feat(gateway/platforms): introduce descriptor registry

Adds the Descriptor/FieldSpec types plus Register/Get/All helpers. No
adapters register yet — later tasks in the platform-registry plan move
each type into a descriptor_<type>.go file.
EOF
)"
```

---

## Task 2: Descriptor sanity invariants

Guards three invariants every descriptor must satisfy. Fails up front if anyone adds a broken descriptor in tasks 3–5.

**Files:**
- Modify: `gateway/platforms/registry_test.go`

- [ ] **Step 1: Write the failing sanity test**

Append to `gateway/platforms/registry_test.go`:

```go
// TestDescriptorInvariants enforces properties every production
// descriptor must satisfy. The test reads from the real registry, so
// it will be meaningful once tasks 3–5 populate it; today it runs
// over an empty registry, which trivially passes.
func TestDescriptorInvariants(t *testing.T) {
	for _, d := range All() {
		d := d
		t.Run(d.Type, func(t *testing.T) {
			if d.Type == "" {
				t.Fatal("Type is empty")
			}
			if d.DisplayName == "" {
				t.Errorf("DisplayName is empty")
			}
			if d.Build == nil {
				t.Errorf("Build is nil")
			}

			seen := map[string]bool{}
			for _, f := range d.Fields {
				if f.Name == "" {
					t.Errorf("field has empty Name")
				}
				if seen[f.Name] {
					t.Errorf("field %q: duplicate Name", f.Name)
				}
				seen[f.Name] = true
				if f.Required && f.Default != nil {
					t.Errorf("field %q: Required && Default != nil", f.Name)
				}
				if f.Kind == FieldEnum && len(f.Enum) == 0 {
					t.Errorf("field %q: FieldEnum with no Enum values", f.Name)
				}
			}
		})
	}
}
```

- [ ] **Step 2: Run the test and confirm it passes trivially**

Run: `go test ./gateway/platforms/... -run TestDescriptorInvariants -v`

Expected: PASS (no subtests run — registry is empty). We want the test in place now so the invariants apply automatically as tasks 3–5 register descriptors.

- [ ] **Step 3: Write the parity-guard test (starts red, stays red until task 5)**

Create `gateway/platforms/descriptor_parity_test.go`:

```go
package platforms

import (
	"testing"
)

// parityCases is the canonical list of (type, minimal valid options)
// pairs copied from the old cli/gateway.go::buildPlatform switch. Every
// case must resolve to a registered descriptor that builds without
// error. If this test drifts from buildPlatform, either the registry is
// missing a type (tasks 3–5 cover all 19) or buildPlatform grew a new
// one that needs a descriptor.
var parityCases = []struct {
	Type    string
	Options map[string]string
}{
	{"api_server", map[string]string{"addr": ":9000"}},
	{"webhook", map[string]string{"url": "https://example.com/hook", "token": "t"}},
	{"telegram", map[string]string{"token": "123:abc"}},
	{"acp", map[string]string{"addr": ":9001", "token": "t"}},
	{"slack", map[string]string{"webhook_url": "https://hooks.slack.com/xxx"}},
	{"discord", map[string]string{"webhook_url": "https://discord.com/api/webhooks/xxx"}},
	{"mattermost", map[string]string{"webhook_url": "https://mm.example.com/hooks/xxx"}},
	{"feishu", map[string]string{"webhook_url": "https://open.feishu.cn/open-apis/bot/xxx"}},
	{"dingtalk", map[string]string{"webhook_url": "https://oapi.dingtalk.com/robot/send?access_token=xxx"}},
	{"wecom", map[string]string{"webhook_url": "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=xxx"}},
	{"email", map[string]string{
		"host": "smtp.example.com", "port": "587",
		"username": "u", "password": "p",
		"from": "from@example.com", "to": "to@example.com",
	}},
	{"sms", map[string]string{
		"account_sid": "ACxxx", "auth_token": "token",
		"from": "+10000000000", "to": "+10000000001",
	}},
	{"signal", map[string]string{"base_url": "http://localhost:8080", "account": "+10000000000"}},
	{"whatsapp", map[string]string{"phone_id": "123", "access_token": "t"}},
	{"matrix", map[string]string{
		"home_server": "https://matrix.org", "access_token": "t", "room_id": "!room:matrix.org",
	}},
	{"homeassistant", map[string]string{
		"base_url": "http://homeassistant.local:8123", "access_token": "t", "service": "notify",
	}},
	{"slack_events", map[string]string{"addr": ":8082", "bot_token": "xoxb-xxx"}},
	{"discord_bot", map[string]string{"token": "xxx", "channel_id": "1"}},
	{"mattermost_bot", map[string]string{"base_url": "https://mm.example.com", "token": "t", "channel_id": "c"}},
}

func TestDescriptorParity_AllTypesRegistered(t *testing.T) {
	for _, tc := range parityCases {
		tc := tc
		t.Run(tc.Type, func(t *testing.T) {
			d, ok := Get(tc.Type)
			if !ok {
				t.Fatalf("no descriptor registered for %q", tc.Type)
			}
			if d.Build == nil {
				t.Fatalf("%q: Build is nil", tc.Type)
			}
			plat, err := d.Build(tc.Options)
			if err != nil {
				t.Fatalf("%q: Build returned error: %v", tc.Type, err)
			}
			if plat == nil {
				t.Fatalf("%q: Build returned nil platform", tc.Type)
			}
			if plat.Name() == "" {
				t.Errorf("%q: platform has empty Name()", tc.Type)
			}
		})
	}
}

func TestDescriptorParity_CoverageMatchesCaseCount(t *testing.T) {
	// Any drift from 19 is intentional and should update parityCases
	// plus the descriptor files together.
	if want, got := 19, len(parityCases); got != want {
		t.Fatalf("parityCases has %d entries, want %d — update parityCases and descriptors in lockstep", got, want)
	}
}
```

- [ ] **Step 4: Run it and confirm it fails**

Run: `go test ./gateway/platforms/... -run TestDescriptorParity -v`

Expected: FAIL for every subtest with `no descriptor registered for "<type>"`, because no descriptor files exist yet. The "CoverageMatchesCaseCount" subtest passes. Keep this running red — tasks 3–5 drive it to green type by type.

- [ ] **Step 5: Commit**

```bash
git add gateway/platforms/registry_test.go gateway/platforms/descriptor_parity_test.go
git commit -m "$(cat <<'EOF'
test(gateway/platforms): add sanity + parity guards for descriptors

TestDescriptorInvariants enforces per-descriptor shape; parity_test
locks in the 19 (type, options) pairs currently accepted by
cli/gateway.go::buildPlatform. Parity stays red until every type gets a
descriptor in subsequent plan tasks.
EOF
)"
```

---

## Task 3: Single-field descriptors (7 files)

Covers `telegram` (1-field secret) plus the six outbound-only webhook posters (`slack`, `discord`, `mattermost`, `feishu`, `dingtalk`, `wecom`), each taking a single `webhook_url` secret. Seven `descriptor_<type>.go` files, all shaped the same.

**Files:**
- Create: `gateway/platforms/descriptor_telegram.go`
- Create: `gateway/platforms/descriptor_slack.go`
- Create: `gateway/platforms/descriptor_discord.go`
- Create: `gateway/platforms/descriptor_mattermost.go`
- Create: `gateway/platforms/descriptor_feishu.go`
- Create: `gateway/platforms/descriptor_dingtalk.go`
- Create: `gateway/platforms/descriptor_wecom.go`

- [ ] **Step 1: Create `descriptor_telegram.go`**

```go
package platforms

import "github.com/odysseythink/hermind/gateway"

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
	})
}
```

- [ ] **Step 2: Create `descriptor_slack.go`**

```go
package platforms

import "github.com/odysseythink/hermind/gateway"

func init() {
	Register(Descriptor{
		Type:        "slack",
		DisplayName: "Slack (Incoming Webhook)",
		Summary:     "Outbound-only Slack messages via an incoming webhook URL.",
		Fields: []FieldSpec{
			{Name: "webhook_url", Label: "Webhook URL", Kind: FieldSecret, Required: true,
				Help: "From Slack app → Incoming Webhooks."},
		},
		Build: func(opts map[string]string) (gateway.Platform, error) {
			return NewSlack(opts["webhook_url"]), nil
		},
	})
}
```

- [ ] **Step 3: Create `descriptor_discord.go`**

```go
package platforms

import "github.com/odysseythink/hermind/gateway"

func init() {
	Register(Descriptor{
		Type:        "discord",
		DisplayName: "Discord (Incoming Webhook)",
		Summary:     "Outbound-only Discord messages via a channel webhook URL.",
		Fields: []FieldSpec{
			{Name: "webhook_url", Label: "Webhook URL", Kind: FieldSecret, Required: true,
				Help: "From Discord channel settings → Integrations → Webhooks."},
		},
		Build: func(opts map[string]string) (gateway.Platform, error) {
			return NewDiscord(opts["webhook_url"]), nil
		},
	})
}
```

- [ ] **Step 4: Create `descriptor_mattermost.go`**

```go
package platforms

import "github.com/odysseythink/hermind/gateway"

func init() {
	Register(Descriptor{
		Type:        "mattermost",
		DisplayName: "Mattermost (Incoming Webhook)",
		Summary:     "Outbound-only Mattermost messages via an incoming webhook URL.",
		Fields: []FieldSpec{
			{Name: "webhook_url", Label: "Webhook URL", Kind: FieldSecret, Required: true},
		},
		Build: func(opts map[string]string) (gateway.Platform, error) {
			return NewMattermost(opts["webhook_url"]), nil
		},
	})
}
```

- [ ] **Step 5: Create `descriptor_feishu.go`**

```go
package platforms

import "github.com/odysseythink/hermind/gateway"

func init() {
	Register(Descriptor{
		Type:        "feishu",
		DisplayName: "Feishu / Lark (Bot Webhook)",
		Summary:     "Outbound-only Feishu/Lark bot via a custom bot webhook.",
		Fields: []FieldSpec{
			{Name: "webhook_url", Label: "Webhook URL", Kind: FieldSecret, Required: true},
		},
		Build: func(opts map[string]string) (gateway.Platform, error) {
			return NewFeishu(opts["webhook_url"]), nil
		},
	})
}
```

- [ ] **Step 6: Create `descriptor_dingtalk.go`**

```go
package platforms

import "github.com/odysseythink/hermind/gateway"

func init() {
	Register(Descriptor{
		Type:        "dingtalk",
		DisplayName: "DingTalk (Robot Webhook)",
		Summary:     "Outbound-only DingTalk robot via a custom robot webhook URL.",
		Fields: []FieldSpec{
			{Name: "webhook_url", Label: "Webhook URL", Kind: FieldSecret, Required: true},
		},
		Build: func(opts map[string]string) (gateway.Platform, error) {
			return NewDingTalk(opts["webhook_url"]), nil
		},
	})
}
```

- [ ] **Step 7: Create `descriptor_wecom.go`**

```go
package platforms

import "github.com/odysseythink/hermind/gateway"

func init() {
	Register(Descriptor{
		Type:        "wecom",
		DisplayName: "WeCom (Enterprise WeChat Bot)",
		Summary:     "Outbound-only WeCom bot via a group bot webhook URL.",
		Fields: []FieldSpec{
			{Name: "webhook_url", Label: "Webhook URL", Kind: FieldSecret, Required: true},
		},
		Build: func(opts map[string]string) (gateway.Platform, error) {
			return NewWeCom(opts["webhook_url"]), nil
		},
	})
}
```

- [ ] **Step 8: Run the parity test and confirm 7 subtests now pass**

Run: `go test ./gateway/platforms/... -run TestDescriptorParity -v`

Expected: `TestDescriptorParity_AllTypesRegistered/{telegram,slack,discord,mattermost,feishu,dingtalk,wecom}` all PASS. The other 12 still FAIL. `TestDescriptorParity_CoverageMatchesCaseCount` PASS. `TestDescriptorInvariants` PASS for all 7 registered types.

- [ ] **Step 9: Commit**

```bash
git add gateway/platforms/descriptor_telegram.go \
        gateway/platforms/descriptor_slack.go \
        gateway/platforms/descriptor_discord.go \
        gateway/platforms/descriptor_mattermost.go \
        gateway/platforms/descriptor_feishu.go \
        gateway/platforms/descriptor_dingtalk.go \
        gateway/platforms/descriptor_wecom.go
git commit -m "feat(gateway/platforms): register single-field descriptors (7 types)"
```

---

## Task 4: Two-field and addr-only descriptors (7 files)

Covers `webhook`, `acp`, `signal`, `whatsapp`, `discord_bot`, `slack_events`, `api_server`.

**Files:**
- Create: `gateway/platforms/descriptor_webhook.go`
- Create: `gateway/platforms/descriptor_acp.go`
- Create: `gateway/platforms/descriptor_signal.go`
- Create: `gateway/platforms/descriptor_whatsapp.go`
- Create: `gateway/platforms/descriptor_discord_bot.go`
- Create: `gateway/platforms/descriptor_slack_events.go`
- Create: `gateway/platforms/descriptor_api_server.go`

- [ ] **Step 1: Create `descriptor_webhook.go`**

```go
package platforms

import "github.com/odysseythink/hermind/gateway"

func init() {
	Register(Descriptor{
		Type:        "webhook",
		DisplayName: "Generic Webhook",
		Summary:     "POSTs outgoing messages to an arbitrary URL; optional bearer token.",
		Fields: []FieldSpec{
			{Name: "url", Label: "URL", Kind: FieldString, Required: true,
				Help: "HTTPS endpoint to POST each outgoing message to."},
			{Name: "token", Label: "Bearer Token", Kind: FieldSecret,
				Help: "Optional; sent as Authorization: Bearer <token>."},
		},
		Build: func(opts map[string]string) (gateway.Platform, error) {
			return NewWebhook(opts["url"], opts["token"]), nil
		},
	})
}
```

- [ ] **Step 2: Create `descriptor_acp.go`**

```go
package platforms

import "github.com/odysseythink/hermind/gateway"

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
	})
}
```

- [ ] **Step 3: Create `descriptor_signal.go`**

```go
package platforms

import "github.com/odysseythink/hermind/gateway"

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
	})
}
```

- [ ] **Step 4: Create `descriptor_whatsapp.go`**

```go
package platforms

import "github.com/odysseythink/hermind/gateway"

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
	})
}
```

- [ ] **Step 5: Create `descriptor_discord_bot.go`**

```go
package platforms

import "github.com/odysseythink/hermind/gateway"

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
	})
}
```

- [ ] **Step 6: Create `descriptor_slack_events.go`**

The listener address is required (it must match the URL Slack posts events to), so `Required: true` and no default. The sanity test rejects any descriptor with both `Required: true` and `Default != nil`.

```go
package platforms

import "github.com/odysseythink/hermind/gateway"

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
	})
}
```

- [ ] **Step 7: Create `descriptor_api_server.go`**

`api_server` has one non-secret field with a sensible default.

```go
package platforms

import "github.com/odysseythink/hermind/gateway"

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
	})
}
```

Rationale for the `Build`-time default: the UI will present `:8080` as the placeholder, but a user can wipe the field. `Build` also falls back, so the zero-value behavior matches what `buildPlatform` used to produce when `Options["addr"]` was empty (the constructor itself accepted an empty string).

- [ ] **Step 8: Run the parity test and confirm 14 subtests pass**

Run: `go test ./gateway/platforms/... -run TestDescriptorParity -v`

Expected: 14 `TestDescriptorParity_AllTypesRegistered/<type>` PASS (tasks 3 + 4). 5 still FAIL (matrix, homeassistant, mattermost_bot, sms, email). Sanity test passes for the 14 new descriptors.

- [ ] **Step 9: Commit**

```bash
git add gateway/platforms/descriptor_webhook.go \
        gateway/platforms/descriptor_acp.go \
        gateway/platforms/descriptor_signal.go \
        gateway/platforms/descriptor_whatsapp.go \
        gateway/platforms/descriptor_discord_bot.go \
        gateway/platforms/descriptor_slack_events.go \
        gateway/platforms/descriptor_api_server.go
git commit -m "feat(gateway/platforms): register two-field and addr-only descriptors (7 types)"
```

---

## Task 5: Multi-field descriptors (5 files)

Covers `matrix`, `homeassistant`, `mattermost_bot`, `sms`, `email`.

**Files:**
- Create: `gateway/platforms/descriptor_matrix.go`
- Create: `gateway/platforms/descriptor_homeassistant.go`
- Create: `gateway/platforms/descriptor_mattermost_bot.go`
- Create: `gateway/platforms/descriptor_sms.go`
- Create: `gateway/platforms/descriptor_email.go`

- [ ] **Step 1: Create `descriptor_matrix.go`**

```go
package platforms

import "github.com/odysseythink/hermind/gateway"

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
	})
}
```

- [ ] **Step 2: Create `descriptor_homeassistant.go`**

```go
package platforms

import "github.com/odysseythink/hermind/gateway"

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
	})
}
```

- [ ] **Step 3: Create `descriptor_mattermost_bot.go`**

```go
package platforms

import "github.com/odysseythink/hermind/gateway"

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
	})
}
```

- [ ] **Step 4: Create `descriptor_sms.go`**

```go
package platforms

import "github.com/odysseythink/hermind/gateway"

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
	})
}
```

- [ ] **Step 5: Create `descriptor_email.go`**

```go
package platforms

import "github.com/odysseythink/hermind/gateway"

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
	})
}
```

- [ ] **Step 6: Run the parity test and confirm all 19 subtests pass**

Run: `go test ./gateway/platforms/... -run TestDescriptorParity -v`

Expected: all 19 `TestDescriptorParity_AllTypesRegistered/<type>` PASS. `TestDescriptorParity_CoverageMatchesCaseCount` PASS. `TestDescriptorInvariants` PASS for all 19.

- [ ] **Step 7: Run the full platforms package test suite**

Run: `go test ./gateway/platforms/... -v`

Expected: PASS. Existing adapter tests are unaffected (we added files, did not modify adapters).

- [ ] **Step 8: Commit**

```bash
git add gateway/platforms/descriptor_matrix.go \
        gateway/platforms/descriptor_homeassistant.go \
        gateway/platforms/descriptor_mattermost_bot.go \
        gateway/platforms/descriptor_sms.go \
        gateway/platforms/descriptor_email.go
git commit -m "feat(gateway/platforms): register multi-field descriptors (5 types)"
```

---

## Task 6: Rewrite `cli/gateway.go::buildPlatform` to use the registry

The switch at `cli/gateway.go:140-200` collapses to a registry lookup. Unknown types now return an error (previously the same behavior via the switch default).

**Files:**
- Modify: `cli/gateway.go`
- Create: `cli/gateway_test.go`

- [ ] **Step 1: Write the failing test**

Create `cli/gateway_test.go`:

```go
package cli

import (
	"strings"
	"testing"

	"github.com/odysseythink/hermind/config"
)

func TestBuildPlatform_KnownTypeReturnsPlatform(t *testing.T) {
	pc := config.PlatformConfig{
		Enabled: true,
		Type:    "telegram",
		Options: map[string]string{"token": "123:abc"},
	}
	plat, err := buildPlatform("tg_main", pc)
	if err != nil {
		t.Fatalf("buildPlatform returned error: %v", err)
	}
	if plat == nil {
		t.Fatal("buildPlatform returned nil platform")
	}
}

func TestBuildPlatform_EmptyTypeFallsBackToName(t *testing.T) {
	pc := config.PlatformConfig{
		Enabled: true,
		Type:    "",
		Options: map[string]string{"token": "123:abc"},
	}
	plat, err := buildPlatform("telegram", pc)
	if err != nil {
		t.Fatalf("buildPlatform returned error: %v", err)
	}
	if plat == nil {
		t.Fatal("nil platform for name-fallback case")
	}
}

func TestBuildPlatform_UnknownTypeReturnsError(t *testing.T) {
	pc := config.PlatformConfig{
		Enabled: true,
		Type:    "does-not-exist",
		Options: map[string]string{},
	}
	_, err := buildPlatform("any_name", pc)
	if err == nil {
		t.Fatal("buildPlatform(unknown) returned nil error")
	}
	if !strings.Contains(err.Error(), "unknown platform type") {
		t.Errorf("err = %q, want substring 'unknown platform type'", err)
	}
}
```

- [ ] **Step 2: Run the test — expect it to pass on the current code**

Run: `go test ./cli/... -run 'TestBuildPlatform' -v`

Expected: PASS (current switch already handles all three cases). The test documents intended behavior before we rewrite; if it fails, stop and investigate before proceeding.

- [ ] **Step 3: Rewrite `buildPlatform` in `cli/gateway.go`**

Replace lines 140–200 of `cli/gateway.go` (the entire `buildPlatform` function) with:

```go
// buildPlatform instantiates a platform adapter from its config entry.
// The type is taken from pc.Type, or falls back to the map key when pc.Type
// is empty. Lookup is delegated to the gateway/platforms registry; each
// adapter self-registers via a descriptor_<type>.go init().
func buildPlatform(name string, pc config.PlatformConfig) (gateway.Platform, error) {
	t := strings.ToLower(pc.Type)
	if t == "" {
		t = strings.ToLower(name)
	}
	d, ok := platforms.Get(t)
	if !ok {
		return nil, fmt.Errorf("unknown platform type %q", t)
	}
	return d.Build(pc.Options)
}
```

Verify that `cli/gateway.go` still imports `strings`, `fmt`, `config`, `gateway`, and `platforms`. Remove any imports that become unused (none expected — all five remain in use by the surrounding file).

- [ ] **Step 4: Run the test to verify it still passes**

Run: `go test ./cli/... -run 'TestBuildPlatform' -v`

Expected: PASS — same three tests, now backed by registry.

- [ ] **Step 5: Run the full CLI package tests**

Run: `go test ./cli/...`

Expected: PASS (or the same pre-existing state as the starting branch — the working tree already has unrelated CLI edits; compare to baseline with `git stash && go test ./cli/... && git stash pop` if unsure, but this plan's changes should not alter existing CLI tests).

- [ ] **Step 6: Commit**

```bash
git add cli/gateway.go cli/gateway_test.go
git commit -m "$(cat <<'EOF'
refactor(cli/gateway): route buildPlatform through the registry

Collapses the 19-case switch into a single platforms.Get + Build call.
Behavior is unchanged: empty pc.Type still falls back to the map key;
unknown types still return an "unknown platform type" error.

The registry is now the single source of truth for which gateway types
exist and what fields each needs, unblocking stages 2–5 of the web IM
config plan.
EOF
)"
```

---

## Task 7: Final verification

**Files:** none modified — verification only.

- [ ] **Step 1: `go vet` is clean**

Run: `go vet ./gateway/platforms/... ./cli/...`

Expected: no output, exit 0.

- [ ] **Step 2: Full test suite passes**

Run: `go test ./...`

Expected: PASS across the tree. If anything unrelated to this plan is already broken on the starting branch (the working tree has many unrelated modifications), the baseline must match before the registry work started. Reconcile with:

```bash
git stash
go test ./... 2>&1 | tail -20   # capture baseline
git stash pop
go test ./... 2>&1 | tail -20   # must match or be a strict superset of passes
```

- [ ] **Step 3: Build succeeds**

Run: `go build ./...`

Expected: exit 0.

- [ ] **Step 4: Sanity-check the descriptor file count**

Run: `ls gateway/platforms/descriptor_*.go | wc -l`

Expected: `19` (exactly). If another number, reconcile against the 19-type list in `parityCases`.

- [ ] **Step 5: Confirm the switch is gone**

Run: `grep -n 'case "telegram"' cli/gateway.go || echo "no switch — good"`

Expected: `no switch — good`.

- [ ] **Step 6: Final commit (empty — optional marker only if previous commits were clean)**

Nothing to commit in this task; if you reached here cleanly the previous six tasks each produced one commit. Confirm with:

Run: `git log --oneline -7`

Expected: seven lines, matching the commit messages from tasks 1–6 (plus any pre-existing commits above them).

---

## Self-review notes (for the executing agent)

- Every descriptor file is under 30 lines of Go and imports only `github.com/odysseythink/hermind/gateway`. No cross-descriptor coupling.
- Parity test in task 2 locks the 19-type contract; tasks 3–5 drive it from red to green type by type, so regressions are visible at each commit.
- The `TestDescriptorInvariants` sanity test runs after each new descriptor file lands — catch unexpected schema mistakes before they're committed.
- `Build` closures call the same constructor the old switch called, with arguments extracted from the same `Options` keys. No behavior drift.
- `Test` closures are intentionally `nil`. Stage 2's plan (not yet written) adds them alongside the `/api/platforms/test` handler and its tests.
- Nothing in this plan touches REST, frontend, config, or storage. `hermind` is byte-equivalent in behavior after stage 1.
