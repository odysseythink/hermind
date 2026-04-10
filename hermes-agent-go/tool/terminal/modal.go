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
