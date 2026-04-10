package config

// Config holds all user-configurable settings for hermes-agent.
// YAML tags mirror the existing Python hermes config.yaml format.
type Config struct {
	Model     string                    `yaml:"model"`
	Providers map[string]ProviderConfig `yaml:"providers"`
	Agent     AgentConfig               `yaml:"agent"`
	Storage   StorageConfig             `yaml:"storage"`
}

// ProviderConfig holds settings for a single LLM provider.
type ProviderConfig struct {
	Provider string `yaml:"provider"`
	BaseURL  string `yaml:"base_url,omitempty"`
	APIKey   string `yaml:"api_key"`
	Model    string `yaml:"model"`
}

// AgentConfig holds engine-level settings.
type AgentConfig struct {
	MaxTurns       int `yaml:"max_turns"`
	GatewayTimeout int `yaml:"gateway_timeout,omitempty"`
}

// StorageConfig holds storage driver settings.
type StorageConfig struct {
	Driver      string `yaml:"driver"`
	SQLitePath  string `yaml:"sqlite_path,omitempty"`
	PostgresURL string `yaml:"postgres_url,omitempty"`
}

// Default returns a Config populated with sensible defaults.
// These match the Python hermes defaults.
func Default() *Config {
	return &Config{
		Model:     "anthropic/claude-opus-4-6",
		Providers: map[string]ProviderConfig{},
		Agent: AgentConfig{
			MaxTurns:       90,
			GatewayTimeout: 1800,
		},
		Storage: StorageConfig{
			Driver: "sqlite",
		},
	}
}
