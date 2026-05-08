// provider/classifier_patterns.go
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
