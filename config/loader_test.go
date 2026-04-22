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

func TestLoadFromYAMLParsesMCPServers(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(yamlPath, []byte(`
mcp:
  servers:
    github:
      command: npx
      args: [-y, "@modelcontextprotocol/server-github"]
      env:
        GITHUB_PERSONAL_ACCESS_TOKEN: gh-secret-literal
    filesystem:
      command: npx
      args: [-y, "@modelcontextprotocol/server-filesystem", "/tmp"]
      enabled: false
`), 0o644)
	require.NoError(t, err)

	cfg, err := LoadFromPath(yamlPath)
	require.NoError(t, err)
	require.Contains(t, cfg.MCP.Servers, "github")
	assert.Equal(t, "npx", cfg.MCP.Servers["github"].Command)
	assert.Equal(t, "gh-secret-literal", cfg.MCP.Servers["github"].Env["GITHUB_PERSONAL_ACCESS_TOKEN"])
	assert.True(t, cfg.MCP.Servers["github"].IsEnabled())

	require.Contains(t, cfg.MCP.Servers, "filesystem")
	assert.False(t, cfg.MCP.Servers["filesystem"].IsEnabled())
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

func TestLoadFromPath_Skills(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	yaml := []byte(`
model: anthropic/claude-opus-4-6
providers: {}
agent:
  max_turns: 10
terminal:
  backend: local
storage:
  driver: sqlite
skills:
  disabled:
    - foo
    - bar
  platform_disabled:
    cli: [baz]
    gateway: [qux]
`)
	if err := os.WriteFile(path, yaml, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadFromPath(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(cfg.Skills.Disabled) != 2 || cfg.Skills.Disabled[0] != "foo" {
		t.Errorf("disabled = %v", cfg.Skills.Disabled)
	}
	if got := cfg.Skills.PlatformDisabled["cli"]; len(got) != 1 || got[0] != "baz" {
		t.Errorf("platform_disabled[cli] = %v", got)
	}
}

func TestLoadWebSearchConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(path, []byte(`
web:
  search:
    provider: tavily
    providers:
      tavily:
        api_key: "tav-123"
      brave:
        api_key: "brv-456"
      exa:
        api_key: "exa-789"
`), 0o600)
	require.NoError(t, err)
	cfg, err := LoadFromPath(path)
	require.NoError(t, err)
	assert.Equal(t, "tavily", cfg.Web.Search.Provider)
	assert.Equal(t, "tav-123", cfg.Web.Search.Providers.Tavily.APIKey)
	assert.Equal(t, "brv-456", cfg.Web.Search.Providers.Brave.APIKey)
	assert.Equal(t, "exa-789", cfg.Web.Search.Providers.Exa.APIKey)
}

func TestLoad_AgentDefaultSystemPrompt(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(yamlPath, []byte(`
agent:
  max_turns: 10
  default_system_prompt: "You are a sardonic assistant."
`), 0o644)
	if err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	cfg, err := LoadFromPath(yamlPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got, want := cfg.Agent.DefaultSystemPrompt, "You are a sardonic assistant."; got != want {
		t.Errorf("DefaultSystemPrompt = %q, want %q", got, want)
	}
}

func TestLoadPreservesLiteralEnvString(t *testing.T) {
	// After dropping env:VAR expansion, a config value that happens to start
	// with "env:" must round-trip as a literal string, not trigger lookup.
	t.Setenv("HERMIND_TEST_KEY", "should-not-be-used")
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(yamlPath, []byte(`
providers:
  anthropic:
    provider: anthropic
    api_key: env:HERMIND_TEST_KEY
`), 0o644)
	require.NoError(t, err)

	cfg, err := LoadFromPath(yamlPath)
	require.NoError(t, err)
	assert.Equal(t, "env:HERMIND_TEST_KEY", cfg.Providers["anthropic"].APIKey)
}
