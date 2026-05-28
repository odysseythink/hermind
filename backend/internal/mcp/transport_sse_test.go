package mcp

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/odysseythink/hermind/backend/internal/mcp/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSSETransport_Connect_EndpointDiscovery(t *testing.T) {
	m := testutil.NewSSEMock(t, nil)
	tr, err := newSSETransport(&ServerConfig{URL: m.URL})
	require.NoError(t, err)
	defer tr.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, tr.Connect(ctx))
}

func TestSSETransport_Connect_ExplicitTypeSSE(t *testing.T) {
	m := testutil.NewSSEMock(t, nil)
	tr, err := newSSETransport(&ServerConfig{Type: "sse", URL: m.URL})
	require.NoError(t, err)
	defer tr.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, tr.Connect(ctx))
}

func TestSSETransport_Connect_NoEndpointEvent(t *testing.T) {
	// Server that never sends endpoint event
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, ok := w.(http.Flusher)
		if !ok {
			return
		}
		// Just keep connection open without sending endpoint
		for {
			select {
			case <-r.Context().Done():
				return
			case <-time.After(1 * time.Second):
				fmt.Fprintf(w, ":ping\n\n")
				flusher.Flush()
			}
		}
	}))
	defer srv.Close()

	tr, err := newSSETransport(&ServerConfig{URL: srv.URL + "/sse"})
	require.NoError(t, err)
	defer tr.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err = tr.Connect(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "deadline")
}

func TestSSETransport_Connect_MalformedEndpointEvent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, ok := w.(http.Flusher)
		if !ok {
			return
		}
		fmt.Fprintf(w, "event: endpoint\ndata: not-a-url\n\n")
		flusher.Flush()
		<-r.Context().Done()
	}))
	defer srv.Close()

	tr, err := newSSETransport(&ServerConfig{URL: srv.URL + "/sse"})
	require.NoError(t, err)
	defer tr.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	err = tr.Connect(ctx)
	require.Error(t, err)
}

func TestSSETransport_ListTools_Roundtrip(t *testing.T) {
	m := testutil.NewSSEMock(t, []testutil.ToolDef{
		{Name: "echo", Description: "echo"},
		{Name: "add", Description: "add"},
	})
	tr, err := newSSETransport(&ServerConfig{URL: m.URL})
	require.NoError(t, err)
	defer tr.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, tr.Connect(ctx))

	tools, err := tr.ListTools(ctx)
	require.NoError(t, err)
	require.Len(t, tools, 2)
}

func TestSSETransport_CallTool_Success(t *testing.T) {
	m := testutil.NewSSEMock(t, []testutil.ToolDef{
		{
			Name: "add",
			Handler: func(args map[string]any) (any, error) {
				a, _ := args["a"].(float64)
				b, _ := args["b"].(float64)
				return a + b, nil
			},
		},
	})
	tr, err := newSSETransport(&ServerConfig{URL: m.URL})
	require.NoError(t, err)
	defer tr.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, tr.Connect(ctx))

	res, err := tr.CallTool(ctx, "add", map[string]any{"a": 2, "b": 3})
	require.NoError(t, err)
	result, ok := res.(*mcp.CallToolResult)
	require.True(t, ok)
	require.Len(t, result.Content, 1)
	assert.Equal(t, "5", result.Content[0].(mcp.TextContent).Text)
}

func TestSSETransport_HeadersPropagated(t *testing.T) {
	m := testutil.NewSSEMock(t, nil)
	tr, err := newSSETransport(&ServerConfig{
		URL:     m.URL,
		Headers: map[string]string{"X-Auth": "abc"},
	})
	require.NoError(t, err)
	defer tr.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, tr.Connect(ctx))

	reqs := m.Requests()
	require.GreaterOrEqual(t, len(reqs), 1)
	// At least one request should have the header
	found := false
	for _, r := range reqs {
		if r.Headers.Get("X-Auth") == "abc" {
			found = true
			break
		}
	}
	assert.True(t, found, "X-Auth header not propagated")
}

