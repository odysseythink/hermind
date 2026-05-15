// agent/conversation.go
package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/pantheonadapter"
	"github.com/odysseythink/hermind/provider"
	"github.com/odysseythink/hermind/storage"
	"github.com/odysseythink/hermind/tool/memory/memprovider"
	"github.com/odysseythink/hermind/tool/memory/memprovider/citesink"
	"github.com/odysseythink/mlog"
	"github.com/odysseythink/pantheon/agent/budget"
	"github.com/odysseythink/pantheon/agent/compression"
	"github.com/odysseythink/pantheon/core"
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
	mlog.Debug(opts)
	if err := ctx.Err(); err != nil {
		mlog.Error(err)
		return nil, err
	}

	model := opts.Model

	var history []message.HermindMessage
	if opts.Ephemeral {
		history = append([]message.HermindMessage{}, opts.History...)
	} else if e.storage != nil {
		limit := e.config.HistoryLimit
		if limit <= 0 {
			limit = 1000
		}
		rows, err := e.storage.GetHistory(ctx, limit, 0)
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

	// Log history size for debugging context-overflow issues.
	historyChars := 0
	for _, m := range history {
		historyChars += len(m.Text())
	}
	mlog.Info("conversation history loaded", mlog.Int("messages", len(history)), mlog.Int("chars", historyChars))

	userMsg := message.HermindMessage{
		Role:    core.MESSAGE_ROLE_USER,
		Content: core.NewTextContent(opts.UserMessage),
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

	var toolDefs []core.ToolDefinition
	if e.tools != nil {
		if e.toolSelector != nil {
			toolDefs = e.toolSelector.Select(opts.UserMessage, history, e.tools)
			mlog.Info("dynamic tool selection", mlog.Int("selected", len(toolDefs)), mlog.Int("total", len(e.tools.Entries(nil))))
		} else {
			toolDefs = e.tools.Definitions(nil)
		}
	}

	b := budget.New(e.config.MaxTurns)
	totalUsage := core.Usage{}
	iterations := 0
	emptyRetries := 0
	stripTools := false // set true when retrying after empty response
	var lastResponse message.HermindMessage

	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if !b.Consume() {
			break
		}
		iterations++

		if !opts.Ephemeral && e.compressor != nil && shouldCompress(history, e.config.Compression, e.modelInfo, model) {
			if compressed, err := e.compressor.Compress(ctx, history); err == nil {
				history = compressed
			}
		}

		maxTokens := e.config.MaxTokens
		if maxTokens == 0 {
			maxTokens = 4096
		}
		legacyMsgs := make([]message.Message, len(history))
		for i, m := range history {
			legacyMsgs[i] = message.HermindMessageToLegacy(m)
		}

		req := &provider.Request{
			Model:        model,
			SystemPrompt: systemPrompt,
			Messages:     legacyMsgs,
			MaxTokens:    maxTokens,
		}
		if !stripTools {
			req.Tools = toolDefs
		}

		resp, err := e.streamOnce(ctx, req)
		if err != nil {
			return nil, err
		}

		respMsg := message.LegacyToHermindMessage(resp.Message)
		history = append(history, respMsg)
		lastResponse = respMsg
		if e.memory != nil {
			e.memory.ObserveTurn(respMsg)
		}
		totalUsage.PromptTokens += resp.Usage.InputTokens
		totalUsage.CompletionTokens += resp.Usage.OutputTokens
		totalUsage.CacheReadTokens += resp.Usage.CacheReadTokens
		totalUsage.CacheWriteTokens += resp.Usage.CacheWriteTokens

		if !opts.Ephemeral && e.storage != nil {
			txErr := e.storage.WithTx(ctx, func(tx storage.Tx) error {
				m := &history[len(history)-1]
				if err := e.persistMessageTx(ctx, tx, m); err != nil {
					return err
				}
				return tx.UpdateUsage(ctx, &storage.UsageUpdate{
					InputTokens:      resp.Usage.InputTokens,
					OutputTokens:     resp.Usage.OutputTokens,
					CacheReadTokens:  resp.Usage.CacheReadTokens,
					CacheWriteTokens: resp.Usage.CacheWriteTokens,
				})
			})
			if txErr != nil {
				return nil, fmt.Errorf("engine: persist response: %w", txErr)
			}
		}

		toolCalls := resp.Message.ExtractToolCalls()
		if len(toolCalls) == 0 {
			// If the model produced only whitespace after tool calls,
			// it likely failed to summarize the results. Inject a
			// follow-up message to give it another chance (up to 2
			// retries). This is common with smaller local models that
			// struggle with multi-turn tool calling.
			reply := resp.Message.Text()
			if strings.TrimSpace(reply) == "" && iterations > 1 {
				emptyRetries++
				if emptyRetries <= 2 {
					mlog.Warning("model returned empty response after tool calls, retrying",
						mlog.Int("iteration", iterations), mlog.Int("empty_retry", emptyRetries))
					nudge := message.HermindMessage{
						Role:    core.MESSAGE_ROLE_USER,
						Content: core.NewTextContent("Please provide your answer based on the search results above. Do not call any more tools."),
					}
					history = append(history, nudge)
					if !opts.Ephemeral && e.storage != nil {
						if err := e.persistMessage(ctx, &nudge); err != nil {
							return nil, fmt.Errorf("engine: persist nudge: %w", err)
						}
					}
					stripTools = true
					continue
				}
				mlog.Warning("model returned empty response after tool calls, giving up",
					mlog.Int("iteration", iterations), mlog.Int("empty_retries", emptyRetries))
			}
			break
		}

		if e.bufferEvery > 0 && iterations%e.bufferEvery == 0 &&
			e.memory != nil && len(e.memory.Providers()) > 0 {
			_ = e.memory.SyncTurn(ctx, opts.UserMessage, resp.Message.Text())
		}

		toolResults := e.executeToolCalls(ctx, toolCalls)
		toolResultMsg := message.HermindMessage{
			Role:    core.MESSAGE_ROLE_TOOL,
			Content: toolResults,
		}
		history = append(history, toolResultMsg)

		if !opts.Ephemeral && e.storage != nil {
			if err := e.persistMessage(ctx, &toolResultMsg); err != nil {
				return nil, fmt.Errorf("engine: persist tool result: %w", err)
			}
		}
	}

	assistantReply := lastResponse.Text()

	var verdict *Verdict
	if e.conversationJudge != nil {
		injectedMems := make([]InjectedMemory, len(conversationInjectedMems))
		for i, m := range conversationInjectedMems {
			injectedMems[i] = InjectedMemory{ID: m.ID, Content: m.Content}
		}
		v, _ := e.conversationJudge.Run(ctx, JudgeInput{
			History:          history,
			InjectedMemories: injectedMems,
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
		mlog.Info("conversation.judged",
			mlog.String("outcome", verdict.Outcome),
			mlog.Int("memories_hit", len(verdict.MemoriesUsed)),
			mlog.Int("memories_injected", len(conversationInjectedMems)),
			mlog.Int("skills_extracted", len(verdict.SkillsToExtract)),
			mlog.String("reasoning", verdict.Reasoning),
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

func (e *Engine) streamOnce(ctx context.Context, req *provider.Request) (*provider.Response, error) {
	if e.model == nil {
		return nil, errors.New("engine: no LLM model configured — set one under Settings → Models → Providers")
	}

	maxTokens := req.MaxTokens
	pReq := &core.Request{
		Messages:     messagesToPantheon(req.Messages),
		SystemPrompt: req.SystemPrompt,
		Tools:        req.Tools,
		MaxTokens:    &maxTokens,
	}
	if req.Temperature != nil {
		pReq.Temperature = req.Temperature
	}
	if req.TopP != nil {
		pReq.TopP = req.TopP
	}
	if len(req.StopSequences) > 0 {
		pReq.StopSequences = req.StopSequences
	}
	if len(req.Tools) > 0 {
		pReq.ToolChoice = core.ToolChoice{Mode: core.ToolChoiceModeAuto}
	}

	mlog.Info("streamOnce: starting", mlog.String("model", req.Model), mlog.Int("messages", len(req.Messages)), mlog.Int("tools", len(req.Tools)))
	seq, err := e.model.Stream(ctx, pReq)
	if err != nil {
		mlog.Error("streamOnce: start stream failed", mlog.Err(err))
		return nil, fmt.Errorf("engine: start stream: %w", err)
	}
	mlog.Info("streamOnce: stream started")

	var contentParts []core.ContentParter
	var reasoning string
	var usage core.Usage
	var finishReason string
	var textBuf string
	partCount := 0

	for part, err := range seq {
		partCount++
		if err != nil {
			mlog.Error("streamOnce: stream recv error", mlog.Err(err), mlog.Int("parts_received", partCount))
			return nil, fmt.Errorf("engine: stream recv: %w", err)
		}
		mlog.Debug("streamOnce: part received", mlog.String("type", string(part.Type)), mlog.Int("part_num", partCount))
		switch part.Type {
		case core.StreamPartTypeTextDelta:
			textBuf += part.TextDelta
			if e.onStreamDelta != nil {
				e.onStreamDelta(&provider.StreamDelta{Content: part.TextDelta})
			}
		case core.StreamPartTypeReasoningDelta:
			reasoning += part.ReasoningDelta
		case core.StreamPartTypeToolCall:
			contentParts = append(contentParts, core.ToolCallPart{
				ID:        part.ToolCall.ID,
				Name:      part.ToolCall.Name,
				Arguments: part.ToolCall.Arguments,
			})
		case core.StreamPartTypeUsage:
			if part.Usage != nil {
				usage.PromptTokens = part.Usage.PromptTokens
				usage.CompletionTokens = part.Usage.CompletionTokens
			}
		case core.StreamPartTypeFinish:
			finishReason = part.FinishReason
		}
	}
	mlog.Info("streamOnce: stream done", mlog.Int("parts_total", partCount), mlog.String("finish_reason", finishReason), mlog.Int("text_len", len(textBuf)))

	if textBuf != "" {
		contentParts = append([]core.ContentParter{core.TextPart{Text: textBuf}}, contentParts...)
	}
	if finishReason == "" {
		finishReason = "stop"
	}

	msg := message.MessageFromPantheon(core.Message{
		Role:    core.MESSAGE_ROLE_ASSISTANT,
		Content: contentParts,
	})
	if reasoning != "" {
		msg.Content = append(msg.Content, core.ReasoningPart{Text: reasoning})
	}

	return &provider.Response{
		Message:      message.HermindMessageToLegacy(msg),
		FinishReason: finishReason,
		Usage: message.Usage{
			InputTokens:      usage.PromptTokens,
			OutputTokens:     usage.CompletionTokens,
			PromptTokens:     usage.PromptTokens,
			CompletionTokens: usage.CompletionTokens,
			CacheReadTokens:  usage.CacheReadTokens,
			CacheWriteTokens: usage.CacheWriteTokens,
		},
	}, nil
}

func messagesToPantheon(msgs []message.Message) []core.Message {
	out := make([]core.Message, len(msgs))
	for i, m := range msgs {
		out[i] = message.ToPantheon(message.LegacyToHermindMessage(m))
		roleStr := string(out[i].Role)
		mlog.Debug("messagesToPantheon", mlog.Int("idx", i), mlog.String("role", roleStr), mlog.Int("parts", len(out[i].Content)))
		for j, p := range out[i].Content {
			mlog.Debug("messagesToPantheon part", mlog.Int("msg_idx", i), mlog.Int("part_idx", j), mlog.String("type", fmt.Sprintf("%T", p)))
		}
	}
	return out
}

func (e *Engine) executeToolCalls(ctx context.Context, calls []core.ToolCallPart) []core.ContentParter {
	results := make([]core.ContentParter, 0, len(calls))
	for _, call := range calls {
		if e.onToolStart != nil {
			e.onToolStart(call)
		}

		var result string
		if e.tools == nil {
			result = `{"error":"no tool registry configured"}`
		} else {
			out, err := e.tools.Dispatch(ctx, call.Name, json.RawMessage(call.Arguments))
			if err != nil {
				result = fmt.Sprintf(`{"error":"dispatch failed: %s"}`, err.Error())
			} else {
				result = out
			}
		}

		if e.onToolResult != nil {
			e.onToolResult(call, result)
		}

		results = append(results, core.ToolResultPart{
			ToolCallID: call.ID,
			Name:       call.Name,
			Content:    core.NewTextContent(result),
		})
	}
	return results
}

func (e *Engine) persistMessage(ctx context.Context, m *message.HermindMessage) error {
	stored, err := storedFromMessage(m)
	if err != nil {
		return err
	}
	return e.storage.AppendMessage(ctx, stored)
}

func (e *Engine) persistMessageTx(ctx context.Context, tx storage.Tx, m *message.HermindMessage) error {
	stored, err := storedFromMessage(m)
	if err != nil {
		return err
	}
	return tx.AppendMessage(ctx, stored)
}

func shouldCompress(history []message.HermindMessage, cfg compression.CompressionConfig, r *pantheonadapter.ModelInfoResolver, model string) bool {
	if !cfg.Enabled || cfg.ProtectLast <= 0 {
		return false
	}
	if len(history) > 3*cfg.ProtectLast {
		return true
	}
	if r == nil || cfg.Threshold <= 0 {
		return false
	}
	info := r.Lookup(model)
	if info.ContextLength <= 0 {
		return false
	}
	budget := int(float64(info.ContextLength) * cfg.Threshold)
	if budget <= 0 {
		return false
	}
	return estimateHistoryTokens(r, history) >= budget
}

func estimateHistoryTokens(r *pantheonadapter.ModelInfoResolver, history []message.HermindMessage) int {
	if r == nil {
		return 0
	}
	total := 0
	for _, m := range history {
		total += estimateMessageTokens(r, m)
	}
	return total
}

func estimateMessageTokens(r *pantheonadapter.ModelInfoResolver, m message.HermindMessage) int {
	if r == nil {
		return 0
	}
	total := 0
	for _, p := range m.Content {
		switch part := p.(type) {
		case core.TextPart:
			total += r.EstimateTokens(part.Text)
		case core.ToolCallPart:
			total += r.EstimateTokens(part.Name + part.Arguments)
		case core.ToolResultPart:
			total += r.EstimateTokens(message.HermindMessage{Content: part.Content}.Text())
		}
	}
	return total
}

func storedFromMessage(m *message.HermindMessage) (*storage.StoredMessage, error) {
	contentJSON, err := json.Marshal(m.Content)
	if err != nil {
		return nil, fmt.Errorf("engine: marshal content: %w", err)
	}
	return &storage.StoredMessage{
		Role:       string(m.Role),
		Content:    string(contentJSON),
		ToolCallID: m.ToolCallID,
		Timestamp:  time.Now().UTC(),
		Reasoning:  (*m).ExtractReasoning(),
	}, nil
}

// messageFromStored rebuilds a message.HermindMessage from a StoredMessage
// pulled out of storage.GetHistory.
func messageFromStored(row *storage.StoredMessage) (message.HermindMessage, error) {
	content, err := parseContentJSON([]byte(row.Content))
	if err != nil {
		return message.HermindMessage{}, fmt.Errorf("engine: decode stored content: %w", err)
	}
	if row.Reasoning != "" {
		content = append([]core.ContentParter{core.ReasoningPart{Text: row.Reasoning}}, content...)
	}
	return message.HermindMessage{
		Role:       core.MessageRoleType(row.Role),
		Content:    content,
		ToolCallID: row.ToolCallID,
	}, nil
}

// parseContentJSON parses stored content JSON handling both the old
// hermind Content/ContentBlock formats and the new pantheon parts format.
func parseContentJSON(data []byte) ([]core.ContentParter, error) {
	if len(data) == 0 {
		return nil, nil
	}
	if string(data) == "null" {
		return nil, nil
	}

	// JSON string → text part
	if data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return nil, err
		}
		return core.NewTextContent(s), nil
	}

	// Must be an array
	if data[0] != '[' {
		return nil, fmt.Errorf("content must be string or array, got %q", data[:1])
	}

	// Try new format first (core.Message unmarshal handles ContentParter array)
	wrapper := fmt.Sprintf(`{"role":"user","content":%s}`, string(data))
	var msg core.Message
	if err := json.Unmarshal([]byte(wrapper), &msg); err == nil {
		return msg.Content, nil
	}

	// Fallback: old ContentBlock array format
	var rawItems []json.RawMessage
	if err := json.Unmarshal(data, &rawItems); err != nil {
		return nil, err
	}

	var parts []core.ContentParter
	for _, raw := range rawItems {
		var typ struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(raw, &typ); err != nil {
			return nil, err
		}

		switch typ.Type {
		case "text":
			var p core.TextPart
			if err := json.Unmarshal(raw, &p); err != nil {
				return nil, err
			}
			parts = append(parts, p)
		case "image_url":
			var old struct {
				ImageURL struct {
					URL    string `json:"url"`
					Detail string `json:"detail,omitempty"`
				} `json:"image_url"`
			}
			if err := json.Unmarshal(raw, &old); err != nil {
				return nil, err
			}
			parts = append(parts, core.ImagePart{
				URL:    old.ImageURL.URL,
				Detail: old.ImageURL.Detail,
			})
		case "tool_use":
			var old struct {
				ID    string          `json:"id"`
				Name  string          `json:"name"`
				Input json.RawMessage `json:"input"`
			}
			if err := json.Unmarshal(raw, &old); err != nil {
				return nil, err
			}
			args := string(old.Input)
			if args == "" {
				args = "{}"
			}
			parts = append(parts, core.ToolCallPart{
				ID:        old.ID,
				Name:      old.Name,
				Arguments: args,
			})
		case "tool_result":
			var old struct {
				ID      string `json:"id"`
				Content string `json:"content"`
			}
			if err := json.Unmarshal(raw, &old); err != nil {
				return nil, err
			}
			parts = append(parts, core.ToolResultPart{
				ToolCallID: old.ID,
				Content:    core.NewTextContent(old.Content),
			})
		default:
			return nil, fmt.Errorf("unknown old content block type: %q", typ.Type)
		}
	}
	return parts, nil
}
