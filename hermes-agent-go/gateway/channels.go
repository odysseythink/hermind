package gateway

import (
	"sort"
	"sync"
)

// Channel is a registered inbound channel — e.g. a Slack workspace,
// a Telegram group, a Matrix room. The gateway tracks these in a
// ChannelDirectory for status pages and routing.
type Channel struct {
	Platform string
	ID       string // platform-specific identifier
	Name     string // human-friendly
	Active   bool
}

// Key returns the canonical lookup key.
func (c Channel) Key() string { return c.Platform + ":" + c.ID }

// ChannelDirectory holds the set of registered channels.
type ChannelDirectory struct {
	mu       sync.RWMutex
	channels map[string]*Channel
}

func NewChannelDirectory() *ChannelDirectory {
	return &ChannelDirectory{channels: make(map[string]*Channel)}
}

func (d *ChannelDirectory) Upsert(c Channel) {
	d.mu.Lock()
	defer d.mu.Unlock()
	existing, ok := d.channels[c.Key()]
	if ok {
		existing.Name = c.Name
		existing.Active = c.Active
		return
	}
	// Store a copy so external mutations don't leak in.
	cp := c
	d.channels[c.Key()] = &cp
}

func (d *ChannelDirectory) Remove(platform, id string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.channels, Channel{Platform: platform, ID: id}.Key())
}

func (d *ChannelDirectory) All() []Channel {
	d.mu.RLock()
	defer d.mu.RUnlock()
	out := make([]Channel, 0, len(d.channels))
	for _, c := range d.channels {
		out = append(out, *c)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Platform != out[j].Platform {
			return out[i].Platform < out[j].Platform
		}
		return out[i].ID < out[j].ID
	})
	return out
}