func TestSSETransport_Ping_StreamUp(t *testing.T) {
	m := testutil.NewSSEMock(t, nil)
	tr, err := newSSETransport(&ServerConfig{URL: m.URL})
	require.NoError(t, err)
	defer tr.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, tr.Connect(ctx))

	assert.True(t, tr.Ping(ctx))

	// Close the transport — stream goes down
	assert.NoError(t, tr.Close())
	assert.False(t, tr.Ping(ctx))
}

func TestSSETransport_Close_StopsStream(t *testing.T) {
	m := testutil.NewSSEMock(t, nil)
	baseline := runtime.NumGoroutine()

	tr, err := newSSETransport(&ServerConfig{URL: m.URL})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, tr.Connect(ctx))

	// Give reader goroutine time to start
	time.Sleep(100 * time.Millisecond)

	require.NoError(t, tr.Close())
	time.Sleep(200 * time.Millisecond)

	after := runtime.NumGoroutine()
	assert.LessOrEqual(t, after, baseline+3, "goroutine leak detected: baseline=%d after=%d", baseline, after)
}

func TestSSETransport_Close_Idempotent(t *testing.T) {
	m := testutil.NewSSEMock(t, nil)
	tr, err := newSSETransport(&ServerConfig{URL: m.URL})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, tr.Connect(ctx))

	assert.NoError(t, tr.Close())
	assert.NoError(t, tr.Close())
}

func TestSSETransport_ContextCancel_AbortsCall(t *testing.T) {
	m := testutil.NewSSEMock(t, []testutil.ToolDef{
		{
			Name: "slow",
			Handler: func(args map[string]any) (any, error) {
				// Sleep long enough that cancel happens while in-flight
				time.Sleep(1 * time.Second)
				return "done", nil
			},
		},
	})
	tr, err := newSSETransport(&ServerConfig{URL: m.URL})
	require.NoError(t, err)
	defer tr.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, tr.Connect(ctx))

	callCtx, callCancel := context.WithCancel(context.Background())
	var callErr error
	done := make(chan struct{})
	go func() {
		_, callErr = tr.CallTool(callCtx, "slow", nil)
		close(done)
	}()

	time.Sleep(200 * time.Millisecond)
	callCancel()

	select {
	case <-done:
		require.Error(t, callErr)
	case <-time.After(3 * time.Second):
		t.Fatal("context cancel did not abort call")
	}
}

func TestSSETransport_ServerDisconnect_PingFalse(t *testing.T) {
	m := testutil.NewSSEMock(t, nil)
	tr, err := newSSETransport(&ServerConfig{URL: m.URL})
	require.NoError(t, err)
	defer tr.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, tr.Connect(ctx))

	assert.True(t, tr.Ping(ctx))

	// Close the transport (simulates disconnect from either side)
	require.NoError(t, tr.Close())
	// Ping should eventually fail; may need a retry
	for i := 0; i < 20; i++ {
		if !tr.Ping(ctx) {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("expected Ping to return false after disconnect")
}

func TestSSETransport_ConnectRespectsContext(t *testing.T) {
	m := testutil.NewSSEMock(t, nil)
	tr, err := newSSETransport(&ServerConfig{URL: m.URL})
	require.NoError(t, err)
	defer tr.Close()

	// Use a context that expires before the SSE handshake can complete.
	// An already-cancelled context triggers a data race inside the SDK
	// (concurrent Start/Close), so we use a tiny timeout instead.
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()
	time.Sleep(2 * time.Millisecond)

	err = tr.Connect(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "deadline")
}

func TestSSETransport_GoroutineLeakOnClose(t *testing.T) {
	m := testutil.NewSSEMock(t, nil)
	baseline := runtime.NumGoroutine()

	for i := 0; i < 3; i++ {
		tr, err := newSSETransport(&ServerConfig{URL: m.URL})
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = tr.Connect(ctx)
		cancel()

		_ = tr.Close()
	}

	time.Sleep(300 * time.Millisecond)
	after := runtime.NumGoroutine()
	assert.LessOrEqual(t, after, baseline+3, "goroutine leak after repeated connect/close")
}
