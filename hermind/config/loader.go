package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// DefaultConfigDir is the default location for hermes config files.
const (
	DefaultConfigDir  = "~/.hermes"
	DefaultConfigFile = "config.yaml"
	DefaultDBFile     = "state.db"
)

// Load reads the default config file at ~/.hermes/config.yaml.
// Missing file is not an error — returns defaults.
func Load() (*Config, error) {
	path, err := expandPath(filepath.Join(DefaultConfigDir, DefaultConfigFile))
	if err != nil {
		return nil, err
	}
	return LoadFromPath(path)
}

// LoadFromPath reads a specific config file. Missing file returns defaults.
func LoadFromPath(path string) (*Config, error) {
	cfg := Default()

	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		// Missing config file is OK — defaults apply
		resolveDefaults(cfg)
		return cfg, nil
	}
	if err != nil {
		return nil, fmt.Errorf("config: read %s: %w", path, err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("config: parse %s: %w", path, err)
	}

	if err := expandEnvVars(cfg); err != nil {
		return nil, err
	}
	resolveDefaults(cfg)
	return cfg, nil
}

// expandPath resolves ~ in paths to the user home directory.
// Only handles bare "~" or "~/..." — leaves "~username/..." untouched.
func expandPath(p string) (string, error) {
	if p != "~" && !strings.HasPrefix(p, "~/") {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("config: resolve home: %w", err)
	}
	if p == "~" {
		return home, nil
	}
	return filepath.Join(home, strings.TrimPrefix(p, "~/")), nil
}

// expandEnvVars replaces "env:VAR_NAME" references in api keys with the env value.
func expandEnvVars(cfg *Config) error {
	// Primary providers
	for name, p := range cfg.Providers {
		if strings.HasPrefix(p.APIKey, "env:") {
			varName := strings.TrimPrefix(p.APIKey, "env:")
			if varName == "" {
				return fmt.Errorf("config: provider %q has empty env variable reference", name)
			}
			p.APIKey = os.Getenv(varName)
			cfg.Providers[name] = p
		}
	}
	// Fallback providers
	for i, p := range cfg.FallbackProviders {
		if strings.HasPrefix(p.APIKey, "env:") {
			varName := strings.TrimPrefix(p.APIKey, "env:")
			if varName == "" {
				return fmt.Errorf("config: fallback provider %d has empty env variable reference", i)
			}
			p.APIKey = os.Getenv(varName)
			cfg.FallbackProviders[i] = p
		}
	}
	// Terminal tokens
	if strings.HasPrefix(cfg.Terminal.ModalToken, "env:") {
		varName := strings.TrimPrefix(cfg.Terminal.ModalToken, "env:")
		if varName == "" {
			return fmt.Errorf("config: terminal.modal_token has empty env variable reference")
		}
		cfg.Terminal.ModalToken = os.Getenv(varName)
	}
	if strings.HasPrefix(cfg.Terminal.DaytonaToken, "env:") {
		varName := strings.TrimPrefix(cfg.Terminal.DaytonaToken, "env:")
		if varName == "" {
			return fmt.Errorf("config: terminal.daytona_token has empty env variable reference")
		}
		cfg.Terminal.DaytonaToken = os.Getenv(varName)
	}
	// MCP server env vars
	for name, s := range cfg.MCP.Servers {
		changed := false
		for k, v := range s.Env {
			if strings.HasPrefix(v, "env:") {
				varName := strings.TrimPrefix(v, "env:")
				if varName == "" {
					return fmt.Errorf("config: mcp server %q env var %q has empty env reference", name, k)
				}
				s.Env[k] = os.Getenv(varName)
				changed = true
			}
		}
		if changed {
			cfg.MCP.Servers[name] = s
		}
	}
	return nil
}

// resolveDefaults fills in missing values that depend on environment.
func resolveDefaults(cfg *Config) {
	if cfg.Storage.SQLitePath == "" {
		if home, err := os.UserHomeDir(); err == nil {
			cfg.Storage.SQLitePath = filepath.Join(home, ".hermes", DefaultDBFile)
		}
	}
}
