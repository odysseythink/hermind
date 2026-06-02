package compression

import "regexp"

// RedactPatterns returns the set of regex patterns used to sanitize
// conversation text before summarization.
//
// Design choices (§7):
//   - Bare hex strings (≥8 chars) are stripped — they are usually hashes
//     or identifiers with no semantic value in a summary.
//   - Email addresses are stripped — privacy.
//   - API keys (sk-..., token_...) are KEPT — they may be conversation-relevant
//     (e.g. "use token_abc for the next call").
//   - IPv4/IPv6 addresses are stripped — noise.
//   - UUIDs are stripped — noise.
//   - Base64 blobs (>40 chars) are stripped — usually embedded data.
func RedactPatterns() []*regexp.Regexp {
	return []*regexp.Regexp{
		// Bare hex strings (8+ chars), optional 0x prefix
		regexp.MustCompile(`\b0x[0-9a-fA-F]{8,}\b|\b[0-9a-fA-F]{16,}\b`),

		// Email addresses
		regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`),

		// IPv4 addresses
		regexp.MustCompile(`\b(?:[0-9]{1,3}\.){3}[0-9]{1,3}\b`),

		// IPv6 addresses (compressed and full)
		regexp.MustCompile(`\b(?:[0-9a-fA-F]{1,4}:){2,7}[0-9a-fA-F]{1,4}\b|\b::1\b|\b::(?:[0-9a-fA-F]{1,4}:){0,5}[0-9a-fA-F]{1,4}\b`),

		// UUIDs
		regexp.MustCompile(`\b[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}\b`),

		// Base64 blobs (40+ chars of base64 alphabet)
		regexp.MustCompile(`\b[A-Za-z0-9+/]{40,}={0,2}\b`),
	}
}
