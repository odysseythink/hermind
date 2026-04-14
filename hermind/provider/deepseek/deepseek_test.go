// provider/deepseek/deepseek_test.go
package deepseek

import (
	"testing"

	"github.com/odysseythink/hermind/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRequiresAPIKey(t *testing.T) {
	_, err := New(config.ProviderConfig{Provider: "deepseek"})
	assert.Error(t, err)
}

func TestNewHappyPath(t *testing.T) {
	p, err := New(config.ProviderConfig{
		Provider: "deepseek",
		APIKey:   "sk-test",
		Model:    "deepseek-chat",
	})
	require.NoError(t, err)
	assert.Equal(t, "deepseek", p.Name())
	assert.True(t, p.Available())
}
