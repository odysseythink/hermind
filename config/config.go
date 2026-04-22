package config

// Config holds all user-configurable settings for hermes-agent.
// YAML tags mirror the existing Python hermes config.yaml format.
type Config struct {
	Model             string                    `yaml:"model"`
	Providers         map[string]ProviderConfig `yaml:"providers"`
	FallbackProviders []ProviderConfig          `yaml:"fallback_providers,omitempty"`
	Agent             AgentConfig               `yaml:"agent"`
	Auxiliary         AuxiliaryConfig           `yaml:"auxiliary,omitempty"`
	Terminal          TerminalConfig            `yaml:"terminal"`
	Storage           StorageConfig             `yaml:"storage"`
	MCP               MCPConfig                 `yaml:"mcp,omitempty"`
	Memory            MemoryConfig              `yaml:"memory,omitempty"`
	Browser           BrowserConfig             `yaml:"browser,omitempty"`
	Gateway           GatewayConfig             `yaml:"gateway,omitempty"`
	Cron              CronConfig                `yaml:"cron,omitempty"`
	Logging           LoggingConfig             `yaml:"logging,omitempty"`
	Metrics           MetricsConfig             `yaml:"metrics,omitempty"`
	Tracing           TracingConfig             `yaml:"tracing,omitempty"`
	Skills            SkillsConfig              `yaml:"skills,omitempty"`
	Web               WebConfig                 `yaml:"web,omitempty"`
}

// WebConfig holds configuration for the `web_*` tool family.
// Firecrawl (used by web_extract) continues to read FIRECRAWL_API_KEY
// directly and is not represented here.
type WebConfig struct {
	Search SearchConfig `yaml:"search,omitempty"`
}

// SearchConfig configures the web_search tool's provider abstraction.
// Provider selects the active backend; empty string enables auto-selection
// by priority (tavily > brave > exa > ddg).
type SearchConfig struct {
	Provider  string                `yaml:"provider,omitempty"`
	Providers SearchProvidersConfig `yaml:"providers,omitempty"`
}

// SearchProvidersConfig holds per-provider credentials. DDG does not
// require credentials and therefore has no sub-node.
type SearchProvidersConfig struct {
	Tavily ProviderKeyConfig `yaml:"tavily,omitempty"`
	Brave  ProviderKeyConfig `yaml:"brave,omitempty"`
	Exa    ProviderKeyConfig `yaml:"exa,omitempty"`
}

// ProviderKeyConfig is the shared shape for an API-key-only provider.
type ProviderKeyConfig struct {
	APIKey string `yaml:"api_key,omitempty"`
}

// SkillsConfig records user skill enable/disable selections. It mirrors
// the Python hermes config layout so the same config.yaml works for both.
// An empty struct means "every discovered skill is active".
type SkillsConfig struct {
	// Disabled is the list of skill names disabled on every platform.
	Disabled []string `yaml:"disabled,omitempty"`
	// PlatformDisabled is a per-platform override layered on top of Disabled.
	// Keys match the string passed to the CLI/REPL/gateway startup path
	// (e.g. "cli", "gateway", "cron").
	PlatformDisabled map[string][]string `yaml:"platform_disabled,omitempty"`
}

// CronConfig holds cron scheduler configuration.
type CronConfig struct {
	Jobs []CronJobConfig `yaml:"jobs,omitempty"`
}

// CronJobConfig is a single scheduled prompt.
type CronJobConfig struct {
	Name     string `yaml:"name"`
	Schedule string `yaml:"schedule"` // e.g. "every 5m"
	Prompt   string `yaml:"prompt"`
	Model    string `yaml:"model,omitempty"`
}

// LoggingConfig controls the slog output level.
type LoggingConfig struct {
	Level string `yaml:"level,omitempty"` // debug, info, warn, error
}

// MetricsConfig controls the Prometheus /metrics HTTP server.
type MetricsConfig struct {
	Addr string `yaml:"addr,omitempty"` // e.g. ":9100"; empty disables metrics
}

