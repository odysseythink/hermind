// Package citesink provides context-based plumbing so the
// metaclaw_cite_memory tool can signal that a specific memory
// influenced the assistant reply, without creating a direct
// dependency between the tool registry and the engine.
package citesink

import "context"

type key struct{}

// WithSink attaches a citation sink to ctx. The returned context carries
// the sink through tool dispatch.
func WithSink(ctx context.Context, sink func(memoryID string)) context.Context {
	return context.WithValue(ctx, key{}, sink)
}

// Cite pushes a memory ID into whatever citation sink is on ctx. No-op
// when no sink is registered.
func Cite(ctx context.Context, memoryID string) {
	if sink, ok := ctx.Value(key{}).(func(string)); ok && sink != nil {
		sink(memoryID)
	}
}
