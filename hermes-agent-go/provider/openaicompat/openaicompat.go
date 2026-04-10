// provider/openaicompat/openaicompat.go
package openaicompat

import (
	"errors"
	"net/http"
	"time"

	"github.com/nousresearch/hermes-agent/config"
	"github.com/nousresearch/hermes-agent/provider"
)

const (
	defaultRequestMaxSec = 300
)

// Config tells the Client how to reach the backend.
// Each wrapper provider (openai, deepseek, etc.) fills this in from its own config.
type Config struct {
	// BaseURL is the absolute URL prefix, e.g. "https://api.openai.com/v1".
	// The Client appends "/chat/completions" to this URL.
	BaseURL string

	// APIKey is sent as "Authorization: Bearer <key>".
	APIKey string

	// Model is the default model to use when a Request does not specify one.
	Model string

	// ExtraHeaders are added to every outgoing request (e.g. OpenRouter routing headers).
	ExtraHeaders map[string]string

	// ProviderName is used for error attribution (e.g. "deepseek").
	ProviderName string

	// Timeout overrides the default HTTP client timeout.
	Timeout time.Duration
}

// Client is an OpenAI-compatible provider client. Wrapper providers
// embed or wrap this type — they do not implement Complete/Stream themselves.
//
// Safe for concurrent use (net/http.Client is safe for concurrent use).
type Client struct {
	cfg    Config
	http   *http.Client
}

// NewClient constructs a Client from Config. Returns an error if BaseURL
// or APIKey are empty.
func NewClient(cfg Config) (*Client, error) {
	if cfg.BaseURL == "" {
		return nil, errors.New("openaicompat: BaseURL is required")
	}
	if cfg.APIKey == "" {
		return nil, errors.New("openaicompat: APIKey is required")
	}
	if cfg.ProviderName == "" {
		cfg.ProviderName = "openaicompat"
	}
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = defaultRequestMaxSec * time.Second
	}
	return &Client{
		cfg: cfg,
		http: &http.Client{Timeout: timeout},
	}, nil
}

// NewFromProviderConfig is a convenience that builds Config from the shared
// config.ProviderConfig shape used by the CLI config file.
// The wrapper provider packages use this to minimize their own code.
func NewFromProviderConfig(providerName, defaultBaseURL string, cfg config.ProviderConfig, extraHeaders map[string]string) (*Client, error) {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return NewClient(Config{
		BaseURL:      baseURL,
		APIKey:       cfg.APIKey,
		Model:        cfg.Model,
		ExtraHeaders: extraHeaders,
		ProviderName: providerName,
	})
}

// Name returns the provider name (for provider.Provider interface).
func (c *Client) Name() string { return c.cfg.ProviderName }

// Available returns true if the client is configured.
func (c *Client) Available() bool { return c.cfg.APIKey != "" && c.cfg.BaseURL != "" }

// ModelInfo returns conservative defaults for any model.
// Wrapper providers can override this per model if they want.
func (c *Client) ModelInfo(model string) *provider.ModelInfo {
	return &provider.ModelInfo{
		ContextLength:     128_000,
		MaxOutputTokens:   4_096,
		SupportsVision:    false, // wrapper providers override
		SupportsTools:     true,
		SupportsStreaming: true,
		SupportsCaching:   false,
		SupportsReasoning: false,
	}
}

// EstimateTokens is a rough character-based estimate.
// Plan 6 replaces this with a per-provider tokenizer.
func (c *Client) EstimateTokens(model, text string) (int, error) {
	return len(text) / 4, nil
}

// Compile-time assertion that *Client satisfies provider.Provider.
var _ provider.Provider = (*Client)(nil)
