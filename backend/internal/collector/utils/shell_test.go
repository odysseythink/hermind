package utils

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShellRunner_CheckInstalled(t *testing.T) {
	runner := NewShellRunner()

	assert.True(t, runner.CheckInstalled("echo"), "echo should be installed")
	assert.False(t, runner.CheckInstalled("definitely-not-a-real-command-12345"), "fake command should not be installed")
}

func TestShellRunner_Run(t *testing.T) {
	runner := NewShellRunner()
	ctx := context.Background()

	out, err := runner.Run(ctx, "echo", "hello")
	require.NoError(t, err)
	assert.Equal(t, "hello", out)
}

func TestShellRunner_Run_InvalidCommand(t *testing.T) {
	runner := NewShellRunner()
	ctx := context.Background()

	_, err := runner.Run(ctx, "definitely-not-a-real-command-12345")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed:")
}

func TestShellRunner_RunWithTimeout_Success(t *testing.T) {
	runner := NewShellRunner()
	ctx := context.Background()

	out, err := runner.RunWithTimeout(ctx, 5*time.Second, "echo", "timed")
	require.NoError(t, err)
	assert.Equal(t, "timed", out)
}

func TestShellRunner_RunWithTimeout_Expires(t *testing.T) {
	runner := NewShellRunner()
	ctx := context.Background()

	// Use a very short timeout with a command that sleeps longer.
	_, err := runner.RunWithTimeout(ctx, 100*time.Millisecond, "sleep", "10")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "sleep failed:")
}

func TestShellRunner_RunWithTimeout_ZeroDuration(t *testing.T) {
	runner := NewShellRunner()
	ctx := context.Background()

	// Zero duration should fall back to the provided context (no additional timeout).
	out, err := runner.RunWithTimeout(ctx, 0, "echo", "no-timeout")
	require.NoError(t, err)
	assert.Equal(t, "no-timeout", out)
}
