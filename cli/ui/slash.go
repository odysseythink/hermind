// cli/ui/slash.go
package ui

import (
	"context"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// handleSlashCommand intercepts lines starting with "/" and dispatches
// to the matching command handler. Returns (true, cmd) if the input was
// a recognized slash command.
//
// Dispatch order:
//  1. TUI-only commands (exit/quit/clear/help/model/cost) run inline —
//     they need direct access to Model state.
//  2. Anything else is handed to the skills.SlashRegistry, which covers
//     the builtins installed by RegisterBuiltins plus every skill
//     command produced by RegisterSkills.
//  3. If nothing handled it, an "unknown command" error is shown.
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
		cmds := m.availableSlashCommands()
		m.appendRenderedLine(m.skin.Accent.Render("Commands:"))
		maxName := 0
		for _, c := range cmds {
			if len(c.Name) > maxName {
				maxName = len(c.Name)
			}
		}
		for _, c := range cmds {
			pad := strings.Repeat(" ", maxName-len(c.Name)+2)
			m.appendRenderedLine(m.skin.Muted.Render("  /" + c.Name + pad + c.Description))
		}
		return true, nil

	case "model":
		m.appendRenderedLine(m.skin.Muted.Render("model: ") + m.skin.Accent.Render(m.model))
		return true, nil

	case "cost":
		line := "tokens: " + formatBytesInt(m.totalUsage.InputTokens) + "↑ " +
			formatBytesInt(m.totalUsage.OutputTokens) + "↓  cost: $" + formatCost(m.totalCostUSD)
		m.appendRenderedLine(m.skin.Muted.Render(line))
		return true, nil
	}

	// Fall through to the dynamic registry (skills + skills-package builtins).
	if m.slashReg != nil {
		reply, handled, err := m.slashReg.Dispatch(context.Background(), input)
		if handled {
			if err != nil {
				m.appendRenderedLine(m.skin.Error.Render(err.Error()))
				return true, nil
			}
			if reply != "" {
				for _, line := range strings.Split(strings.TrimRight(reply, "\n"), "\n") {
					m.appendRenderedLine(m.skin.Muted.Render(line))
				}
			}
			return true, nil
		}
	}

	m.appendRenderedLine(m.skin.Error.Render("unknown command: /" + parts[0]))
	return true, nil
}
