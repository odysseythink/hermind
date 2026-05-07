package config

import (
	"github.com/odysseythink/hermind/agent/presence"
)

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
	Benchmark         BenchmarkConfig           `yaml:"benchmark,omitempty"`
	// Proxy controls the Anthropic-compatible /v1/messages endpoint.
	Proxy ProxyConfig `yaml:"proxy,omitempty"`
	// Presence controls the user-presence framework gating background
	// workers (idle consolidator today; future RL hot-swap, embedding
	// rebuilds).
	Presence PresenceConfig `yaml:"presence,omitempty"`
}

// WebConfig holds configuration for the `web_*` tool family.
// Firecrawl (used by web_extract) continues to read FIRECRAWL_API_KEY
// directly and is not represented here.
type WebConfig struct {
	Addr             string       `yaml:"addr,omitempty"`
	Search           SearchConfig `yaml:"search,omitempty"`
	DisableWebFetch  bool         `yaml:"disable_web_fetch,omitempty"`
}

// BenchmarkConfig parameterizes `hermind bench` subcommands.
type BenchmarkConfig struct {
	DatasetSize int          `yaml:"dataset_size,omitempty"` // default 50
	Seed        int64        `yaml:"seed,omitempty"`         // default 42
	JudgeModel  string       `yaml:"judge_model,omitempty"`  // "" = aux, then primary
	OutDir      string       `yaml:"out_dir,omitempty"`      // default ".hermind/benchmark"
	Replay      ReplayConfig `yaml:"replay,omitempty"`
}

// ReplayConfig configures the bench replay subtree's defaults.
type ReplayConfig struct {
	DefaultMode       string `yaml:"default_mode,omitempty"`
	DefaultHistoryCap int    `yaml:"default_history_cap,omitempty"`
	DefaultJudge      string `yaml:"default_judge,omitempty"`
	OutDir            string `yaml:"out_dir,omitempty"`
	JudgeModel        string `yaml:"judge_model,omitempty"`
}

// SearchConfig configures the web_search tool's provider abstraction.
// Provider selects the active backend; empty string enables auto-selection
// by priority (tavily > brave > exa > searxng > bing > DuckDuckGo).
type SearchConfig struct {
	Provider          string                `yaml:"provider,omitempty"`
	Providers         SearchProvidersConfig `yaml:"providers,omitempty"`
	// DefaultNumResults is the fallback when the LLM omits num_results. 0 = 5.
	DefaultNumResults int `yaml:"default_num_results,omitempty"`
	// MaxNumResults caps results per search. 0 = 20.
	MaxNumResults int `yaml:"max_num_results,omitempty"`
}

// DDGProxyConfig holds proxy settings for DuckDuckGo web search requests.
// URL is required to enable proxy; Username and Password are optional for auth.
type DDGProxyConfig struct {
	URL      string `yaml:"url,omitempty"`      // e.g. "http://proxy.corp.com:8080" or "socks5://proxy:1080"
	Username string `yaml:"username,omitempty"` // optional proxy authentication
	Password string `yaml:"password,omitempty"` // optional proxy authentication
}

// SearchProvidersConfig holds per-provider credentials. DuckDuckGo does not
// require credentials and therefore has no sub-node.
type SearchProvidersConfig struct {
	Tavily     ProviderKeyConfig `yaml:"tavily,omitempty"`
	Brave      ProviderKeyConfig `yaml:"brave,omitempty"`
	Exa        ProviderKeyConfig `yaml:"exa,omitempty"`
	DuckDuckGo *DDGProxyConfig   `yaml:"duckduckgo,omitempty"`
	Bing       BingConfig        `yaml:"bing,omitempty"`
	SearXNG    SearXNGConfig     `yaml:"searxng,omitempty"`
}

// ProviderKeyConfig is the shared shape for an API-key-only provider.
type ProviderKeyConfig struct {
	APIKey string `yaml:"api_key,omitempty"`
}

// BingConfig holds Bing-specific search settings.
type BingConfig struct {
	Market string `yaml:"market,omitempty"`
}

