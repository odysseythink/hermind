package api

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/odysseythink/hermind/provider"
	"github.com/odysseythink/hermind/provider/factory"
)

// handleFallbackProvidersModels responds to POST /api/fallback_providers/{index}/models.
// Mirrors handleProvidersModels but addresses by index into the ordered
// FallbackProviders slice.
//
// Status codes:
//
//	200 - {"models": ["id", ...]}
//	400 - index is not a non-negative integer, or factory.New rejected the stored config
//	404 - index is out of range for the current FallbackProviders slice
//	501 - provider type exists but its constructor doesn't implement ModelLister
//	502 - upstream provider errored (network, auth, rate-limit, ...)
func (s *Server) handleFallbackProvidersModels(w http.ResponseWriter, r *http.Request) {
	raw := chi.URLParam(r, "index")
	idx, err := strconv.Atoi(raw)
	if err != nil || idx < 0 {
		http.Error(w, fmt.Sprintf("invalid index %q", raw), http.StatusBadRequest)
		return
	}
	list := s.opts.Config.FallbackProviders
	if idx >= len(list) {
		http.Error(w, fmt.Sprintf("fallback_providers index %d out of range (len=%d)", idx, len(list)), http.StatusNotFound)
		return
	}
	cfg := list[idx]
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
