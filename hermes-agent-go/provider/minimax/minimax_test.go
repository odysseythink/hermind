// provider/minimax/minimax_test.go
package minimax

import (
	"testing"

	"github.com/nousresearch/hermes-agent/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRequiresAPIKey(t *testing.T) {
	_, err := New(config.ProviderConfig{Provider: "minimax"})
	assert.Error(t, err)
}

func TestNewHappyPath(t *testing.T) {
	p, err := New(config.ProviderConfig{
		Provider: "minimax",
		APIKey:   "sk-test",
		Model:    "abab6.5s-chat",
	})
	require.NoError(t, err)
	assert.Equal(t, "minimax", p.Name())
	assert.True(t, p.Available())
}
