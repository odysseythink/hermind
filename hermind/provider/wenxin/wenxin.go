// provider/wenxin/wenxin.go
package wenxin

import (
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/provider"
)

const (
	defaultOAuthURL    = "https://aip.baidubce.com/oauth/2.0/token"
	defaultChatBaseURL = "https://aip.baidubce.com/rpc/2.0/ai_custom/v1/wenxinworkshop/chat"
	defaultTimeoutSec  = 300
)

// Wenxin is Baidu's ERNIE Bot provider. Uses OAuth 2.0 client credentials
// flow to get an access_token, then calls the chat endpoint with the token
// as a URL parameter.
//
// IMPORTANT: Wenxin does NOT support OpenAI-style tool calls in this plan.
// Tool definitions in the request are silently ignored. If the agent needs
// tools, it should use a different provider as primary and Wenxin only as
// a text-only fallback.
type Wenxin struct {
	apiKey      string // Baidu API Key (client_id)
	secretKey   string // Baidu Secret Key (client_secret)
	model       string // e.g., "ernie-4.0-8k", "ernie-speed", "ernie-lite"
	oauthURL    string
	chatBaseURL string
	http        *http.Client

	tokenMu  sync.Mutex
	token    string
	tokenExp time.Time
}

// New constructs a Wenxin provider. The api_key field in the config holds
// both halves separated by a colon: "<api_key>:<secret_key>". This is the
// convention Baidu uses in their own SDK docs.
func New(cfg config.ProviderConfig) (provider.Provider, error) {
	if cfg.APIKey == "" {
		return nil, errors.New("wenxin: api_key is required (format '<api_key>:<secret_key>')")
	}
	parts := strings.SplitN(cfg.APIKey, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return nil, errors.New("wenxin: api_key must be formatted as '<api_key>:<secret_key>'")
	}
	oauthURL := defaultOAuthURL
	chatBaseURL := defaultChatBaseURL
	if cfg.BaseURL != "" {
		// Allow tests to override — assume both endpoints live under the same host
		chatBaseURL = cfg.BaseURL + "/rpc/2.0/ai_custom/v1/wenxinworkshop/chat"
		oauthURL = cfg.BaseURL + "/oauth/2.0/token"
	}
	model := cfg.Model
	if model == "" {
		model = "ernie-speed"
	}
	return &Wenxin{
		apiKey:      parts[0],
		secretKey:   parts[1],
		model:       model,
		oauthURL:    oauthURL,
		chatBaseURL: chatBaseURL,
		http:        &http.Client{Timeout: defaultTimeoutSec * time.Second},
	}, nil
}

// Name returns "wenxin".
func (w *Wenxin) Name() string { return "wenxin" }

// Available returns true if api_key is configured.
func (w *Wenxin) Available() bool { return w.apiKey != "" && w.secretKey != "" }

// ModelInfo returns conservative defaults for Wenxin models.
func (w *Wenxin) ModelInfo(model string) *provider.ModelInfo {
	return &provider.ModelInfo{
		ContextLength:     8_000, // ernie-speed default; larger for ernie-4.0-128k
		MaxOutputTokens:   2_000,
		SupportsVision:    false,
		SupportsTools:     false, // not supported in Plan 3
		SupportsStreaming: true,
		SupportsCaching:   false,
		SupportsReasoning: false,
	}
}

// EstimateTokens: rough character-based estimate.
func (w *Wenxin) EstimateTokens(model, text string) (int, error) {
	return len(text) / 3, nil // Chinese chars are roughly 1 token, English ~4 chars/token
}

// Compile-time interface check.
var _ provider.Provider = (*Wenxin)(nil)
