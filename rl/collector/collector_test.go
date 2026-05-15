package collector

import (
	"context"
	"testing"
	"time"

	"github.com/odysseythink/hermind/agent/batch"
	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/rl/trajectory"
	"github.com/odysseythink/pantheon/core"
)

type recordingSink struct {
	episodes []trajectory.Episode
	closed   bool
}

func (r *recordingSink) Write(_ context.Context, ep trajectory.Episode) error {
	r.episodes = append(r.episodes, ep)
	return nil
}
func (r *recordingSink) Close() error { r.closed = true; return nil }

func TestCollector_OnTrajectory_PromptResponseFallback(t *testing.T) {
	sink := &recordingSink{}
	c := New(sink, trajectory.Meta{
		Environment: "test-env",
		ConfigID:    "run-v1",
	})

	tr := &batch.Trajectory{
		ID:          "item-1",
		Model:       "stub/model",
		Environment: "override-env",
		Prompt:      "hi",
		Response:    "hello",
		StartedAt:   time.Unix(1700000000, 0),
		FinishedAt:  time.Unix(1700000010, 0),
	}
	if err := c.OnTrajectory(context.Background(), tr); err != nil {
		t.Fatal(err)
	}
	if len(sink.episodes) != 1 {
		t.Fatalf("expected 1 episode, got %d", len(sink.episodes))
	}
	ep := sink.episodes[0]
	if ep.EpisodeID != "ep-item-1" {
		t.Errorf("episode id = %q", ep.EpisodeID)
	}
	if ep.Meta.Environment != "override-env" {
		t.Errorf("env = %q, want trajectory override", ep.Meta.Environment)
	}
	if ep.Meta.ConfigID != "run-v1" {
		t.Errorf("config_id = %q, want template", ep.Meta.ConfigID)
	}
	if ep.Meta.Model != "stub/model" {
		t.Errorf("model = %q", ep.Meta.Model)
	}
	if ep.Meta.StartedAt != 1700000000 || ep.Meta.EndedAt != 1700000010 {
		t.Errorf("timestamps = %d/%d", ep.Meta.StartedAt, ep.Meta.EndedAt)
	}
	if len(ep.Steps) != 2 || ep.Steps[0].From != "user" || ep.Steps[0].Value != "hi" ||
		ep.Steps[1].From != "assistant" || ep.Steps[1].Value != "hello" {
		t.Errorf("steps = %+v", ep.Steps)
	}
}

func TestCollector_OnTrajectory_UsesMessagesWhenPresent(t *testing.T) {
	sink := &recordingSink{}
	c := New(sink, trajectory.Meta{Model: "default-model"})

	tr := &batch.Trajectory{
		ID:    "item-2",
		Model: "m",
		Messages: []message.HermindMessage{
			{Role: core.MESSAGE_ROLE_USER, Content: core.NewTextContent("u")},
			{Role: core.MESSAGE_ROLE_ASSISTANT, Content: core.NewTextContent("a")},
			{Role: core.MESSAGE_ROLE_TOOL, Content: []core.ContentParter{core.ToolResultPart{ToolCallID: "c-1", Name: "search", Content: core.NewTextContent("t")}}, ToolCallID: "c-1"},
			{Role: core.MESSAGE_ROLE_SYSTEM, Content: core.NewTextContent("s")},
		},
		Prompt:   "should-not-appear",
		Response: "should-not-appear",
	}
	if err := c.OnTrajectory(context.Background(), tr); err != nil {
		t.Fatal(err)
	}
	ep := sink.episodes[0]
	if len(ep.Steps) != 4 {
		t.Fatalf("steps = %d", len(ep.Steps))
	}
	wantFrom := []string{"user", "assistant", "tool", "system"}
	for i, s := range ep.Steps {
		if s.From != wantFrom[i] {
			t.Errorf("steps[%d].from = %q, want %q", i, s.From, wantFrom[i])
		}
	}
	if ep.Steps[2].ToolName != "search" || ep.Steps[2].ToolCallID != "c-1" {
		t.Errorf("tool step metadata lost: %+v", ep.Steps[2])
	}
}

func TestCollector_OnTrajectory_NilReturnsError(t *testing.T) {
	c := New(&recordingSink{}, trajectory.Meta{})
	if err := c.OnTrajectory(context.Background(), nil); err == nil {
		t.Error("expected error for nil trajectory")
	}
}

func TestCollector_ImplementsBatchSink(t *testing.T) {
	// Compile-time assertion: a *Collector satisfies batch.TrajectorySink.
	var _ batch.TrajectorySink = New(&recordingSink{}, trajectory.Meta{})
}

func TestCollector_Close_ForwardsToSink(t *testing.T) {
	sink := &recordingSink{}
	c := New(sink, trajectory.Meta{})
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	if !sink.closed {
		t.Error("underlying sink was not closed")
	}
}

func TestCollector_EpisodeID_FallbackWhenEmpty(t *testing.T) {
	sink := &recordingSink{}
	c := New(sink, trajectory.Meta{})
	tr := &batch.Trajectory{Prompt: "p", Response: "r"}
	if err := c.OnTrajectory(context.Background(), tr); err != nil {
		t.Fatal(err)
	}
	id := sink.episodes[0].EpisodeID
	if id == "" || id == "ep-" {
		t.Errorf("expected fallback episode id, got %q", id)
	}
}
