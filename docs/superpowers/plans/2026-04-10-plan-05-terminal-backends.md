# Plan 5: Remaining Terminal Backends Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the 5 remaining terminal backends (Docker, SSH, Singularity, Modal, Daytona) so the agent can execute shell commands in containers, over SSH to remote hosts, in HPC environments, or via serverless cloud platforms.

**Architecture:** Each backend implements the existing `terminal.Backend` interface from Plan 2. Docker and Singularity wrap their respective CLI tools as subprocesses (avoids heavy SDK dependencies). SSH uses `golang.org/x/crypto/ssh` for a pure-Go implementation with no external CLI. Modal and Daytona use their HTTP APIs with documented request/response shapes. The factory at `terminal.New(backendType, cfg)` is expanded to dispatch to all 6 backends based on config. Config gains new fields for per-backend settings (Docker image, SSH credentials, Modal/Daytona API endpoints).

**Tech Stack:** Go 1.25, `os/exec` (Docker + Singularity subprocess), `golang.org/x/crypto/ssh` (SSH), `net/http` (Modal + Daytona), existing `config`, `tool`, `tool/terminal` packages. No new major dependencies beyond `x/crypto`.

**Deliverable at end of plan:**
```yaml
# ~/.hermes/config.yaml
terminal:
  backend: docker
  docker_image: golang:1.25-alpine
  timeout: 300

# or:
terminal:
  backend: ssh
  ssh_host: my-dev-box.example.com
  ssh_user: dev
  ssh_key: /home/me/.ssh/id_ed25519

# or:
terminal:
  backend: modal
  modal_base_url: https://api.modal.com/v1
  modal_token: env:MODAL_TOKEN

# or:
terminal:
  backend: local   # still the default
```

Running `./bin/hermes` picks the configured backend. Tool calls to `shell_execute` route through the right backend transparently — the Engine doesn't know or care which backend is used.

**Non-goals for this plan (deferred):**
- Persistent shell state across tool calls (all backends are stateless per-exec in Plan 5) — Plan 6 adds persistent shells
- Docker volume mounts beyond a static list from config — Plan 6 adds dynamic volume injection
- SSH multiplexing / connection reuse across calls — Plan 6
- Modal function caching (every exec rebuilds the image) — out of scope, that's a Modal feature
- Container image building — out of scope, users pre-build images
- Windows-specific backend quirks (docker for Windows, WSL) — best effort, not tested
- Integration tests against real Docker daemon / SSH servers / Modal accounts — unit tests with mocks only

**Plans 1-4 dependencies this plan touches:**
- `hermes-agent-go/tool/terminal/terminal.go` — `New(backendType, cfg)` factory expanded to 6 cases
- `hermes-agent-go/config/config.go` — `TerminalConfig` struct (new or expanded) with per-backend fields
- `hermes-agent-go/cli/repl.go` — uses `config.Terminal.Backend` instead of hardcoded "local"

---

## File Structure

```
hermes-agent-go/
├── config/
│   └── config.go                 # MODIFIED: add TerminalConfig
├── tool/terminal/
│   ├── terminal.go               # MODIFIED: factory dispatches all 6 backends
│   ├── local.go                  # (unchanged)
│   ├── local_test.go             # (unchanged)
│   ├── tools.go                  # (unchanged)
│   ├── docker.go                 # NEW: Docker CLI subprocess backend
│   ├── docker_test.go
│   ├── ssh.go                    # NEW: SSH backend via crypto/ssh
│   ├── ssh_test.go
│   ├── singularity.go            # NEW: Singularity CLI subprocess backend
│   ├── singularity_test.go
│   ├── modal.go                  # NEW: Modal HTTP API backend
│   ├── modal_test.go
│   ├── daytona.go                # NEW: Daytona HTTP API backend
│   └── daytona_test.go
└── cli/
    └── repl.go                   # MODIFIED: respects config.Terminal.Backend
```

---

## Task 1: Expand TerminalConfig

**Files:**
- Modify: `hermes-agent-go/config/config.go`
- Modify: `hermes-agent-go/config/loader_test.go`

- [ ] **Step 1: Add failing test**

Append to `hermes-agent-go/config/loader_test.go`:

```go
func TestLoadFromYAMLParsesTerminalConfig(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(yamlPath, []byte(`
terminal:
  backend: docker
  cwd: /workspace
  timeout: 120
  docker_image: golang:1.25-alpine
  docker_volumes:
    - /host/src:/workspace
  ssh_host: dev.example.com
  ssh_user: dev
  ssh_key: /home/me/.ssh/id_ed25519
  modal_base_url: https://api.modal.com/v1
  modal_token: test-modal-token
  daytona_base_url: https://api.daytona.io/v1
  daytona_token: test-daytona-token
  singularity_image: /opt/img/ubuntu.sif
`), 0o644)
	require.NoError(t, err)

	cfg, err := LoadFromPath(yamlPath)
	require.NoError(t, err)
	assert.Equal(t, "docker", cfg.Terminal.Backend)
	assert.Equal(t, "/workspace", cfg.Terminal.Cwd)
	assert.Equal(t, 120, cfg.Terminal.Timeout)
	assert.Equal(t, "golang:1.25-alpine", cfg.Terminal.DockerImage)
	assert.Equal(t, []string{"/host/src:/workspace"}, cfg.Terminal.DockerVolumes)
	assert.Equal(t, "dev.example.com", cfg.Terminal.SSHHost)
	assert.Equal(t, "dev", cfg.Terminal.SSHUser)
	assert.Equal(t, "https://api.modal.com/v1", cfg.Terminal.ModalBaseURL)
	assert.Equal(t, "test-modal-token", cfg.Terminal.ModalToken)
	assert.Equal(t, "https://api.daytona.io/v1", cfg.Terminal.DaytonaBaseURL)
	assert.Equal(t, "/opt/img/ubuntu.sif", cfg.Terminal.SingularityImage)
}
```

- [ ] **Step 2: Add the `Terminal` field to `Config` and define `TerminalConfig`**

In `config/config.go`, modify the `Config` struct:

```go
type Config struct {
	Model             string                    `yaml:"model"`
	Providers         map[string]ProviderConfig `yaml:"providers"`
	FallbackProviders []ProviderConfig          `yaml:"fallback_providers,omitempty"`
	Agent             AgentConfig               `yaml:"agent"`
	Terminal          TerminalConfig            `yaml:"terminal"`
	Storage           StorageConfig             `yaml:"storage"`
}
```

Then add the `TerminalConfig` type (place it after `AgentConfig`):

