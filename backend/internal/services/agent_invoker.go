package services

import (
	"context"

	"github.com/odysseythink/hermind/backend/internal/models"
)

// AgentHandoff carries the UUID and one-time WS token needed to dial the agent runtime.
type AgentHandoff struct {
	UUID    string
	WSToken string
}

// AgentInvoker is the narrow interface ChatService uses to handle @agent triggers.
// Implemented by agent.Runtime. Kept in services/ to avoid making chat_service.go
// import the agent package directly.
type AgentInvoker interface {
	IsAgentInvocation(ctx context.Context, ws *models.Workspace, message string) (bool, error)
	PrepareInvocationHandoff(ctx context.Context, ws *models.Workspace, user *models.User, thread *models.WorkspaceThread, prompt string) (*AgentHandoff, error)
}
