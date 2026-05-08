// provider/kimi/kimi_test.go
package kimi

import (
	"testing"

	"github.com/odysseythink/hermind/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRequiresAPIKey(t *testing.T) {
	_, err := New(config.ProviderConfig{Provider: "kimi"})
	assert.Error(t, err)
}

func TestNewHappyPath(t *testing.T) {
	p, err := New(config.ProviderConfig{
		Provider: "kimi",
		APIKey:   "sk-test",
		Model:    "moonshot-v1-32k",
	})
	require.NoError(t, err)
	assert.Equal(t, "kimi", p.Name())
	assert.True(t, p.Available())
}
