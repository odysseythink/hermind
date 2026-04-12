# Phase 6: RL Training Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Go management layer for RL training that delegates actual training to the Python/Tinker-Atropos infrastructure via subprocess management.

**Architecture:** New `rl/` package with config management, subprocess lifecycle, and WandB HTTP queries. CLI subcommand group `hermes rl` with list/config/start/status/stop. Tool functions registered in the tool system for agent use. Zero new Go dependencies — uses `os/exec` for subprocess management and `net/http` for WandB API.

**Tech Stack:** Go 1.25, stdlib `os/exec`, `net/http`, `encoding/json`, `gopkg.in/yaml.v3`

---

## File Structure

```
hermes-agent-go/
├── rl/
│   ├── config.go           # Config read/write/validate (locked + editable fields)
│   ├── config_test.go      # Config tests
│   ├── manager.go          # Subprocess lifecycle (start/stop/status)
│   ├── manager_test.go     # Manager tests
│   ├── wandb.go            # WandB HTTP status queries
│   ├── wandb_test.go       # WandB tests
│   └── env.go              # Environment discovery (scan for BaseEnv subclasses)
├── cli/
│   ├── root.go             # (modify: add newRLCmd)
│   └── rl.go               # CLI subcommand group
├── tool/rl/
│   └── register.go         # Tool function registration
```

---

### Task 1: RL config management

**Files:**
- Create: `hermes-agent-go/rl/config.go`
- Create: `hermes-agent-go/rl/config_test.go`

- [ ] **Step 1: Write the failing test**

Create `hermes-agent-go/rl/config_test.go`:

```go
package rl

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigDefaults(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Tokenizer != "Qwen/Qwen3-8B" {
		t.Errorf("tokenizer = %q", cfg.Tokenizer)
	}
	if cfg.MaxWorkers != 2048 {
		t.Errorf("max_workers = %d", cfg.MaxWorkers)
	}
}

func TestConfigIsLocked(t *testing.T) {
	cfg := DefaultConfig()
	if !cfg.IsLocked("tokenizer") {
		t.Error("tokenizer should be locked")
	}
	if !cfg.IsLocked("max_workers") {
		t.Error("max_workers should be locked")
	}
	if cfg.IsLocked("lora_rank") {
		t.Error("lora_rank should not be locked")
	}
	if cfg.IsLocked("learning_rate") {
		t.Error("learning_rate should not be locked")
	}
}

func TestConfigSetEditable(t *testing.T) {
	cfg := DefaultConfig()
	if err := cfg.Set("lora_rank", 64); err != nil {
		t.Fatalf("Set lora_rank: %v", err)
	}
	if cfg.LoraRank != 64 {
		t.Errorf("lora_rank = %d", cfg.LoraRank)
	}
}

func TestConfigSetLockedFails(t *testing.T) {
	cfg := DefaultConfig()
	if err := cfg.Set("tokenizer", "other"); err == nil {
		t.Error("expected error setting locked field")
	}
}

func TestConfigSaveLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	cfg := DefaultConfig()
	cfg.LoraRank = 64
	if err := cfg.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.LoraRank != 64 {
		t.Errorf("lora_rank = %d", loaded.LoraRank)
	}
	// Locked fields should still have defaults.
	if loaded.Tokenizer != "Qwen/Qwen3-8B" {
		t.Errorf("tokenizer = %q", loaded.Tokenizer)
	}
}

func TestConfigValidate(t *testing.T) {
	cfg := DefaultConfig()
	cfg.LoraRank = -1
	if err := cfg.Validate(); err == nil {
		t.Error("expected validation error for negative lora_rank")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd hermes-agent-go && go test ./rl/ -run TestConfig -v
```

Expected: Compilation error — package doesn't exist.

- [ ] **Step 3: Implement config.go**

Create `hermes-agent-go/rl/config.go`:

