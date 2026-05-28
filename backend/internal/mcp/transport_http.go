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

type httpTransport struct {
	client    *client.Client
	closeOnce sync.Once
	closed    chan struct{}
}

func newHTTPTransport(srv *ServerConfig) (Transport, error) {
	u, err := url.Parse(srv.URL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("%w: invalid URL %q", ErrInvalidServerType, srv.URL)
	}

	opts := make([]transport.StreamableHTTPCOption, 0)
	if len(srv.Headers) > 0 {
		opts = append(opts, transport.WithHTTPHeaders(srv.Headers))
	}

	c, err := client.NewStreamableHttpClient(srv.URL, opts...)
	if err != nil {
		return nil, fmt.Errorf("http transport: create client: %w", err)
	}

	return &httpTransport{
		client: c,
		closed: make(chan struct{}),
	}, nil
}

func (t *httpTransport) Connect(ctx context.Context) error {
	if err := t.client.Start(ctx); err != nil {
		_ = t.client.Close()
		return fmt.Errorf("http transport: start client: %w", err)
	}

	initReq := mcp.InitializeRequest{
		Params: mcp.InitializeParams{
			ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
			ClientInfo:      mcp.Implementation{Name: "anythingllm", Version: "1.0.0"},
		},
	}
	if _, err := t.client.Initialize(ctx, initReq); err != nil {
		_ = t.client.Close()
		return fmt.Errorf("http transport: initialize: %w", err)
	}

	return nil
}

func (t *httpTransport) Close() error {
	var err error
	t.closeOnce.Do(func() {
		close(t.closed)
		err = t.client.Close()
	})
	return err
}

func (t *httpTransport) Ping(ctx context.Context) bool {
	select {
	case <-t.closed:
		return false
	default:
	}
	return t.client.Ping(ctx) == nil
}

func (t *httpTransport) ListTools(ctx context.Context) ([]ToolSchema, error) {
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

func (t *httpTransport) CallTool(ctx context.Context, name string, args map[string]any) (any, error) {
	return t.client.CallTool(ctx, mcp.CallToolRequest{
		Request: mcp.Request{Method: "tools/call"},
		Params: mcp.CallToolParams{
			Name:      name,
			Arguments: args,
		},
	})
}

func (t *httpTransport) ProcessInfo() *ProcessInfo {
	return nil
}
