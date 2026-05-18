package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/odysseythink/hermind/api"
	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/pantheonadapter"
	"github.com/odysseythink/mlog"
	"github.com/odysseythink/pantheon/core"
)

// buildProviders constructs primary and auxiliary LanguageModel instances from
// cfg. It mirrors the provider-building logic in BuildEngineDeps so that
// RebuildProviderDeps can reuse the same code paths.
//
// When the primary provider has no API key, primary is nil and err is nil
// (degraded mode). An error is returned only when the configuration is
// syntactically invalid (unknown provider, bad base URL, etc.).
func buildProviders(ctx context.Context, cfg *config.Config) (primary, aux core.LanguageModel, err error) {
	primaryName := cfg.Model
	if idx := strings.Index(cfg.Model, "/"); idx >= 0 {
		primaryName = cfg.Model[:idx]
	}

	pCfg, ok := cfg.Providers[primaryName]
	if !ok {
		pCfg = config.ProviderConfig{Provider: primaryName}
	}
	if pCfg.Provider == "" {
		pCfg.Provider = primaryName
	}
	if primaryName == "anthropic" && pCfg.APIKey == "" {
		if envKey := os.Getenv("ANTHROPIC_API_KEY"); envKey != "" {
			pCfg.APIKey = envKey
		}
	}
	if pCfg.Model == "" {
		pCfg.Model = defaultModelFromString(cfg.Model)
	}

	if pCfg.APIKey != "" {
		primary, err = pantheonadapter.BuildPrimaryModel(ctx, pCfg)
		if err != nil {
			return nil, nil, fmt.Errorf("build primary provider: %w", err)
		}
	}

	fallbackCfgs := cfg.FallbackProviders
	if primary != nil && len(fallbackCfgs) > 0 {
		fbModel, fbErr := pantheonadapter.BuildFallbackModel(ctx, pCfg, fallbackCfgs)
		if fbErr == nil {
			primary = fbModel
		}
	}

	if cfg.Auxiliary.APIKey != "" || cfg.Auxiliary.Provider != "" {
		auxCfg := config.ProviderConfig{
			Provider: cfg.Auxiliary.Provider,
			BaseURL:  cfg.Auxiliary.BaseURL,
			APIKey:   cfg.Auxiliary.APIKey,
			Model:    cfg.Auxiliary.Model,
		}
		if auxCfg.Provider == "" {
			auxCfg.Provider = "anthropic"
		}
		var auxErr error
		aux, auxErr = pantheonadapter.BuildModel(ctx, auxCfg)
		if auxErr != nil {
			mlog.Warning("build auxiliary provider failed", mlog.String("err", auxErr.Error()))
		}
	}
	if aux == nil {
		aux = primary
	}

	return primary, aux, nil
}

// RebuildProviderDeps creates a new EngineDeps with updated Provider and
// AuxProvider based on cfg, copying all other fields from current.
func RebuildProviderDeps(ctx context.Context, cfg *config.Config, current *api.EngineDeps) (*api.EngineDeps, error) {
	primary, aux, err := buildProviders(ctx, cfg)
	if err != nil {
		return nil, err
	}
	newDeps := *current
	newDeps.Provider = primary
	newDeps.AuxProvider = aux
	return &newDeps, nil
}
