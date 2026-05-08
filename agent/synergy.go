package agent

import "strings"

// applySynergyBudget composes active skills and recalled memories within
// an optional combined character budget. When TokenBudget is 0 this is a
// pass-through. When > 0 the budget is split by SkillRatio (default 0.35)
// between skills and memories, and items are truncated in order until the
// share fills. Memories whose Jaccard overlap with any skill body exceeds
// DedupJaccard are dropped first, so skills always "win" on conflicting
// content.
//
// The function measures length in raw characters — that's a reasonable
// proxy for token count for English / markdown prose, avoids pulling in
// a tiktoken dependency, and stays stable for prompt caching.
func applySynergyBudget(skills []ActiveSkill, memories []string, b SynergyBudget) ([]ActiveSkill, []string) {
	if b.DedupJaccard > 0 && len(skills) > 0 && len(memories) > 0 {
		memories = dropMemoriesOverlappingSkills(skills, memories, b.DedupJaccard)
	}
	if b.TokenBudget <= 0 {
		return skills, memories
	}

	ratio := b.SkillRatio
	if ratio <= 0 {
		ratio = 0.35
	}
	if ratio > 1 {
		ratio = 1
	}
	skillShare := int(float64(b.TokenBudget) * ratio)
	memShare := b.TokenBudget - skillShare

	skills = capActiveSkills(skills, skillShare)
	memories = capStrings(memories, memShare)
	return skills, memories
}

func capActiveSkills(skills []ActiveSkill, budget int) []ActiveSkill {
	if budget <= 0 || len(skills) == 0 {
		return nil
	}
	out := make([]ActiveSkill, 0, len(skills))
	used := 0
	for _, s := range skills {
		cost := len(s.Name) + len(s.Description) + len(s.Body)
		if used+cost > budget {
			if used >= budget {
				break
			}
			remaining := budget - used
			if remaining < 32 {
				break
			}
			truncated := s
			if len(truncated.Body) > remaining {
				truncated.Body = strings.TrimSpace(truncated.Body[:remaining]) + "…"
			}
			out = append(out, truncated)
			break
		}
		out = append(out, s)
		used += cost
	}
	return out
}

func capStrings(items []string, budget int) []string {
	if budget <= 0 || len(items) == 0 {
		return nil
	}
	out := make([]string, 0, len(items))
	used := 0
	for _, s := range items {
		if used+len(s) > budget {
			if used >= budget {
				break
			}
			remaining := budget - used
			if remaining < 32 {
				break
			}
			out = append(out, strings.TrimSpace(s[:remaining])+"…")
			break
		}
		out = append(out, s)
		used += len(s)
	}
	return out
}

func dropMemoriesOverlappingSkills(skills []ActiveSkill, memories []string, threshold float64) []string {
	skillSets := make([]map[string]struct{}, 0, len(skills))
	for _, s := range skills {
		skillSets = append(skillSets, tokenSet(s.Body))
	}
	out := make([]string, 0, len(memories))
	for _, m := range memories {
		mset := tokenSet(m)
		drop := false
		for _, sset := range skillSets {
			if jaccard(sset, mset) >= threshold {
				drop = true
				break
			}
		}
		if !drop {
			out = append(out, m)
		}
	}
	return out
}

func tokenSet(s string) map[string]struct{} {
	out := make(map[string]struct{})
	for _, tok := range strings.Fields(strings.ToLower(s)) {
		tok = strings.Trim(tok, ".,;:!?\"'()[]{}`")
		if len(tok) < 3 {
			continue
		}
		out[tok] = struct{}{}
	}
	return out
}

func jaccard(a, b map[string]struct{}) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	inter := 0
	small, large := a, b
	if len(a) > len(b) {
		small, large = b, a
	}
	for k := range small {
		if _, ok := large[k]; ok {
			inter++
		}
	}
	union := len(a) + len(b) - inter
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}
