package cli

import (
	"fmt"
	"os"

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
			reg, _ := loadSkills()
			all := reg.All()
			if len(all) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no skills installed")
				fmt.Fprintf(cmd.OutOrStdout(), "install skills into %s\n", skills.DefaultHome())
				return nil
			}
			for _, s := range all {
				desc := s.Description
				if len(desc) > 72 {
					desc = desc[:72] + "…"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  %-32s %s\n", s.Name, desc)
			}
			return nil
		},
	}
}

func newSkillsEnableCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "enable <name>",
		Short: "Enable a skill for this profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return skillsPersistActive(args[0], true)
		},
	}
}

func newSkillsDisableCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "disable <name>",
		Short: "Disable a skill for this profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return skillsPersistActive(args[0], false)
		},
	}
}

// loadSkills walks the skills home and returns a populated Registry.
// Errors are logged to stderr but do not fail the command — a missing
// home directory is expected on a fresh install.
func loadSkills() (*skills.Registry, error) {
	reg := skills.NewRegistry()
	l := skills.NewLoader(skills.DefaultHome())
	all, errs := l.Load()
	for _, e := range errs {
		fmt.Fprintf(os.Stderr, "skills: %s: %v\n", e.Path, e.Err)
	}
	for _, s := range all {
		reg.Add(s)
	}
	return reg, nil
}

// skillsPersistActive flips a skill's active state in the profile
// config file. For Phase 10a it just prints — Plan 11c will wire
// persistence into the profile layout.
func skillsPersistActive(name string, enable bool) error {
	action := "enabled"
	if !enable {
		action = "disabled"
	}
	fmt.Printf("skills: %s %s (in-session only; profile persistence pending Phase 11c)\n", name, action)
	return nil
}
