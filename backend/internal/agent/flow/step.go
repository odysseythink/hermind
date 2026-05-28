package flow

import (
	"encoding/json"
	"fmt"
)

// Step represents a single block in an agent flow.
type Step struct {
	Type      string         `json:"type"`
	Config    map[string]any `json:"config"`
	ResultVar string         // extracted from config.resultVariable / config.responseVariable
}

// ParseStep converts a raw JSON value into a Step.
func ParseStep(raw any) (*Step, error) {
	blob, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("marshal step: %w", err)
	}
	var s Step
	if err := json.Unmarshal(blob, &s); err != nil {
		return nil, fmt.Errorf("unmarshal step: %w", err)
	}
	if s.Type == "" {
		return nil, fmt.Errorf("step missing type")
	}
	if cfg := s.Config; cfg != nil {
		if v, _ := cfg["resultVariable"].(string); v != "" {
			s.ResultVar = v
		}
		if v, _ := cfg["responseVariable"].(string); v != "" {
			s.ResultVar = v
		}
	}
	return &s, nil
}
