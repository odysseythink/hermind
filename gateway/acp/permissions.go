package acp

import "sync"

// Permissions holds a simple token-to-scope map. A scope is a
// colon-separated action string like "messages:send" or "tools:*".
type Permissions struct {
	mu     sync.RWMutex
	scopes map[string]map[string]bool // token -> set of scopes
}

func NewPermissions() *Permissions {
	return &Permissions{scopes: map[string]map[string]bool{}}
}

// Grant adds one or more scopes to a token.
func (p *Permissions) Grant(token string, scopes ...string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.scopes[token] == nil {
		p.scopes[token] = map[string]bool{}
	}
	for _, s := range scopes {
		p.scopes[token][s] = true
	}
}

// Revoke removes a token completely.
func (p *Permissions) Revoke(token string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.scopes, token)
}

// Allow returns whether the token is authorized for the given action.
// Wildcard scopes (e.g. "tools:*") match prefixes.
func (p *Permissions) Allow(token, action string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	set, ok := p.scopes[token]
	if !ok {
		return false
	}
	if set["*"] || set[action] {
		return true
	}
	// Wildcard prefix like "tools:*".
	for s := range set {
		if len(s) >= 2 && s[len(s)-1] == '*' {
			prefix := s[:len(s)-1]
			if len(action) >= len(prefix) && action[:len(prefix)] == prefix {
				return true
			}
		}
	}
	return false
}
