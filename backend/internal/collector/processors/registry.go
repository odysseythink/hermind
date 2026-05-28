package processors

import (
	"sync"

	"github.com/odysseythink/hermind/backend/internal/collector/pipeline"
)

// Registry maps file extensions to ContentExtractor implementations.
type Registry struct {
	mu    sync.RWMutex
	byExt map[string]pipeline.ContentExtractor
}

// NewRegistry creates a new empty Registry.
func NewRegistry() *Registry {
	return &Registry{byExt: make(map[string]pipeline.ContentExtractor)}
}

// Register binds an extractor to the given file extension (e.g. ".txt").
func (r *Registry) Register(ext string, extractor pipeline.ContentExtractor) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byExt[ext] = extractor
}

// Get returns the extractor registered for the given extension, or nil.
func (r *Registry) Get(ext string) pipeline.ContentExtractor {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.byExt[ext]
}

// AllExtensions returns a slice of all registered extensions.
func (r *Registry) AllExtensions() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.byExt))
	for ext := range r.byExt {
		out = append(out, ext)
	}
	return out
}
