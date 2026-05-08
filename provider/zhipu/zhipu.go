// provider/zhipu/zhipu.go
package zhipu

import (
	"context"
	"fmt"
	"time"

	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/provider"
	"github.com/odysseythink/hermind/provider/openaicompat"
)

const (
	defaultBaseURL = "https://open.bigmodel.cn/api/paas/v4"
	jwtTTL         = time.Hour
)

// Zhipu wraps an openaicompat.Client and rotates the Bearer token on every call.
// Zhipu uses JWT-based auth instead of a static API key, so we can't use
// the Client's ExtraHeaders map directly — the token must be regenerated
// before each request.
type Zhipu struct {
	// inner holds the underlying openaicompat.Client. We update its APIKey
	// immediately before each Complete/Stream call via SetAPIKey.
	inner  *openaicompat.Client
	apiKey string
}

// New constructs a Zhipu provider. The api_key must be formatted as
// "<key_id>.<secret>" — this is the format Zhipu AI provides.
func New(cfg config.ProviderConfig) (provider.Provider, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("zhipu: api_key is required")
	}
	// Validate the key format early so misconfiguration surfaces at startup.
	if _, err := signJWT(cfg.APIKey, jwtTTL); err != nil {
		return nil, err
	}

	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	inner, err := openaicompat.NewClient(openaicompat.Config{
		BaseURL:      baseURL,
		APIKey:       "placeholder", // required by NewClient validation but overridden below
		Model:        cfg.Model,
		ProviderName: "zhipu",
	})
	if err != nil {
		return nil, err
	}
	return &Zhipu{inner: inner, apiKey: cfg.APIKey}, nil
}

func (z *Zhipu) Name() string                            { return "zhipu" }
func (z *Zhipu) Available() bool                         { return z.apiKey != "" }
func (z *Zhipu) ModelInfo(m string) *provider.ModelInfo  { return z.inner.ModelInfo(m) }
func (z *Zhipu) EstimateTokens(m, t string) (int, error) { return z.inner.EstimateTokens(m, t) }

// Complete signs a JWT and delegates to the inner client with the signed token.
// We do this by temporarily swapping the inner client's APIKey (thread-safe
// because each RunConversation creates its own Engine and provider set).
func (z *Zhipu) Complete(ctx context.Context, req *provider.Request) (*provider.Response, error) {
	if err := z.signAndInject(); err != nil {
		return nil, err
	}
	return z.inner.Complete(ctx, req)
}

func (z *Zhipu) Stream(ctx context.Context, req *provider.Request) (provider.Stream, error) {
	if err := z.signAndInject(); err != nil {
		return nil, err
	}
	return z.inner.Stream(ctx, req)
}

// signAndInject generates a fresh JWT and stores it where openaicompat.Client
// will find it. openaicompat reads APIKey from its Config — we mutate the
// Config field directly via SetAPIKey.
//
// Concurrency note: a single Zhipu provider instance is not safe for
// concurrent Complete/Stream calls because the inner Client's APIKey is
// mutated. This matches the existing Engine's "single-use per conversation"
// contract.
func (z *Zhipu) signAndInject() error {
	token, err := signJWT(z.apiKey, jwtTTL)
	if err != nil {
		return err
	}
	z.inner.SetAPIKey(token)
	return nil
}

// Compile-time assertion
var _ provider.Provider = (*Zhipu)(nil)
