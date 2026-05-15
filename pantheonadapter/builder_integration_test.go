package pantheonadapter

import (
	"context"
	"os"
	"testing"

	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/pantheon/core"
	"github.com/odysseythink/pantheon/extensions/fallback"
	"github.com/odysseythink/pantheon/extensions/retry"
)

// openAIIntegrationConfig reads OpenAI integration test configuration from environment variables.
func openAIIntegrationConfig(t *testing.T) (apiKey, model string) {
	t.Helper()
	apiKey = os.Getenv("OPENAI_API_KEY")
	model = os.Getenv("OPENAI_MODEL")
	if apiKey == "" {
		t.Skip("Skipping integration test: set OPENAI_API_KEY to run")
	}
	if model == "" {
		model = "gpt-4o-mini"
	}
	return apiKey, model
}

// anthropicIntegrationConfig reads Anthropic integration test configuration from environment variables.
func anthropicIntegrationConfig(t *testing.T) (apiKey, model string) {
	t.Helper()
	apiKey = os.Getenv("ANTHROPIC_API_KEY")
	model = os.Getenv("ANTHROPIC_MODEL")
	if apiKey == "" {
		t.Skip("Skipping integration test: set ANTHROPIC_API_KEY to run")
	}
	if model == "" {
		model = "claude-sonnet-4"
	}
	return apiKey, model
}

// deepseekIntegrationConfig reads DeepSeek integration test configuration from environment variables.
func deepseekIntegrationConfig(t *testing.T) (apiKey, model string) {
	t.Helper()
	apiKey = os.Getenv("DEEPSEEK_API_KEY")
	model = os.Getenv("DEEPSEEK_MODEL")
	if apiKey == "" {
		t.Skip("Skipping integration test: set DEEPSEEK_API_KEY to run")
	}
	if model == "" {
		model = "deepseek-chat"
	}
	return apiKey, model
}

// qwenIntegrationConfig reads Qwen integration test configuration from environment variables.
func qwenIntegrationConfig(t *testing.T) (apiKey, baseURL, model string) {
	t.Helper()
	apiKey = os.Getenv("QWEN_API_KEY")
	baseURL = os.Getenv("QWEN_BASE_URL")
	model = os.Getenv("QWEN_MODEL")
	if apiKey == "" {
		t.Skip("Skipping integration test: set QWEN_API_KEY to run")
	}
	if baseURL == "" {
		baseURL = "https://dashscope.aliyuncs.com/compatible-mode"
	}
	if model == "" {
		model = "qwen-turbo"
	}
	return apiKey, baseURL, model
}

// zhipuIntegrationConfig reads Zhipu integration test configuration from environment variables.
func zhipuIntegrationConfig(t *testing.T) (apiKey, baseURL, model string) {
	t.Helper()
	apiKey = os.Getenv("ZHIPU_API_KEY")
	baseURL = os.Getenv("ZHIPU_BASE_URL")
	model = os.Getenv("ZHIPU_MODEL")
	if apiKey == "" {
		t.Skip("Skipping integration test: set ZHIPU_API_KEY to run")
	}
	if baseURL == "" {
		baseURL = "https://open.bigmodel.cn/api/paas"
	}
	if model == "" {
		model = "glm-4-flash"
	}
	return apiKey, baseURL, model
}

// kimiIntegrationConfig reads Kimi integration test configuration from environment variables.
func kimiIntegrationConfig(t *testing.T) (apiKey, baseURL, model string) {
	t.Helper()
	apiKey = os.Getenv("KIMI_API_KEY")
	baseURL = os.Getenv("KIMI_BASE_URL")
	model = os.Getenv("KIMI_MODEL")
	if apiKey == "" {
		t.Skip("Skipping integration test: set KIMI_API_KEY to run")
	}
	if baseURL == "" {
		baseURL = "https://api.moonshot.cn/v1"
	}
	if model == "" {
		model = "moonshot-v1-8k"
	}
	return apiKey, baseURL, model
}

