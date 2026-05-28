package agent_test

import (
	"testing"

	"github.com/odysseythink/hermind/backend/internal/agent"
	"github.com/stretchr/testify/require"
)

func TestSupportsNativeToolCalling_KnownProvider(t *testing.T) {
	require.True(t, agent.SupportsNativeToolCallingForTesting("openai"))
	require.True(t, agent.SupportsNativeToolCallingForTesting("anthropic"))
	require.True(t, agent.SupportsNativeToolCallingForTesting("ollama"))
	require.True(t, agent.SupportsNativeToolCallingForTesting("groq"))
	require.True(t, agent.SupportsNativeToolCallingForTesting("mistral"))
	require.True(t, agent.SupportsNativeToolCallingForTesting("google"))
	require.True(t, agent.SupportsNativeToolCallingForTesting("deepseek"))
	require.True(t, agent.SupportsNativeToolCallingForTesting("openrouter"))
}

func TestSupportsNativeToolCalling_UnknownProvider_False(t *testing.T) {
	require.False(t, agent.SupportsNativeToolCallingForTesting("xai"))
	require.False(t, agent.SupportsNativeToolCallingForTesting("localai"))
	require.False(t, agent.SupportsNativeToolCallingForTesting(""))
}

func TestSupportsNativeToolCalling_CaseInsensitive(t *testing.T) {
	require.True(t, agent.SupportsNativeToolCallingForTesting("OpenAI"))
	require.True(t, agent.SupportsNativeToolCallingForTesting("OLLAMA"))
	require.True(t, agent.SupportsNativeToolCallingForTesting("Anthropic"))
}
