package memprovider

import (
	"fmt"
	"strings"

	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/provider"
	"github.com/odysseythink/hermind/storage"
	"github.com/odysseythink/hermind/tool/embedding"
)

// FactoryOption customizes how memprovider.New builds a Provider.
// Only the holographic backend currently uses these; other providers
// are configured via config.MemoryConfig alone.
type FactoryOption func(*factoryOptions)

type factoryOptions struct {
	storage   storage.Storage
	llm       provider.Provider
	embedder  embedding.Embedder
	skillsCfg *config.SkillsConfig
}

// WithStorage injects a shared storage.Storage into the factory so
// local-backed providers (holographic) can reuse the same SQLite store
// as the built-in memory tool.
func WithStorage(s storage.Storage) FactoryOption {
	return func(o *factoryOptions) { o.storage = s }
}

// WithLLM injects an LLM provider into the factory for MetaClaw.
func WithLLM(p provider.Provider) FactoryOption {
	return func(o *factoryOptions) { o.llm = p }
}

// WithEmbedder injects an embedder into the factory for MetaClaw.
func WithEmbedder(e embedding.Embedder) FactoryOption {
	return func(o *factoryOptions) { o.embedder = e }
}

// WithSkillsConfig injects a SkillsConfig into the factory for MetaClaw.
func WithSkillsConfig(cfg *config.SkillsConfig) FactoryOption {
	return func(o *factoryOptions) { o.skillsCfg = cfg }
}

// New builds the active external memory provider from configuration.
// Returns (nil, nil) when no provider is configured. Returns an error
// when the provider name is unknown or the selected provider has no
// credentials / dependencies.
func New(cfg config.MemoryConfig, opts ...FactoryOption) (Provider, error) {
	fo := &factoryOptions{}
	for _, opt := range opts {
		opt(fo)
	}

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
	case "hindsight":
		if cfg.Hindsight.APIKey == "" {
			return nil, fmt.Errorf("memprovider: hindsight requires api_key")
		}
		return NewHindsight(cfg.Hindsight), nil
	case "retaindb":
		if cfg.RetainDB.APIKey == "" {
			return nil, fmt.Errorf("memprovider: retaindb requires api_key")
		}
		return NewRetainDB(cfg.RetainDB), nil
	case "openviking":
		return NewOpenViking(cfg.OpenViking), nil
	case "byterover":
		return NewByterover(cfg.Byterover), nil
	case "holographic":
		if fo.storage == nil {
			return nil, fmt.Errorf("memprovider: holographic requires storage (pass WithStorage)")
		}
		return NewHolographic(fo.storage), nil
	case "metaclaw":
		if fo.storage == nil {
			return nil, fmt.Errorf("memprovider: metaclaw requires storage (pass WithStorage)")
		}
		return NewMetaClaw(fo.storage, fo.llm, fo.embedder, fo.skillsCfg), nil
	default:
		return nil, fmt.Errorf("memprovider: unknown provider %q", name)
	}
}
