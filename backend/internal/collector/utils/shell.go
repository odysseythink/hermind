package utils

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Default timeouts for external tool invocations.
const (
	DefaultTimeoutTesseract = 5 * time.Minute
	DefaultTimeoutPdftotext = 5 * time.Minute
	DefaultTimeoutPdftoppm  = 5 * time.Minute
	DefaultTimeoutFFmpeg    = 10 * time.Minute
	DefaultTimeoutWhisper   = 30 * time.Minute
	DefaultTimeoutChromedp  = 3 * time.Minute
)

// ShellRunner executes shell commands with context support.
type ShellRunner struct{}

// NewShellRunner creates a new ShellRunner.
func NewShellRunner() *ShellRunner { return &ShellRunner{} }

// Run executes a command with the given name and arguments.
func (s *ShellRunner) Run(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s failed: %w (output: %s)", name, err, string(out))
	}
	return strings.TrimSpace(string(out)), nil
}

// RunWithEnvAndTimeout executes a command with optional extra environment
// variables and an enforced timeout.
func (s *ShellRunner) RunWithEnvAndTimeout(ctx context.Context, timeout time.Duration, env []string, name string, args ...string) (string, error) {
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	cmd := exec.CommandContext(ctx, name, args...)
	if len(env) > 0 {
		cmd.Env = env
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s failed: %w (output: %s)", name, err, string(out))
	}
	return strings.TrimSpace(string(out)), nil
}

// RunWithTimeout executes a command with the given name and arguments,
// enforcing a default timeout if the provided context does not already have one.
func (s *ShellRunner) RunWithTimeout(ctx context.Context, timeout time.Duration, name string, args ...string) (string, error) {
	return s.RunWithEnvAndTimeout(ctx, timeout, nil, name, args...)
}

// CheckInstalled returns true if the named executable is found in PATH.
func (s *ShellRunner) CheckInstalled(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
