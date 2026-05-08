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

// stubAuxProvider returns a canned text response from Complete(). Optionally
// estimates tokens via `estimator` (default: 0 for every call) so tests can
// drive the per-message-size pruning path.
type stubAuxProvider struct {
	response  string
	estimator func(text string) int
	calls     int
}

func (s *stubAuxProvider) Name() string                         { return "stub-aux" }
func (s *stubAuxProvider) Available() bool                      { return true }
func (s *stubAuxProvider) ModelInfo(string) *provider.ModelInfo { return nil }
func (s *stubAuxProvider) EstimateTokens(_ string, text string) (int, error) {
	if s.estimator == nil {
		return 0, nil
	}
	return s.estimator(text), nil
}
func (s *stubAuxProvider) Stream(ctx context.Context, req *provider.Request) (provider.Stream, error) {
	return nil, nil
}
func (s *stubAuxProvider) Complete(ctx context.Context, req *provider.Request) (*provider.Response, error) {
	s.calls++
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

// TestCompressorSummarizesOversizedTailMessage covers fix #3: even when the
// tail (protected from middle-summary) holds a single 200KB+ paste, the
// compressor must replace it with an aux summary so the request doesn't
// blow the model context window.
func TestCompressorSummarizesOversizedTailMessage(t *testing.T) {
	cfg := config.CompressionConfig{
		Enabled:             true,
		Threshold:           0.5,
		TargetRatio:         0.2,
		ProtectLast:         3,
		MaxPasses:           3,
		PerMessageMaxTokens: 100,
	}
	aux := &stubAuxProvider{
		response:  "summary of the giant paste",
		estimator: func(text string) int { return len(text) },
	}
	c := NewCompressor(cfg, aux)

	giant := strings.Repeat("x", 5000) // 5000 "tokens" by stub estimator, > 100
	history := []message.Message{
		{Role: message.RoleUser, Content: message.TextContent("head 1")},
		{Role: message.RoleAssistant, Content: message.TextContent("head 2")},
		{Role: message.RoleUser, Content: message.TextContent("head 3")},
		{Role: message.RoleAssistant, Content: message.TextContent("middle 1")},
		{Role: message.RoleUser, Content: message.TextContent("middle 2")},
		{Role: message.RoleAssistant, Content: message.TextContent("tail 1")},
		{Role: message.RoleUser, Content: message.TextContent(giant)}, // tail 2: oversized
		{Role: message.RoleAssistant, Content: message.TextContent("tail 3")},
	}

	compressed, err := c.Compress(context.Background(), history)
	require.NoError(t, err)

	// The giant paste must NOT survive verbatim.
	for _, m := range compressed {
		assert.NotEqual(t, giant, m.Content.Text(), "oversized message must be summarized away")
	}

	// At least one message is the single-message summary marker (distinct
	// from the middle-summary marker).
	foundOversizedSummary := false
	for _, m := range compressed {
		if strings.HasPrefix(m.Content.Text(), "[Summarized large message]") {
			foundOversizedSummary = true
			break
		}
	}
	assert.True(t, foundOversizedSummary, "expected a [Summarized large message] entry replacing the giant paste")
}

// TestCompressorRespectsDisabledPerMessageCap verifies that setting
// PerMessageMaxTokens to a negative value disables the per-message check
// (escape hatch for callers that explicitly want raw behaviour).
func TestCompressorRespectsDisabledPerMessageCap(t *testing.T) {
	cfg := config.CompressionConfig{
		Enabled:             true,
		ProtectLast:         3,
		PerMessageMaxTokens: -1, // disabled
	}
	aux := &stubAuxProvider{
		response:  "should not be called",
		estimator: func(text string) int { return len(text) },
	}
	c := NewCompressor(cfg, aux)

	giant := strings.Repeat("x", 5000)
	history := []message.Message{
		{Role: message.RoleUser, Content: message.TextContent("a")},
		{Role: message.RoleAssistant, Content: message.TextContent(giant)},
	}

	compressed, err := c.Compress(context.Background(), history)
	require.NoError(t, err)
	assert.Equal(t, history, compressed)
	assert.Equal(t, 0, aux.calls, "aux must not be called when per-message cap is disabled and history is short")
}
