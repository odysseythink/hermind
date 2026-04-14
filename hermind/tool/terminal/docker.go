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
