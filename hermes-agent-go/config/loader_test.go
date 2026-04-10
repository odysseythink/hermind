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

func TestLoadFromYAMLParsesTerminalConfig(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(yamlPath, []byte(`
terminal:
  backend: docker
  cwd: /workspace
  timeout: 120
  docker_image: golang:1.25-alpine
  docker_volumes:
    - /host/src:/workspace
  ssh_host: dev.example.com
  ssh_user: dev
  ssh_key: /home/me/.ssh/id_ed25519
  modal_base_url: https://api.modal.com/v1
  modal_token: test-modal-token
  daytona_base_url: https://api.daytona.io/v1
  daytona_token: test-daytona-token
  singularity_image: /opt/img/ubuntu.sif
`), 0o644)
	require.NoError(t, err)

	cfg, err := LoadFromPath(yamlPath)
	require.NoError(t, err)
	assert.Equal(t, "docker", cfg.Terminal.Backend)
	assert.Equal(t, "/workspace", cfg.Terminal.Cwd)
	assert.Equal(t, 120, cfg.Terminal.Timeout)
	assert.Equal(t, "golang:1.25-alpine", cfg.Terminal.DockerImage)
	assert.Equal(t, []string{"/host/src:/workspace"}, cfg.Terminal.DockerVolumes)
	assert.Equal(t, "dev.example.com", cfg.Terminal.SSHHost)
	assert.Equal(t, "dev", cfg.Terminal.SSHUser)
	assert.Equal(t, "https://api.modal.com/v1", cfg.Terminal.ModalBaseURL)
	assert.Equal(t, "test-modal-token", cfg.Terminal.ModalToken)
	assert.Equal(t, "https://api.daytona.io/v1", cfg.Terminal.DaytonaBaseURL)
	assert.Equal(t, "/opt/img/ubuntu.sif", cfg.Terminal.SingularityImage)
}

func TestLoadFromYAMLParsesFallbackProviders(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(yamlPath, []byte(`
model: anthropic/claude-opus-4-6
providers:
  anthropic:
    provider: anthropic
    api_key: sk-anthropic
    model: claude-opus-4-6
fallback_providers:
  - provider: deepseek
    api_key: sk-deepseek
    model: deepseek-chat
  - provider: openai
    api_key: sk-openai
    model: gpt-4o
`), 0o644)
	require.NoError(t, err)

	cfg, err := LoadFromPath(yamlPath)
	require.NoError(t, err)
	require.Len(t, cfg.FallbackProviders, 2)
	assert.Equal(t, "deepseek", cfg.FallbackProviders[0].Provider)
	assert.Equal(t, "sk-deepseek", cfg.FallbackProviders[0].APIKey)
	assert.Equal(t, "openai", cfg.FallbackProviders[1].Provider)
}
