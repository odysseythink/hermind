package descriptor

import "testing"

func TestMCPSectionRegistered(t *testing.T) {
	s, ok := Get("mcp")
	if !ok {
		t.Fatal("Get(\"mcp\") returned ok=false — did mcp.go init() register?")
	}
	if s.GroupID != "advanced" {
		t.Errorf("GroupID = %q, want %q", s.GroupID, "advanced")
	}
	if s.Shape != ShapeKeyedMap {
		t.Errorf("Shape = %v, want ShapeKeyedMap", s.Shape)
	}
	if s.Subkey != "servers" {
		t.Errorf("Subkey = %q, want %q", s.Subkey, "servers")
	}
	if !s.NoDiscriminator {
		t.Error("NoDiscriminator = false, want true")
	}
	if s.Label == "" {
		t.Error("Label is empty")
	}
}

func TestMCPFieldsPresent(t *testing.T) {
	s, _ := Get("mcp")
	byName := map[string]*FieldSpec{}
	for i := range s.Fields {
		byName[s.Fields[i].Name] = &s.Fields[i]
	}

	// Both fields must be present
	for _, name := range []string{"command", "enabled"} {
		if _, ok := byName[name]; !ok {
			t.Errorf("field %q missing", name)
		}
	}

	// No "provider" field — MCP uses NoDiscriminator
	if _, ok := byName["provider"]; ok {
		t.Error("unexpected 'provider' field found — mcp uses NoDiscriminator")
	}

	// Kinds
	if f := byName["command"]; f != nil && f.Kind != FieldString {
		t.Errorf("command.Kind = %s, want string", f.Kind)
	}
	if f := byName["enabled"]; f != nil && f.Kind != FieldBool {
		t.Errorf("enabled.Kind = %s, want bool", f.Kind)
	}
}

func TestMCPRequiredFields(t *testing.T) {
	s, _ := Get("mcp")
	byName := map[string]*FieldSpec{}
	for i := range s.Fields {
		byName[s.Fields[i].Name] = &s.Fields[i]
	}

	// command is required
	if f := byName["command"]; f != nil && !f.Required {
		t.Error("command.Required = false, want true")
	}
	// enabled is optional
	if f := byName["enabled"]; f != nil && f.Required {
		t.Error("enabled.Required = true, want false (optional)")
	}
}
