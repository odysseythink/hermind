package editor

import (
	"strings"
	"testing"
)

func TestSchemaHasKeyFields(t *testing.T) {
	s := Schema()
	seen := map[string]Field{}
	for _, f := range s {
		seen[f.Path] = f
	}
	for _, p := range []string{
		"model",
		"terminal.backend",
		"storage.sqlite_path",
		"agent.max_turns",
		"agent.compression.threshold",
		"memory.provider",
		"browser.provider",
	} {
		if _, ok := seen[p]; !ok {
			t.Errorf("missing field %q", p)
		}
	}
}

func TestSchemaFieldsHaveLabelsAndSections(t *testing.T) {
	for _, f := range Schema() {
		if strings.TrimSpace(f.Label) == "" {
			t.Errorf("%s: empty Label", f.Path)
		}
		if strings.TrimSpace(f.Section) == "" {
			t.Errorf("%s: empty Section", f.Path)
		}
	}
}

func TestSchemaEnumValidateRejectsUnknown(t *testing.T) {
	for _, f := range Schema() {
		if f.Kind != KindEnum {
			continue
		}
		if f.Validate == nil {
			t.Errorf("%s: enum without Validate", f.Path)
			continue
		}
		if err := f.Validate("___not_a_value___"); err == nil {
			t.Errorf("%s: Validate accepted bogus value", f.Path)
		}
	}
}
