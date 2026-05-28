package agent_test

import (
	"context"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/agent"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/pkg/utils"
	"github.com/stretchr/testify/require"
)

func TestParseAgentHandles_AtAgentPrefix_Detected(t *testing.T) {
	handles := agent.ParseAgentHandlesForTesting("@agent help me")
	require.Len(t, handles, 1)
	require.Equal(t, "@agent", handles[0])
}

func TestParseAgentHandles_NoPrefix_Empty(t *testing.T) {
	require.Empty(t, agent.ParseAgentHandlesForTesting("hello world"))
}

func TestParseAgentHandles_MidSentenceAt_Ignored(t *testing.T) {
	// Only leading @agent counts; mid-sentence @agent is ignored.
	require.Empty(t, agent.ParseAgentHandlesForTesting("hi @agent in the middle"))
}

func TestParseAgentHandles_LeadingWhitespace_Tolerated(t *testing.T) {
	handles := agent.ParseAgentHandlesForTesting("  \t@agent do this")
	require.Len(t, handles, 1)
	require.Equal(t, "@agent", handles[0])
}

func TestParseAgentHandles_MultipleHandles(t *testing.T) {
	handles := agent.ParseAgentHandlesForTesting("@agent @research find docs")
	require.Len(t, handles, 2)
	require.Equal(t, "@agent", handles[0])
	require.Equal(t, "@research", handles[1])
}

func TestIsAgentInvocation_AtAgentMessage_True(t *testing.T) {
	env := newAgentTestEnv(t)
	ws := seedWorkspace(t, env.DB)
	got, err := env.Runtime.IsAgentInvocation(context.Background(), ws, "@agent help me")
	require.NoError(t, err)
	require.True(t, got)
}

func TestIsAgentInvocation_AutomaticModeNativeProvider_True(t *testing.T) {
	env := newAgentTestEnv(t)
	ws := &models.Workspace{
		ChatMode:      utils.Ptr("automatic"),
		AgentProvider: utils.Ptr("openai"),
		AgentModel:    utils.Ptr("gpt-4o-mini"),
	}
	require.NoError(t, env.DB.Create(ws).Error)
	got, err := env.Runtime.IsAgentInvocation(context.Background(), ws, "what time is it?")
	require.NoError(t, err)
	require.True(t, got)
}

func TestIsAgentInvocation_AutomaticModeUnknownProvider_False(t *testing.T) {
	env := newAgentTestEnv(t)
	ws := &models.Workspace{
		ChatMode:      utils.Ptr("automatic"),
		AgentProvider: utils.Ptr("xai"),
	}
	require.NoError(t, env.DB.Create(ws).Error)
	got, err := env.Runtime.IsAgentInvocation(context.Background(), ws, "what time is it?")
	require.NoError(t, err)
	require.False(t, got)
}

func TestIsAgentInvocation_ChatModeNoPrefix_False(t *testing.T) {
	env := newAgentTestEnv(t)
	ws := &models.Workspace{
		ChatMode:      utils.Ptr("chat"),
		AgentProvider: utils.Ptr("openai"),
	}
	require.NoError(t, env.DB.Create(ws).Error)
	got, err := env.Runtime.IsAgentInvocation(context.Background(), ws, "what time is it?")
	require.NoError(t, err)
	require.False(t, got)
}

func TestIsAgentInvocation_NilWorkspace_False(t *testing.T) {
	env := newAgentTestEnv(t)
	got, err := env.Runtime.IsAgentInvocation(context.Background(), nil, "@agent hi")
	require.NoError(t, err)
	require.False(t, got)
}

func TestIsAgentInvocation_FallsBackToChatProvider(t *testing.T) {
	env := newAgentTestEnv(t)
	ws := &models.Workspace{
		ChatMode:     utils.Ptr("automatic"),
		ChatProvider: utils.Ptr("anthropic"),
	}
	require.NoError(t, env.DB.Create(ws).Error)
	got, err := env.Runtime.IsAgentInvocation(context.Background(), ws, "hello")
	require.NoError(t, err)
	require.True(t, got)
}

func TestIsAgentInvocation_FallsBackToCfgProvider(t *testing.T) {
	env := newAgentTestEnv(t)
	env.Cfg.LLMProvider = "ollama"
	ws := &models.Workspace{
		ChatMode: utils.Ptr("automatic"),
	}
	require.NoError(t, env.DB.Create(ws).Error)
	got, err := env.Runtime.IsAgentInvocation(context.Background(), ws, "hello")
	require.NoError(t, err)
	require.True(t, got)
}

func TestHandoff_HappyPath_ReturnsUUIDAndToken(t *testing.T) {
	env := newAgentTestEnv(t)
	ws := seedWorkspace(t, env.DB)
	ho, err := env.Runtime.PrepareInvocationHandoff(context.Background(), ws, env.User, nil, "@agent hi")
	require.NoError(t, err)
	require.NotEmpty(t, ho.UUID)
	require.NotEmpty(t, ho.WSToken)
	require.NotEqual(t, "AUTH_DISABLED_BYPASS", ho.WSToken)
}

func TestHandoff_AuthDisabled_ReturnsSentinelToken(t *testing.T) {
	env := newAgentTestEnv(t)
	ws := seedWorkspace(t, env.DB)
	ho, err := env.Runtime.PrepareInvocationHandoff(context.Background(), ws, nil, nil, "@agent hi")
	require.NoError(t, err)
	require.NotEmpty(t, ho.UUID)
	require.Equal(t, "AUTH_DISABLED_BYPASS", ho.WSToken)
}

func TestHandoff_TempTokenFailure_RollsBackInvocation(t *testing.T) {
	env := newAgentTestEnv(t)
	ws := seedWorkspace(t, env.DB)
	// Drop the temp-token table so IssueWithTTL fails, triggering rollback.
	require.NoError(t, env.DB.Migrator().DropTable(&models.TemporaryAuthToken{}))
	_, err := env.Runtime.PrepareInvocationHandoff(context.Background(), ws, env.User, nil, "@agent hi")
	require.Error(t, err)

	// Verify NO orphan invocation row
	var count int64
	env.DB.Model(&models.WorkspaceAgentInvocation{}).Count(&count)
	require.Equal(t, int64(0), count)
}
