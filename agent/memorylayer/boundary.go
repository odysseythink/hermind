package memorylayer

import (
	"context"
	"sync"
	"time"

	"github.com/odysseythink/hermind/tool/embedding"
	pembed "github.com/odysseythink/pantheon/extensions/embed"
)

type BoundaryConfig struct {
	HardTokenLimit            int           // default 8000
	HardTurnLimit             int           // default 20
	SoftTokenThreshold        int           // default 1500 (below this, topic shift not checked)
	IdleGap                   time.Duration // default 10 * time.Minute
	TopicShiftCosineThreshold float64       // default 0.55 (cosine < threshold = shift)
	EnableTopicShift          bool          // default true
}

func (c *BoundaryConfig) fill() {
	if c.HardTokenLimit <= 0 {
		c.HardTokenLimit = 8000
	}
	if c.HardTurnLimit <= 0 {
		c.HardTurnLimit = 20
	}
	if c.SoftTokenThreshold <= 0 {
		c.SoftTokenThreshold = 1500
	}
	if c.IdleGap <= 0 {
		c.IdleGap = 10 * time.Minute
	}
	if c.TopicShiftCosineThreshold <= 0 {
		c.TopicShiftCosineThreshold = 0.55
	}
}

type Turn struct {
	ID        int64
	UserMsg   string
	Assistant string
	Tokens    int
	Timestamp time.Time
	Embedding []float32 // optional; computed lazily for topic shift
}

type Boundary struct {
	Turns      []Turn
	TokenCount int
	Reason     string // "hard_token" | "hard_turn" | "idle" | "topic_shift" | "flush"
}

type BoundaryDetector struct {
	cfg      BoundaryConfig
	embedder embedding.Embedder // optional; required only if EnableTopicShift

	mu     sync.Mutex
	buf    []Turn
	tokens int
}

func NewBoundaryDetector(cfg BoundaryConfig, emb embedding.Embedder) *BoundaryDetector {
	cfg.fill()
	return &BoundaryDetector{cfg: cfg, embedder: emb}
}

// Observe appends a turn. Returns a non-nil Boundary if a boundary just
// triggered; the buffer is reset in that case.
func (d *BoundaryDetector) Observe(ctx context.Context, t Turn) *Boundary {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Idle gap is computed against the LAST turn in the existing buffer,
	// BEFORE appending the new one.
	if len(d.buf) > 0 && t.Timestamp.Sub(d.buf[len(d.buf)-1].Timestamp) > d.cfg.IdleGap {
		b := d.snapshotAndReset("idle")
		d.buf = append(d.buf, t)
		d.tokens = t.Tokens
		return b
	}

	d.buf = append(d.buf, t)
	d.tokens += t.Tokens

	switch {
	case d.tokens >= d.cfg.HardTokenLimit:
		return d.snapshotAndReset("hard_token")
	case len(d.buf) >= d.cfg.HardTurnLimit:
		return d.snapshotAndReset("hard_turn")
	case d.cfg.EnableTopicShift && d.tokens >= d.cfg.SoftTokenThreshold && d.detectTopicShift(ctx):
		return d.snapshotAndReset("topic_shift")
	}
	return nil
}

// Flush emits whatever is buffered with reason="flush". Used at shutdown.
func (d *BoundaryDetector) Flush() *Boundary {
	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.buf) == 0 {
		return nil
	}
	return d.snapshotAndReset("flush")
}

func (d *BoundaryDetector) snapshotAndReset(reason string) *Boundary {
	b := &Boundary{
		Turns:      append([]Turn(nil), d.buf...),
		TokenCount: d.tokens,
		Reason:     reason,
	}
	d.buf = d.buf[:0]
	d.tokens = 0
	return b
}

// detectTopicShift compares the embedding of buf[0] vs buf[last]. It
// computes embeddings lazily and caches them on the turns in the buffer.
func (d *BoundaryDetector) detectTopicShift(ctx context.Context) bool {
	if d.embedder == nil || len(d.buf) < 2 {
		return false
	}
	head := &d.buf[0]
	tail := &d.buf[len(d.buf)-1]
	if head.Embedding == nil {
		if v, err := d.embedder.Embed(ctx, head.UserMsg); err == nil {
			head.Embedding = v
		}
	}
	if tail.Embedding == nil {
		if v, err := d.embedder.Embed(ctx, tail.UserMsg); err == nil {
			tail.Embedding = v
		}
	}
	if head.Embedding == nil || tail.Embedding == nil {
		return false
	}
	cos := pembed.Cosine(head.Embedding, tail.Embedding)
	return float64(cos) < d.cfg.TopicShiftCosineThreshold
}
