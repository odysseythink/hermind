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

func TestPresenceConfigDefault(t *testing.T) {
	var cfg PresenceConfig
	require.Equal(t, 0, cfg.HTTPIdleAbsentAfterSeconds)
	require.False(t, cfg.SleepWindow.Enabled)
}

func TestPresenceConfigYAMLRoundTrip(t *testing.T) {
	yamlSrc := []byte(
		"presence:\n" +
			"  http_idle_absent_after_seconds: 300\n" +
			"  sleep_window:\n" +
			"    enabled: true\n" +
			"    start: \"23:00\"\n" +
			"    end: \"07:00\"\n" +
			"    timezone: \"America/Los_Angeles\"\n",
	)
	var cfg Config
	require.NoError(t, yaml.Unmarshal(yamlSrc, &cfg))
	require.Equal(t, 300, cfg.Presence.HTTPIdleAbsentAfterSeconds)
	require.True(t, cfg.Presence.SleepWindow.Enabled)
	require.Equal(t, "23:00", cfg.Presence.SleepWindow.Start)
	require.Equal(t, "07:00", cfg.Presence.SleepWindow.End)
	require.Equal(t, "America/Los_Angeles", cfg.Presence.SleepWindow.Timezone)
}
