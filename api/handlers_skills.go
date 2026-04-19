package api

import "net/http"

// handleSkillsList responds to GET /api/skills. It derives the enabled
// state from the on-disk config (which is authoritative) and reports
// disabled skills explicitly. The set of *available* skills is loaded
// at REPL start time elsewhere; surfacing the discovered list here
// would require moving the loader out of the agent package, deferred
// to a later plan.
func (s *Server) handleSkillsList(w http.ResponseWriter, _ *http.Request) {
	disabled := map[string]struct{}{}
	for _, name := range s.opts.Config.Skills.Disabled {
		disabled[name] = struct{}{}
	}
	out := make([]SkillDTO, 0, len(disabled))
	for name := range disabled {
		out = append(out, SkillDTO{Name: name, Enabled: false})
	}
	writeJSON(w, SkillsResponse{Skills: out})
}
