package descriptor

import "testing"

func TestTerminalSectionRegistered(t *testing.T) {
	s, ok := Get("terminal")
	if !ok {
		t.Fatal(`Get("terminal") returned ok=false`)
	}
	if s.GroupID != "runtime" {
		t.Errorf("GroupID = %q, want %q", s.GroupID, "runtime")
	}

	wantKinds := map[string]FieldKind{
		"backend":           FieldEnum,
		"cwd":               FieldString,
		"timeout":           FieldInt,
		"docker_image":      FieldString,
		"ssh_host":          FieldString,
		"ssh_user":          FieldString,
		"ssh_key":           FieldString,
		"modal_base_url":    FieldString,
		"modal_token":       FieldSecret,
		"daytona_base_url":  FieldString,
		"daytona_token":     FieldSecret,
		"singularity_image": FieldString,
	}
	gotKinds := map[string]FieldKind{}
	for _, f := range s.Fields {
		gotKinds[f.Name] = f.Kind
	}
	for name, kind := range wantKinds {
		got, ok := gotKinds[name]
		if !ok {
			t.Errorf("missing field %q", name)
			continue
		}
		if got != kind {
			t.Errorf("field %q: Kind = %s, want %s", name, got, kind)
		}
	}
}

func TestTerminalBackendIsEnumWithSixChoices(t *testing.T) {
	s, _ := Get("terminal")
	var backend *FieldSpec
	for i := range s.Fields {
		if s.Fields[i].Name == "backend" {
			backend = &s.Fields[i]
			break
		}
	}
	if backend == nil {
		t.Fatal("backend field not found")
	}
	if backend.Kind != FieldEnum {
		t.Fatalf("backend.Kind = %s, want enum", backend.Kind)
	}
	if backend.Default != "local" {
		t.Errorf("backend.Default = %v, want \"local\"", backend.Default)
	}
	want := map[string]bool{
		"local":       true,
		"docker":      true,
		"ssh":         true,
		"modal":       true,
		"daytona":     true,
		"singularity": true,
	}
	for _, v := range backend.Enum {
		delete(want, v)
	}
	if len(want) > 0 {
		t.Errorf("backend.Enum missing %v, got %v", want, backend.Enum)
	}
}

func TestTerminalBackendGating(t *testing.T) {
	s, _ := Get("terminal")
	gate := map[string]string{
		"docker_image":      "docker",
		"ssh_host":          "ssh",
		"ssh_user":          "ssh",
		"ssh_key":           "ssh",
		"modal_base_url":    "modal",
		"modal_token":       "modal",
		"daytona_base_url":  "daytona",
		"daytona_token":     "daytona",
		"singularity_image": "singularity",
	}
	for _, f := range s.Fields {
		want, gated := gate[f.Name]
		if !gated {
			continue
		}
		if f.VisibleWhen == nil {
			t.Errorf("field %q: VisibleWhen is nil", f.Name)
			continue
		}
		if f.VisibleWhen.Field != "backend" {
			t.Errorf("field %q: VisibleWhen.Field = %q, want \"backend\"", f.Name, f.VisibleWhen.Field)
		}
		if f.VisibleWhen.Equals != want {
			t.Errorf("field %q: VisibleWhen.Equals = %v, want %q", f.Name, f.VisibleWhen.Equals, want)
		}
	}
}

func TestTerminalSharedFieldsAreAlwaysVisible(t *testing.T) {
	s, _ := Get("terminal")
	shared := map[string]bool{"cwd": true, "timeout": true}
	for _, f := range s.Fields {
		if !shared[f.Name] {
			continue
		}
		if f.VisibleWhen != nil {
			t.Errorf("field %q: VisibleWhen = %+v, want nil (shared across backends)", f.Name, f.VisibleWhen)
		}
	}
}
