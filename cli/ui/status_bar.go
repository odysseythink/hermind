// cli/ui/status_bar.go
package ui

import "strings"

// renderStatusBar builds the bottom status line: "tokens: 1.2k↑ 340↓  cost: $0.08  ◈ state".
func (m Model) renderStatusBar() string {
	var state string
	switch m.state {
	case StateIdle:
		state = m.skin.Muted.Render(m.skin.ActiveChar + " idle")
	case StateStreaming:
		state = m.skin.Accent.Render(m.skin.ActiveChar + " streaming")
	case StateToolRunning:
		state = m.skin.Success.Render(m.skin.ActiveChar + " tool running")
	}

	tokens := "tokens: " + formatBytesInt(m.totalUsage.InputTokens) + "↑ " + formatBytesInt(m.totalUsage.OutputTokens) + "↓"
	cost := "cost: $" + formatCost(m.totalCostUSD)

	parts := []string{
		m.skin.Muted.Render(tokens),
		m.skin.Muted.Render(cost),
		state,
	}
	return strings.Join(parts, "  ")
}

// formatCost returns a dollar amount with 3 decimal places.
func formatCost(c float64) string {
	if c == 0 {
		return "0.000"
	}
	// Cheap formatter: round to milli-dollars.
	n := int(c * 1000)
	dollars := n / 1000
	cents := n % 1000
	// Pad cents to 3 digits
	centsStr := itoa(cents)
	for len(centsStr) < 3 {
		centsStr = "0" + centsStr
	}
	return itoa(dollars) + "." + centsStr
}
