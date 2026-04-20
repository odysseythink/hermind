package acp

import "encoding/json"

// RegistryOpts controls the generated agent.json content.
type RegistryOpts struct {
	// Version is the hermind version string embedded in the manifest.
	Version string
	// BinaryPath is the command ACP clients should launch.
	// Empty string falls back to "hermind" on $PATH.
	BinaryPath string
	// IconURL is an optional absolute URL to an agent icon.
	IconURL string
	// Name overrides the registered agent name. Empty defaults to
	// "hermind".
	Name string
	// DisplayName overrides the human-friendly display name. Empty
	// defaults to "Hermind".
	DisplayName string
	// Description overrides the agent description. Empty defaults to
	// "Go port of the hermes AI agent framework.".
	Description string
	// Args is the list of arguments passed to the binary when launched
	// by the ACP client. Defaults to ["acp"].
	Args []string
}

// BuildRegistry returns the JSON body of an ACP `.well-known/agent.json`
// registry entry. Writing it to disk is the caller's responsibility.
// The bytes include a trailing newline so they concatenate cleanly.
func BuildRegistry(opts RegistryOpts) []byte {
	name := opts.Name
	if name == "" {
		name = "hermind"
	}
	display := opts.DisplayName
	if display == "" {
		display = "Hermind"
	}
	desc := opts.Description
	if desc == "" {
		desc = "Go port of the hermes AI agent framework."
	}
	cmd := opts.BinaryPath
	if cmd == "" {
		cmd = "hermind"
	}
	args := opts.Args
	if len(args) == 0 {
		args = []string{"acp"}
	}

	entry := map[string]any{
		"schema_version": 1,
		"name":           name,
		"display_name":   display,
		"version":        opts.Version,
		"description":    desc,
		"distribution": map[string]any{
			"type":    "command",
			"command": cmd,
			"args":    args,
		},
	}
	if opts.IconURL != "" {
		entry["icon"] = opts.IconURL
	}
	out, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		// json.MarshalIndent on a plain map never errors in practice,
		// but fall back to a minimal encoding just in case.
		out, _ = json.Marshal(entry)
	}
	return append(out, '\n')
}
