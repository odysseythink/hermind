package acp

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

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

// PermissionOutcome mirrors the ACP response.outcome.optionId values
// returned by session/request_permission.
type PermissionOutcome int

const (
	// PermissionDeny is the safe default: deny the tool call.
	PermissionDeny PermissionOutcome = iota
	// PermissionAllowOnce grants permission for this single call.
	PermissionAllowOnce
	// PermissionAllowAlways grants permission for subsequent identical
	// calls in the same session.
	PermissionAllowAlways
)

// String returns the camel-case form used in ACP messages.
func (p PermissionOutcome) String() string {
	switch p {
	case PermissionAllowOnce:
		return "allow_once"
	case PermissionAllowAlways:
		return "allow_always"
	default:
		return "deny"
	}
}

// PermissionBroker issues session/request_permission calls and waits
// for matching responses. It bridges hermind's synchronous tool
// approval hook to the ACP client's async JSON-RPC round-trip. On
// timeout it returns PermissionDeny so the agent stays safe-by-default
// when the editor is slow or absent.
type PermissionBroker struct {
	send    func([]byte)
	timeout time.Duration

	mu      sync.Mutex
	nextID  int64
	pending map[int64]chan PermissionOutcome
}

// NewPermissionBroker constructs a broker. send is the function the
// server uses to write outbound JSON-RPC frames (typically closed
// over the transport's stdout + mutex). A zero timeout defaults to
// 60 seconds matching the Python acp_adapter reference.
func NewPermissionBroker(send func([]byte), timeout time.Duration) *PermissionBroker {
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	return &PermissionBroker{
		send:    send,
		timeout: timeout,
		pending: map[int64]chan PermissionOutcome{},
	}
}

// Request sends a permission request and blocks until the client
// responds, the context is cancelled, or the timeout fires. Timeout
// and cancellation both resolve to PermissionDeny.
func (b *PermissionBroker) Request(ctx context.Context, sessionID, description, kind string) (PermissionOutcome, error) {
	id := atomic.AddInt64(&b.nextID, 1)
	ch := make(chan PermissionOutcome, 1)

	b.mu.Lock()
	b.pending[id] = ch
	b.mu.Unlock()

	frame, err := json.Marshal(struct {
		Version string      `json:"jsonrpc"`
		ID      int64       `json:"id"`
		Method  string      `json:"method"`
		Params  interface{} `json:"params"`
	}{
		Version: "2.0",
		ID:      id,
		Method:  "session/request_permission",
		Params: map[string]any{
			"sessionId": sessionID,
			"toolCall": map[string]any{
				"toolCallId": "perm-check",
				"title":      description,
				"kind":       kind,
			},
			"options": []map[string]string{
				{"optionId": "allow_once", "kind": "allow_once", "name": "Allow once"},
				{"optionId": "allow_always", "kind": "allow_always", "name": "Allow always"},
				{"optionId": "deny", "kind": "reject_once", "name": "Deny"},
			},
		},
	})
	if err != nil {
		b.clear(id)
		return PermissionDeny, err
	}
	frame = append(frame, '\n')
	b.send(frame)

	select {
	case outcome := <-ch:
		return outcome, nil
	case <-time.After(b.timeout):
		b.clear(id)
		return PermissionDeny, fmt.Errorf("acp: permission request timeout after %s", b.timeout)
	case <-ctx.Done():
		b.clear(id)
		return PermissionDeny, ctx.Err()
	}
}

// HandleResponse feeds an incoming JSON-RPC response back into the
// matching Request call. It is a no-op if the ID is not pending.
func (b *PermissionBroker) HandleResponse(raw []byte) {
	var resp struct {
		ID     int64 `json:"id"`
		Result struct {
			Outcome struct {
				OptionID string `json:"optionId"`
			} `json:"outcome"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return
	}
	outcome := PermissionDeny
	switch resp.Result.Outcome.OptionID {
	case "allow_once":
		outcome = PermissionAllowOnce
	case "allow_always":
		outcome = PermissionAllowAlways
	}

	b.mu.Lock()
	ch, ok := b.pending[resp.ID]
	delete(b.pending, resp.ID)
	b.mu.Unlock()
	if !ok {
		return
	}
	select {
	case ch <- outcome:
	default:
		// Already signaled (timeout path); drop.
	}
}

func (b *PermissionBroker) clear(id int64) {
	b.mu.Lock()
	delete(b.pending, id)
	b.mu.Unlock()
}
