package compression

import "strings"

// modelContextLength maps model identifiers to their context-window sizes.
// Values are in tokens. The map covers the most common models across all
// Pantheon-supported providers. Unknown models fall back to 8192.
var modelContextLength = map[string]int{
	// OpenAI
	"gpt-4o":              128000,
	"gpt-4o-mini":         128000,
	"gpt-4-turbo":         128000,
	"gpt-4-turbo-preview": 128000,
	"gpt-4-1106-preview":  128000,
	"gpt-4-0125-preview":  128000,
	"gpt-4":               8192,
	"gpt-4-32k":           32768,
	"gpt-3.5-turbo":       16385,
	"gpt-3.5-turbo-16k":   16385,
	"o1-preview":          128000,
	"o1-mini":             128000,

	// Anthropic
	"claude-3-5-sonnet-20241022": 200000,
	"claude-3-5-sonnet-latest":   200000,
	"claude-3-5-haiku-20241022":  200000,
	"claude-3-opus-20240229":     200000,
	"claude-3-sonnet-20240229":   200000,
	"claude-3-haiku-20240307":    200000,
	"claude-2.1":                 200000,
	"claude-2.0":                 100000,
	"claude-instant-1.2":         100000,

	// Gemini
	"gemini-1.5-pro":   2097152,
	"gemini-1.5-flash": 1048576,
	"gemini-1.0-pro":   32768,

	// Meta (via various providers)
	"llama-3.1-70b": 131072,
	"llama-3.1-8b":  131072,
	"llama-3-70b":   8192,
	"llama-3-8b":    8192,

	// Mistral
	"mistral-large-latest": 131072,
	"mistral-medium":       32768,
	"mistral-small":        32768,
	"mixtral-8x22b":        65536,
	"mixtral-8x7b":         32768,

	// Cohere
	"command-r-plus": 128000,
	"command-r":      128000,

	// DeepSeek
	"deepseek-chat":  65536,
	"deepseek-coder": 65536,

	// Perplexity
	"llama-3.1-sonar-large-128k-online": 128000,
	"llama-3.1-sonar-small-128k-online": 128000,
}

// defaultContextLength is the conservative fallback for unknown models.
const defaultContextLength = 8192

// ContextLengthFor returns the context-window size (in tokens) for the given
// model identifier. If the model is not in the map, it returns
// defaultContextLength (8192). The lookup is case-insensitive.
func ContextLengthFor(model string) int {
	model = strings.ToLower(strings.TrimSpace(model))
	if n, ok := modelContextLength[model]; ok {
		return n
	}
	// Try prefix match for dated model variants (e.g. gpt-4o-2024-08-06)
	for k, v := range modelContextLength {
		if strings.HasPrefix(model, k) {
			return v
		}
	}
	return defaultContextLength
}
