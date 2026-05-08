// provider/embedcapable.go
package provider

import "context"

// EmbedCapable is an optional capability for providers that expose a
// text-embedding endpoint. Callers type-assert before using:
//
//	if ec, ok := p.(provider.EmbedCapable); ok { ... }
type EmbedCapable interface {
	// Embed returns a float32 vector for the given text.
	// Returns an error if the embedding call fails.
	Embed(ctx context.Context, model, text string) ([]float32, error)
}
