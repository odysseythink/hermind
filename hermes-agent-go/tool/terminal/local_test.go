package terminal

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/nousresearch/hermes-agent/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLocalExecSimpleCommand(t *testing.T) {
	b, err := NewLocal(Config{})
	require.NoError(t, err)
	defer b.Close()

	result, err := b.Execute(context.Background(), "echo hello", &ExecOptions{Timeout: 5 * time.Second})
	require.NoError(t, err)
	assert.Equal(t, 0, result.ExitCode)
	assert.Contains(t, result.Stdout, "hello")
}

func TestLocalExecCaptureStderr(t *testing.T) {
	b, _ := NewLocal(Config{})
	defer b.Close()
	result, err := b.Execute(context.Background(), "echo oops 1>&2", &ExecOptions{Timeout: 5 * time.Second})
	require.NoError(t, err)
	assert.Contains(t, result.Stderr, "oops")
}

func TestLocalExecPreservesExitCode(t *testing.T) {
	b, _ := NewLocal(Config{})
	defer b.Close()
	result, err := b.Execute(context.Background(), "exit 7", &ExecOptions{Timeout: 5 * time.Second})
	require.NoError(t, err)
	assert.Equal(t, 7, result.ExitCode)
}

func TestLocalExecRespectsCwd(t *testing.T) {
	dir := t.TempDir()
	b, _ := NewLocal(Config{})
	defer b.Close()
	result, err := b.Execute(context.Background(), "pwd", &ExecOptions{
		Cwd:     dir,
		Timeout: 5 * time.Second,
	})
	require.NoError(t, err)
	// macOS may prepend /private to temp dirs — use HasSuffix to tolerate
	got := strings.TrimSpace(result.Stdout)
	assert.True(t, strings.HasSuffix(got, dir), "expected %q to end with %q", got, dir)
}

func TestLocalExecTimeout(t *testing.T) {
	b, _ := NewLocal(Config{})
	defer b.Close()
	start := time.Now()
	result, err := b.Execute(context.Background(), "sleep 10", &ExecOptions{Timeout: 200 * time.Millisecond})
	elapsed := time.Since(start)

	assert.Less(t, elapsed, 2*time.Second, "timeout should kill the process quickly")
	if err == nil {
		// Some shells report timeout via non-zero exit code instead of error
		assert.NotEqual(t, 0, result.ExitCode)
	}
}

func TestLocalExecStdin(t *testing.T) {
	b, _ := NewLocal(Config{})
	defer b.Close()
	result, err := b.Execute(context.Background(), "cat", &ExecOptions{
		Stdin:   "piped input",
		Timeout: 5 * time.Second,
	})
	require.NoError(t, err)
	assert.Contains(t, result.Stdout, "piped input")
}

func TestShellExecuteToolRegisters(t *testing.T) {
	reg := tool.NewRegistry()
	backend, _ := NewLocal(Config{})
	defer backend.Close()

	RegisterShellExecute(reg, backend)

	defs := reg.Definitions(nil)
	require.Len(t, defs, 1)
	assert.Equal(t, "shell_execute", defs[0].Function.Name)
}

func TestShellExecuteToolDispatch(t *testing.T) {
	reg := tool.NewRegistry()
	backend, _ := NewLocal(Config{})
	defer backend.Close()
	RegisterShellExecute(reg, backend)

	args := json.RawMessage(`{"command":"echo hi"}`)
	result, err := reg.Dispatch(context.Background(), "shell_execute", args)
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal([]byte(result), &decoded))
	assert.Contains(t, decoded["stdout"], "hi")
	assert.Equal(t, float64(0), decoded["exit_code"])
}

func TestShellExecuteRejectsMissingCommand(t *testing.T) {
	reg := tool.NewRegistry()
	backend, _ := NewLocal(Config{})
	defer backend.Close()
	RegisterShellExecute(reg, backend)

	args := json.RawMessage(`{}`)
	result, err := reg.Dispatch(context.Background(), "shell_execute", args)
	require.NoError(t, err)
	assert.Contains(t, result, `"error"`)
	assert.Contains(t, result, "command")
}

// Factory dispatch tests

func TestNewFactoryDispatchesLocal(t *testing.T) {
	b, err := New("local", Config{})
	require.NoError(t, err)
	_, ok := b.(*Local)
	assert.True(t, ok)
}

func TestNewFactoryDispatchesSSH(t *testing.T) {
	b, err := New("ssh", Config{SSHHost: "h", SSHUser: "u", SSHKey: "/tmp/k"})
	require.NoError(t, err)
	_, ok := b.(*SSH)
	assert.True(t, ok)
}

func TestNewFactoryDispatchesModal(t *testing.T) {
	b, err := New("modal", Config{ModalBaseURL: "https://x", ModalToken: "t"})
	require.NoError(t, err)
	_, ok := b.(*Modal)
	assert.True(t, ok)
}

func TestNewFactoryDispatchesDaytona(t *testing.T) {
	b, err := New("daytona", Config{DaytonaBaseURL: "https://x", DaytonaToken: "t"})
	require.NoError(t, err)
	_, ok := b.(*Daytona)
	assert.True(t, ok)
}

func TestNewFactoryUnknown(t *testing.T) {
	_, err := New("made-up-backend", Config{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not supported")
}
