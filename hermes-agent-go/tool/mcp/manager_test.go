// tool/mcp/manager_test.go
package mcp

import (
	"context"
	"testing"

	"github.com/nousresearch/hermes-agent/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManagerNewIsEmpty(t *testing.T) {
	reg := tool.NewRegistry()
	m := NewManager("test-version", reg)
	assert.Empty(t, m.Servers())
}

func TestManagerStartRejectsEmptyCommand(t *testing.T) {
	reg := tool.NewRegistry()
	m := NewManager("test-version", reg)
	err := m.startOne(context.Background(), ServerConfig{Name: "x", Command: ""})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "command is required")
}

func TestManagerCloseIsIdempotent(t *testing.T) {
	reg := tool.NewRegistry()
	m := NewManager("test-version", reg)
	require.NoError(t, m.Close())
	require.NoError(t, m.Close())
}

func TestJoinErrorsSingle(t *testing.T) {
	err := joinErrors([]error{assertError{msg: "one"}})
	assert.EqualError(t, err, "one")
}

func TestJoinErrorsMultiple(t *testing.T) {
	err := joinErrors([]error{assertError{msg: "one"}, assertError{msg: "two"}})
	assert.Contains(t, err.Error(), "one")
	assert.Contains(t, err.Error(), "two")
}

// assertError is a minimal error type for tests.
type assertError struct{ msg string }

func (e assertError) Error() string { return e.msg }
