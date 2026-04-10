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
