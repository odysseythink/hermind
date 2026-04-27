// agent/compression.go
package agent

import (
	"context"
	"fmt"

	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/provider"
)

// Compressor summarizes middle-of-history messages using an auxiliary LLM
// to reduce token count while preserving the conversation's head and tail.
type Compressor struct {
	cfg config.CompressionConfig
	aux provider.Provider
}

// NewCompressor constructs a Compressor. `aux` is the auxiliary provider
// used for the summarization call. If aux is nil, Compress returns the
// history unchanged.
func NewCompressor(cfg config.CompressionConfig, aux provider.Provider) *Compressor {
	return &Compressor{cfg: cfg, aux: aux}
}

// defaultPerMessageMaxTokens is the per-message ceiling used when
// CompressionConfig.PerMessageMaxTokens is unset (zero value). 8000 tokens
// is roughly 32KB of plain text — large enough to keep typical chat turns
// verbatim, small enough that a 200KB+ paste gets summarized.
const defaultPerMessageMaxTokens = 8000

// Compress summarizes the middle of the history and returns a shortened
// version. The head (first 3 messages) and tail (last ProtectLast messages)
// are preserved by default, but any single text-only message in head/tail
// that exceeds PerMessageMaxTokens is replaced with an aux-LLM summary so a
// 200KB+ paste in the protected tail can't blow the context window on its
// own. The middle is replaced by a single assistant summary message.
//
// If compression is disabled in config, the original is returned.
// If the auxiliary provider is nil, the original is returned.
// If the history is shorter than head + tail + 1, only the per-message
// oversize check runs (the middle-summary step is skipped).
func (c *Compressor) Compress(ctx context.Context, history []message.Message) ([]message.Message, error) {
	if !c.cfg.Enabled || c.aux == nil {
		return history, nil
	}

	const headCount = 3
	tailCount := c.cfg.ProtectLast
	if tailCount < 1 {
		tailCount = 20
	}

	// History too short for middle compression — but we still want to trim
	// any single oversized message that snuck in.
	if len(history) <= headCount+tailCount {
		return c.compressOversizedMessages(ctx, history), nil
	}

	head := history[:headCount]
	tail := history[len(history)-tailCount:]
	middle := history[headCount : len(history)-tailCount]

	if len(middle) == 0 {
		return c.compressOversizedMessages(ctx, history), nil
	}

	summary, err := c.summarize(ctx, middle)
	if err != nil {
		return nil, fmt.Errorf("compression: summarize: %w", err)
	}

	result := make([]message.Message, 0, headCount+1+tailCount)
	result = append(result, c.compressOversizedMessages(ctx, head)...)
	result = append(result, message.Message{
		Role:    message.RoleAssistant,
		Content: message.TextContent("[Compressed summary of earlier conversation]\n" + summary),
	})
	result = append(result, c.compressOversizedMessages(ctx, tail)...)
	return result, nil
}

// compressOversizedMessages replaces each text-only message whose estimated
// token count exceeds the per-message ceiling with an aux-summarized version.
// Tool-use / tool-result blocks pass through untouched because they carry
// structural ids that pair across messages — summarizing them would orphan
// the partner. On summarize-error a message is kept verbatim; the engine
// will surface the eventual provider 400 rather than silently drop content.
func (c *Compressor) compressOversizedMessages(ctx context.Context, msgs []message.Message) []message.Message {
	threshold := c.cfg.PerMessageMaxTokens
	if threshold == 0 {
		threshold = defaultPerMessageMaxTokens
	}
	if threshold < 0 {
		return msgs
	}
	out := make([]message.Message, 0, len(msgs))
	for _, m := range msgs {
		if !m.Content.IsText() {
			out = append(out, m)
			continue
		}
		text := m.Content.Text()
		size, err := c.aux.EstimateTokens("", text)
		if err != nil || size <= threshold {
			out = append(out, m)
			continue
		}
		summary, err := c.summarizeSingle(ctx, m.Role, text)
		if err != nil || summary == "" {
			out = append(out, m)
			continue
		}
		out = append(out, message.Message{
			Role:    m.Role,
			Content: message.TextContent("[Summarized large message]\n" + summary),
		})
	}
	return out
}

// summarizeSingle asks the aux provider for a terse summary of a single
// oversized message. The role is passed through to the prompt so the
// summarizer can preserve "what the user said" vs "what the assistant said".
func (c *Compressor) summarizeSingle(ctx context.Context, role message.Role, text string) (string, error) {
	systemPrompt := fmt.Sprintf(
		"You are a summarizer. The %s sent a message that is too large to keep verbatim. "+
			"Produce a terse, structured summary preserving key facts, decisions, code references, "+
			"file paths, error messages, and identifiers. Keep it under 500 words.",
		role,
	)
	req := &provider.Request{
		Model:        "",
		SystemPrompt: systemPrompt,
		Messages: []message.Message{
			{
				Role:    message.RoleUser,
				Content: message.TextContent(text),
			},
		},
		MaxTokens: 1000,
	}
	resp, err := c.aux.Complete(ctx, req)
	if err != nil {
		return "", err
	}
	if resp.Message.Content.IsText() {
		return resp.Message.Content.Text(), nil
	}
	var out string
	for _, b := range resp.Message.Content.Blocks() {
		if b.Type == "text" {
			out += b.Text
		}
	}
	return out, nil
}

// summarize sends the middle messages to the auxiliary provider with
// a terse summarization prompt and returns the assistant's text response.
func (c *Compressor) summarize(ctx context.Context, middle []message.Message) (string, error) {
	// Build a condensed transcript to hand to the aux provider.
	transcript := renderTranscript(middle)

	systemPrompt := "You are a summarizer. Produce a terse, bullet-point summary of the conversation below, preserving key facts, decisions, and code references. Keep it under 500 words."

	req := &provider.Request{
		Model:        "", // use aux provider's default
		SystemPrompt: systemPrompt,
		Messages: []message.Message{
			{
				Role:    message.RoleUser,
				Content: message.TextContent(transcript),
			},
		},
		MaxTokens: 1000,
	}

	resp, err := c.aux.Complete(ctx, req)
	if err != nil {
		return "", err
	}
	if resp.Message.Content.IsText() {
		return resp.Message.Content.Text(), nil
	}
	// If the aux provider returned block content, concatenate text blocks.
	var text string
	for _, b := range resp.Message.Content.Blocks() {
		if b.Type == "text" {
			text += b.Text
		}
	}
	return text, nil
}

// renderTranscript builds a plain-text transcript of conversation messages.
func renderTranscript(msgs []message.Message) string {
	var out string
	for i, m := range msgs {
		out += fmt.Sprintf("%d. %s: ", i+1, m.Role)
		if m.Content.IsText() {
			out += m.Content.Text()
		} else {
			// Summarize block content as "[tool use]" etc.
			for _, b := range m.Content.Blocks() {
				switch b.Type {
				case "text":
					out += b.Text
				case "tool_use":
					out += "[tool_use: " + b.ToolUseName + "]"
				case "tool_result":
					out += "[tool_result]"
				}
			}
		}
		out += "\n"
	}
	return out
}
