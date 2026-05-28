package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/odysseythink/hermind/backend/internal/middleware"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/services"
)

// ParseAgentHandlesForTesting wraps parseAgentHandles for tests.
func ParseAgentHandlesForTesting(s string) []string { return parseAgentHandles(s) }

func parseAgentHandles(message string) []string {
	msg := strings.TrimLeft(message, " \t\n\r")
	if !strings.HasPrefix(msg, "@agent") {
		return nil
	}
	out := []string{}
	for _, tok := range strings.Fields(msg) {
		if strings.HasPrefix(tok, "@") {
			out = append(out, tok)
		}
	}
	return out
}

// IsAgentInvocation returns true when:
//  1. message starts with "@agent", OR
//  2. workspace.chatMode == "automatic" AND the workspace's agent provider supports native tool calling.
func (r *Runtime) IsAgentInvocation(ctx context.Context, ws *models.Workspace, message string) (bool, error) {
	if ws == nil {
		return false, nil
	}
	if len(parseAgentHandles(message)) > 0 {
		return true, nil
	}

	mode := ""
	if ws.ChatMode != nil {
		mode = *ws.ChatMode
	}
	if mode != "automatic" {
		return false, nil
	}

	provider := ""
	switch {
	case ws.AgentProvider != nil && *ws.AgentProvider != "":
		provider = *ws.AgentProvider
	case ws.ChatProvider != nil && *ws.ChatProvider != "":
		provider = *ws.ChatProvider
	default:
		provider = r.deps.Cfg.LLMProvider
	}
	return supportsNativeToolCalling(provider), nil
}

// PrepareInvocationHandoff creates a WorkspaceAgentInvocation row and issues a
// single-use 3-minute WS token. If token issuance fails the invocation row is
// deleted so we never orphan a "ready" UUID that can't be authenticated.
func (r *Runtime) PrepareInvocationHandoff(ctx context.Context, ws *models.Workspace, user *models.User, thread *models.WorkspaceThread, prompt string) (*services.AgentHandoff, error) {
	if user == nil || user.ID == 0 {
		// Auth-disabled mode: hand back the sentinel that WSValidatedRequest accepts.
		uid, err := r.CreateInvocation(ctx, ws, nil, thread, prompt)
		if err != nil {
			return nil, err
		}
		return &services.AgentHandoff{UUID: uid, WSToken: middleware.AuthDisabledBypassToken}, nil
	}

	uid, err := r.CreateInvocation(ctx, ws, user, thread, prompt)
	if err != nil {
		return nil, fmt.Errorf("create invocation: %w", err)
	}

	tok, err := r.deps.TempTokenSvc.IssueWithTTL(ctx, user.ID, 3*time.Minute)
	if err != nil {
		_ = r.DeleteInvocation(ctx, uid) // rollback
		return nil, fmt.Errorf("issue WS token: %w", err)
	}
	return &services.AgentHandoff{UUID: uid, WSToken: tok}, nil
}
