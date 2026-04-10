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
func expandPath(p string) (string, error) {
	if !strings.HasPrefix(p, "~") {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("config: resolve home: %w", err)
	}
	return filepath.Join(home, strings.TrimPrefix(p, "~")), nil
}

// expandEnvVars replaces "env:VAR_NAME" references in api keys with the env value.
func expandEnvVars(cfg *Config) error {
	for name, p := range cfg.Providers {
		if strings.HasPrefix(p.APIKey, "env:") {
			varName := strings.TrimPrefix(p.APIKey, "env:")
			p.APIKey = os.Getenv(varName)
			cfg.Providers[name] = p
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
