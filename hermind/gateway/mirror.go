package gateway

import (
	"context"
	"log/slog"
	"sync"
)

// Mirror replicates every outbound message to a list of extra
// platforms in addition to the originating one. Used to cross-post
// a Telegram reply into Discord, Slack, etc.
type Mirror struct {
	mu      sync.RWMutex
	targets []string // platform names to also receive the message
	gateway *Gateway
}

// NewMirror builds a Mirror that forwards to the given target
// platforms on top of the original. Target names must already be
// registered on the Gateway.
func NewMirror(g *Gateway, targets ...string) *Mirror {
	return &Mirror{targets: targets, gateway: g}
}

// AsPostHook returns a PostHook that copies the outgoing message to
// the Mirror's target platforms. It never drops the original reply.
func (m *Mirror) AsPostHook() PostHook {
	return func(ctx context.Context, in IncomingMessage, out *OutgoingMessage) (*OutgoingMessage, error) {
		if m.gateway == nil || out == nil {
			return out, nil
		}
		m.mu.RLock()
		targets := append([]string{}, m.targets...)
		m.mu.RUnlock()
		for _, t := range targets {
			if t == in.Platform {
				continue
			}
			p, ok := m.gateway.platforms[t]
			if !ok {
				continue
			}
			go func(p Platform) {
				if err := p.SendReply(ctx, *out); err != nil {
					slog.WarnContext(ctx, "gateway: mirror send failed", "platform", p.Name(), "err", err.Error())
				}
			}(p)
		}
		return out, nil
	}
}
