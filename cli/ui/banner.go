// cli/ui/banner.go
package ui

import "strings"

// bannerASCII is the stylized HERMES logo shown at the top of the REPL.
// Kept short (3 lines) so it doesn't dominate the viewport on small terminals.
const bannerASCII = `╭─────────────────────────╮
│    HERMES AGENT         │
╰─────────────────────────╯`

// renderBanner returns the banner styled by the current skin.
func (m Model) renderBanner() string {
	return m.skin.Accent.Render(bannerASCII)
}

// renderContextBar returns the "claude-opus-4-6 · session abc12345" line.
func (m Model) renderContextBar() string {
	return m.skin.Muted.Render("  " + m.getRuntime().Model + "  ·  session " + shortSessionID(m.sessionID))
}

// shortSessionID returns the first 8 characters of a session UUID.
func shortSessionID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

// Silence unused import
var _ = strings.Builder{}
