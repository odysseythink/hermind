package agent

import (
	"strings"
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
