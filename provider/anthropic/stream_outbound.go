// Package anthropic — outbound SSE state machine for the Anthropic
// Messages API streaming response shape.
package anthropic

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/odysseythink/hermind/provider"
	"github.com/odysseythink/pantheon/core"
	"github.com/odysseythink/pantheon/types"
)

// StreamOutbound consumes events from the provider stream and writes
// Anthropic-format SSE to w until the stream ends or errors.
// Context cancellation closes the upstream stream and returns nil
// (clean shutdown).
//
// keepAlive controls the idle ping interval. Values <= 0 default to 15s.
func StreamOutbound(
	ctx context.Context,
	w http.ResponseWriter,
	stream core.StreamResponse,
	requestModel, msgID string,
	keepAlive time.Duration,
) error {
	if keepAlive <= 0 {
		keepAlive = 15 * time.Second
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		return fmt.Errorf("anthropic: ResponseWriter does not support flushing")
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	state := &sseWriter{
		w:            w,
		flusher:      flusher,
		msgID:        msgID,
		model:        requestModel,
		curBlockIdx:  0,
		curBlockType: "",
		started:      false,
	}

	// Pump events from the stream into a channel so we can multiplex
	// against context cancellation and the keep-alive ticker.
	type recvResult struct {
		part *core.StreamPart
		err  error
		done bool
	}
	recvCh := make(chan recvResult, 1)
	go func() {
		for part, err := range stream {
			recvCh <- recvResult{part: part, err: err}
			if err != nil {
				return
			}
		}
		recvCh <- recvResult{done: true}
	}()

	ticker := time.NewTicker(keepAlive)
	defer ticker.Stop()

	var usage core.Usage
	var finishReason string

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			state.writePing()
		case r := <-recvCh:
			if r.err != nil {
				state.writeError(r.err)
				return nil
			}
			if r.done {
				state.finish(&provider.Response{
					FinishReason: finishReason,
					Usage:        usage,
				})
				return nil
			}
			part := r.part
			switch part.Type {
			case core.StreamPartTypeTextDelta:
				state.handleDelta(&provider.StreamDelta{Content: part.TextDelta})
			case core.StreamPartTypeToolCall:
				state.handleDelta(&provider.StreamDelta{
					ToolCalls: []types.ToolCall{{
						ID:   part.ToolCall.ID,
						Type: "function",
						Function: types.ToolCallFunction{
							Name:      part.ToolCall.Name,
							Arguments: part.ToolCall.Arguments,
						},
					}},
				})
			case core.StreamPartTypeUsage:
				if part.Usage != nil {
					usage.PromptTokens = part.Usage.PromptTokens
					usage.CompletionTokens = part.Usage.CompletionTokens
				}
			case core.StreamPartTypeFinish:
				finishReason = part.FinishReason
			}
			// Reset the keep-alive ticker on any real event.
			ticker.Reset(keepAlive)
		}
	}
}

type sseWriter struct {
	w            http.ResponseWriter
	flusher      http.Flusher
	msgID, model string

	curBlockIdx  int
	curBlockType string // "" | "text" | "tool_use"
	started      bool
}

func (s *sseWriter) writeEvent(eventName string, data any) {
	payload, _ := json.Marshal(data)
	fmt.Fprintf(s.w, "event: %s\n", eventName)
	fmt.Fprintf(s.w, "data: %s\n\n", payload)
	s.flusher.Flush()
}

func (s *sseWriter) ensureStarted() {
	if s.started {
		return
	}
	s.started = true
	s.writeEvent("message_start", map[string]any{
		"type": "message_start",
		"message": map[string]any{
			"id":            s.msgID,
			"type":          "message",
			"role":          "assistant",
			"content":       []any{},
			"model":         s.model,
			"stop_reason":   nil,
			"stop_sequence": nil,
			"usage": map[string]any{
				"input_tokens":  0,
				"output_tokens": 0,
			},
		},
	})
}

func (s *sseWriter) startBlock(blockType string, extra map[string]any) {
	cb := map[string]any{"type": blockType}
	if blockType == "text" {
		cb["text"] = ""
	}
	for k, v := range extra {
		cb[k] = v
	}
	s.writeEvent("content_block_start", map[string]any{
		"type":          "content_block_start",
		"index":         s.curBlockIdx,
		"content_block": cb,
	})
	s.curBlockType = blockType
}

func (s *sseWriter) stopBlock() {
	if s.curBlockType == "" {
		return
	}
	s.writeEvent("content_block_stop", map[string]any{
		"type":  "content_block_stop",
		"index": s.curBlockIdx,
	})
	s.curBlockIdx++
	s.curBlockType = ""
}

func (s *sseWriter) writeTextDelta(text string) {
	if s.curBlockType != "text" {
		s.stopBlock()
		s.startBlock("text", nil)
	}
	s.writeEvent("content_block_delta", map[string]any{
		"type":  "content_block_delta",
		"index": s.curBlockIdx,
		"delta": map[string]any{
			"type": "text_delta",
			"text": text,
		},
	})
}

func (s *sseWriter) writeToolUse(id, name, argsJSON string) {
	// Stop any open block (text or otherwise).
	s.stopBlock()
	s.startBlock("tool_use", map[string]any{
		"id":    id,
		"name":  name,
		"input": map[string]any{},
	})
	s.writeEvent("content_block_delta", map[string]any{
		"type":  "content_block_delta",
		"index": s.curBlockIdx,
		"delta": map[string]any{
			"type":         "input_json_delta",
			"partial_json": argsJSON,
		},
	})
	s.stopBlock()
}

func (s *sseWriter) handleDelta(d *provider.StreamDelta) {
	if d == nil {
		return
	}
	s.ensureStarted()
	if d.Content != "" {
		s.writeTextDelta(d.Content)
	}
	for _, tc := range d.ToolCalls {
		s.writeToolUse(tc.ID, tc.Function.Name, tc.Function.Arguments)
	}
	// d.Reasoning is intentionally ignored in v1.
}

func (s *sseWriter) finish(resp *provider.Response) {
	s.ensureStarted()
	s.stopBlock()
	usage := map[string]any{"output_tokens": 0}
	stopReason := "end_turn"
	if resp != nil {
		usage["output_tokens"] = resp.Usage.CompletionTokens
		stopReason = mapFinishReason(resp.FinishReason)
	}
	s.writeEvent("message_delta", map[string]any{
		"type": "message_delta",
		"delta": map[string]any{
			"stop_reason":   stopReason,
			"stop_sequence": nil,
		},
		"usage": usage,
	})
	s.writeEvent("message_stop", map[string]any{"type": "message_stop"})
}

func (s *sseWriter) writeError(err error) {
	s.ensureStarted()
	s.stopBlock()
	msg := "internal_error"
	if err != nil {
		msg = err.Error()
	}
	s.writeEvent("error", map[string]any{
		"type": "error",
		"error": map[string]any{
			"type":    "api_error",
			"message": msg,
		},
	})
}

func (s *sseWriter) writePing() {
	s.writeEvent("ping", map[string]any{"type": "ping"})
}
