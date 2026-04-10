// provider/qwen/qwen_test.go
package qwen

import (
	"testing"

	"github.com/nousresearch/hermes-agent/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRequiresAPIKey(t *testing.T) {
	_, err := New(config.ProviderConfig{Provider: "qwen"})
	assert.Error(t, err)
}

func TestNewHappyPath(t *testing.T) {
	p, err := New(config.ProviderConfig{
		Provider: "qwen",
		APIKey:   "sk-test",
		Model:    "qwen-max",
	})
	require.NoError(t, err)
	assert.Equal(t, "qwen", p.Name())
	assert.True(t, p.Available())
}
