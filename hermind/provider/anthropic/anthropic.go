// provider/anthropic/anthropic.go
package anthropic

import (
	"errors"
	"net/http"
	"time"

	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/provider"
)

const (
	defaultBaseURL       = "https://api.anthropic.com"
	defaultAPIVersion    = "2023-06-01"
	defaultRequestMaxSec = 300
)

// Anthropic is the provider.Provider implementation for Claude models.
type Anthropic struct {
	apiKey  string
	baseURL string
	model   string
	client  *http.Client
}

// New constructs an Anthropic provider from config. Returns an error if
// the API key is missing.
func New(cfg config.ProviderConfig) (provider.Provider, error) {
	if cfg.APIKey == "" {
		return nil, errors.New("anthropic: api_key is required")
	}
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &Anthropic{
		apiKey:  cfg.APIKey,
		baseURL: baseURL,
		model:   cfg.Model,
		client: &http.Client{
			Timeout: defaultRequestMaxSec * time.Second,
		},
	}, nil
}

// Name returns "anthropic".
func (a *Anthropic) Name() string { return "anthropic" }

// Available returns true when an API key is set.
func (a *Anthropic) Available() bool { return a.apiKey != "" }

// ModelInfo returns capabilities for known Claude models. For unknown
// models, returns a conservative default.
func (a *Anthropic) ModelInfo(model string) *provider.ModelInfo {
	// For the minimal plan, all Anthropic models get the same info.
	// Plan 3 will add per-model capability detection.
	return &provider.ModelInfo{
		ContextLength:     200_000,
		MaxOutputTokens:   8_192,
		SupportsVision:    true,
		SupportsTools:     true,
		SupportsStreaming: true,
		SupportsCaching:   true,
		SupportsReasoning: false,
	}
}

// EstimateTokens provides a rough character-based estimate.
// Plan 3 will replace this with a proper tokenizer.
func (a *Anthropic) EstimateTokens(model, text string) (int, error) {
	// ~4 characters per token is the common rule of thumb for English.
	return len(text) / 4, nil
}
