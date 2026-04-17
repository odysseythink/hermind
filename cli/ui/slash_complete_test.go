package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestFilterSlashCommands_EmptyPrefixReturnsAll(t *testing.T) {
	got := filterSlashCommands("")
	if len(got) != len(slashCommands) {
		t.Errorf("len = %d, want %d", len(got), len(slashCommands))
	}
}

func TestFilterSlashCommands_PrefixNarrows(t *testing.T) {
	got := filterSlashCommands("c")
	names := map[string]bool{}
	for _, c := range got {
		names[c.Name] = true
	}
	if !names["clear"] || !names["cost"] {
		t.Errorf("expected clear+cost, got %v", names)
	}
	if names["help"] || names["exit"] {
		t.Errorf("unexpected non-c entries: %v", names)
	}
}

func TestFilterSlashCommands_UnknownPrefix(t *testing.T) {
	got := filterSlashCommands("xyz")
	if got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestCompletion_ShowsOnSlash(t *testing.T) {
	m := newTestModel()
	m.input.SetValue("/")
	m.updateCompletion()
	if m.completion == nil {
		t.Fatal("completion should be visible when input is \"/\"")
	}
	if len(m.completion.matches) != len(slashCommands) {
		t.Errorf("expected all commands, got %d", len(m.completion.matches))
	}
}

func TestCompletion_HidesWhenInputEmpty(t *testing.T) {
	m := newTestModel()
	m.input.SetValue("")
	m.updateCompletion()
	if m.completion != nil {
		t.Errorf("completion should be hidden for empty input")
	}
}

func TestCompletion_HidesAfterSpace(t *testing.T) {
	m := newTestModel()
	m.input.SetValue("/clear ")
	m.updateCompletion()
	if m.completion != nil {
		t.Errorf("completion should hide once user starts typing args")
	}
}

func TestCompletion_TabAcceptsSelected(t *testing.T) {
	m := newTestModel()
	m.input.SetValue("/c")
	m.updateCompletion()
	// First match of "/c" is "/clear".
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m2 := updated.(Model)
	if m2.input.Value() != "/clear" {
		t.Errorf("input after tab = %q, want /clear", m2.input.Value())
	}
	if m2.completion != nil {
		t.Errorf("completion should be hidden after accept")
	}
}

func TestCompletion_DownMovesSelection(t *testing.T) {
	m := newTestModel()
	m.input.SetValue("/c")
	m.updateCompletion()
	if m.completion.selected != 0 {
		t.Fatalf("initial selected = %d, want 0", m.completion.selected)
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m2 := updated.(Model)
	if m2.completion.selected != 1 {
		t.Errorf("after down, selected = %d, want 1", m2.completion.selected)
	}
	// Accept the second match — should be /cost.
	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyTab})
	m3 := updated.(Model)
	if m3.input.Value() != "/cost" {
		t.Errorf("input = %q, want /cost", m3.input.Value())
	}
}

func TestCompletion_EscDismisses(t *testing.T) {
	m := newTestModel()
	m.input.SetValue("/c")
	m.updateCompletion()
	if m.completion == nil {
		t.Fatal("popup should be open")
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m2 := updated.(Model)
	if m2.completion != nil {
		t.Errorf("popup should close after Esc")
	}
	// Input text stays.
	if m2.input.Value() != "/c" {
		t.Errorf("input text should be preserved, got %q", m2.input.Value())
	}
}

func TestCompletion_UpWrapsToLast(t *testing.T) {
	m := newTestModel()
	m.input.SetValue("/")
	m.updateCompletion()
	n := len(m.completion.matches)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m2 := updated.(Model)
	if m2.completion.selected != n-1 {
		t.Errorf("up from 0 should wrap to %d, got %d", n-1, m2.completion.selected)
	}
}
