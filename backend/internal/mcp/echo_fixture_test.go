package mcp

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEchoBinPath(t *testing.T) {
	bin := os.Getenv("MCP_ECHO_BIN")
	require.NotEmpty(t, bin, "MCP_ECHO_BIN must be set by TestMain")
	assert.FileExists(t, bin)
}