```go
package rl

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config holds RL training configuration.
// Some fields are locked (infrastructure) and some are editable (hyperparameters).
type Config struct {
	// Locked infrastructure fields.
	Tokenizer  string `yaml:"tokenizer"`
	MaxWorkers int    `yaml:"max_workers"`

	// Editable hyperparameters.
	LoraRank           int     `yaml:"lora_rank"`
	LearningRate       float64 `yaml:"learning_rate"`
	CheckpointInterval int     `yaml:"checkpoint_interval"`
	WandbName          string  `yaml:"wandb_name"`
	Environment        string  `yaml:"environment"`
}

// lockedFields is the set of fields that cannot be modified via Set.
var lockedFields = map[string]bool{
	"tokenizer":   true,
	"max_workers": true,
}

// DefaultConfig returns the default training configuration.
func DefaultConfig() *Config {
	return &Config{
		Tokenizer:          "Qwen/Qwen3-8B",
		MaxWorkers:         2048,
		LoraRank:           32,
		LearningRate:       0.00004,
		CheckpointInterval: 100,
	}
}

// IsLocked reports whether a field is locked (infrastructure setting).
func (c *Config) IsLocked(field string) bool {
	return lockedFields[field]
}

// Set updates an editable field by name. Returns error if field is locked.
func (c *Config) Set(field string, value any) error {
	if c.IsLocked(field) {
		return fmt.Errorf("field %q is locked and cannot be modified", field)
	}
	switch field {
	case "lora_rank":
		v, ok := toInt(value)
		if !ok {
			return fmt.Errorf("lora_rank must be an integer")
		}
		c.LoraRank = v
	case "learning_rate":
		v, ok := toFloat(value)
		if !ok {
			return fmt.Errorf("learning_rate must be a number")
		}
		c.LearningRate = v
	case "checkpoint_interval":
		v, ok := toInt(value)
		if !ok {
			return fmt.Errorf("checkpoint_interval must be an integer")
		}
		c.CheckpointInterval = v
	case "wandb_name":
		v, ok := value.(string)
		if !ok {
			return fmt.Errorf("wandb_name must be a string")
		}
		c.WandbName = v
	case "environment":
		v, ok := value.(string)
		if !ok {
			return fmt.Errorf("environment must be a string")
		}
		c.Environment = v
	default:
		return fmt.Errorf("unknown field: %q", field)
	}
	return nil
}

// Validate checks that all fields have valid values.
func (c *Config) Validate() error {
	if c.LoraRank <= 0 {
		return fmt.Errorf("lora_rank must be positive, got %d", c.LoraRank)
	}
	if c.LearningRate <= 0 {
		return fmt.Errorf("learning_rate must be positive, got %f", c.LearningRate)
	}
	if c.CheckpointInterval <= 0 {
		return fmt.Errorf("checkpoint_interval must be positive, got %d", c.CheckpointInterval)
	}
	return nil
}

// Save writes the config to a YAML file.
func (c *Config) Save(path string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// LoadConfig reads a config from a YAML file, applying defaults for locked fields.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	// Ensure locked fields keep their defaults.
	defaults := DefaultConfig()
	cfg.Tokenizer = defaults.Tokenizer
	cfg.MaxWorkers = defaults.MaxWorkers
	return cfg, nil
}

func toInt(v any) (int, bool) {
	switch val := v.(type) {
	case int:
		return val, true
	case int64:
		return int(val), true
	case float64:
		return int(val), true
	default:
		return 0, false
	}
}

func toFloat(v any) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	default:
		return 0, false
	}
}
```

- [ ] **Step 4: Run tests**

```bash
cd hermes-agent-go && go test ./rl/ -run TestConfig -v
```

Expected: All 6 config tests pass.

- [ ] **Step 5: Commit**

```bash
cd /Users/ranwei/workspace/go_work/hermes-agent-rewrite
git add hermes-agent-go/rl/config.go hermes-agent-go/rl/config_test.go
git commit -m "feat(rl): add training config management with locked/editable fields"
```

---

### Task 2: Subprocess manager

**Files:**
- Create: `hermes-agent-go/rl/manager.go`
- Create: `hermes-agent-go/rl/manager_test.go`

