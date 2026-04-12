package rl

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Tokenizer          string  `yaml:"tokenizer"`
	MaxWorkers         int     `yaml:"max_workers"`
	LoraRank           int     `yaml:"lora_rank"`
	LearningRate       float64 `yaml:"learning_rate"`
	CheckpointInterval int     `yaml:"checkpoint_interval"`
	WandbName          string  `yaml:"wandb_name"`
	Environment        string  `yaml:"environment"`
}

var lockedFields = map[string]bool{
	"tokenizer":   true,
	"max_workers": true,
}

func DefaultConfig() *Config {
	return &Config{
		Tokenizer:          "Qwen/Qwen3-8B",
		MaxWorkers:         2048,
		LoraRank:           32,
		LearningRate:       0.00004,
		CheckpointInterval: 100,
	}
}

func (c *Config) IsLocked(field string) bool {
	return lockedFields[field]
}

func (c *Config) Set(field string, value any) error {
	if c.IsLocked(field) {
		return fmt.Errorf("field %q is locked and cannot be modified", field)
	}
	switch field {
	case "lora_rank":
		v, ok := toInt(value)
		if !ok {
			return fmt.Errorf("lora_rank must be an integer")
		}
		c.LoraRank = v
	case "learning_rate":
		v, ok := toFloat(value)
		if !ok {
			return fmt.Errorf("learning_rate must be a number")
		}
		c.LearningRate = v
	case "checkpoint_interval":
		v, ok := toInt(value)
		if !ok {
			return fmt.Errorf("checkpoint_interval must be an integer")
		}
		c.CheckpointInterval = v
	case "wandb_name":
		v, ok := value.(string)
		if !ok {
			return fmt.Errorf("wandb_name must be a string")
		}
		c.WandbName = v
	case "environment":
		v, ok := value.(string)
		if !ok {
			return fmt.Errorf("environment must be a string")
		}
		c.Environment = v
	default:
		return fmt.Errorf("unknown field: %q", field)
	}
	return nil
}

func (c *Config) Validate() error {
	if c.LoraRank <= 0 {
		return fmt.Errorf("lora_rank must be positive, got %d", c.LoraRank)
	}
	if c.LearningRate <= 0 {
		return fmt.Errorf("learning_rate must be positive, got %f", c.LearningRate)
	}
	if c.CheckpointInterval <= 0 {
		return fmt.Errorf("checkpoint_interval must be positive, got %d", c.CheckpointInterval)
	}
	return nil
}

func (c *Config) Save(path string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	defaults := DefaultConfig()
	cfg.Tokenizer = defaults.Tokenizer
	cfg.MaxWorkers = defaults.MaxWorkers
	return cfg, nil
}

func toInt(v any) (int, bool) {
	switch val := v.(type) {
	case int:
		return val, true
	case int64:
		return int(val), true
	case float64:
		return int(val), true
	default:
		return 0, false
	}
}

func toFloat(v any) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	default:
		return 0, false
	}
}
