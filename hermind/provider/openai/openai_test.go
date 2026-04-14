// provider/openai/openai_test.go
package openai

import (
	"testing"

	"github.com/odysseythink/hermind/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRequiresAPIKey(t *testing.T) {
	_, err := New(config.ProviderConfig{Provider: "openai"})
	assert.Error(t, err)
}

func TestNewHappyPath(t *testing.T) {
	p, err := New(config.ProviderConfig{
		Provider: "openai",
		APIKey:   "sk-test",
		Model:    "gpt-4o",
	})
	require.NoError(t, err)
	assert.Equal(t, "openai", p.Name())
	assert.True(t, p.Available())
}

func TestNewAcceptsCustomBaseURL(t *testing.T) {
	p, err := New(config.ProviderConfig{
		Provider: "openai",
		APIKey:   "sk-test",
		BaseURL:  "https://custom.example.com/v1",
		Model:    "gpt-4o",
	})
	require.NoError(t, err)
	assert.Equal(t, "openai", p.Name())
}
