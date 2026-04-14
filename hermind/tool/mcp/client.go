// tool/mcp/client.go
package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"
)

// Client is a minimal MCP client. It owns a Transport and serializes
// requests through it. A reader goroutine delivers responses to per-call
// channels keyed by JSON-RPC ID.
//
// Not safe for concurrent Start/Close, but Call is safe for concurrent use.
type Client struct {
	transport Transport

	nextID  atomic.Int64
	pending sync.Map // map[int64]chan *jsonrpcResponse

	closeOnce sync.Once
	closed    chan struct{}

	// serverInfo is populated by Initialize
	serverName    string
	serverVersion string
}

// NewClient wraps a Transport into a Client.
func NewClient(t Transport) *Client {
	return &Client{
		transport: t,
		closed:    make(chan struct{}),
	}
}

// Start the transport and the reader loop.
func (c *Client) Start(ctx context.Context) error {
	if err := c.transport.Start(ctx); err != nil {
		return err
	}
	go c.readLoop()
	return nil
}

// Close shuts down the client and its transport.
func (c *Client) Close() error {
	c.closeOnce.Do(func() {
		close(c.closed)
		_ = c.transport.Close()
	})
	return nil
}

// Call sends a request and waits for the response.
// Returns the response Result or an error (timeout, transport failure, server error).
func (c *Client) Call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	id := c.nextID.Add(1)
	req, err := newRequest(id, method, params)
	if err != nil {
		return nil, fmt.Errorf("mcp client: new request: %w", err)
	}

	ch := make(chan *jsonrpcResponse, 1)
	c.pending.Store(id, ch)
	defer c.pending.Delete(id)

	if err := c.transport.Send(req); err != nil {
		return nil, fmt.Errorf("mcp client: send: %w", err)
	}

	// Apply a default 30 second timeout if the context has no deadline.
	callCtx := ctx
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		callCtx, cancel = context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
	}

	select {
	case resp := <-ch:
		if resp.Error != nil {
			return nil, fmt.Errorf("mcp server: %s (code %d)", resp.Error.Message, resp.Error.Code)
		}
		return resp.Result, nil
	case <-callCtx.Done():
		return nil, callCtx.Err()
	case <-c.closed:
		return nil, errors.New("mcp client: closed")
	}
}

// Notify sends a notification (no response expected).
func (c *Client) Notify(method string, params any) error {
	n, err := newNotification(method, params)
	if err != nil {
		return fmt.Errorf("mcp client: new notification: %w", err)
	}
	return c.transport.Send(n)
}

// ServerInfo returns the server name and version negotiated during Initialize.
func (c *Client) ServerInfo() (name, version string) {
	return c.serverName, c.serverVersion
}

// readLoop pumps incoming messages from the transport to waiting callers.
// Runs until the transport returns io.EOF or the client is closed.
func (c *Client) readLoop() {
	for {
		select {
		case <-c.closed:
			return
		default:
		}

		raw, err := c.transport.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return
			}
			// Other errors: log-only in Plan 6b (no logger wired).
			// Close the client to surface the error to waiting callers.
			_ = c.Close()
			return
		}

		// Try to decode as a response first
		var resp jsonrpcResponse
		if err := json.Unmarshal(raw, &resp); err != nil {
			// Malformed — drop and continue
			continue
		}
		if resp.ID != 0 {
			if chAny, ok := c.pending.LoadAndDelete(resp.ID); ok {
				if ch, ok := chAny.(chan *jsonrpcResponse); ok {
					ch <- &resp
				}
			}
			continue
		}

		// No ID — it's a notification from the server. Plan 6b ignores these.
	}
}
