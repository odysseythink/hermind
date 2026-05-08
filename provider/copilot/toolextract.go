package copilot

import (
	"encoding/json"
	"regexp"
	"strings"
)

// ToolCall is the parsed form of a <tool_call>{...}</tool_call> block.
// The shape mirrors OpenAI's function-calling schema because that's
// what the Copilot CLI emits.
type ToolCall struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

var toolCallRe = regexp.MustCompile(`(?s)<tool_call>\s*(\{.*?\})\s*</tool_call>`)

// ExtractToolCalls scans text for <tool_call>{...}</tool_call> blocks.
// It returns every successfully-parsed call plus the text with the
// blocks removed. Malformed blocks are left in place — the caller
// should render them as text so the user can see what went wrong.
func ExtractToolCalls(text string) ([]ToolCall, string) {
	matches := toolCallRe.FindAllStringSubmatchIndex(text, -1)
	if len(matches) == 0 {
		return nil, text
	}
	var calls []ToolCall
	var sb strings.Builder
	cursor := 0
	for _, m := range matches {
		// m = [fullStart, fullEnd, groupStart, groupEnd]
		block := text[m[2]:m[3]]
		var tc ToolCall
		if err := json.Unmarshal([]byte(block), &tc); err != nil {
			// Keep the raw block in-line; skip the call.
			sb.WriteString(text[cursor:m[1]])
			cursor = m[1]
			continue
		}
		// Drop the matched range from cleaned output.
		sb.WriteString(text[cursor:m[0]])
		cursor = m[1]
		calls = append(calls, tc)
	}
	sb.WriteString(text[cursor:])
	return calls, sb.String()
}
