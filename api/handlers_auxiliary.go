package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/provider"
	"github.com/odysseythink/hermind/provider/factory"
)

// resolveAuxiliaryConfig returns the ProviderConfig used to instantiate the
// auxiliary provider for "fetch models" and "test" endpoints. When the
// auxiliary block is non-blank we use it directly; when blank we fall back to
// the main provider config — matching the engine_deps.go contract that aux
// reuses the main provider when unconfigured.
func resolveAuxiliaryConfig(c *config.Config) (config.ProviderConfig, error) {
	aux := c.Auxiliary
	if aux.APIKey != "" || aux.Provider != "" {
		cfg := config.ProviderConfig{
			Provider: aux.Provider,
			BaseURL:  aux.BaseURL,
			APIKey:   aux.APIKey,
			Model:    aux.Model,
		}
		if cfg.Provider == "" {
			cfg.Provider = "anthropic"
		}
		return cfg, nil
	}
	primaryName := c.Model
	if idx := strings.Index(c.Model, "/"); idx >= 0 {
		primaryName = c.Model[:idx]
	}
	if primaryName == "" {
		return config.ProviderConfig{}, errors.New("auxiliary unconfigured and no main provider model set")
	}
	pCfg, ok := c.Providers[primaryName]
	if !ok {
		return config.ProviderConfig{}, fmt.Errorf("auxiliary unconfigured and main provider %q not found in providers", primaryName)
	}
	if pCfg.Provider == "" {
		pCfg.Provider = primaryName
	}
	if pCfg.Model == "" {
		if idx := strings.Index(c.Model, "/"); idx >= 0 {
			pCfg.Model = c.Model[idx+1:]
		}
	}
	return pCfg, nil
}

// handleAuxiliaryModels responds to POST /api/auxiliary/models.
// Resolves the effective auxiliary provider config (with main-provider fallback),
// builds the provider via factory.New, and calls ListModels with a 10s timeout.
//
// Status codes:
//
//	200 - {"models": ["id", ...]}
//	400 - resolution failed (no aux + no usable main) or factory rejected config
//	501 - provider type exists but its constructor doesn't implement ModelLister
//	502 - upstream provider errored
func (s *Server) handleAuxiliaryModels(w http.ResponseWriter, r *http.Request) {
	cfg, err := resolveAuxiliaryConfig(s.opts.Config)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	p, err := factory.New(cfg)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	lister, ok := p.(provider.ModelLister)
	if !ok {
		http.Error(w,
			fmt.Sprintf("provider %q does not support model listing", cfg.Provider),
			http.StatusNotImplemented)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	models, err := lister.ListModels(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, struct {
		Models []string `json:"models"`
	}{Models: models})
}

// handleAuxiliaryTest responds to POST /api/auxiliary/test. Sends a tiny
// 1-token "ping" completion through the resolved auxiliary provider to verify
// the credentials + model id are usable end-to-end. Returns latency_ms on
// success so the UI can show a brief connectivity confirmation.
//
// Status codes:
//
//	200 - {"ok": true, "latency_ms": N}
//	400 - resolution failed or factory rejected config
//	502 - upstream provider errored on Complete
func (s *Server) handleAuxiliaryTest(w http.ResponseWriter, r *http.Request) {
	cfg, err := resolveAuxiliaryConfig(s.opts.Config)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	p, err := factory.New(cfg)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	runProviderPing(w, r, p, cfg.Model)
}

// runProviderPing performs the 1-token completion ping and writes the JSON
// response. Shared between the auxiliary and per-provider test endpoints so
// both share latency reporting and timeout behavior.
func runProviderPing(w http.ResponseWriter, r *http.Request, p provider.Provider, model string) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	req := &provider.Request{
		Model: model,
		Messages: []message.Message{
			{Role: message.RoleUser, Content: message.TextContent("ping")},
		},
		MaxTokens: 1,
	}
	start := time.Now()
	if _, err := p.Complete(ctx, req); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, struct {
		OK        bool  `json:"ok"`
		LatencyMS int64 `json:"latency_ms"`
	}{OK: true, LatencyMS: time.Since(start).Milliseconds()})
}
