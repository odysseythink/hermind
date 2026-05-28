package workers

import "context"

// Job is the unified abstraction for a background task.
type Job interface {
	// Name returns the job identifier used in logs and monitoring.
	Name() string

	// Schedule returns a cron expression (e.g. "0 */12 * * *")
	// or a fixed interval (e.g. "@every 12h").
	// An empty string means the job is NOT auto-scheduled by cron.
	Schedule() string

	// Enabled is checked before each execution.
	// If it returns false the run is skipped for this cycle.
	Enabled(ctx context.Context) bool

	// Run executes the job body. ctx carries a per-job timeout.
	Run(ctx context.Context) error
}
