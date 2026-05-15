package pantheonadapter

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/pantheon/core"
	"github.com/odysseythink/pantheon/extensions/fallback"
	"github.com/odysseythink/pantheon/extensions/retry"
	"github.com/odysseythink/pantheon/providers/openaicompat"
	"github.com/odysseythink/pantheon/types"
)

func ptr(s string) *string { return &s }

func TestBuildProvider_OpenAI(t *testing.T) {
	p, err := buildProvider(config.ProviderConfig{
		Provider: "openai",
		APIKey:   "test-key",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := p.Name(), "openai"; got != want {
		t.Errorf("Name() = %q, want %q", got, want)
	}
}

func TestBuildProvider_Anthropic(t *testing.T) {
	p, err := buildProvider(config.ProviderConfig{
		Provider: "anthropic",
		APIKey:   "test-key",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := p.Name(), "anthropic"; got != want {
		t.Errorf("Name() = %q, want %q", got, want)
	}
}

func TestBuildProvider_DeepSeek(t *testing.T) {
	p, err := buildProvider(config.ProviderConfig{
		Provider: "deepseek",
		BaseURL:  "https://api.deepseek.com",
		APIKey:   "test-key",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := p.Name(), "deepseek"; got != want {
		t.Errorf("Name() = %q, want %q", got, want)
	}
}

func TestBuildProvider_Unknown(t *testing.T) {
	_, err := buildProvider(config.ProviderConfig{
		Provider: "unknown-provider",
		APIKey:   "test-key",
	})
	if err == nil {
		t.Fatal("expected error for unknown provider, got nil")
	}
	if got, want := err.Error(), "unknown provider: unknown-provider"; got != want {
		t.Errorf("error = %q, want %q", got, want)
	}
}

func TestBuildProvider_OpenRouter(t *testing.T) {
	p, err := buildProvider(config.ProviderConfig{
		Provider: "openrouter",
		APIKey:   "test-key",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := p.Name(), "openrouter"; got != want {
		t.Errorf("Name() = %q, want %q", got, want)
	}
}

func TestBuildProvider_AllCompatProviders(t *testing.T) {
	// Each entry: provider name → expected Name() result.
	// Some providers map to a native implementation with a different reported name.
	providers := map[string]string{
		"deepseek":         "deepseek",
		"qwen":             "qwen",
		"zhipu":            "zhipu",
		"kimi":             "kimi",
		"minimax":          "minimax",
		"wenxin":           "wenxin",
		"moonshot":         "kimi",   // moonshot routes to kimi native provider
		"glm":              "zhipu",  // glm routes to zhipu native provider
		"ernie":            "wenxin", // ernie routes to wenxin native provider
		"openaicompatible": "openaicompatible",
	}
	for name, expectedName := range providers {
		t.Run(name, func(t *testing.T) {
			p, err := buildProvider(config.ProviderConfig{
				Provider: name,
				BaseURL:  "https://api.example.com",
				APIKey:   "test-key",
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got, want := p.Name(), expectedName; got != want {
				t.Errorf("Name() = %q, want %q", got, want)
			}
		})
	}
}

func TestWithBaseURL_Empty(t *testing.T) {
	result := withBaseURL(func(s string) string { return s + "-suffix" }, "")
	if len(result) != 0 {
		t.Errorf("expected empty slice, got %v", result)
	}
}

func TestCompatModel_ProviderAndModel(t *testing.T) {
	p, err := buildProvider(config.ProviderConfig{
		Provider: "deepseek",
		BaseURL:  "https://api.deepseek.com",
		APIKey:   "test-key",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	lm, err := p.LanguageModel(context.Background(), "deepseek-chat")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got, want := lm.Provider(), "deepseek"; got != want {
		t.Errorf("Provider() = %q, want %q", got, want)
	}
	if got, want := lm.Model(), "deepseek-chat"; got != want {
		t.Errorf("Model() = %q, want %q", got, want)
	}
}

func TestCompatModel_Generate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		resp := openaicompat.ChatCompletionResponse{
			Model: "test-model",
			Choices: []openaicompat.Choice{{
				Message:      openaicompat.Message{Role: "assistant", Content: "Hello!"},
				FinishReason: ptr("stop"),
			}},
			Usage: &openaicompat.Usage{PromptTokens: 10, CompletionTokens: 2, TotalTokens: 12},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p, err := buildProvider(config.ProviderConfig{
		Provider: "openaicompatible",
		BaseURL:  server.URL,
		APIKey:   "test-key",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cp, ok := p.(*compatProvider)
	if !ok {
		t.Fatalf("expected *compatProvider, got %T", p)
	}
	cp.client.HTTPClient = server.Client()

	lm, err := p.LanguageModel(context.Background(), "test-model")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp, err := lm.Generate(context.Background(), &core.Request{
		Messages: []core.Message{
			{Role: core.MESSAGE_ROLE_USER, Content: []core.ContentParter{core.TextPart{Text: "Hi"}}},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Message.Content) != 1 {
		t.Fatalf("expected 1 content part, got %d", len(resp.Message.Content))
	}
	if tp, ok := resp.Message.Content[0].(core.TextPart); !ok || tp.Text != "Hello!" {
		t.Errorf("unexpected response: %+v", resp.Message.Content[0])
	}
	if resp.FinishReason != "stop" {
		t.Errorf("unexpected finish reason: %q", resp.FinishReason)
	}
	if resp.Usage.TotalTokens != 12 {
		t.Errorf("unexpected total tokens: %d", resp.Usage.TotalTokens)
	}
}

func TestCompatModel_Stream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("expected Flusher")
		}

		chunks := []openaicompat.ChatCompletionResponse{
			{Model: "test-model", Choices: []openaicompat.Choice{{
				Delta: openaicompat.Message{Role: "assistant", Content: "Hello"},
			}}},
			{Model: "test-model", Choices: []openaicompat.Choice{{
				Delta: openaicompat.Message{Content: " world"},
			}}},
			{Model: "test-model", Choices: []openaicompat.Choice{{
				Delta:        openaicompat.Message{Content: ""},
				FinishReason: ptr("stop"),
			}}},
		}
		for _, c := range chunks {
			data, _ := json.Marshal(c)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
	defer server.Close()

	p, err := buildProvider(config.ProviderConfig{
		Provider: "openaicompatible",
		BaseURL:  server.URL,
		APIKey:   "test-key",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cp, ok := p.(*compatProvider)
	if !ok {
		t.Fatalf("expected *compatProvider, got %T", p)
	}
	cp.client.HTTPClient = server.Client()

	lm, err := p.LanguageModel(context.Background(), "test-model")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stream, err := lm.Stream(context.Background(), &core.Request{
		Messages: []core.Message{
			{Role: core.MESSAGE_ROLE_USER, Content: []core.ContentParter{core.TextPart{Text: "Hi"}}},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
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

	got := ""
	for _, d := range textDeltas {
		got += d
	}
	if got != "Hello world" {
		t.Errorf("text deltas: got %q, want %q", got, "Hello world")
	}
	if finishReason != "stop" {
		t.Errorf("finish reason: got %q, want stop", finishReason)
	}
}

func TestCompatModel_GenerateObject_JSONMode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req openaicompat.ChatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode error: %v", err)
		}
		if req.ResponseFormat == nil {
			t.Error("expected response format")
		}

		resp := openaicompat.ChatCompletionResponse{
			Model: "test-model",
			Choices: []openaicompat.Choice{{
				Message:      openaicompat.Message{Role: "assistant", Content: `{"greeting":"hello"}`},
				FinishReason: ptr("stop"),
			}},
			Usage: &openaicompat.Usage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p, err := buildProvider(config.ProviderConfig{
		Provider: "openaicompatible",
		BaseURL:  server.URL,
		APIKey:   "test-key",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cp, ok := p.(*compatProvider)
	if !ok {
		t.Fatalf("expected *compatProvider, got %T", p)
	}
	cp.client.HTTPClient = server.Client()

	lm, err := p.LanguageModel(context.Background(), "test-model")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	schema := &core.Schema{
		Type: "object",
		Properties: map[string]*core.Schema{
			"greeting": {Type: "string"},
		},
	}

	resp, err := lm.GenerateObject(context.Background(), &core.ObjectRequest{
		Messages: []core.Message{
			{Role: core.MESSAGE_ROLE_USER, Content: []core.ContentParter{core.TextPart{Text: "Say hello"}}},
		},
		Schema: schema,
		Mode:   core.ObjectModeJSON,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Object == nil {
		t.Fatal("expected non-nil object")
	}
	if resp.Object["greeting"] != "hello" {
		t.Errorf("unexpected object: %+v", resp.Object)
	}
	if resp.Usage.TotalTokens != 15 {
		t.Errorf("unexpected total tokens: %d", resp.Usage.TotalTokens)
	}
}

func TestCompatModel_GenerateObject_ToolMode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req openaicompat.ChatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode error: %v", err)
		}
		if len(req.Tools) != 1 {
			t.Errorf("expected 1 tool, got %d", len(req.Tools))
		}

		resp := openaicompat.ChatCompletionResponse{
			Model: "test-model",
			Choices: []openaicompat.Choice{{
				Message: openaicompat.Message{
					Role: "assistant",
					ToolCalls: []types.ToolCall{{
						ID:   "call_1",
						Type: "function",
						Function: struct {
							Name      string `json:"name"`
							Arguments string `json:"arguments"`
						}{
							Name:      "generate_object",
							Arguments: `{"greeting":"hello"}`,
						},
					}},
				},
				FinishReason: ptr("stop"),
			}},
			Usage: &openaicompat.Usage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p, err := buildProvider(config.ProviderConfig{
		Provider: "openaicompatible",
		BaseURL:  server.URL,
		APIKey:   "test-key",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cp, ok := p.(*compatProvider)
	if !ok {
		t.Fatalf("expected *compatProvider, got %T", p)
	}
	cp.client.HTTPClient = server.Client()

	lm, err := p.LanguageModel(context.Background(), "test-model")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	schema := &core.Schema{
		Type: "object",
		Properties: map[string]*core.Schema{
			"greeting": {Type: "string"},
		},
	}

	resp, err := lm.GenerateObject(context.Background(), &core.ObjectRequest{
		Messages: []core.Message{
			{Role: core.MESSAGE_ROLE_USER, Content: []core.ContentParter{core.TextPart{Text: "Say hello"}}},
		},
		Schema: schema,
		Mode:   core.ObjectModeTool,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Object == nil {
		t.Fatal("expected non-nil object")
	}
	if resp.Object["greeting"] != "hello" {
		t.Errorf("unexpected object: %+v", resp.Object)
	}
}

func TestCompatModel_GenerateObject_TextMode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req openaicompat.ChatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode error: %v", err)
		}
		if req.ResponseFormat == nil {
			t.Error("expected response format")
		}

		resp := openaicompat.ChatCompletionResponse{
			Model: "test-model",
			Choices: []openaicompat.Choice{{
				Message:      openaicompat.Message{Role: "assistant", Content: `{"greeting":"hello"}`},
				FinishReason: ptr("stop"),
			}},
			Usage: &openaicompat.Usage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p, err := buildProvider(config.ProviderConfig{
		Provider: "openaicompatible",
		BaseURL:  server.URL,
		APIKey:   "test-key",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cp, ok := p.(*compatProvider)
	if !ok {
		t.Fatalf("expected *compatProvider, got %T", p)
	}
	cp.client.HTTPClient = server.Client()

	lm, err := p.LanguageModel(context.Background(), "test-model")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	schema := &core.Schema{
		Type: "object",
		Properties: map[string]*core.Schema{
			"greeting": {Type: "string"},
		},
	}

	resp, err := lm.GenerateObject(context.Background(), &core.ObjectRequest{
		Messages: []core.Message{
			{Role: core.MESSAGE_ROLE_USER, Content: []core.ContentParter{core.TextPart{Text: "Say hello"}}},
		},
		Schema: schema,
		Mode:   core.ObjectModeText,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Object == nil {
		t.Fatal("expected non-nil object")
	}
	if resp.Object["greeting"] != "hello" {
		t.Errorf("unexpected object: %+v", resp.Object)
	}
}

func TestCompatModel_GenerateObject_NoContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := openaicompat.ChatCompletionResponse{
			Model: "test-model",
			Choices: []openaicompat.Choice{{
				Message:      openaicompat.Message{Role: "assistant", Content: ""},
				FinishReason: ptr("stop"),
			}},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p, err := buildProvider(config.ProviderConfig{
		Provider: "openaicompatible",
		BaseURL:  server.URL,
		APIKey:   "test-key",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cp, ok := p.(*compatProvider)
	if !ok {
		t.Fatalf("expected *compatProvider, got %T", p)
	}
	cp.client.HTTPClient = server.Client()

	lm, err := p.LanguageModel(context.Background(), "test-model")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	schema := &core.Schema{Type: "object"}
	_, err = lm.GenerateObject(context.Background(), &core.ObjectRequest{
		Messages: []core.Message{
			{Role: core.MESSAGE_ROLE_USER, Content: []core.ContentParter{core.TextPart{Text: "Say hello"}}},
		},
		Schema: schema,
		Mode:   core.ObjectModeAuto,
	})
	if err == nil {
		t.Fatal("expected error for empty content")
	}
}

func TestBuildModel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := openaicompat.ChatCompletionResponse{
			Model: "test-model",
			Choices: []openaicompat.Choice{{
				Message:      openaicompat.Message{Role: "assistant", Content: "Hi!"},
				FinishReason: ptr("stop"),
			}},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	lm, err := BuildModel(context.Background(), config.ProviderConfig{
		Provider: "deepseek",
		BaseURL:  server.URL,
		APIKey:   "test-key",
		Model:    "test-model",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := lm.Provider(), "deepseek"; got != want {
		t.Errorf("Provider() = %q, want %q", got, want)
	}
	if got, want := lm.Model(), "test-model"; got != want {
		t.Errorf("Model() = %q, want %q", got, want)
	}
}

func TestBuildPrimaryModel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := openaicompat.ChatCompletionResponse{
			Model: "test-model",
			Choices: []openaicompat.Choice{{
				Message:      openaicompat.Message{Role: "assistant", Content: "Hi!"},
				FinishReason: ptr("stop"),
			}},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	lm, err := BuildPrimaryModel(context.Background(), config.ProviderConfig{
		Provider: "deepseek",
		BaseURL:  server.URL,
		APIKey:   "test-key",
		Model:    "test-model",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// BuildPrimaryModel wraps the model with retry.
	rm, ok := lm.(*retry.Model)
	if !ok {
		t.Fatalf("expected *retry.Model, got %T", lm)
	}
	if rm.MaxRetries != 3 {
		t.Errorf("MaxRetries = %d, want 3", rm.MaxRetries)
	}
}

func TestBuildFallbackModel_WithFallbacks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := openaicompat.ChatCompletionResponse{
			Model: "test-model",
			Choices: []openaicompat.Choice{{
				Message:      openaicompat.Message{Role: "assistant", Content: "Hi!"},
				FinishReason: ptr("stop"),
			}},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	lm, err := BuildFallbackModel(context.Background(),
		config.ProviderConfig{
			Provider: "deepseek",
			BaseURL:  server.URL,
			APIKey:   "test-key",
			Model:    "test-model",
		},
		[]config.ProviderConfig{
			{
				Provider: "qwen",
				BaseURL:  server.URL,
				APIKey:   "fallback-key",
				Model:    "fallback-model",
			},
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// With fallbacks, the model is wrapped in fallback.Model.
	fm, ok := lm.(*fallback.Model)
	if !ok {
		t.Fatalf("expected *fallback.Model, got %T", lm)
	}
	if len(fm.Candidates) != 2 {
		t.Errorf("expected 2 candidates, got %d", len(fm.Candidates))
	}
}

func TestBuildFallbackModel_WithoutFallbacks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := openaicompat.ChatCompletionResponse{
			Model: "test-model",
			Choices: []openaicompat.Choice{{
				Message:      openaicompat.Message{Role: "assistant", Content: "Hi!"},
				FinishReason: ptr("stop"),
			}},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	lm, err := BuildFallbackModel(context.Background(),
		config.ProviderConfig{
			Provider: "deepseek",
			BaseURL:  server.URL,
			APIKey:   "test-key",
			Model:    "test-model",
		},
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Without fallbacks, returns the primary model directly (wrapped in retry.Model).
	_, ok := lm.(*retry.Model)
	if !ok {
		t.Fatalf("expected *retry.Model, got %T", lm)
	}
}

func TestBuildFallbackModel_SkipsEmptyAPIKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := openaicompat.ChatCompletionResponse{
			Model: "test-model",
			Choices: []openaicompat.Choice{{
				Message:      openaicompat.Message{Role: "assistant", Content: "Hi!"},
				FinishReason: ptr("stop"),
			}},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	lm, err := BuildFallbackModel(context.Background(),
		config.ProviderConfig{
			Provider: "deepseek",
			BaseURL:  server.URL,
			APIKey:   "test-key",
			Model:    "test-model",
		},
		[]config.ProviderConfig{
			{
				Provider: "qwen",
				BaseURL:  server.URL,
				APIKey:   "", // empty key should be skipped
				Model:    "fallback-model",
			},
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Empty API key fallback should be skipped, leaving only 1 candidate.
	// When there's only 1 candidate, BuildFallbackModel returns primary directly (retry.Model).
	_, ok := lm.(*retry.Model)
	if !ok {
		t.Fatalf("expected *retry.Model, got %T", lm)
	}
}

func TestBuildModel_UnknownProvider(t *testing.T) {
	_, err := BuildModel(context.Background(), config.ProviderConfig{
		Provider: "unknown",
		APIKey:   "test-key",
		Model:    "test-model",
	})
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
}

func TestBuildProvider_OpenAI_WithBaseURL(t *testing.T) {
	p, err := buildProvider(config.ProviderConfig{
		Provider: "openai",
		APIKey:   "test-key",
		BaseURL:  "https://custom.openai.com",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := p.Name(), "openai"; got != want {
		t.Errorf("Name() = %q, want %q", got, want)
	}
}

func TestBuildPrimaryModel_Error(t *testing.T) {
	_, err := BuildPrimaryModel(context.Background(), config.ProviderConfig{
		Provider: "unknown",
		APIKey:   "test-key",
		Model:    "test-model",
	})
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
}

func TestBuildFallbackModel_PrimaryError(t *testing.T) {
	_, err := BuildFallbackModel(context.Background(),
		config.ProviderConfig{
			Provider: "unknown",
			APIKey:   "test-key",
			Model:    "test-model",
		},
		nil,
	)
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
}

func TestBuildFallbackModel_FallbackError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := openaicompat.ChatCompletionResponse{
			Model: "test-model",
			Choices: []openaicompat.Choice{{
				Message:      openaicompat.Message{Role: "assistant", Content: "Hi!"},
				FinishReason: ptr("stop"),
			}},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	_, err := BuildFallbackModel(context.Background(),
		config.ProviderConfig{
			Provider: "deepseek",
			BaseURL:  server.URL,
			APIKey:   "test-key",
			Model:    "test-model",
		},
		[]config.ProviderConfig{
			{
				Provider: "unknown", // will cause BuildModel to fail
				APIKey:   "fallback-key",
				Model:    "fallback-model",
			},
		},
	)
	if err == nil {
		t.Fatal("expected error when fallback provider is unknown")
	}
}

func TestCompatModel_GenerateObject_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("invalid key"))
	}))
	defer server.Close()

	p, err := buildProvider(config.ProviderConfig{
		Provider: "openaicompatible",
		BaseURL:  server.URL,
		APIKey:   "bad-key",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cp, ok := p.(*compatProvider)
	if !ok {
		t.Fatalf("expected *compatProvider, got %T", p)
	}
	cp.client.HTTPClient = server.Client()

	lm, err := p.LanguageModel(context.Background(), "test-model")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	schema := &core.Schema{Type: "object"}
	_, err = lm.GenerateObject(context.Background(), &core.ObjectRequest{
		Messages: []core.Message{
			{Role: core.MESSAGE_ROLE_USER, Content: []core.ContentParter{core.TextPart{Text: "Say hello"}}},
		},
		Schema: schema,
		Mode:   core.ObjectModeAuto,
	})
	if err == nil {
		t.Fatal("expected error for failed generation")
	}
}
