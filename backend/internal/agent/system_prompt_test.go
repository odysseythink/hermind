package agent_test

import (
	"testing"

	"github.com/odysseythink/hermind/backend/internal/agent"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/pkg/utils"
	"github.com/stretchr/testify/require"
)

func TestSystemPrompt_WorkspaceOverride(t *testing.T) {
	ws := &models.Workspace{OpenAiPrompt: utils.Ptr("You are pirate Bob.")}
	got := agent.ResolveSystemPromptForTesting(ws, nil)
	require.Equal(t, "You are pirate Bob.", got)
}

func TestSystemPrompt_FallbackDefault(t *testing.T) {
	ws := &models.Workspace{}
	got := agent.ResolveSystemPromptForTesting(ws, nil)
	require.Contains(t, got, "helpful")
}
