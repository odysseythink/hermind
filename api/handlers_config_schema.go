package api

import (
	"context"
	"log/slog"
	"net/http"
	"sort"

	"github.com/odysseythink/hermind/config/descriptor"
	"github.com/odysseythink/hermind/skills"
)

// handleConfigSchema responds to GET /api/config/schema with every
// section registered via descriptor.Register. The response is sorted by
// section key so the frontend can render a deterministic navigation.
func (s *Server) handleConfigSchema(w http.ResponseWriter, r *http.Request) {
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
			if f.DatalistSource != nil {
				dto.DatalistSource = &DatalistSourceDTO{
					Section: f.DatalistSource.Section,
					Field:   f.DatalistSource.Field,
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
	for i := range out.Sections {
		if out.Sections[i].Key != "skills" {
			continue
		}
		names := discoveredSkillNames(r.Context())
		for j := range out.Sections[i].Fields {
			if out.Sections[i].Fields[j].Name == "disabled" {
				out.Sections[i].Fields[j].Enum = names
			}
		}
	}
	writeJSON(w, out)
}

// discoveredSkillNames walks the default skills home and returns sorted
// discovered skill names. When no SKILL.md files are found, returns nil.
// Per-file parse errors are logged as warnings but do not prevent a
// partial result — callers still receive names from successfully parsed
// files.
func discoveredSkillNames(ctx context.Context) []string {
	l := skills.NewLoader(skills.DefaultHome())
	all, errs := l.Load()
	for _, e := range errs {
		slog.WarnContext(ctx, "skills: failed to parse skill file", "path", e.Path, "err", e.Err)
	}
	if len(all) == 0 {
		return nil
	}
	out := make([]string, 0, len(all))
	for _, s := range all {
		out = append(out, s.Name)
	}
	sort.Strings(out)
	return out
}

// shapeString converts a descriptor.SectionShape to the JSON-wire string.
// ShapeMap (zero value) returns "" so the DTO's omitempty tag drops the key.
func shapeString(s descriptor.SectionShape) string {
	switch s {
	case descriptor.ShapeScalar:
		return "scalar"
	case descriptor.ShapeKeyedMap:
		return "keyed_map"
	case descriptor.ShapeList:
		return "list"
	default:
		return ""
	}
}
