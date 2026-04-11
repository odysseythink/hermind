package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestTrajectoryWriter(t *testing.T) {
	dir := t.TempDir()
	tw, err := NewTrajectoryWriter(dir, "sess-1")
	if err != nil {
		t.Fatal(err)
	}
	if err := tw.Write(TrajectoryEvent{Kind: "user", Content: "hi", SessionID: "sess-1"}); err != nil {
		t.Fatal(err)
	}
	if err := tw.Write(TrajectoryEvent{Kind: "assistant", Content: "hello", SessionID: "sess-1"}); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "sess-1.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 lines, got %d", len(lines))
	}
	if !strings.Contains(lines[0], "\"kind\":\"user\"") {
		t.Errorf("line 0 = %s", lines[0])
	}
}

func TestInsightsRecord(t *testing.T) {
	ins := NewInsights()
	ins.RecordToolCall("shell", 5*time.Millisecond, nil)
	ins.RecordToolCall("shell", 10*time.Millisecond, nil)
	ins.RecordToolCall("read", 1*time.Millisecond, nil)
	ins.RecordIteration()
	if ins.ToolCalls != 3 {
		t.Errorf("toolCalls = %d", ins.ToolCalls)
	}
	if top := ins.TopTools(1); len(top) != 1 || top[0] != "shell" {
		t.Errorf("top = %v", top)
	}
	if ins.MeanDuration() <= 0 {
		t.Error("expected positive mean duration")
	}
	ins.Finish()
}
