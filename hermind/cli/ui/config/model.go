// Package configui is the bubbletea TUI for editing ~/.hermind/config.yaml.
package configui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/odysseythink/hermind/config/editor"
)

// Model is the bubbletea Model for the config screen.
type Model struct {
	doc        *editor.Doc
	sections   []string
	sectionIdx int
	fieldIdx   int
	editing    bool
	ed         *fieldEditor
	dirty      bool
	status     string
}

// NewModel loads path and returns an initial Model.
func NewModel(path string) (Model, error) {
	doc, err := editor.Load(path)
	if err != nil {
		return Model{}, err
	}
	return Model{doc: doc, sections: editor.Sections()}, nil
}

// CurrentSection returns the name of the currently selected section.
func (m Model) CurrentSection() string {
	if len(m.sections) == 0 {
		return ""
	}
	return m.sections[m.sectionIdx]
}

// fieldsInCurrentSection returns the Field list for the selected section.
func (m Model) fieldsInCurrentSection() []editor.Field {
	var out []editor.Field
	for _, f := range editor.Schema() {
		if f.Section == m.CurrentSection() {
			out = append(out, f)
		}
	}
	return out
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd { return nil }

// Run launches the TUI with the given config path.
func Run(path string) error {
	m, err := NewModel(path)
	if err != nil {
		return err
	}
	_, err = tea.NewProgram(m, tea.WithAltScreen()).Run()
	return err
}

// RunFirstRun is like Run but seeds a minimal Doc if path is absent.
func RunFirstRun(path string) error {
	return Run(path) // Load already tolerates missing file.
}
