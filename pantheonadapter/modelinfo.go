package pantheonadapter

import (
	"strings"

	"github.com/odysseythink/hermind/provider"
)

var modelDB = map[string]provider.ModelInfo{
	"gpt-4o": {
		ContextLength:  128000,
		MaxOutputTokens: 16384,
		SupportsVision: true,
		SupportsTools:  true,
	},
	"gpt-4o-mini": {
		ContextLength:  128000,
		MaxOutputTokens: 16384,
		SupportsVision: true,
		SupportsTools:  true,
	},
	"claude-opus-4": {
		ContextLength:  200000,
		MaxOutputTokens: 8192,
		SupportsVision: true,
		SupportsTools:  true,
		SupportsCaching: true,
	},
	"claude-sonnet-4": {
		ContextLength:  200000,
		MaxOutputTokens: 8192,
		SupportsVision: true,
		SupportsTools:  true,
		SupportsCaching: true,
	},
	"claude-3-5-sonnet": {
		ContextLength:  200000,
		MaxOutputTokens: 8192,
		SupportsVision: true,
		SupportsTools:  true,
		SupportsCaching: true,
	},
	"deepseek-chat": {
		ContextLength:  64000,
		MaxOutputTokens: 8192,
		SupportsTools:  true,
	},
	"deepseek-coder": {
		ContextLength:  64000,
		MaxOutputTokens: 8192,
		SupportsTools:  true,
	},
	"qwen": {
		ContextLength:  128000,
		MaxOutputTokens: 8192,
		SupportsTools:  true,
	},
}

// ModelInfoResolver resolves model capabilities for Pantheon-hosted models.
type ModelInfoResolver struct{}

// NewModelInfoResolver creates a new ModelInfoResolver.
func NewModelInfoResolver() *ModelInfoResolver {
	return &ModelInfoResolver{}
}

// Lookup returns model information for the given modelID.
// It performs substring matching against known model identifiers.
// For unknown models, it returns conservative defaults.
func (r *ModelInfoResolver) Lookup(modelID string) *provider.ModelInfo {
	if modelID == "" {
		return r.defaultModelInfo()
	}

	// Check for exact match first.
	if info, ok := modelDB[modelID]; ok {
		return copyModelInfo(info)
	}

	// Substring match: prefer the longest matching key to avoid
	// short prefixes shadowing more specific ones.
	var bestMatch *provider.ModelInfo
	bestLen := 0
	for key, info := range modelDB {
		if strings.Contains(modelID, key) && len(key) > bestLen {
			bestMatch = copyModelInfo(info)
			bestLen = len(key)
		}
	}

	if bestMatch != nil {
		return bestMatch
	}

	return r.defaultModelInfo()
}

func (r *ModelInfoResolver) defaultModelInfo() *provider.ModelInfo {
	return &provider.ModelInfo{
		ContextLength:   128000,
		MaxOutputTokens: 4096,
		SupportsTools:   true,
	}
}

func copyModelInfo(info provider.ModelInfo) *provider.ModelInfo {
	return &provider.ModelInfo{
		ContextLength:     info.ContextLength,
		MaxOutputTokens:   info.MaxOutputTokens,
		SupportsVision:    info.SupportsVision,
		SupportsTools:     info.SupportsTools,
		SupportsStreaming: info.SupportsStreaming,
		SupportsCaching:   info.SupportsCaching,
		SupportsReasoning: info.SupportsReasoning,
	}
}

// EstimateTokens returns a rough token estimate for the given text.
// It uses a character-based heuristic: (len(text) + 3) / 4.
// Returns 0 for an empty string.
func (r *ModelInfoResolver) EstimateTokens(text string) int {
	if text == "" {
		return 0
	}
	return (len(text) + 3) / 4
}
