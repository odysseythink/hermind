package configui

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/odysseythink/hermind/config/editor"
)

// fieldEditor holds per-field editor state while Model.editing==true.
type fieldEditor struct {
	field   editor.Field
	input   textinput.Model
	enumIdx int
}

func newFieldEditor(f editor.Field, current string) fieldEditor {
	ti := textinput.New()
	ti.SetValue(current)
	ti.CursorEnd()
	ti.Focus()
	if f.Kind == editor.KindSecret {
		ti.EchoMode = textinput.EchoPassword
	}
	fe := fieldEditor{field: f, input: ti}
	if f.Kind == editor.KindEnum {
		for i, v := range f.Enum {
			if v == current {
				fe.enumIdx = i
			}
		}
	}
	return fe
}

// commit converts the editor state back into a value and writes it to doc.
// Returns user-visible error string or empty on success.
func (fe fieldEditor) commit(doc *editor.Doc) string {
	switch fe.field.Kind {
	case editor.KindBool:
		return writeField(doc, fe.field, strings.TrimSpace(fe.input.Value()))
	case editor.KindInt:
		s := strings.TrimSpace(fe.input.Value())
		if _, err := strconv.Atoi(s); err != nil {
			return "not an integer: " + err.Error()
		}
		return writeField(doc, fe.field, s)
	case editor.KindFloat:
		s := strings.TrimSpace(fe.input.Value())
		if _, err := strconv.ParseFloat(s, 64); err != nil {
			return "not a number: " + err.Error()
		}
		return writeField(doc, fe.field, s)
	case editor.KindEnum:
		return writeField(doc, fe.field, fe.field.Enum[fe.enumIdx])
	default: // String / Secret
		return writeField(doc, fe.field, fe.input.Value())
	}
}

func writeField(doc *editor.Doc, f editor.Field, v string) string {
	if f.Validate != nil {
		if err := f.Validate(v); err != nil {
			return err.Error()
		}
	}
	if err := doc.Set(f.Path, v); err != nil {
		return err.Error()
	}
	return ""
}
