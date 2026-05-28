package extensions

import (
	"context"
	"fmt"
	"sync"

	"github.com/odysseythink/hermind/backend/internal/collector/core"
)

// Extension is the interface implemented by all collector extensions.
type Extension interface {
	Name() string
	Handle(ctx context.Context, endpoint string, method string, body []byte) (*core.ExtensionResponse, error)
}

// Registry holds and dispatches to registered extensions.
type Registry struct {
	mu         sync.RWMutex
	extensions map[string]Extension
}

// NewRegistry creates a new empty Registry.
func NewRegistry() *Registry {
	return &Registry{extensions: make(map[string]Extension)}
}

// Register adds an extension to the registry.
func (r *Registry) Register(name string, ext Extension) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.extensions[name] = ext
}

// Handle dispatches a request to the extension registered for the given endpoint.
func (r *Registry) Handle(ctx context.Context, endpoint string, method string, body []byte) (*core.ExtensionResponse, error) {
	r.mu.RLock()
	ext, ok := r.extensions[endpoint]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("extension %s not found", endpoint)
	}
	return ext.Handle(ctx, endpoint, method, body)
}
