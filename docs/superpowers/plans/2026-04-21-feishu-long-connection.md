# Feishu Long-Connection Adapter Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the outbound-only Feishu bot-webhook adapter with a bidirectional adapter driven by a self-built Feishu Open Platform app over long-connection (WebSocket).

**Architecture:** `FeishuApp` struct with two dependency seams (`feishuEventStream`, `feishuMessageSender`) for testability. Production seams wrap `github.com/larksuite/oapi-sdk-go/v3` — `larkws.Client` for inbound, `lark.Client.Im.Message.Create` for outbound. Handler registration stays compatible with the existing `gateway.Platform` interface (`Run(ctx, handler)` blocks; `SendReply(ctx, out)` posts one message).

**Tech Stack:** Go 1.21+, `github.com/larksuite/oapi-sdk-go/v3` (new dep), existing `gateway/platforms/` descriptor framework.

**Spec:** `docs/superpowers/specs/2026-04-21-feishu-long-connection-design.md`

---

## File map

| File | Action | Purpose |
|---|---|---|
| `go.mod`, `go.sum` | Modify | Add larksuite SDK dep |
| `gateway/platforms/feishu_app.go` | Create | `FeishuApp` struct, interfaces, event decode, SendReply, production SDK wrappers |
| `gateway/platforms/feishu_app_test.go` | Create | Unit tests with fake stream/sender |
| `gateway/platforms/descriptor_feishu.go` | Rewrite | Replace single `webhook_url` field with 5-field self-built-app descriptor |
| `gateway/platforms/descriptor_feishu_test.go` | Create | Field-level descriptor test |
| `gateway/platforms/chatbots.go` | Modify | Remove `NewFeishu` factory (lines 29–38) |
| `gateway/platforms/chatbots_test.go` | Modify | Remove the feishu subtest case (lines 56–68) |
| `gateway/platforms/descriptor_parity_test.go` | Modify | Update `feishu` parityCases entry to new option shape |
| `docs/smoke/feishu-app.md` | Create | Manual verification flow |
| `CHANGELOG.md` | Modify | BREAKING entry |

---

### Task 1: Add SDK dependency

**Files:**
- Modify: `go.mod`
- Modify: `go.sum`

- [ ] **Step 1: Add the module**

```bash
go get github.com/larksuite/oapi-sdk-go/v3@latest
```

- [ ] **Step 2: Tidy**

```bash
go mod tidy
```

- [ ] **Step 3: Verify the project still compiles**

```bash
go build ./...
```

Expected: exits 0, no new test failures because no code uses the SDK yet.

- [ ] **Step 4: Sanity-check the go.mod entry**

```bash
grep larksuite go.mod
```

Expected: one line like `github.com/larksuite/oapi-sdk-go/v3 vX.Y.Z`.

- [ ] **Step 5: Commit**

```bash
git add go.mod go.sum
git commit -m "chore(deps): add larksuite/oapi-sdk-go v3 for feishu long-connection"
```

---

### Task 2: FeishuApp skeleton + MissingCreds guard

Establish the struct, the two dependency seams, the exported and internal constructors, and the first failing test. No SDK code yet — that comes in Task 12.

**Files:**
- Create: `gateway/platforms/feishu_app.go`
- Create: `gateway/platforms/feishu_app_test.go`

- [ ] **Step 1: Write the failing test**

Put this in a new `gateway/platforms/feishu_app_test.go`:

```go
package platforms

import (
	"strings"
	"testing"
)

func TestFeishuApp_MissingCreds(t *testing.T) {
	cases := []struct {
		name string
		opts map[string]string
	}{
		{"no app_id", map[string]string{"app_secret": "s"}},
		{"no app_secret", map[string]string{"app_id": "a"}},
		{"both empty", map[string]string{}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			fa, err := NewFeishuApp(tc.opts)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if fa != nil {
				t.Errorf("expected nil FeishuApp, got %v", fa)
			}
			if !strings.Contains(err.Error(), "app_id") && !strings.Contains(err.Error(), "app_secret") {
				t.Errorf("error should mention app_id or app_secret: %v", err)
			}
		})
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

```bash
go test ./gateway/platforms -run TestFeishuApp_MissingCreds -v
```

Expected: compile error `undefined: NewFeishuApp`.

- [ ] **Step 3: Create the skeleton file**

Put this in a new `gateway/platforms/feishu_app.go`:

```go
package platforms

import (
	"context"
	"fmt"
	"sync"

	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"

	"github.com/odysseythink/hermind/gateway"
)

// feishuEventStream is the inbound seam. Production wraps *larkws.Client.
// Start blocks until ctx is cancelled and delivers events via the handler
// registered at construction of the concrete implementation.
type feishuEventStream interface {
	Start(ctx context.Context) error
}

// feishuMessageSender is the outbound seam. Production wraps
// *lark.Client.Im.Message.Create.
type feishuMessageSender interface {
	Create(ctx context.Context, chatID, text string) error
}

// FeishuApp is the bidirectional Feishu / Lark adapter backed by a
// self-built app long-connection.
type FeishuApp struct {
	appID         string
	appSecret     string
	domain        string // "feishu" or "lark"
	encryptKey    string
	defaultChatID string

	stream feishuEventStream
	sender feishuMessageSender

	mu      sync.Mutex
	handler gateway.MessageHandler
}

// NewFeishuApp constructs the adapter using production SDK-backed stream
// and sender. Returns an error when required options are missing or when
// legacy webhook_url is present without app_id (migration guard).
func NewFeishuApp(opts map[string]string) (*FeishuApp, error) {
	appID := opts["app_id"]
	appSecret := opts["app_secret"]
	if appID == "" || appSecret == "" {
		return nil, fmt.Errorf("feishu: missing app_id or app_secret")
	}
	fa := &FeishuApp{
		appID:         appID,
		appSecret:     appSecret,
		domain:        opts["domain"],
		encryptKey:    opts["encrypt_key"],
		defaultChatID: opts["default_chat_id"],
	}
	return fa, nil
}

