package memprovider

import (
	"fmt"
	"strings"

	"github.com/nousresearch/hermes-agent/config"
)

// New builds the active external memory provider from configuration.
// Returns (nil, nil) when no provider is configured. Returns an error
// when the provider name is unknown or the selected provider has no
// API key (so typos surface loudly instead of silently doing nothing).
func New(cfg config.MemoryConfig) (Provider, error) {
	name := strings.ToLower(strings.TrimSpace(cfg.Provider))
	if name == "" {
		return nil, nil
	}
	switch name {
	case "honcho":
		if cfg.Honcho.APIKey == "" {
			return nil, fmt.Errorf("memprovider: honcho requires api_key")
		}
		return NewHoncho(cfg.Honcho), nil
	case "mem0":
		if cfg.Mem0.APIKey == "" {
			return nil, fmt.Errorf("memprovider: mem0 requires api_key")
		}
		return NewMem0(cfg.Mem0), nil
	case "supermemory":
		if cfg.Supermemory.APIKey == "" {
			return nil, fmt.Errorf("memprovider: supermemory requires api_key")
		}
		return NewSupermemory(cfg.Supermemory), nil
	default:
		return nil, fmt.Errorf("memprovider: unknown provider %q", name)
	}
}