// TracingConfig controls stdlib-based tracing output.
type TracingConfig struct {
	Enabled bool   `yaml:"enabled,omitempty"`
	File    string `yaml:"file,omitempty"` // path to JSON-lines sink; "" = stderr
}

// GatewayConfig controls the multi-platform gateway.
type GatewayConfig struct {
	Platforms map[string]PlatformConfig `yaml:"platforms,omitempty"`
}

// PlatformConfig is an untyped configuration blob passed to each
// platform adapter. Known keys depend on the adapter (see
// gateway/platforms).
type PlatformConfig struct {
	Enabled bool              `yaml:"enabled"`
	Type    string            `yaml:"type"`
	Options map[string]string `yaml:"options,omitempty"`
}

// BrowserConfig holds browser automation provider configuration.
type BrowserConfig struct {
	Provider    string            `yaml:"provider,omitempty"` // "", "browserbase", "camofox"
	Browserbase BrowserbaseConfig `yaml:"browserbase,omitempty"`
	Camofox     CamofoxConfig     `yaml:"camofox,omitempty"`
}

// BrowserbaseConfig holds Browserbase cloud provider settings.
// Env vars BROWSERBASE_API_KEY / BROWSERBASE_PROJECT_ID take precedence
// over the YAML values at load time (see tool/browser/browserbase.go).
type BrowserbaseConfig struct {
	BaseURL   string `yaml:"base_url,omitempty"`
	APIKey    string `yaml:"api_key,omitempty"`
	ProjectID string `yaml:"project_id,omitempty"`
	KeepAlive bool   `yaml:"keep_alive,omitempty"`
	Proxies   bool   `yaml:"proxies,omitempty"`
}

// CamofoxConfig holds Camofox local browser provider settings.
type CamofoxConfig struct {
	BaseURL            string `yaml:"base_url,omitempty"`            // default http://localhost:9377
	ManagedPersistence bool   `yaml:"managed_persistence,omitempty"` // reuse profiles per user ID
}

// MemoryConfig holds the optional external memory provider configuration.
// At most one provider is active at a time (see tool/memory/memprovider).
type MemoryConfig struct {
	Provider     string             `yaml:"provider,omitempty"` // honcho|mem0|supermemory|hindsight|retaindb|openviking|byterover|holographic
	Honcho       HonchoConfig       `yaml:"honcho,omitempty"`
	Mem0         Mem0Config         `yaml:"mem0,omitempty"`
	Supermemory  SupermemoryConfig  `yaml:"supermemory,omitempty"`
	Hindsight    HindsightConfig    `yaml:"hindsight,omitempty"`
	RetainDB     RetainDBConfig     `yaml:"retaindb,omitempty"`
	OpenViking   OpenVikingConfig   `yaml:"openviking,omitempty"`
	Byterover    ByteroverConfig    `yaml:"byterover,omitempty"`
	Holographic  HolographicConfig  `yaml:"holographic,omitempty"`
}

// RetainDBConfig holds the RetainDB provider configuration.
type RetainDBConfig struct {
	BaseURL string `yaml:"base_url,omitempty"`
	APIKey  string `yaml:"api_key,omitempty"`
	Project string `yaml:"project,omitempty"`
	UserID  string `yaml:"user_id,omitempty"`
}

// OpenVikingConfig holds the OpenViking provider configuration.
type OpenVikingConfig struct {
	Endpoint string `yaml:"endpoint,omitempty"`
	APIKey   string `yaml:"api_key,omitempty"`
}

// ByteroverConfig holds the Byterover CLI wrapper configuration.
// Byterover is driven by a local `brv` CLI; this config only records
// an optional explicit path to the binary and a working directory.
type ByteroverConfig struct {
	BrvPath string `yaml:"brv_path,omitempty"`
	Cwd     string `yaml:"cwd,omitempty"`
}

// HolographicConfig is a placeholder — the holographic provider uses
// the shared SQLite storage so there is no backend URL or key.
type HolographicConfig struct{}

