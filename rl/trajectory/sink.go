package trajectory

import "context"

// Sink accepts completed episodes. Implementations are safe for
// concurrent use; callers never need their own lock.
//
// This interface is intentionally separate from agent/batch/'s
// TrajectorySink: batch's interface speaks in batch.Trajectory (a
// per-item provider response), while this one speaks in Episode
// (the Tinker-compatible RL structure). The adapter lives in
// rl/collector.
type Sink interface {
	// Write records one episode. Returns an error on transport
	// failure — callers typically log and continue.
	Write(ctx context.Context, ep Episode) error

	// Close flushes buffers and releases resources. Must be
	// idempotent: callers may defer Close even on early-exit paths.
	Close() error
}
