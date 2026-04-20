// provider/auxiliary.go
package provider

import (
	"context"
	"errors"

	"github.com/odysseythink/hermind/message"
)

// AuxClient runs lightweight secondary tasks (summarization, vision
// descriptions, cheap re-ranking) across a fallback chain of providers.
// It does not expose tool-use — callers just pass a system prompt and
// a user message and get a text reply back.
//
// AuxClient is intended for use by the agent's compressor, memory
// manager, and any other component that needs cheap LLM calls
// separate from the primary conversation model.
type AuxClient struct {
	providers []Provider
	chain     *FallbackChain
}

// NewAuxClient builds a client from an ordered provider list. The
// first available provider is tried first; IsRetryable / Classify
// decide when to try the next one. nil/empty inputs are allowed —
// Ask will return an error at call time.
func NewAuxClient(providers []Provider) *AuxClient {
	filtered := make([]Provider, 0, len(providers))
	for _, p := range providers {
		if p == nil {
			continue
		}
		filtered = append(filtered, p)
	}
	return &AuxClient{
		providers: filtered,
		chain:     NewFallbackChain(filtered),
	}
}

// Providers returns the ordered provider list wrapped by this client.
// Intended for diagnostics / tests.
func (a *AuxClient) Providers() []Provider {
	out := make([]Provider, len(a.providers))
	copy(out, a.providers)
	return out
}

// Ask sends a two-message request and returns the assistant text.
// system is the instruction prompt; user is the content to operate on.
// MaxTokens defaults to 2048 unless the caller uses AskWithRequest.
func (a *AuxClient) Ask(ctx context.Context, system, user string) (string, error) {
	return a.AskWithRequest(ctx, &Request{
		SystemPrompt: system,
		Messages: []message.Message{
			{Role: message.RoleUser, Content: message.TextContent(user)},
		},
		MaxTokens: 2048,
	})
}

// AskWithRequest is the escape hatch for callers (e.g. the compressor)
// that want to supply a fully-formed Request — typically to override
// Model or inject long context as a user turn.
func (a *AuxClient) AskWithRequest(ctx context.Context, req *Request) (string, error) {
	if a == nil || len(a.providers) == 0 || a.chain == nil {
		return "", errors.New("provider: auxiliary chain is empty")
	}
	resp, err := a.chain.Complete(ctx, req)
	if err != nil {
		return "", err
	}
	if resp.Message.Content.IsText() {
		return resp.Message.Content.Text(), nil
	}
	// Fall back to concatenating text blocks.
	var text string
	for _, b := range resp.Message.Content.Blocks() {
		if b.Type == "text" {
			text += b.Text
		}
	}
	return text, nil
}

// Complete lets callers reuse the AuxClient as a Provider-like object
// (it returns a full Response, not just text). Errors propagate from
// the underlying fallback chain.
func (a *AuxClient) Complete(ctx context.Context, req *Request) (*Response, error) {
	if a == nil || len(a.providers) == 0 || a.chain == nil {
		return nil, errors.New("provider: auxiliary chain is empty")
	}
	return a.chain.Complete(ctx, req)
}
