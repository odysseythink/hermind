package api

import (
	"encoding/json"
	"io"
	"net/http"

	"gopkg.in/yaml.v3"

	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/gateway/platforms"
)

// handleConfigGet responds to GET /api/config with the current Config
// marshaled into a map{string: any} via YAML so the frontend sees the
// same snake_case keys as the on-disk config file.
func (s *Server) handleConfigGet(w http.ResponseWriter, _ *http.Request) {
	data, err := yaml.Marshal(s.opts.Config)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var m map[string]any
	if err := yaml.Unmarshal(data, &m); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	redactSecrets(m)
	writeJSON(w, ConfigResponse{Config: m})
}

// redactSecrets walks m["gateway"]["platforms"][*]["options"], consults
// the platform registry for each entry's Type, and blanks every field
// whose Kind is FieldSecret. Silently ignores unknown types or missing
// sections — we're redacting defensively, not validating.
func redactSecrets(m map[string]any) {
	gw, _ := m["gateway"].(map[string]any)
	plats, _ := gw["platforms"].(map[string]any)
	for _, raw := range plats {
		inst, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		typ, _ := inst["type"].(string)
		if typ == "" {
			continue
		}
		d, ok := platforms.Get(typ)
		if !ok {
			continue
		}
		opts, _ := inst["options"].(map[string]any)
		if opts == nil {
			continue
		}
		for _, f := range d.Fields {
			if f.Kind == platforms.FieldSecret {
				if _, present := opts[f.Name]; present {
					opts[f.Name] = ""
				}
			}
		}
	}
}

// handleConfigPut accepts {"config": {...}} where the inner object is a
// full Config value. The payload is round-tripped through YAML (both
// in and out) so the same tags decide the shape on both sides.
func (s *Server) handleConfigPut(w http.ResponseWriter, r *http.Request) {
	if s.opts.ConfigPath == "" {
		http.Error(w, "config write-back not configured", http.StatusNotImplemented)
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		http.Error(w, err.Error(), http.StatusRequestEntityTooLarge)
		return
	}
	var req struct {
		Config json.RawMessage `json:"config"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	// JSON is a subset of YAML so yaml.Unmarshal accepts it.
	var updated config.Config
	if err := yaml.Unmarshal(req.Config, &updated); err != nil {
		http.Error(w, "invalid config payload: "+err.Error(), http.StatusBadRequest)
		return
	}
	if err := config.SaveToPath(s.opts.ConfigPath, &updated); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	*s.opts.Config = updated
	writeJSON(w, OKResponse{OK: true})
}
