// agent/conversation.go
package agent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/nousresearch/hermes-agent/message"
	"github.com/nousresearch/hermes-agent/provider"
	"github.com/nousresearch/hermes-agent/storage"
)

// RunConversation executes a single conversation turn: sends the user
// message to the LLM, collects the streaming response, persists messages,
// and returns the assistant's reply.
//
// For Plan 1, this is single-turn only — no tools, no loop. Tool use is
// added in Plan 2, which turns this into the full iteration loop.
func (e *Engine) RunConversation(ctx context.Context, opts *RunOptions) (*ConversationResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	model := opts.Model
	if model == "" {
		model = "claude-opus-4-6"
	}

	// Build request
	history := append([]message.Message{}, opts.History...)
	history = append(history, message.Message{
		Role:    message.RoleUser,
		Content: message.TextContent(opts.UserMessage),
	})

	systemPrompt := e.prompt.Build(&PromptOptions{Model: model})

	req := &provider.Request{
		Model:        model,
		SystemPrompt: systemPrompt,
		Messages:     history,
		MaxTokens:    4096,
	}

	// Persist the session and user message if storage is configured
	if e.storage != nil {
		if err := e.ensureSession(ctx, opts, systemPrompt, model); err != nil {
			return nil, fmt.Errorf("engine: ensure session: %w", err)
		}
		if err := e.persistMessage(ctx, opts.SessionID, &history[len(history)-1]); err != nil {
			return nil, fmt.Errorf("engine: persist user message: %w", err)
		}
	}

	// Stream the response
	stream, err := e.provider.Stream(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("engine: start stream: %w", err)
	}
	defer stream.Close()

	var doneEvent *provider.StreamEvent
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		ev, recvErr := stream.Recv()
		if recvErr != nil {
			if errors.Is(recvErr, io.EOF) {
				break
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
			doneEvent = ev
			goto streamComplete
		case provider.EventError:
			return nil, ev.Err
		}
	}
streamComplete:

	if doneEvent == nil || doneEvent.Response == nil {
		return nil, errors.New("engine: stream ended without a done event")
	}

	// Append the assistant response to history
	history = append(history, doneEvent.Response.Message)

	// Persist the assistant message and usage atomically
	if e.storage != nil {
		err := e.storage.WithTx(ctx, func(tx storage.Tx) error {
			m := &history[len(history)-1]
			if err := e.persistMessageTx(ctx, tx, opts.SessionID, m); err != nil {
				return err
			}
			return tx.UpdateUsage(ctx, opts.SessionID, &storage.UsageUpdate{
				InputTokens:      doneEvent.Response.Usage.InputTokens,
				OutputTokens:     doneEvent.Response.Usage.OutputTokens,
				CacheReadTokens:  doneEvent.Response.Usage.CacheReadTokens,
				CacheWriteTokens: doneEvent.Response.Usage.CacheWriteTokens,
			})
		})
		if err != nil {
			return nil, fmt.Errorf("engine: persist response: %w", err)
		}
	}

	return &ConversationResult{
		Response:   doneEvent.Response.Message,
		Messages:   history,
		SessionID:  opts.SessionID,
		Usage:      doneEvent.Response.Usage,
		Iterations: 1,
	}, nil
}

// ensureSession creates a new session row if it doesn't exist yet.
func (e *Engine) ensureSession(ctx context.Context, opts *RunOptions, systemPrompt, model string) error {
	_, err := e.storage.GetSession(ctx, opts.SessionID)
	if err == nil {
		return nil // session exists
	}
	if !errors.Is(err, storage.ErrNotFound) {
		return err
	}
	return e.storage.CreateSession(ctx, &storage.Session{
		ID:           opts.SessionID,
		Source:       e.platform,
		UserID:       opts.UserID,
		Model:        model,
		SystemPrompt: systemPrompt,
		StartedAt:    time.Now().UTC(),
	})
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

// storedFromMessage converts a message.Message to a storage.StoredMessage.
func storedFromMessage(sessionID string, m *message.Message) (*storage.StoredMessage, error) {
	// Serialize Content to JSON for storage
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
