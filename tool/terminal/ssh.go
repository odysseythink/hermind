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
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //nolint:gosec
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
