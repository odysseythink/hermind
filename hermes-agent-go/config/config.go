package config

// Config holds all user-configurable settings for hermes-agent.
// YAML tags mirror the existing Python hermes config.yaml format.
type Config struct {
	Model             string                    `yaml:"model"`
	Providers         map[string]ProviderConfig `yaml:"providers"`
	FallbackProviders []ProviderConfig          `yaml:"fallback_providers,omitempty"`
	Agent             AgentConfig               `yaml:"agent"`
	Terminal          TerminalConfig            `yaml:"terminal"`
	Storage           StorageConfig             `yaml:"storage"`
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

// TerminalConfig holds settings for the terminal (shell exec) backend.
// Only the fields relevant to the selected Backend type are read.
type TerminalConfig struct {
	// Backend selects the execution backend. One of:
	//   "local"       — execute on the host OS via /bin/sh (default)
	//   "docker"      — wrap commands in "docker run --rm -i <image> sh -c ..."
	//   "ssh"         — run commands over SSH to a remote host
	//   "modal"       — call the Modal serverless function API
	//   "daytona"     — call the Daytona workspace exec API
	//   "singularity" — wrap commands in "singularity exec <image> sh -c ..."
	Backend string `yaml:"backend"`

	// Shared: working directory and default timeout (seconds, 0 = backend default)
	Cwd     string `yaml:"cwd,omitempty"`
	Timeout int    `yaml:"timeout,omitempty"`

	// Docker backend
	DockerImage   string   `yaml:"docker_image,omitempty"`
	DockerVolumes []string `yaml:"docker_volumes,omitempty"`

	// SSH backend
	SSHHost string `yaml:"ssh_host,omitempty"`
	SSHUser string `yaml:"ssh_user,omitempty"`
	SSHKey  string `yaml:"ssh_key,omitempty"` // path to private key file

	// Modal backend
	ModalBaseURL string `yaml:"modal_base_url,omitempty"`
	ModalToken   string `yaml:"modal_token,omitempty"`

	// Daytona backend
	DaytonaBaseURL string `yaml:"daytona_base_url,omitempty"`
	DaytonaToken   string `yaml:"daytona_token,omitempty"`

	// Singularity backend
	SingularityImage string `yaml:"singularity_image,omitempty"` // path to .sif file
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
		Terminal: TerminalConfig{
			Backend: "local",
		},
		Storage: StorageConfig{
			Driver: "sqlite",
		},
	}
}
