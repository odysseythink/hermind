// agent/compression_test.go
package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubAuxProvider returns a canned text response from Complete().
type stubAuxProvider struct {
	response string
}

func (s *stubAuxProvider) Name() string                                { return "stub-aux" }
func (s *stubAuxProvider) Available() bool                             { return true }
func (s *stubAuxProvider) ModelInfo(string) *provider.ModelInfo        { return nil }
func (s *stubAuxProvider) EstimateTokens(string, string) (int, error)  { return 0, nil }
func (s *stubAuxProvider) Stream(ctx context.Context, req *provider.Request) (provider.Stream, error) {
	return nil, nil
}
func (s *stubAuxProvider) Complete(ctx context.Context, req *provider.Request) (*provider.Response, error) {
	return &provider.Response{
		Message: message.Message{
			Role:    message.RoleAssistant,
			Content: message.TextContent(s.response),
		},
		FinishReason: "stop",
		Usage:        message.Usage{InputTokens: 100, OutputTokens: 30},
	}, nil
}

func TestCompressorPreservesProtectedMessages(t *testing.T) {
	cfg := config.CompressionConfig{
		Enabled:     true,
		Threshold:   0.5,
		TargetRatio: 0.2,
		ProtectLast: 3,
		MaxPasses:   3,
	}
	aux := &stubAuxProvider{response: "Summary of earlier messages."}
	c := NewCompressor(cfg, aux)

	history := []message.Message{
		{Role: message.RoleUser, Content: message.TextContent("msg 1")},
		{Role: message.RoleAssistant, Content: message.TextContent("msg 2")},
		{Role: message.RoleUser, Content: message.TextContent("msg 3")},
		{Role: message.RoleAssistant, Content: message.TextContent("msg 4")},
		{Role: message.RoleUser, Content: message.TextContent("msg 5")},
		{Role: message.RoleAssistant, Content: message.TextContent("msg 6")},
		{Role: message.RoleUser, Content: message.TextContent("msg 7")},
		{Role: message.RoleAssistant, Content: message.TextContent("msg 8")},
	}

	compressed, err := c.Compress(context.Background(), history)
	require.NoError(t, err)

	// Expected: first 3 (preserved head) + summary + last 3 (protected tail) = 7 messages
	// Actually: preserved head is the first user message pair, then summary, then protect_last
	// The plan says: preserve first 3 + last ProtectLast
	// With 8 messages and ProtectLast=3: head=first 3, tail=last 3, middle=2 → 3 + 1 + 3 = 7
	assert.LessOrEqual(t, len(compressed), len(history))
	assert.Greater(t, len(compressed), 0)

	// Last 3 messages should match the original tail
	tail := compressed[len(compressed)-3:]
	assert.Equal(t, "msg 6", tail[0].Content.Text())
	assert.Equal(t, "msg 7", tail[1].Content.Text())
	assert.Equal(t, "msg 8", tail[2].Content.Text())

	// There should be at least one summary message
	foundSummary := false
	for _, m := range compressed {
		if strings.Contains(m.Content.Text(), "Summary of earlier") {
			foundSummary = true
			break
		}
	}
	assert.True(t, foundSummary, "expected compressed history to contain a summary message")
}

func TestCompressorSkipsShortHistory(t *testing.T) {
	cfg := config.CompressionConfig{
		Enabled:     true,
		Threshold:   0.5,
		TargetRatio: 0.2,
		ProtectLast: 10, // more than history length
		MaxPasses:   3,
	}
	aux := &stubAuxProvider{response: "irrelevant"}
	c := NewCompressor(cfg, aux)

	history := []message.Message{
		{Role: message.RoleUser, Content: message.TextContent("msg 1")},
		{Role: message.RoleAssistant, Content: message.TextContent("msg 2")},
	}

	compressed, err := c.Compress(context.Background(), history)
	require.NoError(t, err)
	// Too short to compress — returned unchanged
	assert.Equal(t, history, compressed)
}

func TestCompressorDisabledReturnsUnchanged(t *testing.T) {
	cfg := config.CompressionConfig{Enabled: false}
	aux := &stubAuxProvider{}
	c := NewCompressor(cfg, aux)

	history := make([]message.Message, 100)
	for i := range history {
		history[i] = message.Message{Role: message.RoleUser, Content: message.TextContent("msg")}
	}

	compressed, err := c.Compress(context.Background(), history)
	require.NoError(t, err)
	assert.Equal(t, history, compressed)
}
