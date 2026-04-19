package cli

import (
	"strings"
	"testing"

	"github.com/odysseythink/hermind/config"
)

func TestBuildPlatform_KnownTypeReturnsPlatform(t *testing.T) {
	pc := config.PlatformConfig{
		Type:    "telegram",
		Options: map[string]string{"token": "123:abc"},
	}
	plat, err := buildPlatform("tg_main", pc)
	if err != nil {
		t.Fatalf("buildPlatform returned error: %v", err)
	}
	if plat == nil {
		t.Fatal("buildPlatform returned nil platform")
	}
}

func TestBuildPlatform_EmptyTypeFallsBackToName(t *testing.T) {
	pc := config.PlatformConfig{
		Options: map[string]string{"token": "123:abc"},
	}
	plat, err := buildPlatform("telegram", pc)
	if err != nil {
		t.Fatalf("buildPlatform returned error: %v", err)
	}
	if plat == nil {
		t.Fatal("nil platform for name-fallback case")
	}
}

// TestBuildPlatform_TypeBeatsName guards against a regression where
// buildPlatform silently prefers name over pc.Type. Here pc.Type points
// at a real registered type (telegram) while name is a user-chosen key
// that matches a *different* registered type (slack). If the resolution
// ever inverted, this test would catch it: name resolves to slack, whose
// Build would receive options missing `webhook_url` and return a
// different platform than the telegram bot we asked for.
func TestBuildPlatform_TypeBeatsName(t *testing.T) {
	pc := config.PlatformConfig{
		Type:    "telegram",
		Options: map[string]string{"token": "123:abc"},
	}
	plat, err := buildPlatform("slack", pc)
	if err != nil {
		t.Fatalf("buildPlatform returned error: %v", err)
	}
	if plat == nil {
		t.Fatal("buildPlatform returned nil platform")
	}
	if got := plat.Name(); got != "telegram" {
		t.Errorf("built platform.Name() = %q, want %q (pc.Type must win over name)", got, "telegram")
	}
}

func TestBuildPlatform_UnknownTypeReturnsError(t *testing.T) {
	pc := config.PlatformConfig{
		Type:    "does-not-exist",
		Options: map[string]string{},
	}
	_, err := buildPlatform("any_name", pc)
	if err == nil {
		t.Fatal("buildPlatform(unknown) returned nil error")
	}
	if !strings.Contains(err.Error(), "unknown platform type") {
		t.Errorf("err = %q, want substring 'unknown platform type'", err)
	}
}
