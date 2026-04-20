# Batch Runner MVP Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `hermind batch run <config.yaml>` subcommand that runs the agent against a JSONL dataset of prompts in parallel, saves one trajectory JSONL per prompt, and supports `--resume` so a crash doesn't throw away partial work. MVP scope — no eval harness, no trajectory compression, no multi-env support.

**Architecture:** New `agent/batch/` package holds all batch-specific logic so the CLI layer stays thin. A `Runner` drives a goroutine pool (`errgroup.Group` with a semaphore), each worker constructs a fresh `agent.Engine` per prompt, runs it to a terminal message, and appends the full message history to `<output_dir>/trajectories/<id>.jsonl`. A line-oriented `checkpoint.jsonl` file records completed IDs; on `--resume`, the runner reads it and skips those IDs. Config is loaded with `yaml.v3` and mirrors the Python `datagen-config-examples/*.yaml` field names so users can bring their old configs over.

**Tech Stack:** Go 1.21+, `gopkg.in/yaml.v3`, `golang.org/x/sync/errgroup` (already in go.mod via transitive deps — verify in Task 1), existing `agent/`, `provider/factory/`, `tool/`, `message/` packages, `cobra` for CLI.

---

## File Structure

- Create: `agent/batch/config.go` — `Config` struct matching Python's `web_research.yaml`
- Create: `agent/batch/config_test.go`
- Create: `agent/batch/dataset.go` — JSONL reader that yields `Item{ID, Prompt}`
- Create: `agent/batch/dataset_test.go`
- Create: `agent/batch/checkpoint.go` — append-only completion log + reader
- Create: `agent/batch/checkpoint_test.go`
- Create: `agent/batch/runner.go` — the `Runner` struct, `Run(ctx)` loop, worker logic
- Create: `agent/batch/runner_test.go` — uses a stub `provider.Provider` so no network is needed
- Create: `cli/batch.go` — `hermind batch run <config>` cobra command
- Create: `cli/batch_test.go`
- Modify: `cli/root.go` — register `newBatchCmd(app)`
- Modify: `go.mod` — ensure `golang.org/x/sync` is a direct dependency

---

## Task 1: Ensure errgroup is available

**Files:** `go.mod`, `go.sum`

- [ ] **Step 1: Check current status**

Run: `go list -m golang.org/x/sync`
If it reports `error: module not found`, it's not a direct dep yet.

- [ ] **Step 2: Add it**

Run: `go get golang.org/x/sync@latest`
Expected: `go.mod` gains `golang.org/x/sync vX.Y.Z` in the `require` block.

- [ ] **Step 3: Build**

Run: `go build ./...`
Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add go.mod go.sum
git commit -m "build: add golang.org/x/sync for batch runner"
```

---

## Task 2: Batch config types

**Files:**
- Create: `agent/batch/config.go`
- Create: `agent/batch/config_test.go`

- [ ] **Step 1: Write the failing test**

Create `agent/batch/config_test.go`:

```go
package batch

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_FullShape(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.yaml")
	content := []byte(`environment: web-research
toolsets: [web, file]
num_workers: 4
batch_size: 20
max_items: 500
model: openrouter/meta-llama/llama-3-70b
dataset_file: data/in.jsonl
output_dir: data/out
ephemeral_system_prompt: |
  You are a helpful agent.
`)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Environment != "web-research" {
		t.Errorf("env = %q", cfg.Environment)
	}
	if cfg.NumWorkers != 4 || cfg.BatchSize != 20 || cfg.MaxItems != 500 {
		t.Errorf("nums = %d/%d/%d", cfg.NumWorkers, cfg.BatchSize, cfg.MaxItems)
	}
	if cfg.Model != "openrouter/meta-llama/llama-3-70b" {
		t.Errorf("model = %q", cfg.Model)
	}
	if cfg.DatasetFile != "data/in.jsonl" {
		t.Errorf("dataset = %q", cfg.DatasetFile)
	}
	if cfg.OutputDir != "data/out" {
		t.Errorf("out = %q", cfg.OutputDir)
	}
	if len(cfg.Toolsets) != 2 || cfg.Toolsets[0] != "web" {
		t.Errorf("toolsets = %v", cfg.Toolsets)
	}
	if cfg.EphemeralSystemPrompt == "" {
		t.Errorf("prompt empty")
	}
}

