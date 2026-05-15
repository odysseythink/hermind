// Package collector adapts hermind's batch runner output into RL
// trajectory Episodes. The Collector implements batch.TrajectorySink
// (so Runner.WithSink accepts it directly) and forwards every
// completed item to a trajectory.Sink as a Tinker-compatible
// Episode.
//
// Splitting the adapter from the data model (rl/trajectory) keeps the
// JSON schema importable by downstream tools without pulling in the
// agent/batch package.
package collector

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/odysseythink/hermind/agent/batch"
	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/rl/trajectory"
	"github.com/odysseythink/pantheon/core"
)

// Collector is the bridge between agent/batch/ and rl/trajectory/.
// It is safe for concurrent use; the underlying trajectory.Sink is
// expected to serialize writes.
type Collector struct {
	sink trajectory.Sink
	meta trajectory.Meta
}

// New constructs a Collector that writes every batch trajectory to the
// provided sink. The meta template is merged with per-trajectory
// values (Model and Environment) so runs carry both static metadata
// (config_id, extra) and the dynamic model/env the batch actually
// used.
func New(sink trajectory.Sink, meta trajectory.Meta) *Collector {
	return &Collector{sink: sink, meta: meta}
}

// OnTrajectory implements batch.TrajectorySink. It converts the
// batch.Trajectory into a trajectory.Episode and hands it off to the
// underlying Sink. Returning an error aborts the batch run (see
// batch.Runner.Run), so we only surface errors the caller cannot
// recover from — a sink-write failure during a run is one such case.
func (c *Collector) OnTrajectory(ctx context.Context, tr *batch.Trajectory) error {
	if tr == nil {
		return fmt.Errorf("collector: nil trajectory")
	}
	ep := c.buildEpisode(tr)
	if err := c.sink.Write(ctx, ep); err != nil {
		return fmt.Errorf("collector: sink write %s: %w", tr.ID, err)
	}
	return nil
}

// Close flushes the underlying sink. Callers typically defer this at
// the end of a run.
func (c *Collector) Close() error {
	if c.sink == nil {
		return nil
	}
	return c.sink.Close()
}

// buildEpisode converts a batch.Trajectory into a trajectory.Episode.
// It prefers the full Messages list when present (so multi-turn runs
// render accurately) and falls back to the prompt + response pair for
// the MVP single-turn path the current runner produces.
func (c *Collector) buildEpisode(tr *batch.Trajectory) trajectory.Episode {
	meta := c.meta // copy the template so we don't mutate shared state
	if tr.Model != "" {
		meta.Model = tr.Model
	}
	if tr.Environment != "" {
		meta.Environment = tr.Environment
	}
	if !tr.StartedAt.IsZero() {
		meta.StartedAt = tr.StartedAt.Unix()
	}
	if !tr.FinishedAt.IsZero() {
		meta.EndedAt = tr.FinishedAt.Unix()
	}

	steps := stepsFromTrajectory(tr)

	return trajectory.Episode{
		EpisodeID: episodeID(tr.ID),
		Meta:      meta,
		Steps:     steps,
	}
}

func stepsFromTrajectory(tr *batch.Trajectory) []trajectory.Step {
	if len(tr.Messages) > 0 {
		steps := make([]trajectory.Step, 0, len(tr.Messages))
		for _, m := range tr.Messages {
			steps = append(steps, stepFromMessage(m))
		}
		return steps
	}
	// MVP fallback: the runner's processOne only fills prompt/response.
	steps := make([]trajectory.Step, 0, 2)
	if tr.Prompt != "" {
		steps = append(steps, trajectory.Step{From: "user", Value: tr.Prompt})
	}
	if tr.Response != "" {
		steps = append(steps, trajectory.Step{From: "assistant", Value: tr.Response})
	}
	return steps
}

func stepFromMessage(m message.HermindMessage) trajectory.Step {
	step := trajectory.Step{
		From:       roleToFrom(m.Role),
		Value:      m.Text(),
		ToolCallID: m.ToolCallID,
	}
	if m.Role == core.MESSAGE_ROLE_TOOL {
		for _, p := range m.Content {
			if tr, ok := p.(core.ToolResultPart); ok {
				step.ToolName = tr.Name
				break
			}
		}
	}
	return step
}

func roleToFrom(role core.MessageRoleType) string {
	switch role {
	case core.MESSAGE_ROLE_USER:
		return "user"
	case core.MESSAGE_ROLE_ASSISTANT:
		return "assistant"
	case core.MESSAGE_ROLE_TOOL:
		return "tool"
	case core.MESSAGE_ROLE_SYSTEM:
		return "system"
	}
	return string(role)
}

// episodeID reuses the batch item ID when present (so trajectories are
// traceable back to the dataset row) and falls back to a random ID
// for pathological cases where the item has no ID.
func episodeID(itemID string) string {
	if itemID != "" {
		return "ep-" + itemID
	}
	var buf [6]byte
	_, _ = rand.Read(buf[:])
	return "ep-" + hex.EncodeToString(buf[:])
}

// Compile-time check: Collector must satisfy batch.TrajectorySink so
// callers can pass it directly to Runner.WithSink.
var _ batch.TrajectorySink = (*Collector)(nil)
