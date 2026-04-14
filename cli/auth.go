package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// credentialsFile holds per-provider API keys in a single YAML map.
// Layout:
//
//	anthropic: sk-ant-...
//	openai: sk-...
//	openrouter: sk-or-...
type credentialsFile struct {
	Path    string
	Entries map[string]string
}

func defaultCredentialsPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".hermind", "credentials.yaml")
}

func loadCredentials() (*credentialsFile, error) {
	cf := &credentialsFile{Path: defaultCredentialsPath(), Entries: map[string]string{}}
	data, err := os.ReadFile(cf.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return cf, nil
		}
		return nil, err
	}
	if err := yaml.Unmarshal(data, &cf.Entries); err != nil {
		return nil, err
	}
	return cf, nil
}

func (cf *credentialsFile) save() error {
	if err := os.MkdirAll(filepath.Dir(cf.Path), 0o755); err != nil {
		return err
	}
	buf, err := yaml.Marshal(cf.Entries)
	if err != nil {
		return err
	}
	// 0600 — credentials file, owner only.
	return os.WriteFile(cf.Path, buf, 0o600)
}

// newAuthCmd creates the "hermind auth" subcommand tree.
func newAuthCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage provider credentials",
	}
	cmd.AddCommand(newAuthSetCmd())
	cmd.AddCommand(newAuthRevokeCmd())
	cmd.AddCommand(newAuthListCmd())
	return cmd
}

func newAuthSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <provider> <api-key>",
		Short: "Store an API key for a provider",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cf, err := loadCredentials()
			if err != nil {
				return err
			}
			cf.Entries[args[0]] = args[1]
			if err := cf.save(); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "stored credential for %s in %s\n", args[0], cf.Path)
			return nil
		},
	}
}

func newAuthRevokeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "revoke <provider>",
		Short: "Delete a stored credential",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cf, err := loadCredentials()
			if err != nil {
				return err
			}
			if _, ok := cf.Entries[args[0]]; !ok {
				fmt.Fprintf(cmd.OutOrStdout(), "no credential stored for %s\n", args[0])
				return nil
			}
			delete(cf.Entries, args[0])
			if err := cf.save(); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "revoked %s\n", args[0])
			return nil
		},
	}
}

func newAuthListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List providers with stored credentials",
		RunE: func(cmd *cobra.Command, args []string) error {
			cf, err := loadCredentials()
			if err != nil {
				return err
			}
			if len(cf.Entries) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no credentials stored")
				return nil
			}
			for name := range cf.Entries {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", name)
			}
			return nil
		},
	}
}
