// cli/ui/slash.go
package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// handleSlashCommand intercepts lines starting with "/" and dispatches
// to the matching command handler. Returns (true, cmd) if the input was
// a recognized slash command.
func (m *Model) handleSlashCommand(input string) (bool, tea.Cmd) {
	if !strings.HasPrefix(input, "/") {
		return false, nil
	}
	parts := strings.Fields(strings.TrimPrefix(input, "/"))
	if len(parts) == 0 {
		return true, nil
	}

	switch parts[0] {
	case "exit", "quit":
		m.quitting = true
		return true, tea.Quit

	case "clear":
		m.rendered = nil
		m.history = nil
		m.viewport.SetContent("")
		m.appendRenderedLine(m.skin.Muted.Render("conversation cleared"))
		return true, nil

	case "help":
		m.appendRenderedLine(m.skin.Accent.Render("Commands:"))
		m.appendRenderedLine(m.skin.Muted.Render("  /help     show this help"))
		m.appendRenderedLine(m.skin.Muted.Render("  /exit     save session and exit"))
		m.appendRenderedLine(m.skin.Muted.Render("  /clear    clear conversation"))
		m.appendRenderedLine(m.skin.Muted.Render("  /model    show active model"))
		m.appendRenderedLine(m.skin.Muted.Render("  /cost     show session cost"))
		return true, nil

	case "model":
		m.appendRenderedLine(m.skin.Muted.Render("model: ") + m.skin.Accent.Render(m.model))
		return true, nil

	case "cost":
		line := "tokens: " + formatBytesInt(m.totalUsage.InputTokens) + "↑ " +
			formatBytesInt(m.totalUsage.OutputTokens) + "↓  cost: $" + formatCost(m.totalCostUSD)
		m.appendRenderedLine(m.skin.Muted.Render(line))
		return true, nil

	default:
		m.appendRenderedLine(m.skin.Error.Render("unknown command: /" + parts[0]))
		return true, nil
	}
}
