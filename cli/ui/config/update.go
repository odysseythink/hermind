package configui

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/odysseythink/hermind/config/editor"
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
		fields := m.fieldsInCurrentSection()
		if m.fieldIdx >= len(fields) {
			return m, nil
		}
		f := fields[m.fieldIdx]
		if f.Kind == editor.KindList {
			return m, nil // list editing is Task 8
		}
		cur, _ := m.doc.Get(f.Path)
		fe := newFieldEditor(f, cur)
		m.ed = &fe
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
		case "a":
			fields := m.fieldsInCurrentSection()
			if m.fieldIdx >= len(fields) || fields[m.fieldIdx].Kind != editor.KindList {
				return m, nil
			}
			f := fields[m.fieldIdx]
			ti := textinput.New()
			ti.Placeholder = "name (e.g. openai)"
			ti.Focus()
			m.ed = &fieldEditor{
				mode:  modeAddItem,
				field: f,
				input: ti,
			}
			m.editing = true
			m.status = "enter new item key, enter to confirm"
		case "d":
			fields := m.fieldsInCurrentSection()
			if m.fieldIdx >= len(fields) || fields[m.fieldIdx].Kind != editor.KindList {
				return m, nil
			}
			f := fields[m.fieldIdx]
			ti := textinput.New()
			ti.Placeholder = "name to delete"
			ti.Focus()
			m.ed = &fieldEditor{
				mode:  modeDeleteItem,
				field: f,
				input: ti,
			}
			m.editing = true
			m.status = "enter item key to delete, enter to confirm"
		}
	}
	return m, nil
}

func (m Model) updateEditing(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Type == tea.KeyEsc {
		m.editing = false
		m.ed = nil
		return m, nil
	}
	if m.ed == nil {
		return m, nil
	}

	switch m.ed.field.Kind {
	case editor.KindEnum:
		switch key.Type {
		case tea.KeyLeft, tea.KeyUp:
			if m.ed.enumIdx > 0 {
				m.ed.enumIdx--
			}
		case tea.KeyRight, tea.KeyDown:
			if m.ed.enumIdx < len(m.ed.field.Enum)-1 {
				m.ed.enumIdx++
			}
		case tea.KeyEnter:
			if errMsg := m.ed.commit(m.doc); errMsg != "" {
				m.status = errMsg
			} else {
				m.dirty = true
				m.editing = false
				m.ed = nil
			}
		}
	case editor.KindBool:
		if key.Type == tea.KeyEnter || key.Type == tea.KeySpace {
			cur := m.ed.input.Value()
			next := "true"
			if cur == "true" {
				next = "false"
			}
			m.ed.input.SetValue(next)
			if errMsg := m.ed.commit(m.doc); errMsg != "" {
				m.status = errMsg
			} else {
				m.dirty = true
				m.editing = false
				m.ed = nil
			}
		}
	default:
		if key.Type == tea.KeyEnter {
			if errMsg := m.ed.commit(m.doc); errMsg != "" {
				m.status = errMsg
			} else {
				m.dirty = true
				m.editing = false
				m.ed = nil
			}
			return m, nil
		}
		var cmd tea.Cmd
		m.ed.input, cmd = m.ed.input.Update(key)
		return m, cmd
	}
	return m, nil
}
