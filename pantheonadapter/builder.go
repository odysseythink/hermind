package pantheonadapter

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/pantheon/core"
	"github.com/odysseythink/pantheon/extensions/fallback"
	"github.com/odysseythink/pantheon/extensions/retry"
	"github.com/odysseythink/pantheon/providers/anthropic"
	"github.com/odysseythink/pantheon/providers/deepseek"
	"github.com/odysseythink/pantheon/providers/kimi"
	"github.com/odysseythink/pantheon/providers/minimax"
	"github.com/odysseythink/pantheon/providers/openaicompat"
	"github.com/odysseythink/pantheon/providers/openrouter"
	"github.com/odysseythink/pantheon/providers/qwen"
	"github.com/odysseythink/pantheon/providers/wenxin"
	"github.com/odysseythink/pantheon/providers/zhipu"
)

// compatProvider wraps an openaicompat.Client to implement core.Provider.
type compatProvider struct {
	name   string
	client *openaicompat.Client
}

func (p *compatProvider) Name() string { return p.name }

func (p *compatProvider) Models(ctx context.Context) ([]core.Model, error) {
	modelsPath := "/v1/models"
	if strings.HasSuffix(p.client.BaseURL, "/v1") {
		modelsPath = "/models"
	}

	req, err := http.NewRequestWithContext(ctx, "GET", p.client.BaseURL+modelsPath, nil)
	if err != nil {
		return nil, fmt.Errorf("list models request: %w", err)
	}
	if p.client.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.client.APIKey)
	}
	for k, v := range p.client.Headers {
		req.Header.Set(k, v)
	}

	httpClient := p.client.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("list models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list models: status %d", resp.StatusCode)
	}

	var body struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("decode list models: %w", err)
	}

	models := make([]core.Model, 0, len(body.Data))
	for _, m := range body.Data {
		models = append(models, core.Model{ID: m.ID})
	}
	return models, nil
}

func (p *compatProvider) LanguageModel(ctx context.Context, modelID string) (core.LanguageModel, error) {
	return &compatModel{
		provider: p,
		client:   p.client,
		model:    modelID,
	}, nil
}

// compatModel wraps an openaicompat.Client to implement core.LanguageModel.
type compatModel struct {
	provider *compatProvider
	client   *openaicompat.Client
	model    string
}

func (m *compatModel) Provider() string { return m.provider.name }
func (m *compatModel) Model() string    { return m.model }

func (m *compatModel) Generate(ctx context.Context, req *core.Request) (*core.Response, error) {
	return m.client.ChatCompletion(ctx, m.model, req)
}

func (m *compatModel) Stream(ctx context.Context, req *core.Request) (core.StreamResponse, error) {
	return m.client.ChatCompletionStream(ctx, m.model, req), nil
}

func (m *compatModel) GenerateObject(ctx context.Context, req *core.ObjectRequest) (*core.ObjectResponse, error) {
	coreReq := &core.Request{
		Messages:        req.Messages,
		SystemPrompt:    req.SystemPrompt,
		MaxTokens:       req.MaxTokens,
		Temperature:     req.Temperature,
		ProviderOptions: req.ProviderOptions,
	}
	if req.Mode == core.ObjectModeAuto || req.Mode == core.ObjectModeJSON {
		coreReq.ResponseFormat = &core.ResponseFormat{
			Type:       core.ResponseFormatTypeJSONSchema,
			JSONSchema: req.Schema,
		}
	} else if req.Mode == core.ObjectModeTool {
		coreReq.Tools = []core.ToolDefinition{{
			Name:        "generate_object",
			Description: "Generate the requested object",
			Parameters:  req.Schema,
		}}
		coreReq.ToolChoice = core.ToolChoice{Mode: core.ToolChoiceModeRequired, Name: "generate_object"}
	} else if req.Mode == core.ObjectModeText {
		coreReq.ResponseFormat = &core.ResponseFormat{Type: core.ResponseFormatTypeText}
	}

	resp, err := m.Generate(ctx, coreReq)
	if err != nil {
		return nil, err
	}
	return openaicompat.ExtractObjectResponse(resp, m.model)
}

