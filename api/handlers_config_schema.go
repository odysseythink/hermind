package api

import (
	"net/http"

	"github.com/odysseythink/hermind/config/descriptor"
)

// handleConfigSchema responds to GET /api/config/schema with every
// section registered via descriptor.Register. The response is sorted by
// section key so the frontend can render a deterministic navigation.
func (s *Server) handleConfigSchema(w http.ResponseWriter, _ *http.Request) {
	all := descriptor.All()
	out := ConfigSchemaResponse{Sections: make([]ConfigSectionDTO, 0, len(all))}
	for _, sec := range all {
		fields := make([]ConfigFieldDTO, 0, len(sec.Fields))
		for _, f := range sec.Fields {
			dto := ConfigFieldDTO{
				Name:     f.Name,
				Label:    f.Label,
				Help:     f.Help,
				Kind:     f.Kind.String(),
				Required: f.Required,
				Default:  f.Default,
				Enum:     f.Enum,
			}
			if f.VisibleWhen != nil {
				dto.VisibleWhen = &PredicateDTO{
					Field:  f.VisibleWhen.Field,
					Equals: f.VisibleWhen.Equals,
				}
			}
			fields = append(fields, dto)
		}
		out.Sections = append(out.Sections, ConfigSectionDTO{
			Key:     sec.Key,
			Label:   sec.Label,
			Summary: sec.Summary,
			GroupID: sec.GroupID,
			Shape:   shapeString(sec.Shape),
			Fields:  fields,
		})
	}
	writeJSON(w, out)
}

// shapeString converts a descriptor.SectionShape to the JSON-wire string.
// ShapeMap (zero value) returns "" so the DTO's omitempty tag drops the key.
func shapeString(s descriptor.SectionShape) string {
	if s == descriptor.ShapeScalar {
		return "scalar"
	}
	return ""
}
