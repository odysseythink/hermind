package terminal

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// Local executes commands on the host OS via /bin/sh -c (or cmd /C on Windows).
// Stateless — no persistent shell support in this plan (Plan 5 will add one).
type Local struct {
	defaultCwd string
}

// NewLocal constructs a Local backend.
func NewLocal(cfg Config) (*Local, error) {
	cwd := cfg.Cwd
	if cwd == "" {
		if wd, err := os.Getwd(); err == nil {
			cwd = wd
		}
	}
	return &Local{defaultCwd: cwd}, nil
}

// Execute runs command via the system shell.
func (l *Local) Execute(ctx context.Context, command string, opts *ExecOptions) (*ExecResult, error) {
	if opts == nil {
		opts = &ExecOptions{}
	}

	// Apply timeout via context.WithTimeout if non-zero
	runCtx := ctx
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		runCtx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}

	shell, shellFlag := defaultShell()
	cmd := exec.CommandContext(runCtx, shell, shellFlag, command)

	// Working directory
	if opts.Cwd != "" {
		cmd.Dir = opts.Cwd
	} else {
		cmd.Dir = l.defaultCwd
	}

	// Environment: inherit host env, then apply overrides
	if len(opts.Env) > 0 {
		cmd.Env = append(os.Environ(), envToSlice(opts.Env)...)
	}

	// Stdin
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
			// Non-exit-code errors (process couldn't start, timeout, etc.)
			// Report as a non-zero exit with the error in stderr.
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

// SupportsPersistentShell always returns false for the stateless local backend.
func (l *Local) SupportsPersistentShell() bool { return false }

// Close is a no-op for the local backend.
func (l *Local) Close() error { return nil }

// defaultShell returns the shell and its "execute this string" flag for the current OS.
func defaultShell() (shell, flag string) {
	if runtime.GOOS == "windows" {
		return "cmd", "/C"
	}
	return "/bin/sh", "-c"
}

// envToSlice converts a map to os.Environ()-style "K=V" slice.
func envToSlice(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k, v := range m {
		out = append(out, fmt.Sprintf("%s=%s", k, v))
	}
	return out
}