// openaiCompatIntegrationConfig reads generic OpenAI-compatible integration test configuration from environment variables.
func openaiCompatIntegrationConfig(t *testing.T) (apiKey, baseURL, model string) {
	t.Helper()
	apiKey = os.Getenv("OPENAICOMPAT_API_KEY")
	baseURL = os.Getenv("OPENAICOMPAT_BASE_URL")
	model = os.Getenv("OPENAICOMPAT_MODEL")
	if apiKey == "" {
		t.Skip("Skipping integration test: set OPENAICOMPAT_API_KEY to run")
	}
	if baseURL == "" {
		t.Skip("Skipping integration test: set OPENAICOMPAT_BASE_URL to run")
	}
	if model == "" {
		t.Skip("Skipping integration test: set OPENAICOMPAT_MODEL to run")
	}
	return apiKey, baseURL, model
}

func TestIntegration_BuildModel_OpenAI_Generate(t *testing.T) {
	apiKey, model := openAIIntegrationConfig(t)

	lm, err := BuildModel(context.Background(), config.ProviderConfig{
		Provider: "openai",
		APIKey:   apiKey,
		Model:    model,
	})
	if err != nil {
		t.Fatalf("BuildModel error: %v", err)
	}

	resp, err := lm.Generate(context.Background(), &core.Request{
		Messages: []core.Message{
			{Role: core.MESSAGE_ROLE_USER, Content: []core.ContentParter{core.TextPart{Text: "Say hello in one word."}}},
		},
	})
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}
	if len(resp.Message.Content) == 0 {
		t.Fatal("expected content in response")
	}
	tp, ok := resp.Message.Content[0].(core.TextPart)
	if !ok {
		t.Fatalf("expected TextPart, got %T", resp.Message.Content[0])
	}
	if tp.Text == "" {
		t.Error("expected non-empty text response")
	}
	t.Logf("Response: %s", tp.Text)
	t.Logf("Usage: prompt=%d completion=%d total=%d", resp.Usage.PromptTokens, resp.Usage.CompletionTokens, resp.Usage.TotalTokens)
}

func TestIntegration_BuildModel_OpenAI_Stream(t *testing.T) {
	apiKey, model := openAIIntegrationConfig(t)

	lm, err := BuildModel(context.Background(), config.ProviderConfig{
		Provider: "openai",
		APIKey:   apiKey,
		Model:    model,
	})
	if err != nil {
		t.Fatalf("BuildModel error: %v", err)
	}

	stream, err := lm.Stream(context.Background(), &core.Request{
		Messages: []core.Message{
			{Role: core.MESSAGE_ROLE_USER, Content: []core.ContentParter{core.TextPart{Text: "Count from 1 to 3."}}},
		},
	})
	if err != nil {
		t.Fatalf("Stream init error: %v", err)
	}

	var textDeltas []string
	var finishReason string
	for part, err := range stream {
		if err != nil {
			t.Fatalf("stream error: %v", err)
		}
		switch part.Type {
		case core.StreamPartTypeTextDelta:
			textDeltas = append(textDeltas, part.TextDelta)
		case core.StreamPartTypeFinish:
			finishReason = part.FinishReason
		}
	}

	fullText := ""
	for _, d := range textDeltas {
		fullText += d
	}
	if fullText == "" {
		t.Error("expected non-empty streamed text")
	}
	if finishReason == "" {
		t.Error("expected non-empty finish reason")
	}
	t.Logf("Streamed text: %s", fullText)
	t.Logf("Finish reason: %s", finishReason)
}

func TestIntegration_BuildModel_Anthropic_Generate(t *testing.T) {
	apiKey, model := anthropicIntegrationConfig(t)

	lm, err := BuildModel(context.Background(), config.ProviderConfig{
		Provider: "anthropic",
		APIKey:   apiKey,
		Model:    model,
	})
	if err != nil {
		t.Fatalf("BuildModel error: %v", err)
	}

	resp, err := lm.Generate(context.Background(), &core.Request{
		Messages: []core.Message{
			{Role: core.MESSAGE_ROLE_USER, Content: []core.ContentParter{core.TextPart{Text: "Say hello in one word."}}},
		},
	})
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}
	if len(resp.Message.Content) == 0 {
		t.Fatal("expected content in response")
	}
	tp, ok := resp.Message.Content[0].(core.TextPart)
	if !ok {
		t.Fatalf("expected TextPart, got %T", resp.Message.Content[0])
	}
	if tp.Text == "" {
		t.Error("expected non-empty text response")
	}
	t.Logf("Response: %s", tp.Text)
	t.Logf("Usage: prompt=%d completion=%d total=%d", resp.Usage.PromptTokens, resp.Usage.CompletionTokens, resp.Usage.TotalTokens)
}

