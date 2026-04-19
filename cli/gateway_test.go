package cli

import (
	"strings"
	"testing"

	"github.com/odysseythink/hermind/config"
)

func TestBuildPlatform_KnownTypeReturnsPlatform(t *testing.T) {
	pc := config.PlatformConfig{
		Enabled: true,
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
		Enabled: true,
		Type:    "",
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

func TestBuildPlatform_UnknownTypeReturnsError(t *testing.T) {
	pc := config.PlatformConfig{
		Enabled: true,
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
