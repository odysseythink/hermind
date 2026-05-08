// agent/budget.go
package agent

import "sync/atomic"

// Budget tracks the remaining iteration budget for a conversation.
// Thread-safe via atomic.Int32. The zero value is not valid; use NewBudget.
type Budget struct {
	max       int
	remaining atomic.Int32
}

// NewBudget constructs a Budget with max iterations.
func NewBudget(max int) *Budget {
	b := &Budget{max: max}
	b.remaining.Store(int32(max))
	return b
}

// Consume attempts to use one iteration. Returns true if the budget was
// decremented while non-negative, false if it went negative.
func (b *Budget) Consume() bool {
	return b.remaining.Add(-1) >= 0
}

// Refund returns one iteration to the budget (used for tools that invoke
// code execution where programmatic tool calls should not count).
func (b *Budget) Refund() {
	b.remaining.Add(1)
}

// Remaining returns the current remaining iteration count.
// May be negative if Consume was called after exhaustion.
func (b *Budget) Remaining() int {
	return int(b.remaining.Load())
}

// Ratio returns the fraction of the budget consumed, from 0.0 (fresh)
// to 1.0 (exhausted).
func (b *Budget) Ratio() float64 {
	if b.max == 0 {
		return 0
	}
	used := b.max - int(b.remaining.Load())
	return float64(used) / float64(b.max)
}
