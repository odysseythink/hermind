package mcp

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/odysseythink/hermind/backend/internal/mcp/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPTransport_Connect_Streamable(t *testing.T) {
	m := testutil.NewStreamableHTTPMock(t, nil)
	tr, err := newHTTPTransport(&ServerConfig{Type: "streamable", URL: m.URL})
	require.NoError(t, err)
	defer tr.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, tr.Connect(ctx))

	reqs := m.Requests()
	require.GreaterOrEqual(t, len(reqs), 1)
	assert.Equal(t, "initialize", reqs[0].Method)
}

func TestHTTPTransport_Connect_HTTPAlias(t *testing.T) {
	m := testutil.NewStreamableHTTPMock(t, nil)
	tr, err := newHTTPTransport(&ServerConfig{Type: "http", URL: m.URL})
	require.NoError(t, err)
	defer tr.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, tr.Connect(ctx))
}

func TestHTTPTransport_Connect_InvalidURL(t *testing.T) {
	_, err := newHTTPTransport(&ServerConfig{Type: "http", URL: "://invalid"})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidServerType)
}

func TestHTTPTransport_Connect_404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	tr, err := newHTTPTransport(&ServerConfig{Type: "http", URL: srv.URL})
	require.NoError(t, err)
	defer tr.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = tr.Connect(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "4xx")
}

func TestHTTPTransport_Connect_TLSError(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping TLS test in short mode")
	}
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	tr, err := newHTTPTransport(&ServerConfig{Type: "http", URL: ts.URL})
	require.NoError(t, err)
	defer tr.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = tr.Connect(ctx)
	require.Error(t, err)
	// TLS certificate verification failure
	assert.Contains(t, err.Error(), "certificate")
}

func TestHTTPTransport_ListTools(t *testing.T) {
	m := testutil.NewStreamableHTTPMock(t, []testutil.ToolDef{
		{Name: "echo", Description: "echo"},
		{Name: "add", Description: "add"},
	})
	tr, err := newHTTPTransport(&ServerConfig{URL: m.URL})
	require.NoError(t, err)
	defer tr.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, tr.Connect(ctx))

	tools, err := tr.ListTools(ctx)
	require.NoError(t, err)
	require.Len(t, tools, 2)
	names := make([]string, len(tools))
	for i, tt := range tools {
		names[i] = tt.Name
	}
	assert.ElementsMatch(t, []string{"echo", "add"}, names)
}

func TestHTTPTransport_CallTool_Success(t *testing.T) {
	m := testutil.NewStreamableHTTPMock(t, []testutil.ToolDef{
		{
			Name: "echo",
			Handler: func(args map[string]any) (any, error) {
				return args["text"], nil
			},
		},
	})
	tr, err := newHTTPTransport(&ServerConfig{URL: m.URL})
	require.NoError(t, err)
	defer tr.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, tr.Connect(ctx))

	res, err := tr.CallTool(ctx, "echo", map[string]any{"text": "hello"})
	require.NoError(t, err)
	result, ok := res.(*mcp.CallToolResult)
	require.True(t, ok)
	require.Len(t, result.Content, 1)
	assert.Equal(t, "hello", result.Content[0].(mcp.TextContent).Text)
}

func TestHTTPTransport_CallTool_HandlerError(t *testing.T) {
	m := testutil.NewStreamableHTTPMock(t, []testutil.ToolDef{
		{
			Name: "fail",
			Handler: func(args map[string]any) (any, error) {
				return nil, fmt.Errorf("boom")
			},
		},
	})
	tr, err := newHTTPTransport(&ServerConfig{URL: m.URL})
	require.NoError(t, err)
	defer tr.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, tr.Connect(ctx))

	_, err = tr.CallTool(ctx, "fail", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "boom")
}

func TestHTTPTransport_HeadersPropagated(t *testing.T) {
	m := testutil.NewStreamableHTTPMock(t, nil)
	tr, err := newHTTPTransport(&ServerConfig{
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
	assert.Equal(t, "abc", reqs[0].Headers.Get("X-Auth"))
}

func TestHTTPTransport_Ping(t *testing.T) {
	m := testutil.NewStreamableHTTPMock(t, nil)
	tr, err := newHTTPTransport(&ServerConfig{URL: m.URL})
	require.NoError(t, err)
	defer tr.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, tr.Connect(ctx))

	assert.True(t, tr.Ping(ctx))

	// Close mock server — next ping should fail
	m.Server.Close()
	assert.False(t, tr.Ping(ctx))
}

func TestHTTPTransport_ProcessInfo_Nil(t *testing.T) {
	m := testutil.NewStreamableHTTPMock(t, nil)
	tr, err := newHTTPTransport(&ServerConfig{URL: m.URL})
	require.NoError(t, err)
	defer tr.Close()
	assert.Nil(t, tr.ProcessInfo())
}

func TestHTTPTransport_Close_Idempotent(t *testing.T) {
	m := testutil.NewStreamableHTTPMock(t, nil)
	tr, err := newHTTPTransport(&ServerConfig{URL: m.URL})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, tr.Connect(ctx))

	assert.NoError(t, tr.Close())
	assert.NoError(t, tr.Close())
}

func TestHTTPTransport_Close_AbortsInFlight(t *testing.T) {
	m := testutil.NewStreamableHTTPMock(t, []testutil.ToolDef{
		{
			Name: "slow_echo",
			Handler: func(args map[string]any) (any, error) {
				// Sleep long enough that Close() is called while still in-flight
				time.Sleep(1 * time.Second)
				return args["text"], nil
			},
		},
	})
	tr, err := newHTTPTransport(&ServerConfig{URL: m.URL})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, tr.Connect(ctx))

	callCtx, callCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer callCancel()

	var callErr error
	done := make(chan struct{})
	go func() {
		_, callErr = tr.CallTool(callCtx, "slow_echo", map[string]any{"text": "hi"})
		close(done)
	}()

	// Give the POST time to be in-flight
	time.Sleep(200 * time.Millisecond)
	_ = tr.Close()

	select {
	case <-done:
		require.Error(t, callErr)
	case <-time.After(3 * time.Second):
		t.Fatal("Close did not abort in-flight call")
	}
}

func TestHTTPTransport_ConnectRespectsContext(t *testing.T) {
	m := testutil.NewStreamableHTTPMock(t, nil)
	tr, err := newHTTPTransport(&ServerConfig{URL: m.URL})
	require.NoError(t, err)
	defer tr.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = tr.Connect(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "context")
}

func TestHTTPTransport_ConcurrentCalls(t *testing.T) {
	m := testutil.NewStreamableHTTPMock(t, []testutil.ToolDef{
		{
			Name: "echo",
			Handler: func(args map[string]any) (any, error) {
				return args["text"], nil
			},
		},
	})
	tr, err := newHTTPTransport(&ServerConfig{URL: m.URL})
	require.NoError(t, err)
	defer tr.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	require.NoError(t, tr.Connect(ctx))

	const n = 10
	results := make([]string, n)
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(idx int) {
			defer wg.Done()
			res, err := tr.CallTool(ctx, "echo", map[string]any{"text": fmt.Sprintf("msg-%d", idx)})
			if err != nil {
				return
			}
			result := res.(*mcp.CallToolResult)
			if len(result.Content) > 0 {
				results[idx] = result.Content[0].(mcp.TextContent).Text
			}
		}(i)
	}
	wg.Wait()

	for i := 0; i < n; i++ {
		assert.Equal(t, fmt.Sprintf("msg-%d", i), results[i])
	}
}
