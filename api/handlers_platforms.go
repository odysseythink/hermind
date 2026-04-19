package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/odysseythink/hermind/gateway/platforms"
)

func (s *Server) handlePlatformsSchema(w http.ResponseWriter, _ *http.Request) {
	all := platforms.All()
	out := PlatformsSchemaResponse{Descriptors: make([]SchemaDescriptorDTO, 0, len(all))}
	for _, d := range all {
		fields := make([]SchemaFieldDTO, 0, len(d.Fields))
		for _, f := range d.Fields {
			fields = append(fields, SchemaFieldDTO{
				Name:     f.Name,
				Label:    f.Label,
				Help:     f.Help,
				Kind:     f.Kind.String(),
				Required: f.Required,
				Default:  f.Default,
				Enum:     f.Enum,
			})
		}
		out.Descriptors = append(out.Descriptors, SchemaDescriptorDTO{
			Type:        d.Type,
			DisplayName: d.DisplayName,
			Summary:     d.Summary,
			Fields:      fields,
		})
	}
	writeJSON(w, out)
}

func (s *Server) handlePlatformReveal(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	pc, ok := s.opts.Config.Gateway.Platforms[key]
	if !ok {
		writeJSONStatus(w, http.StatusNotFound, ErrorResponse{Error: "unknown platform key"})
		return
	}
	var req RevealRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONStatus(w, http.StatusBadRequest, ErrorResponse{Error: "invalid request body"})
		return
	}
	d, ok := platforms.Get(pc.Type)
	if !ok {
		writeJSONStatus(w, http.StatusBadRequest, ErrorResponse{Error: "unknown platform type"})
		return
	}
	var fieldSpec *platforms.FieldSpec
	for i := range d.Fields {
		if d.Fields[i].Name == req.Field {
			fieldSpec = &d.Fields[i]
			break
		}
	}
	if fieldSpec == nil {
		writeJSONStatus(w, http.StatusBadRequest, ErrorResponse{Error: "no such field"})
		return
	}
	if fieldSpec.Kind != platforms.FieldSecret {
		writeJSONStatus(w, http.StatusBadRequest, ErrorResponse{Error: "field is not secret"})
		return
	}
	writeJSON(w, RevealResponse{Value: pc.Options[req.Field]})
}

func (s *Server) handlePlatformTest(w http.ResponseWriter, r *http.Request) {
	if s.opts.Controller == nil {
		writeJSONStatus(w, http.StatusServiceUnavailable,
			ErrorResponse{Error: "gateway controller not configured"})
		return
	}
	key := chi.URLParam(r, "key")
	err := s.opts.Controller.TestPlatform(r.Context(), key)
	switch {
	case err == nil:
		writeJSON(w, PlatformTestResponse{OK: true})
	case errors.Is(err, ErrTestNotImplemented):
		writeJSONStatus(w, http.StatusNotImplemented,
			ErrorResponse{Error: "test not implemented for this platform type"})
	case errors.Is(err, ErrUnknownPlatformKey):
		writeJSONStatus(w, http.StatusNotFound,
			ErrorResponse{Error: "unknown platform key"})
	default:
		writeJSON(w, PlatformTestResponse{OK: false, Error: err.Error()})
	}
}

func (s *Server) handlePlatformsApply(w http.ResponseWriter, r *http.Request) {
	if s.opts.Controller == nil {
		writeJSONStatus(w, http.StatusServiceUnavailable,
			ErrorResponse{Error: "gateway controller not configured"})
		return
	}
	res, err := s.opts.Controller.Apply(r.Context())
	switch {
	case err == nil:
		writeJSON(w, res)
	case errors.Is(err, ErrApplyInProgress):
		writeJSONStatus(w, http.StatusConflict,
			ErrorResponse{Error: "apply already in progress"})
	default:
		writeJSON(w, ApplyResult{OK: false, Error: err.Error(), TookMS: res.TookMS})
	}
}
