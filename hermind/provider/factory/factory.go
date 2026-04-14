// Package factory constructs provider.Provider implementations by name.
package factory

import (
	"fmt"

	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/provider"
	"github.com/odysseythink/hermind/provider/anthropic"
	"github.com/odysseythink/hermind/provider/deepseek"
	"github.com/odysseythink/hermind/provider/kimi"
	"github.com/odysseythink/hermind/provider/minimax"
	"github.com/odysseythink/hermind/provider/openai"
	"github.com/odysseythink/hermind/provider/openrouter"
	"github.com/odysseythink/hermind/provider/qwen"
	"github.com/odysseythink/hermind/provider/wenxin"
	"github.com/odysseythink/hermind/provider/zhipu"
)

// New constructs a provider by name from a config.ProviderConfig.
// Returns an error if the provider name is unknown or configuration is invalid.
func New(cfg config.ProviderConfig) (provider.Provider, error) {
	switch cfg.Provider {
	case "anthropic":
		return anthropic.New(cfg)
	case "openai":
		return openai.New(cfg)
	case "openrouter":
		return openrouter.New(cfg)
	case "deepseek":
		return deepseek.New(cfg)
	case "qwen":
		return qwen.New(cfg)
	case "zhipu", "glm":
		return zhipu.New(cfg)
	case "kimi", "moonshot":
		return kimi.New(cfg)
	case "minimax":
		return minimax.New(cfg)
	case "wenxin", "ernie":
		return wenxin.New(cfg)
	default:
		return nil, fmt.Errorf("factory: unknown provider %q", cfg.Provider)
	}
}
