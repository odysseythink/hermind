package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/odysseythink/hermind/pantheonadapter"
)

// handleProvidersModels responds to POST /api/providers/{name}/models.
// Builds the provider via pantheonadapter.BuildProvider and calls Models
// with a 10s timeout, returning the list of model IDs.
//
// Status codes:
//
//	200 - {"models": ["id", ...]}
//	400 - BuildProvider rejected the stored config
//	404 - no Providers[name] in stored config
//	502 - upstream provider errored (network, auth, rate-limit, ...)
func (s *Server) handleProvidersModels(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	cfg, ok := s.opts.Config.Providers[name]
	if !ok {
		http.Error(w, fmt.Sprintf("unknown provider %q", name), http.StatusNotFound)
		return
	}
	p, err := pantheonadapter.BuildProvider(cfg)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	models, err := p.Models(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	ids := make([]string, len(models))
	for i, m := range models {
		ids[i] = m.ID
	}
	writeJSON(w, struct {
		Models []string `json:"models"`
	}{Models: ids})
}
