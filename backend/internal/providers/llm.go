package providers

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/mlog"
	"github.com/odysseythink/pantheon/core"
)

// LLMChunk is a single chunk from a streaming LLM response.
type LLMChunk struct {
	TextDelta      string
	ReasoningDelta string
	Usage          *core.Usage
	FinishReason   string
	Err            error
}

// LLMProvider is the interface for LLM streaming.
type LLMProvider interface {
	Stream(ctx context.Context, messages []core.Message, systemPrompt string, temperature *float64) (<-chan LLMChunk, error)
	Complete(ctx context.Context, messages []core.Message, systemPrompt string, temperature *float64) (string, error)
	LanguageModel() core.LanguageModel
}

// providerBuilder creates a Pantheon LanguageModel for a specific provider.
type providerBuilder func(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error)

// providerRegistry maps provider names to their builder functions.
// Populated in builders.go.
var providerRegistry = map[string]providerBuilder{}

// PantheonLLM wraps a Pantheon core.LanguageModel for streaming.
type PantheonLLM struct {
	model    core.LanguageModel
	cfg      *config.Config
	settings map[string]string
	refCount atomic.Int64
}

func buildPantheonLLM(ctx context.Context, cfg *config.Config, settings map[string]string) (*PantheonLLM, error) {
	providerName := resolveProviderName(cfg, settings)
	modelID := ResolveModelID(providerName, cfg, settings)

	mlog.Info("buildPantheonLLM: provider=", providerName, " model=", modelID)

	builder, ok := providerRegistry[providerName]
	if !ok {
		return nil, fmt.Errorf("unsupported LLM provider: %s", providerName)
	}

	model, err := builder(ctx, cfg, settings, modelID)
	if err != nil {
		return nil, fmt.Errorf("create %s provider: %w", providerName, err)
	}

	return &PantheonLLM{model: model, cfg: cfg, settings: settings}, nil
}

// NewLLMProvider creates a Pantheon-based LLM provider.
func NewLLMProvider(cfg *config.Config, settings map[string]string) LLMProvider {
	p, err := buildPantheonLLM(context.Background(), cfg, settings)
	if err != nil {
		mlog.Error("NewLLMProvider: builder failed: ", err)
		return &noopLLM{err: err}
	}
	mlog.Info("NewLLMProvider: created PantheonLLM")
	return p
}

func temperatureFromSettings(settings map[string]string, fallback float64) *float64 {
	v, ok := settings["LLMTemperature"]
	if !ok || v == "" {
		if fallback > 0 {
			return &fallback
		}
		return nil
	}
	t, err := strconv.ParseFloat(v, 64)
	if err != nil || t <= 0 {
		if fallback > 0 {
			return &fallback
		}
		return nil
	}
	return &t
}

func maxTokensFromSettings(settings map[string]string, fallback int) *int {
	v, ok := settings["LLMMaxTokens"]
	if !ok || v == "" {
		if fallback > 0 {
			return &fallback
		}
		return nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		if fallback > 0 {
			return &fallback
		}
		return nil
	}
	return &n
}

// Stream implements LLMProvider by calling Pantheon's streaming API.
func (p *PantheonLLM) Stream(ctx context.Context, messages []core.Message, systemPrompt string, temperature *float64) (<-chan LLMChunk, error) {
	req := &core.Request{
		Messages:     messages,
		SystemPrompt: systemPrompt,
	}
	if temperature != nil {
		req.Temperature = temperature
	} else {
		req.Temperature = temperatureFromSettings(p.settings, p.cfg.LLMTemperature)
	}
	req.MaxTokens = maxTokensFromSettings(p.settings, p.cfg.LLMMaxTokens)

	stream, err := p.model.Stream(ctx, req)
	if err != nil {
		return nil, err
	}

	out := make(chan LLMChunk, 16)
	go func() {
		defer close(out)
		for part, err := range stream {
			select {
			case <-ctx.Done():
				return
			default:
			}
			if err != nil {
				out <- LLMChunk{Err: err}
				return
			}
			out <- LLMChunk{
				TextDelta:      part.TextDelta,
				ReasoningDelta: part.ReasoningDelta,
				Usage:          part.Usage,
				FinishReason:   part.FinishReason,
			}
		}
	}()
	return out, nil
}

// Complete implements LLMProvider with a synchronous (non-streaming) call.
func (p *PantheonLLM) Complete(ctx context.Context, messages []core.Message, systemPrompt string, temperature *float64) (string, error) {
	req := &core.Request{
		Messages:     messages,
		SystemPrompt: systemPrompt,
	}
	if temperature != nil {
		req.Temperature = temperature
	} else {
		req.Temperature = temperatureFromSettings(p.settings, p.cfg.LLMTemperature)
	}
	req.MaxTokens = maxTokensFromSettings(p.settings, p.cfg.LLMMaxTokens)

	stream, err := p.model.Stream(ctx, req)
	if err != nil {
		return "", err
	}

	var result strings.Builder
	for part, err := range stream {
		if err != nil {
			return result.String(), err
		}
		result.WriteString(part.TextDelta)
	}
	return result.String(), nil
}

// LanguageModel returns the underlying Pantheon core.LanguageModel.
func (p *PantheonLLM) LanguageModel() core.LanguageModel { return p.model }

// noopLLM is a fallback provider that returns an error.
type noopLLM struct {
	err error
}