func TestIntegration_BuildModel_Anthropic_Stream(t *testing.T) {
	apiKey, model := anthropicIntegrationConfig(t)

	lm, err := BuildModel(context.Background(), config.ProviderConfig{
		Provider: "anthropic",
		APIKey:   apiKey,
		Model:    model,
	})
	if err != nil {
		t.Fatalf("BuildModel error: %v", err)
	}

	stream, err := lm.Stream(context.Background(), &core.Request{
		Messages: []core.Message{
			{Role: core.MESSAGE_ROLE_USER, Content: []core.ContentParter{core.TextPart{Text: "Count from 1 to 3."}}},
		},
	})
	if err != nil {
		t.Fatalf("Stream init error: %v", err)
	}

	var textDeltas []string
	var finishReason string
	for part, err := range stream {
		if err != nil {
			t.Fatalf("stream error: %v", err)
		}
		switch part.Type {
		case core.StreamPartTypeTextDelta:
			textDeltas = append(textDeltas, part.TextDelta)
		case core.StreamPartTypeFinish:
			finishReason = part.FinishReason
		}
	}

	fullText := ""
	for _, d := range textDeltas {
		fullText += d
	}
	if fullText == "" {
		t.Error("expected non-empty streamed text")
	}
	if finishReason == "" {
		t.Error("expected non-empty finish reason")
	}
	t.Logf("Streamed text: %s", fullText)
	t.Logf("Finish reason: %s", finishReason)
}

func TestIntegration_BuildModel_DeepSeek_Generate(t *testing.T) {
	apiKey, model := deepseekIntegrationConfig(t)

	lm, err := BuildModel(context.Background(), config.ProviderConfig{
		Provider: "deepseek",
		BaseURL:  "https://api.deepseek.com",
		APIKey:   apiKey,
		Model:    model,
	})
	if err != nil {
		t.Fatalf("BuildModel error: %v", err)
	}

	resp, err := lm.Generate(context.Background(), &core.Request{
		Messages: []core.Message{
			{Role: core.MESSAGE_ROLE_USER, Content: []core.ContentParter{core.TextPart{Text: "Say hello in one word."}}},
		},
	})
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}
	if len(resp.Message.Content) == 0 {
		t.Fatal("expected content in response")
	}
	tp, ok := resp.Message.Content[0].(core.TextPart)
	if !ok {
		t.Fatalf("expected TextPart, got %T", resp.Message.Content[0])
	}
	if tp.Text == "" {
		t.Error("expected non-empty text response")
	}
	t.Logf("Response: %s", tp.Text)
	t.Logf("Usage: prompt=%d completion=%d total=%d", resp.Usage.PromptTokens, resp.Usage.CompletionTokens, resp.Usage.TotalTokens)
}

func TestIntegration_BuildPrimaryModel(t *testing.T) {
	apiKey, model := openAIIntegrationConfig(t)

	lm, err := BuildPrimaryModel(context.Background(), config.ProviderConfig{
		Provider: "openai",
		APIKey:   apiKey,
		Model:    model,
	})
	if err != nil {
		t.Fatalf("BuildPrimaryModel error: %v", err)
	}

	// Verify it is wrapped with retry.
	rm, ok := lm.(*retry.Model)
	if !ok {
		t.Fatalf("expected *retry.Model, got %T", lm)
	}
	if rm.MaxRetries != 3 {
		t.Errorf("MaxRetries = %d, want 3", rm.MaxRetries)
	}

	// Verify it can actually generate.
	resp, err := lm.Generate(context.Background(), &core.Request{
		Messages: []core.Message{
			{Role: core.MESSAGE_ROLE_USER, Content: []core.ContentParter{core.TextPart{Text: "Say hi in one word."}}},
		},
	})
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}
	if len(resp.Message.Content) == 0 {
		t.Fatal("expected content in response")
	}
	t.Logf("Response: %+v", resp.Message.Content[0])
}

