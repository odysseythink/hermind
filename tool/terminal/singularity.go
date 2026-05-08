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
