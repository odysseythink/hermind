// agent/conversation.go
package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/provider"
	"github.com/odysseythink/hermind/storage"
	"github.com/odysseythink/hermind/tool"
	"github.com/odysseythink/hermind/tool/memory/memprovider"
	"github.com/odysseythink/hermind/tool/memory/memprovider/citesink"
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
		activeSkills = e.activeSkills(opts.UserMessage)
	}
	var injectedMems []memprovider.InjectedMemory
	if e.activeMemories != nil {
		injectedMems = e.activeMemories(ctx, opts.UserMessage)
	}
	memContents := make([]string, 0, len(injectedMems))
	for _, m := range injectedMems {
		memContents = append(memContents, m.Content)
	}
	activeSkills, memContents = applySynergyBudget(activeSkills, memContents, e.synergy)
	systemPrompt := e.prompt.Build(&PromptOptions{
		Model:          model,
		ActiveSkills:   activeSkills,
		ActiveMemories: memContents,
		ObsidianCtx:    opts.ObsidianCtx,
	})

	// Preserve the full injected set for end-of-conversation feedback.
	conversationInjectedMems := injectedMems

	var cited []string
	ctx = citesink.WithSink(ctx, func(id string) { cited = append(cited, id) })

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

		if !opts.Ephemeral && e.compressor != nil && shouldCompress(history, e.config.Compression, e.provider, model) {
			if newHistory, err := e.compressor.Compress(ctx, history); err == nil {
				history = newHistory
			}
		}

		maxTokens := e.config.MaxTokens
		if maxTokens == 0 {
			maxTokens = 4096
		}
		req := &provider.Request{
			Model:        model,
			SystemPrompt: systemPrompt,
			Messages:     history,
			Tools:        toolDefs,
			MaxTokens:    maxTokens,
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

		if e.bufferEvery > 0 && iterations%e.bufferEvery == 0 &&
			e.memory != nil && len(e.memory.Providers()) > 0 {
			_ = e.memory.SyncTurn(ctx, opts.UserMessage, resp.Message.Content.Text())
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

	assistantReply := lastResponse.Content.Text()

	var verdict *Verdict
	if e.conversationJudge != nil {
		v, _ := e.conversationJudge.Run(ctx, JudgeInput{
			History:          history,
			InjectedMemories: conversationInjectedMems,
			InjectedSkills:   activeSkills,
			Platform:         e.platform,
		})
		verdict = v
	}

	if verdict != nil {
		syncMemoryFeedback(ctx, e.storage, conversationInjectedMems, verdict, cited, assistantReply)
	}

	if e.memory != nil && len(e.memory.Providers()) > 0 {
		_ = e.memory.SyncTurn(ctx, opts.UserMessage, assistantReply)
	}

	if e.skillsEvolver != nil {
		_ = e.skillsEvolver.Extract(ctx, history, verdict)
	}

	if verdict != nil {
		slog.Info("conversation.judged",
			"outcome", verdict.Outcome,
			"memories_hit", len(verdict.MemoriesUsed),
			"memories_injected", len(conversationInjectedMems),
			"skills_extracted", len(verdict.SkillsToExtract),
			"reasoning", verdict.Reasoning,
		)
		if e.storage != nil {
			data, _ := json.Marshal(map[string]any{
				"outcome":           verdict.Outcome,
				"memories_hit":      len(verdict.MemoriesUsed),
				"memories_injected": len(conversationInjectedMems),
				"skills_extracted":  len(verdict.SkillsToExtract),
			})
			_ = e.storage.AppendMemoryEvent(ctx, time.Now().UTC(), "conversation.judged", data)
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

// shouldCompress decides whether to invoke the Compressor for this turn.
// Two triggers, OR-ed together:
//
//  1. Message-count trigger (legacy): len(history) > 3*ProtectLast.
//     Cheap, doesn't need a tokenizer — kept as a safety net.
//  2. Token-budget trigger: estimated history tokens >= Threshold *
//     ContextLength of the active model. Catches the "few messages, but
//     one of them is a 700KB paste" case that the count trigger misses.
//
// The token trigger is best-effort: it silently no-ops when the provider
// or model info is unavailable, leaving the count trigger as the floor.
func shouldCompress(history []message.Message, cfg config.CompressionConfig, p provider.Provider, model string) bool {
	if !cfg.Enabled {
		return false
	}
	if cfg.ProtectLast <= 0 {
		return false
	}
	if len(history) > 3*cfg.ProtectLast {
		return true
	}
	if p == nil || cfg.Threshold <= 0 {
		return false
	}
	info := p.ModelInfo(model)
	if info == nil || info.ContextLength <= 0 {
		return false
	}
	budget := int(float64(info.ContextLength) * cfg.Threshold)
	if budget <= 0 {
		return false
	}
	return estimateHistoryTokens(p, model, history) >= budget
}

// estimateHistoryTokens sums the provider's per-block token estimate for
// every message. Returns 0 when the provider can't estimate.
func estimateHistoryTokens(p provider.Provider, model string, history []message.Message) int {
	if p == nil {
		return 0
	}
	total := 0
	for _, m := range history {
		total += estimateMessageTokens(p, model, m)
	}
	return total
}

// estimateMessageTokens counts the rendered text of every block. tool_use
// inputs and tool_result payloads are included because they ride the wire
// alongside chat text.
func estimateMessageTokens(p provider.Provider, model string, m message.Message) int {
	if p == nil {
		return 0
	}
	if m.Content.IsText() {
		n, _ := p.EstimateTokens(model, m.Content.Text())
		return n
	}
	total := 0
	for _, b := range m.Content.Blocks() {
		switch b.Type {
		case "text":
			n, _ := p.EstimateTokens(model, b.Text)
			total += n
		case "tool_use":
			n, _ := p.EstimateTokens(model, b.ToolUseName+string(b.ToolUseInput))
			total += n
		case "tool_result":
			n, _ := p.EstimateTokens(model, b.ToolResult)
			total += n
		}
	}
	return total
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