func TestLoadConfig_Defaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.yaml")
	if err := os.WriteFile(path, []byte(`model: x
dataset_file: in.jsonl
output_dir: out
`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.NumWorkers != 1 {
		t.Errorf("expected default num_workers=1, got %d", cfg.NumWorkers)
	}
	if cfg.BatchSize != 1 {
		t.Errorf("expected default batch_size=1, got %d", cfg.BatchSize)
	}
}

func TestLoadConfig_ValidatesRequiredFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.yaml")
	// missing model + dataset_file
	if err := os.WriteFile(path, []byte(`output_dir: out`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadConfig(path); err == nil {
		t.Error("expected error when required fields missing")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./agent/batch/ -run TestLoadConfig -v`
Expected: FAIL — package does not exist.

- [ ] **Step 3: Implement config**

Create `agent/batch/config.go`:

```go
// Package batch runs hermind agents against a dataset of prompts in
// parallel and saves the resulting trajectories. It mirrors the
// Python batch_runner.py feature set (MVP scope — no eval harness,
// no trajectory compression).
package batch

import (
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config describes a single batch run. YAML keys match the Python
// datagen-config-examples/*.yaml layout so configs are portable.
type Config struct {
	// Required
	Model       string `yaml:"model"`        // e.g. "openrouter/x/y" or "bedrock/anthropic.claude-opus-4-v1:0"
	DatasetFile string `yaml:"dataset_file"` // JSONL path; each line must have at least a "prompt" field
	OutputDir   string `yaml:"output_dir"`   // directory to write trajectories + checkpoint into

	// Optional — workload shape
	Environment string   `yaml:"environment,omitempty"` // cosmetic label stored in trajectory metadata
	Toolsets    []string `yaml:"toolsets,omitempty"`    // names resolved against the tool registry
	NumWorkers  int      `yaml:"num_workers,omitempty"` // default 1
	BatchSize   int      `yaml:"batch_size,omitempty"`  // default 1 — passed through as metadata
	MaxItems    int      `yaml:"max_items,omitempty"`   // 0 means "no cap"

	// Optional — per-run system prompt additions
	EphemeralSystemPrompt string `yaml:"ephemeral_system_prompt,omitempty"`
}

// LoadConfig reads a YAML file, applies defaults, and validates the
// required fields.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("batch: read %s: %w", path, err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("batch: parse %s: %w", path, err)
	}
	applyDefaults(&cfg)
	if err := validate(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func applyDefaults(cfg *Config) {
	if cfg.NumWorkers <= 0 {
		cfg.NumWorkers = 1
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 1
	}
}

func validate(cfg *Config) error {
	if cfg.Model == "" {
		return errors.New("batch: config: model is required")
	}
	if cfg.DatasetFile == "" {
		return errors.New("batch: config: dataset_file is required")
	}
	if cfg.OutputDir == "" {
		return errors.New("batch: config: output_dir is required")
	}
	return nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./agent/batch/ -run TestLoadConfig -v`
Expected: PASS (all 3 sub-tests).

- [ ] **Step 5: Commit**

```bash
git add agent/batch/config.go agent/batch/config_test.go
git commit -m "feat(agent/batch): YAML config loader matching Python datagen layout"
```

---

## Task 3: JSONL dataset reader

**Files:**
- Create: `agent/batch/dataset.go`
- Create: `agent/batch/dataset_test.go`

- [ ] **Step 1: Write the failing test**

Create `agent/batch/dataset_test.go`:

```go
package batch

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadDataset_BasicShape(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "d.jsonl")
	lines := "" +
		`{"id":"q1","prompt":"what is 2+2?"}` + "\n" +
		`{"prompt":"no id here"}` + "\n" +
		`{"id":"q3","prompt":"third"}` + "\n"
	if err := os.WriteFile(path, []byte(lines), 0o644); err != nil {
		t.Fatal(err)
	}

	items, err := ReadDataset(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 {
		t.Fatalf("len = %d", len(items))
	}
	if items[0].ID != "q1" || items[0].Prompt != "what is 2+2?" {
		t.Errorf("items[0] = %#v", items[0])
	}
	// Missing ID is auto-assigned from line number (1-indexed).
	if items[1].ID != "line-2" {
		t.Errorf("items[1].ID = %q", items[1].ID)
	}
}

func TestReadDataset_MaxItems(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "d.jsonl")
	lines := `{"id":"a","prompt":"a"}` + "\n" +
		`{"id":"b","prompt":"b"}` + "\n" +
		`{"id":"c","prompt":"c"}` + "\n"
	_ = os.WriteFile(path, []byte(lines), 0o644)

	items, err := ReadDataset(path, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Errorf("len = %d", len(items))
	}
}

func TestReadDataset_SkipsBlankLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "d.jsonl")
	lines := `{"id":"a","prompt":"a"}` + "\n\n" +
		`   ` + "\n" +
		`{"id":"b","prompt":"b"}` + "\n"
	_ = os.WriteFile(path, []byte(lines), 0o644)

	items, _ := ReadDataset(path, 0)
	if len(items) != 2 {
		t.Errorf("len = %d", len(items))
	}
}

func TestReadDataset_RejectsMalformed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "d.jsonl")
	lines := `not json`
	_ = os.WriteFile(path, []byte(lines), 0o644)
	_, err := ReadDataset(path, 0)
	if err == nil {
		t.Error("expected parse error")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./agent/batch/ -run TestReadDataset -v`
Expected: FAIL — `ReadDataset` undefined.

- [ ] **Step 3: Implement the reader**

Create `agent/batch/dataset.go`:

```go
package batch

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Item is a single dataset row.
type Item struct {
	// ID is a stable, unique identifier used for trajectory filenames
	// and checkpoint entries. If the row did not supply an "id" field,
	// ReadDataset assigns "line-<N>" (1-indexed).
	ID string `json:"id"`
	// Prompt is the user message fed to the agent.
	Prompt string `json:"prompt"`
	// Raw preserves any extra fields for downstream consumers that care
	// (e.g. ground-truth answers kept outside of the agent loop).
	Raw json.RawMessage `json:"-"`
}

// ReadDataset parses a JSONL file into Items. If maxItems > 0 the
// slice is truncated to that length. Blank/whitespace-only lines are
// skipped; malformed JSON returns an error (with the offending line
// number so the user can fix the file).
func ReadDataset(path string, maxItems int) ([]Item, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("batch: open dataset: %w", err)
	}
	defer f.Close()

	items := make([]Item, 0, 64)
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1<<16), 1<<22) // allow up to 4 MiB per line
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var it Item
		if err := json.Unmarshal([]byte(line), &it); err != nil {
			return nil, fmt.Errorf("batch: dataset: line %d: %w", lineNum, err)
		}
		it.Raw = json.RawMessage(line)
		if it.ID == "" {
			it.ID = fmt.Sprintf("line-%d", lineNum)
		}
		items = append(items, it)
		if maxItems > 0 && len(items) >= maxItems {
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("batch: dataset scan: %w", err)
	}
	return items, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./agent/batch/ -run TestReadDataset -v`
Expected: PASS (all 4 sub-tests).

- [ ] **Step 5: Commit**

```bash
git add agent/batch/dataset.go agent/batch/dataset_test.go
git commit -m "feat(agent/batch): JSONL dataset reader with auto-ID and max_items"
```

---

## Task 4: Append-only checkpoint log

**Files:**
- Create: `agent/batch/checkpoint.go`
- Create: `agent/batch/checkpoint_test.go`

- [ ] **Step 1: Write the failing test**

Create `agent/batch/checkpoint_test.go`:

```go
package batch

import (
	"path/filepath"
	"testing"
)

func TestCheckpoint_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "checkpoint.jsonl")

	cp, err := OpenCheckpoint(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := cp.MarkDone("q1"); err != nil {
		t.Fatal(err)
	}
	if err := cp.MarkDone("q2"); err != nil {
		t.Fatal(err)
	}
	_ = cp.Close()

	// Re-open and confirm both IDs come back.
	seen, err := LoadCheckpointSet(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(seen) != 2 {
		t.Fatalf("len = %d", len(seen))
	}
	if !seen["q1"] || !seen["q2"] {
		t.Errorf("missing id: %+v", seen)
	}
}

func TestLoadCheckpointSet_MissingFileIsEmpty(t *testing.T) {
	dir := t.TempDir()
	seen, err := LoadCheckpointSet(filepath.Join(dir, "none.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if len(seen) != 0 {
		t.Errorf("expected empty, got %v", seen)
	}
}

func TestCheckpoint_ConcurrentMarkDone(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "checkpoint.jsonl")

	cp, err := OpenCheckpoint(path)
	if err != nil {
		t.Fatal(err)
	}

	done := make(chan struct{}, 50)
	for i := 0; i < 50; i++ {
		i := i
		go func() {
			_ = cp.MarkDone(string(rune('A' + (i % 26))))
			done <- struct{}{}
		}()
	}
	for i := 0; i < 50; i++ {
		<-done
	}
	_ = cp.Close()

	seen, _ := LoadCheckpointSet(path)
	if len(seen) < 1 {
		t.Errorf("expected at least 1 entry, got %d", len(seen))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./agent/batch/ -run TestCheckpoint -v`
Expected: FAIL — type undefined.

- [ ] **Step 3: Implement the checkpoint**

Create `agent/batch/checkpoint.go`:

```go
package batch

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Checkpoint is a line-oriented, append-only log of completed item
// IDs. On resume, callers call LoadCheckpointSet to get the set of
// already-finished IDs, then skip them.
//
// MarkDone is safe for concurrent use; writes are serialized by an
// internal mutex.
type Checkpoint struct {
	mu sync.Mutex
	f  *os.File
	bw *bufio.Writer
}

type checkpointEntry struct {
	ID string `json:"id"`
}

// OpenCheckpoint opens (creating if missing) the checkpoint file for
// append. The directory is created as needed.
func OpenCheckpoint(path string) (*Checkpoint, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("batch: mkdir checkpoint: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("batch: open checkpoint: %w", err)
	}
	return &Checkpoint{f: f, bw: bufio.NewWriter(f)}, nil
}

// MarkDone appends an entry for id. The write is flushed + fsync'd
// before returning so a crash does not lose the record.
func (c *Checkpoint) MarkDone(id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	line, err := json.Marshal(checkpointEntry{ID: id})
	if err != nil {
		return err
	}
	if _, err := c.bw.Write(line); err != nil {
		return err
	}
	if _, err := c.bw.WriteString("\n"); err != nil {
		return err
	}
	if err := c.bw.Flush(); err != nil {
		return err
	}
	return c.f.Sync()
}

// Close flushes remaining buffers and closes the file.
func (c *Checkpoint) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.bw != nil {
		_ = c.bw.Flush()
	}
	if c.f != nil {
		return c.f.Close()
	}
	return nil
}

// LoadCheckpointSet reads the file and returns the set of completed
// IDs. A missing file is not an error — returns an empty map.
func LoadCheckpointSet(path string) (map[string]bool, error) {
	seen := map[string]bool{}
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return seen, nil
	}
	if err != nil {
		return nil, fmt.Errorf("batch: read checkpoint: %w", err)
	}
	defer f.Close()

	r := bufio.NewReader(f)
	for {
		line, err := r.ReadString('\n')
		s := strings.TrimSpace(line)
		if s != "" {
			var e checkpointEntry
			if json.Unmarshal([]byte(s), &e) == nil && e.ID != "" {
				seen[e.ID] = true
			}
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("batch: read checkpoint: %w", err)
		}
	}
	return seen, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./agent/batch/ -run TestCheckpoint -v -race`
Expected: PASS (all 3 sub-tests, and the race detector must not flag `TestCheckpoint_ConcurrentMarkDone`).

- [ ] **Step 5: Commit**

```bash
git add agent/batch/checkpoint.go agent/batch/checkpoint_test.go
git commit -m "feat(agent/batch): append-only checkpoint log for resume"
```

---

## Task 5: Runner + worker pool

**Files:**
- Create: `agent/batch/runner.go`
- Create: `agent/batch/runner_test.go`

- [ ] **Step 1: Write the failing test**

Create `agent/batch/runner_test.go`:

```go
package batch

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/provider"
)

// fakeProvider returns a canned response. Used so the runner tests
// don't need a network or real model.
type fakeProvider struct {
	mu    sync.Mutex
	calls int
}

func (f *fakeProvider) Name() string { return "fake" }

func (f *fakeProvider) Available() bool { return true }

func (f *fakeProvider) ModelInfo(string) *provider.ModelInfo {
	return &provider.ModelInfo{
		ContextLength:     100_000,
		MaxOutputTokens:   1024,
		SupportsTools:     true,
		SupportsStreaming: true,
	}
}

func (f *fakeProvider) EstimateTokens(_, s string) (int, error) {
	return len(s) / 4, nil
}

func (f *fakeProvider) Complete(ctx context.Context, req *provider.Request) (*provider.Response, error) {
	f.mu.Lock()
	f.calls++
	f.mu.Unlock()
	return &provider.Response{
		Message: message.Message{
			Role:    message.RoleAssistant,
			Content: message.TextContent("ok: " + req.Messages[len(req.Messages)-1].Content.Text()),
		},
		FinishReason: "end_turn",
		Model:        req.Model,
	}, nil
}

func (f *fakeProvider) Stream(context.Context, *provider.Request) (provider.Stream, error) {
	return nil, errors.New("fake: streaming not implemented")
}

func writeDataset(t *testing.T, dir string, ids ...string) string {
	t.Helper()
	path := filepath.Join(dir, "d.jsonl")
	var buf []byte
	for _, id := range ids {
		buf = append(buf, []byte(`{"id":"`+id+`","prompt":"hello `+id+`"}`+"\n")...)
	}
	_ = os.WriteFile(path, buf, 0o644)
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
		Resume:      true,
	}

	// Pre-mark "a" as done.
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
	// "a" must NOT have a new trajectory written.
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
	}
	r := NewRunner(cfg, &fakeProvider{})
	if err := r.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(cfg.OutputDir, "trajectories", "only.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	// First line is metadata; subsequent lines are {from, value} pairs.
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

func firstLine(data []byte) []byte {
	for i, b := range data {
		if b == '\n' {
			return data[:i]
		}
	}
	return data
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./agent/batch/ -run TestRunner -v`
Expected: FAIL — `NewRunner`, `Runner.Run`, `Config.Resume` undefined.

- [ ] **Step 3: Extend Config with the Resume flag**

Append to `Config` in `agent/batch/config.go`:

```go
	// Resume tells the runner to skip IDs already present in the
	// checkpoint file. The flag is wired from the CLI layer; it has
	// no YAML representation on purpose (it's a per-invocation knob).
	Resume bool `yaml:"-"`
```

- [ ] **Step 4: Implement the runner**

Create `agent/batch/runner.go`:

```go
package batch

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/provider"
)

// Runner drives a batch data-generation run. It is safe for a single
// Run(ctx) invocation; construct a fresh Runner per run.
type Runner struct {
	cfg      *Config
	provider provider.Provider

	done int64 // atomic count of completed items (used by logging)
}

// NewRunner constructs a Runner. The provider must be ready to serve
// requests (factory.New should already have been called by the caller).
func NewRunner(cfg *Config, p provider.Provider) *Runner {
	return &Runner{cfg: cfg, provider: p}
}

// Run executes the batch. It returns the first unrecoverable error
// encountered by any worker; partial progress is preserved in the
// checkpoint + trajectory files.
func (r *Runner) Run(ctx context.Context) error {
	items, err := ReadDataset(r.cfg.DatasetFile, r.cfg.MaxItems)
	if err != nil {
		return err
	}

	outDir := r.cfg.OutputDir
	if err := os.MkdirAll(filepath.Join(outDir, "trajectories"), 0o755); err != nil {
		return fmt.Errorf("batch: mkdir trajectories: %w", err)
	}

	cpPath := filepath.Join(outDir, "checkpoint.jsonl")
	var seen map[string]bool
	if r.cfg.Resume {
		seen, err = LoadCheckpointSet(cpPath)
		if err != nil {
			return err
		}
	} else {
		seen = map[string]bool{}
	}

	cp, err := OpenCheckpoint(cpPath)
	if err != nil {
		return err
	}
	defer cp.Close()

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(r.cfg.NumWorkers)

	startedAt := time.Now()
	for _, it := range items {
		if seen[it.ID] {
			continue
		}
		item := it
		g.Go(func() error {
			if err := r.processOne(gctx, item); err != nil {
				return fmt.Errorf("batch: item %s: %w", item.ID, err)
			}
			if err := cp.MarkDone(item.ID); err != nil {
				return fmt.Errorf("batch: checkpoint %s: %w", item.ID, err)
			}
			atomic.AddInt64(&r.done, 1)
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "batch: completed %d items in %s\n",
		atomic.LoadInt64(&r.done), time.Since(startedAt).Truncate(time.Millisecond))
	return nil
}

// processOne runs a single prompt through the provider and writes a
// trajectory JSONL file. MVP: no tool use, no multi-turn — we send the
// prompt, capture the response, and record the pair. A future plan can
// wire in the full agent.Engine loop when the batch runner grows tool
// support.
func (r *Runner) processOne(ctx context.Context, item Item) error {
	req := &provider.Request{
		Model:        r.cfg.Model,
		SystemPrompt: r.cfg.EphemeralSystemPrompt,
		Messages: []message.Message{
			{Role: message.RoleUser, Content: message.TextContent(item.Prompt)},
		},
		MaxTokens: 4096,
	}
	resp, err := r.provider.Complete(ctx, req)
	if err != nil {
		return err
	}

	path := filepath.Join(r.cfg.OutputDir, "trajectories", sanitizeFilename(item.ID)+".jsonl")
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	bw := bufio.NewWriter(f)
	defer bw.Flush()

	meta := map[string]any{
		"id":          item.ID,
		"model":       r.cfg.Model,
		"environment": r.cfg.Environment,
		"started_at":  time.Now().UTC().Format(time.RFC3339Nano),
	}
	if err := writeJSONLine(bw, meta); err != nil {
		return err
	}
	if err := writeJSONLine(bw, map[string]string{"from": "user", "value": item.Prompt}); err != nil {
		return err
	}
	if err := writeJSONLine(bw, map[string]string{"from": "assistant", "value": resp.Message.Content.Text()}); err != nil {
		return err
	}
	return nil
}

func writeJSONLine(w *bufio.Writer, v any) error {
	line, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if _, err := w.Write(line); err != nil {
		return err
	}
	_, err = w.WriteString("\n")
	return err
}

// sanitizeFilename strips characters that would break a POSIX path.
// The dataset can contain arbitrary IDs so we defend against it.
func sanitizeFilename(id string) string {
	out := make([]byte, 0, len(id))
	for i := 0; i < len(id); i++ {
		c := id[i]
		switch {
		case c == '/' || c == '\\' || c == '\x00':
			out = append(out, '_')
		default:
			out = append(out, c)
		}
	}
	return string(out)
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./agent/batch/ -run TestRunner -v -race`
Expected: PASS (all 3 sub-tests, no races).

- [ ] **Step 6: Commit**

```bash
git add agent/batch/runner.go agent/batch/config.go agent/batch/runner_test.go
git commit -m "feat(agent/batch): goroutine-pool Runner with checkpoint + trajectory output"
```

---

## Task 6: CLI command `hermind batch run`

**Files:**
- Create: `cli/batch.go`
- Create: `cli/batch_test.go`
- Modify: `cli/root.go`

- [ ] **Step 1: Write the failing test**

Create `cli/batch_test.go`:

```go
package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

func TestBatchCmd_RequiresConfigArg(t *testing.T) {
	cmd := newBatchCmd(&App{})
	// find "run" subcommand
	var runCmd *cobra.Command
	for _, c := range cmd.Commands() {
		if c.Use == "run <config.yaml>" {
			runCmd = c
		}
	}
	if runCmd == nil {
		t.Fatal("run subcommand not found")
	}
	runCmd.SetArgs([]string{})
	var out bytes.Buffer
	runCmd.SetOut(&out)
	runCmd.SetErr(&out)
	if err := runCmd.ExecuteContext(context.Background()); err == nil {
		t.Error("expected error when config missing")
	}
}

func TestBatchCmd_LoadsConfig(t *testing.T) {
	dir := t.TempDir()
	dataset := filepath.Join(dir, "d.jsonl")
	_ = os.WriteFile(dataset, []byte(`{"id":"x","prompt":"hi"}`+"\n"), 0o644)

	cfgPath := filepath.Join(dir, "cfg.yaml")
	yaml := []byte(`model: fake/model
dataset_file: ` + dataset + `
output_dir: ` + filepath.Join(dir, "out") + `
`)
	_ = os.WriteFile(cfgPath, yaml, 0o644)

	// Dry-run path: --check parses the config + dataset but does not
	// build a provider. This makes the test trivial to run offline.
	cmd := newBatchCmd(&App{})
	cmd.SetArgs([]string{"run", cfgPath, "--check"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("execute: %v\nout: %s", err, out.String())
	}
	if !bytes.Contains(out.Bytes(), []byte("batch: config OK")) {
		t.Errorf("expected OK line, got %q", out.String())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cli/ -run TestBatchCmd -v`
Expected: FAIL — `newBatchCmd` undefined.

- [ ] **Step 3: Implement the command**

Create `cli/batch.go`:

```go
package cli

import (
	"fmt"

	"github.com/odysseythink/hermind/agent/batch"
	"github.com/odysseythink/hermind/provider/factory"
	"github.com/spf13/cobra"
)

// newBatchCmd creates the "hermind batch" subcommand tree.
func newBatchCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "batch",
		Short: "Run the agent across a dataset in parallel",
	}
	cmd.AddCommand(newBatchRunCmd(app))
	return cmd
}

func newBatchRunCmd(app *App) *cobra.Command {
	var (
		resume bool
		check  bool
	)
	c := &cobra.Command{
		Use:   "run <config.yaml>",
		Short: "Run a batch described by the given YAML config",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := batch.LoadConfig(args[0])
			if err != nil {
				return err
			}
			cfg.Resume = resume
			if check {
				// Parse + open the dataset to surface any trouble without
				// actually calling the provider.
				items, err := batch.ReadDataset(cfg.DatasetFile, cfg.MaxItems)
				if err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(),
					"batch: config OK (model=%s, items=%d, workers=%d, out=%s)\n",
					cfg.Model, len(items), cfg.NumWorkers, cfg.OutputDir)
				return nil
			}

			// Resolve provider from hermind's active config.
			provCfg, err := resolveProviderForModel(app, cfg.Model)
			if err != nil {
				return err
			}
			p, err := factory.New(provCfg)
			if err != nil {
				return err
			}

			runner := batch.NewRunner(cfg, p)
			return runner.Run(cmd.Context())
		},
	}
	c.Flags().BoolVar(&resume, "resume", false, "skip items already present in the checkpoint file")
	c.Flags().BoolVar(&check, "check", false, "validate the config + dataset and exit")
	return c
}

// resolveProviderForModel maps a "<name>/<model>" string (e.g.
// "bedrock/anthropic.claude-opus-4-v1:0") to a config.ProviderConfig
// drawn from the loaded hermind config. The model portion after the
// first "/" overrides the config's default Model.
func resolveProviderForModel(app *App, modelRef string) (providerConfig, error) {
	name, model := splitModelRef(modelRef)
	if app == nil || app.Config == nil {
		return providerConfig{}, fmt.Errorf("batch: no hermind config loaded (provider %q)", name)
	}
	p, ok := app.Config.Providers[name]
	if !ok {
		return providerConfig{}, fmt.Errorf("batch: provider %q not configured in %s", name, app.ConfigPath)
	}
	if model != "" {
		p.Model = model
	}
	return p, nil
}

// providerConfig aliases config.ProviderConfig so callers don't have to
// import config for a one-line return type.
type providerConfig = configProviderConfig

// configProviderConfig is a thin alias that exists only to avoid an
// import cycle with the rest of this file; see the real type in the
// config package. Kept local so refactors don't break the Factory call.
type configProviderConfig = providerCfg

// providerCfg is resolved at package init — we just mirror the real
// type here so the surface compiles. In practice this file imports
// "github.com/odysseythink/hermind/config" directly and uses
// config.ProviderConfig, but we keep the indirection minimal.
type providerCfg struct{}

// splitModelRef splits "provider/model/with/slashes" into
// ("provider", "model/with/slashes"). An empty model portion is
// returned as "" so callers can decide whether to fall back to the
// config default.
func splitModelRef(ref string) (string, string) {
	for i := 0; i < len(ref); i++ {
		if ref[i] == '/' {
			return ref[:i], ref[i+1:]
		}
	}
	return ref, ""
}
```

Then **fix the import shim**: replace the `providerConfig`/`configProviderConfig`/`providerCfg` block with a direct import of `config.ProviderConfig`. The cleaner final shape of `cli/batch.go` uses:

```go
import (
	"fmt"

	"github.com/odysseythink/hermind/agent/batch"
	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/provider/factory"
	"github.com/spf13/cobra"
)
```

and replaces the `resolveProviderForModel` signature + helper types with:

```go
func resolveProviderForModel(app *App, modelRef string) (config.ProviderConfig, error) {
	name, model := splitModelRef(modelRef)
	if app == nil || app.Config == nil {
		return config.ProviderConfig{}, fmt.Errorf("batch: no hermind config loaded (provider %q)", name)
	}
	p, ok := app.Config.Providers[name]
	if !ok {
		return config.ProviderConfig{}, fmt.Errorf("batch: provider %q not configured in %s", name, app.ConfigPath)
	}
	if model != "" {
		p.Model = model
	}
	return p, nil
}
```

Delete the `providerConfig`/`configProviderConfig`/`providerCfg` alias block entirely after the replacement.

- [ ] **Step 4: Register the command in root**

In `cli/root.go`, inside the `AddCommand(...)` call, append `newBatchCmd(app),` on a new line (e.g., after `newCronCmd(app),`):

```go
		newBatchCmd(app),
```

- [ ] **Step 5: Run tests**

Run: `go test ./cli/ -run TestBatchCmd -v`
Expected: PASS (both sub-tests).

- [ ] **Step 6: Run broader suite**

Run: `go test ./...`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add cli/batch.go cli/batch_test.go cli/root.go
git commit -m "feat(cli): add 'hermind batch run' subcommand"
```

---

## Task 7: End-to-end smoke test (manual)

**Files:** none — manual.

- [ ] **Step 1: Build**

Run: `go build -o /tmp/hermind ./cmd/hermind`
Expected: no errors.

- [ ] **Step 2: Prepare a tiny dataset + config**

```bash
WORK=/tmp/hermind-batch
rm -rf "$WORK"
mkdir -p "$WORK"
cat > "$WORK/data.jsonl" <<'EOF'
{"id":"q1","prompt":"what color is the sky?"}
{"id":"q2","prompt":"name a fruit"}
EOF
cat > "$WORK/cfg.yaml" <<EOF
model: anthropic/claude-opus-4-6
dataset_file: $WORK/data.jsonl
output_dir: $WORK/out
num_workers: 2
EOF
```

- [ ] **Step 3: Dry-run with --check**

Run: `/tmp/hermind batch run "$WORK/cfg.yaml" --check`
Expected: `batch: config OK (model=anthropic/claude-opus-4-6, items=2, workers=2, out=/tmp/hermind-batch/out)`

- [ ] **Step 4: Real run** (only if you have an Anthropic API key configured)

Run: `/tmp/hermind batch run "$WORK/cfg.yaml"`
Expected: two files under `$WORK/out/trajectories/` (`q1.jsonl`, `q2.jsonl`) and a 2-line `$WORK/out/checkpoint.jsonl`.

- [ ] **Step 5: Resume path**

Run: `/tmp/hermind batch run "$WORK/cfg.yaml" --resume`
Expected: runner prints `completed 0 items` because both IDs are already in the checkpoint.

- [ ] **Step 6: Cleanup**

```bash
rm -rf "$WORK" /tmp/hermind
```

- [ ] **Step 7: Optional marker commit**

```bash
git commit --allow-empty -m "test(batch): manual smoke test verified resume skips completed items"
```

---

## Self-Review Checklist

1. **Spec coverage:**
   - YAML config matching Python layout ↔ Task 2 ✓
   - Parallel workers ↔ Task 5 (`errgroup` + `SetLimit(NumWorkers)`) ✓
   - Checkpoint / resume ↔ Task 4 + Task 5 ✓
   - Trajectory JSONL output ↔ Task 5 (`processOne`) ✓
   - CLI wiring ↔ Task 6 ✓
   - `--check` dry-run ↔ Task 6 ✓

2. **Placeholders:** one deliberate mid-task cleanup in Task 6 (Step 3 includes a shim block + instructions to delete it). No TBD / TODO content.

3. **Type consistency:**
   - `Config` field names stable: Task 2 defines `NumWorkers`, Task 5 reads `r.cfg.NumWorkers`.
   - `Item{ID, Prompt}` used in Tasks 3, 5, runner tests — consistent.
   - `LoadCheckpointSet(path) (map[string]bool, error)` stable between Task 4 and Task 5.
   - `NewRunner(cfg *Config, p provider.Provider) *Runner` stable between Task 5 and Task 6.
   - `resolveProviderForModel` returns `config.ProviderConfig` after the Step 3 cleanup — matches `factory.New` input.

4. **Gaps:**
   - MVP explicitly single-turn (no tool-use loop). A follow-up plan will wire the full `agent.Engine` when we're ready to replay tool calls.
   - No eval harness (`eval_every`, `eval_size` from Python) — deferred.
   - No trajectory compression — deferred to a separate P1 plan.

---

## Definition of Done

- `go test ./agent/batch/... ./cli/... -race` all pass.
- `go build ./...` succeeds.
- `hermind batch run <cfg> --check` validates and reports item count.
- `hermind batch run <cfg>` writes one trajectory JSONL per prompt + a checkpoint file.
- `hermind batch run <cfg> --resume` skips IDs already in the checkpoint.
