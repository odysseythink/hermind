package cli

import (
	"testing"

	"github.com/odysseythink/hermind/config"
)

func TestBuildGateway_EmptyConfigBuildsEmptyGateway(t *testing.T) {
	cfg := config.Config{}
	g, err := BuildGateway(BuildGatewayDeps{Config: cfg})
	if err != nil {
		t.Fatalf("BuildGateway returned error: %v", err)
	}
	if g == nil {
		t.Fatal("BuildGateway returned nil")
	}
}

func TestBuildGateway_RegistersEnabledPlatforms(t *testing.T) {
	cfg := config.Config{}
	cfg.Gateway.Platforms = map[string]config.PlatformConfig{
		"telegram": {Enabled: true, Type: "telegram", Options: map[string]string{"token": "t"}},
		"off":      {Enabled: false, Type: "telegram", Options: map[string]string{"token": "t"}},
	}
	g, err := BuildGateway(BuildGatewayDeps{Config: cfg})
	if err != nil {
		t.Fatalf("BuildGateway: %v", err)
	}
	names := g.Names()
	if len(names) != 1 || names[0] != "telegram" {
		t.Errorf("registered platforms = %v, want [telegram]", names)
	}
}

func TestBuildGateway_UnknownTypeReturnsError(t *testing.T) {
	cfg := config.Config{}
	cfg.Gateway.Platforms = map[string]config.PlatformConfig{
		"bad": {Enabled: true, Type: "does-not-exist"},
	}
	if _, err := BuildGateway(BuildGatewayDeps{Config: cfg}); err == nil {
		t.Fatal("BuildGateway(unknown type) returned nil error")
	}
}
