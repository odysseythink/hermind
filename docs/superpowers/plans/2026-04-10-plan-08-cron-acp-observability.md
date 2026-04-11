# Plan 8: Cron, ACP, and Observability Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add three operational features: (1) a cron scheduler that runs prompts on a recurring schedule via the Engine, (2) an ACP (Agent Communication Protocol) gateway platform exposing a JSON HTTP interface suitable for programmatic integrations, and (3) structured `slog` JSON logging threaded through the gateway and cron.

**Architecture:**
- **Cron**: a `cron.Scheduler` type owns a slice of `Job` structs, each with a parsed interval schedule. On `Start` it fires a goroutine per job that ticks on the schedule and invokes a caller-provided `JobRunner`. Schedule strings use a small grammar: `"every 5m"`, `"every 1h30m"`, `"every 6h"`, `"every 1d"`. A `hermes cron` subcommand loads jobs from config and runs them against the primary provider.
- **ACP**: a new `gateway/platforms/acp.go` adapter exposing a POST `/acp/messages` endpoint. Shape differs from api_server (`{"session_id":"...", "parts":[{"type":"text","text":"..."}]}`) to match the ACP schema. ACP replies use the same shape.
- **Observability**: a new `logging` package wraps `log/slog` with a JSON handler by default. The gateway replaces `log.Printf` calls with `slog`. A request ID is attached to each platform dispatch via `context.Context`.

**Tech Stack:** Go 1.25 stdlib `log/slog`, `context`, `time`. No new deps.

**Deliverable at end of plan:**
```yaml
cron:
  jobs:
    - name: daily_summary
      schedule: every 24h
      prompt: Summarize yesterday's commits in the hermes-agent-go repo.
```
```
$ hermes cron
{"time":"2026-04-11T08:00:00Z","level":"INFO","msg":"cron: scheduler started","jobs":1}
{"time":"2026-04-12T08:00:00Z","level":"INFO","msg":"cron: running job","job":"daily_summary"}
```

**Non-goals for this plan (deferred):**
- Full crontab-style (`* * * * *`) syntax — interval-only in Plan 8
- Metrics export (Prometheus, OpenTelemetry) — Plan 8b
- Distributed tracing — later
- ACP authentication — token header added but no OAuth flow
- Skipping missed runs after downtime — simple catch-up only

**Plans 1-7 dependencies this plan touches:**
- `config/config.go` — add `CronConfig`, `CronJobConfig`
- `cron/` — NEW package
- `logging/` — NEW package
- `gateway/platforms/acp.go` — NEW
- `gateway/gateway.go` — MODIFIED: replace log.Printf with slog
- `cli/root.go` — add `cron` subcommand
- `cli/cron.go` — NEW

---

## File Structure

```
hermes-agent-go/
├── config/config.go               # MODIFIED: CronConfig, JSONLogLevel
├── logging/
│   ├── logging.go                 # Setup, WithRequestID, ContextLogger
│   └── logging_test.go
├── cron/
│   ├── schedule.go                # ParseSchedule, Schedule type
│   ├── schedule_test.go
│   ├── scheduler.go               # Scheduler, Job, Run
│   └── scheduler_test.go
├── gateway/
│   ├── gateway.go                 # MODIFIED: use slog
│   └── platforms/
│       ├── acp.go                 # ACP HTTP adapter
│       └── acp_test.go
├── cli/
│   ├── root.go                    # MODIFIED: add cron subcommand
│   └── cron.go                    # newCronCmd
```

---

## Task 1: logging package

- [ ] **Step 1:** Create `logging/logging.go`:

```go
// Package logging wraps log/slog with a JSON handler and a small
// per-request context helper. All gateway, cron, and platform code
// should use slog.InfoContext/ErrorContext with a context that has
// been enriched via WithRequestID so request IDs appear in logs.
package logging

import (
	"context"
	"log/slog"
	"os"

	"github.com/google/uuid"
)

type ctxKey int

const requestIDKey ctxKey = 1

// Setup installs a JSON slog handler as the default logger.
// level is one of "debug", "info", "warn", "error" (default info).
func Setup(level string) {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	h := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: lvl})
	slog.SetDefault(slog.New(&contextHandler{inner: h}))
}

// WithRequestID attaches a request ID to ctx. If id is empty a new
// UUID is generated.
func WithRequestID(ctx context.Context, id string) context.Context {
	if id == "" {
		id = uuid.NewString()
	}
	return context.WithValue(ctx, requestIDKey, id)
}

// RequestID returns the request ID stored in ctx, or "" if none.
func RequestID(ctx context.Context) string {
	if v, ok := ctx.Value(requestIDKey).(string); ok {
		return v
	}
	return ""
}

// contextHandler is a slog.Handler that injects the request ID from
// the context into every log record.
type contextHandler struct {
	inner slog.Handler
}

func (c *contextHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return c.inner.Enabled(ctx, level)
}

func (c *contextHandler) Handle(ctx context.Context, r slog.Record) error {
	if id := RequestID(ctx); id != "" {
		r.AddAttrs(slog.String("request_id", id))
	}
	return c.inner.Handle(ctx, r)
}

func (c *contextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &contextHandler{inner: c.inner.WithAttrs(attrs)}
}

func (c *contextHandler) WithGroup(name string) slog.Handler {
	return &contextHandler{inner: c.inner.WithGroup(name)}
}
```

- [ ] **Step 2:** Create `logging/logging_test.go`:

```go
package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
)

func TestWithRequestIDRoundTrip(t *testing.T) {
	ctx := WithRequestID(context.Background(), "req-1")
	if RequestID(ctx) != "req-1" {
		t.Errorf("unexpected request id: %q", RequestID(ctx))
	}
}

func TestWithRequestIDGenerates(t *testing.T) {
	ctx := WithRequestID(context.Background(), "")
	if RequestID(ctx) == "" {
		t.Error("expected generated id")
	}
}

func TestContextHandlerInjectsRequestID(t *testing.T) {
	buf := &bytes.Buffer{}
	base := slog.NewJSONHandler(buf, nil)
	h := &contextHandler{inner: base}
	logger := slog.New(h)
	ctx := WithRequestID(context.Background(), "req-42")
	logger.InfoContext(ctx, "hello")
	if !strings.Contains(buf.String(), "req-42") {
		t.Errorf("expected request_id in log, got %s", buf.String())
	}
	// Ensure it's valid JSON
	var rec map[string]any
	if err := json.Unmarshal(buf.Bytes(), &rec); err != nil {
		t.Errorf("invalid json: %v", err)
	}
	if rec["request_id"] != "req-42" {
		t.Errorf("request_id = %v", rec["request_id"])
	}
}
```

- [ ] **Step 3:** `go test ./logging/...` — PASS.
- [ ] **Step 4:** Commit `feat(logging): add slog JSON handler with request-id injection`.

---

## Task 2: cron.Schedule parser

- [ ] **Step 1:** Create `cron/schedule.go`:

```go
// Package cron provides a tiny interval-based job scheduler for
// running agent prompts on a recurring schedule. Schedules use a
// small grammar — "every 5m", "every 1h30m", "every 24h" — parsed
// into a time.Duration via ParseSchedule.
package cron

import (
	"fmt"
	"strings"
	"time"
)

// Schedule describes when a Job should fire.
type Schedule struct {
	Every time.Duration
}

// ParseSchedule accepts strings like "every 5m" or "every 1h30m".
// Returns a Schedule with the parsed interval. Zero and negative
// intervals are rejected.
func ParseSchedule(s string) (Schedule, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if !strings.HasPrefix(s, "every ") {
		return Schedule{}, fmt.Errorf("cron: schedule must start with 'every ', got %q", s)
	}
	rest := strings.TrimSpace(strings.TrimPrefix(s, "every "))
	// Support day suffix by expanding to hours (time.ParseDuration has no "d").
	if strings.HasSuffix(rest, "d") {
		d := strings.TrimSuffix(rest, "d")
		n, err := time.ParseDuration(d + "h")
		if err != nil {
			return Schedule{}, fmt.Errorf("cron: invalid day interval: %w", err)
		}
		return Schedule{Every: n * 24}, nil
	}
	d, err := time.ParseDuration(rest)
	if err != nil {
		return Schedule{}, fmt.Errorf("cron: invalid duration %q: %w", rest, err)
	}
	if d <= 0 {
		return Schedule{}, fmt.Errorf("cron: interval must be positive, got %s", d)
	}
	return Schedule{Every: d}, nil
}
```

- [ ] **Step 2:** Create `cron/schedule_test.go`:

```go
package cron

import (
	"testing"
	"time"
)

func TestParseSchedule(t *testing.T) {
	cases := []struct {
		in   string
		want time.Duration
	}{
		{"every 5m", 5 * time.Minute},
		{"every 1h30m", 90 * time.Minute},
		{"every 24h", 24 * time.Hour},
		{"every 1d", 24 * time.Hour},
		{"every 7d", 7 * 24 * time.Hour},
	}
	for _, c := range cases {
		s, err := ParseSchedule(c.in)
		if err != nil {
			t.Errorf("ParseSchedule(%q): unexpected error: %v", c.in, err)
			continue
		}
		if s.Every != c.want {
			t.Errorf("ParseSchedule(%q): got %s, want %s", c.in, s.Every, c.want)
		}
	}
}

func TestParseScheduleErrors(t *testing.T) {
	bad := []string{"5m", "every now", "every -1m", "every 0s"}
	for _, in := range bad {
		if _, err := ParseSchedule(in); err == nil {
			t.Errorf("expected error for %q", in)
		}
	}
}
```

- [ ] **Step 3:** `go test ./cron/...` — PASS.
- [ ] **Step 4:** Commit `feat(cron): add ParseSchedule for interval schedules`.

---

## Task 3: cron.Scheduler

- [ ] **Step 1:** Create `cron/scheduler.go`:

```go
package cron

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// Job is a single scheduled work unit.
type Job struct {
	Name     string
	Schedule Schedule
	// Run is called on each tick; it should be fast to start and
	// honor ctx cancellation. Scheduler logs any returned error.
	Run func(ctx context.Context) error
}

// Scheduler runs a fixed set of Jobs concurrently.
type Scheduler struct {
	jobs []Job
	mu   sync.Mutex
}

func NewScheduler() *Scheduler { return &Scheduler{} }

// Add registers a job. Must be called before Start.
func (s *Scheduler) Add(j Job) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs = append(s.jobs, j)
}

// Start runs all registered jobs until ctx is cancelled.
func (s *Scheduler) Start(ctx context.Context) error {
	s.mu.Lock()
	jobs := append([]Job{}, s.jobs...)
	s.mu.Unlock()
	if len(jobs) == 0 {
		return fmt.Errorf("cron: no jobs registered")
	}
	slog.InfoContext(ctx, "cron: scheduler started", "jobs", len(jobs))

	var wg sync.WaitGroup
	for _, j := range jobs {
		wg.Add(1)
		go func(j Job) {
			defer wg.Done()
			runJobLoop(ctx, j)
		}(j)
	}
	wg.Wait()
	return nil
}

// runJobLoop fires j.Run on each tick. The first run happens after
// the first tick — there is no run-on-startup in Plan 8.
func runJobLoop(ctx context.Context, j Job) {
	t := time.NewTicker(j.Schedule.Every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			slog.InfoContext(ctx, "cron: running job", "job", j.Name)
			if err := j.Run(ctx); err != nil {
				slog.ErrorContext(ctx, "cron: job failed", "job", j.Name, "err", err.Error())
			}
		}
	}
}
```

- [ ] **Step 2:** Create `cron/scheduler_test.go`:

```go
package cron

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestSchedulerFiresJob(t *testing.T) {
	var hits int32
	s := NewScheduler()
	s.Add(Job{
		Name:     "test",
		Schedule: Schedule{Every: 20 * time.Millisecond},
		Run: func(context.Context) error {
			atomic.AddInt32(&hits, 1)
			return nil
		},
	})
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Millisecond)
	defer cancel()
	_ = s.Start(ctx)
	if atomic.LoadInt32(&hits) < 2 {
		t.Errorf("expected at least 2 hits, got %d", hits)
	}
}

func TestSchedulerNoJobs(t *testing.T) {
	s := NewScheduler()
	if err := s.Start(context.Background()); err == nil {
		t.Error("expected error")
	}
}
```

- [ ] **Step 3:** `go test ./cron/...` — PASS.
- [ ] **Step 4:** Commit `feat(cron): add Scheduler with per-job goroutines`.

---

## Task 4: Config block + CLI cron command

- [ ] **Step 1:** Add to `config/config.go`:

```go
// CronConfig holds cron scheduler configuration.
type CronConfig struct {
	Jobs []CronJobConfig `yaml:"jobs,omitempty"`
}

// CronJobConfig is a single scheduled prompt.
type CronJobConfig struct {
	Name     string `yaml:"name"`
	Schedule string `yaml:"schedule"` // e.g. "every 5m"
	Prompt   string `yaml:"prompt"`
	Model    string `yaml:"model,omitempty"`
}

// LoggingConfig controls the slog output level.
type LoggingConfig struct {
	Level string `yaml:"level,omitempty"` // debug, info, warn, error
}
```

