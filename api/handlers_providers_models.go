package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/odysseythink/hermind/provider"
	"github.com/odysseythink/hermind/provider/factory"
)

// handleProvidersModels responds to POST /api/providers/{name}/models.
// Reads config.Providers[name], dispatches via factory.New, type-asserts
// provider.ModelLister, and calls ListModels with a 10s timeout.
// Matches the legacy cli/ui/webconfig/handlers.go:258 behavior but lives
// in the Stage-1+ api package.
//
// Status codes:
//
//	200 - {"models": ["id", ...]}
//	400 - factory.New rejected the stored config
//	404 - no Providers[name] in stored config
//	501 - provider type exists but its constructor doesn't implement ModelLister
//	502 - upstream provider errored (network, auth, rate-limit, ...)
func (s *Server) handleProvidersModels(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	cfg, ok := s.opts.Config.Providers[name]
	if !ok {
		http.Error(w, fmt.Sprintf("unknown provider %q", name), http.StatusNotFound)
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
