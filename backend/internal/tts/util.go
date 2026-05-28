package tts

// pick returns the first non-empty string among settings[key] and fallback.
func pick(key string, settings map[string]string, fallback string) string {
	if v, ok := settings[key]; ok && v != "" {
		return v
	}
	return fallback
}