- [ ] **Step 1: Write the failing test**

Create `hermes-agent-go/rl/manager_test.go`:

```go
package rl

import (
	"context"
	"testing"
	"time"
)

func TestManagerStartAndStop(t *testing.T) {
	m := NewManager()

	// Start a simple process (sleep).
	runID, err := m.Start(context.Background(), "sleep", []string{"10"})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if runID == "" {
		t.Fatal("empty run ID")
	}

	// Check status.
	status := m.Status(runID)
	if status.State != "running" {
		t.Errorf("state = %q", status.State)
	}

	// Stop it.
	if err := m.Stop(runID); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	// Wait a bit for process to exit.
	time.Sleep(100 * time.Millisecond)

	status = m.Status(runID)
	if status.State != "stopped" {
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
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd hermes-agent-go && go test ./rl/ -run TestManager -v
```

Expected: Compilation error.

- [ ] **Step 3: Implement manager.go**

Create `hermes-agent-go/rl/manager.go`:

```go
package rl

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
)

// RunStatus describes the state of a training run.
type RunStatus struct {
	RunID     string    `json:"run_id"`
	State     string    `json:"state"` // "running", "stopped", "failed", "unknown"
	Command   string    `json:"command"`
	StartTime time.Time `json:"start_time"`
	PID       int       `json:"pid,omitempty"`
	Error     string    `json:"error,omitempty"`
}

type run struct {
	id        string
	cmd       *exec.Cmd
	startTime time.Time
	done      chan struct{}
	err       error
}

// Manager manages training subprocess lifecycles.
type Manager struct {
	mu   sync.Mutex
	runs map[string]*run
}

func NewManager() *Manager {
	return &Manager{runs: make(map[string]*run)}
}

// Start launches a command as a managed subprocess.
func (m *Manager) Start(ctx context.Context, command string, args []string) (string, error) {
	id := uuid.New().String()[:8]
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("rl: start %s: %w", command, err)
	}

	r := &run{
		id:        id,
		cmd:       cmd,
		startTime: time.Now().UTC(),
		done:      make(chan struct{}),
	}

	// Wait for the process in background.
	go func() {
		r.err = cmd.Wait()
		close(r.done)
		slog.Info("rl: process exited", "run_id", id, "err", r.err)
	}()

	m.mu.Lock()
	m.runs[id] = r
	m.mu.Unlock()

	slog.Info("rl: started", "run_id", id, "pid", cmd.Process.Pid, "command", command)
	return id, nil
}

// Status returns the current state of a run.
func (m *Manager) Status(runID string) RunStatus {
	m.mu.Lock()
	r, ok := m.runs[runID]
	m.mu.Unlock()

	if !ok {
		return RunStatus{RunID: runID, State: "unknown"}
	}

	select {
	case <-r.done:
		state := "stopped"
		errMsg := ""
		if r.err != nil {
			state = "failed"
			errMsg = r.err.Error()
		}
		return RunStatus{
			RunID:     runID,
			State:     state,
			Command:   r.cmd.Path,
			StartTime: r.startTime,
			Error:     errMsg,
		}
	default:
		return RunStatus{
			RunID:     runID,
			State:     "running",
			Command:   r.cmd.Path,
			StartTime: r.startTime,
			PID:       r.cmd.Process.Pid,
		}
	}
}

// Stop sends SIGTERM to a running process, waits 30s, then SIGKILL.
func (m *Manager) Stop(runID string) error {
	m.mu.Lock()
	r, ok := m.runs[runID]
	m.mu.Unlock()

	if !ok {
		return fmt.Errorf("rl: unknown run %q", runID)
	}

	// Send SIGTERM to the process group.
	if r.cmd.Process != nil {
		_ = syscall.Kill(-r.cmd.Process.Pid, syscall.SIGTERM)
	}

	// Wait up to 30s for graceful exit.
	select {
	case <-r.done:
		return nil
	case <-time.After(30 * time.Second):
	}

	// Force kill.
	if r.cmd.Process != nil {
		_ = syscall.Kill(-r.cmd.Process.Pid, syscall.SIGKILL)
	}

	<-r.done
	return nil
}

// List returns the status of all tracked runs.
func (m *Manager) List() []RunStatus {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []RunStatus
	for id := range m.runs {
		// Release lock temporarily for Status.
		m.mu.Unlock()
		out = append(out, m.Status(id))
		m.mu.Lock()
	}
	return out
}
```