func (n *noopLLM) Stream(ctx context.Context, messages []core.Message, systemPrompt string, temperature *float64) (<-chan LLMChunk, error) {
	return nil, n.err
}

func (n *noopLLM) Complete(ctx context.Context, messages []core.Message, systemPrompt string, temperature *float64) (string, error) {
	return "", n.err
}

func (n *noopLLM) LanguageModel() core.LanguageModel { return nil }

// ManagedLLMProvider is a hot-swappable LLMProvider wrapper.
type ManagedLLMProvider struct {
	mu      sync.RWMutex
	current *PantheonLLM
	cfg     *config.Config
	sysSvc  SettingsReader
}

// NewManagedLLMProvider creates the managed wrapper and performs initial build.
func NewManagedLLMProvider(cfg *config.Config, sysSvc SettingsReader, initialSettings map[string]string) (*ManagedLLMProvider, error) {
	p, err := buildPantheonLLM(context.Background(), cfg, initialSettings)
	if err != nil {
		return nil, fmt.Errorf("initial LLM build failed: %w", err)
	}
	return &ManagedLLMProvider{cfg: cfg, sysSvc: sysSvc, current: p}, nil
}

// Reload rebuilds the provider from current settings.
// Returns an error if the new provider cannot be built; the current provider is kept unchanged.
func (m *ManagedLLMProvider) Reload(ctx context.Context) error {
	settings, err := m.sysSvc.GetAllSettings(ctx)
	if err != nil {
		return fmt.Errorf("load settings: %w", err)
	}

	providerName := resolveProviderName(m.cfg, settings)

	newP, err := buildPantheonLLM(ctx, m.cfg, settings)
	if err != nil {
		return err
	}

	m.mu.Lock()
	old := m.current
	m.current = newP
	m.mu.Unlock()

	apiKey, _ := ResolveAPIKey(providerName, settings, m.cfg)
	mlog.Info("LLM provider switched",
		mlog.String("oldProvider", old.model.Provider()),
		mlog.String("oldModel", old.model.Model()),
		mlog.String("newProvider", newP.model.Provider()),
		mlog.String("newModel", newP.model.Model()),
		mlog.String("keyMask", maskKey(apiKey)),
	)

	if old != nil {
		go m.awaitRelease(old)
	}
	return nil
}

func (m *ManagedLLMProvider) awaitRelease(p *PantheonLLM) {
	for p.refCount.Load() > 0 {
		time.Sleep(50 * time.Millisecond)
	}
	mlog.Debug("old LLM provider released",
		mlog.String("provider", p.model.Provider()),
		mlog.String("model", p.model.Model()),
	)
}

func (m *ManagedLLMProvider) acquire() *PantheonLLM {
	m.mu.RLock()
	p := m.current
	m.mu.RUnlock()
	if p == nil {
		return nil
	}
	p.refCount.Add(1)
	return p
}

func (m *ManagedLLMProvider) release(p *PantheonLLM) {
	if p == nil {
		return
	}
	p.refCount.Add(-1)
}

// Stream implements LLMProvider.
func (m *ManagedLLMProvider) Stream(ctx context.Context, messages []core.Message, systemPrompt string, temperature *float64) (<-chan LLMChunk, error) {
	out := make(chan LLMChunk, 16)
	go func() {
		defer close(out)
		p := m.acquire()
		if p == nil {
			out <- LLMChunk{Err: fmt.Errorf("LLM provider not available")}
			return
		}
		defer m.release(p)
		chunks, err := p.Stream(ctx, messages, systemPrompt, temperature)
		if err != nil {
			out <- LLMChunk{Err: err}
			return
		}
		for c := range chunks {
			out <- c
		}
	}()
	return out, nil
}

// Complete implements LLMProvider.
func (m *ManagedLLMProvider) Complete(ctx context.Context, messages []core.Message, systemPrompt string, temperature *float64) (string, error) {
	p := m.acquire()
	if p == nil {
		return "", fmt.Errorf("LLM provider not available")
	}
	defer m.release(p)
	return p.Complete(ctx, messages, systemPrompt, temperature)
}

// LanguageModel returns a proxy that follows live provider updates.
func (m *ManagedLLMProvider) LanguageModel() core.LanguageModel {
	return &languageModelProxy{manager: m}
}

// OnSettingChanged implements providers.SettingObserver.
func (m *ManagedLLMProvider) OnSettingChanged(ctx context.Context, key, value string) error {
	if !isLLMReloadKey(key) {
		return nil
	}
	if err := m.Reload(ctx); err != nil {
		mlog.Error("LLM provider reload failed after setting change", mlog.String("key", key), mlog.Err(err))
		return err
	}
	return nil
}

// isLLMReloadKey determines whether a setting change should trigger an LLM provider rebuild.
func isLLMReloadKey(key string) bool {
	switch key {
	case "LLMProvider", "LLMModel", "LLMApiKey", "LLMTemperature", "LLMMaxTokens":
		return true
	}
	switch {
	case strings.HasSuffix(key, "ModelPref"),
		strings.HasSuffix(key, "ApiKey"),
		strings.HasSuffix(key, "BasePath"),
		strings.HasSuffix(key, "BaseURL"),
		strings.HasSuffix(key, "Endpoint"),
		strings.HasSuffix(key, "Deployment"),
		strings.HasSuffix(key, "ResourceName"):
		return true
	}
	return false
}
