package mcp

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func echoBin(t *testing.T) string {
	bin := os.Getenv("MCP_ECHO_BIN")
	require.NotEmpty(t, bin, "MCP_ECHO_BIN not set; run via go test with TestMain")
	return bin
}

func TestStdioTransport_Connect_Echo(t *testing.T) {
	tr := mustStdioTransport(t, echoBin(t))
	defer tr.Close()

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	require.NoError(t, tr.Connect(ctx))
}

func TestStdioTransport_Connect_NonexistentCommand(t *testing.T) {
	tr, err := newStdioTransport(&ServerConfig{Command: "/nonexistent/binary/for/sure"})
	require.NoError(t, err)
	defer tr.Close()

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	err = tr.Connect(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "start command")
}

func TestStdioTransport_Connect_Timeout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("sleep command not available on Windows")
	}
	tr, err := newStdioTransport(&ServerConfig{Command: "sleep", Args: []string{"10"}})
	require.NoError(t, err)
	defer tr.Close()

	ctx, cancel := context.WithTimeout(t.Context(), 1*time.Second)
	defer cancel()
	err = tr.Connect(ctx)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestStdioTransport_ListTools(t *testing.T) {
	tr := mustStdioTransport(t, echoBin(t))
	defer tr.Close()

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	require.NoError(t, tr.Connect(ctx))

	tools, err := tr.ListTools(ctx)
	require.NoError(t, err)
	require.Len(t, tools, 3)
	names := make([]string, len(tools))
	for i, tl := range tools {
		names[i] = tl.Name
	}
	assert.ElementsMatch(t, []string{"echo", "add", "slow_echo"}, names)
}

func TestStdioTransport_CallTool_Echo(t *testing.T) {
	tr := mustStdioTransport(t, echoBin(t))
	defer tr.Close()

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	require.NoError(t, tr.Connect(ctx))

	res, err := tr.CallTool(ctx, "echo", map[string]any{"text": "hi"})
	require.NoError(t, err)
	result, ok := res.(*mcp.CallToolResult)
	require.True(t, ok)
	require.Len(t, result.Content, 1)
	tc, ok := mcp.AsTextContent(result.Content[0])
	require.True(t, ok)
	assert.Equal(t, "hi", tc.Text)
}

func TestStdioTransport_CallTool_Add(t *testing.T) {
	tr := mustStdioTransport(t, echoBin(t))
	defer tr.Close()

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	require.NoError(t, tr.Connect(ctx))

	res, err := tr.CallTool(ctx, "add", map[string]any{"a": 2, "b": 3})
	require.NoError(t, err)
	result, ok := res.(*mcp.CallToolResult)
	require.True(t, ok)
	require.Len(t, result.Content, 1)
	tc, ok := mcp.AsTextContent(result.Content[0])
	require.True(t, ok)
	assert.Equal(t, "sum=5", tc.Text)
}

func TestStdioTransport_CallTool_UnknownTool(t *testing.T) {
	tr := mustStdioTransport(t, echoBin(t))
	defer tr.Close()

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	require.NoError(t, tr.Connect(ctx))

	_, err := tr.CallTool(ctx, "nonexistent", map[string]any{})
	require.Error(t, err)
}

func TestStdioTransport_Ping(t *testing.T) {
	tr := mustStdioTransport(t, echoBin(t))
	defer tr.Close()

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	require.NoError(t, tr.Connect(ctx))
	assert.True(t, tr.Ping(ctx))

	require.NoError(t, tr.Close())
	assert.False(t, tr.Ping(ctx))
}

func TestStdioTransport_ProcessInfo(t *testing.T) {
	tr := mustStdioTransport(t, echoBin(t))
	defer tr.Close()

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	require.NoError(t, tr.Connect(ctx))

	info := tr.ProcessInfo()
	require.NotNil(t, info)
	assert.Greater(t, info.PID, 0)
	assert.Contains(t, info.Cmd, "echo-mcp")
}

func TestStdioTransport_Close_Idempotent(t *testing.T) {
	tr := mustStdioTransport(t, echoBin(t))

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	require.NoError(t, tr.Connect(ctx))

	assert.NoError(t, tr.Close())
	assert.NoError(t, tr.Close())
}

func TestStdioTransport_Close_GracefulSIGTERM(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("SIGTERM not available on Windows")
	}
	tr := mustStdioTransport(t, echoBin(t))

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	require.NoError(t, tr.Connect(ctx))

	pid := tr.ProcessInfo().PID
	require.NoError(t, tr.Close())

	// Verify process is gone within 4s.
	require.Eventually(t, func() bool {
		proc, err := os.FindProcess(pid)
		if err != nil {
			return true
		}
		return proc.Signal(syscall.Signal(0)) != nil
	}, 4*time.Second, 100*time.Millisecond, "child process should be reaped after Close")
}

func TestStdioTransport_Close_KillsRunaway(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("SIGTERM/SIGKILL not available on Windows")
	}

	// Build sig-ignorer binary on the fly.
	sigBin := filepath.Join(t.TempDir(), "sig-ignorer")
	buildCmd := exec.Command("go", "build", "-o", sigBin, "./testdata/sig-ignorer")
	buildCmd.Dir = "."
	require.NoError(t, buildCmd.Run())

	tr := mustStdioTransport(t, sigBin)

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	require.NoError(t, tr.Connect(ctx))

	pid := tr.ProcessInfo().PID

	start := time.Now()
	require.NoError(t, tr.Close())
	elapsed := time.Since(start)

	// Should have been killed by SIGKILL after 3s grace.
	assert.Less(t, elapsed, 4*time.Second, "SIGKILL should fire within 3s grace + slack")

	proc, _ := os.FindProcess(pid)
	if proc != nil {
		assert.NotNil(t, proc.Signal(syscall.Signal(0)), "process should be gone")
	}
}

func mustStdioTransport(t *testing.T, bin string) Transport {
	tr, err := newStdioTransport(&ServerConfig{Command: bin})
	require.NoError(t, err)
	t.Cleanup(func() { _ = tr.Close() })
	return tr
}
