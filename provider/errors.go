// provider/errors.go
package provider

import "errors"

// ErrorKind is the shared error taxonomy across all providers.
type ErrorKind int

const (
	ErrUnknown        ErrorKind = iota // fallback
	ErrRateLimit                       // 429: retry with backoff, eligible for fallback
	ErrAuth                            // 401/403: do not retry
	ErrContentFilter                   // content blocked: do not retry, return to user
	ErrInvalidRequest                  // 400: do not retry, likely a bug
	ErrTimeout                         // request timed out: retry once, then fallback
	ErrServerError                     // 5xx: retry with backoff, eligible for fallback
	ErrContextTooLong                  // context window exceeded: trigger compression
)

// Error is the shared error type returned by all provider implementations.
// Vendor-specific errors are mapped to this type in each provider package.
type Error struct {
	Kind       ErrorKind
	Provider   string // "anthropic", "openai", ...
	StatusCode int    // HTTP status if available, 0 otherwise
	Message    string
	Cause      error // wrapped original error
}

func (e *Error) Error() string {
	if e.Provider != "" {
		return e.Provider + ": " + e.Message
	}
	return e.Message
}

func (e *Error) Unwrap() error {
	return e.Cause
}

// IsRetryable returns true if the error is worth retrying (possibly on
// a different provider via the fallback chain).
func IsRetryable(err error) bool {
	var pErr *Error
	if !errors.As(err, &pErr) {
		return false
	}
	switch pErr.Kind {
	case ErrRateLimit, ErrTimeout, ErrServerError:
		return true
	default:
		return false
	}
}

// ErrAllProvidersFailed is returned when the fallback chain exhausts
// all configured providers.
var ErrAllProvidersFailed = errors.New("provider: all providers failed")
