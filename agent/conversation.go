// agent/conversation.go
package agent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/provider"
	"github.com/odysseythink/hermind/storage"
	"github.com/odysseythink/hermind/tool"
)

// RunConversation runs a conversation turn — or multiple turns if the LLM
// issues tool calls. Each LLM call is one turn. The loop continues until:
//
//	(1) the LLM responds without any tool_use blocks (final answer),
//	(2) the iteration budget is exhausted,
//	(3) the context is canceled,
//	(4) the provider returns a non-retryable error.
//
// Single-turn (no tools) behavior matches Plan 1 exactly.
func (e *Engine) RunConversation(ctx context.Context, opts *RunOptions) (*ConversationResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	model := opts.Model
	if model == "" {
		model = "claude-opus-4-6"
	}

	// Copy caller's history so we don't mutate it
	history := append([]message.Message{}, opts.History...)
	history = append(history, message.Message{
		Role:    message.RoleUser,
		Content: message.TextContent(opts.UserMessage),
	})
	if e.memory != nil {
		e.memory.ObserveTurn(history[len(history)-1])
	}

	var activeSkills []ActiveSkill
	if e.activeSkills != nil {
		activeSkills = e.activeSkills()
	}
	systemPrompt := e.prompt.Build(&PromptOptions{Model: model, ActiveSkills: activeSkills})

	// Persist the session + the incoming user message (if storage is configured).
	// effectivePrompt starts as the freshly built prompt; if storage is configured
	// we replace it with the stored (frozen at creation) prompt so later config
	// changes don't leak into long-running sessions.
	effectivePrompt := systemPrompt
	if e.storage != nil {
		sess, created, err := e.ensureSession(ctx, opts, systemPrompt, opts.UserMessage, model)
		if err != nil {
			return nil, fmt.Errorf("engine: ensure session: %w", err)
		}
		effectivePrompt = sess.SystemPrompt
		if err := e.persistMessage(ctx, opts.SessionID, &history[len(history)-1]); err != nil {
			return nil, fmt.Errorf("engine: persist user message: %w", err)
		}
		if created && e.onSessionCreated != nil {
			e.onSessionCreated(sess)
		}
	}

	// Collect tool definitions from the registry (empty slice if nil)
	var toolDefs []tool.ToolDefinition
	if e.tools != nil {
		toolDefs = e.tools.Definitions(nil)
	}

	budget := NewBudget(e.config.MaxTurns)
	totalUsage := message.Usage{}
	iterations := 0
	var lastResponse message.Message

	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if !budget.Consume() {
			// Budget exhausted — return what we have so far
			break
		}
		iterations++

		// Compression check: if enabled and history is long enough,
		// replace the middle of history with a summary.
		if e.compressor != nil && shouldCompress(history, e.config.Compression) {
			newHistory, err := e.compressor.Compress(ctx, history)
			if err != nil {
				// Don't fail the conversation on compression errors — log and continue.
				// Plan 6 keeps this simple; logging is deferred.
				_ = err
			} else {
				history = newHistory
			}
		}

		req := &provider.Request{
			Model:        model,
			SystemPrompt: effectivePrompt,
			Messages:     history,
			Tools:        toolDefs,
			MaxTokens:    4096,
		}

		resp, err := e.streamOnce(ctx, req)
		if err != nil {
			return nil, err
		}

		// Append assistant response to history
		history = append(history, resp.Message)
		lastResponse = resp.Message
		if e.memory != nil {
			e.memory.ObserveTurn(resp.Message)
		}
		totalUsage.InputTokens += resp.Usage.InputTokens
		totalUsage.OutputTokens += resp.Usage.OutputTokens
		totalUsage.CacheReadTokens += resp.Usage.CacheReadTokens
		totalUsage.CacheWriteTokens += resp.Usage.CacheWriteTokens

		// Persist the assistant message + usage atomically (if storage configured)
		if e.storage != nil {
			respCopy := resp // capture for closure
			txErr := e.storage.WithTx(ctx, func(tx storage.Tx) error {
				m := &history[len(history)-1]
				if err := e.persistMessageTx(ctx, tx, opts.SessionID, m); err != nil {
					return err
				}
				return tx.UpdateUsage(ctx, opts.SessionID, &storage.UsageUpdate{
					InputTokens:      respCopy.Usage.InputTokens,
					OutputTokens:     respCopy.Usage.OutputTokens,
					CacheReadTokens:  respCopy.Usage.CacheReadTokens,
					CacheWriteTokens: respCopy.Usage.CacheWriteTokens,
				})
			})
			if txErr != nil {
				return nil, fmt.Errorf("engine: persist response: %w", txErr)
			}
		}

		// Extract tool_use blocks from the response
		toolCalls := extractToolCalls(resp.Message.Content)
		if len(toolCalls) == 0 {
			// No tool calls → this is the final answer
			break
		}

		// Execute tool calls sequentially (Plan 5 adds parallelism)
		toolResults := e.executeToolCalls(ctx, toolCalls)

		// Append tool results as a user message with tool_result blocks
		toolResultMsg := message.Message{
			Role:    message.RoleUser,
			Content: message.BlockContent(toolResults),
		}
		history = append(history, toolResultMsg)

		if e.storage != nil {
			if err := e.persistMessage(ctx, opts.SessionID, &history[len(history)-1]); err != nil {
				return nil, fmt.Errorf("engine: persist tool result: %w", err)
			}
		}
	}

	return &ConversationResult{
		Response:   lastResponse,
		Messages:   history,
		SessionID:  opts.SessionID,
		Usage:      totalUsage,
		Iterations: iterations,
	}, nil
}

