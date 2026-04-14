package gateway

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

// Pairing ties a platform user identity to a hermind profile via a
// short-lived one-time token. The typical UX is:
//   1. Operator runs `hermind pair create bob` → prints TOKEN.
//   2. Operator tells bob to DM "/pair TOKEN" on the messaging
//      platform.
//   3. The gateway calls Pairing.Redeem(TOKEN, platform, userID)
//      which records the link and wipes the token.
type Pairing struct {
	mu     sync.Mutex
	tokens map[string]*pairingToken // token -> record
	links  map[string]string        // "platform:userID" -> profileName
	ttl    time.Duration
}

type pairingToken struct {
	ProfileName string
	CreatedAt   time.Time
}

// NewPairing builds a pairing manager with the given token TTL.
// Zero ttl means tokens never expire.
func NewPairing(ttl time.Duration) *Pairing {
	return &Pairing{
		tokens: map[string]*pairingToken{},
		links:  map[string]string{},
		ttl:    ttl,
	}
}

// Create returns a new single-use pairing token for the profile.
func (p *Pairing) Create(profileName string) string {
	p.mu.Lock()
	defer p.mu.Unlock()
	var buf [16]byte
	_, _ = rand.Read(buf[:])
	token := hex.EncodeToString(buf[:])
	p.tokens[token] = &pairingToken{ProfileName: profileName, CreatedAt: time.Now().UTC()}
	return token
}

// Redeem consumes a token, linking (platform, userID) to the
// profile that created the token. Returns the profile name on
// success.
func (p *Pairing) Redeem(token, platform, userID string) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	rec, ok := p.tokens[token]
	if !ok {
		return "", errors.New("pairing: unknown token")
	}
	if p.ttl > 0 && time.Since(rec.CreatedAt) > p.ttl {
		delete(p.tokens, token)
		return "", errors.New("pairing: token expired")
	}
	delete(p.tokens, token)
	p.links[platform+":"+userID] = rec.ProfileName
	return rec.ProfileName, nil
}

// Lookup returns the profile linked to a platform user, or "".
func (p *Pairing) Lookup(platform, userID string) string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.links[platform+":"+userID]
}
