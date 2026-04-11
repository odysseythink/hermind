package agent

import "regexp"

// redactPatterns are regexes that match common secrets. The
// replacement string is always "[REDACTED]".
var redactPatterns = []*regexp.Regexp{
	// Generic bearer tokens
	regexp.MustCompile(`(?i)bearer\s+[A-Za-z0-9\-._~+/=]{16,}`),
	// AWS-style access keys
	regexp.MustCompile(`AKIA[0-9A-Z]{16}`),
	// OpenAI / Anthropic keys
	regexp.MustCompile(`sk-(?:ant-|proj-|live-)?[A-Za-z0-9_-]{20,}`),
	regexp.MustCompile(`sk-or-v1-[A-Za-z0-9]{32,}`),
	// Generic long hex tokens
	regexp.MustCompile(`\b[a-f0-9]{32,64}\b`),
	// Password=... / api_key=...
	regexp.MustCompile(`(?i)(password|api_key|apikey|token)\s*[:=]\s*["']?[^\s"']{8,}`),
	// Email addresses
	regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`),
}

// Redact walks the input and replaces anything matching a known
// secret pattern with [REDACTED]. Safe to call on arbitrary text.
func Redact(s string) string {
	out := s
	for _, re := range redactPatterns {
		out = re.ReplaceAllString(out, "[REDACTED]")
	}
	return out
}