// newFeishuAppForTest builds an adapter with injected seams. Unexported
// so tests in this package are the only callers.
func newFeishuAppForTest(stream feishuEventStream, sender feishuMessageSender, defaultChatID string) *FeishuApp {
	return &FeishuApp{
		stream:        stream,
		sender:        sender,
		defaultChatID: defaultChatID,
	}
}

// Name returns the canonical platform name.
func (fa *FeishuApp) Name() string { return "feishu" }

// Run starts the long-connection event loop. It blocks until ctx is
// cancelled or the stream errors out.
func (fa *FeishuApp) Run(ctx context.Context, h gateway.MessageHandler) error {
	return fmt.Errorf("feishu: Run not implemented yet")
}

// SendReply posts a text message to the target chat. When out.ChatID is
// empty, falls back to the adapter's default_chat_id.
func (fa *FeishuApp) SendReply(ctx context.Context, out gateway.OutgoingMessage) error {
	return fmt.Errorf("feishu: SendReply not implemented yet")
}

// handleEvent is invoked by the event stream for each inbound event.
func (fa *FeishuApp) handleEvent(ctx context.Context, evt *larkim.P2MessageReceiveV1) error {
	return nil
}

// stringPtrValue dereferences a *string safely.
func stringPtrValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
```

- [ ] **Step 4: Run the test to verify it passes**

```bash
go test ./gateway/platforms -run TestFeishuApp_MissingCreds -v
```

Expected: PASS (all three subcases).

- [ ] **Step 5: Run the full package to verify no regressions**

```bash
go test ./gateway/platforms -v
```

Expected: existing tests pass; note `TestDescriptorParity_AllTypesRegistered/feishu` may now fail because the old `NewFeishu(url)` is still registered but nothing in Task 2 has changed that yet. **Do not commit if any pre-existing test newly fails.** If only the `TestFeishuApp_*` tests affect results, proceed.

- [ ] **Step 6: Commit**

```bash
git add gateway/platforms/feishu_app.go gateway/platforms/feishu_app_test.go
git commit -m "feat(gateway/platforms): FeishuApp skeleton + MissingCreds guard"
```

---

### Task 3: Migration guard for legacy webhook_url

Surface the hard-break migration as a startup error so operators see it immediately when upgrading.

**Files:**
- Modify: `gateway/platforms/feishu_app.go` (NewFeishuApp)
- Modify: `gateway/platforms/feishu_app_test.go`

- [ ] **Step 1: Write the failing test**

Append to `gateway/platforms/feishu_app_test.go`:

```go
func TestFeishuApp_WebhookURLSurfaced(t *testing.T) {
	fa, err := NewFeishuApp(map[string]string{
		"webhook_url": "https://open.feishu.cn/open-apis/bot/v2/hook/xxxx",
	})
	if err == nil {
		t.Fatal("expected migration error, got nil")
	}
	if fa != nil {
		t.Errorf("expected nil FeishuApp on migration error, got %v", fa)
	}
	if !strings.Contains(err.Error(), "webhook_url is no longer supported") {
		t.Errorf("error should mention migration: %v", err)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

```bash
go test ./gateway/platforms -run TestFeishuApp_WebhookURLSurfaced -v
```

Expected: FAIL because the current `NewFeishuApp` returns the generic "missing app_id or app_secret" error instead of the migration-specific message.

- [ ] **Step 3: Update NewFeishuApp to check webhook_url first**

Replace the opening of `NewFeishuApp` in `gateway/platforms/feishu_app.go`:

```go
func NewFeishuApp(opts map[string]string) (*FeishuApp, error) {
	appID := opts["app_id"]
	appSecret := opts["app_secret"]
	if appID == "" && opts["webhook_url"] != "" {
		return nil, fmt.Errorf("feishu: webhook_url is no longer supported; migrate to a self-built app (see CHANGELOG)")
	}
	if appID == "" || appSecret == "" {
		return nil, fmt.Errorf("feishu: missing app_id or app_secret")
	}
	fa := &FeishuApp{
		appID:         appID,
		appSecret:     appSecret,
		domain:        opts["domain"],
		encryptKey:    opts["encrypt_key"],
		defaultChatID: opts["default_chat_id"],
	}
	return fa, nil
}
```

- [ ] **Step 4: Run both tests**

```bash
go test ./gateway/platforms -run 'TestFeishuApp_MissingCreds|TestFeishuApp_WebhookURLSurfaced' -v
```

Expected: PASS for all subcases.

- [ ] **Step 5: Commit**

```bash
git add gateway/platforms/feishu_app.go gateway/platforms/feishu_app_test.go
git commit -m "feat(gateway/platforms): feishu startup errors on legacy webhook_url"
```

---

### Task 4: Inbound text event → IncomingMessage

Implement `handleEvent` for the happy path: `msg_type == "text"` with a simple content payload.

**Files:**
- Modify: `gateway/platforms/feishu_app.go` (handleEvent + Run)
- Modify: `gateway/platforms/feishu_app_test.go`

- [ ] **Step 1: Add fake seams at the top of feishu_app_test.go**

Add after the existing imports (keep them; add `context`, `sync`, and the SDK `larkim` import):

```go
import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"

	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"

	"github.com/odysseythink/hermind/gateway"
)

// --- test fakes ---

type fakeStream struct {
	startCh chan struct{}
	err     error
}

func (f *fakeStream) Start(ctx context.Context) error {
	if f.startCh != nil {
		close(f.startCh)
	}
	<-ctx.Done()
	return ctx.Err()
}

type fakeSender struct {
	mu    sync.Mutex
	calls []sendCall
	err   error
}

type sendCall struct {
	chatID string
	text   string
}

func (f *fakeSender) Create(ctx context.Context, chatID, text string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, sendCall{chatID: chatID, text: text})
	return f.err
}

func (f *fakeSender) recorded() []sendCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]sendCall, len(f.calls))
	copy(out, f.calls)
	return out
}

// buildTextEvent returns a minimally populated P2MessageReceiveV1 with
// msg_type=text and the given content payload.
func buildTextEvent(chatID, messageID, openID, contentJSON, msgType string) *larkim.P2MessageReceiveV1 {
	cid := chatID
	mid := messageID
	oid := openID
	mt := msgType
	content := contentJSON
	return &larkim.P2MessageReceiveV1{
		Event: &larkim.P2MessageReceiveV1Data{
			Sender: &larkim.EventSender{
				SenderId: &larkim.UserId{OpenId: &oid},
			},
			Message: &larkim.EventMessage{
				MessageId:   &mid,
				ChatId:      &cid,
				MessageType: &mt,
				Content:     &content,
			},
		},
	}
}
```

Then add the test:

```go
func TestFeishuApp_IncomingText(t *testing.T) {
	sender := &fakeSender{}
	fa := newFeishuAppForTest(nil, sender, "")

	var got gateway.IncomingMessage
	var gotCalled bool
	handler := func(ctx context.Context, in gateway.IncomingMessage) (*gateway.OutgoingMessage, error) {
		got = in
		gotCalled = true
		return nil, nil
	}
	fa.mu.Lock()
	fa.handler = handler
	fa.mu.Unlock()

	content, _ := json.Marshal(map[string]string{"text": "hello world"})
	evt := buildTextEvent("oc_abc", "om_123", "ou_xyz", string(content), "text")

	if err := fa.handleEvent(context.Background(), evt); err != nil {
		t.Fatalf("handleEvent: %v", err)
	}
	if !gotCalled {
		t.Fatal("handler not called")
	}
	if got.Platform != "feishu" {
		t.Errorf("Platform = %q, want feishu", got.Platform)
	}
	if got.ChatID != "oc_abc" {
		t.Errorf("ChatID = %q, want oc_abc", got.ChatID)
	}
	if got.UserID != "ou_xyz" {
		t.Errorf("UserID = %q, want ou_xyz", got.UserID)
	}
	if got.MessageID != "om_123" {
		t.Errorf("MessageID = %q, want om_123", got.MessageID)
	}
	if got.Text != "hello world" {
		t.Errorf("Text = %q, want hello world", got.Text)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

```bash
go test ./gateway/platforms -run TestFeishuApp_IncomingText -v
```

Expected: FAIL — handler never called because `handleEvent` returns nil without doing anything.

- [ ] **Step 3: Implement handleEvent**

Replace the stub in `gateway/platforms/feishu_app.go`:

```go
func (fa *FeishuApp) handleEvent(ctx context.Context, evt *larkim.P2MessageReceiveV1) error {
	if evt == nil || evt.Event == nil || evt.Event.Message == nil {
		return nil
	}
	msg := evt.Event.Message
	if stringPtrValue(msg.MessageType) != "text" {
		return nil
	}

	var content struct {
		Text string `json:"text"`
	}
	raw := stringPtrValue(msg.Content)
	if err := json.Unmarshal([]byte(raw), &content); err != nil {
		return nil // malformed content — drop, don't kill the stream
	}

	openID := ""
	if evt.Event.Sender != nil && evt.Event.Sender.SenderId != nil {
		openID = stringPtrValue(evt.Event.Sender.SenderId.OpenId)
	}

	in := gateway.IncomingMessage{
		Platform:  "feishu",
		UserID:    openID,
		ChatID:    stringPtrValue(msg.ChatId),
		Text:      content.Text,
		MessageID: stringPtrValue(msg.MessageId),
	}

	fa.mu.Lock()
	h := fa.handler
	fa.mu.Unlock()
	if h == nil {
		return nil
	}
	out, err := h(ctx, in)
	if err != nil {
		return nil // handler errors logged at gateway layer; don't kill stream
	}
	if out == nil {
		return nil
	}
	_ = fa.SendReply(ctx, *out) // SendReply errors also don't kill stream
	return nil
}
```

Add `"encoding/json"` to the imports at the top of `feishu_app.go`.

- [ ] **Step 4: Run the test to verify it passes**

```bash
go test ./gateway/platforms -run TestFeishuApp_IncomingText -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add gateway/platforms/feishu_app.go gateway/platforms/feishu_app_test.go
git commit -m "feat(gateway/platforms): feishu handleEvent routes text messages"
```

---

### Task 5: Strip @-mention tokens from inbound text

**Files:**
- Modify: `gateway/platforms/feishu_app.go`
- Modify: `gateway/platforms/feishu_app_test.go`

- [ ] **Step 1: Write the failing test**

Append to `gateway/platforms/feishu_app_test.go`:

```go
func TestFeishuApp_StripsAtMention(t *testing.T) {
	fa := newFeishuAppForTest(nil, &fakeSender{}, "")
	var gotText string
	fa.mu.Lock()
	fa.handler = func(ctx context.Context, in gateway.IncomingMessage) (*gateway.OutgoingMessage, error) {
		gotText = in.Text
		return nil, nil
	}
	fa.mu.Unlock()

	cases := []struct{ raw, want string }{
		{`{"text":"@_user_1 hello"}`, "hello"},
		{`{"text":"hey @_user_2  how are you"}`, "hey how are you"},
		{`{"text":"@_user_1 @_user_2 hi"}`, "hi"},
		{`{"text":"plain"}`, "plain"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.raw, func(t *testing.T) {
			gotText = ""
			evt := buildTextEvent("oc", "om", "ou", tc.raw, "text")
			if err := fa.handleEvent(context.Background(), evt); err != nil {
				t.Fatalf("handleEvent: %v", err)
			}
			if gotText != tc.want {
				t.Errorf("got %q, want %q", gotText, tc.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

```bash
go test ./gateway/platforms -run TestFeishuApp_StripsAtMention -v
```

Expected: FAIL — first case fails because `content.Text` is `"@_user_1 hello"` and handler receives it unchanged.

- [ ] **Step 3: Add a regex helper and wire it in**

In `gateway/platforms/feishu_app.go`, add at package level:

```go
var feishuMentionRE = regexp.MustCompile(`@_user_\d+\s*`)
```

Add `"regexp"` to imports.

Modify `handleEvent` to apply it. Change the `in.Text` assignment line to:

```go
	cleaned := strings.TrimSpace(feishuMentionRE.ReplaceAllString(content.Text, ""))
	// collapse double spaces left behind between two mentions
	cleaned = strings.Join(strings.Fields(cleaned), " ")

	in := gateway.IncomingMessage{
		Platform:  "feishu",
		UserID:    openID,
		ChatID:    stringPtrValue(msg.ChatId),
		Text:      cleaned,
		MessageID: stringPtrValue(msg.MessageId),
	}
```

Add `"strings"` to imports.

- [ ] **Step 4: Run the test to verify it passes**

```bash
go test ./gateway/platforms -run TestFeishuApp_StripsAtMention -v
```

Expected: PASS all four subcases.

- [ ] **Step 5: Run all feishu tests to confirm no regression on Task 4**

```bash
go test ./gateway/platforms -run TestFeishuApp -v
```

Expected: PASS — `TestFeishuApp_IncomingText` still passes because its content has no mention tokens.

- [ ] **Step 6: Commit**

```bash
git add gateway/platforms/feishu_app.go gateway/platforms/feishu_app_test.go
git commit -m "feat(gateway/platforms): feishu strips @-mention tokens from inbound text"
```

---

### Task 6: Ignore non-text events

**Files:**
- Modify: `gateway/platforms/feishu_app_test.go`

Implementation already present (Task 4 added the `msg_type != "text"` guard). This task adds the regression test so the guard doesn't silently break.

- [ ] **Step 1: Write the test**

Append to `gateway/platforms/feishu_app_test.go`:

```go
func TestFeishuApp_IgnoresNonText(t *testing.T) {
	fa := newFeishuAppForTest(nil, &fakeSender{}, "")
	called := false
	fa.mu.Lock()
	fa.handler = func(ctx context.Context, in gateway.IncomingMessage) (*gateway.OutgoingMessage, error) {
		called = true
		return nil, nil
	}
	fa.mu.Unlock()

	for _, mt := range []string{"image", "file", "sticker", "post", ""} {
		mt := mt
		t.Run("msg_type="+mt, func(t *testing.T) {
			called = false
			evt := buildTextEvent("oc", "om", "ou", `{"text":"ignored"}`, mt)
			if err := fa.handleEvent(context.Background(), evt); err != nil {
				t.Fatalf("handleEvent: %v", err)
			}
			if called {
				t.Errorf("handler called for msg_type=%q; want dropped", mt)
			}
		})
	}
}
```

- [ ] **Step 2: Run the test**

```bash
go test ./gateway/platforms -run TestFeishuApp_IgnoresNonText -v
```

Expected: PASS on all five subcases (implementation from Task 4 already filters).

- [ ] **Step 3: Commit**

```bash
git add gateway/platforms/feishu_app_test.go
git commit -m "test(gateway/platforms): feishu ignores non-text events"
```

---

### Task 7: SendReply — reply to source chat_id

**Files:**
- Modify: `gateway/platforms/feishu_app.go` (SendReply)
- Modify: `gateway/platforms/feishu_app_test.go`

- [ ] **Step 1: Write the failing test**

Append to `gateway/platforms/feishu_app_test.go`:

```go
func TestFeishuApp_SendReplyToSource(t *testing.T) {
	sender := &fakeSender{}
	fa := newFeishuAppForTest(nil, sender, "oc_default")

	err := fa.SendReply(context.Background(), gateway.OutgoingMessage{
		ChatID: "oc_a",
		Text:   "hi",
	})
	if err != nil {
		t.Fatalf("SendReply: %v", err)
	}
	calls := sender.recorded()
	if len(calls) != 1 {
		t.Fatalf("want 1 call, got %d", len(calls))
	}
	if calls[0].chatID != "oc_a" {
		t.Errorf("chatID = %q, want oc_a (should NOT fall back to default)", calls[0].chatID)
	}
	if calls[0].text != "hi" {
		t.Errorf("text = %q, want hi", calls[0].text)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

```bash
go test ./gateway/platforms -run TestFeishuApp_SendReplyToSource -v
```

Expected: FAIL — current stub returns an error.

- [ ] **Step 3: Implement SendReply**

Replace the stub `SendReply` in `gateway/platforms/feishu_app.go`:

```go
func (fa *FeishuApp) SendReply(ctx context.Context, out gateway.OutgoingMessage) error {
	target := out.ChatID
	if target == "" {
		target = fa.defaultChatID
	}
	if target == "" {
		return fmt.Errorf("feishu: no target chat_id (out.ChatID empty and default_chat_id not set)")
	}
	if fa.sender == nil {
		return fmt.Errorf("feishu: sender not initialised")
	}
	return fa.sender.Create(ctx, target, out.Text)
}
```

- [ ] **Step 4: Run the test to verify it passes**

```bash
go test ./gateway/platforms -run TestFeishuApp_SendReplyToSource -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add gateway/platforms/feishu_app.go gateway/platforms/feishu_app_test.go
git commit -m "feat(gateway/platforms): feishu SendReply routes to source chat"
```

---

### Task 8: SendReply — fallback to default_chat_id

**Files:**
- Modify: `gateway/platforms/feishu_app_test.go`

- [ ] **Step 1: Write the test**

Append:

```go
func TestFeishuApp_SendReplyFallback(t *testing.T) {
	sender := &fakeSender{}
	fa := newFeishuAppForTest(nil, sender, "oc_default")

	err := fa.SendReply(context.Background(), gateway.OutgoingMessage{
		ChatID: "", // empty → fallback
		Text:   "push",
	})
	if err != nil {
		t.Fatalf("SendReply: %v", err)
	}
	calls := sender.recorded()
	if len(calls) != 1 {
		t.Fatalf("want 1 call, got %d", len(calls))
	}
	if calls[0].chatID != "oc_default" {
		t.Errorf("chatID = %q, want oc_default", calls[0].chatID)
	}
}
```

- [ ] **Step 2: Run**

```bash
go test ./gateway/platforms -run TestFeishuApp_SendReplyFallback -v
```

Expected: PASS (implementation from Task 7 already covers fallback).

- [ ] **Step 3: Commit**

```bash
git add gateway/platforms/feishu_app_test.go
git commit -m "test(gateway/platforms): feishu SendReply falls back to default_chat_id"
```

---

### Task 9: SendReply — error when no target

**Files:**
- Modify: `gateway/platforms/feishu_app_test.go`

- [ ] **Step 1: Write the test**

Append:

```go
func TestFeishuApp_SendReplyNoTarget(t *testing.T) {
	sender := &fakeSender{}
	fa := newFeishuAppForTest(nil, sender, "") // no default

	err := fa.SendReply(context.Background(), gateway.OutgoingMessage{
		ChatID: "",
		Text:   "orphan",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "no target") {
		t.Errorf("error should mention no target: %v", err)
	}
	if len(sender.recorded()) != 0 {
		t.Errorf("sender should not have been called")
	}
}
```

- [ ] **Step 2: Run**

```bash
go test ./gateway/platforms -run TestFeishuApp_SendReplyNoTarget -v
```

Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add gateway/platforms/feishu_app_test.go
git commit -m "test(gateway/platforms): feishu SendReply errors when no target"
```

---

### Task 10: Run — plumbs handler to stream; respects ctx

Wire `Run` so the stream starts, the handler is registered, and ctx cancel unwinds.

**Files:**
- Modify: `gateway/platforms/feishu_app.go` (Run)
- Modify: `gateway/platforms/feishu_app_test.go`

- [ ] **Step 1: Write the failing test**

Add `"time"` to `feishu_app_test.go`'s import block.

Append:

```go
func TestFeishuApp_ContextCancels(t *testing.T) {
	stream := &fakeStream{startCh: make(chan struct{})}
	fa := newFeishuAppForTest(stream, &fakeSender{}, "")

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- fa.Run(ctx, func(ctx context.Context, in gateway.IncomingMessage) (*gateway.OutgoingMessage, error) {
			return nil, nil
		})
	}()

	<-stream.startCh // wait for stream.Start to be entered
	cancel()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("Run should return a non-nil error on ctx cancel (ctx.Err())")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return within 2s of ctx cancel")
	}
}
```

- [ ] **Step 2: Run to verify it fails**

```bash
go test ./gateway/platforms -run TestFeishuApp_ContextCancels -v
```

Expected: FAIL — current stub returns `"Run not implemented yet"` immediately, so the goroutine returns before cancel is called.

- [ ] **Step 3: Implement Run**

Replace the stub in `gateway/platforms/feishu_app.go`:

```go
func (fa *FeishuApp) Run(ctx context.Context, h gateway.MessageHandler) error {
	if fa.stream == nil {
		return fmt.Errorf("feishu: stream not initialised")
	}
	fa.mu.Lock()
	fa.handler = h
	fa.mu.Unlock()
	defer func() {
		fa.mu.Lock()
		fa.handler = nil
		fa.mu.Unlock()
	}()
	return fa.stream.Start(ctx)
}
```

- [ ] **Step 4: Run to verify it passes**

```bash
go test ./gateway/platforms -run TestFeishuApp_ContextCancels -v
```

Expected: PASS.

- [ ] **Step 5: Re-run every feishu test to confirm no regressions**

```bash
go test ./gateway/platforms -run TestFeishuApp -v
```

Expected: all nine `TestFeishuApp_*` tests PASS.

- [ ] **Step 6: Commit**

```bash
git add gateway/platforms/feishu_app.go gateway/platforms/feishu_app_test.go
git commit -m "feat(gateway/platforms): feishu Run registers handler, respects ctx"
```

---

### Task 11: Handler error does not kill the stream

**Files:**
- Modify: `gateway/platforms/feishu_app_test.go`

- [ ] **Step 1: Write the test**

Append:

```go
func TestFeishuApp_HandlerErrorDoesNotKillStream(t *testing.T) {
	fa := newFeishuAppForTest(nil, &fakeSender{}, "")

	calls := 0
	fa.mu.Lock()
	fa.handler = func(ctx context.Context, in gateway.IncomingMessage) (*gateway.OutgoingMessage, error) {
		calls++
		if calls == 1 {
			return nil, fmt.Errorf("handler boom")
		}
		return nil, nil
	}
	fa.mu.Unlock()

	evt1 := buildTextEvent("oc", "om1", "ou", `{"text":"one"}`, "text")
	evt2 := buildTextEvent("oc", "om2", "ou", `{"text":"two"}`, "text")

	if err := fa.handleEvent(context.Background(), evt1); err != nil {
		t.Fatalf("handleEvent#1: %v", err)
	}
	if err := fa.handleEvent(context.Background(), evt2); err != nil {
		t.Fatalf("handleEvent#2: %v", err)
	}
	if calls != 2 {
		t.Errorf("handler called %d times, want 2", calls)
	}
}
```

Add `"fmt"` to the test file's imports if not already present (the `errors.New`/`fmt.Errorf` usage requires it).

- [ ] **Step 2: Run**

```bash
go test ./gateway/platforms -run TestFeishuApp_HandlerErrorDoesNotKillStream -v
```

Expected: PASS — `handleEvent` (from Task 4) already swallows handler errors.

- [ ] **Step 3: Commit**

```bash
git add gateway/platforms/feishu_app_test.go
git commit -m "test(gateway/platforms): feishu handler error does not kill stream"
```

---

### Task 12: Production SDK wiring (stream + sender)

Add the SDK-backed stream and sender implementations and have `NewFeishuApp` construct them. No new behaviour tests — Task 13's parity/build test covers the wiring.

**Files:**
- Modify: `gateway/platforms/feishu_app.go`

- [ ] **Step 1: Add the production implementations**

At the bottom of `gateway/platforms/feishu_app.go`, add:

```go
// --- production SDK-backed implementations ---

// feishuSDKStream wraps *larkws.Client.
type feishuSDKStream struct {
	cli *larkws.Client
}

func (s *feishuSDKStream) Start(ctx context.Context) error {
	return s.cli.Start(ctx)
}

// feishuSDKSender wraps *lark.Client's im.Message.Create.
type feishuSDKSender struct {
	cli *lark.Client
}

func (s *feishuSDKSender) Create(ctx context.Context, chatID, text string) error {
	content := larkim.NewTextMsgBuilder().Text(text).Build()
	req := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType(larkim.ReceiveIdTypeChatId).
		Body(larkim.NewCreateMessageReqBodyBuilder().
			MsgType(larkim.MsgTypeText).
			ReceiveId(chatID).
			Content(content).
			Build()).
		Build()
	resp, err := s.cli.Im.Message.Create(ctx, req)
	if err != nil {
		return fmt.Errorf("feishu: send failed: %w", err)
	}
	if !resp.Success() {
		return fmt.Errorf("feishu: send failed: code=%d msg=%s requestID=%s",
			resp.Code, resp.Msg, resp.RequestId())
	}
	return nil
}

// baseURLFor maps the descriptor's domain enum to the SDK's open-platform
// base URL. Defaults to the CN (feishu) URL when empty or unknown.
func baseURLFor(domain string) string {
	switch domain {
	case "lark":
		return lark.LarkBaseUrl
	default:
		return lark.FeishuBaseUrl
	}
}
```

Update the imports at the top of `gateway/platforms/feishu_app.go`:

```go
import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"

	"github.com/odysseythink/hermind/gateway"
)
```

- [ ] **Step 2: Hook the SDK into NewFeishuApp**

Replace the tail of `NewFeishuApp` (after the credential checks) with:

```go
	fa := &FeishuApp{
		appID:         appID,
		appSecret:     appSecret,
		domain:        opts["domain"],
		encryptKey:    opts["encrypt_key"],
		defaultChatID: opts["default_chat_id"],
	}

	base := baseURLFor(fa.domain)

	ev := dispatcher.NewEventDispatcher("", fa.encryptKey).
		OnP2MessageReceiveV1(func(ctx context.Context, evt *larkim.P2MessageReceiveV1) error {
			return fa.handleEvent(ctx, evt)
		})
	wsCli := larkws.NewClient(fa.appID, fa.appSecret,
		larkws.WithEventHandler(ev),
		larkws.WithDomain(base),
	)
	fa.stream = &feishuSDKStream{cli: wsCli}

	restCli := lark.NewClient(fa.appID, fa.appSecret,
		lark.WithOpenBaseUrl(base),
	)
	fa.sender = &feishuSDKSender{cli: restCli}

	return fa, nil
}
```

- [ ] **Step 3: Compile**

```bash
go build ./gateway/platforms
```

Expected: exits 0.

- [ ] **Step 4: Run feishu tests to confirm fake-seam tests still work**

```bash
go test ./gateway/platforms -run TestFeishuApp -v
```

Expected: all nine `TestFeishuApp_*` tests PASS. `newFeishuAppForTest` bypasses the SDK construction, so these tests are unaffected.

- [ ] **Step 5: Commit**

```bash
git add gateway/platforms/feishu_app.go
git commit -m "feat(gateway/platforms): wire larkws + lark SDK into FeishuApp"
```

---

### Task 13: Rewrite descriptor + parity update

**Files:**
- Modify: `gateway/platforms/descriptor_feishu.go` (full rewrite)
- Create: `gateway/platforms/descriptor_feishu_test.go`
- Modify: `gateway/platforms/descriptor_parity_test.go` (line 24)

- [ ] **Step 1: Write the failing descriptor test**

Create `gateway/platforms/descriptor_feishu_test.go`:

```go
package platforms

import (
	"testing"
)

func TestDescriptorFeishu_Fields(t *testing.T) {
	d, ok := Get("feishu")
	if !ok {
		t.Fatal("no descriptor registered for feishu")
	}
	if d.DisplayName != "Feishu / Lark (Self-built App)" {
		t.Errorf("DisplayName = %q", d.DisplayName)
	}
	want := []struct {
		Name     string
		Kind     FieldKind
		Required bool
		Enum     []string
	}{
		{"app_id", FieldString, true, nil},
		{"app_secret", FieldSecret, true, nil},
		{"domain", FieldEnum, true, []string{"feishu", "lark"}},
		{"encrypt_key", FieldSecret, false, nil},
		{"default_chat_id", FieldString, false, nil},
	}
	if len(d.Fields) != len(want) {
		t.Fatalf("Fields len = %d, want %d", len(d.Fields), len(want))
	}
	for i, w := range want {
		got := d.Fields[i]
		if got.Name != w.Name {
			t.Errorf("Fields[%d].Name = %q, want %q", i, got.Name, w.Name)
		}
		if got.Kind != w.Kind {
			t.Errorf("Fields[%d].Kind = %v, want %v", i, got.Kind, w.Kind)
		}
		if got.Required != w.Required {
			t.Errorf("Fields[%d].Required = %v, want %v", i, got.Required, w.Required)
		}
		if w.Enum != nil {
			if len(got.Enum) != len(w.Enum) {
				t.Errorf("Fields[%d].Enum len = %d, want %d", i, len(got.Enum), len(w.Enum))
			} else {
				for j := range w.Enum {
					if got.Enum[j] != w.Enum[j] {
						t.Errorf("Fields[%d].Enum[%d] = %q, want %q", i, j, got.Enum[j], w.Enum[j])
					}
				}
			}
		}
	}
}
```

- [ ] **Step 2: Run to verify it fails**

```bash
go test ./gateway/platforms -run TestDescriptorFeishu_Fields -v
```

Expected: FAIL — the existing `descriptor_feishu.go` registers `DisplayName "Feishu / Lark (Bot Webhook)"` with a single `webhook_url` field.

- [ ] **Step 3: Replace descriptor_feishu.go**

Overwrite `gateway/platforms/descriptor_feishu.go`:

```go
package platforms

import "github.com/odysseythink/hermind/gateway"

func init() {
	Register(Descriptor{
		Type:        "feishu",
		DisplayName: "Feishu / Lark (Self-built App)",
		Summary:     "Bidirectional Feishu/Lark adapter via a self-built app long-connection.",
		Fields: []FieldSpec{
			{Name: "app_id", Label: "App ID", Kind: FieldString, Required: true,
				Help: "Self-built app ID from the Feishu Open Platform console."},
			{Name: "app_secret", Label: "App Secret", Kind: FieldSecret, Required: true,
				Help: "App secret paired with App ID."},
			{Name: "domain", Label: "Domain", Kind: FieldEnum, Required: true,
				Enum: []string{"feishu", "lark"}, Default: "feishu",
				Help: "feishu = feishu.cn (CN). lark = larksuite.com (overseas)."},
			{Name: "encrypt_key", Label: "Encrypt Key", Kind: FieldSecret,
				Help: "Only needed when Encrypted Push is enabled in the app console."},
			{Name: "default_chat_id", Label: "Default Chat ID", Kind: FieldString,
				Help: "Fallback chat_id for pushes with no inbound context (e.g. oc_xxxx)."},
		},
		Build: func(opts map[string]string) (gateway.Platform, error) {
			return NewFeishuApp(opts)
		},
	})
}
```

- [ ] **Step 4: Update the parity case**

Open `gateway/platforms/descriptor_parity_test.go`. Replace line 24:

```go
	{"feishu", map[string]string{"webhook_url": "https://open.feishu.cn/open-apis/bot/xxx"}},
```

with:

```go
	{"feishu", map[string]string{"app_id": "a", "app_secret": "s", "domain": "feishu"}},
```

- [ ] **Step 5: Run descriptor + parity tests**

```bash
go test ./gateway/platforms -run 'TestDescriptorFeishu_Fields|TestDescriptorParity' -v
```

Expected: all PASS. The parity test now constructs a real `*FeishuApp` via the SDK path (Task 12 wiring); that's fine because the SDK only starts the WS connection on `Start`, not at construction.

- [ ] **Step 6: Commit**

```bash
git add gateway/platforms/descriptor_feishu.go gateway/platforms/descriptor_feishu_test.go gateway/platforms/descriptor_parity_test.go
git commit -m "feat(gateway/platforms): rewrite feishu descriptor for self-built app"
```

---

### Task 14: Remove the legacy NewFeishu factory

**Files:**
- Modify: `gateway/platforms/chatbots.go` (delete lines 29–38 — the `NewFeishu` function and its preceding doc comment)
- Modify: `gateway/platforms/chatbots_test.go` (delete lines 56–68 — the `feishu` subtest case)

- [ ] **Step 1: Remove the factory from chatbots.go**

Open `gateway/platforms/chatbots.go`. Delete these lines (currently 29–38):

```go
// NewFeishu builds a Feishu / Lark incoming-webhook bot.
// Feishu expects: {"msg_type":"text","content":{"text":"..."}}.
func NewFeishu(url string) *WebhookBot {
	return NewWebhookBot("feishu", url, func(out gateway.OutgoingMessage) any {
		return map[string]any{
			"msg_type": "text",
			"content":  map[string]string{"text": out.Text},
		}
	})
}
```

- [ ] **Step 2: Remove the subtest case from chatbots_test.go**

Open `gateway/platforms/chatbots_test.go`. Delete the feishu case (currently lines 56–68):

```go
		{
			name:         "feishu",
			ctor:         NewFeishu,
			containsText: true,
			assert: func(t *testing.T, body map[string]any) {
				if body["msg_type"] != "text" {
					t.Errorf("feishu msg_type = %v", body["msg_type"])
				}
				c, _ := body["content"].(map[string]any)
				if c == nil || c["text"] != "hi" {
					t.Errorf("feishu content = %v", body["content"])
				}
			},
		},
```

- [ ] **Step 3: Compile**

```bash
go build ./gateway/platforms
```

Expected: exits 0. If the linker complains about `NewFeishu` used elsewhere:

```bash
grep -rn 'NewFeishu\b' .
```

If any result appears outside `chatbots.go`/`chatbots_test.go`, that call site needs fixing in this task. With the current repo layout no other file references it (`chatbots.go:29-38` and `chatbots_test.go` were the only sites).

- [ ] **Step 4: Run the full package**

```bash
go test ./gateway/platforms -v
```

Expected: all tests PASS, including:
- `TestChatbotsSendReply` (slack / discord / mattermost / dingtalk / wecom — feishu subtest gone)
- `TestDescriptorParity_*`
- `TestFeishuApp_*`
- `TestDescriptorFeishu_Fields`

- [ ] **Step 5: Commit**

```bash
git add gateway/platforms/chatbots.go gateway/platforms/chatbots_test.go
git commit -m "refactor(gateway/platforms): drop legacy NewFeishu webhook factory"
```

---

### Task 15: Smoke doc

**Files:**
- Create: `docs/smoke/feishu-app.md`

- [ ] **Step 1: Write the smoke doc**

Create `docs/smoke/feishu-app.md`:

```markdown
# Feishu self-built app smoke flow

Manual verification that the `feishu` adapter connects via long-connection
and round-trips a text message.

## 1. Create a self-built app

1. Open <https://open.feishu.cn/app> and create a custom app.
2. Grab **App ID** and **App Secret** from "Credentials & Basic Info".

## 2. Enable long-connection + permissions

1. Under **Event & Callback → Events**, subscribe to `im.message.receive_v1`.
2. Under **Event & Callback → Subscription Mode**, switch to
   "Use long-connection to receive events".
3. Under **Permissions & Scopes**, grant at least:
   - `im:message` (send messages)
   - `im:message.group_at_msg:readonly` (receive @mentions in groups)
   - `im:message.p2p_msg:readonly` (receive DMs)
4. If you enable **Encrypt Push**, copy the encrypt key.
5. Create a version and release internally.

## 3. Configure hermind

Example `config.yaml`:

```yaml
gateway:
  platforms:
    feishu_main:
      type: feishu
      options:
        app_id: "cli_xxxxxxxxxxxx"
        app_secret: "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
        domain: "feishu"            # use "lark" for overseas Lark
        # encrypt_key: "..."        # only if Encrypt Push is enabled
        # default_chat_id: "oc_xxx" # only for pure-push scenarios
```

Restart `hermind`. Tail the log and look for an SDK line like
`ws client: connect success`.

## 4. Exercise

1. DM the bot from your Feishu account. Expect the app's handler to be
   invoked (gateway log shows inbound) and a reply delivered back in the DM.
2. Add the bot to a group and @mention it. Expect the `@_user_N` token to
   be stripped from the inbound text, and the reply to land in the group.
3. For pure push, omit `default_chat_id` and send an `OutgoingMessage` with
   an empty `ChatID` — expect an error from the gateway layer. Then set
   `default_chat_id` to a chat you control and retry; expect delivery.

## 5. Negative check — legacy config

Put `webhook_url: "https://open.feishu.cn/open-apis/bot/v2/hook/xxxx"`
under a `feishu` instance's options (no `app_id`). Restart. Expect a
startup error mentioning "webhook_url is no longer supported".
```

- [ ] **Step 2: Commit**

```bash
git add docs/smoke/feishu-app.md
git commit -m "docs(smoke): feishu self-built app verification flow"
```

---

### Task 16: CHANGELOG entry

**Files:**
- Modify: `CHANGELOG.md`

- [ ] **Step 1: Inspect the existing format**

```bash
head -40 CHANGELOG.md
```

Note the top-most unreleased heading (if any) and the bullet style so your entry matches.

- [ ] **Step 2: Add the entry**

Open `CHANGELOG.md`. Under the current unreleased section (create one at the top if none exists), add:

```markdown
### Breaking

- **Feishu platform (`feishu`) switched from one-way bot webhook to
  self-built app over long-connection.** The `webhook_url` option is
  removed. Replace it with `app_id`, `app_secret`, `domain`, and
  optionally `encrypt_key` / `default_chat_id`. Recreate your Feishu bot
  as a self-built app in the Open Platform console (see
  `docs/smoke/feishu-app.md`). On startup, any `feishu` instance still
  carrying `webhook_url` without `app_id` will fail with a migration
  error.
```

- [ ] **Step 3: Run the full test suite once more**

```bash
go test ./...
```

Expected: exits 0. Any new failure here is a sign the previous tasks introduced a regression — stop and fix before committing.

- [ ] **Step 4: Commit**

```bash
git add CHANGELOG.md
git commit -m "docs(changelog): feishu breaking — self-built app replaces webhook"
```

---

## Self-review checklist (run by implementer before handing back)

- [ ] `go test ./...` passes.
- [ ] `go vet ./...` is clean.
- [ ] No stray `// TODO` left in `feishu_app.go`.
- [ ] `grep -rn 'NewFeishu\b' .` returns only `NewFeishuApp` hits.
- [ ] `grep -rn 'webhook_url' gateway/platforms/` returns no legacy feishu
      references (the parity test's string was updated in Task 13; a stray
      one here means Task 14 missed a call site).
- [ ] `docs/superpowers/specs/2026-04-21-feishu-long-connection-design.md`
      exists and is unchanged — this plan is its implementation.
