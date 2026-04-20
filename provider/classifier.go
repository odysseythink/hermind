// provider/classifier.go
package provider

import (
	"errors"
	"fmt"
	"strings"
)

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
	Reason     FailoverReason
	StatusCode int
	Provider   string
	Model      string
	Message    string

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
//  1. Provider-specific markers (thinking signature, long-context tier)
//  2. Structured provider.Error kinds
//  3. Message-pattern tables
//  4. Disconnect-while-big heuristic → context overflow
//  5. Default: FailoverUnknown (retryable with backoff)
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

	// Permanent-auth marker wins early so billing/auth patterns don't
	// shadow it.
	if strings.Contains(lower, "auth_permanent") {
		out.Reason = FailoverAuthPermanent
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
