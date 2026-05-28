package agent

import "github.com/odysseythink/hermind/backend/internal/models"

const defaultSystemPrompt = `You are a helpful AI assistant. You can use available tools to answer the user's questions.`

func resolveSystemPrompt(ws *models.Workspace, user *models.User) string {
	if ws != nil && ws.OpenAiPrompt != nil && *ws.OpenAiPrompt != "" {
		return *ws.OpenAiPrompt
	}
	return defaultSystemPrompt
}

// ResolveSystemPromptForTesting exposes resolveSystemPrompt for unit tests.
func ResolveSystemPromptForTesting(ws *models.Workspace, user *models.User) string {
	return resolveSystemPrompt(ws, user)
}
