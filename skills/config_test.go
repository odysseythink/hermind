package skills

import (
	"reflect"
	"sort"
	"testing"

	_ "github.com/odysseythink/hermind/logging"

	"github.com/odysseythink/hermind/config"
)

func TestDisabledForPlatform_Union(t *testing.T) {
	cfg := config.SkillsConfig{
		Disabled: []string{"always-off"},
		PlatformDisabled: map[string][]string{
			"cli":     {"cli-only"},
			"gateway": {"gateway-only"},
		},
	}
	got := DisabledForPlatform(cfg, "cli")
	sort.Strings(got)
	want := []string{"always-off", "cli-only"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestDisabledForPlatform_EmptyPlatform(t *testing.T) {
	cfg := config.SkillsConfig{Disabled: []string{"a"}}
	got := DisabledForPlatform(cfg, "")
	if len(got) != 1 || got[0] != "a" {
		t.Errorf("got %v", got)
	}
}

func TestWithDisabledUpdate_AddGlobal(t *testing.T) {
	cfg := config.SkillsConfig{Disabled: []string{"x"}}
	got := WithDisabledUpdate(cfg, "y", "", true)
	want := []string{"x", "y"}
	if !reflect.DeepEqual(got.Disabled, want) {
		t.Errorf("got %v, want %v", got.Disabled, want)
	}
}

func TestWithDisabledUpdate_RemoveGlobal(t *testing.T) {
	cfg := config.SkillsConfig{Disabled: []string{"x", "y"}}
	got := WithDisabledUpdate(cfg, "y", "", false)
	if len(got.Disabled) != 1 || got.Disabled[0] != "x" {
		t.Errorf("got %v", got.Disabled)
	}
}

func TestWithDisabledUpdate_AddPlatform(t *testing.T) {
	cfg := config.SkillsConfig{}
	got := WithDisabledUpdate(cfg, "y", "cli", true)
	if len(got.PlatformDisabled["cli"]) != 1 || got.PlatformDisabled["cli"][0] != "y" {
		t.Errorf("got %v", got.PlatformDisabled)
	}
}

func TestWithDisabledUpdate_RemovePlatform(t *testing.T) {
	cfg := config.SkillsConfig{
		PlatformDisabled: map[string][]string{"cli": {"y", "z"}},
	}
	got := WithDisabledUpdate(cfg, "y", "cli", false)
	if len(got.PlatformDisabled["cli"]) != 1 || got.PlatformDisabled["cli"][0] != "z" {
		t.Errorf("got %v", got.PlatformDisabled)
	}
}

func TestWithDisabledUpdate_NoDuplicate(t *testing.T) {
	cfg := config.SkillsConfig{Disabled: []string{"x"}}
	got := WithDisabledUpdate(cfg, "x", "", true)
	if len(got.Disabled) != 1 {
		t.Errorf("expected no duplicate, got %v", got.Disabled)
	}
}
