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

	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/provider"
)

// fakeStream implements provider.Stream against a queued slice.
type fakeStream struct {
	events []provider.StreamEvent
	idx    int
	closed bool
}

func (f *fakeStream) Recv() (*provider.StreamEvent, error) {
	if f.idx >= len(f.events) {
		return nil, errors.New("fakeStream: exhausted")
	}
	ev := f.events[f.idx]
	f.idx++
	return &ev, nil
}

func (f *fakeStream) Close() error { f.closed = true; return nil }

func TestStreamOutbound_TextOnly(t *testing.T) {
	stream := &fakeStream{events: []provider.StreamEvent{
		{Type: provider.EventDelta, Delta: &provider.StreamDelta{Content: "hel"}},
		{Type: provider.EventDelta, Delta: &provider.StreamDelta{Content: "lo"}},
		{Type: provider.EventDone, Response: &provider.Response{
			FinishReason: "stop",
			Usage:        message.Usage{InputTokens: 5, OutputTokens: 2},
		}},
	}}
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
	stream := &fakeStream{events: []provider.StreamEvent{
		{Type: provider.EventDelta, Delta: &provider.StreamDelta{Content: "calling"}},
		{Type: provider.EventDelta, Delta: &provider.StreamDelta{ToolCalls: []message.ToolCall{{
			ID: "toolu_1", Type: "function",
			Function: message.ToolCallFunction{Name: "get_weather", Arguments: `{"city":"SF"}`},
		}}}},
		{Type: provider.EventDone, Response: &provider.Response{
			FinishReason: "tool_calls",
			Usage:        message.Usage{InputTokens: 8, OutputTokens: 4},
		}},
	}}
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
	require.True(t, strings.Contains(body, `"city":"SF"`))
	require.True(t, strings.Contains(body, `"stop_reason":"tool_use"`))
}

func TestStreamOutbound_ErrorEvent(t *testing.T) {
	stream := &fakeStream{events: []provider.StreamEvent{
		{Type: provider.EventDelta, Delta: &provider.StreamDelta{Content: "partial"}},
		{Type: provider.EventError, Err: errors.New("upstream blew up")},
	}}
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

// delayedFakeStream emits a single text delta then EventDone after `delay`.
type delayedFakeStream struct {
	delay time.Duration
	step  int
}

func newDelayedFakeStream(delay time.Duration) *delayedFakeStream {
	return &delayedFakeStream{delay: delay}
}

func (d *delayedFakeStream) Recv() (*provider.StreamEvent, error) {
	switch d.step {
	case 0:
		d.step++
		// Block to simulate a slow upstream so the keep-alive timer fires.
		time.Sleep(d.delay)
		return &provider.StreamEvent{Type: provider.EventDelta, Delta: &provider.StreamDelta{Content: "x"}}, nil
	case 1:
		d.step++
		return &provider.StreamEvent{
			Type:     provider.EventDone,
			Response: &provider.Response{FinishReason: "stop"},
		}, nil
	default:
		return nil, errors.New("delayedFakeStream: exhausted")
	}
}

func (d *delayedFakeStream) Close() error { return nil }

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
