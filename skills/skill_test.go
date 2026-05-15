package skills

import "testing"

func TestRegistryActivate(t *testing.T) {
	r := NewRegistry()
	r.Add(&Skill{Name: "a"})
	r.Add(&Skill{Name: "b"})
	if err := r.Activate("a"); err != nil {
		t.Fatal(err)
	}
	if err := r.Activate("missing"); err == nil {
		t.Error("expected error for missing skill")
	}
	if got := r.Active(); len(got) != 1 || got[0].Name != "a" {
		t.Errorf("active = %v", got)
	}
	r.Deactivate("a")
	if len(r.Active()) != 0 {
		t.Error("expected no active after deactivate")
	}
	if len(r.All()) != 2 {
		t.Error("expected 2 registered")
	}
}

func TestRegistryApplyConfig_ActivatesUnlessDisabled(t *testing.T) {
	r := NewRegistry()
	r.Add(&Skill{Name: "a"})
	r.Add(&Skill{Name: "b"})
	r.Add(&Skill{Name: "c"})

	r.ApplyConfig([]string{"b"})

	active := r.Active()
	if len(active) != 2 {
		t.Fatalf("expected 2 active, got %v", active)
	}
	got := []string{active[0].Name, active[1].Name}
	want := []string{"a", "c"}
	if got[0] != want[0] || got[1] != want[1] {
		t.Errorf("active = %v, want %v", got, want)
	}
}

func TestRegistryApplyConfig_UnknownNameIgnored(t *testing.T) {
	r := NewRegistry()
	r.Add(&Skill{Name: "a"})
	// "ghost" does not exist — must not panic or block "a"
	r.ApplyConfig([]string{"ghost"})
	if got := r.Active(); len(got) != 1 || got[0].Name != "a" {
		t.Errorf("active = %v", got)
	}
}