```go
// TerminalConfig holds settings for the terminal (shell exec) backend.
// Only the fields relevant to the selected Backend type are read.
type TerminalConfig struct {
	// Backend selects the execution backend. One of:
	//   "local"       — execute on the host OS via /bin/sh (default)
	//   "docker"      — wrap commands in "docker run --rm -i <image> sh -c ..."
	//   "ssh"         — run commands over SSH to a remote host
	//   "modal"       — call the Modal serverless function API
	//   "daytona"     — call the Daytona workspace exec API
	//   "singularity" — wrap commands in "singularity exec <image> sh -c ..."
	Backend string `yaml:"backend"`

	// Shared: working directory and default timeout (seconds, 0 = backend default)
	Cwd     string `yaml:"cwd,omitempty"`
	Timeout int    `yaml:"timeout,omitempty"`

	// Docker backend
	DockerImage   string   `yaml:"docker_image,omitempty"`
	DockerVolumes []string `yaml:"docker_volumes,omitempty"`

	// SSH backend
	SSHHost string `yaml:"ssh_host,omitempty"`
	SSHUser string `yaml:"ssh_user,omitempty"`
	SSHKey  string `yaml:"ssh_key,omitempty"` // path to private key file

	// Modal backend
	ModalBaseURL string `yaml:"modal_base_url,omitempty"`
	ModalToken   string `yaml:"modal_token,omitempty"`

	// Daytona backend
	DaytonaBaseURL string `yaml:"daytona_base_url,omitempty"`
	DaytonaToken   string `yaml:"daytona_token,omitempty"`

	// Singularity backend
	SingularityImage string `yaml:"singularity_image,omitempty"` // path to .sif file
}
```

- [ ] **Step 3: Default `Backend` to "local" and support env var expansion for tokens**

In `config/config.go`, update `Default()` to initialize `Terminal.Backend`:

```go
func Default() *Config {
	return &Config{
		Model:     "anthropic/claude-opus-4-6",
		Providers: map[string]ProviderConfig{},
		Agent: AgentConfig{
			MaxTurns:       90,
			GatewayTimeout: 1800,
		},
		Terminal: TerminalConfig{
			Backend: "local",
		},
		Storage: StorageConfig{
			Driver: "sqlite",
		},
	}
}
```

In `config/loader.go`, update `expandEnvVars` to also expand `ModalToken` and `DaytonaToken`:

```go
func expandEnvVars(cfg *Config) error {
	// Primary providers
	for name, p := range cfg.Providers {
		if strings.HasPrefix(p.APIKey, "env:") {
			varName := strings.TrimPrefix(p.APIKey, "env:")
			if varName == "" {
				return fmt.Errorf("config: provider %q has empty env variable reference", name)
			}
			p.APIKey = os.Getenv(varName)
			cfg.Providers[name] = p
		}
	}
	// Fallback providers
	for i, p := range cfg.FallbackProviders {
		if strings.HasPrefix(p.APIKey, "env:") {
			varName := strings.TrimPrefix(p.APIKey, "env:")
			if varName == "" {
				return fmt.Errorf("config: fallback provider %d has empty env variable reference", i)
			}
			p.APIKey = os.Getenv(varName)
			cfg.FallbackProviders[i] = p
		}
	}
	// Terminal tokens
	if strings.HasPrefix(cfg.Terminal.ModalToken, "env:") {
		varName := strings.TrimPrefix(cfg.Terminal.ModalToken, "env:")
		if varName == "" {
			return fmt.Errorf("config: terminal.modal_token has empty env variable reference")
		}
		cfg.Terminal.ModalToken = os.Getenv(varName)
	}
	if strings.HasPrefix(cfg.Terminal.DaytonaToken, "env:") {
		varName := strings.TrimPrefix(cfg.Terminal.DaytonaToken, "env:")
		if varName == "" {
			return fmt.Errorf("config: terminal.daytona_token has empty env variable reference")
		}
		cfg.Terminal.DaytonaToken = os.Getenv(varName)
	}
	return nil
}
```

- [ ] **Step 4: Run config tests**

```bash
cd /Users/ranwei/workspace/go_work/hermes-agent-rewrite/hermes-agent-go
go test -race ./config/...
```

Expected: PASS. All previous config tests + the new TerminalConfig test.

- [ ] **Step 5: Commit**

```bash
cd /Users/ranwei/workspace/go_work/hermes-agent-rewrite
git add hermes-agent-go/config/config.go hermes-agent-go/config/loader.go hermes-agent-go/config/loader_test.go
git commit -m "feat(config): add TerminalConfig with per-backend settings"
```

---

## Task 2: SSH Backend

**Files:**
- Create: `hermes-agent-go/tool/terminal/ssh.go`
- Create: `hermes-agent-go/tool/terminal/ssh_test.go`

SSH uses `golang.org/x/crypto/ssh` for a pure-Go SSH client. Connection is opened per Execute call (Plan 6 adds connection pooling).

- [ ] **Step 1: Add the crypto/ssh dependency**

```bash
cd /Users/ranwei/workspace/go_work/hermes-agent-rewrite/hermes-agent-go
go get golang.org/x/crypto/ssh
```

- [ ] **Step 2: Write the SSH backend**

