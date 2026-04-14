package gateway

import "sync"

// Dedup is a small FIFO-bounded in-memory cache of recently seen
// message IDs. It is intentionally simple: when capacity is reached
// the oldest entries are dropped.
type Dedup struct {
	mu       sync.Mutex
	capacity int
	order    []string
	set      map[string]struct{}
}

func NewDedup(capacity int) *Dedup {
	if capacity <= 0 {
		capacity = 1024
	}
	return &Dedup{
		capacity: capacity,
		set:      make(map[string]struct{}, capacity),
	}
}

// Seen returns true if key was already observed, inserting it otherwise.
func (d *Dedup) Seen(key string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, ok := d.set[key]; ok {
		return true
	}
	d.set[key] = struct{}{}
	d.order = append(d.order, key)
	if len(d.order) > d.capacity {
		drop := d.order[0]
		d.order = d.order[1:]
		delete(d.set, drop)
	}
	return false
}
