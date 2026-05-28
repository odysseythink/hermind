package tts

import "context"

// Synthesis holds the result of a text-to-speech conversion.
type Synthesis struct {
	Audio       []byte
	ContentType string // e.g. "audio/mpeg"
}

// Provider is the abstraction for a TTS backend.
type Provider interface {
	Synthesize(ctx context.Context, text string) (*Synthesis, error)
	Available() bool
	Name() string
}
