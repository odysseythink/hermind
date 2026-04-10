// provider/fallback.go
package provider

import (
	"context"
	"errors"
	"fmt"
)

// FallbackChain tries each provider in order and stops at the first success
// or a non-retryable error. It is single-use and not thread-safe. Create
// a new chain per conversation to avoid shared mutable state.
type FallbackChain struct {
	providers []Provider
}

// NewFallbackChain constructs a chain from an ordered list of providers.
// The first provider is the primary; others are tried in order on failure.
func NewFallbackChain(providers []Provider) *FallbackChain {
	return &FallbackChain{providers: providers}
}

// Complete tries each provider in order until one succeeds or a
// non-retryable error is encountered.
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
		if !IsRetryable(err) {
			// Stop on non-retryable errors (auth, content filter, bad request)
			return nil, err
		}
		// Continue to the next provider
	}
	return nil, fmt.Errorf("%w: last error: %v", ErrAllProvidersFailed, lastErr)
}

// Stream tries each provider in order for a streaming call.
// Note: once a stream is returned, subsequent failures during Recv() are
// the caller's responsibility to handle (typically by restarting the call).
func (fc *FallbackChain) Stream(ctx context.Context, req *Request) (Stream, error) {
	if len(fc.providers) == 0 {
		return nil, errors.New("provider: fallback chain is empty")
	}

	var lastErr error
	for _, p := range fc.providers {
		stream, err := p.Stream(ctx, req)
		if err == nil {
			return stream, nil
		}
		lastErr = err
		if !IsRetryable(err) {
			return nil, err
		}
	}
	return nil, fmt.Errorf("%w: last error: %v", ErrAllProvidersFailed, lastErr)
}
