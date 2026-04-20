// Package factory constructs provider.Provider implementations by name.
package factory

import (
	"fmt"

	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/provider"
	"github.com/odysseythink/hermind/provider/anthropic"
	"github.com/odysseythink/hermind/provider/bedrock"
	"github.com/odysseythink/hermind/provider/copilot"
	"github.com/odysseythink/hermind/provider/deepseek"
	"github.com/odysseythink/hermind/provider/kimi"
	"github.com/odysseythink/hermind/provider/minimax"
	"github.com/odysseythink/hermind/provider/openai"
	"github.com/odysseythink/hermind/provider/openrouter"
	"github.com/odysseythink/hermind/provider/qwen"
	"github.com/odysseythink/hermind/provider/wenxin"
	"github.com/odysseythink/hermind/provider/zhipu"
)

// constructor builds a provider.Provider from a config.ProviderConfig.
type constructor func(cfg config.ProviderConfig) (provider.Provider, error)

// primary maps the canonical provider name to its constructor. Drives both
// New (resolution) and Types (dropdown enumeration). Keep this the single
// source of truth — don't add a case to New without adding an entry here.
var primary = map[string]constructor{
	"anthropic":  anthropic.New,
	"openai":     openai.New,
	"openrouter": openrouter.New,
	"deepseek":   deepseek.New,
	"qwen":       qwen.New,
	"zhipu":      zhipu.New,
	"kimi":       kimi.New,
	"minimax":    minimax.New,
	"wenxin":     wenxin.New,
	"copilot":    copilot.New,
	"bedrock":    bedrock.New,
}

// aliases maps alternate provider names to their canonical name. These resolve
// in New but are excluded from Types so the UI dropdown shows one entry per
// provider, not both the primary and its aliases.
var aliases = map[string]string{
	"glm":      "zhipu",
	"moonshot": "kimi",
	"ernie":    "wenxin",
}

// New constructs a provider by name from a config.ProviderConfig.
// Returns an error if the provider name is unknown or configuration is invalid.
func New(cfg config.ProviderConfig) (provider.Provider, error) {
	name := cfg.Provider
	if alt, ok := aliases[name]; ok {
		name = alt
	}
	ctor, ok := primary[name]
	if !ok {
		return nil, fmt.Errorf("factory: unknown provider %q", cfg.Provider)
	}
	return ctor(cfg)
}
