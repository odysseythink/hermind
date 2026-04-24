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

// RunConversation runs one or more turns of the conversation until:
//
//	(1) the LLM responds without any tool_use blocks (final answer),
//	(2) the iteration budget is exhausted,
//	(3) the context is canceled,
//	(4) the provider returns a non-retryable error.
//
// For non-ephemeral runs the engine loads prior history from storage
// and persists each new message. For ephemeral runs (Ephemeral=true),
// history is read only from opts.History and nothing is persisted.
func (e *Engine) RunConversation(ctx context.Context, opts *RunOptions) (*ConversationResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	model := opts.Model

	var history []message.Message
	if opts.Ephemeral {
		history = append([]message.Message{}, opts.History...)
	} else if e.storage != nil {
		rows, err := e.storage.GetHistory(ctx, 0, 0)
		if err != nil {
			return nil, fmt.Errorf("engine: load history: %w", err)
		}
		for _, row := range rows {
			msg, err := messageFromStored(row)
			if err != nil {
				return nil, err
			}
			history = append(history, msg)
		}
	}

	userMsg := message.Message{
		Role:    message.RoleUser,
		Content: message.TextContent(opts.UserMessage),
	}
	history = append(history, userMsg)
	if e.memory != nil {
		e.memory.ObserveTurn(userMsg)
	}

	if !opts.Ephemeral && e.storage != nil {
		if err := e.persistMessage(ctx, &userMsg); err != nil {
			return nil, fmt.Errorf("engine: persist user message: %w", err)
		}
	}

	var activeSkills []ActiveSkill
	if e.activeSkills != nil {
		activeSkills = e.activeSkills()
	}
	systemPrompt := e.prompt.Build(&PromptOptions{Model: model, ActiveSkills: activeSkills})

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
			break
		}
		iterations++

		if !opts.Ephemeral && e.compressor != nil && shouldCompress(history, e.config.Compression) {
			if newHistory, err := e.compressor.Compress(ctx, history); err == nil {
				history = newHistory
			}
		}

		req := &provider.Request{
			Model:        model,
			SystemPrompt: systemPrompt,
			Messages:     history,
			Tools:        toolDefs,
			MaxTokens:    4096,
		}

		resp, err := e.streamOnce(ctx, req)
		if err != nil {
			return nil, err
		}

		history = append(history, resp.Message)
		lastResponse = resp.Message
		if e.memory != nil {
			e.memory.ObserveTurn(resp.Message)
		}
		totalUsage.InputTokens += resp.Usage.InputTokens
		totalUsage.OutputTokens += resp.Usage.OutputTokens
		totalUsage.CacheReadTokens += resp.Usage.CacheReadTokens
		totalUsage.CacheWriteTokens += resp.Usage.CacheWriteTokens

		if !opts.Ephemeral && e.storage != nil {
			respCopy := resp
			txErr := e.storage.WithTx(ctx, func(tx storage.Tx) error {
				m := &history[len(history)-1]
				if err := e.persistMessageTx(ctx, tx, m); err != nil {
					return err
				}
				return tx.UpdateUsage(ctx, &storage.UsageUpdate{
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

		toolCalls := extractToolCalls(resp.Message.Content)
		if len(toolCalls) == 0 {
			break
		}

		toolResults := e.executeToolCalls(ctx, toolCalls)
		toolResultMsg := message.Message{
			Role:    message.RoleUser,
			Content: message.BlockContent(toolResults),
		}
		history = append(history, toolResultMsg)

		if !opts.Ephemeral && e.storage != nil {
			if err := e.persistMessage(ctx, &toolResultMsg); err != nil {
				return nil, fmt.Errorf("engine: persist tool result: %w", err)
			}
		}
	}

	return &ConversationResult{
		Response:   lastResponse,
		Messages:   history,
		Usage:      totalUsage,
		Iterations: iterations,
	}, nil
}

// streamOnce runs a single provider stream and collects the full response.
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

func (e *Engine) persistMessage(ctx context.Context, m *message.Message) error {
	stored, err := storedFromMessage(m)
	if err != nil {
		return err
	}
	return e.storage.AppendMessage(ctx, stored)
}

func (e *Engine) persistMessageTx(ctx context.Context, tx storage.Tx, m *message.Message) error {
	stored, err := storedFromMessage(m)
	if err != nil {
		return err
	}
	return tx.AppendMessage(ctx, stored)
}

func shouldCompress(history []message.Message, cfg config.CompressionConfig) bool {
	if !cfg.Enabled {
		return false
	}
	if cfg.ProtectLast <= 0 {
		return false
	}
	return len(history) > 3*cfg.ProtectLast
}

func storedFromMessage(m *message.Message) (*storage.StoredMessage, error) {
	contentJSON, err := m.Content.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("engine: marshal content: %w", err)
	}
	return &storage.StoredMessage{
		Role:         string(m.Role),
		Content:      string(contentJSON),
		ToolCallID:   m.ToolCallID,
		ToolName:     m.ToolName,
		Timestamp:    time.Now().UTC(),
		FinishReason: m.FinishReason,
		Reasoning:    m.Reasoning,
	}, nil
}

// messageFromStored rebuilds a message.Message from a StoredMessage
// pulled out of storage.GetHistory.
func messageFromStored(row *storage.StoredMessage) (message.Message, error) {
	var content message.Content
	if err := content.UnmarshalJSON([]byte(row.Content)); err != nil {
		return message.Message{}, fmt.Errorf("engine: decode stored content: %w", err)
	}
	return message.Message{
		Role:         message.Role(row.Role),
		Content:      content,
		ToolCallID:   row.ToolCallID,
		ToolName:     row.ToolName,
		FinishReason: row.FinishReason,
		Reasoning:    row.Reasoning,
	}, nil
}
