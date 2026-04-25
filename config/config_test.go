package config

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestSkillsConfigGenerationHalfLifeDefaultZero(t *testing.T) {
	var cfg SkillsConfig
	require.Equal(t, 0, cfg.GenerationHalfLife)
}

func TestSkillsConfigGenerationHalfLifeYAML(t *testing.T) {
	yamlSrc := []byte(`generation_half_life: 5` + "\n")
	var cfg SkillsConfig
	require.NoError(t, yaml.Unmarshal(yamlSrc, &cfg))
	require.Equal(t, 5, cfg.GenerationHalfLife)
}

func TestProxyConfigDefaultDisabled(t *testing.T) {
	var cfg ProxyConfig
	require.False(t, cfg.Enabled)
	require.Equal(t, 0, cfg.KeepAliveSeconds)
}

func TestProxyConfigYAMLRoundTrip(t *testing.T) {
	yamlSrc := []byte(
		"proxy:\n" +
			"  enabled: true\n" +
			"  keep_alive_seconds: 30\n",
	)
	var cfg Config
	require.NoError(t, yaml.Unmarshal(yamlSrc, &cfg))
	require.True(t, cfg.Proxy.Enabled)
	require.Equal(t, 30, cfg.Proxy.KeepAliveSeconds)
}
