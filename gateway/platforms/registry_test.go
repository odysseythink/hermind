package platforms

import (
	"context"
	"testing"

	"github.com/odysseythink/hermind/gateway"
)

// stubPlatform is a minimal gateway.Platform used only for registry tests.
type stubPlatform struct{ name string }

func (s *stubPlatform) Name() string                                        { return s.name }
func (s *stubPlatform) Run(ctx context.Context, h gateway.MessageHandler) error { <-ctx.Done(); return nil }
func (s *stubPlatform) SendReply(ctx context.Context, out gateway.OutgoingMessage) error { return nil }

func TestRegister_GetReturnsRegistered(t *testing.T) {
	resetRegistryForTest(t)

	d := Descriptor{
		Type:        "stub",
		DisplayName: "Stub",
		Build: func(opts map[string]string) (gateway.Platform, error) {
			return &stubPlatform{name: "stub"}, nil
		},
	}
	Register(d)

	got, ok := Get("stub")
	if !ok {
		t.Fatalf("Get(\"stub\"): ok=false, want true")
	}
	if got.Type != "stub" {
		t.Errorf("got.Type = %q, want %q", got.Type, "stub")
	}
}

func TestGet_MissingReturnsFalse(t *testing.T) {
	resetRegistryForTest(t)
	if _, ok := Get("does-not-exist"); ok {
		t.Fatal("Get of unknown type returned ok=true")
	}
}

func TestAll_ReturnsSortedByType(t *testing.T) {
	resetRegistryForTest(t)
	Register(Descriptor{Type: "zeta", Build: mustBuildStub("zeta")})
	Register(Descriptor{Type: "alpha", Build: mustBuildStub("alpha")})
	Register(Descriptor{Type: "mu", Build: mustBuildStub("mu")})

	got := All()
	if len(got) != 3 {
		t.Fatalf("len(All()) = %d, want 3", len(got))
	}
	want := []string{"alpha", "mu", "zeta"}
	for i, w := range want {
		if got[i].Type != w {
			t.Errorf("All()[%d].Type = %q, want %q", i, got[i].Type, w)
		}
	}
}

func TestRegister_DuplicateTypeOverwrites(t *testing.T) {
	resetRegistryForTest(t)
	Register(Descriptor{Type: "dup", DisplayName: "first", Build: mustBuildStub("dup")})
	Register(Descriptor{Type: "dup", DisplayName: "second", Build: mustBuildStub("dup")})

	got, _ := Get("dup")
	if got.DisplayName != "second" {
		t.Errorf("after overwrite, DisplayName = %q, want %q", got.DisplayName, "second")
	}
	if n := len(All()); n != 1 {
		t.Errorf("len(All()) = %d, want 1 after overwrite", n)
	}
}

func mustBuildStub(name string) func(map[string]string) (gateway.Platform, error) {
	return func(map[string]string) (gateway.Platform, error) {
		return &stubPlatform{name: name}, nil
	}
}

// resetRegistryForTest swaps in a fresh map for the current test and
// restores the original after t finishes.
//
// NOTE: Do not call t.Parallel() in any test that uses this helper —
// it mutates the package-level registry map and is not goroutine-safe.
func resetRegistryForTest(t *testing.T) {
	t.Helper()
	saved := registry
	registry = map[string]Descriptor{}
	t.Cleanup(func() { registry = saved })
}

// TestDescriptorInvariants enforces properties every production
// descriptor must satisfy. The test reads from the real registry, so
// it will be meaningful once tasks 3–5 populate it; today it runs
// over an empty registry, which trivially passes.
func TestDescriptorInvariants(t *testing.T) {
	for _, d := range All() {
		d := d
		t.Run(d.Type, func(t *testing.T) {
			if d.Type == "" {
				t.Fatal("Type is empty")
			}
			if d.DisplayName == "" {
				t.Errorf("DisplayName is empty")
			}
			if d.Build == nil {
				t.Errorf("Build is nil")
			}

			seen := map[string]bool{}
			for _, f := range d.Fields {
				if f.Name == "" {
					t.Errorf("field has empty Name")
				}
				if seen[f.Name] {
					t.Errorf("field %q: duplicate Name", f.Name)
				}
				seen[f.Name] = true
				if f.Kind == FieldUnknown {
					t.Errorf("field %q: Kind left at FieldUnknown sentinel — set it explicitly", f.Name)
				}
				if f.Required && f.Default != nil {
					t.Errorf("field %q: Required && Default != nil", f.Name)
				}
				if f.Kind == FieldEnum && len(f.Enum) == 0 {
					t.Errorf("field %q: FieldEnum with no Enum values", f.Name)
				}
			}
		})
	}
}
