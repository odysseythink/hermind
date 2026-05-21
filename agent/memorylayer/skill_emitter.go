package memorylayer

import (
	"context"

	"github.com/odysseythink/mlog"
)

// SkillCandidate is a memory-layer signal that a boundary's turns may
// contain a reusable skill. The emitter does not decide; skills.Evolver
// remains the single writer.
type SkillCandidate struct {
	BoundaryReason string
	Turns          []TurnRef // last N turns; emitter trims to MaxTurns
}

// TurnRef is a lightweight turn snapshot carried in a SkillCandidate.
type TurnRef struct {
	ID        int64
	UserMsg   string
	Assistant string
}

type SkillEmitterConfig struct {
	Enabled  bool
	MaxTurns int // default 8; cap on how much context the candidate carries
}

func (c *SkillEmitterConfig) fill() {
	if c.MaxTurns <= 0 {
		c.MaxTurns = 8
	}
}

type SkillEmitter struct {
	cfg SkillEmitterConfig
	// sink is set by SetSink; nil means "no listener" → emit is a no-op.
	sink func(SkillCandidate)
}

func NewSkillEmitter(cfg SkillEmitterConfig) *SkillEmitter {
	cfg.fill()
	return &SkillEmitter{cfg: cfg}
}

func (e *SkillEmitter) SetSink(fn func(SkillCandidate)) { e.sink = fn }

func (e *SkillEmitter) Emit(ctx context.Context, b *Boundary) {
	if e == nil || !e.cfg.Enabled || e.sink == nil || b == nil || len(b.Turns) == 0 {
		return
	}
	n := len(b.Turns)
	if n > e.cfg.MaxTurns {
		n = e.cfg.MaxTurns
	}
	turns := make([]TurnRef, 0, n)
	for _, t := range b.Turns[len(b.Turns)-n:] {
		turns = append(turns, TurnRef{ID: t.ID, UserMsg: t.UserMsg, Assistant: t.Assistant})
	}
	defer func() {
		if r := recover(); r != nil {
			mlog.Warning("skill_emitter: sink panicked", mlog.Any("panic", r))
		}
	}()
	e.sink(SkillCandidate{BoundaryReason: b.Reason, Turns: turns})
}
