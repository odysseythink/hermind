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
		maxName := 0
		for _, c := range slashCommands {
			if len(c.Name) > maxName {
				maxName = len(c.Name)
			}
		}
		for _, c := range slashCommands {
			pad := strings.Repeat(" ", maxName-len(c.Name)+2)
			m.appendRenderedLine(m.skin.Muted.Render("  /" + c.Name + pad + c.Description))
		}
		return true, nil

	case "model":
		m.appendRenderedLine(m.skin.Muted.Render("model: ") + m.skin.Accent.Render(m.getRuntime().Model))
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
