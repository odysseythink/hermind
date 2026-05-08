package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/skills"
	"github.com/spf13/cobra"
)

// newSkillsCmd creates the "hermind skills" subcommand tree.
func newSkillsCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skills",
		Short: "Manage hermind skill packages",
	}
	cmd.AddCommand(newSkillsListCmd(app))
	cmd.AddCommand(newSkillsEnableCmd(app))
	cmd.AddCommand(newSkillsDisableCmd(app))
	return cmd
}

func newSkillsListCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List installed skills",
		RunE: func(cmd *cobra.Command, args []string) error {
			reg, _ := loadSkills(app)
			all := reg.All()
			if len(all) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no skills installed")
				fmt.Fprintf(cmd.OutOrStdout(), "install skills into %s\n", skills.DefaultHome())
				return nil
			}
			var cfgSkills config.SkillsConfig
			if app != nil && app.Config != nil {
				cfgSkills = app.Config.Skills
			}
			disabled := skills.DisabledForPlatform(cfgSkills, "")
			off := make(map[string]struct{}, len(disabled))
			for _, n := range disabled {
				off[n] = struct{}{}
			}
			for _, s := range all {
				marker := "●"
				if _, d := off[s.Name]; d {
					marker = "○"
				}
				desc := s.Description
				if len(desc) > 68 {
					desc = desc[:68] + "…"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  %s %-32s %s\n", marker, s.Name, desc)
			}
			return nil
		},
	}
}

func newSkillsEnableCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "enable <name>",
		Short: "Enable a skill (removes it from config.skills.disabled)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return skillsPersistActiveWithOut(app, args[0], true, cmd.ErrOrStderr())
		},
	}
}

func newSkillsDisableCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "disable <name>",
		Short: "Disable a skill (adds it to config.skills.disabled)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return skillsPersistActiveWithOut(app, args[0], false, cmd.ErrOrStderr())
		},
	}
}

// loadSkills walks the skills home and applies config-driven disablement.
// Errors during skill parsing are logged to stderr but do not fail the
// command — a missing home directory is expected on fresh installs.
func loadSkills(app *App) (*skills.Registry, error) {
	reg := skills.NewRegistry()
	l := skills.NewLoader(skills.DefaultHome())
	all, errs := l.Load()
	for _, e := range errs {
		fmt.Fprintf(os.Stderr, "skills: %s: %v\n", e.Path, e.Err)
	}
	for _, s := range all {
		reg.Add(s)
	}
	if app != nil && app.Config != nil {
		// "" = no platform override; CLI-specific startup paths
		// (REPL, gateway, cron) can call ApplyConfig themselves with
		// a non-empty platform name.
		reg.ApplyConfig(skills.DisabledForPlatform(app.Config.Skills, ""))
	}
	return reg, nil
}

// skillsPersistActive updates the config file to reflect a skill's
// new enable/disable state. enable=true removes the skill from
// config.skills.disabled; enable=false adds it.
//
// Returns an error if the skill is unknown (no SKILL.md under the
// skills home directory matches).
func skillsPersistActive(app *App, name string, enable bool) error {
	return skillsPersistActiveWithOut(app, name, enable, os.Stderr)
}

// skillsPersistActiveWithOut is the io.Writer-injectable variant used
// by tests. Non-test callers should use skillsPersistActive.
func skillsPersistActiveWithOut(app *App, name string, enable bool, stderr io.Writer) error {
	reg, _ := loadSkills(app)
	if reg.Get(name) == nil {
		return fmt.Errorf("skills: unknown skill %q (not found under %s)", name, skills.DefaultHome())
	}

	if app == nil || app.Config == nil {
		// Fallback to legacy behavior when there is no config to write to.
		action := "enabled"
		if !enable {
			action = "disabled"
		}
		fmt.Fprintf(stderr, "skills: %s %s (in-session only; no config loaded)\n", name, action)
		return nil
	}

	app.Config.Skills = skills.WithDisabledUpdate(app.Config.Skills, name, "", !enable)

	if app.ConfigPath == "" {
		return fmt.Errorf("skills: persist config: no config path on app")
	}
	if err := config.SaveToPath(app.ConfigPath, app.Config); err != nil {
		return fmt.Errorf("skills: persist config: %w", err)
	}

	action := "enabled"
	if !enable {
		action = "disabled"
	}
	fmt.Fprintf(stderr, "skills: %s %s (saved to %s)\n", name, action, app.ConfigPath)
	return nil
}
