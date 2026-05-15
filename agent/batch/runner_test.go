package batch

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/odysseythink/pantheon/core"
)

// fakeProvider returns a canned response. Used so the runner tests
// don't need a network or real model.
type fakeProvider struct {
	mu    sync.Mutex
	calls int
}

func (f *fakeProvider) Provider() string { return "fake" }
func (f *fakeProvider) Model() string    { return "fake/model" }

func (f *fakeProvider) Generate(ctx context.Context, req *core.Request) (*core.Response, error) {
	f.mu.Lock()
	f.calls++
	f.mu.Unlock()
	last := req.Messages[len(req.Messages)-1]
	var prompt string
	for _, part := range last.Content {
		if p, ok := part.(core.TextPart); ok {
			prompt = p.Text
			break
		}
	}
	return &core.Response{
		Message: core.Message{
			Role:    core.MESSAGE_ROLE_ASSISTANT,
			Content: []core.ContentParter{core.TextPart{Text: "ok: " + prompt}},
		},
		FinishReason: "end_turn",
		Model:        "fake/model",
		Usage:        core.Usage{PromptTokens: 10, CompletionTokens: 5},
	}, nil
}

func (f *fakeProvider) Stream(context.Context, *core.Request) (core.StreamResponse, error) {
	return nil, errors.New("fake: streaming not implemented")
}

func (f *fakeProvider) GenerateObject(context.Context, *core.ObjectRequest) (*core.ObjectResponse, error) {
	return nil, errors.New("fake: GenerateObject not implemented")
}

func writeDataset(t *testing.T, dir string, ids ...string) string {
	t.Helper()
	path := filepath.Join(dir, "d.jsonl")
	var buf []byte
	for _, id := range ids {
		buf = append(buf, []byte(`{"id":"`+id+`","prompt":"hello `+id+`"}`+"\n")...)
	}
	if err := os.WriteFile(path, buf, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRunner_WritesTrajectoryPerItem(t *testing.T) {
	dir := t.TempDir()
	dataset := writeDataset(t, dir, "a", "b", "c")
	cfg := &Config{
		Model:       "fake/model",
		DatasetFile: dataset,
		OutputDir:   filepath.Join(dir, "out"),
		NumWorkers:  2,
		MaxTokens:   1024,
	}

	r := NewRunner(cfg, &fakeProvider{})
	if err := r.Run(context.Background()); err != nil {
		t.Fatalf("run: %v", err)
	}

	for _, id := range []string{"a", "b", "c"} {
		p := filepath.Join(cfg.OutputDir, "trajectories", id+".jsonl")
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("missing trajectory %s: %v", id, err)
		}
		if len(data) == 0 {
			t.Errorf("empty trajectory %s", id)
		}
	}

	seen, _ := LoadCheckpointSet(filepath.Join(cfg.OutputDir, "checkpoint.jsonl"))
	if len(seen) != 3 {
		t.Errorf("checkpoint has %d entries, want 3", len(seen))
	}
}

func TestRunner_ResumeSkipsCompleted(t *testing.T) {
	dir := t.TempDir()
	dataset := writeDataset(t, dir, "a", "b", "c")
	cfg := &Config{
		Model:       "fake/model",
		DatasetFile: dataset,
		OutputDir:   filepath.Join(dir, "out"),
		NumWorkers:  1,
		MaxTokens:   1024,
		Resume:      true,
	}

	cpPath := filepath.Join(cfg.OutputDir, "checkpoint.jsonl")
	cp, err := OpenCheckpoint(cpPath)
	if err != nil {
		t.Fatal(err)
	}
	_ = cp.MarkDone("a")
	_ = cp.Close()

	fp := &fakeProvider{}
	r := NewRunner(cfg, fp)
	if err := r.Run(context.Background()); err != nil {
		t.Fatal(err)
	}

	if fp.calls != 2 {
		t.Errorf("expected 2 calls (b+c), got %d", fp.calls)
	}
	if _, err := os.Stat(filepath.Join(cfg.OutputDir, "trajectories", "a.jsonl")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("expected a.jsonl skipped, stat err = %v", err)
	}
}

func TestRunner_TrajectoryShape(t *testing.T) {
	dir := t.TempDir()
	dataset := writeDataset(t, dir, "only")
	cfg := &Config{
		Model:       "fake/model",
		DatasetFile: dataset,
		OutputDir:   filepath.Join(dir, "out"),
		NumWorkers:  1,
		MaxTokens:   1024,
	}
	r := NewRunner(cfg, &fakeProvider{})
	if err := r.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(cfg.OutputDir, "trajectories", "only.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var meta map[string]any
	if err := json.Unmarshal(firstLine(data), &meta); err != nil {
		t.Fatalf("meta: %v", err)
	}
	if meta["id"] != "only" {
		t.Errorf("meta.id = %v", meta["id"])
	}
	if meta["model"] != "fake/model" {
		t.Errorf("meta.model = %v", meta["model"])
	}
}

func TestRunner_TrajectorySinkReceivesEveryItem(t *testing.T) {
	dir := t.TempDir()
	dataset := writeDataset(t, dir, "x", "y")
	cfg := &Config{
		Model:       "fake/model",
		DatasetFile: dataset,
		OutputDir:   filepath.Join(dir, "out"),
		NumWorkers:  2,
		MaxTokens:   1024,
	}

	var (
		mu   sync.Mutex
		seen []string
	)
	sink := TrajectorySinkFunc(func(ctx context.Context, tr *Trajectory) error {
		mu.Lock()
		defer mu.Unlock()
		seen = append(seen, tr.ID)
		if tr.Response == "" {
			t.Errorf("expected response, got empty")
		}
		return nil
	})

	r := NewRunner(cfg, &fakeProvider{}).WithSink(sink)
	if err := r.Run(context.Background()); err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(seen) != 2 {
		t.Errorf("sink saw %d items, want 2", len(seen))
	}
}

func TestRunner_SinkErrorAbortsCheckpoint(t *testing.T) {
	dir := t.TempDir()
	dataset := writeDataset(t, dir, "a")
	cfg := &Config{
		Model:       "fake/model",
		DatasetFile: dataset,
		OutputDir:   filepath.Join(dir, "out"),
		NumWorkers:  1,
		MaxTokens:   1024,
	}

	boom := errors.New("sink boom")
	sink := TrajectorySinkFunc(func(ctx context.Context, tr *Trajectory) error { return boom })
	r := NewRunner(cfg, &fakeProvider{}).WithSink(sink)
	err := r.Run(context.Background())
	if err == nil {
		t.Fatal("expected sink error to propagate")
	}
	seen, _ := LoadCheckpointSet(filepath.Join(cfg.OutputDir, "checkpoint.jsonl"))
	if seen["a"] {
		t.Errorf("expected checkpoint NOT to record 'a' when sink failed")
	}
}

func firstLine(data []byte) []byte {
	for i, b := range data {
		if b == '\n' {
			return data[:i]
		}
	}
	return data
}