**Note:** The `List()` method has a lock issue — it unlocks/relocks inside the loop which is fragile. A better implementation collects the keys first, then calls Status outside the lock:

```go
func (m *Manager) List() []RunStatus {
	m.mu.Lock()
	ids := make([]string, 0, len(m.runs))
	for id := range m.runs {
		ids = append(ids, id)
	}
	m.mu.Unlock()

	out := make([]RunStatus, 0, len(ids))
	for _, id := range ids {
		out = append(out, m.Status(id))
	}
	return out
}
```

Use this corrected version.

- [ ] **Step 4: Run tests**

```bash
cd hermes-agent-go && go test ./rl/ -run TestManager -v
```

Expected: All 3 manager tests pass.

- [ ] **Step 5: Commit**

```bash
cd /Users/ranwei/workspace/go_work/hermes-agent-rewrite
git add hermes-agent-go/rl/manager.go hermes-agent-go/rl/manager_test.go
git commit -m "feat(rl): add subprocess manager for training lifecycle"
```

---

### Task 3: WandB HTTP status queries

**Files:**
- Create: `hermes-agent-go/rl/wandb.go`
- Create: `hermes-agent-go/rl/wandb_test.go`

- [ ] **Step 1: Write the failing test**

Create `hermes-agent-go/rl/wandb_test.go`:

```go
package rl

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWandBGetRunStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer wb-key" {
			t.Errorf("auth = %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"state": "running",
			"config": {"learning_rate": {"value": 0.0001}},
			"summary": {"loss": 0.5, "step": 100}
		}`))
	}))
	defer srv.Close()

	client := NewWandBClient("wb-key", srv.URL)
	status, err := client.GetRunStatus(context.Background(), "entity/project/runs/run123")
	if err != nil {
		t.Fatalf("GetRunStatus: %v", err)
	}
	if status.State != "running" {
		t.Errorf("state = %q", status.State)
	}
	if status.Summary["loss"] != 0.5 {
		t.Errorf("loss = %v", status.Summary["loss"])
	}
	if status.Summary["step"] != float64(100) {
		t.Errorf("step = %v", status.Summary["step"])
	}
}

func TestWandBClientNotConfigured(t *testing.T) {
	client := NewWandBClient("", "")
	_, err := client.GetRunStatus(context.Background(), "entity/project/runs/run123")
	if err == nil {
		t.Error("expected error when not configured")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd hermes-agent-go && go test ./rl/ -run TestWandB -v
```

Expected: Compilation error.

- [ ] **Step 3: Implement wandb.go**

Create `hermes-agent-go/rl/wandb.go`:

```go
package rl

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// WandBRunStatus holds the status of a WandB run.
type WandBRunStatus struct {
	State   string             `json:"state"`
	Config  map[string]any     `json:"config"`
	Summary map[string]float64 `json:"summary"`
}

// WandBClient queries the WandB API for run status.
type WandBClient struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

func NewWandBClient(apiKey, baseURL string) *WandBClient {
	if baseURL == "" {
		baseURL = "https://api.wandb.ai"
	}
	return &WandBClient{
		apiKey:  apiKey,
		baseURL: baseURL,
		client:  &http.Client{Timeout: 15 * time.Second},
	}
}

// GetRunStatus fetches the current status of a WandB run.
// path is like "entity/project/runs/run_id".
func (w *WandBClient) GetRunStatus(ctx context.Context, path string) (*WandBRunStatus, error) {
	if w.apiKey == "" {
		return nil, fmt.Errorf("wandb: api key not configured")
	}
	url := w.baseURL + "/api/v1/" + path
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+w.apiKey)

	resp, err := w.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("wandb: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("wandb: status %d: %s", resp.StatusCode, string(body))
	}

	var status WandBRunStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, fmt.Errorf("wandb: decode: %w", err)
	}
	return &status, nil
}
```

- [ ] **Step 4: Run tests**

```bash
cd hermes-agent-go && go test ./rl/ -run TestWandB -v
```

Expected: Both tests pass.

- [ ] **Step 5: Commit**

```bash
cd /Users/ranwei/workspace/go_work/hermes-agent-rewrite
git add hermes-agent-go/rl/wandb.go hermes-agent-go/rl/wandb_test.go
git commit -m "feat(rl): add WandB HTTP client for training status queries"
```

---

### Task 4: CLI subcommand group + tool registration

**Files:**
- Create: `hermes-agent-go/cli/rl.go`
- Modify: `hermes-agent-go/cli/root.go`
- Create: `hermes-agent-go/tool/rl/register.go`

- [ ] **Step 1: Create cli/rl.go**

Create `hermes-agent-go/cli/rl.go`:

```go
package cli

