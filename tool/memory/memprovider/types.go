package memprovider

// InjectedMemory is a minimal view of a memory that has been surfaced to
// the agent for a turn. The ID lets downstream consumers (the feedback
// loop, the judge, telemetry) reference the source memory without
// re-querying storage.
type InjectedMemory struct {
	ID      string
	Content string
}
