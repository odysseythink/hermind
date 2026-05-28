package tts

import (
	"context"
	"fmt"
)

type nativeProvider struct{}

// NewNativeProvider returns a stub Provider that delegates TTS to the browser.
func NewNativeProvider() Provider { return &nativeProvider{} }

func (n *nativeProvider) Name() string    { return "native" }
func (n *nativeProvider) Available() bool { return true }
func (n *nativeProvider) Synthesize(ctx context.Context, text string) (*Synthesis, error) {
	return nil, fmt.Errorf("native TTS is handled by the browser; server-side synthesis not available")
}
