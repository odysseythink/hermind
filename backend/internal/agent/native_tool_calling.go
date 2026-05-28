package agent

import "strings"

// Last verified against pantheon v0.0.9
var providersWithNativeToolCalling = map[string]bool{
	"openai":     true,
	"anthropic":  true,
	"groq":       true,
	"ollama":     true,
	"mistral":    true,
	"google":     true,
	"deepseek":   true,
	"openrouter": true,
}

func supportsNativeToolCalling(provider string) bool {
	return providersWithNativeToolCalling[strings.ToLower(strings.TrimSpace(provider))]
}

// SupportsNativeToolCallingForTesting exposes the unexported predicate for tests.
func SupportsNativeToolCallingForTesting(provider string) bool {
	return supportsNativeToolCalling(provider)
}
