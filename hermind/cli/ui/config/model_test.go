package configui

import (
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNewModelLoadsDoc(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(p, []byte("model: anthropic/claude\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := NewModel(p)
	if err != nil {
		t.Fatal(err)
	}
	if m.CurrentSection() == "" {
		t.Error("no section selected")
	}
}

func TestTabAdvancesSection(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.yaml")
	os.WriteFile(p, []byte(""), 0o644)
	m, _ := NewModel(p)
	first := m.CurrentSection()
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m2.(Model).CurrentSection() == first {
		t.Error("tab did not advance section")
	}
}

func TestAddProvider(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.yaml")
	os.WriteFile(p, []byte("providers:\n  anthropic:\n    api_key: k\n"), 0o644)
	m, _ := NewModel(p)

	// Navigate to Providers section.
	for m.CurrentSection() != "Providers" {
		m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
		m = m2.(Model)
	}

	// 'a' opens the "new item" editor.
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	m = m2.(Model)

	// Type "openai" and press Enter.
	for _, r := range "openai" {
		m2, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = m2.(Model)
	}
	m2, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = m2.(Model)

	if _, ok := m.doc.Get("providers.openai.provider"); !ok {
		t.Error("new provider not created")
	}
}

func TestDeleteProvider(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.yaml")
	os.WriteFile(p, []byte("providers:\n  openai:\n    api_key: k\n"), 0o644)
	m, _ := NewModel(p)

	for m.CurrentSection() != "Providers" {
		m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
		m = m2.(Model)
	}

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	m = m2.(Model)
	for _, r := range "openai" {
		m2, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = m2.(Model)
	}
	m2, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = m2.(Model)

	if _, ok := m.doc.Get("providers.openai.api_key"); ok {
		t.Error("provider still present after delete")
	}
}

func TestEditStringFieldWritesDoc(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.yaml")
	os.WriteFile(p, []byte("model: old\n"), 0o644)
	m, _ := NewModel(p)

	// Enter edit mode on the "model" field.
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Clear the existing value: 3 backspaces to delete "old".
	for i := 0; i < 3; i++ {
		m2, _ = m2.(Model).Update(tea.KeyMsg{Type: tea.KeyBackspace})
	}

	// Type the replacement.
	for _, r := range "new-model" {
		m2, _ = m2.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	// Commit.
	m2, _ = m2.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter})

	got, _ := m2.(Model).doc.Get("model")
	if got != "new-model" {
		t.Errorf("got %q, want %q", got, "new-model")
	}
}