// streamOnce runs a single provider stream and collects the full response.
// Fires the onStreamDelta callback for each delta.
func (e *Engine) streamOnce(ctx context.Context, req *provider.Request) (*provider.Response, error) {
	if e.provider == nil {
		return nil, errors.New("engine: no LLM provider configured — set one under Settings → Models → Providers")
	}
	stream, err := e.provider.Stream(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("engine: start stream: %w", err)
	}
	defer stream.Close()

	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		ev, recvErr := stream.Recv()
		if recvErr != nil {
			if errors.Is(recvErr, io.EOF) {
				return nil, errors.New("engine: stream ended without a done event")
			}
			return nil, fmt.Errorf("engine: stream recv: %w", recvErr)
		}
		if ev == nil {
			continue
		}
		switch ev.Type {
		case provider.EventDelta:
			if e.onStreamDelta != nil && ev.Delta != nil {
				e.onStreamDelta(ev.Delta)
			}
		case provider.EventDone:
			if ev.Response == nil {
				return nil, errors.New("engine: done event has nil response")
			}
			return ev.Response, nil
		case provider.EventError:
			return nil, ev.Err
		}
	}
}

// extractToolCalls returns all tool_use blocks from a content union.
// If the content is plain text, returns nil.
func extractToolCalls(c message.Content) []message.ContentBlock {
	if c.IsText() {
		return nil
	}
	var calls []message.ContentBlock
	for _, b := range c.Blocks() {
		if b.Type == "tool_use" {
			calls = append(calls, b)
		}
	}
	return calls
}

// executeToolCalls dispatches each tool call through the registry and
// returns the results as tool_result content blocks.
// If the registry is nil, returns error results for every call.
func (e *Engine) executeToolCalls(ctx context.Context, calls []message.ContentBlock) []message.ContentBlock {
	results := make([]message.ContentBlock, 0, len(calls))
	for _, call := range calls {
		if e.onToolStart != nil {
			e.onToolStart(call)
		}

		var result string
		if e.tools == nil {
			result = `{"error":"no tool registry configured"}`
		} else {
			out, err := e.tools.Dispatch(ctx, call.ToolUseName, call.ToolUseInput)
			if err != nil {
				result = fmt.Sprintf(`{"error":"dispatch failed: %s"}`, err.Error())
			} else {
				result = out
			}
		}

		if e.onToolResult != nil {
			e.onToolResult(call, result)
		}

		results = append(results, message.ContentBlock{
			Type:       "tool_result",
			ToolUseID:  call.ToolUseID,
			ToolResult: result,
		})
	}
	return results
}

// ensureSession creates a new session row if it doesn't exist. On new rows,
// the stored system prompt is composed as `defaultPrompt + "\n\n" + firstMsg`
// (frozen at creation) and title is DeriveTitle(firstMsg). Returns the session
// (existing or freshly created) and a bool indicating whether this call
// created it.
func (e *Engine) ensureSession(
	ctx context.Context,
	opts *RunOptions,
	defaultPrompt, firstMsg, model string,
) (*storage.Session, bool, error) {
	if s, err := e.storage.GetSession(ctx, opts.SessionID); err == nil {
		return s, false, nil
	} else if !errors.Is(err, storage.ErrNotFound) {
		return nil, false, err
	}

	composed := defaultPrompt
	// Note: firstMsg intentionally no longer concatenated here.
	// It still feeds DeriveTitle below.
	s := &storage.Session{
		ID:           opts.SessionID,
		Source:       e.platform,
		UserID:       opts.UserID,
		Model:        model,
		SystemPrompt: composed,
		Title:        DeriveTitle(firstMsg),
		StartedAt:    time.Now().UTC(),
	}
	if err := e.storage.CreateSession(ctx, s); err != nil {
		return nil, false, err
	}
	return s, true, nil
}

// persistMessage writes a single message outside a transaction.
func (e *Engine) persistMessage(ctx context.Context, sessionID string, m *message.Message) error {
	stored, err := storedFromMessage(sessionID, m)
	if err != nil {
		return err
	}
	return e.storage.AddMessage(ctx, sessionID, stored)
}

// persistMessageTx writes a single message inside an existing transaction.
func (e *Engine) persistMessageTx(ctx context.Context, tx storage.Tx, sessionID string, m *message.Message) error {
	stored, err := storedFromMessage(sessionID, m)
	if err != nil {
		return err
	}
	return tx.AddMessage(ctx, sessionID, stored)
}

// shouldCompress decides whether the current history should be compressed.
// Plan 6 uses a simple count-based trigger: compress if len(history) exceeds
// (1/threshold) * protect_last. Future plans can add token-aware triggers.
func shouldCompress(history []message.Message, cfg config.CompressionConfig) bool {
	if !cfg.Enabled {
		return false
	}
	if cfg.ProtectLast <= 0 {
		return false
	}
	// Trigger compression when we have more than 3× protect_last messages.
	// This roughly corresponds to hitting 50-75% of typical context windows
	// after a long conversation with tool calls.
	return len(history) > 3*cfg.ProtectLast
}

// storedFromMessage converts a message.Message to a storage.StoredMessage.
func storedFromMessage(sessionID string, m *message.Message) (*storage.StoredMessage, error) {
	contentJSON, err := m.Content.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("engine: marshal content: %w", err)
	}
	return &storage.StoredMessage{
		SessionID:    sessionID,
		Role:         string(m.Role),
		Content:      string(contentJSON),
		ToolCallID:   m.ToolCallID,
		ToolName:     m.ToolName,
		Timestamp:    time.Now().UTC(),
		FinishReason: m.FinishReason,
		Reasoning:    m.Reasoning,
	}, nil
}
