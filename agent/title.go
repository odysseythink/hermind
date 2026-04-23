package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/provider"
)

// titleMaxRunes caps DeriveTitle output length in Unicode code points.
const titleMaxRunes = 10

// DeriveTitle produces a short display title from the user's first message:
// replaces newlines with spaces, trims surrounding whitespace, truncates to
// titleMaxRunes code points. Empty input returns an empty string — callers
// render a localized "Untitled" in that case.
func DeriveTitle(msg string) string {
	s := strings.ReplaceAll(msg, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.TrimSpace(s)
	runes := []rune(s)
	if len(runes) > titleMaxRunes {
		runes = runes[:titleMaxRunes]
	}
	return string(runes)
}

// GenerateTitle asks an auxiliary provider to summarize the first few
// messages of a session into a short title (5-8 words). Returns an
// empty string if the provider isn't available.
func GenerateTitle(ctx context.Context, aux provider.Provider, model string, history []message.Message) (string, error) {
	if aux == nil {
		return "", nil
	}
	if len(history) == 0 {
		return "", nil
	}
	// Take at most the first 6 messages and flatten them.
	var b strings.Builder
	limit := 6
	if len(history) < limit {
		limit = len(history)
	}
	for i := 0; i < limit; i++ {
		m := history[i]
		b.WriteString(string(m.Role))
		b.WriteString(": ")
		b.WriteString(m.Content.Text())
		b.WriteString("\n")
	}
	prompt := fmt.Sprintf(
		"Summarize this conversation in a 5-8 word title. Do not use quotes or punctuation at the end. Reply with the title only.\n\n%s",
		b.String(),
	)
	req := &provider.Request{
		Model: model,
		Messages: []message.Message{
			{Role: message.RoleUser, Content: message.TextContent(prompt)},
		},
		MaxTokens: 32,
	}
	resp, err := aux.Complete(ctx, req)
	if err != nil {
		return "", err
	}
	title := strings.TrimSpace(resp.Message.Content.Text())
	// Guard against models that ignore the "no quotes" instruction.
	title = strings.Trim(title, `"' `)
	if len(title) > 128 {
		title = title[:128]
	}
	return title, nil
}
