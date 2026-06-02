package compression

import (
	"context"

	"github.com/odysseythink/pantheon/core"
)

// ContextEngine is the interface for conversation-history compression.
// Pantheon's *compression.Compressor implements this interface natively.
type ContextEngine interface {
	// Compress shortens the given message history, preserving head and tail.
	// If no compression is needed, it returns the history unchanged.
	Compress(ctx context.Context, history []core.Message) ([]core.Message, error)
}
