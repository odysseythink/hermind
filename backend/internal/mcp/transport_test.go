package mcp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTransport_Stdio(t *testing.T) {
	tr, err := newTransport(&ServerConfig{Command: "/usr/bin/true"})
	require.NoError(t, err)
	require.NotNil(t, tr)
}

func TestNewTransport_HTTPDispatched(t *testing.T) {
	tr, err := newTransport(&ServerConfig{Type: "http", URL: "http://x"})
	require.NoError(t, err)
	require.NotNil(t, tr)
}

func TestNewTransport_SSEDispatched(t *testing.T) {
	tr, err := newTransport(&ServerConfig{URL: "http://x"})
	require.NoError(t, err)
	require.NotNil(t, tr)
}

func TestNewTransport_StreamableExplicit(t *testing.T) {
	tr, err := newTransport(&ServerConfig{Type: "streamable", URL: "http://x"})
	require.NoError(t, err)
	require.NotNil(t, tr)
}

func TestNewTransport_InvalidEmpty(t *testing.T) {
	tr, err := newTransport(&ServerConfig{})
	assert.Nil(t, tr)
	assert.ErrorIs(t, err, ErrInvalidServerType)
}
