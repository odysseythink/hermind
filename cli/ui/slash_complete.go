package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// slashCommand is one entry in the completion catalog.
type slashCommand struct {
	Name        string
	Description string
}

// slashCommands is the single source of truth for both the completion
// popup and the `/help` rendering. Keep sorted alphabetically.
var slashCommands = []slashCommand{
	{"clear", "clear the conversation"},
	{"cost", "show session cost and token usage"},
	{"exit", "save session and exit"},
	{"help", "show this help"},
	{"model", "show the active model"},
	{"quit", "alias for /exit"},
}

// completionState is the popup's in-memory state. Nil when the popup is
// hidden. The controller (Model) owns the pointer; callers do not hold it.
type completionState struct {
	matches  []slashCommand
	selected int
}

// filterSlashCommands returns the subset of slashCommands whose Name has
// prefix. An empty prefix returns all commands. Order is preserved.
func filterSlashCommands(prefix string) []slashCommand {
	if prefix == "" {
		out := make([]slashCommand, len(slashCommands))
		copy(out, slashCommands)
		return out
	}
	var out []slashCommand
	for _, c := range slashCommands {
		if strings.HasPrefix(c.Name, prefix) {
			out = append(out, c)
		}
	}
	return out
}

// updateCompletion recomputes the popup state from the current input
// value. The popup is visible exactly when the input starts with "/" and
// at least one command matches the text after the slash (and no space has
// been typed yet — completion is only for the command name itself).
func (m *Model) updateCompletion() {
	text := m.input.Value()
	if !strings.HasPrefix(text, "/") || strings.Contains(text, " ") {
		m.completion = nil
		return
	}
	prefix := strings.TrimPrefix(text, "/")
	matches := filterSlashCommands(prefix)
	if len(matches) == 0 {
		m.completion = nil
		return
	}
	sel := 0
	if m.completion != nil && m.completion.selected < len(matches) {
		sel = m.completion.selected
	}
	m.completion = &completionState{matches: matches, selected: sel}
}

// moveCompletion shifts the highlighted row by delta (+1 down, -1 up).
// Wraps at both ends.
func (m *Model) moveCompletion(delta int) {
	if m.completion == nil || len(m.completion.matches) == 0 {
		return
	}
	n := len(m.completion.matches)
	m.completion.selected = ((m.completion.selected+delta)%n + n) % n
}

// acceptCompletion replaces the input with the highlighted match and
// hides the popup.
func (m *Model) acceptCompletion() {
	if m.completion == nil || len(m.completion.matches) == 0 {
		return
	}
	sel := m.completion.matches[m.completion.selected]
	m.input.SetValue("/" + sel.Name)
	m.input.CursorEnd()
	m.completion = nil
}

// dismissCompletion hides the popup without changing the input.
func (m *Model) dismissCompletion() {
	m.completion = nil
}

// renderCompletion produces the popup as a block of pre-styled lines, or
// empty string if the popup is hidden. Called from View(); the caller
// accounts for its height in the viewport-size math.
func (m *Model) renderCompletion() string {
	if m.completion == nil || len(m.completion.matches) == 0 {
		return ""
	}

	// Measure widest name for alignment.
	maxName := 0
	for _, c := range m.completion.matches {
		if len(c.Name) > maxName {
			maxName = len(c.Name)
		}
	}

	rowStyle := lipgloss.NewStyle()
	selRowStyle := m.skin.Accent.Bold(true)

	lines := make([]string, 0, len(m.completion.matches))
	for i, c := range m.completion.matches {
		marker := "  "
		style := rowStyle
		if i == m.completion.selected {
			marker = m.skin.Accent.Render("▸ ")
			style = selRowStyle
		}
		nameCol := style.Render("/" + c.Name)
		// Pad name column so descriptions align.
		pad := strings.Repeat(" ", maxName-len(c.Name)+2)
		desc := m.skin.Muted.Render(c.Description)
		lines = append(lines, marker+nameCol+pad+desc)
	}
	return strings.Join(lines, "\n")
}

// completionLineCount returns the number of rendered lines, so View() can
// reserve viewport space without re-rendering.
func (m *Model) completionLineCount() int {
	if m.completion == nil {
		return 0
	}
	return len(m.completion.matches)
}
