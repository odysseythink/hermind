package api

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/odysseythink/hermind/pantheonadapter"
)

// handleProvidersTest responds to POST /api/providers/{name}/test.
// Builds the named provider from stored config and pings it with a 1-token
// completion to verify the credentials and the configured model id are
// actually usable. Mirrors handleAuxiliaryTest's response shape.
//
// Status codes:
//
//	200 - {"ok": true, "latency_ms": N}
//	400 - BuildModel rejected the stored config
//	404 - no Providers[name] in stored config
//	502 - upstream provider errored on Generate
func (s *Server) handleProvidersTest(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	cfg, ok := s.opts.Config.Providers[name]
	if !ok {
		http.Error(w, fmt.Sprintf("unknown provider %q", name), http.StatusNotFound)
		return
	}
	p, err := pantheonadapter.BuildModel(r.Context(), cfg)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	runProviderPing(w, r, p, cfg.Model)
}
