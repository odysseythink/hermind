package webconfig

import (
	"encoding/json"
	"net/http"
	"os"

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
		for _, f := range editor.Schema() {
			if f.Path != body.Path {
				continue
			}
			if f.Validate != nil {
				if err := f.Validate(body.Value); err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
			}
		}
		if err := s.doc.Set(body.Path, body.Value); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleSave(w http.ResponseWriter, r *http.Request) {
	if err := s.doc.Save(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleReveal(w http.ResponseWriter, r *http.Request) {
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
	w.WriteHeader(http.StatusNoContent)
	go func() { os.Exit(0) }()
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
