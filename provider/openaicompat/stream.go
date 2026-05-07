// provider/openaicompat/stream.go
package openaicompat

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/provider"
)

// sseMaxLineBytes is the maximum line length for SSE parsing. Default
// bufio.Scanner limit is 64KB which corrupts large streams.
const sseMaxLineBytes = 10 * 1024 * 1024

// Stream starts a streaming chat completion request.
func (c *Client) Stream(ctx context.Context, req *provider.Request) (provider.Stream, error) {
	apiReq := c.buildRequest(req, true)
	body, err := json.Marshal(apiReq)
	if err != nil {
		return nil, fmt.Errorf("%s: marshal request: %w", c.cfg.ProviderName, err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST",
		c.cfg.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("%s: create request: %w", c.cfg.ProviderName, err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	httpReq.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	for k, v := range c.cfg.ExtraHeaders {
		httpReq.Header.Set(k, v)
	}

	log.Printf("[%s] Sending HTTP POST to %s", c.cfg.ProviderName, c.cfg.BaseURL+"/chat/completions")
	httpResp, err := c.http.Do(httpReq)
	if err != nil {
		log.Printf("[%s] HTTP request failed: %v", c.cfg.ProviderName, err)
		return nil, &provider.Error{
			Kind:     provider.ErrServerError,
			Provider: c.cfg.ProviderName,
			Message:  fmt.Sprintf("network error: %v", err),
			Cause:    err,
		}
	}
	log.Printf("[%s] HTTP response status: %d", c.cfg.ProviderName, httpResp.StatusCode)
	if httpResp.StatusCode != http.StatusOK {
		err := mapHTTPError(c.cfg.ProviderName, httpResp)
		_ = httpResp.Body.Close()
		log.Printf("[%s] HTTP error: %v", c.cfg.ProviderName, err)
		return nil, err
	}

	scanner := bufio.NewScanner(httpResp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), sseMaxLineBytes)
	scanner.Split(splitSSEEvents)
	log.Printf("[%s] SSE stream started", c.cfg.ProviderName)

	return &openaiStream{
		providerName: c.cfg.ProviderName,
		resp:         httpResp,
		scanner:      scanner,
		toolCalls:    make(map[int]*toolCallBuilder),
	}, nil
}

// splitSSEEvents is a bufio.SplitFunc that yields one SSE line at a time.
// OpenAI's SSE format uses "data: <json>\n" per event and "data: [DONE]" as
// the terminator. Unlike Anthropic, there is no "event: name" line.
func splitSSEEvents(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if idx := bytes.IndexByte(data, '\n'); idx >= 0 {
		return idx + 1, data[:idx], nil
	}
	if atEOF {
		return len(data), data, nil
	}
	return 0, nil, nil
}

// openaiStream implements provider.Stream.
// NOT thread-safe. One consumer only.
type openaiStream struct {
	providerName string
	resp         *http.Response
	scanner      *bufio.Scanner

	// Accumulated state
	text         strings.Builder
	model        string
	finishReason string
	usage        message.Usage
	done         bool
	closed       bool

	// Tool call accumulator keyed by index (OpenAI streams arguments as
	// concatenated string fragments per index).
	toolCalls map[int]*toolCallBuilder
	toolOrder []int // track order indices were first seen
}

// toolCallBuilder accumulates a streaming tool call across many chunks.
type toolCallBuilder struct {
	ID         string
	Name       string
	ArgBuilder strings.Builder // JSON argument string fragments
}

// Recv reads the next SSE line and returns the corresponding StreamEvent.
// OpenAI may interleave text deltas with tool call deltas on the same choice.
func (s *openaiStream) Recv() (*provider.StreamEvent, error) {
	if s.closed {
		return nil, io.EOF
	}
	if s.done {
		return nil, io.EOF
	}

	for {
		if !s.scanner.Scan() {
			if err := s.scanner.Err(); err != nil {
				log.Printf("[%s] SSE scan error: %v", s.providerName, err)
				return nil, fmt.Errorf("%s stream: scan: %w", s.providerName, err)
			}
			// Clean EOF with no explicit [DONE] terminator — synthesize a Done event.
			log.Printf("[%s] SSE EOF without [DONE]", s.providerName)
			s.done = true
			return s.buildDoneEvent(), nil
		}

		line := bytes.TrimRight(s.scanner.Bytes(), "\r")
		if len(line) == 0 {
			continue // SSE keepalive blank line
		}

		// Ignore non-data lines
		if !bytes.HasPrefix(line, []byte("data:")) {
			continue
		}

		data := bytes.TrimSpace(line[len("data:"):])
		if len(data) == 0 {
			continue
		}

		// Terminator
		if bytes.Equal(data, []byte("[DONE]")) {
			log.Printf("[%s] SSE [DONE] received", s.providerName)
			s.done = true
			return s.buildDoneEvent(), nil
		}

		// Parse the chunk
		var chunk chatStreamChunk
		if err := json.Unmarshal(data, &chunk); err != nil {
			log.Printf("[%s] SSE chunk parse error: %v | data=%s", s.providerName, err, string(data))
			return nil, fmt.Errorf("%s stream: parse chunk: %w", s.providerName, err)
		}

		ev, err := s.handleChunk(&chunk)
		if err != nil {
			return nil, err
		}
		if ev != nil {
			log.Printf("[%s] SSE event: type=%d content=%q", s.providerName, ev.Type, ev.Delta.Content)
			return ev, nil
		}
		// keep scanning
	}
}

// handleChunk processes one chatStreamChunk and returns a StreamEvent if
// the chunk carried visible text. Tool call fragments and usage info are
// accumulated silently.
func (s *openaiStream) handleChunk(chunk *chatStreamChunk) (*provider.StreamEvent, error) {
	if chunk.Model != "" {
		s.model = chunk.Model
	}
	if chunk.Usage != nil {
		s.usage.InputTokens = chunk.Usage.PromptTokens
		s.usage.OutputTokens = chunk.Usage.CompletionTokens
	}

	if len(chunk.Choices) == 0 {
		return nil, nil
	}
	choice := chunk.Choices[0]

	if choice.FinishReason != "" {
		s.finishReason = choice.FinishReason
	}

	// Text delta
	if choice.Delta.Content != "" {
		s.text.WriteString(choice.Delta.Content)
		return &provider.StreamEvent{
			Type: provider.EventDelta,
			Delta: &provider.StreamDelta{
				Content: choice.Delta.Content,
			},
		}, nil
	}

	// Tool call deltas
	for _, tc := range choice.Delta.ToolCalls {
		b, ok := s.toolCalls[tc.Index]
		if !ok {
			b = &toolCallBuilder{}
			s.toolCalls[tc.Index] = b
			s.toolOrder = append(s.toolOrder, tc.Index)
		}
		if tc.ID != "" {
			b.ID = tc.ID
		}
		if tc.Function != nil {
			if tc.Function.Name != "" {
				b.Name = tc.Function.Name
			}
			if tc.Function.Arguments != "" {
				b.ArgBuilder.WriteString(tc.Function.Arguments)
			}
		}
	}

	return nil, nil
}

// buildDoneEvent emits the terminal EventDone with accumulated state.
func (s *openaiStream) buildDoneEvent() *provider.StreamEvent {
	var content message.Content

	if len(s.toolCalls) > 0 {
		log.Printf("[%s] stream done: %d tool calls accumulated", s.providerName, len(s.toolCalls))
		blocks := make([]message.ContentBlock, 0, 1+len(s.toolCalls))
		if s.text.Len() > 0 {
			blocks = append(blocks, message.ContentBlock{
				Type: "text",
				Text: s.text.String(),
			})
		}
		for _, idx := range s.toolOrder {
			b := s.toolCalls[idx]
			if b == nil || b.ID == "" {
				continue
			}
			args := b.ArgBuilder.String()
			if args == "" {
				args = "{}"
			}
			log.Printf("[%s]   tool_call: id=%s name=%s args=%s", s.providerName, b.ID, b.Name, args)
			blocks = append(blocks, message.ContentBlock{
				Type:         "tool_use",
				ToolUseID:    b.ID,
				ToolUseName:  b.Name,
				ToolUseInput: []byte(args),
			})
		}
		content = message.BlockContent(blocks)
	} else {
		log.Printf("[%s] stream done: no tool calls", s.providerName)
		content = message.TextContent(s.text.String())
	}

	return &provider.StreamEvent{
		Type: provider.EventDone,
		Response: &provider.Response{
			Message: message.Message{
				Role:    message.RoleAssistant,
				Content: content,
			},
			FinishReason: s.finishReason,
			Usage:        s.usage,
			Model:        s.model,
		},
	}
}

// Close releases the underlying HTTP response.
func (s *openaiStream) Close() error {
	if s.closed {
		return nil
	}
	s.closed = true
	if s.resp != nil && s.resp.Body != nil {
		return s.resp.Body.Close()
	}
	return nil
}
