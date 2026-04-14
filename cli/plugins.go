package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/odysseythink/hermind/skills"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// enabledSkillsFile holds the list of active skill names per profile.
// Format: a YAML list at ~/.hermind/profiles/<name>/enabled_skills.yaml
// (or ~/.hermind/enabled_skills.yaml for the default profile).
type enabledSkillsFile struct {
	Path  string
	Names []string
}

func enabledSkillsPath() string {
	if v := os.Getenv("HERMIND_HOME"); v != "" {
		return filepath.Join(v, "enabled_skills.yaml")
	}
	home, _ := os.UserHomeDir()
	base := filepath.Join(home, ".hermind")
	if p := ActiveProfile(); p != "" {
		base = filepath.Join(base, "profiles", p)
	}
	return filepath.Join(base, "enabled_skills.yaml")
}

func loadEnabledSkills() (*enabledSkillsFile, error) {
	ef := &enabledSkillsFile{Path: enabledSkillsPath()}
	data, err := os.ReadFile(ef.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return ef, nil
		}
		return nil, err
	}
	if err := yaml.Unmarshal(data, &ef.Names); err != nil {
		return nil, err
	}
	return ef, nil
}

func (ef *enabledSkillsFile) save() error {
	if err := os.MkdirAll(filepath.Dir(ef.Path), 0o755); err != nil {
		return err
	}
	buf, err := yaml.Marshal(ef.Names)
	if err != nil {
		return err
	}
	return os.WriteFile(ef.Path, buf, 0o644)
}

func (ef *enabledSkillsFile) contains(name string) bool {
	for _, n := range ef.Names {
		if n == name {
			return true
		}
	}
	return false
}

func (ef *enabledSkillsFile) add(name string) {
	if !ef.contains(name) {
		ef.Names = append(ef.Names, name)
		sort.Strings(ef.Names)
	}
}

func (ef *enabledSkillsFile) remove(name string) {
	out := ef.Names[:0]
	for _, n := range ef.Names {
		if n != name {
			out = append(out, n)
		}
	}
	ef.Names = out
}

// newPluginsCmd creates the "hermind plugins" subcommand — an alias
// for skill enable/disable with a different verb that matches the
// Python CLI.
func newPluginsCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "plugins",
		Short:   "List and toggle skill plugins",
		Aliases: []string{"plugin"},
	}
	cmd.AddCommand(newPluginsListCmd())
	cmd.AddCommand(newPluginsEnableCmd())
	cmd.AddCommand(newPluginsDisableCmd())
	return cmd
}

func newPluginsListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List available skills with their enable state",
		RunE: func(cmd *cobra.Command, args []string) error {
			ef, err := loadEnabledSkills()
			if err != nil {
				return err
			}
			l := skills.NewLoader(skills.DefaultHome())
			all, _ := l.Load()
			if len(all) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no skills installed")
				return nil
			}
			for _, s := range all {
				marker := "○"
				if ef.contains(s.Name) {
					marker = "●"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  %s %-32s %s\n", marker, s.Name, s.Description)
			}
			return nil
		},
	}
}

func newPluginsEnableCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "enable <name>",
		Short: "Enable a skill for the active profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ef, err := loadEnabledSkills()
			if err != nil {
				return err
			}
			ef.add(args[0])
			if err := ef.save(); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "enabled %s (profile: %s)\n", args[0], profileLabel())
			return nil
		},
	}
}

func newPluginsDisableCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "disable <name>",
		Short: "Disable a skill for the active profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ef, err := loadEnabledSkills()
			if err != nil {
				return err
			}
			ef.remove(args[0])
			if err := ef.save(); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "disabled %s (profile: %s)\n", args[0], profileLabel())
			return nil
		},
	}
}

func profileLabel() string {
	if p := ActiveProfile(); p != "" {
		return p
	}
	return "default"
}
