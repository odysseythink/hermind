package configui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	if m.editing {
		return m.updateEditing(key)
	}

	switch key.Type {
	case tea.KeyTab:
		m.sectionIdx = (m.sectionIdx + 1) % len(m.sections)
		m.fieldIdx = 0
	case tea.KeyShiftTab:
		m.sectionIdx = (m.sectionIdx - 1 + len(m.sections)) % len(m.sections)
		m.fieldIdx = 0
	case tea.KeyUp:
		if m.fieldIdx > 0 {
			m.fieldIdx--
		}
	case tea.KeyDown:
		if m.fieldIdx < len(m.fieldsInCurrentSection())-1 {
			m.fieldIdx++
		}
	case tea.KeyEnter:
		m.editing = true
	case tea.KeyRunes:
		switch string(key.Runes) {
		case "q":
			return m, tea.Quit
		case "s":
			if err := m.doc.Save(); err != nil {
				m.status = "save failed: " + err.Error()
			} else {
				m.dirty = false
				m.status = "saved. restart hermind to apply."
			}
		}
	}
	return m, nil
}

// updateEditing is filled in by Task 7 (field editors). For now, Esc cancels.
func (m Model) updateEditing(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Type == tea.KeyEsc {
		m.editing = false
	}
	return m, nil
}