func TestIntegration_BuildFallbackModel(t *testing.T) {
	primaryAPIKey, primaryModel := openAIIntegrationConfig(t)
	fallbackAPIKey, fallbackModel := anthropicIntegrationConfig(t)

	lm, err := BuildFallbackModel(context.Background(),
		config.ProviderConfig{
			Provider: "openai",
			APIKey:   primaryAPIKey,
			Model:    primaryModel,
		},
		[]config.ProviderConfig{
			{
				Provider: "anthropic",
				APIKey:   fallbackAPIKey,
				Model:    fallbackModel,
			},
		},
	)
	if err != nil {
		t.Fatalf("BuildFallbackModel error: %v", err)
	}

	// Verify it is wrapped with fallback.
	fm, ok := lm.(*fallback.Model)
	if !ok {
		t.Fatalf("expected *fallback.Model, got %T", lm)
	}
	if len(fm.Candidates) != 2 {
		t.Errorf("expected 2 candidates, got %d", len(fm.Candidates))
	}

	// Verify it can actually generate (should use primary).
	resp, err := lm.Generate(context.Background(), &core.Request{
		Messages: []core.Message{
			{Role: core.MESSAGE_ROLE_USER, Content: []core.ContentParter{core.TextPart{Text: "Say hi in one word."}}},
		},
	})
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}
	if len(resp.Message.Content) == 0 {
		t.Fatal("expected content in response")
	}
	t.Logf("Response: %+v", resp.Message.Content[0])
}

func TestIntegration_BuildModel_OpenAI_GenerateObject(t *testing.T) {
	apiKey, model := openAIIntegrationConfig(t)

	lm, err := BuildModel(context.Background(), config.ProviderConfig{
		Provider: "openai",
		APIKey:   apiKey,
		Model:    model,
	})
	if err != nil {
		t.Fatalf("BuildModel error: %v", err)
	}

	schema := &core.Schema{
		Type: "object",
		Properties: map[string]*core.Schema{
			"greeting": {Type: "string"},
		},
		Required: []string{"greeting"},
	}

	resp, err := lm.GenerateObject(context.Background(), &core.ObjectRequest{
		Messages: []core.Message{
			{Role: core.MESSAGE_ROLE_USER, Content: []core.ContentParter{core.TextPart{Text: "Return a greeting object"}}},
		},
		Schema: schema,
		Mode:   core.ObjectModeJSON,
	})
	if err != nil {
		t.Fatalf("GenerateObject error: %v", err)
	}
	if resp.Object == nil {
		t.Fatal("expected non-nil object")
	}
	if _, ok := resp.Object["greeting"]; !ok {
		t.Errorf("expected object to have 'greeting' field, got %+v", resp.Object)
	}
	t.Logf("Object: %+v", resp.Object)
	t.Logf("Usage: prompt=%d completion=%d total=%d", resp.Usage.PromptTokens, resp.Usage.CompletionTokens, resp.Usage.TotalTokens)
}

func TestIntegration_BuildModel_Qwen_Generate(t *testing.T) {
	apiKey, baseURL, model := qwenIntegrationConfig(t)

	lm, err := BuildModel(context.Background(), config.ProviderConfig{
		Provider: "qwen",
		BaseURL:  baseURL,
		APIKey:   apiKey,
		Model:    model,
	})
	if err != nil {
		t.Fatalf("BuildModel error: %v", err)
	}

	resp, err := lm.Generate(context.Background(), &core.Request{
		Messages: []core.Message{
			{Role: core.MESSAGE_ROLE_USER, Content: []core.ContentParter{core.TextPart{Text: "Say hello in one word."}}},
		},
	})
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}
	if len(resp.Message.Content) == 0 {
		t.Fatal("expected content in response")
	}
	tp, ok := resp.Message.Content[0].(core.TextPart)
	if !ok {
		t.Fatalf("expected TextPart, got %T", resp.Message.Content[0])
	}
	if tp.Text == "" {
		t.Error("expected non-empty text response")
	}
	t.Logf("Response: %s", tp.Text)
	t.Logf("Usage: prompt=%d completion=%d total=%d", resp.Usage.PromptTokens, resp.Usage.CompletionTokens, resp.Usage.TotalTokens)
}

