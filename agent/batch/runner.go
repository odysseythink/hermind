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

// Trajectory is the structured result produced for a single dataset
// item. It is the contract between the batch runner and any pluggable
// consumer (e.g. the RL trajectory bridge). Fields are flat by design
// so serialization stays cheap.
type Trajectory struct {
	ID          string            `json:"id"`
	Model       string            `json:"model"`
	Environment string            `json:"environment,omitempty"`
	Prompt      string            `json:"prompt"`
	Response    string            `json:"response"`
	Messages    []message.Message `json:"messages,omitempty"`
	Usage       message.Usage     `json:"usage,omitempty"`
	StartedAt   time.Time         `json:"started_at"`
	FinishedAt  time.Time         `json:"finished_at"`
	// Raw echoes the original dataset line so downstream consumers can
	// recover fields the runner does not understand (e.g. ground-truth
	// answers used by an offline reward model).
	Raw json.RawMessage `json:"raw,omitempty"`
}

// TrajectorySink is the hook point exposed for external consumers such
// as the RL trajectory bridge. Implementations receive every completed
// trajectory in worker-goroutine order; they MUST be safe for
// concurrent use. A nil sink is a valid no-op.
//
// The interface lives in agent/batch/ on purpose — it keeps batch/
// free of any rl/ dependency while giving the rl package a stable
// surface to implement against.
type TrajectorySink interface {
	// OnTrajectory is called once per completed item, after the
	// trajectory JSONL file has been flushed to disk but BEFORE the
	// checkpoint entry is written. Returning a non-nil error aborts
	// the run and prevents the checkpoint entry from being recorded,
	// so the item will be retried on the next --resume.
	OnTrajectory(ctx context.Context, tr *Trajectory) error
}

// TrajectorySinkFunc is an adapter that lets a plain function satisfy
// TrajectorySink. Useful for tests and one-off hooks.
type TrajectorySinkFunc func(ctx context.Context, tr *Trajectory) error

// OnTrajectory implements TrajectorySink.
func (f TrajectorySinkFunc) OnTrajectory(ctx context.Context, tr *Trajectory) error {
	return f(ctx, tr)
}

// Runner drives a batch data-generation run. It is safe for a single
// Run(ctx) invocation; construct a fresh Runner per run.
type Runner struct {
	cfg      *Config
	provider provider.Provider
	sink     TrajectorySink

	done int64 // atomic count of completed items (used by logging)
}

// NewRunner constructs a Runner. The provider must be ready to serve
// requests (factory.New should already have been called by the caller).
func NewRunner(cfg *Config, p provider.Provider) *Runner {
	return &Runner{cfg: cfg, provider: p}
}

// WithSink attaches a TrajectorySink. Pass nil to clear. Returns the
// runner for chaining.
func (r *Runner) WithSink(s TrajectorySink) *Runner {
	r.sink = s
	return r
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
			tr, err := r.processOne(gctx, item)
			if err != nil {
				return fmt.Errorf("batch: item %s: %w", item.ID, err)
			}
			if r.sink != nil {
				if err := r.sink.OnTrajectory(gctx, tr); err != nil {
					return fmt.Errorf("batch: sink %s: %w", item.ID, err)
				}
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
// trajectory JSONL file. MVP: no tool use, no multi-turn — we send
// the prompt, capture the response, and record the pair. A future
// plan can wire in the full agent.Engine loop when the batch runner
// grows tool support (see Config.MaxTurns, currently reserved).
func (r *Runner) processOne(ctx context.Context, item Item) (*Trajectory, error) {
	req := &provider.Request{
		Model:        r.cfg.Model,
		SystemPrompt: r.cfg.EphemeralSystemPrompt,
		Messages: []message.Message{
			{Role: message.RoleUser, Content: message.TextContent(item.Prompt)},
		},
		MaxTokens: r.cfg.MaxTokens,
	}
	startedAt := time.Now().UTC()
	resp, err := r.provider.Complete(ctx, req)
	if err != nil {
		return nil, err
	}
	finishedAt := time.Now().UTC()

	tr := &Trajectory{
		ID:          item.ID,
		Model:       r.cfg.Model,
		Environment: r.cfg.Environment,
		Prompt:      item.Prompt,
		Response:    resp.Message.Content.Text(),
		Messages: []message.Message{
			{Role: message.RoleUser, Content: message.TextContent(item.Prompt)},
			resp.Message,
		},
		Usage:      resp.Usage,
		StartedAt:  startedAt,
		FinishedAt: finishedAt,
		Raw:        item.Raw,
	}

	path := filepath.Join(r.cfg.OutputDir, "trajectories", sanitizeFilename(item.ID)+".jsonl")
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	bw := bufio.NewWriter(f)

	meta := map[string]any{
		"id":          tr.ID,
		"model":       tr.Model,
		"environment": tr.Environment,
		"started_at":  tr.StartedAt.Format(time.RFC3339Nano),
		"finished_at": tr.FinishedAt.Format(time.RFC3339Nano),
	}
	if err := writeJSONLine(bw, meta); err != nil {
		return nil, err
	}
	if err := writeJSONLine(bw, map[string]string{"from": "user", "value": tr.Prompt}); err != nil {
		return nil, err
	}
	if err := writeJSONLine(bw, map[string]string{"from": "assistant", "value": tr.Response}); err != nil {
		return nil, err
	}
	if err := bw.Flush(); err != nil {
		return nil, err
	}
	return tr, nil
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
	if id == "" {
		return "_"
	}
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
