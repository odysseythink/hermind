package api

import "net/http"

// handleProvidersList responds to GET /api/providers with the configured
// providers. API keys are never surfaced: only the provider slug, model,
// and base URL. The frontend treats this as a read-only discovery
// endpoint; edits happen via PUT /api/config.
func (s *Server) handleProvidersList(w http.ResponseWriter, _ *http.Request) {
	out := make([]ProviderDTO, 0, len(s.opts.Config.Providers))
	for name, p := range s.opts.Config.Providers {
		out = append(out, ProviderDTO{
			Name:     name,
			Provider: p.Provider,
			Model:    p.Model,
			BaseURL:  p.BaseURL,
		})
	}
	writeJSON(w, ProvidersResponse{Providers: out})
}
