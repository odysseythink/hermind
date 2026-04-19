package api

import (
	"net/http"

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
