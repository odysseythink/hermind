package api

import (
	"net/http"
	"time"
)

// handleStatus responds to GET /api/status. Public endpoint.
func (s *Server) handleStatus(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, StatusResponse{
		Version:       s.opts.Version,
		UptimeSec:     int64(time.Since(s.bootedAt).Seconds()),
		StorageDriver: s.driverName(),
		InstanceRoot:  s.opts.InstanceRoot,
		CurrentModel:  s.opts.Config.Model,
	})
}

// handleModelInfo responds to GET /api/model/info. Public endpoint.
// Context length / tool support are best-effort heuristics keyed on the
// configured provider slug; we prefer "stay usable" over "strict
// accuracy" until the provider factory exposes a capability map.
func (s *Server) handleModelInfo(w http.ResponseWriter, _ *http.Request) {
	resp := ModelInfoResponse{Model: s.opts.Config.Model}
	if len(s.opts.Config.Providers) > 0 && s.opts.Config.Model != "" {
		resp.ContextLength = 200_000
		resp.SupportsTools = true
		resp.SupportsVision = true
		resp.MaxOutputTokens = 8192
	}
	writeJSON(w, resp)
}