// SearXNGConfig holds SearXNG-specific search settings.
type SearXNGConfig struct {
	BaseURL string `yaml:"base_url,omitempty"`
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
	// AutoExtract enables the skills Evolver, which calls the LLM after each
	// conversation to extract reusable skill snippets and save them to the
	// instance's skills/ directory. Default false.
	AutoExtract bool `yaml:"auto_extract,omitempty"`
	// InjectCount is the maximum number of dynamically retrieved skills
	// injected into the system prompt per turn. 0 means inject nothing.
	// Default 3.
	InjectCount int `yaml:"inject_count,omitempty"`
	// GenerationHalfLife controls staleness decay applied to memory
	// reinforcement signals as the skills library evolves. Each "skill
	// generation" gap halves the signal weight every `GenerationHalfLife`
	// generations. Default 5 (set by reader fallback). 0 disables the
	// decay (signals weighted as 1.0).
	GenerationHalfLife int `yaml:"generation_half_life,omitempty"`
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
	Provider    string            `yaml:"provider,omitempty"` // honcho|mem0|supermemory|hindsight|retaindb|openviking|byterover|holographic|metaclaw
	Honcho      HonchoConfig      `yaml:"honcho,omitempty"`
	Mem0        Mem0Config        `yaml:"mem0,omitempty"`
	Supermemory SupermemoryConfig `yaml:"supermemory,omitempty"`
	Hindsight   HindsightConfig   `yaml:"hindsight,omitempty"`
	RetainDB    RetainDBConfig    `yaml:"retaindb,omitempty"`
	OpenViking  OpenVikingConfig  `yaml:"openviking,omitempty"`
	Byterover   ByteroverConfig   `yaml:"byterover,omitempty"`
	Holographic HolographicConfig `yaml:"holographic,omitempty"`
	MetaClaw    MetaClawConfig    `yaml:"metaclaw,omitempty"`
	// ConsolidateIntervalSeconds drives the idle consolidation ticker.
	// Default 900 (15 min). 0 disables the idle path entirely.
	ConsolidateIntervalSeconds int `yaml:"consolidate_interval_seconds,omitempty"`
	// ConsolidateIdleAfterSeconds is the required quiet window before the
	// consolidator runs. Default 300 (5 min).
	ConsolidateIdleAfterSeconds int `yaml:"consolidate_idle_after_seconds,omitempty"`
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

// MetaClawConfig configures the metaclaw provider. The provider uses
// the shared SQLite storage and the main LLM provider for extraction,
// so no external credentials are required — but a few knobs control
// runtime behavior.
type MetaClawConfig struct {
	// InjectCount is the maximum number of recalled memories injected
	// into the system prompt per turn. 0 disables memory injection.
	// Default 3.
	InjectCount int `yaml:"inject_count,omitempty"`

	// BufferEvery, if > 0, calls SyncTurn every N assistant turns inside
	// the main conversation loop rather than only after the loop ends.
	// Reduces the mid-session memory blackout window for long turns.
	// Default 0 (sync only at end).
	BufferEvery int `yaml:"buffer_every,omitempty"`

	// SynergyTokenBudget caps the combined tokens of injected skills
	// and recalled memories. 0 disables the combined cap (skills and
	// memories are each rendered in full). Default 0.
	SynergyTokenBudget int `yaml:"synergy_token_budget,omitempty"`

	// SynergySkillRatio is the fraction of SynergyTokenBudget reserved
	// for skills (memories get 1 - ratio). Default 0.35.
	SynergySkillRatio float64 `yaml:"synergy_skill_ratio,omitempty"`

	// JudgeEnabled turns on the ConversationJudge feedback loop at
	// end-of-conversation. Default false (opt-in).
	JudgeEnabled bool `yaml:"judge_enabled,omitempty"`

	// SummaryEvery controls working_summary refresh cadence (in SyncTurn
	// calls). Default 10; 0 disables.
	SummaryEvery int `yaml:"summary_every,omitempty"`

	// MemoryTypes allow-lists the memory types extraction may emit.
	// Nil or empty means all six types; non-empty is an allow-list.
	MemoryTypes []string `yaml:"memory_types,omitempty"`
}

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
	// PerMessageMaxTokens is the per-message ceiling above which a single
	// message in head/tail is replaced with an aux-LLM summary even when the
	// surrounding messages are compact. Defends against single 200KB+ pastes
	// landing in the protected tail. Default 8000 if 0; set negative to disable.
	PerMessageMaxTokens int `yaml:"per_message_max_tokens,omitempty"`
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
	// MaxTokens caps the output tokens per LLM turn. 0 uses the default (4096).
	MaxTokens int `yaml:"max_tokens,omitempty"`
	// DynamicToolSelection filters the tool list sent to the LLM based on the
	// user query keywords. Reduces token usage for small-context models.
	DynamicToolSelection bool `yaml:"dynamic_tool_selection,omitempty"`
	// HistoryLimit caps the number of past messages loaded into context.
	// 0 means unlimited (default). Set to 20-50 for small-context models.
	HistoryLimit int `yaml:"history_limit,omitempty"`
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

// ProxyConfig controls the Anthropic-compatible /v1/messages proxy
// endpoint. The endpoint is opt-in: it is only mounted when Enabled is true.
type ProxyConfig struct {
	// Enabled exposes /v1/messages at the root path. Default false.
	Enabled bool `yaml:"enabled,omitempty"`
	// KeepAliveSeconds is the SSE keep-alive ping interval in seconds.
	// Default 15. Values <= 0 are clamped to 15 by the server.
	KeepAliveSeconds int `yaml:"keep_alive_seconds,omitempty"`
}

// PresenceConfig configures the user-presence framework that gates
// background workers like the idle consolidator. v1 ships HTTP-idle
// and sleep-window sources; future sub-projects add keyboard-idle and
// calendar busy.
type PresenceConfig struct {
	// HTTPIdleAbsentAfterSeconds is the quiet window before the HTTP-idle
	// source votes Absent. Default 300 (5 min); 0 disables the signal.
	//
	// Migration: when this is 0 (unset), the legacy
	// MemoryConfig.ConsolidateIdleAfterSeconds is consulted instead and
	// a one-time deprecation log is emitted by the consumer.
	HTTPIdleAbsentAfterSeconds int `yaml:"http_idle_absent_after_seconds,omitempty"`

	// SleepWindow gates background work to the configured local-clock
	// window when Enabled. Default disabled.
	SleepWindow presence.SleepWindowConfig `yaml:"sleep_window,omitempty"`
}

func Default() *Config {
	return &Config{
		Model:     "anthropic/claude-opus-4-6",
		Providers: map[string]ProviderConfig{},
		Agent: AgentConfig{
			MaxTurns:       15,
			GatewayTimeout: 1800,
			Compression: CompressionConfig{
				Enabled:             true,
				Threshold:           0.5,
				TargetRatio:         0.2,
				ProtectLast:         20,
				MaxPasses:           3,
				PerMessageMaxTokens: 8000,
			},
		},
		Terminal: TerminalConfig{
			Backend: "local",
		},
		Storage: StorageConfig{
			Driver: "sqlite",
		},
		Memory: MemoryConfig{
			ConsolidateIntervalSeconds:  900,
			ConsolidateIdleAfterSeconds: 300,
			MetaClaw: MetaClawConfig{
				SummaryEvery: 10,
			},
		},
		Benchmark: BenchmarkConfig{DatasetSize: 50, Seed: 42, OutDir: ".hermind/benchmark"},
	}
}

// UnmarshalYAML handles empty options string by converting it to an empty map.
// This allows the frontend to send empty strings for optional text fields.
//
// yaml.v3 does not promote embedded-pointer fields without an explicit
// ",inline" tag, so the old *alias embedding silently dropped "enabled" and
// "type". Use a plain map decode instead to avoid that pitfall.
func (pc *PlatformConfig) UnmarshalYAML(unmarshal func(any) error) error {
	var raw map[string]any
	if err := unmarshal(&raw); err != nil {
		return err
	}
	if enabled, ok := raw["enabled"].(bool); ok {
		pc.Enabled = enabled
	}
	if typ, ok := raw["type"].(string); ok {
		pc.Type = typ
	}
	if opts, found := raw["options"]; found {
		switch v := opts.(type) {
		case string:
			if v == "" {
				pc.Options = map[string]string{}
			}
		case map[string]interface{}:
			m := make(map[string]string, len(v))
			for k, val := range v {
				if s, ok := val.(string); ok {
					m[k] = s
				}
			}
			pc.Options = m
		case map[string]string:
			pc.Options = v
		}
	}
	return nil
}