func TestIntegration_BuildModel_Zhipu_Generate(t *testing.T) {
	apiKey, baseURL, model := zhipuIntegrationConfig(t)

	lm, err := BuildModel(context.Background(), config.ProviderConfig{
		Provider: "zhipu",
		BaseURL:  baseURL,
		APIKey:   apiKey,
		Model:    model,
	})
	if err != nil {
		t.Fatalf("BuildModel error: %v", err)
	}

	resp, err := lm.Generate(context.Background(), &core.Request{
		Messages: []core.Message{
			{Role: core.MESSAGE_ROLE_USER, Content: []core.ContentParter{core.TextPart{Text: "Say hello in one word."}}},
		},
	})
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}
	if len(resp.Message.Content) == 0 {
		t.Fatal("expected content in response")
	}
	tp, ok := resp.Message.Content[0].(core.TextPart)
	if !ok {
		t.Fatalf("expected TextPart, got %T", resp.Message.Content[0])
	}
	if tp.Text == "" {
		t.Error("expected non-empty text response")
	}
	t.Logf("Response: %s", tp.Text)
	t.Logf("Usage: prompt=%d completion=%d total=%d", resp.Usage.PromptTokens, resp.Usage.CompletionTokens, resp.Usage.TotalTokens)
}

func TestIntegration_BuildModel_Kimi_Generate(t *testing.T) {
	apiKey, baseURL, model := kimiIntegrationConfig(t)

	lm, err := BuildModel(context.Background(), config.ProviderConfig{
		Provider: "kimi",
		BaseURL:  baseURL,
		APIKey:   apiKey,
		Model:    model,
	})
	if err != nil {
		t.Fatalf("BuildModel error: %v", err)
	}

	resp, err := lm.Generate(context.Background(), &core.Request{
		Messages: []core.Message{
			{Role: core.MESSAGE_ROLE_USER, Content: []core.ContentParter{core.TextPart{Text: "Say hello in one word."}}},
		},
	})
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}
	if len(resp.Message.Content) == 0 {
		t.Fatal("expected content in response")
	}
	tp, ok := resp.Message.Content[0].(core.TextPart)
	if !ok {
		t.Fatalf("expected TextPart, got %T", resp.Message.Content[0])
	}
	if tp.Text == "" {
		t.Error("expected non-empty text response")
	}
	t.Logf("Response: %s", tp.Text)
	t.Logf("Usage: prompt=%d completion=%d total=%d", resp.Usage.PromptTokens, resp.Usage.CompletionTokens, resp.Usage.TotalTokens)
}

func TestIntegration_BuildModel_OpenAICompatible_Generate(t *testing.T) {
	apiKey, baseURL, model := openaiCompatIntegrationConfig(t)

	lm, err := BuildModel(context.Background(), config.ProviderConfig{
		Provider: "openaicompatible",
		BaseURL:  baseURL,
		APIKey:   apiKey,
		Model:    model,
	})
	if err != nil {
		t.Fatalf("BuildModel error: %v", err)
	}

	resp, err := lm.Generate(context.Background(), &core.Request{
		Messages: []core.Message{
			{Role: core.MESSAGE_ROLE_USER, Content: []core.ContentParter{core.TextPart{Text: "Say hello in one word."}}},
		},
	})
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}
	if len(resp.Message.Content) == 0 {
		t.Fatal("expected content in response")
	}
	tp, ok := resp.Message.Content[0].(core.TextPart)
	if !ok {
		t.Fatalf("expected TextPart, got %T", resp.Message.Content[0])
	}
	if tp.Text == "" {
		t.Error("expected non-empty text response")
	}
	t.Logf("Response: %s", tp.Text)
	t.Logf("Usage: prompt=%d completion=%d total=%d", resp.Usage.PromptTokens, resp.Usage.CompletionTokens, resp.Usage.TotalTokens)
}
