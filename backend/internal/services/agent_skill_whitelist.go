package services

import (
	"context"
	"encoding/json"
	"fmt"
)

// AgentSkillWhitelistService manages per-user skill whitelists stored as
// JSON arrays in SystemSetting.
type AgentSkillWhitelistService struct {
	sysSvc *SystemService
}

// NewAgentSkillWhitelistService creates a new whitelist service.
func NewAgentSkillWhitelistService(sysSvc *SystemService) *AgentSkillWhitelistService {
	return &AgentSkillWhitelistService{sysSvc: sysSvc}
}

func (s *AgentSkillWhitelistService) label(userID *int) string {
	if userID == nil || *userID == 0 {
		return "whitelisted_agent_skills"
	}
	return fmt.Sprintf("user_%d_whitelisted_agent_skills", *userID)
}

// Get returns the whitelist for the given user.
func (s *AgentSkillWhitelistService) Get(ctx context.Context, userID *int) ([]string, error) {
	raw, err := s.sysSvc.GetSetting(ctx, s.label(userID))
	if err != nil || raw == "" {
		return nil, nil
	}
	var arr []string
	if err := json.Unmarshal([]byte(raw), &arr); err != nil {
		return nil, nil
	}
	return arr, nil
}

// Add inserts a skill into the whitelist (idempotent).
func (s *AgentSkillWhitelistService) Add(ctx context.Context, userID *int, skill string) error {
	if skill == "" {
		return fmt.Errorf("skill name required")
	}
	list, _ := s.Get(ctx, userID)
	for _, x := range list {
		if x == skill {
			return nil
		}
	}
	list = append(list, skill)
	raw, _ := json.Marshal(list)
	return s.sysSvc.SetSetting(ctx, s.label(userID), string(raw))
}

// Remove deletes a skill from the whitelist.
func (s *AgentSkillWhitelistService) Remove(ctx context.Context, userID *int, skill string) error {
	list, _ := s.Get(ctx, userID)
	out := make([]string, 0, len(list))
	for _, x := range list {
		if x != skill {
			out = append(out, x)
		}
	}
	raw, _ := json.Marshal(out)
	return s.sysSvc.SetSetting(ctx, s.label(userID), string(raw))
}

// IsWhitelisted checks if a skill is in the whitelist.
func (s *AgentSkillWhitelistService) IsWhitelisted(ctx context.Context, userID *int, skill string) bool {
	list, _ := s.Get(ctx, userID)
	for _, x := range list {
		if x == skill {
			return true
		}
	}
	return false
}

// ClearSingleUser resets the single-user (non-multi-user) whitelist.
func (s *AgentSkillWhitelistService) ClearSingleUser(ctx context.Context) error {
	return s.sysSvc.SetSetting(ctx, "whitelisted_agent_skills", "[]")
}
