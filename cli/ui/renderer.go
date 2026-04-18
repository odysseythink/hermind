// cli/ui/renderer.go
package ui

import (
	"strings"

	"github.com/charmbracelet/glamour"
)

// renderAssistantText runs the assistant's final markdown text through
// glamour for pretty terminal rendering. If glamour fails for any reason,
// falls back to the raw text.
func renderAssistantText(text string, skin Skin) string {
	if text == "" {
		return ""
	}
	// Hard-code the style. glamour's "auto" mode queries the terminal's
	// background color (OSC 11) and cursor position (CSI CPR) via termenv
	// to pick dark vs. light. Those queries return responses on stdin
	// while bubbletea is in the altScreen, and the bytes leak into the
	// textarea as garbage like `]11;rgb:..." and `[<row>;<col>R`. Using
	// a fixed style keeps glamour from touching the terminal.
	style := "dark"
	if skin.Name == "minimal" {
		style = "ascii"
	}

	out, err := glamour.Render(text, style)
	if err != nil {
		return text
	}
	return strings.TrimRight(out, "\n")
}

// renderToolResult formats a tool result as a list of pre-rendered lines.
// Wraps the snippet in `│` prefixes and ends with `└`.
// Truncates to ~12 lines to keep the viewport from overflowing.
func renderToolResult(result string, skin Skin) []string {
	const maxLines = 12
	const maxChars = 600

	snippet := result
	if len(snippet) > maxChars {
		snippet = snippet[:maxChars] + "\n... [truncated]"
	}
	lines := strings.Split(snippet, "\n")
	if len(lines) > maxLines {
		lines = append(lines[:maxLines], "... [+"+itoa(len(strings.Split(snippet, "\n"))-maxLines)+" lines]")
	}

	out := make([]string, 0, len(lines)+1)
	for _, l := range lines {
		out = append(out, skin.Muted.Render(skin.OutputPrefix+" ")+l)
	}
	out = append(out, skin.Muted.Render(skin.OutputEnd))
	return out
}
