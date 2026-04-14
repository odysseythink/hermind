package webconfig

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"time"

	"github.com/odysseythink/hermind/config/editor"
)

// schemaField is the JSON-serialisable view of editor.Field (omits Validate).
type schemaField struct {
	Path    string   `json:"path"`
	Label   string   `json:"label"`
	Help    string   `json:"help,omitempty"`
	Kind    int      `json:"kind"`
	Enum    []string `json:"enum,omitempty"`
	Section string   `json:"section"`
}

func (s *Server) handleSchema(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	fields := editor.Schema()
	out := make([]schemaField, len(fields))
	for i, f := range fields {
		out[i] = schemaField{
			Path:    f.Path,
			Label:   f.Label,
			Help:    f.Help,
			Kind:    int(f.Kind),
			Enum:    f.Enum,
			Section: f.Section,
		}
	}
	writeJSON(w, out)
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		out := map[string]any{}
		for _, f := range editor.Schema() {
			if f.Kind == editor.KindList {
				continue
			}
			v, _ := s.doc.Get(f.Path)
			if f.Kind == editor.KindSecret && v != "" {
				v = "••••"
			}
			out[f.Path] = v
		}
		writeJSON(w, out)
	case http.MethodPost:
		var body struct {
			Path  string `json:"path"`
			Value any    `json:"value"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		var matched *editor.Field
		for i := range editor.Schema() {
			f := editor.Schema()[i]
			if f.Path == body.Path {
				matched = &f
				break
			}
		}
		if matched == nil {
			http.Error(w, "unknown field: "+body.Path, http.StatusBadRequest)
			return
		}
		if matched.Kind == editor.KindList {
			http.Error(w, "list fields cannot be set directly", http.StatusBadRequest)
			return
		}
		if matched.Validate != nil {
			if err := matched.Validate(body.Value); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		}
		// Coerce JSON numbers to the YAML type declared by the schema.
		v := body.Value
		switch matched.Kind {
		case editor.KindInt:
			if f, ok := v.(float64); ok {
				v = int64(f)
			}
		case editor.KindFloat:
			if f, ok := v.(float64); ok {
				v = f
			}
		case editor.KindBool:
			if b, ok := v.(bool); ok {
				v = b
			}
		}
		if err := s.doc.Set(body.Path, v); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleSave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := s.doc.Save(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleReveal(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !isLocalOrigin(r) {
		http.Error(w, "cross-origin denied", http.StatusForbidden)
		return
	}
	var body struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	for _, f := range editor.Schema() {
		if f.Path == body.Path && f.Kind == editor.KindSecret {
			v, _ := s.doc.Get(body.Path)
			writeJSON(w, map[string]string{"value": v})
			return
		}
	}
	http.Error(w, "not a secret field", http.StatusBadRequest)
}

func (s *Server) handleShutdown(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.WriteHeader(http.StatusNoContent)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if s.srv != nil {
			_ = s.srv.Shutdown(ctx)
		}
	}()
}

// isLocalOrigin returns true when the request carries no Origin header, or
// when Origin points at loopback. Defends /api/reveal from DNS-rebinding
// attacks that would otherwise let a malicious site exfiltrate secrets
// through the user's browser.
func isLocalOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true // same-origin fetch sends no Origin
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	host := u.Hostname()
	return host == "127.0.0.1" || host == "localhost" || host == "::1"
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
