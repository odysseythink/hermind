package skills

import (
	"context"
	"fmt"
	"strings"
)

// RegisterBuiltins registers the standard REPL slash commands:
// /help, /skills, /reset (emits a reset token), /memory, /model,
// /profile. These are intentionally simple — the REPL can treat the
// returned reply text as a synthetic system message.
//
// Callers pass a few callbacks so the builtins don't need to import
// the CLI package directly.
type BuiltinHooks struct {
	// Skills used to fulfil /skills.
	Skills *Registry
	// Reset clears the current session and returns a short confirmation.
	Reset func() error
	// CurrentModel returns the active model string.
	CurrentModel func() string
	// SwitchModel swaps the active model.
	SwitchModel func(name string) error
	// CurrentProfile returns the active profile name.
	CurrentProfile func() string
}

func RegisterBuiltins(reg *SlashRegistry, hooks BuiltinHooks) {
	reg.Register(&SlashCommand{
		Name:        "help",
		Source:      "builtin",
		Description: "Show available slash commands",
		Handler: func(ctx context.Context, args []string) (string, error) {
			var b strings.Builder
			b.WriteString("available commands:\n")
			for _, c := range reg.All() {
				b.WriteString("  /")
				b.WriteString(c.Name)
				if c.Description != "" {
					b.WriteString(" — ")
					b.WriteString(c.Description)
				}
				b.WriteString("\n")
			}
			return b.String(), nil
		},
	})

	reg.Register(&SlashCommand{
		Name:        "skills",
		Source:      "builtin",
		Description: "List installed skills and their status",
		Handler: func(ctx context.Context, args []string) (string, error) {
			if hooks.Skills == nil {
				return "skills registry unavailable", nil
			}
			all := hooks.Skills.All()
			if len(all) == 0 {
				return "no skills installed", nil
			}
			active := make(map[string]bool)
			for _, s := range hooks.Skills.Active() {
				active[s.Name] = true
			}
			var b strings.Builder
			for _, s := range all {
				marker := "○"
				if active[s.Name] {
					marker = "●"
				}
				fmt.Fprintf(&b, "  %s %-32s %s\n", marker, s.Name, s.Description)
			}
			return b.String(), nil
		},
	})

	reg.Register(&SlashCommand{
		Name:        "reset",
		Source:      "builtin",
		Description: "Clear the current session history",
		Handler: func(ctx context.Context, args []string) (string, error) {
			if hooks.Reset == nil {
				return "reset handler not wired", nil
			}
			if err := hooks.Reset(); err != nil {
				return "", err
			}
			return "session cleared", nil
		},
	})

	reg.Register(&SlashCommand{
		Name:        "model",
		Source:      "builtin",
		Description: "Show or switch the active model",
		Handler: func(ctx context.Context, args []string) (string, error) {
			if len(args) == 0 {
				if hooks.CurrentModel == nil {
					return "current model: unknown", nil
				}
				return "current model: " + hooks.CurrentModel(), nil
			}
			if hooks.SwitchModel == nil {
				return "model switching not wired", nil
			}
			if err := hooks.SwitchModel(args[0]); err != nil {
				return "", err
			}
			return "switched to " + args[0], nil
		},
	})

	reg.Register(&SlashCommand{
		Name:        "profile",
		Source:      "builtin",
		Description: "Show active profile",
		Handler: func(ctx context.Context, args []string) (string, error) {
			if hooks.CurrentProfile == nil {
				return "active profile: default", nil
			}
			return "active profile: " + hooks.CurrentProfile(), nil
		},
	})
}

// RegisterSkills registers one slash command per installed skill. Each
// command toggles the skill's active state: running /foo activates the
// foo skill (subsequent agent turns see its body in the system prompt),
// and running /foo off deactivates it. When a skill declares extra
// command aliases via its YAML `commands:` list, those are registered
// too and behave identically.
func RegisterSkills(slash *SlashRegistry, skillsReg *Registry) {
	register := func(cmdName string, skill *Skill) {
		slash.Register(&SlashCommand{
			Name:        cmdName,
			Source:      skill.Name,
			Description: fmt.Sprintf("[%s] %s", skill.Name, skill.Description),
			Handler: func(ctx context.Context, args []string) (string, error) {
				if len(args) > 0 && (args[0] == "off" || args[0] == "deactivate") {
					skillsReg.Deactivate(skill.Name)
					return fmt.Sprintf("skill %s: deactivated", skill.Name), nil
				}
				if err := skillsReg.Activate(skill.Name); err != nil {
					return "", err
				}
				return fmt.Sprintf("skill %s: activated — its guidance is now in effect", skill.Name), nil
			},
		})
	}

	for _, s := range skillsReg.All() {
		register(s.Name, s)
		for _, cmdName := range s.Commands {
			if cmdName == s.Name {
				continue
			}
			register(cmdName, s)
		}
	}
}