```go
// tool/terminal/ssh.go
package terminal

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

// SSH is a remote-exec backend that opens a fresh SSH connection per call.
// Connection pooling and persistent shells are deferred to Plan 6.
type SSH struct {
	host    string
	port    int
	user    string
	keyPath string
}

// NewSSH constructs an SSH backend from config.
func NewSSH(cfg Config) (*SSH, error) {
	if cfg.SSHHost == "" {
		return nil, errors.New("ssh: ssh_host is required")
	}
	if cfg.SSHUser == "" {
		return nil, errors.New("ssh: ssh_user is required")
	}
	if cfg.SSHKey == "" {
		return nil, errors.New("ssh: ssh_key is required")
	}

	// Allow "host:port" in SSHHost
	host := cfg.SSHHost
	port := 22
	if idx := strings.LastIndex(host, ":"); idx != -1 {
		if p, err := strconv.Atoi(host[idx+1:]); err == nil {
			port = p
			host = host[:idx]
		}
	}

	return &SSH{
		host:    host,
		port:    port,
		user:    cfg.SSHUser,
		keyPath: cfg.SSHKey,
	}, nil
}

// Execute opens a fresh SSH session and runs the command.
// The command is passed through /bin/sh -c on the remote side.
func (s *SSH) Execute(ctx context.Context, command string, opts *ExecOptions) (*ExecResult, error) {
	if opts == nil {
		opts = &ExecOptions{}
	}

	// Load the private key
	keyData, err := os.ReadFile(s.keyPath)
	if err != nil {
		return nil, fmt.Errorf("ssh: read key %s: %w", s.keyPath, err)
	}
	signer, err := ssh.ParsePrivateKey(keyData)
	if err != nil {
		return nil, fmt.Errorf("ssh: parse key: %w", err)
	}

	clientConfig := &ssh.ClientConfig{
		User: s.user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		// Plan 5 uses InsecureIgnoreHostKey for simplicity. Plan 6 should
		// add known_hosts verification via ssh.FixedHostKey or ssh.KnownHosts.
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	addr := net.JoinHostPort(s.host, strconv.Itoa(s.port))
	client, err := ssh.Dial("tcp", addr, clientConfig)
	if err != nil {
		return nil, fmt.Errorf("ssh: dial %s: %w", addr, err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("ssh: new session: %w", err)
	}
	defer session.Close()

	// Build the wrapped command: cd + env + sh -c
	wrapped := buildWrappedCommand(command, opts)

	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	if opts.Stdin != "" {
		session.Stdin = strings.NewReader(opts.Stdin)
	}

	// Apply timeout via context cancellation.
	// crypto/ssh does not honor ctx directly, so we close the session on timeout.
	var runErr error
	done := make(chan struct{})
	go func() {
		runErr = session.Run(wrapped)
		close(done)
	}()

	runCtx := ctx
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		runCtx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}

	start := time.Now()
	select {
	case <-done:
	case <-runCtx.Done():
		_ = session.Signal(ssh.SIGKILL)
		<-done // wait for the goroutine to finish
		return &ExecResult{
			Stdout:   stdout.String(),
			Stderr:   stderr.String() + "\n[timeout]",
			ExitCode: -1,
			Duration: time.Since(start),
		}, nil
	}

	exitCode := 0
	if runErr != nil {
		var exitErr *ssh.ExitError
		if errors.As(runErr, &exitErr) {
			exitCode = exitErr.ExitStatus()
		} else {
			exitCode = -1
			if stderr.Len() == 0 {
				stderr.WriteString(runErr.Error())
			}
		}
	}

	return &ExecResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
		Duration: time.Since(start),
	}, nil
}

// SupportsPersistentShell returns false — Plan 5 opens a fresh connection per call.
func (s *SSH) SupportsPersistentShell() bool { return false }

// Close is a no-op for the stateless SSH backend.
func (s *SSH) Close() error { return nil }

// buildWrappedCommand prepends `cd <dir>` and `export KEY=VAL` statements
// to the command, then wraps the whole thing in `/bin/sh -c '...'`.
// The result is suitable for passing as a single string to a remote shell.
func buildWrappedCommand(command string, opts *ExecOptions) string {
	var b strings.Builder
	if opts.Cwd != "" {
		fmt.Fprintf(&b, "cd %s && ", shellQuote(opts.Cwd))
	}
	for k, v := range opts.Env {
		fmt.Fprintf(&b, "export %s=%s && ", k, shellQuote(v))
	}
	b.WriteString(command)
	return b.String()
}

// shellQuote wraps a value in single quotes, escaping any internal single quotes.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}
```

- [ ] **Step 3: Write the SSH tests**

SSH tests use an in-process SSH server (via `golang.org/x/crypto/ssh` server-side APIs). This is complex enough that we'll test the building-block functions directly and smoke-test the NewSSH factory.

```go
// tool/terminal/ssh_test.go
package terminal

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSSHRequiresHost(t *testing.T) {
	_, err := NewSSH(Config{SSHUser: "u", SSHKey: "/tmp/k"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ssh_host")
}

func TestNewSSHRequiresUser(t *testing.T) {
	_, err := NewSSH(Config{SSHHost: "h", SSHKey: "/tmp/k"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ssh_user")
}

func TestNewSSHRequiresKey(t *testing.T) {
	_, err := NewSSH(Config{SSHHost: "h", SSHUser: "u"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ssh_key")
}

func TestNewSSHParsesHostPort(t *testing.T) {
	s, err := NewSSH(Config{SSHHost: "h:2222", SSHUser: "u", SSHKey: "/tmp/k"})
	require.NoError(t, err)
	assert.Equal(t, "h", s.host)
	assert.Equal(t, 2222, s.port)
}

func TestNewSSHDefaultPort(t *testing.T) {
	s, err := NewSSH(Config{SSHHost: "h", SSHUser: "u", SSHKey: "/tmp/k"})
	require.NoError(t, err)
	assert.Equal(t, 22, s.port)
}

func TestSSHSupportsPersistentShellIsFalse(t *testing.T) {
	s, err := NewSSH(Config{SSHHost: "h", SSHUser: "u", SSHKey: "/tmp/k"})
	require.NoError(t, err)
	assert.False(t, s.SupportsPersistentShell())
	assert.NoError(t, s.Close())
}

func TestBuildWrappedCommandSimple(t *testing.T) {
	got := buildWrappedCommand("echo hi", &ExecOptions{})
	assert.Equal(t, "echo hi", got)
}

func TestBuildWrappedCommandWithCwd(t *testing.T) {
	got := buildWrappedCommand("echo hi", &ExecOptions{Cwd: "/tmp"})
	assert.Equal(t, "cd '/tmp' && echo hi", got)
}

func TestBuildWrappedCommandWithEnv(t *testing.T) {
	got := buildWrappedCommand("echo $FOO", &ExecOptions{
		Env: map[string]string{"FOO": "bar"},
	})
	assert.Contains(t, got, "export FOO='bar'")
	assert.Contains(t, got, "echo $FOO")
}

func TestShellQuoteEscapesQuotes(t *testing.T) {
	assert.Equal(t, `'hello'`, shellQuote("hello"))
	assert.Equal(t, `'it'"'"'s'`, shellQuote("it's"))
}
```

- [ ] **Step 4: Run tests**

```bash
go test -race ./tool/terminal/...
```

Expected: PASS. All previous terminal tests + 10 new SSH tests.

- [ ] **Step 5: Commit**

```bash
git add hermes-agent-go/tool/terminal/ssh.go hermes-agent-go/tool/terminal/ssh_test.go hermes-agent-go/go.mod hermes-agent-go/go.sum
git commit -m "feat(tool/terminal): add SSH backend via crypto/ssh"
```

---

## Task 3: Docker Backend (CLI Subprocess)

