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
