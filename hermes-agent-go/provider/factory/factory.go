// Package factory constructs provider.Provider implementations by name.
package factory

import (
	"fmt"

	"github.com/nousresearch/hermes-agent/config"
	"github.com/nousresearch/hermes-agent/provider"
	"github.com/nousresearch/hermes-agent/provider/anthropic"
	"github.com/nousresearch/hermes-agent/provider/deepseek"
	"github.com/nousresearch/hermes-agent/provider/kimi"
	"github.com/nousresearch/hermes-agent/provider/minimax"
	"github.com/nousresearch/hermes-agent/provider/openai"
	"github.com/nousresearch/hermes-agent/provider/openrouter"
	"github.com/nousresearch/hermes-agent/provider/qwen"
	"github.com/nousresearch/hermes-agent/provider/wenxin"
	"github.com/nousresearch/hermes-agent/provider/zhipu"
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
