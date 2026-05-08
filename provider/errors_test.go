// provider/errors_test.go
package provider

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsRetryableForRateLimit(t *testing.T) {
	err := &Error{Kind: ErrRateLimit, Message: "rate limited"}
	assert.True(t, IsRetryable(err))
}

func TestIsRetryableForServerError(t *testing.T) {
	err := &Error{Kind: ErrServerError, Message: "5xx"}
	assert.True(t, IsRetryable(err))
}

func TestIsRetryableForTimeout(t *testing.T) {
	err := &Error{Kind: ErrTimeout, Message: "timeout"}
	assert.True(t, IsRetryable(err))
}

func TestNotRetryableForAuth(t *testing.T) {
	err := &Error{Kind: ErrAuth, Message: "bad key"}
	assert.False(t, IsRetryable(err))
}

func TestNotRetryableForContentFilter(t *testing.T) {
	err := &Error{Kind: ErrContentFilter, Message: "blocked"}
	assert.False(t, IsRetryable(err))
}

func TestNotRetryableForContextTooLong(t *testing.T) {
	err := &Error{Kind: ErrContextTooLong, Message: "too long"}
	assert.False(t, IsRetryable(err))
}

func TestIsRetryableForNonProviderError(t *testing.T) {
	err := errors.New("random error")
	assert.False(t, IsRetryable(err))
}

func TestErrorUnwrap(t *testing.T) {
	cause := errors.New("network")
	err := &Error{Kind: ErrTimeout, Message: "slow", Cause: cause}
	assert.ErrorIs(t, err, cause)
}
