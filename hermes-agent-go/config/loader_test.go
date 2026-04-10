package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfigHasSensibleDefaults(t *testing.T) {
	cfg := Default()
	assert.Equal(t, "anthropic/claude-opus-4-6", cfg.Model)
	assert.Equal(t, 90, cfg.Agent.MaxTurns)
	assert.Equal(t, 1800, cfg.Agent.GatewayTimeout)
	assert.Equal(t, "sqlite", cfg.Storage.Driver)
	assert.NotNil(t, cfg.Providers)
}

func TestLoadFromYAML(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(yamlPath, []byte(`
model: anthropic/claude-sonnet-4-6
providers:
  anthropic:
    provider: anthropic
    api_key: sk-test-abc
    model: claude-sonnet-4-6
agent:
  max_turns: 42
storage:
  driver: sqlite
  sqlite_path: /tmp/test.db
`), 0o644)
	require.NoError(t, err)

	cfg, err := LoadFromPath(yamlPath)
	require.NoError(t, err)
	assert.Equal(t, "anthropic/claude-sonnet-4-6", cfg.Model)
	assert.Equal(t, 42, cfg.Agent.MaxTurns)
	assert.Equal(t, "sk-test-abc", cfg.Providers["anthropic"].APIKey)
	assert.Equal(t, "/tmp/test.db", cfg.Storage.SQLitePath)
}

func TestLoadFromMissingFileReturnsDefaults(t *testing.T) {
	cfg, err := LoadFromPath("/nonexistent/path/config.yaml")
	require.NoError(t, err)
	assert.Equal(t, Default().Model, cfg.Model)
}

func TestEnvVarExpansion(t *testing.T) {
	t.Setenv("HERMES_TEST_KEY", "sk-from-env")
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(yamlPath, []byte(`
providers:
  anthropic:
    provider: anthropic
    api_key: env:HERMES_TEST_KEY
`), 0o644)
	require.NoError(t, err)

	cfg, err := LoadFromPath(yamlPath)
	require.NoError(t, err)
	assert.Equal(t, "sk-from-env", cfg.Providers["anthropic"].APIKey)
}

func TestEnvVarExpansionRejectsEmpty(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(yamlPath, []byte(`
providers:
  anthropic:
    provider: anthropic
    api_key: "env:"
`), 0o644)
	require.NoError(t, err)

	_, err = LoadFromPath(yamlPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty env variable")
}