**Files:**
- Create: `hermes-agent-go/tool/terminal/docker.go`
- Create: `hermes-agent-go/tool/terminal/docker_test.go`

We wrap the `docker` CLI as a subprocess rather than using the heavy `docker/docker/client` SDK. Each call runs `docker run --rm -i <image> sh -c '<command>'`. Volumes, working directory, environment, and stdin are passed via `docker run` flags.

- [ ] **Step 1: Write the Docker backend**

```go
// tool/terminal/docker.go
package terminal

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Docker wraps the `docker` CLI to run commands in ephemeral containers.
// Each Execute call spawns a fresh `docker run --rm` subprocess.
type Docker struct {
	image   string
	volumes []string
}

// NewDocker constructs a Docker backend from config. The docker_image field
// is required. Optional docker_volumes are mounted into every container.
func NewDocker(cfg Config) (*Docker, error) {
	if cfg.DockerImage == "" {
		return nil, errors.New("docker: docker_image is required")
	}
	// Sanity check: docker CLI available
	if _, err := exec.LookPath("docker"); err != nil {
		return nil, fmt.Errorf("docker: `docker` CLI not found in PATH: %w", err)
	}
	return &Docker{
		image:   cfg.DockerImage,
		volumes: append([]string{}, cfg.DockerVolumes...),
	}, nil
}

// Execute runs a command inside an ephemeral container.
func (d *Docker) Execute(ctx context.Context, command string, opts *ExecOptions) (*ExecResult, error) {
	if opts == nil {
		opts = &ExecOptions{}
	}

	runCtx := ctx
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		runCtx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}

	args := d.buildArgs(opts, command)
	cmd := exec.CommandContext(runCtx, "docker", args...)

	if opts.Stdin != "" {
		cmd.Stdin = strings.NewReader(opts.Stdin)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	runErr := cmd.Run()
	duration := time.Since(start)

	exitCode := 0
	if runErr != nil {
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
			if stderr.Len() == 0 {
				stderr.WriteString(runErr.Error())
			}
		}
	}

	return &ExecResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
		Duration: duration,
	}, nil
}

// buildArgs constructs the `docker run` argument list.
func (d *Docker) buildArgs(opts *ExecOptions, command string) []string {
	args := []string{"run", "--rm", "-i"}

	// Working directory inside the container
	if opts.Cwd != "" {
		args = append(args, "--workdir", opts.Cwd)
	}

	// Volume mounts from config
	for _, v := range d.volumes {
		args = append(args, "--volume", v)
	}

	// Environment variables
	for k, v := range opts.Env {
		args = append(args, "--env", fmt.Sprintf("%s=%s", k, v))
	}

	// Image + shell + command
	args = append(args, d.image, "sh", "-c", command)

	return args
}

// SupportsPersistentShell returns false — every call starts a new container.
func (d *Docker) SupportsPersistentShell() bool { return false }

// Close is a no-op for the stateless Docker backend.
func (d *Docker) Close() error { return nil }
```

- [ ] **Step 2: Write the Docker tests**

Docker tests exercise the argument-building logic directly; actual container execution requires a Docker daemon and is out of scope for unit tests.

```go
// tool/terminal/docker_test.go
package terminal

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDockerRequiresImage(t *testing.T) {
	_, err := NewDocker(Config{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "docker_image")
}

func TestDockerBuildArgsMinimal(t *testing.T) {
	d := &Docker{image: "alpine:3.19"}
	args := d.buildArgs(&ExecOptions{}, "echo hi")
	assert.Equal(t, []string{
		"run", "--rm", "-i",
		"alpine:3.19", "sh", "-c", "echo hi",
	}, args)
}

func TestDockerBuildArgsWithCwd(t *testing.T) {
	d := &Docker{image: "alpine:3.19"}
	args := d.buildArgs(&ExecOptions{Cwd: "/app"}, "pwd")
	assert.Equal(t, []string{
		"run", "--rm", "-i",
		"--workdir", "/app",
		"alpine:3.19", "sh", "-c", "pwd",
	}, args)
}

func TestDockerBuildArgsWithVolumes(t *testing.T) {
	d := &Docker{
		image:   "alpine:3.19",
		volumes: []string{"/host/src:/workspace", "/tmp:/tmp"},
	}
	args := d.buildArgs(&ExecOptions{}, "ls")
	assert.Contains(t, args, "--volume")
	assert.Contains(t, args, "/host/src:/workspace")
	assert.Contains(t, args, "/tmp:/tmp")
}

func TestDockerBuildArgsWithEnv(t *testing.T) {
	d := &Docker{image: "alpine:3.19"}
	args := d.buildArgs(&ExecOptions{Env: map[string]string{"FOO": "bar"}}, "printenv FOO")
	assert.Contains(t, args, "--env")
	assert.Contains(t, args, "FOO=bar")
}

func TestDockerSupportsPersistentShellIsFalse(t *testing.T) {
	d := &Docker{image: "alpine:3.19"}
	assert.False(t, d.SupportsPersistentShell())
	assert.NoError(t, d.Close())
}
```

**Note on `TestNewDockerRequiresImage`:** This test exercises the image-empty branch. `NewDocker` also calls `exec.LookPath("docker")` — if Docker isn't installed on the test host, that branch fails too. Since CI may or may not have docker, the test for the image-required branch uses `Config{}` which triggers the early image check before LookPath.

- [ ] **Step 3: Run tests**

```bash
go test -race ./tool/terminal/...
```

Expected: PASS. All previous terminal tests + 6 new Docker tests.

- [ ] **Step 4: Commit**

```bash
git add hermes-agent-go/tool/terminal/docker.go hermes-agent-go/tool/terminal/docker_test.go
git commit -m "feat(tool/terminal): add Docker backend wrapping docker CLI"
```

---

## Task 4: Singularity Backend

**Files:**
- Create: `hermes-agent-go/tool/terminal/singularity.go`
- Create: `hermes-agent-go/tool/terminal/singularity_test.go`

Singularity (a.k.a. Apptainer) is the HPC-centric container runtime. We wrap the `singularity` CLI the same way we wrap `docker`: subprocess per call. Images are `.sif` files on disk.

- [ ] **Step 1: Write the Singularity backend**

