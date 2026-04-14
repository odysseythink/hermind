package editor

import "fmt"

// Kind enumerates the UI renderer used for a Field.
type Kind int

const (
	KindString Kind = iota
	KindInt
	KindFloat
	KindBool
	KindEnum
	KindSecret
	KindList
)

// Field describes a single editable setting. Both the TUI and Web UI
// render forms from Schema().
type Field struct {
	Path     string
	Label    string
	Help     string
	Kind     Kind
	Enum     []string
	Section  string
	Validate func(any) error
}

func enumValidator(allowed []string) func(any) error {
	return func(v any) error {
		s, ok := v.(string)
		if !ok {
			return fmt.Errorf("expected string, got %T", v)
		}
		for _, a := range allowed {
			if a == s {
				return nil
			}
		}
		return fmt.Errorf("value %q not in %v", s, allowed)
	}
}

// Schema returns the static field catalog. Order determines display order.
func Schema() []Field {
	terminalBackends := []string{"local", "modal", "singularity"}
	storageDrivers := []string{"sqlite"}
	memoryProviders := []string{"", "honcho", "mem0", "supermemory", "hindsight", "retaindb", "openviking", "byterover", "holographic"}
	browserProviders := []string{"", "browserbase", "camofox"}

	return []Field{
		// --- Model ---
		{Path: "model", Label: "Active model", Section: "Model", Kind: KindString,
			Help: "provider/model-name, e.g. anthropic/claude-opus-4-6"},

		// --- Agent ---
		{Path: "agent.max_turns", Label: "Max turns", Section: "Agent", Kind: KindInt,
			Help: "Maximum tool-use iterations per prompt."},
		{Path: "agent.gateway_timeout", Label: "Gateway timeout (s)", Section: "Agent", Kind: KindInt},
		{Path: "agent.compression.enabled", Label: "Compression enabled", Section: "Agent", Kind: KindBool},
		{Path: "agent.compression.threshold", Label: "Compression threshold", Section: "Agent", Kind: KindFloat,
			Help: "Fraction of context length at which compression triggers (0.0-1.0)."},
		{Path: "agent.compression.target_ratio", Label: "Compression target ratio", Section: "Agent", Kind: KindFloat},
		{Path: "agent.compression.protect_last", Label: "Protect last N messages", Section: "Agent", Kind: KindInt},
		{Path: "agent.compression.max_passes", Label: "Max compression passes", Section: "Agent", Kind: KindInt},

		// --- Terminal ---
		{Path: "terminal.backend", Label: "Terminal backend", Section: "Terminal",
			Kind: KindEnum, Enum: terminalBackends,
			Validate: enumValidator(terminalBackends)},

		// --- Storage ---
		{Path: "storage.driver", Label: "Storage driver", Section: "Storage",
			Kind: KindEnum, Enum: storageDrivers,
			Validate: enumValidator(storageDrivers)},
		{Path: "storage.sqlite_path", Label: "SQLite path", Section: "Storage", Kind: KindString},

		// --- Memory ---
		{Path: "memory.provider", Label: "Memory provider", Section: "Memory",
			Kind: KindEnum, Enum: memoryProviders,
			Validate: enumValidator(memoryProviders)},
		{Path: "memory.honcho.api_key", Label: "Honcho API key", Section: "Memory", Kind: KindSecret},
		{Path: "memory.mem0.api_key", Label: "Mem0 API key", Section: "Memory", Kind: KindSecret},
		{Path: "memory.supermemory.api_key", Label: "Supermemory API key", Section: "Memory", Kind: KindSecret},

		// --- Browser ---
		{Path: "browser.provider", Label: "Browser provider", Section: "Browser",
			Kind: KindEnum, Enum: browserProviders,
			Validate: enumValidator(browserProviders)},
		{Path: "browser.browserbase.api_key", Label: "Browserbase API key", Section: "Browser", Kind: KindSecret},
		{Path: "browser.browserbase.project_id", Label: "Browserbase project ID", Section: "Browser", Kind: KindString},
		{Path: "browser.camofox.base_url", Label: "Camofox base URL", Section: "Browser", Kind: KindString},

		// --- Providers (list) ---
		{Path: "providers", Label: "Providers", Section: "Providers", Kind: KindList,
			Help: "Add, remove, or edit LLM provider credentials."},

		// --- MCP (list) ---
		{Path: "mcp.servers", Label: "MCP servers", Section: "MCP", Kind: KindList},
	}
}

// Sections returns distinct Section names in first-seen order.
func Sections() []string {
	seen := map[string]bool{}
	var out []string
	for _, f := range Schema() {
		if seen[f.Section] {
			continue
		}
		seen[f.Section] = true
		out = append(out, f.Section)
	}
	return out
}