// withBaseURL returns a slice containing the base URL option when url is non-empty.
func withBaseURL[T any](setter func(string) T, url string) []T {
	if url != "" {
		return []T{setter(url)}
	}
	return nil
}

// buildProvider creates a pantheon core.Provider from a hermind ProviderConfig.
func buildProvider(cfg config.ProviderConfig) (core.Provider, error) {
	switch strings.ToLower(cfg.Provider) {
	case "openai":
		baseURL := cfg.BaseURL
		if baseURL == "" {
			baseURL = "https://api.openai.com"
		}
		client := openaicompat.NewClient(baseURL, cfg.APIKey)
		if strings.HasSuffix(baseURL, "/v1") {
			client.ChatCompletionPath = "/chat/completions"
		}
		return &compatProvider{name: cfg.Provider, client: client}, nil
	case "anthropic":
		return anthropic.New(cfg.APIKey, withBaseURL(anthropic.WithBaseURL, cfg.BaseURL)...)
	case "openrouter":
		return openrouter.New(cfg.APIKey, withBaseURL(openrouter.WithBaseURL, cfg.BaseURL)...)
	case "deepseek":
		return deepseek.New(cfg.APIKey, withBaseURL(deepseek.WithBaseURL, cfg.BaseURL)...)
	case "qwen":
		return qwen.New(cfg.APIKey, withBaseURL(qwen.WithBaseURL, cfg.BaseURL)...)
	case "zhipu", "glm":
		return zhipu.New(cfg.APIKey)
	case "kimi", "moonshot":
		return kimi.New(cfg.APIKey, withBaseURL(kimi.WithBaseURL, cfg.BaseURL)...)
	case "minimax":
		return minimax.New(cfg.APIKey, withBaseURL(minimax.WithBaseURL, cfg.BaseURL)...)
	case "wenxin", "ernie":
		return wenxin.New(cfg.APIKey, withBaseURL(wenxin.WithBaseURL, cfg.BaseURL)...)
	case "openaicompatible":
		client := openaicompat.NewClient(cfg.BaseURL, cfg.APIKey)
		if strings.HasSuffix(cfg.BaseURL, "/v1") {
			client.ChatCompletionPath = "/chat/completions"
		}
		return &compatProvider{name: cfg.Provider, client: client}, nil
	default:
		return nil, fmt.Errorf("unknown provider: %s", cfg.Provider)
	}
}

// BuildProvider builds a pantheon core.Provider from a hermind ProviderConfig.
func BuildProvider(cfg config.ProviderConfig) (core.Provider, error) {
	return buildProvider(cfg)
}

// BuildModel builds a pantheon core.LanguageModel from a hermind ProviderConfig.
func BuildModel(ctx context.Context, cfg config.ProviderConfig) (core.LanguageModel, error) {
	provider, err := buildProvider(cfg)
	if err != nil {
		return nil, err
	}
	return provider.LanguageModel(ctx, cfg.Model)
}

// BuildPrimaryModel builds the primary language model with retry wrapping.
func BuildPrimaryModel(ctx context.Context, cfg config.ProviderConfig) (core.LanguageModel, error) {
	m, err := BuildModel(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return &retry.Model{Inner: m, MaxRetries: 3}, nil
}

// BuildFallbackModel builds a primary model with retry and optional fallback candidates.
func BuildFallbackModel(ctx context.Context, primaryCfg config.ProviderConfig, fallbackCfgs []config.ProviderConfig) (core.LanguageModel, error) {
	primary, err := BuildPrimaryModel(ctx, primaryCfg)
	if err != nil {
		return nil, err
	}

	candidates := []core.LanguageModel{primary}
	for _, cfg := range fallbackCfgs {
		if cfg.APIKey == "" {
			continue
		}
		m, err := BuildModel(ctx, cfg)
		if err != nil {
			return nil, err
		}
		candidates = append(candidates, m)
	}

	if len(candidates) == 1 {
		return primary, nil
	}
	return &fallback.Model{Candidates: candidates}, nil
}
