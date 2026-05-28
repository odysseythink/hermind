package services

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

type AgentFlowService struct {
	flowsDir string
}

type FlowConfig struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Active      bool   `json:"active,omitempty"`
	Steps       []any  `json:"steps,omitempty"`
}

type FlowSummary struct {
	Name        string `json:"name"`
	UUID        string `json:"uuid"`
	Description string `json:"description,omitempty"`
	Active      bool   `json:"active"`
}

type LoadedFlow struct {
	Name   string     `json:"name"`
	UUID   string     `json:"uuid"`
	Config FlowConfig `json:"config"`
}

func NewAgentFlowService(storageDir string) *AgentFlowService {
	flowsDir := filepath.Join(storageDir, "plugins", "agent-flows")
	_ = os.MkdirAll(flowsDir, 0755)
	return &AgentFlowService{flowsDir: flowsDir}
}

func (s *AgentFlowService) ListFlows() ([]FlowSummary, error) {
	entries, err := os.ReadDir(s.flowsDir)
	if err != nil {
		return nil, err
	}
	flows := make([]FlowSummary, 0)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".json")
		data, err := os.ReadFile(filepath.Join(s.flowsDir, entry.Name()))
		if err != nil {
			continue
		}
		var cfg FlowConfig
		if err := json.Unmarshal(data, &cfg); err != nil {
			continue
		}
		flows = append(flows, FlowSummary{
			Name:        cfg.Name,
			UUID:        id,
			Description: cfg.Description,
			Active:      cfg.Active,
		})
	}
	return flows, nil
}

func (s *AgentFlowService) LoadFlow(flowUUID string) (*LoadedFlow, error) {
	if flowUUID == "" {
		return nil, fmt.Errorf("uuid required")
	}
	path := filepath.Join(s.flowsDir, flowUUID+".json")
	if !strings.HasPrefix(filepath.Clean(path), filepath.Clean(s.flowsDir)) {
		return nil, fmt.Errorf("invalid uuid")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg FlowConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &LoadedFlow{
		Name:   cfg.Name,
		UUID:   flowUUID,
		Config: cfg,
	}, nil
}

func (s *AgentFlowService) SaveFlow(name string, config FlowConfig, flowUUID string) (string, error) {
	if flowUUID == "" {
		flowUUID = uuid.New().String()
	}
	path := filepath.Join(s.flowsDir, flowUUID+".json")
	if !strings.HasPrefix(filepath.Clean(path), filepath.Clean(s.flowsDir)) {
		return "", fmt.Errorf("invalid uuid")
	}
	config.Name = name
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return "", err
	}
	return flowUUID, nil
}

func (s *AgentFlowService) DeleteFlow(flowUUID string) error {
	path := filepath.Join(s.flowsDir, flowUUID+".json")
	if !strings.HasPrefix(filepath.Clean(path), filepath.Clean(s.flowsDir)) {
		return fmt.Errorf("invalid uuid")
	}
	return os.Remove(path)
}
