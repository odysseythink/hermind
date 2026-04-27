// agent/conversation_test.go
package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/provider"
	"github.com/stretchr/testify/assert"
)

// estimatorProvider is a minimal Provider stub that lets a test drive the
// token-budget trigger of shouldCompress. ModelInfo and EstimateTokens are
// the only methods the trigger calls.
type estimatorProvider struct {
	contextLen int
	tokensFor  func(text string) int
}

func (p *estimatorProvider) Name() string      { return "estimator" }
func (p *estimatorProvider) Available() bool   { return true }
func (p *estimatorProvider) ModelInfo(string) *provider.ModelInfo {
	return &provider.ModelInfo{ContextLength: p.contextLen}
}
func (p *estimatorProvider) EstimateTokens(_ string, text string) (int, error) {
	if p.tokensFor == nil {
		return len(text) / 4, nil
	}
	return p.tokensFor(text), nil
}
func (p *estimatorProvider) Stream(context.Context, *provider.Request) (provider.Stream, error) {
	return nil, nil
}
func (p *estimatorProvider) Complete(context.Context, *provider.Request) (*provider.Response, error) {
	return nil, nil
}

func TestShouldCompress_DisabledOrZeroProtectLast(t *testing.T) {
	hist := []message.Message{{Role: message.RoleUser, Content: message.TextContent("hi")}}

	assert.False(t, shouldCompress(hist, config.CompressionConfig{Enabled: false, ProtectLast: 20}, nil, ""),
		"disabled compression must never trigger")
	assert.False(t, shouldCompress(hist, config.CompressionConfig{Enabled: true, ProtectLast: 0}, nil, ""),
		"zero ProtectLast must never trigger")
}

func TestShouldCompress_MessageCountTrigger(t *testing.T) {
	cfg := config.CompressionConfig{Enabled: true, ProtectLast: 5} // threshold = 15 messages
	hist := make([]message.Message, 16)
	for i := range hist {
		hist[i] = message.Message{Role: message.RoleUser, Content: message.TextContent("x")}
	}
	// No provider supplied — should still trigger via the count fallback.
	assert.True(t, shouldCompress(hist, cfg, nil, ""))
}

// TestShouldCompress_TokenBudgetTrigger covers fix #2: when the message
// count is below the legacy threshold but a single oversized paste pushes
// estimated tokens past Threshold * ContextLength, compression must fire.
func TestShouldCompress_TokenBudgetTrigger(t *testing.T) {
	cfg := config.CompressionConfig{
		Enabled:     true,
		Threshold:   0.5,
		ProtectLast: 20, // count threshold = 60 messages, never reached here
	}
	p := &estimatorProvider{
		contextLen: 200_000,
		tokensFor: func(text string) int {
			// Trigger: a single 150K-token paste alone passes 0.5 * 200K = 100K.
			return len(text)
		},
	}
	giant := strings.Repeat("x", 150_000)
	hist := []message.Message{
		{Role: message.RoleUser, Content: message.TextContent("intro")},
		{Role: message.RoleAssistant, Content: message.TextContent("ack")},
		{Role: message.RoleUser, Content: message.TextContent(giant)},
	}

	assert.True(t, shouldCompress(hist, cfg, p, "qwen-plus"),
		"token budget exceeded — must trigger compression")
}

func TestShouldCompress_TokenBudgetUnderLimit(t *testing.T) {
	cfg := config.CompressionConfig{
		Enabled:     true,
		Threshold:   0.5,
		ProtectLast: 20,
	}
	p := &estimatorProvider{
		contextLen: 200_000,
		tokensFor:  func(text string) int { return len(text) },
	}
	hist := []message.Message{
		{Role: message.RoleUser, Content: message.TextContent("hi")},
		{Role: message.RoleAssistant, Content: message.TextContent("hello")},
	}

	assert.False(t, shouldCompress(hist, cfg, p, "qwen-plus"),
		"well under both triggers — must not compress")
}

func TestShouldCompress_NilProviderNoTokenTrigger(t *testing.T) {
	cfg := config.CompressionConfig{
		Enabled:     true,
		Threshold:   0.5,
		ProtectLast: 20,
	}
	hist := make([]message.Message, 5)
	for i := range hist {
		hist[i] = message.Message{Role: message.RoleUser, Content: message.TextContent(strings.Repeat("x", 1_000_000))}
	}
	// No provider — token trigger silently no-ops, count trigger doesn't fire.
	assert.False(t, shouldCompress(hist, cfg, nil, ""))
}
