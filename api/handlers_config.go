package api

import (
	"encoding/json"
	"io"
	"net/http"

	"gopkg.in/yaml.v3"

	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/config/descriptor"
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

// redactSecrets blanks every secret field in m, covering two universes:
//
//  1. gateway.platforms[*].options — fields of Kind FieldSecret on the
//     platform descriptor registered in gateway/platforms.
//  2. m[section.Key][field.Name] — fields of Kind FieldSecret on every
//     config section registered in config/descriptor.
//
// Silently ignores unknown types, missing keys, or non-map values —
// we're redacting defensively, not validating.
func redactSecrets(m map[string]any) {
	redactPlatformSecrets(m)
	redactSectionSecrets(m)
}

func redactPlatformSecrets(m map[string]any) {
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

func redactSectionSecrets(m map[string]any) {
	for _, sec := range descriptor.All() {
		blob, ok := m[sec.Key].(map[string]any)
		if !ok {
			continue
		}
		for _, f := range sec.Fields {
			if f.Kind != descriptor.FieldSecret {
				continue
			}
			if _, present := blob[f.Name]; present {
				blob[f.Name] = ""
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
	preserveSecrets(&updated, s.opts.Config)
	if err := config.SaveToPath(s.opts.ConfigPath, &updated); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	*s.opts.Config = updated
	writeJSON(w, OKResponse{OK: true})
}

// preserveSecrets copies every blank secret in updated back from current,
// covering both platform secrets (gateway.platforms[*].options) and
// section secrets registered in config/descriptor. Keys missing from
// current (new platforms, new providers, …) are left as-is.
func preserveSecrets(updated, current *config.Config) {
	preservePlatformSecrets(updated, current)
	preserveSectionSecrets(updated, current)
}

func preservePlatformSecrets(updated, current *config.Config) {
	for key, newPC := range updated.Gateway.Platforms {
		curPC, ok := current.Gateway.Platforms[key]
		if !ok {
			continue
		}
		d, ok := platforms.Get(newPC.Type)
		if !ok {
			continue
		}
		if newPC.Options == nil {
			newPC.Options = map[string]string{}
		}
		for _, f := range d.Fields {
			if f.Kind != platforms.FieldSecret {
				continue
			}
			if newPC.Options[f.Name] == "" {
				newPC.Options[f.Name] = curPC.Options[f.Name]
			}
		}
		updated.Gateway.Platforms[key] = newPC
	}
}

// preserveSectionSecrets round-trips blanks for every FieldSecret on a
// registered section. The mapping from (section key, field name) to the
// Go struct field is done via a YAML round-trip: marshal both configs
// into map[string]any, mutate updated's map, re-unmarshal back into
// updated. This avoids reflection-over-struct-tags gymnastics at the
// cost of two marshal/unmarshal cycles — acceptable because PUT is cold.
func preserveSectionSecrets(updated, current *config.Config) {
	sections := descriptor.All()
	if len(sections) == 0 {
		return
	}
	// Detect any secret field that's blank in updated — cheap reflection
	// would also work, but YAML round-trip keeps this keyed on yaml tags,
	// same as the rest of the config handler.
	updBytes, err := yaml.Marshal(updated)
	if err != nil {
		return
	}
	curBytes, err := yaml.Marshal(current)
	if err != nil {
		return
	}
	var updM, curM map[string]any
	if err := yaml.Unmarshal(updBytes, &updM); err != nil {
		return
	}
	if err := yaml.Unmarshal(curBytes, &curM); err != nil {
		return
	}

	changed := false
	for _, sec := range sections {
		upd, ok := updM[sec.Key].(map[string]any)
		if !ok {
			continue
		}
		cur, _ := curM[sec.Key].(map[string]any)
		for _, f := range sec.Fields {
			if f.Kind != descriptor.FieldSecret {
				continue
			}
			newVal, _ := upd[f.Name].(string)
			if newVal != "" {
				continue
			}
			if cur == nil {
				continue
			}
			prevVal, _ := cur[f.Name].(string)
			if prevVal == "" {
				continue
			}
			upd[f.Name] = prevVal
			changed = true
		}
		if changed {
			updM[sec.Key] = upd
		}
	}
	if !changed {
		return
	}

	reBytes, err := yaml.Marshal(updM)
	if err != nil {
		return
	}
	_ = yaml.Unmarshal(reBytes, updated)
}
