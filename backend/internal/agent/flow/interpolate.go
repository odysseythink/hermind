package flow

import (
	"regexp"
	"strings"
)

var interpolateRE = regexp.MustCompile(`\{\{(\w+(?:\.\w+)?)\}\}`)

// Interpolate replaces {{var}} placeholders in s with values from vars.
// Unmatched variables are left as-is.
func Interpolate(s string, vars map[string]string) string {
	if vars == nil {
		vars = map[string]string{}
	}
	return interpolateRE.ReplaceAllStringFunc(s, func(m string) string {
		name := strings.TrimSpace(m[2 : len(m)-2])
		if name == "lastStep.output" {
			return vars["__last_output"]
		}
		if v, ok := vars[name]; ok {
			return v
		}
		return m // leave unmatched variable as-is
	})
}
