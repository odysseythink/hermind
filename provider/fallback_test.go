// provider/fallback_test.go
package provider

import (
	"context"
	"errors"
	"testing"

	"github.com/odysseythink/hermind/message"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockProvider is a test helper implementing Provider.
type mockProvider struct {
	name      string
	err       error
	resp      *Response
	callCount int
}

func (m *mockProvider) Name() string { return m.name }
func (m *mockProvider) Complete(ctx context.Context, req *Request) (*Response, error) {
	m.callCount++
	if m.err != nil {
		return nil, m.err
	}
	return m.resp, nil
}
func (m *mockProvider) Stream(ctx context.Context, req *Request) (Stream, error) {
	return nil, errors.New("not implemented")
}
func (m *mockProvider) ModelInfo(string) *ModelInfo                { return nil }
func (m *mockProvider) EstimateTokens(string, string) (int, error) { return 0, nil }
func (m *mockProvider) Available() bool                            { return true }

func TestFallbackFirstProviderSucceeds(t *testing.T) {
	p1 := &mockProvider{name: "primary", resp: &Response{Message: message.Message{Role: message.RoleAssistant, Content: message.TextContent("ok")}}}
	p2 := &mockProvider{name: "secondary"}
	chain := NewFallbackChain([]Provider{p1, p2})

	resp, err := chain.Complete(context.Background(), &Request{Model: "test"})
	require.NoError(t, err)
	assert.Equal(t, "ok", resp.Message.Content.Text())
	assert.Equal(t, 1, p1.callCount)
	assert.Equal(t, 0, p2.callCount)
}

func TestFallbackFirstFailsRetryableSecondSucceeds(t *testing.T) {
	p1 := &mockProvider{name: "primary", err: &Error{Kind: ErrRateLimit, Message: "429"}}
	p2 := &mockProvider{name: "secondary", resp: &Response{Message: message.Message{Role: message.RoleAssistant, Content: message.TextContent("fallback")}}}
	chain := NewFallbackChain([]Provider{p1, p2})

	resp, err := chain.Complete(context.Background(), &Request{Model: "test"})
	require.NoError(t, err)
	assert.Equal(t, "fallback", resp.Message.Content.Text())
	assert.Equal(t, 1, p1.callCount)
	assert.Equal(t, 1, p2.callCount)
}

func TestFallbackAllFail(t *testing.T) {
	p1 := &mockProvider{name: "primary", err: &Error{Kind: ErrRateLimit, Message: "429"}}
	p2 := &mockProvider{name: "secondary", err: &Error{Kind: ErrServerError, Message: "500"}}
	chain := NewFallbackChain([]Provider{p1, p2})

	_, err := chain.Complete(context.Background(), &Request{Model: "test"})
	assert.ErrorIs(t, err, ErrAllProvidersFailed)
}

func TestFallbackStopsOnNonRetryable(t *testing.T) {
	p1 := &mockProvider{name: "primary", err: &Error{Kind: ErrAuth, Message: "bad key"}}
	p2 := &mockProvider{name: "secondary"}
	chain := NewFallbackChain([]Provider{p1, p2})

	_, err := chain.Complete(context.Background(), &Request{Model: "test"})
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrAllProvidersFailed)
	assert.Equal(t, 0, p2.callCount)
}
