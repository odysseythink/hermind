package agent

import (
	"regexp"
	"strings"
)

// RoutingRule maps a detected task kind to a preferred model.
type RoutingRule struct {
	Pattern *regexp.Regexp
	Model   string
	Kind    string
}

// DefaultRoutingRules are simple keyword-based rules for Phase 12b.
// They are intentionally conservative — only routes when the prompt
// strongly matches a category. Unmatched prompts fall through to the
// caller's default model.
var DefaultRoutingRules = []RoutingRule{
	{
		Pattern: regexp.MustCompile(`(?i)\b(prove|theorem|lemma|integral|derivative|calculus|math(s)?)\b`),
		Model:   "o1",
		Kind:    "math",
	},
	{
		Pattern: regexp.MustCompile(`(?i)\b(refactor|debug|implement|unit test|typescript|rust|kotlin|swift)\b`),
		Model:   "claude-opus-4-6",
		Kind:    "code",
	},
	{
		Pattern: regexp.MustCompile(`(?i)\b(poem|story|short story|limerick|creative writing)\b`),
		Model:   "claude-sonnet-4-6",
		Kind:    "creative",
	},
	{
		Pattern: regexp.MustCompile(`(?i)\b(summar(ize|y)|tldr|extract|list)\b`),
		Model:   "gpt-4o-mini",
		Kind:    "summary",
	},
}

// SelectModel runs the prompt through the routing rules and returns
// the matched model, or `fallback` when no rule matches.
func SelectModel(prompt, fallback string, rules []RoutingRule) (string, string) {
	if rules == nil {
		rules = DefaultRoutingRules
	}
	lp := strings.ToLower(prompt)
	for _, r := range rules {
		if r.Pattern.MatchString(lp) {
			return r.Model, r.Kind
		}
	}
	return fallback, "default"
}