Then add fields to `Config`:
```go
Cron              CronConfig                `yaml:"cron,omitempty"`
Logging           LoggingConfig             `yaml:"logging,omitempty"`
```

- [ ] **Step 2:** Create `cli/cron.go`:

```go
package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/google/uuid"
	"github.com/nousresearch/hermes-agent/agent"
	"github.com/nousresearch/hermes-agent/config"
	"github.com/nousresearch/hermes-agent/cron"
	"github.com/nousresearch/hermes-agent/logging"
	"github.com/spf13/cobra"
)

func newCronCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "cron",
		Short: "Run scheduled cron jobs",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCron(cmd.Context(), app)
		},
	}
}

func runCron(ctx context.Context, app *App) error {
	logging.Setup(app.Config.Logging.Level)

	if err := ensureStorage(app); err != nil {
		return err
	}
	primary, _, err := buildPrimaryProvider(app.Config)
	if err != nil {
		return err
	}

	sched := cron.NewScheduler()
	for _, jc := range app.Config.Cron.Jobs {
		if jc.Name == "" || jc.Schedule == "" || jc.Prompt == "" {
			fmt.Fprintf(os.Stderr, "cron: skipping malformed job %q\n", jc.Name)
			continue
		}
		schedule, err := cron.ParseSchedule(jc.Schedule)
		if err != nil {
			fmt.Fprintf(os.Stderr, "cron: skipping %s: %v\n", jc.Name, err)
			continue
		}
		job := buildCronJob(jc, schedule, primary, app)
		sched.Add(job)
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() { <-sig; cancel() }()

	return sched.Start(runCtx)
}

func buildCronJob(jc config.CronJobConfig, sched cron.Schedule, prov any, app *App) cron.Job {
	// Capture by value for the closure.
	jobName := jc.Name
	prompt := jc.Prompt
	model := jc.Model
	return cron.Job{
		Name:     jobName,
		Schedule: sched,
		Run: func(ctx context.Context) error {
			ctx = logging.WithRequestID(ctx, uuid.NewString())
			eng := agent.NewEngineWithTools(
				provFrom(prov), app.Storage, nil,
				app.Config.Agent, "cron",
			)
			_, err := eng.RunConversation(ctx, &agent.RunOptions{
				UserMessage: prompt,
				SessionID:   "cron-" + jobName + "-" + uuid.NewString(),
				Model:       model,
			})
			return err
		},
	}
}

// provFrom is a tiny adapter from any to provider.Provider. It exists
// so buildCronJob doesn't need to import the provider package just to
// type the parameter.
func provFrom(p any) provFromType { return p.(provFromType) }

type provFromType interface {
	// Matches provider.Provider — but we declare it locally so cli/cron.go
	// doesn't depend on the full provider package for a type assertion.
	Name() string
}
```

Note: the `provFrom`/`provFromType` trick is ugly — replace with a direct import of `provider.Provider` if easier. The plan allows that.

- [ ] **Step 3:** Add the subcommand to `cli/root.go` via `newCronCmd(app)`.
- [ ] **Step 4:** `go build ./... && go test ./...` — PASS.
- [ ] **Step 5:** Commit `feat(cli): add hermes cron subcommand wired to cron.Scheduler`.

---

## Task 5: ACP platform adapter

- [ ] **Step 1:** Create `gateway/platforms/acp.go`:

