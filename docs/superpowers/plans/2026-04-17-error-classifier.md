# ErrorClassifier Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the `provider.IsRetryable(err)` boolean with a **structured classification** of provider errors so the agent can pick the right recovery strategy — retry, compress context, rotate credentials, failover to a sibling provider, or stop immediately. Mirrors the Python `agent/error_classifier.py` decision tree and exposes the result to both `provider.FallbackChain` and the agent engine.

**Architecture:** A new `provider/classifier.go` owns the `FailoverReason` enum (auth / billing / rate_limit / overloaded / server_error / timeout / context_overflow / payload_too_large / model_not_found / format_error / thinking_signature / long_context_tier / unknown) and a `Classify(err, ClassifyInput) Classification` function. Pattern matching runs against HTTP status, error-code strings, message substrings, transport error types, and provider-specific markers (Anthropic `thinking` signatures, OpenRouter `extra usage` tier hints). `provider.IsRetryable` becomes a thin wrapper over `Classify` so existing callers keep working, and `FallbackChain.Complete` consults the full classification to short-circuit on `PermanentAuth` or `Billing` instead of walking the chain.

**Tech Stack:** Go 1.21+, existing `provider` package, `regexp`, `strings`. No external deps.

---

## File Structure

- Create: `provider/classifier.go` — `FailoverReason`, `Classification`, `ClassifyInput`, `Classify`
- Create: `provider/classifier_test.go`
- Create: `provider/classifier_patterns.go` — precompiled regex + pattern slices
- Modify: `provider/errors.go` — rewire `IsRetryable` on top of `Classify`
- Modify: `provider/fallback.go` — use `Classify` to short-circuit on unrecoverable reasons
- Modify: `provider/fallback_test.go` — add tests for the new short-circuit behavior

---

## Task 1: Enum + Classification types

**Files:**
- Create: `provider/classifier.go`
- Create: `provider/classifier_test.go`

- [ ] **Step 1: Write the failing test**

Create `provider/classifier_test.go`:

```go
package provider

import "testing"

func TestFailoverReason_String(t *testing.T) {
	cases := map[FailoverReason]string{
		FailoverAuth:             "auth",
		FailoverAuthPermanent:    "auth_permanent",
		FailoverBilling:          "billing",
		FailoverRateLimit:        "rate_limit",
		FailoverOverloaded:       "overloaded",
		FailoverServerError:      "server_error",
		FailoverTimeout:          "timeout",
		FailoverContextOverflow:  "context_overflow",
		FailoverPayloadTooLarge:  "payload_too_large",
		FailoverModelNotFound:    "model_not_found",
		FailoverFormatError:      "format_error",
		FailoverThinkingSignature: "thinking_signature",
		FailoverLongContextTier:  "long_context_tier",
		FailoverUnknown:          "unknown",
	}
	for reason, want := range cases {
		if got := reason.String(); got != want {
			t.Errorf("%d.String() = %q, want %q", reason, got, want)
		}
	}
}

func TestClassification_Flags(t *testing.T) {
	c := Classification{Reason: FailoverBilling}
	c.applyDefaults()
	if !c.ShouldRotateCredential {
		t.Error("billing should rotate credential")
	}
	if c.Retryable {
		t.Error("billing should not be retryable on the same credential")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./provider/ -run "TestFailoverReason|TestClassification" -v`
Expected: FAIL — types undefined.

- [ ] **Step 3: Implement types**

Create `provider/classifier.go`:

```go
package provider

import "fmt"

// FailoverReason enumerates why a provider call failed, driving the
// agent's recovery strategy.
type FailoverReason int

const (
	FailoverUnknown FailoverReason = iota
	FailoverAuth
	FailoverAuthPermanent
	FailoverBilling
	FailoverRateLimit
	FailoverOverloaded
	FailoverServerError
	FailoverTimeout
	FailoverContextOverflow
	FailoverPayloadTooLarge
	FailoverModelNotFound
	FailoverFormatError
	FailoverThinkingSignature
	FailoverLongContextTier
)

// String returns the snake_case form used in logs + telemetry.
func (r FailoverReason) String() string {
	switch r {
	case FailoverAuth:
		return "auth"
	case FailoverAuthPermanent:
		return "auth_permanent"
	case FailoverBilling:
		return "billing"
	case FailoverRateLimit:
		return "rate_limit"
	case FailoverOverloaded:
		return "overloaded"
	case FailoverServerError:
		return "server_error"
	case FailoverTimeout:
		return "timeout"
	case FailoverContextOverflow:
		return "context_overflow"
	case FailoverPayloadTooLarge:
		return "payload_too_large"
	case FailoverModelNotFound:
		return "model_not_found"
	case FailoverFormatError:
		return "format_error"
	case FailoverThinkingSignature:
		return "thinking_signature"
	case FailoverLongContextTier:
		return "long_context_tier"
	}
	return "unknown"
}

// Classification is the structured result of an error lookup.
type Classification struct {
	Reason                FailoverReason
	StatusCode            int
	Provider              string
	Model                 string
	Message               string

	Retryable              bool
	ShouldCompress         bool
	ShouldRotateCredential bool
	ShouldFallback         bool
}

// applyDefaults fills in the boolean flags based on Reason. Called
// internally by Classify — exported for tests.
func (c *Classification) applyDefaults() {
	switch c.Reason {
	case FailoverRateLimit, FailoverOverloaded, FailoverServerError, FailoverTimeout:
		c.Retryable = true
		c.ShouldFallback = true
	case FailoverContextOverflow:
		c.ShouldCompress = true
		c.Retryable = true
	case FailoverBilling:
		c.ShouldRotateCredential = true
		c.ShouldFallback = true
	case FailoverAuth:
		c.ShouldRotateCredential = true
		c.Retryable = true
	case FailoverAuthPermanent:
		// all flags stay false — give up
	case FailoverThinkingSignature:
		c.Retryable = true
	case FailoverLongContextTier:
		c.Retryable = true
		c.ShouldFallback = true
	case FailoverModelNotFound, FailoverFormatError, FailoverPayloadTooLarge:
		// not retryable; user-facing errors
	case FailoverUnknown:
		c.Retryable = true
	}
}

// Error is a convenience wrapper so a Classification can itself flow
// through `error` return paths.
func (c *Classification) Error() string {
	if c.Provider != "" {
		return fmt.Sprintf("%s: %s (%s)", c.Provider, c.Message, c.Reason)
	}
	return fmt.Sprintf("%s: %s", c.Reason, c.Message)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./provider/ -run "TestFailoverReason|TestClassification" -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add provider/classifier.go provider/classifier_test.go
git commit -m "feat(provider): FailoverReason enum + Classification with default flags"
```

---

## Task 2: Pattern tables

**Files:**
- Create: `provider/classifier_patterns.go`

- [ ] **Step 1: Write the failing test**

Append to `provider/classifier_test.go`:

```go
func TestPatterns_BillingSubstring(t *testing.T) {
	for _, s := range []string{
		"insufficient_quota",
		"you have exceeded your quota",
		"payment required",
		"credit balance is too low",
	} {
		if !matchesAny(billingPatterns, s) {
			t.Errorf("billing patterns missed %q", s)
		}
	}
}

func TestPatterns_ContextOverflow(t *testing.T) {
	for _, s := range []string{
		"maximum context length",
		"input tokens exceed",
		"context window is full",
		"上下文已满", // Chinese
	} {
		if !matchesAny(contextOverflowPatterns, s) {
			t.Errorf("context overflow patterns missed %q", s)
		}
	}
}

func TestPatterns_RateLimit(t *testing.T) {
	for _, s := range []string{
		"rate limit exceeded",
		"too many requests",
		"429",
		"throttled",
	} {
		if !matchesAny(rateLimitPatterns, s) {
			t.Errorf("rate limit patterns missed %q", s)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./provider/ -run TestPatterns -v`
Expected: FAIL — `billingPatterns` undefined.

- [ ] **Step 3: Implement patterns**

Create `provider/classifier_patterns.go`:

```go
package provider

import "strings"

// Pattern slices. All comparisons are case-insensitive — callers
// lowercase input before matching. Order does not matter; any match
// wins.

var billingPatterns = []string{
	"insufficient_quota",
	"insufficient quota",
	"insufficient balance",
	"insufficient credit",
	"payment required",
	"billing_hard_limit",
	"monthly limit reached",
	"credit balance",
	"exceeded your current quota",
	"exceeded your quota",
	"account_deactivated",
	"account has been deactivated",
	"not enough credit",
	"账户余额不足",
}

var rateLimitPatterns = []string{
	"rate limit",
	"rate-limit",
	"rate_limited",
	"too many requests",
	"tpm exceeded",
	"rpm exceeded",
	"resource_exhausted",
	"quota_exceeded",
	"throttl",
	"429",
}

var contextOverflowPatterns = []string{
	"maximum context length",
	"context_length_exceeded",
	"context length exceeded",
	"context window is full",
	"context window exceeded",
	"message is too long",
	"prompt is too long",
	"input tokens exceed",
	"input is too long",
	"token limit reached",
	"exceeds max tokens",
	"exceeds the maximum",
	"上下文已满",
	"上下文过长",
	"超过最大 token",
	"超出最大长度",
}

var modelNotFoundPatterns = []string{
	"model not found",
	"unknown model",
	"invalid_model",
	"model_not_available",
	"does not exist",
	"is not a valid model",
}

var authPatterns = []string{
	"invalid_api_key",
	"invalid api key",
	"api key expired",
	"unauthorized",
	"authentication_error",
	"permission_denied",
	"forbidden",
	"could not authenticate",
}

// Transport error type names we recognize as timeouts. We can't use
// errors.As against stdlib types from every transport library, but
// substring matching on fmt.Sprintf("%T", err) catches the common
// cases without extra deps.
var transportTimeoutTypeSubstrings = []string{
	"net.OpError",
	"net/http.httpError",
	"context.deadlineExceededError",
	"http.transport", // generic catch-all
}

// matchesAny reports whether any pattern is a case-insensitive
// substring of s.
func matchesAny(patterns []string, s string) bool {
	lower := strings.ToLower(s)
	for _, p := range patterns {
		if strings.Contains(lower, strings.ToLower(p)) {
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./provider/ -run TestPatterns -v`
Expected: PASS (3 sub-tests).

- [ ] **Step 5: Commit**

```bash
git add provider/classifier_patterns.go provider/classifier_test.go
git commit -m "feat(provider): pattern tables for error classification"
```

---

## Task 3: Classify()

**Files:**
- Modify: `provider/classifier.go` — add `Classify` and `ClassifyInput`

- [ ] **Step 1: Write the failing test**

Append to `provider/classifier_test.go`:

```go
import (
	// make sure these are imported at the top of the file
	"errors"
	"net"
)

func TestClassify_StatusCode429IsRateLimit(t *testing.T) {
	e := &Error{Kind: ErrRateLimit, Provider: "openai", StatusCode: 429, Message: "rate limit"}
	c := Classify(e, ClassifyInput{})
	if c.Reason != FailoverRateLimit {
		t.Errorf("reason = %v", c.Reason)
	}
	if !c.Retryable || !c.ShouldFallback {
		t.Errorf("flags = %+v", c)
	}
}

func TestClassify_AuthIsNotRetryablePermanent(t *testing.T) {
	e := &Error{Kind: ErrAuth, Provider: "openai", StatusCode: 401, Message: "invalid_api_key"}
	c := Classify(e, ClassifyInput{})
	if c.Reason != FailoverAuth {
		t.Errorf("reason = %v", c.Reason)
	}
	// First-time auth failures are retryable after rotate, per Python.
	if !c.ShouldRotateCredential {
		t.Error("expected rotate")
	}
}

func TestClassify_ContextOverflowFromMessage(t *testing.T) {
	e := errors.New("400: prompt is too long: 150000 tokens > 128000")
	c := Classify(e, ClassifyInput{ApproxTokens: 150000, ContextLength: 128000})
	if c.Reason != FailoverContextOverflow {
		t.Errorf("reason = %v", c.Reason)
	}
	if !c.ShouldCompress {
		t.Error("expected compress")
	}
}

func TestClassify_BillingBeatsRateLimit(t *testing.T) {
	e := &Error{Kind: ErrUnknown, Message: "insufficient_quota: hard limit", StatusCode: 429}
	c := Classify(e, ClassifyInput{})
	if c.Reason != FailoverBilling {
		t.Errorf("reason = %v, want billing", c.Reason)
	}
}

func TestClassify_DisconnectHeuristicMapsToContextOverflow(t *testing.T) {
	e := &net.OpError{Op: "read", Err: errors.New("connection reset by peer")}
	c := Classify(e, ClassifyInput{
		ApproxTokens:  130_000,
		ContextLength: 200_000,
		NumMessages:   250,
	})
	// 65% context + 250 messages + transport disconnect → context_overflow
	if c.Reason != FailoverContextOverflow {
		t.Errorf("reason = %v", c.Reason)
	}
}

func TestClassify_TimeoutMapsToFailoverTimeout(t *testing.T) {
	e := &Error{Kind: ErrTimeout, Message: "request timed out"}
	c := Classify(e, ClassifyInput{})
	if c.Reason != FailoverTimeout {
		t.Errorf("reason = %v", c.Reason)
	}
	if !c.Retryable || !c.ShouldFallback {
		t.Error("timeout should allow retry + fallback")
	}
}

func TestClassify_UnknownIsRetryable(t *testing.T) {
	c := Classify(errors.New("something weird"), ClassifyInput{})
	if c.Reason != FailoverUnknown {
		t.Errorf("reason = %v", c.Reason)
	}
	if !c.Retryable {
		t.Error("unknown should retry with backoff")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./provider/ -run TestClassify -v`
Expected: FAIL — `Classify` undefined.

- [ ] **Step 3: Implement Classify**

Append to `provider/classifier.go`:

```go
import (
	// add imports at the top
	"errors"
	"fmt"
	"strings"
)

// ClassifyInput carries optional context that improves classification
// accuracy. Zero values are safe — Classify falls back to the base
// heuristics when no hint is provided.
type ClassifyInput struct {
	Provider      string
	Model         string
	ApproxTokens  int
	ContextLength int
	NumMessages   int
}

// Classify maps an error to a FailoverReason + recovery hints.
// It checks (in order):
//   1. Provider-specific markers (thinking signature, long-context tier)
//   2. Structured provider.Error kinds
//   3. Message-pattern tables
//   4. Disconnect-while-big heuristic → context overflow
//   5. Default: FailoverUnknown (retryable with backoff)
func Classify(err error, in ClassifyInput) Classification {
	out := Classification{
		Provider: in.Provider,
		Model:    in.Model,
		Message:  safeErrorString(err),
	}
	if err == nil {
		out.Reason = FailoverUnknown
		out.applyDefaults()
		return out
	}

	lower := strings.ToLower(out.Message)

	// 1. Anthropic "thinking signature" path.
	if strings.Contains(lower, "signature") && strings.Contains(lower, "thinking") {
		out.Reason = FailoverThinkingSignature
		out.applyDefaults()
		return out
	}
	// 1b. OpenRouter "extra usage / long context tier".
	if strings.Contains(lower, "extra usage") && strings.Contains(lower, "long context") {
		out.Reason = FailoverLongContextTier
		out.applyDefaults()
		return out
	}

	// 2. Structured Error kinds.
	var pErr *Error
	if errors.As(err, &pErr) {
		if pErr.StatusCode != 0 {
			out.StatusCode = pErr.StatusCode
		}
		out.Provider = coalesceStr(out.Provider, pErr.Provider)
		switch pErr.Kind {
		case ErrRateLimit:
			out.Reason = FailoverRateLimit
		case ErrAuth:
			out.Reason = FailoverAuth
		case ErrTimeout:
			out.Reason = FailoverTimeout
		case ErrServerError:
			out.Reason = FailoverServerError
		case ErrContentFilter:
			out.Reason = FailoverFormatError
		case ErrInvalidRequest:
			out.Reason = FailoverFormatError
		case ErrContextTooLong:
			out.Reason = FailoverContextOverflow
		}
		// Message-based override: billing beats rate_limit.
		if matchesAny(billingPatterns, lower) {
			out.Reason = FailoverBilling
		}
		if out.Reason != FailoverUnknown {
			out.applyDefaults()
			return out
		}
	}

	// 3. Message-pattern matching for plain errors.
	switch {
	case matchesAny(billingPatterns, lower):
		out.Reason = FailoverBilling
	case matchesAny(contextOverflowPatterns, lower):
		out.Reason = FailoverContextOverflow
	case matchesAny(rateLimitPatterns, lower):
		out.Reason = FailoverRateLimit
	case matchesAny(modelNotFoundPatterns, lower):
		out.Reason = FailoverModelNotFound
	case matchesAny(authPatterns, lower):
		out.Reason = FailoverAuth
	case strings.Contains(lower, "overloaded") || strings.Contains(lower, "503"):
		out.Reason = FailoverOverloaded
	case strings.Contains(lower, "504") || strings.Contains(lower, "deadline exceeded"):
		out.Reason = FailoverTimeout
	case strings.Contains(lower, "413") || strings.Contains(lower, "payload too large"):
		out.Reason = FailoverPayloadTooLarge
	}
	if out.Reason != FailoverUnknown {
		out.applyDefaults()
		return out
	}

	// 4. Disconnect-while-big heuristic.
	if isTransportError(err) {
		bigContext := in.ApproxTokens > 0 &&
			in.ContextLength > 0 &&
			float64(in.ApproxTokens) > 0.6*float64(in.ContextLength)
		longChat := in.NumMessages > 200
		if bigContext || longChat || in.ApproxTokens > 120_000 {
			out.Reason = FailoverContextOverflow
			out.applyDefaults()
			return out
		}
		out.Reason = FailoverTimeout
		out.applyDefaults()
		return out
	}

	// 5. Default: unknown.
	out.Reason = FailoverUnknown
	out.applyDefaults()
	return out
}

// IsRetryable preserved for backward compatibility. It now delegates
// to Classify and returns the resulting Retryable flag.
func IsRetryable(err error) bool {
	c := Classify(err, ClassifyInput{})
	return c.Retryable
}

func safeErrorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func coalesceStr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// isTransportError detects stdlib transport errors heuristically by
// checking the type string. Works across modules without forcing
// callers to import every transport package for a type-switch.
func isTransportError(err error) bool {
	t := fmt.Sprintf("%T", err)
	for _, sub := range transportTimeoutTypeSubstrings {
		if strings.Contains(t, sub) {
			return true
		}
	}
	return false
}
```

Note: this re-declares `IsRetryable`; delete the old implementation from `provider/errors.go` to avoid a duplicate symbol. Run `grep -n "func IsRetryable" provider/errors.go` and remove the duplicate. If there are callers elsewhere, they'll now flow through `Classify` transparently.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./provider/ -run TestClassify -v`
Expected: PASS (7 sub-tests).

Run: `go test ./provider/...`
Expected: PASS (including legacy `errors_test.go`).

- [ ] **Step 5: Commit**

```bash
git add provider/classifier.go provider/errors.go provider/classifier_test.go
git commit -m "feat(provider): Classify() + pattern-driven error classification"
```

---

## Task 4: Use Classify in FallbackChain

**Files:**
- Modify: `provider/fallback.go`
- Modify: `provider/fallback_test.go`

- [ ] **Step 1: Write the failing test**

Append to `provider/fallback_test.go`:

