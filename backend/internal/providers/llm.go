package providers

import (
	"context"
	"fmt"
	"strings"

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

// PantheonLLM wraps a Pantheon core.LanguageModel for streaming.
type PantheonLLM struct {
	model core.LanguageModel
	cfg   *config.Config
}

// providerBuilder creates a Pantheon LanguageModel for a specific provider.
type providerBuilder func(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error)

// providerRegistry maps provider names to their builder functions.
// Populated in builders.go.
var providerRegistry = map[string]providerBuilder{}

// NewLLMProvider creates a Pantheon-based LLM provider.
// settings is a map of DB system_settings (e.g. LLMProvider, OllamaLLMBasePath, etc.)
func NewLLMProvider(cfg *config.Config, settings map[string]string) LLMProvider {
	providerName := resolveProviderName(cfg, settings)
	modelID := resolveModelID(providerName, cfg, settings)

	mlog.Info("NewLLMProvider: provider=", providerName, " model=", modelID)

	builder, ok := providerRegistry[providerName]
	if !ok {
		mlog.Error("NewLLMProvider: unsupported provider ", providerName)
		return &noopLLM{err: fmt.Errorf("unsupported LLM provider: %s", providerName)}
	}

	model, err := builder(context.Background(), cfg, settings, modelID)
	if err != nil {
		mlog.Error("NewLLMProvider: builder failed for ", providerName, ": ", err)
		return &noopLLM{err: fmt.Errorf("create %s provider: %w", providerName, err)}
	}

	mlog.Info("NewLLMProvider: created PantheonLLM (", providerName, ") with model ", modelID)
	return &PantheonLLM{model: model, cfg: cfg}
}

// Stream implements LLMProvider by calling Pantheon's streaming API.
func (p *PantheonLLM) Stream(ctx context.Context, messages []core.Message, systemPrompt string, temperature *float64) (<-chan LLMChunk, error) {
	mlog.Info("PantheonLLM.Stream: called with ", len(messages), " messages")
	req := &core.Request{
		Messages:     messages,
		SystemPrompt: systemPrompt,
	}
	if temperature != nil {
		req.Temperature = temperature
	} else if p.cfg.LLMTemperature > 0 {
		req.Temperature = &p.cfg.LLMTemperature
	}
	if p.cfg.LLMMaxTokens > 0 {
		req.MaxTokens = &p.cfg.LLMMaxTokens
	}

	stream, err := p.model.Stream(ctx, req)
	if err != nil {
		mlog.Error("PantheonLLM.Stream: model.Stream failed: ", err)
		return nil, err
	}

	out := make(chan LLMChunk, 16)
	go func() {
		defer close(out)
		mlog.Info("PantheonLLM.Stream: goroutine started, reading from stream")
		chunkCount := 0
		for part, err := range stream {
			select {
			case <-ctx.Done():
				mlog.Info("PantheonLLM.Stream: context done")
				return
			default:
			}
			if err != nil {
				mlog.Error("PantheonLLM.Stream: stream error: ", err)
				out <- LLMChunk{Err: err}
				return
			}
			chunkCount++
			if chunkCount <= 3 || part.FinishReason != "" {
				mlog.Info("PantheonLLM.Stream: chunk #", chunkCount, " delta=", part.TextDelta, " finish=", part.FinishReason)
			}
			out <- LLMChunk{
				TextDelta:      part.TextDelta,
				ReasoningDelta: part.ReasoningDelta,
				Usage:          part.Usage,
				FinishReason:   part.FinishReason,
			}
		}
		mlog.Info("PantheonLLM.Stream: stream exhausted, total chunks=", chunkCount)
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
	} else if p.cfg.LLMTemperature > 0 {
		req.Temperature = &p.cfg.LLMTemperature
	}
	if p.cfg.LLMMaxTokens > 0 {
		req.MaxTokens = &p.cfg.LLMMaxTokens
	}

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

// LanguageModel returns the underlying Pantheon core.LanguageModel.
func (p *PantheonLLM) LanguageModel() core.LanguageModel { return p.model }
