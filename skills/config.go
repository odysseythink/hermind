package skills

import (
	"sort"

	"github.com/odysseythink/hermind/config"
)

// DisabledForPlatform returns the union of global Disabled and the
// platform-specific override list, deduplicated. Order is not
// guaranteed — callers that care should sort.
func DisabledForPlatform(cfg config.SkillsConfig, platform string) []string {
	seen := make(map[string]struct{}, len(cfg.Disabled))
	for _, n := range cfg.Disabled {
		seen[n] = struct{}{}
	}
	if platform != "" {
		for _, n := range cfg.PlatformDisabled[platform] {
			seen[n] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for n := range seen {
		out = append(out, n)
	}
	return out
}

// WithDisabledUpdate returns a new SkillsConfig value with the given
// skill's disabled state flipped. If platform is "", the global
// Disabled list is edited; otherwise the platform-specific override
// list is edited.
//
// disabled=true → the skill gets added to the list (if not present).
// disabled=false → the skill gets removed (if present).
func WithDisabledUpdate(cfg config.SkillsConfig, name, platform string, disabled bool) config.SkillsConfig {
	out := cfg
	if platform == "" {
		out.Disabled = updateStringList(cfg.Disabled, name, disabled)
		return out
	}
	if out.PlatformDisabled == nil {
		out.PlatformDisabled = map[string][]string{}
	} else {
		// shallow copy of the map so callers can't see partial mutation
		cp := make(map[string][]string, len(out.PlatformDisabled))
		for k, v := range out.PlatformDisabled {
			cp[k] = append([]string(nil), v...)
		}
		out.PlatformDisabled = cp
	}
	out.PlatformDisabled[platform] = updateStringList(out.PlatformDisabled[platform], name, disabled)
	if len(out.PlatformDisabled[platform]) == 0 {
		delete(out.PlatformDisabled, platform)
	}
	return out
}

func updateStringList(list []string, name string, add bool) []string {
	seen := false
	out := make([]string, 0, len(list)+1)
	for _, n := range list {
		if n == name {
			seen = true
			if add {
				out = append(out, n) // keep single copy
			}
			continue
		}
		out = append(out, n)
	}
	if add && !seen {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}
