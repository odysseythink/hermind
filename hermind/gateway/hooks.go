package gateway

import (
	"context"
	"sync"
)

// PreHook runs before the gateway invokes the agent. It may mutate
// the IncomingMessage, drop it by returning (nil, nil), or return
// an error to abort. Returning a non-nil *IncomingMessage lets the
// next hook see the updated value.
type PreHook func(ctx context.Context, in *IncomingMessage) (*IncomingMessage, error)

// PostHook runs after the agent produces a reply. Mutating the
// OutgoingMessage is fine. Returning nil drops the reply.
type PostHook func(ctx context.Context, in IncomingMessage, out *OutgoingMessage) (*OutgoingMessage, error)

// Hooks is a small composable set of PreHook / PostHook chains.
type Hooks struct {
	mu    sync.RWMutex
	pres  []PreHook
	posts []PostHook
}

// NewHooks builds an empty Hooks.
func NewHooks() *Hooks { return &Hooks{} }

// AddPre appends a pre-hook.
func (h *Hooks) AddPre(fn PreHook) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.pres = append(h.pres, fn)
}

// AddPost appends a post-hook.
func (h *Hooks) AddPost(fn PostHook) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.posts = append(h.posts, fn)
}

// RunPre walks the pre-hook chain. Stops on first nil return (drop)
// or error.
func (h *Hooks) RunPre(ctx context.Context, in IncomingMessage) (*IncomingMessage, error) {
	h.mu.RLock()
	pres := append([]PreHook{}, h.pres...)
	h.mu.RUnlock()
	current := &in
	for _, fn := range pres {
		next, err := fn(ctx, current)
		if err != nil {
			return nil, err
		}
		if next == nil {
			return nil, nil
		}
		current = next
	}
	return current, nil
}

// RunPost walks the post-hook chain.
func (h *Hooks) RunPost(ctx context.Context, in IncomingMessage, out OutgoingMessage) (*OutgoingMessage, error) {
	h.mu.RLock()
	posts := append([]PostHook{}, h.posts...)
	h.mu.RUnlock()
	current := &out
	for _, fn := range posts {
		next, err := fn(ctx, in, current)
		if err != nil {
			return nil, err
		}
		if next == nil {
			return nil, nil
		}
		current = next
	}
	return current, nil
}
