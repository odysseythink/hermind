package anthropic

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/odysseythink/pantheon/core"
)

func TestStreamOutbound_TextOnly(t *testing.T) {
	stream := func(yield func(*core.StreamPart, error) bool) {
		if !yield(&core.StreamPart{Type: core.StreamPartTypeTextDelta, TextDelta: "hel"}, nil) {
			return
		}
		if !yield(&core.StreamPart{Type: core.StreamPartTypeTextDelta, TextDelta: "lo"}, nil) {
			return
		}
		if !yield(&core.StreamPart{Type: core.StreamPartTypeUsage, Usage: &core.Usage{PromptTokens: 5, CompletionTokens: 2}}, nil) {
			return
		}
		if !yield(&core.StreamPart{Type: core.StreamPartTypeFinish, FinishReason: "stop"}, nil) {
			return
		}
	}
	rr := httptest.NewRecorder()
	err := StreamOutbound(context.Background(), rr, stream, "claude-sonnet-4-6", "msg_x", time.Hour)
	require.NoError(t, err)

	body := rr.Body.String()
	// Verify the event sequence in order
	require.True(t, strings.Index(body, "event: message_start") >= 0)
	require.True(t, strings.Index(body, "event: content_block_start") >= 0)
	require.True(t, strings.Contains(body, `"type":"text"`))
	require.True(t, strings.Contains(body, `"text":"hel"`))
	require.True(t, strings.Contains(body, `"text":"lo"`))
	require.True(t, strings.Index(body, "event: content_block_stop") >= 0)
	require.True(t, strings.Index(body, "event: message_delta") >= 0)
	require.True(t, strings.Contains(body, `"stop_reason":"end_turn"`))
	require.True(t, strings.Contains(body, `"output_tokens":2`))

	// Sanity check: the order of events matches the wire spec.
	startIdx := strings.Index(body, "message_start")
	cbStart := strings.Index(body, "content_block_start")
	cbStop := strings.Index(body, "content_block_stop")
	mDelta := strings.Index(body, "message_delta")
	mStop := strings.Index(body, "message_stop")
	require.True(t, startIdx < cbStart && cbStart < cbStop && cbStop < mDelta && mDelta < mStop,
		"events must be in the correct order; got: %s", body)
}

func TestStreamOutbound_TextThenToolUse(t *testing.T) {
	stream := func(yield func(*core.StreamPart, error) bool) {
		if !yield(&core.StreamPart{Type: core.StreamPartTypeTextDelta, TextDelta: "calling"}, nil) {
			return
		}
		if !yield(&core.StreamPart{Type: core.StreamPartTypeToolCall, ToolCall: &core.ToolCallPart{
			ID: "toolu_1", Name: "get_weather", Arguments: `{"city":"SF"}`,
		}}, nil) {
			return
		}
		if !yield(&core.StreamPart{Type: core.StreamPartTypeUsage, Usage: &core.Usage{PromptTokens: 8, CompletionTokens: 4}}, nil) {
			return
		}
		if !yield(&core.StreamPart{Type: core.StreamPartTypeFinish, FinishReason: "tool_calls"}, nil) {
			return
		}
	}
	rr := httptest.NewRecorder()
	err := StreamOutbound(context.Background(), rr, stream, "x", "msg_y", time.Hour)
	require.NoError(t, err)

	body := rr.Body.String()
	// Block 0 = text, block 1 = tool_use
	require.True(t, strings.Contains(body, `"index":0`))
	require.True(t, strings.Contains(body, `"index":1`))
	require.True(t, strings.Contains(body, `"type":"tool_use"`))
	require.True(t, strings.Contains(body, `"id":"toolu_1"`))
	require.True(t, strings.Contains(body, `"name":"get_weather"`))
	// The tool_use input is emitted via a single input_json_delta that
	// contains the entire JSON arguments string.
	require.True(t, strings.Contains(body, `"input_json_delta"`))
	// partial_json is a JSON-encoded STRING per Anthropic spec; the
	// embedded JSON's quotes appear escaped in the wire bytes.
	require.True(t, strings.Contains(body, `"partial_json":"{\"city\":\"SF\"}"`),
		"partial_json must be sent as a JSON-encoded string, not an object; got: %s", body)
	require.True(t, strings.Contains(body, `"stop_reason":"tool_use"`))
}

func TestStreamOutbound_ErrorEvent(t *testing.T) {
	stream := func(yield func(*core.StreamPart, error) bool) {
		if !yield(&core.StreamPart{Type: core.StreamPartTypeTextDelta, TextDelta: "partial"}, nil) {
			return
		}
		yield(nil, errors.New("upstream blew up"))
	}
	rr := httptest.NewRecorder()
	err := StreamOutbound(context.Background(), rr, stream, "x", "msg_z", time.Hour)
	require.NoError(t, err, "stream errors are written as SSE events, not propagated")

	body := rr.Body.String()
	require.True(t, strings.Contains(body, "event: error"))
	require.True(t, strings.Contains(body, "upstream blew up"))
}

func TestStreamOutbound_KeepAlivePings(t *testing.T) {
	// Use a stream that delays via a goroutine and a short keep-alive.
	stream := newDelayedFakeStream(50 * time.Millisecond)
	rr := httptest.NewRecorder()
	err := StreamOutbound(context.Background(), rr, stream, "x", "msg_p", 20*time.Millisecond)
	require.NoError(t, err)

	// Count ping events; should be at least 1.
	count := strings.Count(rr.Body.String(), "event: ping")
	require.GreaterOrEqual(t, count, 1)
}

// delayedFakeStream emits a single text delta then finishes after `delay`.
func newDelayedFakeStream(delay time.Duration) core.StreamResponse {
	return func(yield func(*core.StreamPart, error) bool) {
		time.Sleep(delay)
		if !yield(&core.StreamPart{Type: core.StreamPartTypeTextDelta, TextDelta: "x"}, nil) {
			return
		}
	}
}

// Quick parser for SSE event names used by other tests.
func parseSSEEvents(body string) []string {
	scanner := bufio.NewScanner(bytes.NewBufferString(body))
	var names []string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "event: ") {
			names = append(names, strings.TrimPrefix(line, "event: "))
		}
	}
	return names
}
