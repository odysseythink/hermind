package terminal

import (
	"context"
	"fmt"
	"time"
)

// Backend executes shell commands. Implementations may run locally, in a
// Docker container, over SSH, or via a serverless runtime.
// Implementations must be safe for concurrent use unless documented otherwise.
type Backend interface {
	// Execute runs a command and returns its result.
	Execute(ctx context.Context, command string, opts *ExecOptions) (*ExecResult, error)
	// SupportsPersistentShell reports whether the backend maintains state
	// (cwd, env vars) across Execute calls in the same Backend instance.
	SupportsPersistentShell() bool
	// Close releases any resources held by the backend.
	Close() error
}

// ExecOptions control a single Execute invocation.
type ExecOptions struct {
	Cwd     string
	Env     map[string]string
	Timeout time.Duration
	Stdin   string
}

// ExecResult holds the outcome of Execute.
type ExecResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
	Duration time.Duration
}

// Config is the shared configuration for terminal backend factories.
// Only the fields relevant to a given backend are read.
type Config struct {
	Cwd             string
	DockerImage     string
	DockerVolumes   []string
	SSHHost         string
	SSHUser         string
	SSHKey          string
	PersistentShell bool // hint only — not all backends support it
	Timeout         time.Duration

	// Modal backend
	ModalBaseURL string
	ModalToken   string

	// Daytona backend
	DaytonaBaseURL string
	DaytonaToken   string

	// Singularity backend
	SingularityImage string // path to .sif file
}

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