```go
package platforms

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/nousresearch/hermes-agent/gateway"
)

// ACP is a minimal Agent Communication Protocol adapter. It exposes a
// POST /acp/messages endpoint that accepts an ACP message envelope
// and returns the assistant's reply synchronously.
//
// ACP message shape (Plan 8 subset):
//
//	{
//	  "session_id": "...",
//	  "parts": [{"type":"text","text":"..."}]
//	}
//
// The adapter only handles text parts; structured parts (images,
// tool calls) are ignored in Plan 8.
type ACP struct {
	addr  string
	token string
	srv   *http.Server
}

func NewACP(addr, token string) *ACP {
	if addr == "" {
		addr = ":8081"
	}
	return &ACP{addr: addr, token: token}
}

func (a *ACP) Name() string { return "acp" }

type acpPart struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type acpRequest struct {
	SessionID string    `json:"session_id"`
	Parts     []acpPart `json:"parts"`
}

type acpResponse struct {
	SessionID string    `json:"session_id"`
	Parts     []acpPart `json:"parts"`
}

func (a *ACP) Run(ctx context.Context, handler gateway.MessageHandler) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/acp/messages", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if a.token != "" && r.Header.Get("Authorization") != "Bearer "+a.token {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		var req acpRequest
		if err := json.Unmarshal(body, &req); err != nil {
			http.Error(w, "invalid json: "+err.Error(), http.StatusBadRequest)
			return
		}
		text := ""
		for _, p := range req.Parts {
			if p.Type == "text" {
				text += p.Text
			}
		}
		if text == "" {
			http.Error(w, "no text parts", http.StatusBadRequest)
			return
		}
		in := gateway.IncomingMessage{
			Platform: a.Name(),
			UserID:   req.SessionID,
			ChatID:   req.SessionID,
			Text:     text,
		}
		out, err := handler(r.Context(), in)
		if err != nil {
			http.Error(w, "handler error: "+err.Error(), http.StatusInternalServerError)
			return
		}
		reply := acpResponse{SessionID: req.SessionID, Parts: []acpPart{}}
		if out != nil {
			reply.Parts = append(reply.Parts, acpPart{Type: "text", Text: out.Text})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(reply)
	})

	srv := &http.Server{Addr: a.addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	a.srv = srv

	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
		return nil
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("acp: %w", err)
	}
}

func (a *ACP) SendReply(context.Context, gateway.OutgoingMessage) error { return nil }
```

- [ ] **Step 2:** Create `gateway/platforms/acp_test.go`:

```go
package platforms

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/nousresearch/hermes-agent/gateway"
)

func TestACPRoundTripWithAuth(t *testing.T) {
	addr := freePort(t)
	srv := NewACP(addr, "tok")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Run(ctx, func(_ context.Context, in gateway.IncomingMessage) (*gateway.OutgoingMessage, error) {
			return &gateway.OutgoingMessage{Text: "echo: " + in.Text}, nil
		})
	}()

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if c, err := net.Dial("tcp", addr); err == nil {
			_ = c.Close()
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	body := `{"session_id":"s1","parts":[{"type":"text","text":"hello"}]}`
	req, _ := http.NewRequest("POST", "http://"+addr+"/acp/messages", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer tok")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var out acpResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(out.Parts) != 1 || out.Parts[0].Text != "echo: hello" {
		t.Errorf("unexpected parts: %+v", out.Parts)
	}

	// Missing auth should 401
	reqNoAuth, _ := http.NewRequest("POST", "http://"+addr+"/acp/messages", bytes.NewBufferString(body))
	reqNoAuth.Header.Set("Content-Type", "application/json")
	resp2, err := http.DefaultClient.Do(reqNoAuth)
	if err != nil {
		t.Fatalf("post2: %v", err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != 401 {
		t.Errorf("expected 401, got %d", resp2.StatusCode)
	}

	cancel()
	<-errCh
}
```

- [ ] **Step 3:** Wire ACP into `cli/gateway.go` `buildPlatform` switch:

```go
case "acp":
    return platforms.NewACP(pc.Options["addr"], pc.Options["token"]), nil
```

- [ ] **Step 4:** `go test ./gateway/platforms/...` — PASS.
- [ ] **Step 5:** Commit `feat(gateway/platforms): add ACP adapter with token auth`.

---

## Task 6: Slog in gateway.go

- [ ] **Step 1:** In `gateway/gateway.go`, replace `log.Printf` calls with `slog.InfoContext`/`slog.ErrorContext`, remove `log` import.

Specifically:
- `log.Printf("gateway: starting platform %s", name)` → `slog.InfoContext(ctx, "gateway: starting platform", "platform", name)`
- In `handler.go`, `log.Printf("gateway: %s: handler error: %v", …)` → `slog.ErrorContext(ctx, "gateway: handler error", "platform", p.Name(), "err", err.Error())`

Attach a new request ID at the start of `handleMessage`:
```go
ctx = logging.WithRequestID(ctx, "")
```

- [ ] **Step 2:** `go test ./gateway/...` — PASS.
- [ ] **Step 3:** `go test ./...` — PASS.
- [ ] **Step 4:** Commit `feat(gateway): switch to slog with request-id context`.

---

## Verification Checklist

- [ ] `go test ./logging/... ./cron/... ./gateway/...` passes
- [ ] `hermes cron` starts when at least one cron.jobs entry is valid
- [ ] ACP POST round-trips when enabled in gateway config
- [ ] Gateway logs are JSON on stderr when `logging.level: info`
