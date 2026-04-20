// provider/classifier_test.go
package provider

import (
	"errors"
	"net"
	"testing"
)

func TestFailoverReason_String(t *testing.T) {
	cases := map[FailoverReason]string{
		FailoverAuth:              "auth",
		FailoverAuthPermanent:     "auth_permanent",
		FailoverBilling:           "billing",
		FailoverRateLimit:         "rate_limit",
		FailoverOverloaded:        "overloaded",
		FailoverServerError:       "server_error",
		FailoverTimeout:           "timeout",
		FailoverContextOverflow:   "context_overflow",
		FailoverPayloadTooLarge:   "payload_too_large",
		FailoverModelNotFound:     "model_not_found",
		FailoverFormatError:       "format_error",
		FailoverThinkingSignature: "thinking_signature",
		FailoverLongContextTier:   "long_context_tier",
		FailoverUnknown:           "unknown",
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
