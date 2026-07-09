package agent_test

import (
	"testing"

	"github.com/odysseythink/hermind/backend/internal/agent"
	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/stretchr/testify/require"
)

func TestLLMFactory_Ollama_BuildsModel(t *testing.T) {
	cfg := &config.Config{LLMProvider: "ollama", LLMModel: "llama3"}
	ws := &models.Workspace{}
	lm, err := agent.BuildLanguageModelForTesting(ws, map[string]string{
		"LLMProvider":        "ollama",
		"OllamaLLMBasePath":  "http://127.0.0.1:11434",
		"OllamaLLMModelPref": "llama3",
	}, cfg)
	require.NoError(t, err)
	require.Equal(t, "ollama", lm.Provider())
	require.Equal(t, "llama3", lm.Model())
}

func TestLLMFactory_OpenAI_BuildsModel(t *testing.T) {
	cfg := &config.Config{LLMProvider: "openai", LLMModel: "gpt-4o", LLMApiKey: "sk-test"}
	ws := &models.Workspace{}
	lm, err := agent.BuildLanguageModelForTesting(ws, map[string]string{
		"LLMProvider":     "openai",
		"OpenAiModelPref": "gpt-4o",
		"LLMApiKey":       "sk-test",
	}, cfg)
	require.NoError(t, err)
	require.Equal(t, "openai", lm.Provider())
	require.Equal(t, "gpt-4o", lm.Model())
}

func TestLLMFactory_NoAPIKey_ReturnsError(t *testing.T) {
	cfg := &config.Config{LLMProvider: "openai", LLMModel: "gpt-4o"}
	ws := &models.Workspace{}
	_, err := agent.BuildLanguageModelForTesting(ws, map[string]string{
		"LLMProvider": "openai",
	}, cfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no LLM API key")
}

func TestLLMFactory_KeyPriority_DBSpecificBeatsEnvGeneric(t *testing.T) {
	cfg := &config.Config{
		LLMProvider: "openai",
		LLMModel:    "gpt-4o",
		LLMApiKey:   "env-generic-key",
	}
	ws := &models.Workspace{}
	// DB-specific OpenAiKey should beat env-generic LLMApiKey
	lm, err := agent.BuildLanguageModelForTesting(ws, map[string]string{
		"LLMProvider": "openai",
		"OpenAiKey":   "db-specific-key",
	}, cfg)
	require.NoError(t, err)
	require.NotNil(t, lm)
	require.Equal(t, "openai", lm.Provider())
}

func TestLLMFactory_WorkspaceOverride_PreferredOverGlobal(t *testing.T) {
	cfg := &config.Config{LLMProvider: "openai", LLMModel: "gpt-4o", LLMApiKey: "sk-test"}
	ws := &models.Workspace{}
	lm, err := agent.BuildLanguageModelForTesting(ws, map[string]string{
		"LLMProvider":     "openai",
		"OpenAiModelPref": "gpt-4o-mini",
		"LLMApiKey":       "sk-test",
	}, cfg)
	require.NoError(t, err)
	require.Equal(t, "gpt-4o-mini", lm.Model())
}

func TestRuntime_LanguageModelFor_CachesByProviderAndModel(t *testing.T) {
	env := newAgentTestEnv(t)
	ws := seedWorkspace(t, env.DB)

	// First call creates the model
	lm1, err := env.Runtime.LanguageModelFor(ws, map[string]string{
		"LLMProvider":        "ollama",
		"OllamaLLMModelPref": "llama3",
		"OllamaLLMBasePath":  "http://127.0.0.1:11434",
	})
	require.NoError(t, err)
	require.NotNil(t, lm1)

	// Second call should return the same cached instance
	lm2, err := env.Runtime.LanguageModelFor(ws, map[string]string{
		"LLMProvider":        "ollama",
		"OllamaLLMModelPref": "llama3",
		"OllamaLLMBasePath":  "http://127.0.0.1:11434",
	})
	require.NoError(t, err)
	require.Equal(t, lm1, lm2)
}
