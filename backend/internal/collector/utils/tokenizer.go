package utils

import (
	"github.com/pkoukk/tiktoken-go"
)

// Tokenizer provides token-count estimation using tiktoken.
type Tokenizer struct {
	encoder *tiktoken.Tiktoken
}

// NewTokenizer creates a new Tokenizer using the cl100k_base encoding.
func NewTokenizer() (*Tokenizer, error) {
	enc, err := tiktoken.GetEncoding("cl100k_base")
	if err != nil {
		return nil, err
	}
	return &Tokenizer{encoder: enc}, nil
}

// Count returns the token count for the given input string.
// For very large inputs it falls back to a character-length heuristic.
func (t *Tokenizer) Count(input string) int {
	const maxKBEstimate = 10 * 1024
	const divisor = 8
	if len(input) > maxKBEstimate {
		return (len(input) + divisor - 1) / divisor
	}
	tokens := t.encoder.Encode(input, nil, nil)
	return len(tokens)
}
