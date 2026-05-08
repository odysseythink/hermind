package rl

import (
	"context"
	"testing"
	"time"
)

func TestManagerStartAndStop(t *testing.T) {
	m := NewManager()
	runID, err := m.Start(context.Background(), "sleep", []string{"10"})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if runID == "" {
		t.Fatal("empty run ID")
	}

	status := m.Status(runID)
	if status.State != "running" {
		t.Errorf("state = %q", status.State)
	}

	if err := m.Stop(runID); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	time.Sleep(100 * time.Millisecond)

	status = m.Status(runID)
	if status.State != "stopped" && status.State != "failed" {
		t.Errorf("state after stop = %q", status.State)
	}
}

func TestManagerStatusUnknown(t *testing.T) {
	m := NewManager()
	status := m.Status("nonexistent")
	if status.State != "unknown" {
		t.Errorf("state = %q", status.State)
	}
}

func TestManagerList(t *testing.T) {
	m := NewManager()
	id1, _ := m.Start(context.Background(), "sleep", []string{"10"})
	id2, _ := m.Start(context.Background(), "sleep", []string{"10"})
	runs := m.List()
	if len(runs) < 2 {
		t.Errorf("expected >= 2 runs, got %d", len(runs))
	}
	_ = m.Stop(id1)
	_ = m.Stop(id2)
}