import (
	"encoding/json"
	"fmt"

	"github.com/nousresearch/hermes-agent/rl"
	"github.com/spf13/cobra"
)

func newRLCmd(app *App) *cobra.Command {
	manager := rl.NewManager()

	cmd := &cobra.Command{
		Use:   "rl",
		Short: "Manage RL training runs",
	}

	cmd.AddCommand(
		newRLConfigCmd(app),
		newRLStartCmd(app, manager),
		newRLStatusCmd(manager),
		newRLStopCmd(manager),
		newRLListCmd(manager),
	)

	return cmd
}

func newRLConfigCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "config",
		Short: "Show current RL training configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := rl.DefaultConfig()
			data, _ := json.MarshalIndent(cfg, "", "  ")
			fmt.Println(string(data))
			return nil
		},
	}
}

func newRLStartCmd(app *App, manager *rl.Manager) *cobra.Command {
	return &cobra.Command{
		Use:   "start [python-entrypoint]",
		Short: "Start a training run",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := manager.Start(cmd.Context(), args[0], args[1:])
			if err != nil {
				return err
			}
			fmt.Printf("Training run started: %s\n", id)
			return nil
		},
	}
}

func newRLStatusCmd(manager *rl.Manager) *cobra.Command {
	return &cobra.Command{
		Use:   "status [run-id]",
		Short: "Check training run status",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			status := manager.Status(args[0])
			data, _ := json.MarshalIndent(status, "", "  ")
			fmt.Println(string(data))
			return nil
		},
	}
}

func newRLStopCmd(manager *rl.Manager) *cobra.Command {
	return &cobra.Command{
		Use:   "stop [run-id]",
		Short: "Stop a training run",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := manager.Stop(args[0]); err != nil {
				return err
			}
			fmt.Printf("Run %s stopped\n", args[0])
			return nil
		},
	}
}

