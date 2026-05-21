package memorylayer

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSkillEmitter_SinkNotSet(t *testing.T) {
	se := NewSkillEmitter(SkillEmitterConfig{Enabled: true, MaxTurns: 8})
	// No sink set → should be a no-op without panic.
	se.Emit(context.Background(), &Boundary{Turns: []Turn{{ID: 1, UserMsg: "hello"}}})
}

func TestSkillEmitter_EmitsAllTurns(t *testing.T) {
	se := NewSkillEmitter(SkillEmitterConfig{Enabled: true, MaxTurns: 8})
	var got SkillCandidate
	se.SetSink(func(cand SkillCandidate) {
		got = cand
	})
	se.Emit(context.Background(), &Boundary{
		Reason: "hard_turn",
		Turns: []Turn{
			{ID: 1, UserMsg: "a", Assistant: "A"},
			{ID: 2, UserMsg: "b", Assistant: "B"},
			{ID: 3, UserMsg: "c", Assistant: "C"},
			{ID: 4, UserMsg: "d", Assistant: "D"},
			{ID: 5, UserMsg: "e", Assistant: "E"},
		},
	})
	assert.Equal(t, "hard_turn", got.BoundaryReason)
	assert.Len(t, got.Turns, 5)
	assert.Equal(t, int64(1), got.Turns[0].ID)
	assert.Equal(t, "e", got.Turns[4].UserMsg)
}

func TestSkillEmitter_TrimsToMaxTurns(t *testing.T) {
	se := NewSkillEmitter(SkillEmitterConfig{Enabled: true, MaxTurns: 3})
	var got SkillCandidate
	se.SetSink(func(cand SkillCandidate) {
		got = cand
	})
	se.Emit(context.Background(), &Boundary{
		Reason: "hard_turn",
		Turns: []Turn{
			{ID: 1, UserMsg: "a"},
			{ID: 2, UserMsg: "b"},
			{ID: 3, UserMsg: "c"},
			{ID: 4, UserMsg: "d"},
			{ID: 5, UserMsg: "e"},
		},
	})
	assert.Len(t, got.Turns, 3)
	assert.Equal(t, int64(3), got.Turns[0].ID)
	assert.Equal(t, int64(5), got.Turns[2].ID)
}

func TestSkillEmitter_SinkPanicRecovered(t *testing.T) {
	se := NewSkillEmitter(SkillEmitterConfig{Enabled: true, MaxTurns: 8})
	se.SetSink(func(cand SkillCandidate) {
		panic("intentional")
	})
	// Should not propagate panic.
	se.Emit(context.Background(), &Boundary{Turns: []Turn{{ID: 1, UserMsg: "hello"}}})
}

func TestSkillEmitter_Disabled(t *testing.T) {
	se := NewSkillEmitter(SkillEmitterConfig{Enabled: false})
	var called bool
	se.SetSink(func(cand SkillCandidate) {
		called = true
	})
	se.Emit(context.Background(), &Boundary{Turns: []Turn{{ID: 1, UserMsg: "hello"}}})
	assert.False(t, called)
}

func TestSkillEmitter_NilBoundary(t *testing.T) {
	se := NewSkillEmitter(SkillEmitterConfig{Enabled: true})
	var called bool
	se.SetSink(func(cand SkillCandidate) {
		called = true
	})
	se.Emit(context.Background(), nil)
	assert.False(t, called)
}
