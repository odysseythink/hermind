// tool/mcp/transport_test.go
package mcp

import (
	"context"
	"encoding/json"
	"io"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStdioTransportEchoes verifies that Send/Recv round-trip a message
// through a subprocess that echoes stdin to stdout.
func TestStdioTransportEchoes(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("sh subprocess not available on Windows")
	}

	// A tiny shell script: read a line and write it back to stdout.
	// Using `cat` is simpler and works across all Unix shells.
	tr := NewStdioTransport("/bin/sh", []string{"-c", "cat"}, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	require.NoError(t, tr.Start(ctx))
	defer tr.Close()

	// Send a JSON-RPC request
	req, err := newRequest(1, "ping", nil)
	require.NoError(t, err)
	require.NoError(t, tr.Send(req))

	// Receive the echo
	raw, err := tr.Recv()
	require.NoError(t, err)

	var got jsonrpcRequest
	require.NoError(t, json.Unmarshal(raw, &got))
	assert.Equal(t, int64(1), got.ID)
	assert.Equal(t, "ping", got.Method)
}

func TestStdioTransportCloseIsIdempotent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("sh subprocess not available on Windows")
	}
	tr := NewStdioTransport("/bin/sh", []string{"-c", "cat"}, nil)
	require.NoError(t, tr.Start(context.Background()))
	require.NoError(t, tr.Close())
	require.NoError(t, tr.Close()) // second close must not panic
}

func TestStdioTransportSendBeforeStartFails(t *testing.T) {
	tr := NewStdioTransport("/bin/sh", []string{"-c", "cat"}, nil)
	err := tr.Send(map[string]any{"foo": "bar"})
	assert.Error(t, err)
}

func TestStdioTransportRecvAfterCloseReturnsEOF(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("sh subprocess not available on Windows")
	}
	tr := NewStdioTransport("/bin/sh", []string{"-c", "cat"}, nil)
	require.NoError(t, tr.Start(context.Background()))
	require.NoError(t, tr.Close())

	_, err := tr.Recv()
	assert.ErrorIs(t, err, io.EOF)
}
