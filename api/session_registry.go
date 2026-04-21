package api

import "sync"

// SessionRegistry tracks which session IDs currently have a running
// Engine invocation, along with the cancel function that stops it.
// Zero value is not usable; use NewSessionRegistry.
type SessionRegistry struct {
	mu      sync.Mutex
	running map[string]func()
}

func NewSessionRegistry() *SessionRegistry {
	return &SessionRegistry{running: make(map[string]func())}
}

// Register stores cancel under id. Returns false if id is already present,
// in which case the caller is expected to reject the request (busy).
func (r *SessionRegistry) Register(id string, cancel func()) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.running[id]; ok {
		return false
	}
	r.running[id] = cancel
	return true
}

// Cancel invokes and removes the cancel func for id. Returns false if
// the id was not registered.
func (r *SessionRegistry) Cancel(id string) bool {
	r.mu.Lock()
	cancel, ok := r.running[id]
	if ok {
		delete(r.running, id)
	}
	r.mu.Unlock()
	if !ok {
		return false
	}
	cancel()
	return true
}

// Clear removes id without invoking its cancel func. Used by the
// runner goroutine on natural completion.
func (r *SessionRegistry) Clear(id string) {
	r.mu.Lock()
	delete(r.running, id)
	r.mu.Unlock()
}

func (r *SessionRegistry) IsBusy(id string) bool {
	r.mu.Lock()
	_, ok := r.running[id]
	r.mu.Unlock()
	return ok
}