```go
// tool/terminal/singularity.go
package terminal

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Singularity wraps the `singularity` CLI to run commands inside .sif images.
// Typically used on HPC clusters where Docker is unavailable.
type Singularity struct {
	image string
}

// NewSingularity constructs a Singularity backend from config.
// The singularity_image field is required (path to a .sif file).
func NewSingularity(cfg Config) (*Singularity, error) {
	if cfg.SingularityImage == "" {
		return nil, errors.New("singularity: singularity_image is required")
	}
	if _, err := exec.LookPath("singularity"); err != nil {
		// Fall back to "apptainer" if available
		if _, err2 := exec.LookPath("apptainer"); err2 != nil {
			return nil, fmt.Errorf("singularity: neither `singularity` nor `apptainer` CLI found in PATH: %w", err)
		}
	}
	return &Singularity{image: cfg.SingularityImage}, nil
}

// Execute runs a command inside the Singularity image.
func (s *Singularity) Execute(ctx context.Context, command string, opts *ExecOptions) (*ExecResult, error) {
	if opts == nil {
		opts = &ExecOptions{}
	}

	runCtx := ctx
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		runCtx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}

	cliName := s.detectCLI()
	args := s.buildArgs(opts, command)
	cmd := exec.CommandContext(runCtx, cliName, args...)

	if opts.Stdin != "" {
		cmd.Stdin = strings.NewReader(opts.Stdin)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	runErr := cmd.Run()
	duration := time.Since(start)

	exitCode := 0
	if runErr != nil {
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
			if stderr.Len() == 0 {
				stderr.WriteString(runErr.Error())
			}
		}
	}

	return &ExecResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
		Duration: duration,
	}, nil
}

// detectCLI returns "singularity" if available, else "apptainer".
// Apptainer is the open-source fork of Singularity and uses identical CLI syntax.
func (s *Singularity) detectCLI() string {
	if _, err := exec.LookPath("singularity"); err == nil {
		return "singularity"
	}
	return "apptainer"
}

// buildArgs constructs the singularity exec argument list.
func (s *Singularity) buildArgs(opts *ExecOptions, command string) []string {
	args := []string{"exec"}

	// Working directory inside the container
	if opts.Cwd != "" {
		args = append(args, "--pwd", opts.Cwd)
	}

	// Environment variables
	for k, v := range opts.Env {
		args = append(args, "--env", fmt.Sprintf("%s=%s", k, v))
	}

	// Image + shell + command
	args = append(args, s.image, "sh", "-c", command)

	return args
}

// SupportsPersistentShell returns false.
func (s *Singularity) SupportsPersistentShell() bool { return false }

// Close is a no-op.
func (s *Singularity) Close() error { return nil }
```

- [ ] **Step 2: Write the tests**

```go
// tool/terminal/singularity_test.go
package terminal

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSingularityRequiresImage(t *testing.T) {
	_, err := NewSingularity(Config{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "singularity_image")
}

func TestSingularityBuildArgsMinimal(t *testing.T) {
	s := &Singularity{image: "/opt/ubuntu.sif"}
	args := s.buildArgs(&ExecOptions{}, "echo hi")
	assert.Equal(t, []string{
		"exec",
		"/opt/ubuntu.sif", "sh", "-c", "echo hi",
	}, args)
}

func TestSingularityBuildArgsWithCwd(t *testing.T) {
	s := &Singularity{image: "/opt/ubuntu.sif"}
	args := s.buildArgs(&ExecOptions{Cwd: "/workspace"}, "pwd")
	assert.Contains(t, args, "--pwd")
	assert.Contains(t, args, "/workspace")
}

func TestSingularityBuildArgsWithEnv(t *testing.T) {
	s := &Singularity{image: "/opt/ubuntu.sif"}
	args := s.buildArgs(&ExecOptions{Env: map[string]string{"FOO": "bar"}}, "printenv FOO")
	assert.Contains(t, args, "--env")
	assert.Contains(t, args, "FOO=bar")
}

func TestSingularitySupportsPersistentShellIsFalse(t *testing.T) {
	s := &Singularity{image: "/opt/ubuntu.sif"}
	assert.False(t, s.SupportsPersistentShell())
	assert.NoError(t, s.Close())
}
```

- [ ] **Step 3: Run tests**

```bash
go test -race ./tool/terminal/...
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add hermes-agent-go/tool/terminal/singularity.go hermes-agent-go/tool/terminal/singularity_test.go
git commit -m "feat(tool/terminal): add Singularity backend wrapping singularity/apptainer CLI"
```

---

## Task 5: Modal Backend (HTTP API)

**Files:**
- Create: `hermes-agent-go/tool/terminal/modal.go`
- Create: `hermes-agent-go/tool/terminal/modal_test.go`

Modal is a serverless Python platform. We implement the backend against an assumed HTTP API shape:

```
POST {base_url}/v1/exec
Headers: Authorization: Bearer <token>
Body: {"command": "...", "cwd": "...", "env": {...}, "stdin": "...", "timeout_seconds": N}
Response: {"stdout": "...", "stderr": "...", "exit_code": N, "duration_ms": N}
```

Users with real Modal accounts will need to adjust this to match the actual Modal API. The plan's design is that the HTTP shape is small enough to swap easily.

- [ ] **Step 1: Write the Modal backend**

```go
// tool/terminal/modal.go
package terminal

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

// Modal is a serverless backend that POSTs commands to a Modal HTTP API endpoint.
// The expected request/response shape is documented at the top of the file.
//
// Each Execute call makes one HTTP POST. No connection reuse, no state.
type Modal struct {
	baseURL string
	token   string
	http    *http.Client
}

// NewModal constructs a Modal backend from config.
func NewModal(cfg Config) (*Modal, error) {
	if cfg.ModalBaseURL == "" {
		return nil, errors.New("modal: modal_base_url is required")
	}
	if cfg.ModalToken == "" {
		return nil, errors.New("modal: modal_token is required")
	}
	return &Modal{
		baseURL: cfg.ModalBaseURL,
		token:   cfg.ModalToken,
		http:    &http.Client{Timeout: 600 * time.Second},
	}, nil
}

// modalExecRequest is the body sent to POST /v1/exec.
type modalExecRequest struct {
	Command        string            `json:"command"`
	Cwd            string            `json:"cwd,omitempty"`
	Env            map[string]string `json:"env,omitempty"`
	Stdin          string            `json:"stdin,omitempty"`
	TimeoutSeconds int               `json:"timeout_seconds,omitempty"`
}

// modalExecResponse is the body returned by POST /v1/exec.
type modalExecResponse struct {
	Stdout     string `json:"stdout"`
	Stderr     string `json:"stderr"`
	ExitCode   int    `json:"exit_code"`
	DurationMS int64  `json:"duration_ms"`
}

// Execute sends a command to the Modal API.
func (m *Modal) Execute(ctx context.Context, command string, opts *ExecOptions) (*ExecResult, error) {
	if opts == nil {
		opts = &ExecOptions{}
	}

	reqBody := modalExecRequest{
		Command: command,
		Cwd:     opts.Cwd,
		Env:     opts.Env,
		Stdin:   opts.Stdin,
	}
	if opts.Timeout > 0 {
		reqBody.TimeoutSeconds = int(opts.Timeout.Seconds())
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("modal: marshal: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", m.baseURL+"/v1/exec", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("modal: new request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+m.token)

	start := time.Now()
	resp, err := m.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("modal: network: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("modal: http %d", resp.StatusCode)
	}

	var out modalExecResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("modal: decode: %w", err)
	}

	duration := time.Duration(out.DurationMS) * time.Millisecond
	if duration == 0 {
		duration = time.Since(start)
	}
	return &ExecResult{
		Stdout:   out.Stdout,
		Stderr:   out.Stderr,
		ExitCode: out.ExitCode,
		Duration: duration,
	}, nil
}

// SupportsPersistentShell returns false.
func (m *Modal) SupportsPersistentShell() bool { return false }

// Close is a no-op.
func (m *Modal) Close() error { return nil }
```

