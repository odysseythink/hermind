package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// profileRoot returns ~/.hermind/profiles (or $HERMIND_HOME/profiles).
func profileRoot() string {
	if v := os.Getenv("HERMIND_HOME"); v != "" {
		return filepath.Join(v, "profiles")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".hermind", "profiles")
}

// activeProfileFile remembers which profile was last selected.
func activeProfileFile() string {
	if v := os.Getenv("HERMIND_HOME"); v != "" {
		return filepath.Join(v, "active_profile")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".hermind", "active_profile")
}

// ActiveProfile returns the currently selected profile name. Empty
// string means "use the top-level ~/.hermind layout" (legacy mode).
func ActiveProfile() string {
	if v := strings.TrimSpace(os.Getenv("HERMIND_PROFILE")); v != "" {
		return v
	}
	data, err := os.ReadFile(activeProfileFile())
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func newProfileCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "profile",
		Short: "Manage named configuration profiles",
	}
	cmd.AddCommand(newProfileListCmd())
	cmd.AddCommand(newProfileCreateCmd())
	cmd.AddCommand(newProfileSwitchCmd())
	cmd.AddCommand(newProfileDeleteCmd())
	cmd.AddCommand(newProfileCurrentCmd())
	return cmd
}

func newProfileListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List installed profiles",
		RunE: func(cmd *cobra.Command, args []string) error {
			entries, err := os.ReadDir(profileRoot())
			if err != nil {
				if os.IsNotExist(err) {
					fmt.Fprintln(cmd.OutOrStdout(), "no profiles yet")
					return nil
				}
				return err
			}
			active := ActiveProfile()
			var names []string
			for _, e := range entries {
				if e.IsDir() {
					names = append(names, e.Name())
				}
			}
			sort.Strings(names)
			for _, n := range names {
				marker := "  "
				if n == active {
					marker = "* "
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s%s\n", marker, n)
			}
			return nil
		},
	}
}

func newProfileCreateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "create <name>",
		Short: "Create a new profile directory",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			dir := filepath.Join(profileRoot(), name)
			if _, err := os.Stat(dir); err == nil {
				return fmt.Errorf("profile %q already exists", name)
			}
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return err
			}
			// Seed an empty config.yaml so the profile is usable.
			if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("model: anthropic/claude-opus-4-6\n"), 0o644); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "created profile %s at %s\n", name, dir)
			return nil
		},
	}
}

func newProfileSwitchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "switch <name>",
		Short: "Make <name> the active profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			dir := filepath.Join(profileRoot(), name)
			if _, err := os.Stat(dir); err != nil {
				return fmt.Errorf("profile %q does not exist (run `hermind profile create %s` first)", name, name)
			}
			if err := os.WriteFile(activeProfileFile(), []byte(name+"\n"), 0o644); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "active profile set to %s\n", name)
			return nil
		},
	}
}

func newProfileDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <name>",
		Short: "Remove a profile directory",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := filepath.Join(profileRoot(), args[0])
			if err := os.RemoveAll(dir); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "deleted %s\n", dir)
			return nil
		},
	}
}

func newProfileCurrentCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "current",
		Short: "Show the active profile",
		RunE: func(cmd *cobra.Command, args []string) error {
			active := ActiveProfile()
			if active == "" {
				fmt.Fprintln(cmd.OutOrStdout(), "(default — no profile selected)")
				return nil
			}
			fmt.Fprintln(cmd.OutOrStdout(), active)
			return nil
		},
	}
}
