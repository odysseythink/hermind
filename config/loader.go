package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	DefaultConfigFile = "config.yaml"
	DefaultDBFile     = "state.db"
)

// Load reads <instance-root>/config.yaml. Missing file returns defaults.
func Load() (*Config, error) {
	root, err := InstanceRoot()
	if err != nil {
		return nil, err
	}
	return LoadFromPath(filepath.Join(root, DefaultConfigFile))
}

// LoadFromPath reads a specific config file. Missing file returns defaults.
func LoadFromPath(path string) (*Config, error) {
	cfg := Default()

	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		resolveDefaults(cfg)
		return cfg, nil
	}
	if err != nil {
		return nil, fmt.Errorf("config: read %s: %w", path, err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("config: parse %s: %w", path, err)
	}

	resolveDefaults(cfg)
	return cfg, nil
}

// resolveDefaults fills in environment-dependent values after a load.
func resolveDefaults(cfg *Config) {
	if cfg.Storage.SQLitePath == "" {
		if root, err := InstanceRoot(); err == nil {
			cfg.Storage.SQLitePath = filepath.Join(root, DefaultDBFile)
		}
	}
}
