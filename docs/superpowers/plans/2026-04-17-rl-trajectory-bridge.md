# RL Trajectory Bridge Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stop trying to reimplement Tinker/Atropos in Go. Instead, make hermind a **trajectory producer** that writes JSONL episodes in the exact shape the Python trainer expects, with an optional gRPC sink for online data plumbing. Python (Tinker + Atropos) remains the trainer; hermind stays in its lane — running the agent, collecting the trajectory, and delivering it.

**Architecture:** A new `rl/trajectory/` package owns the episode data model and serialization. A `Sink` interface abstracts delivery; the MVP ships `FileSink` (append-only JSONL to a local path or S3 via minio-go in a later plan) and stubs the optional `GRPCSink`. A new `rl/collector/` package has a `Collector` that wraps an `agent.Engine`, intercepts each turn's messages + tool calls, and assembles an `Episode`. The existing `hermind batch run` (from Plan C) becomes the primary integration point: pass `--rl-sink file:/path/out.jsonl` and every prompt's conversation is also emitted as a trajectory.

**Tech Stack:** Go 1.21+, existing `agent`, `message`, `tool`, `config`, `provider` packages. `encoding/json` for JSONL. gRPC sink is OPTIONAL and gated behind a `grpc` build tag so default builds don't need the SDK.

---

## File Structure

- Create: `rl/trajectory/trajectory.go` — `Episode`, `Step`, `TrajectoryMeta` types
- Create: `rl/trajectory/trajectory_test.go`
- Create: `rl/trajectory/sink.go` — `Sink` interface
- Create: `rl/trajectory/file_sink.go` — `FileSink` JSONL writer
- Create: `rl/trajectory/file_sink_test.go`
- Create: `rl/collector/collector.go` — `Collector` that turns agent runs into `Episode` values
- Create: `rl/collector/collector_test.go`
- Modify: `config/config.go` — add `RLConfig` block (sink path, optional gRPC target)
- Modify: `config/loader_test.go`
- Modify: `cli/batch.go` (from Plan C) — add `--rl-sink` flag that opens a Sink and calls `Collector.Observe` after each prompt
- Modify: `cli/batch_test.go`

---

## Task 1: Episode data model

**Files:**
- Create: `rl/trajectory/trajectory.go`
- Create: `rl/trajectory/trajectory_test.go`

- [ ] **Step 1: Write the failing test**

Create `rl/trajectory/trajectory_test.go`:

```go
package trajectory

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestEpisode_JSONShapeMatchesTinker(t *testing.T) {
	ep := Episode{
		EpisodeID: "ep-1",
		Meta: Meta{
			Environment: "web-research",
			ConfigID:    "run-v1",
			Model:       "anthropic/claude-opus-4-6",
			StartedAt:   1700000000,
		},
		Steps: []Step{
			{From: "user", Value: "what is 2+2?"},
			{From: "assistant", Value: "4"},
		},
		EpisodeReward: 1.0,
	}
	data, _ := json.Marshal(ep)
	// Tinker expects "from"/"value" pairs, "episode_id", "steps",
	// "episode_reward", and a "meta" block.
	for _, want := range []string{
		`"episode_id":"ep-1"`,
		`"from":"user"`,
		`"value":"what is 2+2?"`,
		`"episode_reward":1`,
		`"environment":"web-research"`,
	} {
		if !strings.Contains(string(data), want) {
			t.Errorf("missing %s in %s", want, data)
		}
	}
}

func TestEpisode_EmptyStepsAllowed(t *testing.T) {
	ep := Episode{EpisodeID: "empty"}
	data, err := json.Marshal(ep)
	if err != nil {
		t.Fatal(err)
	}
	// Empty steps must render as []`, not null, so the trainer's
	// iteration does not choke.
	if !strings.Contains(string(data), `"steps":[]`) {
		t.Errorf("steps not empty array: %s", data)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./rl/trajectory/ -v`
Expected: FAIL — package undefined.

- [ ] **Step 3: Implement the types**

Create `rl/trajectory/trajectory.go`:

```go
// Package trajectory defines the data model hermind emits for RL
// training. The JSON shape matches what Tinker + Atropos consume,
// so Python trainers can read hermind episodes without a translator.
package trajectory

// Episode is one complete agent interaction.
type Episode struct {
	EpisodeID     string  `json:"episode_id"`
	Meta          Meta    `json:"meta"`
	Steps         []Step  `json:"steps"`
	EpisodeReward float64 `json:"episode_reward"`
}

// Meta holds episode-wide metadata.
type Meta struct {
	Environment string `json:"environment"`
	ConfigID    string `json:"config_id,omitempty"`
	Model       string `json:"model"`
	StartedAt   int64  `json:"started_at"`          // unix seconds
	EndedAt     int64  `json:"ended_at,omitempty"`  // unix seconds
	Extra       map[string]interface{} `json:"extra,omitempty"`
}

// Step is a single message turn inside an episode.
// The from/value shape is the Tinker-native format.
type Step struct {
	From       string  `json:"from"`              // "user" | "assistant" | "tool" | "system"
	Value      string  `json:"value"`             // free-form text
	ToolName   string  `json:"tool_name,omitempty"`
	ToolCallID string  `json:"tool_call_id,omitempty"`
	Reward     float64 `json:"reward,omitempty"`
	Tokens     int     `json:"tokens,omitempty"`
}

// MarshalJSON ensures a nil Steps slice renders as [] rather than null.
func (e Episode) MarshalJSON() ([]byte, error) {
	type alias Episode
	if e.Steps == nil {
		e.Steps = []Step{}
	}
	// Defer to the default encoder via a type alias to avoid recursion.
	return marshalEpisode(alias(e))
}
```

Create a companion file `rl/trajectory/json.go`:

```go
package trajectory

import "encoding/json"

type alias Episode

func marshalEpisode(a alias) ([]byte, error) {
	return json.Marshal(a)
}
```

(The two-file split keeps the `MarshalJSON` trick tidy.)

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./rl/trajectory/ -run TestEpisode -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add rl/trajectory/trajectory.go rl/trajectory/json.go rl/trajectory/trajectory_test.go
git commit -m "feat(rl/trajectory): Episode/Step/Meta types matching Tinker JSONL shape"
```

---

## Task 2: Sink interface + FileSink

**Files:**
- Create: `rl/trajectory/sink.go`
- Create: `rl/trajectory/file_sink.go`
- Create: `rl/trajectory/file_sink_test.go`

- [ ] **Step 1: Write the failing test**

Create `rl/trajectory/file_sink_test.go`:

```go
package trajectory

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestFileSink_AppendJSONL(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "episodes.jsonl")
	sink, err := NewFileSink(path)
	if err != nil {
		t.Fatal(err)
	}
	defer sink.Close()

	_ = sink.Write(context.Background(), Episode{EpisodeID: "a", Steps: []Step{{From: "user", Value: "hi"}}})
	_ = sink.Write(context.Background(), Episode{EpisodeID: "b", Steps: []Step{{From: "user", Value: "hi2"}}})

	data, _ := os.ReadFile(path)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	var ep Episode
	if err := json.Unmarshal([]byte(lines[0]), &ep); err != nil {
		t.Fatal(err)
	}
	if ep.EpisodeID != "a" {
		t.Errorf("first id = %q", ep.EpisodeID)
	}
}

func TestFileSink_ConcurrentWrites(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "episodes.jsonl")
	sink, err := NewFileSink(path)
	if err != nil {
		t.Fatal(err)
	}
	defer sink.Close()

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			_ = sink.Write(context.Background(), Episode{EpisodeID: string(rune('a' + (n % 26)))})
		}(i)
	}
	wg.Wait()

	// Re-read: every line must parse as a full episode (if writes
	// interleaved, some would be garbled).
	f, _ := os.Open(path)
	defer f.Close()
	scan := bufio.NewScanner(f)
	for scan.Scan() {
		var ep Episode
		if err := json.Unmarshal(scan.Bytes(), &ep); err != nil {
			t.Errorf("corrupt line: %s", scan.Text())
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./rl/trajectory/ -run TestFileSink -v`
Expected: FAIL — `NewFileSink` undefined.

- [ ] **Step 3: Implement Sink + FileSink**

Create `rl/trajectory/sink.go`:

```go
package trajectory

import "context"

// Sink accepts completed episodes. Implementations are safe for
// concurrent use; callers never need their own lock.
type Sink interface {
	// Write records one episode. Returns an error on transport
	// failure — callers typically log and continue.
	Write(ctx context.Context, ep Episode) error

	// Close flushes buffers and releases resources. Must be idempotent.
	Close() error
}
```

Create `rl/trajectory/file_sink.go`:

```go
package trajectory

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// FileSink appends episodes as JSONL to a local file. Writes are
// mutex-serialized and each line is fsync'd so crash-at-any-moment
// doesn't leave a torn episode on disk.
type FileSink struct {
	mu sync.Mutex
	f  *os.File
	bw *bufio.Writer
}

// NewFileSink opens path for append, creating it (and any parent
// directories) if necessary.
func NewFileSink(path string) (*FileSink, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("trajectory: mkdir: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("trajectory: open sink: %w", err)
	}
	return &FileSink{f: f, bw: bufio.NewWriter(f)}, nil
}

// Write serializes ep as a single JSON line and flushes+fsyncs it.
func (s *FileSink) Write(_ context.Context, ep Episode) error {
	data, err := json.Marshal(ep)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.bw.Write(data); err != nil {
		return err
	}
	if err := s.bw.WriteByte('\n'); err != nil {
		return err
	}
	if err := s.bw.Flush(); err != nil {
		return err
	}
	return s.f.Sync()
}

// Close flushes and closes the underlying file.
func (s *FileSink) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.bw != nil {
		_ = s.bw.Flush()
		s.bw = nil
	}
	if s.f != nil {
		err := s.f.Close()
		s.f = nil
		return err
	}
	return nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./rl/trajectory/ -run TestFileSink -v -race`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add rl/trajectory/sink.go rl/trajectory/file_sink.go rl/trajectory/file_sink_test.go
git commit -m "feat(rl/trajectory): Sink interface + FileSink JSONL writer"
```

---

## Task 3: Collector

**Files:**
- Create: `rl/collector/collector.go`
- Create: `rl/collector/collector_test.go`

- [ ] **Step 1: Write the failing test**

Create `rl/collector/collector_test.go`:

```go
package collector

import (
	"context"
	"testing"

	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/rl/trajectory"
)

type recordingSink struct {
	episodes []trajectory.Episode
}

func (r *recordingSink) Write(_ context.Context, ep trajectory.Episode) error {
	r.episodes = append(r.episodes, ep)
	return nil
}
func (r *recordingSink) Close() error { return nil }

func TestCollector_ObserveBuildsEpisode(t *testing.T) {
	sink := &recordingSink{}
	c := NewCollector(sink, trajectory.Meta{
		Environment: "test-env",
		Model:       "stub/model",
	})

	// Start a run and feed it messages.
	run := c.StartRun("my-prompt")
	run.Append(message.Message{Role: message.RoleUser, Content: message.TextContent("hi")})
	run.Append(message.Message{Role: message.RoleAssistant, Content: message.TextContent("hello")})
	run.SetReward(1.0)
	if err := run.End(context.Background()); err != nil {
		t.Fatal(err)
	}

	if len(sink.episodes) != 1 {
		t.Fatalf("expected 1 episode, got %d", len(sink.episodes))
	}
	ep := sink.episodes[0]
	if ep.EpisodeID == "" {
		t.Error("missing episode id")
	}
	if len(ep.Steps) != 2 {
		t.Errorf("steps = %d", len(ep.Steps))
	}
	if ep.EpisodeReward != 1.0 {
		t.Errorf("reward = %v", ep.EpisodeReward)
	}
	if ep.Meta.Environment != "test-env" {
		t.Errorf("env = %q", ep.Meta.Environment)
	}
}

func TestCollector_Observe_RolesMappedToFromField(t *testing.T) {
	sink := &recordingSink{}
	c := NewCollector(sink, trajectory.Meta{Model: "m"})

	run := c.StartRun("prompt")
	run.Append(message.Message{Role: message.RoleUser, Content: message.TextContent("u")})
	run.Append(message.Message{Role: message.RoleAssistant, Content: message.TextContent("a")})
	run.Append(message.Message{Role: message.RoleTool, Content: message.TextContent("t")})
	_ = run.End(context.Background())

	want := []string{"user", "assistant", "tool"}
	for i, s := range sink.episodes[0].Steps {
		if s.From != want[i] {
			t.Errorf("steps[%d].from = %q, want %q", i, s.From, want[i])
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./rl/collector/ -v`
Expected: FAIL.

- [ ] **Step 3: Implement the collector**

Create `rl/collector/collector.go`:

```go
// Package collector observes an agent run and produces an RL episode
// that downstream Python trainers can consume.
package collector

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/rl/trajectory"
)

// Collector produces one Episode per agent run. It is safe for
// concurrent use — each run gets its own Run handle.
type Collector struct {
	sink trajectory.Sink
	meta trajectory.Meta
}

// NewCollector constructs a collector that writes to sink and stamps
// every episode with the given meta template.
func NewCollector(sink trajectory.Sink, meta trajectory.Meta) *Collector {
	return &Collector{sink: sink, meta: meta}
}

// Run represents a single in-flight episode.
type Run struct {
	parent *Collector
	ep     trajectory.Episode
}

// StartRun opens a new episode. The initial user prompt is optional —
// pass "" if the agent is being driven by a non-text environment.
func (c *Collector) StartRun(prompt string) *Run {
	id := "ep-" + randID()
	meta := c.meta // copy
	meta.StartedAt = time.Now().Unix()

	ep := trajectory.Episode{
		EpisodeID: id,
		Meta:      meta,
	}
	r := &Run{parent: c, ep: ep}
	if prompt != "" {
		r.ep.Steps = append(r.ep.Steps, trajectory.Step{
			From:  "user",
			Value: prompt,
		})
	}
	return r
}

// Append records one message turn into the in-flight episode.
func (r *Run) Append(m message.Message) {
	r.ep.Steps = append(r.ep.Steps, trajectory.Step{
		From:  roleToFrom(m.Role),
		Value: m.Content.Text(),
	})
}

// SetReward overrides the episode reward (default 0).
func (r *Run) SetReward(v float64) { r.ep.EpisodeReward = v }

// End flushes the episode to the sink. It stamps EndedAt, then writes
// the episode through the parent collector's Sink.
func (r *Run) End(ctx context.Context) error {
	r.ep.Meta.EndedAt = time.Now().Unix()
	return r.parent.sink.Write(ctx, r.ep)
}

func roleToFrom(role message.Role) string {
	switch role {
	case message.RoleUser:
		return "user"
	case message.RoleAssistant:
		return "assistant"
	case message.RoleTool:
		return "tool"
	case message.RoleSystem:
		return "system"
	}
	return string(role)
}

func randID() string {
	var buf [6]byte
	_, _ = rand.Read(buf[:])
	return hex.EncodeToString(buf[:])
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./rl/collector/ -v -race`
Expected: PASS (both sub-tests).

- [ ] **Step 5: Commit**

```bash
git add rl/collector/collector.go rl/collector/collector_test.go
git commit -m "feat(rl/collector): observe agent runs and emit Episode records"
```

---

## Task 4: Config block

**Files:**
- Modify: `config/config.go`
- Modify: `config/loader_test.go`

- [ ] **Step 1: Write the failing test**

Append to `config/loader_test.go`:

```go
func TestLoadFromPath_RLSink(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	_ = os.WriteFile(path, []byte(`
model: test
rl:
  sink:
    kind: file
    path: /tmp/episodes.jsonl
  meta:
    environment: web-research
    config_id: run-v1
`), 0o644)
	cfg, err := LoadFromPath(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.RL.Sink.Kind != "file" || cfg.RL.Sink.Path != "/tmp/episodes.jsonl" {
		t.Errorf("sink = %+v", cfg.RL.Sink)
	}
	if cfg.RL.Meta.Environment != "web-research" {
		t.Errorf("env = %q", cfg.RL.Meta.Environment)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./config/ -run TestLoadFromPath_RLSink -v`
Expected: FAIL — `cfg.RL` undefined.

- [ ] **Step 3: Add the config types**

In `config/config.go`, append the `RLConfig` block (don't collide with the existing `rl/` package which only holds a manager):

```go
// RLConfig wires hermind's trajectory producer. Empty → no sink, no
// recording. Populate sink.path (and optionally sink.kind) to write
// Tinker-compatible JSONL alongside your batch runs.
type RLConfig struct {
	Sink RLSink `yaml:"sink,omitempty"`
	Meta RLMeta `yaml:"meta,omitempty"`
}

// RLSink points at a sink destination.
type RLSink struct {
	Kind string `yaml:"kind,omitempty"` // "file" (default) or "grpc"
	Path string `yaml:"path,omitempty"` // file path for kind=file
	Addr string `yaml:"addr,omitempty"` // gRPC target for kind=grpc
}

// RLMeta is stamped onto every episode emitted while this config is
// active. Callers can override per-run.
type RLMeta struct {
	Environment string `yaml:"environment,omitempty"`
	ConfigID    string `yaml:"config_id,omitempty"`
}
```

Add the field to `Config`:

```go
type Config struct {
	// ... existing fields ...
	RL                RLConfig                  `yaml:"rl,omitempty"`
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./config/ -run TestLoadFromPath_RLSink -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add config/config.go config/loader_test.go
git commit -m "feat(config): add RL sink + episode metadata block"
```

---

## Task 5: Wire the collector into `hermind batch run`

**Files:**
- Modify: `cli/batch.go` (from Plan C)
- Modify: `cli/batch_test.go`

- [ ] **Step 1: Write the failing test**

Append to `cli/batch_test.go`:

```go
func TestBatchCmd_WritesRLTrajectories(t *testing.T) {
	dir := t.TempDir()
	dataset := filepath.Join(dir, "d.jsonl")
	_ = os.WriteFile(dataset, []byte(`{"id":"a","prompt":"hi"}`+"\n"), 0o644)

	cfgPath := filepath.Join(dir, "cfg.yaml")
	_ = os.WriteFile(cfgPath, []byte(`model: fake/model
dataset_file: `+dataset+`
output_dir: `+filepath.Join(dir, "out")+`
`), 0o644)

	sinkPath := filepath.Join(dir, "episodes.jsonl")

	cmd := newBatchCmd(&App{
		Config:     &config.Config{Providers: map[string]config.ProviderConfig{"fake": {Provider: "anthropic", APIKey: "x", Model: "fake"}}},
		ConfigPath: cfgPath,
	})
	cmd.SetArgs([]string{"run", cfgPath, "--check", "--rl-sink", "file:" + sinkPath})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("execute: %v\n%s", err, out.String())
	}
	// With --check we don't actually run the provider; we just verify
	// the flag is parsed and the sink file is touchable.
	if !bytes.Contains(out.Bytes(), []byte("rl sink: file:"+sinkPath)) {
		t.Errorf("expected rl sink line in output:\n%s", out.String())
	}
	if _, err := os.Stat(sinkPath); err != nil {
		t.Errorf("sink file not created on --check: %v", err)
	}
}
```

Imports: `"bytes"`, `"github.com/odysseythink/hermind/config"`, `"path/filepath"`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cli/ -run TestBatchCmd_WritesRLTrajectories -v`
Expected: FAIL — `--rl-sink` flag doesn't exist.

- [ ] **Step 3: Wire the flag into the batch command**

Extend `cli/batch.go`'s `newBatchRunCmd`:

```go
func newBatchRunCmd(app *App) *cobra.Command {
	var (
		resume bool
		check  bool
		rlSink string
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

			// Open an RL sink if requested. "file:/path" opens a JSONL
			// FileSink; other schemes are reserved for future work.
			var sink trajectory.Sink
			if rlSink != "" {
				sink, err = openRLSink(rlSink)
				if err != nil {
					return err
				}
				defer sink.Close()
				fmt.Fprintf(cmd.OutOrStdout(), "rl sink: %s\n", rlSink)
			}

			if check {
				items, err := batch.ReadDataset(cfg.DatasetFile, cfg.MaxItems)
				if err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(),
					"batch: config OK (model=%s, items=%d, workers=%d, out=%s)\n",
					cfg.Model, len(items), cfg.NumWorkers, cfg.OutputDir)
				return nil
			}

			provCfg, err := resolveProviderForModel(app, cfg.Model)
			if err != nil {
				return err
			}
			p, err := factory.New(provCfg)
			if err != nil {
				return err
			}

			runner := batch.NewRunner(cfg, p)
			if sink != nil {
				coll := collector.NewCollector(sink, trajectory.Meta{
					Environment: cfg.Environment,
					Model:       cfg.Model,
				})
				runner.AttachObserver(func(itemID, prompt string, history []message.Message) {
					run := coll.StartRun(prompt)
					for _, m := range history {
						run.Append(m)
					}
					_ = run.End(cmd.Context())
				})
			}
			return runner.Run(cmd.Context())
		},
	}
	c.Flags().BoolVar(&resume, "resume", false, "skip items already present in the checkpoint file")
	c.Flags().BoolVar(&check, "check", false, "validate the config + dataset and exit")
	c.Flags().StringVar(&rlSink, "rl-sink", "", `record RL episodes (e.g. "file:/path/episodes.jsonl")`)
	return c
}

// openRLSink parses a "kind:value" spec and returns the sink.
func openRLSink(spec string) (trajectory.Sink, error) {
	i := strings.IndexByte(spec, ':')
	if i <= 0 || i == len(spec)-1 {
		return nil, fmt.Errorf("cli: rl sink spec %q must be kind:value", spec)
	}
	kind, value := spec[:i], spec[i+1:]
	switch kind {
	case "file":
		return trajectory.NewFileSink(value)
	}
	return nil, fmt.Errorf("cli: unknown rl sink kind %q", kind)
}
```

Needed imports: `"strings"`, `"github.com/odysseythink/hermind/message"`, `"github.com/odysseythink/hermind/rl/collector"`, `"github.com/odysseythink/hermind/rl/trajectory"`.

- [ ] **Step 4: Add AttachObserver to batch.Runner**

In `agent/batch/runner.go`, extend `Runner`:

```go
type Runner struct {
	cfg      *Config
	provider provider.Provider
	observer func(itemID, prompt string, history []message.Message)

	done int64
}

// AttachObserver registers a callback fired after each item completes.
// The history arg is the list of messages exchanged for this item
// (user prompt + assistant reply in the MVP).
func (r *Runner) AttachObserver(cb func(itemID, prompt string, history []message.Message)) {
	r.observer = cb
}
```

Inside `processOne`, after the response has been persisted, call the observer:

```go
	if r.observer != nil {
		r.observer(item.ID, item.Prompt, []message.Message{
			{Role: message.RoleUser, Content: message.TextContent(item.Prompt)},
			{Role: message.RoleAssistant, Content: message.TextContent(resp.Message.Content.Text())},
		})
	}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./cli/ -run TestBatchCmd_WritesRLTrajectories -v`
Expected: PASS.

Run: `go test ./...`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add cli/batch.go cli/batch_test.go agent/batch/runner.go
git commit -m "feat(cli/batch): --rl-sink flag writes Tinker-compatible episodes"
```

---

## Task 6: Manual smoke test

- [ ] **Step 1: Build + mini dataset**

```bash
go build -o /tmp/hermind ./cmd/hermind
WORK=/tmp/hermind-rl
rm -rf "$WORK" && mkdir -p "$WORK"
cat > "$WORK/data.jsonl" <<'EOF'
{"id":"q1","prompt":"what is 2+2?"}
EOF
cat > "$WORK/cfg.yaml" <<EOF
model: anthropic/claude-opus-4-6
dataset_file: $WORK/data.jsonl
output_dir: $WORK/out
num_workers: 1
EOF
```

- [ ] **Step 2: Run with --rl-sink**

```bash
/tmp/hermind batch run "$WORK/cfg.yaml" --rl-sink file:"$WORK/episodes.jsonl"
cat "$WORK/episodes.jsonl"
```

Expected: one JSONL line per prompt, parseable as an `Episode` with Steps `[user, assistant]`.

- [ ] **Step 3: Feed into a Python trainer** (optional)

```bash
python - <<'PY'
import json, pathlib
for line in pathlib.Path("/tmp/hermind-rl/episodes.jsonl").read_text().splitlines():
    ep = json.loads(line)
    assert ep["steps"][0]["from"] == "user"
    print(ep["episode_id"], ep["meta"]["environment"])
PY
```

Expected: the Python script succeeds — confirming schema compatibility.

- [ ] **Step 4: Cleanup**

```bash
rm -rf /tmp/hermind /tmp/hermind-rl
```

---

## Self-Review Checklist

1. **Spec coverage:**
   - Episode/Step/Meta JSON shape matches Tinker ↔ Task 1 ✓
   - Pluggable Sink interface ↔ Task 2 ✓
   - FileSink JSONL with fsync ↔ Task 2 ✓
   - Collector observes agent runs ↔ Task 3 ✓
   - Config block ↔ Task 4 ✓
   - Integration with batch command ↔ Task 5 ✓
   - Observer hook on Runner ↔ Task 5 ✓

2. **Placeholders:** The gRPC sink is intentionally out of scope; the Sink interface leaves room for a `grpc_sink.go` in a follow-up without API change.

3. **Type consistency:**
   - `Episode{EpisodeID, Meta, Steps, EpisodeReward}` stable across Tasks 1, 2, 3, 5.
   - `Sink.Write(ctx, ep) error` / `Close() error` signature stable.
   - `Collector.StartRun(prompt) *Run` / `Run.Append` / `Run.SetReward` / `Run.End(ctx)` stable between Task 3 and Task 5.
   - Runner `AttachObserver(cb)` signature matches the Task 5 integration.

4. **Gaps (future work):**
   - gRPC sink for online training data pipelines.
   - Per-step reward attribution (current MVP only records episode-level reward).
   - Tool-call-aware Step enrichment (hook into `agent.Engine` callbacks so tool invocations become separate `tool` steps rather than being folded into assistant text).

---

## Definition of Done

- `go test ./rl/... ./cli/... ./agent/batch/... -race` all pass.
- `hermind batch run cfg.yaml --rl-sink file:out.jsonl` produces a Tinker-compatible file.
- Python deserializer parses every line without error.
- `FileSink.Close()` is idempotent and safe to call twice.
