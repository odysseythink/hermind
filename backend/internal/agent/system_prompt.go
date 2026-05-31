package agent

import (
	"strings"

	"github.com/odysseythink/hermind/backend/internal/models"
)

const defaultSystemPrompt = `You are a helpful AI assistant. You can use available tools to answer the user's questions.`

func resolveSystemPrompt(ws *models.Workspace, user *models.User, skills []models.AgentSkill) string {
	var sb strings.Builder
	if ws != nil && ws.OpenAiPrompt != nil && *ws.OpenAiPrompt != "" {
		sb.WriteString(*ws.OpenAiPrompt)
	} else {
		sb.WriteString(defaultSystemPrompt)
	}

	if len(skills) > 0 {
		sb.WriteString("\n\n## Available Skills\n")
		sb.WriteString("You have access to the following skills in this workspace. Load a skill with skill_view(name) when relevant.\n\n")
		for _, s := range skills {
			if s.Description != "" {
				sb.WriteString("- " + s.Name + " — " + s.Description + "\n")
			} else {
				sb.WriteString("- " + s.Name + "\n")
			}
		}
	}

	return sb.String()
}

// ResolveSystemPromptForTesting exposes resolveSystemPrompt for unit tests.
func ResolveSystemPromptForTesting(ws *models.Workspace, user *models.User) string {
	return resolveSystemPrompt(ws, user, nil)
}
