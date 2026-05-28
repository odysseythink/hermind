package utils

// EstimateTokenCount returns a rough token-count estimate for the given text.
// It approximates tiktoken behaviour:
//   - ASCII characters: ~4 chars per token
//   - Non-ASCII (CJK, emoji, symbols): ~1 char per token
//
// This is intentionally simple — accurate enough for usage reporting without
// adding a heavy tokenizer dependency.
func EstimateTokenCount(text string) int {
	if text == "" {
		return 0
	}
	asciiChars := 0
	nonAsciiChars := 0
	for _, r := range text {
		if r < 128 {
			asciiChars++
		} else {
			nonAsciiChars++
		}
	}
	return asciiChars/4 + nonAsciiChars
}
