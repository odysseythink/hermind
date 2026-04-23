package cli

import (
	"net"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListenRandomLocalhost_PicksSomethingInRange(t *testing.T) {
	ln, err := listenRandomLocalhost()
	require.NoError(t, err)
	defer ln.Close()

	addr, ok := ln.Addr().(*net.TCPAddr)
	require.True(t, ok)
	assert.Equal(t, "127.0.0.1", addr.IP.String())
	assert.GreaterOrEqual(t, addr.Port, 30000)
	assert.Less(t, addr.Port, 40000)
}

func TestListenRandomLocalhost_RetriesWhenPortBusy(t *testing.T) {
	occupier, err := net.Listen("tcp", "127.0.0.1:35000")
	require.NoError(t, err)
	defer occupier.Close()

	ln, err := listenRandomLocalhost()
	require.NoError(t, err)
	defer ln.Close()

	addr := ln.Addr().(*net.TCPAddr)
	assert.NotEqual(t, 35000, addr.Port)
}

func TestListenOnRange_InvalidRange(t *testing.T) {
	_, err := listenOnRange(1, 0, 1)
	require.Error(t, err)
	assert.True(t,
		strings.Contains(err.Error(), "invalid port range"),
		"got %v", err)
}