- [ ] **Step 2: Write the Modal tests**

```go
// tool/terminal/modal_test.go
package terminal

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewModalRequiresBaseURL(t *testing.T) {
	_, err := NewModal(Config{ModalToken: "t"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "modal_base_url")
}

func TestNewModalRequiresToken(t *testing.T) {
	_, err := NewModal(Config{ModalBaseURL: "https://x"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "modal_token")
}

func TestModalExecuteHappyPath(t *testing.T) {
	var captured modalExecRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/exec", r.URL.Path)
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		body, _ := io.ReadAll(r.Body)
		require.NoError(t, json.Unmarshal(body, &captured))

		resp := modalExecResponse{
			Stdout:     "hello\n",
			Stderr:     "",
			ExitCode:   0,
			DurationMS: 42,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	m, err := NewModal(Config{ModalBaseURL: srv.URL, ModalToken: "test-token"})
	require.NoError(t, err)

	result, err := m.Execute(context.Background(), "echo hello", &ExecOptions{
		Cwd:     "/workspace",
		Env:     map[string]string{"FOO": "bar"},
		Timeout: 30 * time.Second,
	})
	require.NoError(t, err)
	assert.Equal(t, "hello\n", result.Stdout)
	assert.Equal(t, 0, result.ExitCode)
	assert.Equal(t, "echo hello", captured.Command)
	assert.Equal(t, "/workspace", captured.Cwd)
	assert.Equal(t, "bar", captured.Env["FOO"])
	assert.Equal(t, 30, captured.TimeoutSeconds)
}

func TestModalExecuteHandlesNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	m, _ := NewModal(Config{ModalBaseURL: srv.URL, ModalToken: "t"})
	_, err := m.Execute(context.Background(), "echo hi", &ExecOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "http 500")
}

func TestModalSupportsPersistentShellIsFalse(t *testing.T) {
	m, _ := NewModal(Config{ModalBaseURL: "https://x", ModalToken: "t"})
	assert.False(t, m.SupportsPersistentShell())
	assert.NoError(t, m.Close())
}
```

- [ ] **Step 3: Run tests**

```bash
go test -race ./tool/terminal/...
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add hermes-agent-go/tool/terminal/modal.go hermes-agent-go/tool/terminal/modal_test.go
git commit -m "feat(tool/terminal): add Modal HTTP API backend"
```

---

## Task 6: Daytona Backend (HTTP API)

**Files:**
- Create: `hermes-agent-go/tool/terminal/daytona.go`
- Create: `hermes-agent-go/tool/terminal/daytona_test.go`

Daytona provides persistent dev environments via HTTP API. Assumed shape:

```
POST {base_url}/v1/workspace/exec
Headers: Authorization: Bearer <token>
Body: {"command": "...", "cwd": "...", "env": {...}, "stdin": "...", "timeout_seconds": N}
Response: {"stdout": "...", "stderr": "...", "exit_code": N, "duration_ms": N}
```

- [ ] **Step 1: Write the Daytona backend**

```go
// tool/terminal/daytona.go
package terminal

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

// Daytona is a persistent dev-environment backend using the Daytona HTTP API.
// Expected request/response shape is the same as Modal's /v1/exec endpoint.
//
// Unlike Modal, Daytona workspaces persist between calls — but Plan 5 still
// treats the backend as stateless (each call hits the same workspace).
// Plan 6 could add workspace selection/creation.
type Daytona struct {
	baseURL string
	token   string
	http    *http.Client
}

// NewDaytona constructs a Daytona backend.
func NewDaytona(cfg Config) (*Daytona, error) {
	if cfg.DaytonaBaseURL == "" {
		return nil, errors.New("daytona: daytona_base_url is required")
	}
	if cfg.DaytonaToken == "" {
		return nil, errors.New("daytona: daytona_token is required")
	}
	return &Daytona{
		baseURL: cfg.DaytonaBaseURL,
		token:   cfg.DaytonaToken,
		http:    &http.Client{Timeout: 600 * time.Second},
	}, nil
}

// daytonaExecRequest matches the HTTP wire format.
type daytonaExecRequest struct {
	Command        string            `json:"command"`
	Cwd            string            `json:"cwd,omitempty"`
	Env            map[string]string `json:"env,omitempty"`
	Stdin          string            `json:"stdin,omitempty"`
	TimeoutSeconds int               `json:"timeout_seconds,omitempty"`
}

// daytonaExecResponse is the response body shape.
type daytonaExecResponse struct {
	Stdout     string `json:"stdout"`
	Stderr     string `json:"stderr"`
	ExitCode   int    `json:"exit_code"`
	DurationMS int64  `json:"duration_ms"`
}

// Execute POSTs the command to the Daytona workspace exec endpoint.
func (d *Daytona) Execute(ctx context.Context, command string, opts *ExecOptions) (*ExecResult, error) {
	if opts == nil {
		opts = &ExecOptions{}
	}

	reqBody := daytonaExecRequest{
		Command: command,
		Cwd:     opts.Cwd,
		Env:     opts.Env,
		Stdin:   opts.Stdin,
	}
	if opts.Timeout > 0 {
		reqBody.TimeoutSeconds = int(opts.Timeout.Seconds())
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("daytona: marshal: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", d.baseURL+"/v1/workspace/exec", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("daytona: new request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+d.token)

	start := time.Now()
	resp, err := d.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("daytona: network: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("daytona: http %d", resp.StatusCode)
	}

	var out daytonaExecResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("daytona: decode: %w", err)
	}

	duration := time.Duration(out.DurationMS) * time.Millisecond
	if duration == 0 {
		duration = time.Since(start)
	}
	return &ExecResult{
		Stdout:   out.Stdout,
		Stderr:   out.Stderr,
		ExitCode: out.ExitCode,
		Duration: duration,
	}, nil
}

// SupportsPersistentShell returns false.
// (The workspace is persistent but individual exec calls don't share a shell process.)
func (d *Daytona) SupportsPersistentShell() bool { return false }

// Close is a no-op.
func (d *Daytona) Close() error { return nil }
```