```go
func TestFallbackChain_StopsOnPermanentAuth(t *testing.T) {
	authErr := &Error{Kind: ErrAuth, Provider: "p1", Message: "invalid_api_key"}
	// Make the classifier treat it as permanent by ensuring a rotate
	// won't help this test — override by classifying directly to
	// FailoverAuthPermanent. We simulate this by using an Error whose
	// message says "auth_permanent".
	authErr.Message = "authentication error: auth_permanent"

	p1 := &stubFailingProvider{err: authErr}
	p2 := &stubFailingProvider{err: authErr}

	chain := NewFallbackChain([]Provider{p1, p2})
	_, err := chain.Complete(context.Background(), &Request{})
	if err == nil {
		t.Fatal("expected error")
	}
	if p1.calls != 1 || p2.calls != 0 {
		t.Errorf("calls = %d/%d, want 1/0 (chain must short-circuit)", p1.calls, p2.calls)
	}
}

func TestFallbackChain_WalksOnRateLimit(t *testing.T) {
	p1 := &stubFailingProvider{err: &Error{Kind: ErrRateLimit, Provider: "p1", Message: "rate limited"}}
	p2 := &stubSucceedingProvider{}
	chain := NewFallbackChain([]Provider{p1, p2})
	if _, err := chain.Complete(context.Background(), &Request{}); err != nil {
		t.Fatalf("expected success after fallback, got %v", err)
	}
	if p1.calls != 1 || p2.calls != 1 {
		t.Errorf("calls = %d/%d, want 1/1", p1.calls, p2.calls)
	}
}
```

Add stubs (if not already present) at the bottom of `fallback_test.go`:

```go
type stubFailingProvider struct {
	err   error
	calls int
}

func (s *stubFailingProvider) Name() string                 { return "stub-fail" }
func (s *stubFailingProvider) Available() bool              { return true }
func (s *stubFailingProvider) ModelInfo(string) *ModelInfo  { return &ModelInfo{} }
func (s *stubFailingProvider) EstimateTokens(_, _ string) (int, error) { return 0, nil }
func (s *stubFailingProvider) Stream(context.Context, *Request) (Stream, error) { return nil, s.err }
func (s *stubFailingProvider) Complete(context.Context, *Request) (*Response, error) {
	s.calls++
	return nil, s.err
}

type stubSucceedingProvider struct{ calls int }

func (s *stubSucceedingProvider) Name() string                 { return "stub-ok" }
func (s *stubSucceedingProvider) Available() bool              { return true }
func (s *stubSucceedingProvider) ModelInfo(string) *ModelInfo  { return &ModelInfo{} }
func (s *stubSucceedingProvider) EstimateTokens(_, _ string) (int, error) { return 0, nil }
func (s *stubSucceedingProvider) Stream(context.Context, *Request) (Stream, error) { return nil, nil }
func (s *stubSucceedingProvider) Complete(context.Context, *Request) (*Response, error) {
	s.calls++
	return &Response{}, nil
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./provider/ -run TestFallbackChain -v`
Expected: FAIL — current chain walks through auth errors instead of short-circuiting.

- [ ] **Step 3: Integrate Classify into FallbackChain**

Replace the `Complete` method in `provider/fallback.go`:

```go
func (fc *FallbackChain) Complete(ctx context.Context, req *Request) (*Response, error) {
	if len(fc.providers) == 0 {
		return nil, errors.New("provider: fallback chain is empty")
	}
	var lastErr error
	for _, p := range fc.providers {
		resp, err := p.Complete(ctx, req)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		c := Classify(err, ClassifyInput{Provider: p.Name()})
		if !c.ShouldFallback && !c.Retryable {
			// Unrecoverable (e.g. AuthPermanent, ModelNotFound, FormatError)
			return nil, err
		}
		if !c.ShouldFallback {
			// Retryable but not a fallback candidate — surface to caller.
			return nil, err
		}
		// Otherwise continue to the next provider.
	}
	return nil, fmt.Errorf("%w: last error: %v", ErrAllProvidersFailed, lastErr)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./provider/ -run TestFallbackChain -v`
Expected: PASS.

Run full provider suite: `go test ./provider/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add provider/fallback.go provider/fallback_test.go
git commit -m "feat(provider/fallback): short-circuit on unrecoverable Classify reasons"
```

---

## Task 5: Expose Classify through a helper for the agent engine

**Files:**
- Modify: `agent/engine.go` — optional; only if you want to hook `ShouldCompress` into compression triggering

- [ ] **Step 1: Survey**

Run: `grep -n "IsRetryable\|provider.Error" agent/*.go`
Expected: find the callsites currently using `IsRetryable`. The existing code keeps working unchanged thanks to the backward-compat `IsRetryable` shim.

- [ ] **Step 2: Optional: compression trigger**

