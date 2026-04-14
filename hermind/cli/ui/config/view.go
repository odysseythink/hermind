package configui

import (
	"fmt"
	"strings"

	"github.com/odysseythink/hermind/config/editor"
)

// View implements tea.Model.
func (m Model) View() string {
	var b strings.Builder
	b.WriteString("hermind config — [tab] section  [↑↓] field  [enter] edit/commit  [esc] cancel  [a] add  [d] del  [s] save  [q] quit\n\n")

	// section column
	b.WriteString("Sections:\n")
	for i, s := range m.sections {
		marker := "  "
		if i == m.sectionIdx {
			marker = "> "
		}
		fmt.Fprintf(&b, "%s%s\n", marker, s)
	}
	b.WriteString("\nFields:\n")

	for i, f := range m.fieldsInCurrentSection() {
		marker := "  "
		if i == m.fieldIdx {
			marker = "> "
		}
		if f.Kind == editor.KindList {
			fmt.Fprintf(&b, "%s%-28s [a: add, d: delete]\n", marker, f.Label+":")
			for _, item := range m.doc.MapKeys(f.Path) {
				fmt.Fprintf(&b, "    - %s\n", item)
			}
			continue
		}
		val, _ := m.doc.Get(f.Path)
		if f.Kind == editor.KindSecret && val != "" {
			val = "••••"
		}
		fmt.Fprintf(&b, "%s%-28s %s\n", marker, f.Label+":", val)
	}

	if help := m.currentFieldHelp(); help != "" {
		b.WriteString("\n")
		b.WriteString(help)
	}
	if m.status != "" {
		b.WriteString("\n")
		b.WriteString(m.status)
	}
	return b.String()
}

func (m Model) currentFieldHelp() string {
	fields := m.fieldsInCurrentSection()
	if m.fieldIdx >= len(fields) {
		return ""
	}
	return fields[m.fieldIdx].Help
}