// HindsightConfig holds the Hindsight cloud provider configuration.
type HindsightConfig struct {
	BaseURL string `yaml:"base_url,omitempty"`
	APIKey  string `yaml:"api_key,omitempty"`
	BankID  string `yaml:"bank_id,omitempty"`
	Budget  string `yaml:"budget,omitempty"` // low, mid, high
}

// HonchoConfig holds the Honcho provider configuration.
type HonchoConfig struct {
	BaseURL   string `yaml:"base_url,omitempty"`
	APIKey    string `yaml:"api_key,omitempty"`
	Workspace string `yaml:"workspace,omitempty"`
	Peer      string `yaml:"peer,omitempty"`
}

// Mem0Config holds the Mem0 provider configuration.
type Mem0Config struct {
	BaseURL string `yaml:"base_url,omitempty"`
	APIKey  string `yaml:"api_key,omitempty"`
	UserID  string `yaml:"user_id,omitempty"`
}

// SupermemoryConfig holds the Supermemory provider configuration.
type SupermemoryConfig struct {
	BaseURL string `yaml:"base_url,omitempty"`
	APIKey  string `yaml:"api_key,omitempty"`
	UserID  string `yaml:"user_id,omitempty"`
}

// MCPConfig holds the configured MCP server list.
// Each server is identified by its key in the map.
type MCPConfig struct {
	Servers map[string]MCPServerConfig `yaml:"servers,omitempty"`
}

// MCPServerConfig describes a single MCP server to start on CLI launch.
// Plan 6b supports stdio transport only (subprocess with command + args + env).
// HTTP/SSE transport is deferred.
type MCPServerConfig struct {
	Command string            `yaml:"command"` // e.g. "npx"
	Args    []string          `yaml:"args,omitempty"`
	Env     map[string]string `yaml:"env,omitempty"`
	// Enabled defaults to true if unset. Use false to disable a server
	// without deleting its config block.
	Enabled *bool `yaml:"enabled,omitempty"`
}

// IsEnabled returns true unless Enabled is explicitly false.
func (m MCPServerConfig) IsEnabled() bool {
	if m.Enabled == nil {
		return true
	}
	return *m.Enabled
}

// ProviderConfig holds settings for a single LLM provider.
type ProviderConfig struct {
	Provider string `yaml:"provider"`
	BaseURL  string `yaml:"base_url,omitempty"`
	APIKey   string `yaml:"api_key"`
	Model    string `yaml:"model"`
}

// CompressionConfig controls context compression behavior.
// When the conversation history exceeds Threshold * model context length,
// the Engine summarizes middle messages via the auxiliary provider.
type CompressionConfig struct {
	Enabled     bool    `yaml:"enabled"`      // default true
	Threshold   float64 `yaml:"threshold"`    // default 0.5 (50% of context)
	TargetRatio float64 `yaml:"target_ratio"` // default 0.2 (compress to 20%)
	ProtectLast int     `yaml:"protect_last"` // default 20 messages
	MaxPasses   int     `yaml:"max_passes"`   // default 3
}

// AuxiliaryConfig holds the auxiliary provider used for compression,
// vision summarization, and other secondary tasks.
// If unset, the main provider is used.
type AuxiliaryConfig struct {
	Provider string `yaml:"provider,omitempty"`
	BaseURL  string `yaml:"base_url,omitempty"`
	APIKey   string `yaml:"api_key,omitempty"`
	Model    string `yaml:"model,omitempty"`
}

// AgentConfig holds engine-level settings.
type AgentConfig struct {
	MaxTurns            int               `yaml:"max_turns"`
	GatewayTimeout      int               `yaml:"gateway_timeout,omitempty"`
	Compression         CompressionConfig `yaml:"compression,omitempty"`
	DefaultSystemPrompt string            `yaml:"default_system_prompt,omitempty"`
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
			Compression: CompressionConfig{
				Enabled:     true,
				Threshold:   0.5,
				TargetRatio: 0.2,
				ProtectLast: 20,
				MaxPasses:   3,
			},
		},
		Terminal: TerminalConfig{
			Backend: "local",
		},
		Storage: StorageConfig{
			Driver: "sqlite",
		},
	}
}
