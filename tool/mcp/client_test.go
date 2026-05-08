// tool/mcp/client_test.go
package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeTransport is an in-memory Transport for testing the Client.
// It buffers outgoing messages and delivers scripted responses.
type fakeTransport struct {
	mu        sync.Mutex
	started   bool
	closed    bool
	sent      []json.RawMessage
	responses chan json.RawMessage
}

func newFakeTransport() *fakeTransport {
	return &fakeTransport{
		responses: make(chan json.RawMessage, 16),
	}
}

func (f *fakeTransport) Start(ctx context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.started = true
	return nil
}

func (f *fakeTransport) Send(msg any) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	f.sent = append(f.sent, data)
	return nil
}

func (f *fakeTransport) Recv() (json.RawMessage, error) {
	select {
	case r, ok := <-f.responses:
		if !ok {
			return nil, io.EOF
		}
		return r, nil
	case <-time.After(5 * time.Second):
		return nil, errors.New("fake transport: recv timeout")
	}
}

func (f *fakeTransport) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.closed {
		f.closed = true
		close(f.responses)
	}
	return nil
}

// injectResponse queues a scripted response to be delivered on the next Recv.
func (f *fakeTransport) injectResponse(resp *jsonrpcResponse) {
	data, _ := json.Marshal(resp)
	f.responses <- data
}

func TestClientCallHappyPath(t *testing.T) {
	ft := newFakeTransport()
	c := NewClient(ft)
	require.NoError(t, c.Start(context.Background()))
	defer c.Close()

	// Arrange: inject a response for ID 1
	go func() {
		// Wait briefly so the Call has a chance to register its pending channel.
		time.Sleep(10 * time.Millisecond)
		ft.injectResponse(&jsonrpcResponse{
			JSONRPC: "2.0",
			ID:      1,
			Result:  json.RawMessage(`{"ok":true}`),
		})
	}()

	result, err := c.Call(context.Background(), "ping", nil)
	require.NoError(t, err)
	assert.JSONEq(t, `{"ok":true}`, string(result))
}

func TestClientCallReturnsServerError(t *testing.T) {
	ft := newFakeTransport()
	c := NewClient(ft)
	require.NoError(t, c.Start(context.Background()))
	defer c.Close()

	go func() {
		time.Sleep(10 * time.Millisecond)
		ft.injectResponse(&jsonrpcResponse{
			JSONRPC: "2.0",
			ID:      1,
			Error:   &jsonrpcError{Code: -32601, Message: "method not found"},
		})
	}()

	_, err := c.Call(context.Background(), "bogus", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "method not found")
}

func TestClientCallTimeout(t *testing.T) {
	ft := newFakeTransport()
	c := NewClient(ft)
	require.NoError(t, c.Start(context.Background()))
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := c.Call(ctx, "slow", nil)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestClientCallAfterCloseReturnsError(t *testing.T) {
	ft := newFakeTransport()
	c := NewClient(ft)
	require.NoError(t, c.Start(context.Background()))
	require.NoError(t, c.Close())

	_, err := c.Call(context.Background(), "ping", nil)
	assert.Error(t, err)
}

func TestClientConcurrentCalls(t *testing.T) {
	ft := newFakeTransport()
	c := NewClient(ft)
	require.NoError(t, c.Start(context.Background()))
	defer c.Close()

	// Responder goroutine: read the sent requests, echo responses with matching IDs
	go func() {
		for i := 0; i < 5; i++ {
			time.Sleep(5 * time.Millisecond)
			ft.mu.Lock()
			numSent := len(ft.sent)
			ft.mu.Unlock()
			if numSent > i {
				// Parse the i-th sent request to get its ID
				var req jsonrpcRequest
				ft.mu.Lock()
				_ = json.Unmarshal(ft.sent[i], &req)
				ft.mu.Unlock()
				ft.injectResponse(&jsonrpcResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Result:  json.RawMessage(`"ok"`),
				})
			}
		}
	}()

	var wg sync.WaitGroup
	results := make([]error, 5)
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, results[idx] = c.Call(context.Background(), "ping", nil)
		}(i)
	}
	wg.Wait()

	for _, err := range results {
		assert.NoError(t, err)
	}
}
