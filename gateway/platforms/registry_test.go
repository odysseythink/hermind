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
}

func mustBuildStub(name string) func(map[string]string) (gateway.Platform, error) {
	return func(map[string]string) (gateway.Platform, error) {
		return &stubPlatform{name: name}, nil
	}
}

// resetRegistryForTest swaps in a fresh map for the current test and
// restores the original after t finishes.
func resetRegistryForTest(t *testing.T) {
	t.Helper()
	saved := registry
	registry = map[string]Descriptor{}
	t.Cleanup(func() { registry = saved })
}