func newRLListCmd(manager *rl.Manager) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all training runs",
		RunE: func(cmd *cobra.Command, args []string) error {
			runs := manager.List()
			if len(runs) == 0 {
				fmt.Println("No active runs")
				return nil
			}
			data, _ := json.MarshalIndent(runs, "", "  ")
			fmt.Println(string(data))
			return nil
		},
	}
}
```

- [ ] **Step 2: Wire into root.go**

In `hermes-agent-go/cli/root.go`, add `newRLCmd(app)` to the `AddCommand` call:

Find:
```go
	root.AddCommand(
		newRunCmd(app),
		newGatewayCmd(app),
```

Add `newRLCmd(app),` after `newUpgradeCmd(app),`:

```go
		newUpgradeCmd(app),
		newRLCmd(app),
		newVersionCmd(),
```

- [ ] **Step 3: Create tool/rl/register.go**

Create `hermes-agent-go/tool/rl/register.go`:

```go
package rl

import (
	"context"
	"encoding/json"
	"fmt"

	rlpkg "github.com/nousresearch/hermes-agent/rl"
	"github.com/nousresearch/hermes-agent/tool"
)

// RegisterAll registers RL training tools in the registry.
func RegisterAll(reg *tool.Registry, manager *rlpkg.Manager) {
	reg.Register(&tool.Entry{
		Name:        "rl_get_current_config",
		Toolset:     "rl",
		Description: "Get the current RL training configuration",
		Emoji:       "⚙️",
		Handler: func(ctx context.Context, args json.RawMessage) (string, error) {
			cfg := rlpkg.DefaultConfig()
			data, _ := json.MarshalIndent(cfg, "", "  ")
			return string(data), nil
		},
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name:        "rl_get_current_config",
				Description: "Get the current RL training configuration, showing locked and editable fields.",
				Parameters:  json.RawMessage(`{"type":"object","properties":{}}`),
			},
		},
	})

	reg.Register(&tool.Entry{
		Name:        "rl_start_training",
		Toolset:     "rl",
		Description: "Start a training run",
		Emoji:       "🚀",
		Handler: func(ctx context.Context, args json.RawMessage) (string, error) {
			var params struct {
				Command string   `json:"command"`
				Args    []string `json:"args"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", err
			}
			if params.Command == "" {
				return "", fmt.Errorf("command is required")
			}
			id, err := manager.Start(ctx, params.Command, params.Args)
			if err != nil {
				return "", err
			}
			return fmt.Sprintf(`{"run_id":"%s","status":"started"}`, id), nil
		},
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name:        "rl_start_training",
				Description: "Start a new RL training run with a Python entrypoint.",
				Parameters:  json.RawMessage(`{"type":"object","properties":{"command":{"type":"string","description":"Python entrypoint command"},"args":{"type":"array","items":{"type":"string"},"description":"Command arguments"}},"required":["command"]}`),
			},
		},
	})

	reg.Register(&tool.Entry{
		Name:        "rl_check_status",
		Toolset:     "rl",
		Description: "Check training run status",
		Emoji:       "📊",
		Handler: func(ctx context.Context, args json.RawMessage) (string, error) {
			var params struct {
				RunID string `json:"run_id"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", err
			}
			status := manager.Status(params.RunID)
			data, _ := json.MarshalIndent(status, "", "  ")
			return string(data), nil
		},
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name:        "rl_check_status",
				Description: "Check the status of a training run.",
				Parameters:  json.RawMessage(`{"type":"object","properties":{"run_id":{"type":"string","description":"The run ID to check"}},"required":["run_id"]}`),
			},
		},
	})

	reg.Register(&tool.Entry{
		Name:        "rl_stop_training",
		Toolset:     "rl",
		Description: "Stop a training run",
		Emoji:       "🛑",
		Handler: func(ctx context.Context, args json.RawMessage) (string, error) {
			var params struct {
				RunID string `json:"run_id"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", err
			}
			if err := manager.Stop(params.RunID); err != nil {
				return "", err
			}
			return fmt.Sprintf(`{"run_id":"%s","status":"stopped"}`, params.RunID), nil
		},
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name:        "rl_stop_training",
				Description: "Stop an active training run.",
				Parameters:  json.RawMessage(`{"type":"object","properties":{"run_id":{"type":"string","description":"The run ID to stop"}},"required":["run_id"]}`),
			},
		},
	})
}
```

- [ ] **Step 4: Verify it compiles**

```bash
cd hermes-agent-go && go build ./cli/ && go build ./tool/rl/
```

Expected: Both compile.

- [ ] **Step 5: Run all tests**

```bash
cd hermes-agent-go && go test ./...
```

Expected: All pass.

- [ ] **Step 6: Commit**

```bash
cd /Users/ranwei/workspace/go_work/hermes-agent-rewrite
git add hermes-agent-go/cli/rl.go hermes-agent-go/cli/root.go hermes-agent-go/tool/rl/register.go
git commit -m "feat(rl): add CLI subcommand group and tool registration"
```
