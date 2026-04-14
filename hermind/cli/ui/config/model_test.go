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

func TestEditStringFieldWritesDoc(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.yaml")
	os.WriteFile(p, []byte("model: old\n"), 0o644)
	m, _ := NewModel(p)
	// navigate to "model" field (first field of "Model" section, default)
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // enter edit
	for _, r := range "new-model" {
		m2, _ = m2.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m2, _ = m2.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter}) // commit
	got, _ := m2.(Model).doc.Get("model")
	if got != "new-model" {
		t.Errorf("got %q, want %q", got, "new-model")
	}
}
