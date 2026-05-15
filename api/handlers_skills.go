package api

import (
	"context"
	"net/http"
	"path/filepath"
	"sort"

	"github.com/odysseythink/hermind/skills"
	"github.com/odysseythink/mlog"
)

// handleSkillsList responds to GET /api/skills. Walks <InstanceRoot>/skills/
// to discover installed skills, enriches each with the description parsed
// from SKILL.md front-matter, and marks Enabled by checking the skill name
// against config.skills.disabled. Names in the disabled list that are not
// present on disk are emitted as "ghost" rows so users can still un-disable
// orphaned entries from the UI.
func (s *Server) handleSkillsList(w http.ResponseWriter, r *http.Request) {
	skillsHome := filepath.Join(s.opts.InstanceRoot, "skills")
	loaded := loadSkillsForList(r.Context(), skillsHome)

	disabled := map[string]struct{}{}
	for _, name := range s.opts.Config.Skills.Disabled {
		disabled[name] = struct{}{}
	}

	out := make([]SkillDTO, 0, len(loaded)+len(disabled))
	seen := map[string]struct{}{}
	for _, sk := range loaded {
		_, isDisabled := disabled[sk.Name]
		out = append(out, SkillDTO{
			Name:        sk.Name,
			Description: sk.Description,
			Enabled:     !isDisabled,
		})
		seen[sk.Name] = struct{}{}
	}
	for name := range disabled {
		if _, found := seen[name]; found {
			continue
		}
		out = append(out, SkillDTO{Name: name, Enabled: false})
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	writeJSON(w, SkillsResponse{Skills: out})
}

// loadSkillsForList wraps skills.NewLoader and downgrades parse errors to
// warnings — same forgiveness as discoveredSkillNames in
// handlers_config_schema.go. A bad SKILL.md never fails the whole request.
func loadSkillsForList(ctx context.Context, home string) []*skills.Skill {
	l := skills.NewLoader(home)
	loaded, errs := l.Load()
	for _, e := range errs {
		mlog.WarningContext(ctx, "skills: failed to parse skill file", mlog.String("path", e.Path), mlog.String("err", e.Err.Error()))
	}
	return loaded
}
