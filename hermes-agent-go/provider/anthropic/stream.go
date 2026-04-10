// provider/anthropic/stream.go
package anthropic

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/nousresearch/hermes-agent/message"
	"github.com/nousresearch/hermes-agent/provider"
)

// SSE scanner buffer size: 10MB is enough for any realistic LLM streaming chunk.
// Default bufio.Scanner limit is 64KB which corrupts large tool-call streams.
const sseMaxLineBytes = 10 * 1024 * 1024

// streamEvent names used by Anthropic SSE.
const (
	anthropicEventMessageStart      = "message_start"
	anthropicEventContentBlockStart = "content_block_start"
	anthropicEventContentBlockDelta = "content_block_delta"
	anthropicEventContentBlockStop  = "content_block_stop"
	anthropicEventMessageDelta      = "message_delta"
	anthropicEventMessageStop       = "message_stop"
	anthropicEventPing              = "ping"
	anthropicEventError             = "error"
)

// Stream starts a streaming request to /v1/messages.
func (a *Anthropic) Stream(ctx context.Context, req *provider.Request) (provider.Stream, error) {
	apiReq := a.buildRequest(req, true)
	body, err := json.Marshal(apiReq)
	if err != nil {
		return nil, fmt.Errorf("anthropic: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST",
		a.baseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("anthropic: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	httpReq.Header.Set("x-api-key", a.apiKey)
	httpReq.Header.Set("anthropic-version", defaultAPIVersion)

	httpResp, err := a.client.Do(httpReq)
	if err != nil {
		return nil, &provider.Error{
			Kind:     provider.ErrServerError,
			Provider: "anthropic",
			Message:  fmt.Sprintf("network error: %v", err),
			Cause:    err,
		}
	}
	if httpResp.StatusCode != http.StatusOK {
		err := mapHTTPError(httpResp)
		_ = httpResp.Body.Close()
		return nil, err
	}

	scanner := bufio.NewScanner(httpResp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), sseMaxLineBytes)
	// Custom split: SSE uses "\n\n" to separate events
	scanner.Split(splitSSEEvents)

	return &anthropicStream{
		resp:    httpResp,
		scanner: scanner,
		usage:   message.Usage{},
	}, nil
}

// splitSSEEvents is a bufio.SplitFunc that yields complete SSE events.
// An event is terminated by a blank line (\n\n).
func splitSSEEvents(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if idx := bytes.Index(data, []byte("\n\n")); idx >= 0 {
		return idx + 2, data[:idx], nil
	}
	// Also accept "\r\n\r\n" terminators
	if idx := bytes.Index(data, []byte("\r\n\r\n")); idx >= 0 {
		return idx + 4, data[:idx], nil
	}
	if atEOF {
		return len(data), data, nil
	}
	return 0, nil, nil
}

// anthropicStream implements provider.Stream.
// NOT thread-safe. One consumer only.
type anthropicStream struct {
	resp    *http.Response
	scanner *bufio.Scanner
	// accumulated state
	text         strings.Builder
	model        string
	finishReason string
	usage        message.Usage
	done         bool
	closed       bool
}

// Recv reads the next SSE event and returns a StreamEvent.
func (s *anthropicStream) Recv() (*provider.StreamEvent, error) {
	if s.closed {
		return nil, io.EOF
	}
	if s.done {
		return nil, io.EOF
	}
	for {
		if !s.scanner.Scan() {
			if err := s.scanner.Err(); err != nil {
				return nil, fmt.Errorf("anthropic stream: scan: %w", err)
			}
			// Clean EOF — synthesize a Done event
			s.done = true
			return s.buildDoneEvent(), nil
		}
		eventType, data := parseSSEEvent(s.scanner.Bytes())
		if eventType == "" {
			continue
		}
		ev, err := s.handleEvent(eventType, data)
		if err != nil {
			return nil, err
		}
		if ev != nil {
			return ev, nil
		}
		// Keep scanning if the event produced no output (e.g., ping)
	}
}

// parseSSEEvent extracts "event:" and "data:" fields from an SSE frame.
func parseSSEEvent(frame []byte) (eventType string, data []byte) {
	for _, line := range bytes.Split(frame, []byte("\n")) {
		line = bytes.TrimRight(line, "\r")
		if bytes.HasPrefix(line, []byte("event:")) {
			eventType = string(bytes.TrimSpace(line[len("event:"):]))
		} else if bytes.HasPrefix(line, []byte("data:")) {
			data = bytes.TrimSpace(line[len("data:"):])
		}
	}
	return eventType, data
}

// handleEvent dispatches an SSE event to the right handler based on its type.
// Returns a non-nil StreamEvent to surface to the caller, or nil to continue scanning.
func (s *anthropicStream) handleEvent(eventType string, data []byte) (*provider.StreamEvent, error) {
	switch eventType {
	case anthropicEventMessageStart:
		var ev struct {
			Message messagesResponse `json:"message"`
		}
		if err := json.Unmarshal(data, &ev); err != nil {
			return nil, fmt.Errorf("anthropic stream: parse message_start: %w", err)
		}
		s.model = ev.Message.Model
		s.usage.InputTokens = ev.Message.Usage.InputTokens
		return nil, nil
	case anthropicEventContentBlockDelta:
		var ev struct {
			Delta struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"delta"`
		}
		if err := json.Unmarshal(data, &ev); err != nil {
			return nil, fmt.Errorf("anthropic stream: parse delta: %w", err)
		}
		if ev.Delta.Type == "text_delta" && ev.Delta.Text != "" {
			s.text.WriteString(ev.Delta.Text)
			return &provider.StreamEvent{
				Type: provider.EventDelta,
				Delta: &provider.StreamDelta{
					Content: ev.Delta.Text,
				},
			}, nil
		}
		return nil, nil
	case anthropicEventMessageDelta:
		var ev struct {
			Delta struct {
				StopReason string `json:"stop_reason"`
			} `json:"delta"`
			Usage apiUsage `json:"usage"`
		}
		if err := json.Unmarshal(data, &ev); err != nil {
			return nil, fmt.Errorf("anthropic stream: parse message_delta: %w", err)
		}
		if ev.Delta.StopReason != "" {
			s.finishReason = ev.Delta.StopReason
		}
		s.usage.OutputTokens = ev.Usage.OutputTokens
		return nil, nil
	case anthropicEventMessageStop:
		s.done = true
		return s.buildDoneEvent(), nil
	case anthropicEventError:
		return nil, &provider.Error{
			Kind:     provider.ErrUnknown,
			Provider: "anthropic",
			Message:  "stream error event: " + string(data),
		}
	case anthropicEventPing, anthropicEventContentBlockStart, anthropicEventContentBlockStop:
		// Ignore these — they carry no output for our purposes
		return nil, nil
	default:
		return nil, nil
	}
}

// buildDoneEvent creates the terminal EventDone with accumulated state.
func (s *anthropicStream) buildDoneEvent() *provider.StreamEvent {
	return &provider.StreamEvent{
		Type: provider.EventDone,
		Response: &provider.Response{
			Message: message.Message{
				Role:    message.RoleAssistant,
				Content: message.TextContent(s.text.String()),
			},
			FinishReason: s.finishReason,
			Usage:        s.usage,
			Model:        s.model,
		},
	}
}

// Close releases the underlying HTTP connection.
func (s *anthropicStream) Close() error {
	if s.closed {
		return nil
	}
	s.closed = true
	if s.resp != nil && s.resp.Body != nil {
		return s.resp.Body.Close()
	}
	return nil
}

// Compile-time assertion that *Anthropic satisfies provider.Provider.
var _ provider.Provider = (*Anthropic)(nil)