- [ ] **Step 2: Write the Daytona tests**

```go
// tool/terminal/daytona_test.go
package terminal

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDaytonaRequiresBaseURL(t *testing.T) {
	_, err := NewDaytona(Config{DaytonaToken: "t"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "daytona_base_url")
}

func TestNewDaytonaRequiresToken(t *testing.T) {
	_, err := NewDaytona(Config{DaytonaBaseURL: "https://x"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "daytona_token")
}

func TestDaytonaExecuteHappyPath(t *testing.T) {
	var captured daytonaExecRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/workspace/exec", r.URL.Path)
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		body, _ := io.ReadAll(r.Body)
		require.NoError(t, json.Unmarshal(body, &captured))

		resp := daytonaExecResponse{
			Stdout:     "world\n",
			Stderr:     "",
			ExitCode:   0,
			DurationMS: 55,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	d, err := NewDaytona(Config{DaytonaBaseURL: srv.URL, DaytonaToken: "test-token"})
	require.NoError(t, err)

	result, err := d.Execute(context.Background(), "echo world", &ExecOptions{})
	require.NoError(t, err)
	assert.Equal(t, "world\n", result.Stdout)
	assert.Equal(t, 0, result.ExitCode)
	assert.Equal(t, "echo world", captured.Command)
}

func TestDaytonaExecuteHandlesNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	d, _ := NewDaytona(Config{DaytonaBaseURL: srv.URL, DaytonaToken: "t"})
	_, err := d.Execute(context.Background(), "echo hi", &ExecOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "http 502")
}

func TestDaytonaSupportsPersistentShellIsFalse(t *testing.T) {
	d, _ := NewDaytona(Config{DaytonaBaseURL: "https://x", DaytonaToken: "t"})
	assert.False(t, d.SupportsPersistentShell())
	assert.NoError(t, d.Close())
}
```

- [ ] **Step 3: Run tests**

```bash
go test -race ./tool/terminal/...
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add hermes-agent-go/tool/terminal/daytona.go hermes-agent-go/tool/terminal/daytona_test.go
git commit -m "feat(tool/terminal): add Daytona HTTP API backend"
```

---

## Task 7: Factory Update for All 6 Backends

**Files:**
- Modify: `hermes-agent-go/tool/terminal/terminal.go`

- [ ] **Step 1: Update the `New` factory**

Find the existing `New` function in `tool/terminal/terminal.go` and replace its body:

```go
// New constructs a backend by name. Returns a helpful error for unknown types.
func New(backendType string, cfg Config) (Backend, error) {
	switch backendType {
	case "local", "":
		return NewLocal(cfg)
	case "docker":
		return NewDocker(cfg)
	case "ssh":
		return NewSSH(cfg)
	case "singularity", "apptainer":
		return NewSingularity(cfg)
	case "modal":
		return NewModal(cfg)
	case "daytona":
		return NewDaytona(cfg)
	default:
		return nil, fmt.Errorf("terminal: backend %q is not supported", backendType)
	}
}
```

Ensure `fmt` is imported in `terminal.go`.

- [ ] **Step 2: Add a factory test**

Append to `hermes-agent-go/tool/terminal/local_test.go` (or create `factory_test.go` if preferred):

```go
func TestNewFactoryDispatchesLocal(t *testing.T) {
	b, err := New("local", Config{})
	require.NoError(t, err)
	_, ok := b.(*Local)
	assert.True(t, ok)
}

func TestNewFactoryDispatchesSSH(t *testing.T) {
	b, err := New("ssh", Config{SSHHost: "h", SSHUser: "u", SSHKey: "/tmp/k"})
	require.NoError(t, err)
	_, ok := b.(*SSH)
	assert.True(t, ok)
}

func TestNewFactoryDispatchesModal(t *testing.T) {
	b, err := New("modal", Config{ModalBaseURL: "https://x", ModalToken: "t"})
	require.NoError(t, err)
	_, ok := b.(*Modal)
	assert.True(t, ok)
}

func TestNewFactoryDispatchesDaytona(t *testing.T) {
	b, err := New("daytona", Config{DaytonaBaseURL: "https://x", DaytonaToken: "t"})
	require.NoError(t, err)
	_, ok := b.(*Daytona)
	assert.True(t, ok)
}

func TestNewFactoryUnknown(t *testing.T) {
	_, err := New("made-up-backend", Config{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not supported")
}
```

Note: Docker and Singularity factory tests depend on `docker`/`singularity` being on PATH. Skip those in unit tests.

- [ ] **Step 3: Run tests**

```bash
cd /Users/ranwei/workspace/go_work/hermes-agent-rewrite/hermes-agent-go
go test -race ./tool/terminal/...
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add hermes-agent-go/tool/terminal/terminal.go hermes-agent-go/tool/terminal/local_test.go
git commit -m "feat(tool/terminal): expand factory to dispatch all 6 backends"
```

---

## Task 8: CLI Uses Configured Backend

**Files:**
- Modify: `hermes-agent-go/cli/repl.go`

- [ ] **Step 1: Update the terminal backend construction in runREPL**

Find the block in `cli/repl.go` that currently does:

```go
localBackend, err := terminal.NewLocal(terminal.Config{})
if err != nil {
    return fmt.Errorf("hermes: create terminal backend: %w", err)
}
defer localBackend.Close()
terminal.RegisterShellExecute(toolRegistry, localBackend)
```

Replace it with:

```go
termCfg := terminal.Config{
	Cwd:              app.Config.Terminal.Cwd,
	DockerImage:      app.Config.Terminal.DockerImage,
	DockerVolumes:    app.Config.Terminal.DockerVolumes,
	SSHHost:          app.Config.Terminal.SSHHost,
	SSHUser:          app.Config.Terminal.SSHUser,
	SSHKey:           app.Config.Terminal.SSHKey,
	SingularityImage: app.Config.Terminal.SingularityImage,
	ModalBaseURL:     app.Config.Terminal.ModalBaseURL,
	ModalToken:       app.Config.Terminal.ModalToken,
	DaytonaBaseURL:   app.Config.Terminal.DaytonaBaseURL,
	DaytonaToken:     app.Config.Terminal.DaytonaToken,
}
if app.Config.Terminal.Timeout > 0 {
	termCfg.Timeout = time.Duration(app.Config.Terminal.Timeout) * time.Second
}

backend, err := terminal.New(app.Config.Terminal.Backend, termCfg)
if err != nil {
	return fmt.Errorf("hermes: create terminal backend %q: %w", app.Config.Terminal.Backend, err)
}
defer backend.Close()
terminal.RegisterShellExecute(toolRegistry, backend)
```

Add `"time"` to the imports of `cli/repl.go` if it's not already there.

- [ ] **Step 2: Verify the `terminal.Config` type has all the fields we reference**

Read `hermes-agent-go/tool/terminal/terminal.go` and confirm the `Config` struct contains: `Cwd`, `DockerImage`, `DockerVolumes`, `SSHHost`, `SSHUser`, `SSHKey`, `SingularityImage`, `ModalBaseURL`, `ModalToken`, `DaytonaBaseURL`, `DaytonaToken`, `Timeout`. The Plan 2 version already had most of these (Docker, SSH, Timeout). Add any missing fields (ModalBaseURL, ModalToken, DaytonaBaseURL, DaytonaToken, SingularityImage) to the `Config` struct if they're not there.

If the existing `terminal.Config` in `terminal.go` is missing fields, update it:

```go
// Config is the shared configuration for terminal backend factories.
// Only the fields relevant to a given backend are read.
type Config struct {
	Cwd     string
	Timeout time.Duration

	// Docker
	DockerImage   string
	DockerVolumes []string

	// SSH
	SSHHost string
	SSHUser string
	SSHKey  string

	// Singularity
	SingularityImage string

	// Modal
	ModalBaseURL string
	ModalToken   string

	// Daytona
	DaytonaBaseURL string
	DaytonaToken   string

	// Legacy / unused in Plan 5 (kept for interface stability)
	PersistentShell bool
}
```

- [ ] **Step 3: Run the full test suite**

```bash
cd /Users/ranwei/workspace/go_work/hermes-agent-rewrite/hermes-agent-go
go test -race ./...
```

Expected: PASS. All existing tests + new terminal tests. The `TestEndToEndToolLoop` from Plan 2 still passes because it uses a real `terminal.NewLocal` directly.

- [ ] **Step 4: Build**

```bash
go build ./...
make build
./bin/hermes version
```

- [ ] **Step 5: Commit**

```bash
cd /Users/ranwei/workspace/go_work/hermes-agent-rewrite
git add hermes-agent-go/cli/repl.go hermes-agent-go/tool/terminal/terminal.go
git commit -m "feat(cli): use config.Terminal.Backend to select exec backend"
```

---

## Task 9: Final Verification

No commit. Run and report:

- [ ] **Step 1: Full test suite with coverage**

```bash
cd /Users/ranwei/workspace/go_work/hermes-agent-rewrite/hermes-agent-go
go test -race -cover ./...
```

Expected: ALL packages pass. `tool/terminal` coverage should rise significantly (new tests for SSH, Docker, Singularity, Modal, Daytona).

- [ ] **Step 2: go vet**

```bash
go vet ./...
```

Expected: clean.

- [ ] **Step 3: Build binary**

```bash
make build
./bin/hermes version
```

Expected: binary builds and prints version.

- [ ] **Step 4: Verify git log**

```bash
cd /Users/ranwei/workspace/go_work/hermes-agent-rewrite
git log --oneline hermes-agent-go/ | head -12
```

Expected: ~8 new commits from Plan 5.

- [ ] **Step 5: Plan 5 done. Proceed to Plan 6 (MCP + Memory + Context Compression) or Plan 7 (Gateway).**

---

## Plan 5 Self-Review Notes

**Spec coverage:**
- Docker terminal backend via CLI subprocess — Task 3
- SSH terminal backend via crypto/ssh — Task 2
- Singularity terminal backend via CLI subprocess — Task 4
- Modal HTTP API backend — Task 5
- Daytona HTTP API backend — Task 6
- TerminalConfig expansion — Task 1
- Factory dispatches all 6 backends — Task 7
- CLI uses configured backend — Task 8
- Final verification — Task 9

**Explicitly out of scope for Plan 5:**
- Persistent shell state — Plan 6
- SSH connection pooling — Plan 6
- Docker volume hot-mounting per command — Plan 6
- Known_hosts verification — Plan 6 (currently uses InsecureIgnoreHostKey)
- Modal/Daytona API shape validation against real accounts — users adjust
- Integration tests with real Docker/SSH/Modal/Daytona — unit tests only
- Browser automation, web tools, code execution, vision, delegate, MCP — Plan 5b or later

**Placeholder check:** None. All code blocks are complete and executable.

**Type consistency:**
- `terminal.Config` expanded in Task 8 with Modal/Daytona/Singularity fields — used by Tasks 5, 6, 4
- `config.TerminalConfig` defined in Task 1 — used by Task 8
- `NewSSH`, `NewDocker`, `NewSingularity`, `NewModal`, `NewDaytona` constructors — defined in Tasks 2-6, used by Task 7
- `SSH`, `Docker`, `Singularity`, `Modal`, `Daytona` structs — defined in Tasks 2-6

**Known assumptions (call-outs for users):**
1. **Modal HTTP API shape** is assumed based on typical REST conventions. Real Modal uses a different API (Modal Python SDK wraps gRPC). Users with real Modal accounts may need to customize the `Execute` method to match the actual endpoint. The plan's architecture keeps this isolated to `tool/terminal/modal.go`.

2. **Daytona HTTP API shape** is assumed similarly. Same disclaimer applies.

3. **Singularity vs Apptainer** — both CLIs have identical syntax. The backend picks whichever is available on PATH. If neither is installed, factory returns a clear error.

4. **InsecureIgnoreHostKey** is used for SSH host verification in Plan 5 for ease of first-time setup. A production deployment should read `~/.ssh/known_hosts` or pin a known host key. Plan 6 should fix this.

5. **Docker subprocess vs SDK** — the Plan 5 implementation shells out to `docker` CLI rather than using `docker/docker/client`. This keeps the dependency tree small (~10MB less) and matches how most users invoke Docker anyway.
