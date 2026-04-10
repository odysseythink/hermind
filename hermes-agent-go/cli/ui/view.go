// cli/ui/view.go
package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// View implements tea.Model. Renders the 4-zone layout:
//
//	Zone 1: Banner (3 lines)
//	Zone 2: Context bar (1 line)
//	Zone 3: Conversation viewport (variable)
//	Zone 4: Input textarea (variable)
//	Zone 5: Status bar (1 line)
func (m Model) View() string {
	if m.quitting {
		return m.renderExitSummary()
	}

	// Fixed-size zones
	banner := m.renderBanner()
	contextBar := m.renderContextBar()
	statusBar := m.renderStatusBar()
	input := m.renderInput()

	// Calculate viewport height: total - banner - context - input - status - separators
	bannerLines := strings.Count(banner, "\n") + 1
	inputLines := strings.Count(input, "\n") + 1
	reserved := bannerLines + 1 /*context*/ + inputLines + 1 /*status*/ + 3 /*separators*/
	vpHeight := m.height - reserved
	if vpHeight < 3 {
		vpHeight = 3
	}
	m.viewport.Width = m.width
	m.viewport.Height = vpHeight

	sep := m.skin.Border.Render(strings.Repeat("─", max(1, m.width)))

	// Assemble
	return strings.Join([]string{
		banner,
		contextBar,
		sep,
		m.viewport.View(),
		sep,
		input,
		sep,
		statusBar,
	}, "\n")
}

// renderInput returns the styled input component.
func (m Model) renderInput() string {
	return m.input.View()
}

// renderExitSummary is shown after /exit before the program terminates.
func (m Model) renderExitSummary() string {
	summary := strings.Builder{}
	summary.WriteString("\n")
	summary.WriteString(m.skin.Accent.Render("Session complete."))
	summary.WriteString("\n")
	summary.WriteString(m.skin.Muted.Render(
		"  " + shortSessionID(m.sessionID) +
			"  ·  " + itoa(m.turnCount*2) + " messages" +
			"  ·  " + itoa(m.toolCalls) + " tool calls" +
			"  ·  " + formatBytesInt(m.totalUsage.InputTokens) + "↑ " + formatBytesInt(m.totalUsage.OutputTokens) + "↓" +
			"  ·  $" + formatCost(m.totalCostUSD),
	))
	summary.WriteString("\n")
	return summary.String()
}

// max returns the larger of two ints.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Silence unused import
var _ = lipgloss.NewStyle