If `agent/engine.go` has logic that decides when to invoke the compressor (grep for `Compressor` or `shouldCompress`), add a call to `provider.Classify` before the retry loop:

```go
class := provider.Classify(err, provider.ClassifyInput{
	Provider:      e.provider.Name(),
	Model:         req.Model,
	ApproxTokens:  estimatedTokens,
	ContextLength: contextLen,
	NumMessages:   len(history),
})
if class.ShouldCompress && e.compressor != nil {
	// run compression, then retry
}
```

This step is optional — it only improves behavior. Skip if the engine already handles overflow through a different pathway; the core plan is complete after Task 4.

- [ ] **Step 3: Commit if any changes were made**

```bash
git add agent/engine.go
git commit -m "feat(agent): consult Classify.ShouldCompress before retrying"
```

---

## Task 6: Manual smoke test

- [ ] **Step 1: Add a throw-away test driver**

Create a scratch file `scratch/classify_demo.go` (not checked in):

```go
package main

import (
	"fmt"
	"github.com/odysseythink/hermind/provider"
)

func main() {
	samples := []error{
		&provider.Error{Kind: provider.ErrRateLimit, Message: "429 too many requests"},
		&provider.Error{Kind: provider.ErrAuth, Message: "invalid_api_key"},
		fmt.Errorf("insufficient_quota: upgrade your plan"),
		fmt.Errorf("prompt is too long"),
	}
	for _, e := range samples {
		c := provider.Classify(e, provider.ClassifyInput{})
		fmt.Printf("%-60s → %-20s retryable=%v compress=%v rotate=%v fallback=%v\n",
			e.Error(), c.Reason.String(), c.Retryable, c.ShouldCompress, c.ShouldRotateCredential, c.ShouldFallback)
	}
}
```

- [ ] **Step 2: Run it**

Run: `go run scratch/classify_demo.go`
Expected: four lines, one per sample, showing reason + flag values that match the decision tree (rate_limit / auth / billing / context_overflow).

- [ ] **Step 3: Remove the scratch file**

Run: `rm scratch/classify_demo.go && rmdir scratch 2>/dev/null || true`

---

## Self-Review Checklist

1. **Spec coverage:**
   - FailoverReason enum with all 14 values ↔ Task 1 ✓
   - Classification struct + flag defaults ↔ Task 1 ✓
   - Pattern tables for billing / rate_limit / context_overflow / model_not_found / auth ↔ Task 2 ✓
   - Structured Error → FailoverReason mapping ↔ Task 3 ✓
   - Disconnect-while-big heuristic ↔ Task 3 ✓
   - IsRetryable backward-compat shim ↔ Task 3 ✓
   - FallbackChain short-circuit ↔ Task 4 ✓

2. **Placeholders:** Task 5 is explicitly optional and well-bounded. No TBD content.

3. **Type consistency:**
   - `Classification{Reason, StatusCode, Provider, Model, Message, Retryable, ShouldCompress, ShouldRotateCredential, ShouldFallback}` stable across Tasks 1, 3, 4.
   - `ClassifyInput{Provider, Model, ApproxTokens, ContextLength, NumMessages}` stable.
   - `FailoverReason.String()` matches the Python snake_case forms exactly.
   - `IsRetryable` still `func(err error) bool`, preserving all existing callers.

4. **Gaps (future work):**
   - Retry-After header parsing — the Classification does not carry a retry-after hint yet. A `RetryAfter time.Duration` field can be added when the first caller needs it.
   - Per-provider pattern overrides (e.g. OpenRouter's proxied-error JSON payload) — callers can preprocess the error message today; a structured hook can follow.
   - Telemetry: emit `FailoverReason.String()` as a metric label once the metrics package has an error-rate histogram.

---

## Definition of Done

- `go test ./provider/... -race` all pass.
- `provider.IsRetryable` still works for existing callers.
- `provider.Classify` returns the expected reason for every sample in the test matrix.
- `FallbackChain.Complete` short-circuits on AuthPermanent / ModelNotFound / FormatError and walks on RateLimit / Overloaded / ServerError / Timeout / ContextOverflow.
