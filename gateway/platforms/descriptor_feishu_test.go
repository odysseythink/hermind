package platforms

import (
	"testing"
)

func TestDescriptorFeishu_Fields(t *testing.T) {
	d, ok := Get("feishu")
	if !ok {
		t.Fatal("no descriptor registered for feishu")
	}
	if d.DisplayName != "Feishu / Lark (Self-built App)" {
		t.Errorf("DisplayName = %q", d.DisplayName)
	}
	want := []struct {
		Name     string
		Kind     FieldKind
		Required bool
		Enum     []string
	}{
		{"app_id", FieldString, true, nil},
		{"app_secret", FieldSecret, true, nil},
		{"domain", FieldEnum, true, []string{"feishu", "lark"}},
		{"encrypt_key", FieldSecret, false, nil},
		{"default_chat_id", FieldString, false, nil},
	}
	if len(d.Fields) != len(want) {
		t.Fatalf("Fields len = %d, want %d", len(d.Fields), len(want))
	}
	for i, w := range want {
		got := d.Fields[i]
		if got.Name != w.Name {
			t.Errorf("Fields[%d].Name = %q, want %q", i, got.Name, w.Name)
		}
		if got.Kind != w.Kind {
			t.Errorf("Fields[%d].Kind = %v, want %v", i, got.Kind, w.Kind)
		}
		if got.Required != w.Required {
			t.Errorf("Fields[%d].Required = %v, want %v", i, got.Required, w.Required)
		}
		if w.Enum != nil {
			if len(got.Enum) != len(w.Enum) {
				t.Errorf("Fields[%d].Enum len = %d, want %d", i, len(got.Enum), len(w.Enum))
			} else {
				for j := range w.Enum {
					if got.Enum[j] != w.Enum[j] {
						t.Errorf("Fields[%d].Enum[%d] = %q, want %q", i, j, got.Enum[j], w.Enum[j])
					}
				}
			}
		}
	}
}
