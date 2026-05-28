package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sync"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
)

type sseTransport struct {
	client      *client.Client
	startCancel context.CancelFunc
	closeOnce   sync.Once
	closed      chan struct{}
}

func newSSETransport(srv *ServerConfig) (Transport, error) {
	u, err := url.Parse(srv.URL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("%w: invalid URL %q", ErrInvalidServerType, srv.URL)
	}

	opts := make([]transport.ClientOption, 0)
	if len(srv.Headers) > 0 {
		opts = append(opts, transport.WithHeaders(srv.Headers))
	}

	c, err := client.NewSSEMCPClient(srv.URL, opts...)
	if err != nil {
		return nil, fmt.Errorf("sse transport: create client: %w", err)
	}

	return &sseTransport{
		client: c,
		closed: make(chan struct{}),
	}, nil
}

func (t *sseTransport) Connect(ctx context.Context) error {
	// Use a background context for Start so the persistent SSE stream
	// outlives the connect-timeout context. Start runs in its own
	// goroutine so we can respect the caller's deadline without
	// binding the stream lifetime to it.
	startCtx, startCancel := context.WithCancel(context.Background())
	t.startCancel = startCancel

	startDone := make(chan error, 1)
	go func() {
		startDone <- t.client.Start(startCtx)
	}()

	select {
	case err := <-startDone:
		if err != nil {
			t.startCancel()
			t.startCancel = nil
			return fmt.Errorf("sse transport: start client: %w", err)
		}
	case <-ctx.Done():
		t.startCancel()
		t.startCancel = nil
		// Wait for the Start goroutine to finish before returning so
		// that a subsequent Close() doesn't race with SDK internals.
		<-startDone
		return fmt.Errorf("sse transport: start client: %w", ctx.Err())
	}

	initReq := mcp.InitializeRequest{
		Params: mcp.InitializeParams{
			ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
			ClientInfo:      mcp.Implementation{Name: "anythingllm", Version: "1.0.0"},
		},
	}
	if _, err := t.client.Initialize(ctx, initReq); err != nil {
		t.startCancel()
		t.startCancel = nil
		_ = t.client.Close()
		return fmt.Errorf("sse transport: initialize: %w", err)
	}

	return nil
}

func (t *sseTransport) Close() error {
	var err error
	t.closeOnce.Do(func() {
		close(t.closed)
		if t.startCancel != nil {
			t.startCancel()
		}
		err = t.client.Close()
	})
	return err
}

func (t *sseTransport) Ping(ctx context.Context) bool {
	select {
	case <-t.closed:
		return false
	default:
	}
	return t.client.Ping(ctx) == nil
}

func (t *sseTransport) ListTools(ctx context.Context) ([]ToolSchema, error) {
	raw, err := t.client.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		return nil, err
	}
	out := make([]ToolSchema, 0, len(raw.Tools))
	for _, r := range raw.Tools {
		schemaJSON, _ := json.Marshal(r.InputSchema)
		out = append(out, ToolSchema{
			Name:        r.Name,
			Description: r.Description,
			InputSchema: schemaJSON,
		})
	}
	return out, nil
}

func (t *sseTransport) CallTool(ctx context.Context, name string, args map[string]any) (any, error) {
	return t.client.CallTool(ctx, mcp.CallToolRequest{
		Request: mcp.Request{Method: "tools/call"},
		Params: mcp.CallToolParams{
			Name:      name,
			Arguments: args,
		},
	})
}

func (t *sseTransport) ProcessInfo() *ProcessInfo {
	return nil
}
