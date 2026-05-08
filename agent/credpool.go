package agent

import (
	"errors"
	"sync"
	"sync/atomic"
)

// CredentialPool round-robins across a fixed set of API keys for one
// provider. It is safe for concurrent use. Disabled keys (e.g. on
// rate-limit error) are skipped until Reset is called.
type CredentialPool struct {
	mu       sync.Mutex
	keys     []string
	disabled []bool
	cursor   atomic.Int64
}

// NewCredentialPool builds a pool from a non-empty slice of keys.
func NewCredentialPool(keys []string) (*CredentialPool, error) {
	if len(keys) == 0 {
		return nil, errors.New("credpool: no keys")
	}
	return &CredentialPool{
		keys:     append([]string{}, keys...),
		disabled: make([]bool, len(keys)),
	}, nil
}

// Next returns the next usable key. Returns "" if every key is
// disabled (caller should fail fast).
func (p *CredentialPool) Next() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	n := len(p.keys)
	for i := 0; i < n; i++ {
		idx := int(p.cursor.Add(1)) % n
		if idx < 0 {
			idx += n
		}
		if !p.disabled[idx] {
			return p.keys[idx]
		}
	}
	return ""
}

// Disable marks a specific key as unusable (e.g. after a 429 / 401).
func (p *CredentialPool) Disable(key string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for i, k := range p.keys {
		if k == key {
			p.disabled[i] = true
			return
		}
	}
}

// Reset re-enables every key.
func (p *CredentialPool) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for i := range p.disabled {
		p.disabled[i] = false
	}
}

// Available reports how many keys are still usable.
func (p *CredentialPool) Available() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	n := 0
	for _, d := range p.disabled {
		if !d {
			n++
		}
	}
	return n
}
