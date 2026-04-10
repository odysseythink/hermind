// agent/compression.go
package agent

import (
	"context"
	"fmt"

	"github.com/nousresearch/hermes-agent/config"
	"github.com/nousresearch/hermes-agent/message"
	"github.com/nousresearch/hermes-agent/provider"
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

// Compress summarizes the middle of the history and returns a shortened
// version. The head (first 3 messages) and tail (last ProtectLast messages)
// are preserved verbatim; the middle is replaced by a single assistant
// summary message.
//
// If the history is shorter than head + tail + 1, the original is returned.
// If compression is disabled in config, the original is returned.
// If the auxiliary provider is nil, the original is returned.
func (c *Compressor) Compress(ctx context.Context, history []message.Message) ([]message.Message, error) {
	if !c.cfg.Enabled || c.aux == nil {
		return history, nil
	}

	const headCount = 3
	tailCount := c.cfg.ProtectLast
	if tailCount < 1 {
		tailCount = 20
	}

	// Not enough history to compress
	if len(history) <= headCount+tailCount {
		return history, nil
	}

	head := history[:headCount]
	tail := history[len(history)-tailCount:]
	middle := history[headCount : len(history)-tailCount]

	if len(middle) == 0 {
		return history, nil
	}

	summary, err := c.summarize(ctx, middle)
	if err != nil {
		return nil, fmt.Errorf("compression: summarize: %w", err)
	}

	result := make([]message.Message, 0, headCount+1+tailCount)
	result = append(result, head...)
	result = append(result, message.Message{
		Role:    message.RoleAssistant,
		Content: message.TextContent("[Compressed summary of earlier conversation]\n" + summary),
	})
	result = append(result, tail...)
	return result, nil
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
