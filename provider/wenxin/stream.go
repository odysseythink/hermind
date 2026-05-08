// provider/wenxin/stream.go
package wenxin

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/provider"
)

const wenxinSSEMaxLineBytes = 10 * 1024 * 1024

// Stream starts a streaming chat request to Wenxin.
func (w *Wenxin) Stream(ctx context.Context, req *provider.Request) (provider.Stream, error) {
	token, err := w.getAccessToken(ctx)
	if err != nil {
		return nil, &provider.Error{
			Kind:     provider.ErrAuth,
			Provider: "wenxin",
			Message:  err.Error(),
			Cause:    err,
		}
	}

	apiReq := w.buildRequest(req, true)
	body, err := json.Marshal(apiReq)
	if err != nil {
		return nil, fmt.Errorf("wenxin: marshal: %w", err)
	}

	url := fmt.Sprintf("%s/%s?access_token=%s", w.chatBaseURL, w.modelForURL(req), token)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("wenxin: request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := w.http.Do(httpReq)
	if err != nil {
		return nil, &provider.Error{
			Kind:     provider.ErrServerError,
			Provider: "wenxin",
			Message:  fmt.Sprintf("network: %v", err),
			Cause:    err,
		}
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), wenxinSSEMaxLineBytes)

	return &wenxinStream{
		resp:    resp,
		scanner: scanner,
		model:   w.modelForURL(req),
	}, nil
}

// wenxinStream implements provider.Stream.
type wenxinStream struct {
	resp    *http.Response
	scanner *bufio.Scanner
	text    strings.Builder
	usage   message.Usage
	model   string
	done    bool
	closed  bool
}

// Recv reads the next event. Wenxin uses "data: {...}\n\n" format,
// with `is_end: true` as the terminator.
func (s *wenxinStream) Recv() (*provider.StreamEvent, error) {
	if s.closed || s.done {
		return nil, io.EOF
	}

	for {
		if !s.scanner.Scan() {
			if err := s.scanner.Err(); err != nil {
				return nil, fmt.Errorf("wenxin stream: scan: %w", err)
			}
			s.done = true
			return s.buildDone(), nil
		}

		line := bytes.TrimRight(s.scanner.Bytes(), "\r")
		if len(line) == 0 {
			continue
		}
		if !bytes.HasPrefix(line, []byte("data:")) {
			continue
		}
		data := bytes.TrimSpace(line[len("data:"):])
		if len(data) == 0 {
			continue
		}

		var ev chatStreamEvent
		if err := json.Unmarshal(data, &ev); err != nil {
			return nil, fmt.Errorf("wenxin stream: parse: %w", err)
		}

		if ev.ErrorCode != 0 {
			return nil, mapErrorCode(ev.ErrorCode, ev.ErrorMsg)
		}

		if ev.Result != "" {
			s.text.WriteString(ev.Result)
		}
		if ev.Usage != nil {
			s.usage.InputTokens = ev.Usage.PromptTokens
			s.usage.OutputTokens = ev.Usage.CompletionTokens
		}

		if ev.IsEnd {
			s.done = true
			return s.buildDone(), nil
		}

		if ev.Result != "" {
			return &provider.StreamEvent{
				Type:  provider.EventDelta,
				Delta: &provider.StreamDelta{Content: ev.Result},
			}, nil
		}
	}
}

func (s *wenxinStream) buildDone() *provider.StreamEvent {
	return &provider.StreamEvent{
		Type: provider.EventDone,
		Response: &provider.Response{
			Message: message.Message{
				Role:    message.RoleAssistant,
				Content: message.TextContent(s.text.String()),
			},
			FinishReason: "stop",
			Usage:        s.usage,
			Model:        s.model,
		},
	}
}

// Close releases the HTTP response body.
func (s *wenxinStream) Close() error {
	if s.closed {
		return nil
	}
	s.closed = true
	if s.resp != nil && s.resp.Body != nil {
		return s.resp.Body.Close()
	}
	return nil
}
