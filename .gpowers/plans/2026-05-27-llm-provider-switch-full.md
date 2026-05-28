# LLM Provider Switch 全量补齐实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use gpowers:subagent-driven-development (recommended) or gpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 backend 的 LLM provider 支持从 2 个扩展到 Node 已有的 36 个，通过 Pantheon v0.0.9 的 provider 包实现。

**Architecture:** 用 `providerRegistry map[string]providerBuilder` 替代 giant switch，每个 provider 的构建逻辑独立在 builder 函数中。配置解析提取到 `resolve.go`，`llm.go` 只保留接口定义和入口。所有 36 个 provider 通过纯单元测试验证（mock interface，不触网）。

**Tech Stack:** Go 1.25.5, Pantheon v0.0.9, caarlos0/env/v11, testify

---

## 文件结构

```
backend/internal/providers/
├── llm.go              # 接口 + PantheonLLM + noopLLM + NewLLMProvider 入口
├── builders.go         # 36 个 providerBuilder 函数 + providerRegistry map
├── resolve.go          # 配置解析辅助函数（fallback 链 / Azure endpoint 解析）
├── llm_test.go         # PantheonLLM Stream/Complete 测试（mock core.LanguageModel）
├── builders_test.go    # Registry 完整性 + 各 builder missing-key 测试
└── resolve_test.go     # 配置解析 + Azure endpoint 解析测试
backend/internal/config/
└── config.go           # 新增 ~110 个 provider-specific 配置字段
```

---

## Task 1: resolve.go — 配置解析辅助函数

**Files:**
- Create: `backend/internal/providers/resolve.go`
- Create: `backend/internal/providers/resolve_test.go`

- [ ] **Step 1: Write resolve.go**

```go
package providers

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/odysseythink/hermind/backend/internal/config"
)

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func resolveProviderName(cfg *config.Config, settings map[string]string) string {
	if v, ok := settings["LLMProvider"]; ok && v != "" {
		return v
	}
	return cfg.LLMProvider
}

func resolveModelID(provider string, cfg *config.Config, settings map[string]string) string {
	key := modelPrefKeyForProvider(provider)
	if v, ok := settings[key]; ok && v != "" {
		return v
	}
	if v := cfgModelPref(cfg, provider); v != "" {
		return v
	}
	if cfg.LLMModel != "" {
		return cfg.LLMModel
	}
	return defaultModelForProvider(provider)
}

func modelPrefKeyForProvider(provider string) string {
	switch provider {
	case "openai":
		return "OpenAiModelPref"
	case "azure":
		return "AzureOpenAiModelPref"
	case "anthropic":
		return "AnthropicModelPref"
	case "gemini":
		return "GeminiLLMModelPref"
	case "lmstudio":
		return "LMStudioModelPref"
	case "localai":
		return "LocalAiModelPref"
	case "ollama":
		return "OllamaLLMModelPref"
	case "togetherai":
		return "TogetherAiModelPref"
	case "fireworksai":
		return "FireworksModelPref"
	case "mistral":
		return "MistralModelPref"
	case "huggingface":
		return "HuggingFaceLLMModelPref"
	case "perplexity":
		return "PerplexityModelPref"
	case "openrouter":
		return "OpenRouterModelPref"
	case "novita":
		return "NovitaModelPref"
	case "groq":
		return "GroqModelPref"
	case "koboldcpp":
		return "KoboldCPPModelPref"
	case "textgenwebui":
		return "TextGenWebUIModelPref"
	case "cohere":
		return "CohereModelPref"
	case "litellm":
		return "LiteLLMModelPref"
	case "generic-openai":
		return "GenericOpenAiModelPref"
	case "bedrock":
		return "AWSBedrockModelPref"
	case "deepseek":
		return "DeepSeekModelPref"
	case "apipie":
		return "ApiPieModelPref"
	case "xai":
		return "XaiModelPref"
	case "nvidia-nim":
		return "NvidiaNimModelPref"
	case "ppio":
		return "PpioModelPref"
	case "dpaiStudio":
		return "DellProModelPref"
	case "moonshotai":
		return "MoonshotModelPref"
	case "cometapi":
		return "CometApiModelPref"
	case "foundry":
		return "FoundryModelPref"
	case "zai":
		return "ZaiModelPref"
	case "giteeai":
		return "GiteeAiModelPref"
	case "docker-model-runner":
		return "DockerModelRunnerModelPref"
	case "privatemode":
		return "PrivateModeModelPref"
	case "sambanova":
		return "SambaNovaModelPref"
	case "lemonade":
		return "LemonadeModelPref"
	}
	return ""
}

func cfgModelPref(cfg *config.Config, provider string) string {
	switch provider {
	case "openai":
		return cfg.OpenAiModelPref
	case "azure":
		return cfg.AzureOpenAiModelPref
	case "anthropic":
		return cfg.AnthropicModelPref
	case "gemini":
		return cfg.GeminiModelPref
	case "lmstudio":
		return cfg.LMStudioModelPref
	case "localai":
		return cfg.LocalAiModelPref
	case "ollama":
		return cfg.OllamaModelPref
	case "togetherai":
		return cfg.TogetherAiModelPref
	case "fireworksai":
		return cfg.FireworksModelPref
	case "mistral":
		return cfg.MistralModelPref
	case "huggingface":
		return ""
	case "perplexity":
		return cfg.PerplexityModelPref
	case "openrouter":
		return cfg.OpenRouterModelPref
	case "novita":
		return cfg.NovitaModelPref
	case "groq":
		return cfg.GroqModelPref
	case "koboldcpp":
		return cfg.KoboldModelPref
	case "textgenwebui":
		return ""
	case "cohere":
		return cfg.CohereModelPref
	case "litellm":
		return cfg.LiteLLMModelPref
	case "generic-openai":
		return cfg.GenericOpenAiModelPref
	case "bedrock":
		return cfg.AWSBedrockModelPref
	case "deepseek":
		return cfg.DeepSeekModelPref
	case "apipie":
		return cfg.ApiPieModelPref
	case "xai":
		return cfg.XaiModelPref
	case "nvidia-nim":
		return cfg.NvidiaNimModelPref
	case "ppio":
		return cfg.PpioModelPref
	case "dpaiStudio":
		return cfg.DellProModelPref
	case "moonshotai":
		return cfg.MoonshotModelPref
	case "cometapi":
		return cfg.CometApiModelPref
	case "foundry":
		return cfg.FoundryModelPref
	case "zai":
		return cfg.ZaiModelPref
	case "giteeai":
		return cfg.GiteeModelPref
	case "docker-model-runner":
		return cfg.DockerModelModelPref
	case "privatemode":
		return cfg.PrivateModeModelPref
	case "sambanova":
		return cfg.SambaNovaModelPref
	case "lemonade":
		return cfg.LemonadeModelPref
	}
	return ""
}

func defaultModelForProvider(provider string) string {
	switch provider {
	case "openai":
		return "gpt-4o-mini"
	case "azure":
		return "gpt-4o"
	case "anthropic":
		return "claude-3-5-sonnet-20241022"
	case "gemini":
		return "gemini-1.5-flash"
	case "lmstudio":
		return ""
	case "localai":
		return ""
	case "ollama":
		return ""
	case "togetherai":
		return ""
	case "fireworksai":
		return ""
	case "mistral":
		return "mistral-large-latest"
	case "huggingface":
		return ""
	case "perplexity":
		return "llama-3.1-sonar-small-128k-online"
	case "openrouter":
		return ""
	case "novita":
		return ""
	case "groq":
		return "llama3-8b-8192"
	case "koboldcpp":
		return ""
	case "textgenwebui":
		return ""
	case "cohere":
		return "command-r"
	case "litellm":
		return ""
	case "generic-openai":
		return ""
	case "bedrock":
		return ""
	case "deepseek":
		return "deepseek-chat"
	case "apipie":
		return ""
	case "xai":
		return "grok-beta"
	case "nvidia-nim":
		return ""
	case "ppio":
		return ""
	case "dpaiStudio":
		return ""
	case "moonshotai":
		return "moonshot-v1-8k"
	case "cometapi":
		return ""
	case "foundry":
		return ""
	case "zai":
		return ""
	case "giteeai":
		return ""
	case "docker-model-runner":
		return ""
	case "privatemode":
		return ""
	case "sambanova":
		return ""
	case "lemonade":
		return ""
	}
	return "gpt-4o-mini"
}

func parseAzureEndpoint(endpoint string) (resourceName, deployment string, err error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", "", fmt.Errorf("parse azure endpoint: %w", err)
	}
	// Hostname: myresource.openai.azure.com
	host := u.Hostname()
	parts := strings.Split(host, ".")
	if len(parts) < 4 || !strings.HasSuffix(host, ".openai.azure.com") {
		return "", "", fmt.Errorf("azure endpoint hostname must be {resourceName}.openai.azure.com")
	}
	resourceName = parts[0]

	// Path: /openai/deployments/{deployment}
	path := strings.Trim(u.Path, "/")
	pathParts := strings.Split(path, "/")
	if len(pathParts) >= 3 && pathParts[0] == "openai" && pathParts[1] == "deployments" {
		deployment = pathParts[2]
	}

	if deployment == "" {
		return "", "", fmt.Errorf("azure endpoint path must contain /openai/deployments/{deployment}")
	}
	return resourceName, deployment, nil
}
```

- [ ] **Step 2: Write resolve_test.go**

```go
package providers

import (
	"testing"

	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestFirstNonEmpty(t *testing.T) {
	assert.Equal(t, "a", firstNonEmpty("", "a", "b"))
	assert.Equal(t, "", firstNonEmpty("", ""))
	assert.Equal(t, "x", firstNonEmpty("x"))
}

func TestResolveProviderName(t *testing.T) {
	cfg := &config.Config{LLMProvider: "openai"}
	assert.Equal(t, "openai", resolveProviderName(cfg, map[string]string{}))
	assert.Equal(t, "anthropic", resolveProviderName(cfg, map[string]string{"LLMProvider": "anthropic"}))
}

func TestResolveModelID(t *testing.T) {
	cfg := &config.Config{LLMModel: "fallback-model"}
	settings := map[string]string{"OpenAiModelPref": "gpt-4o"}
	assert.Equal(t, "gpt-4o", resolveModelID("openai", cfg, settings))

	// fallback to cfg.LLMModel
	assert.Equal(t, "fallback-model", resolveModelID("mistral", cfg, map[string]string{}))

	// default
	cfg2 := &config.Config{}
	assert.Equal(t, "gpt-4o-mini", resolveModelID("openai", cfg2, map[string]string{}))
	assert.Equal(t, "claude-3-5-sonnet-20241022", resolveModelID("anthropic", cfg2, map[string]string{}))
}

func TestParseAzureEndpoint(t *testing.T) {
	rn, dep, err := parseAzureEndpoint("https://myresource.openai.azure.com/openai/deployments/mydeployment")
	assert.NoError(t, err)
	assert.Equal(t, "myresource", rn)
	assert.Equal(t, "mydeployment", dep)

	_, _, err = parseAzureEndpoint("not-a-url")
	assert.Error(t, err)

	_, _, err = parseAzureEndpoint("https://example.com/")
	assert.Error(t, err)
}
```

- [ ] **Step 3: Run tests**

```bash
cd backend && go test -tags "fts5 nolancedb" ./internal/providers/ -run "TestFirstNonEmpty|TestResolveProviderName|TestResolveModelID|TestParseAzureEndpoint" -v
```

Expected: PASS (4 tests)

- [ ] **Step 4: Commit**

```bash
git add backend/internal/providers/resolve.go backend/internal/providers/resolve_test.go
git commit -m "feat(providers): add resolve.go with config fallback chain and Azure endpoint parser"
```

---

## Task 2: llm.go 重构 — 提取 Registry 入口

**Files:**
- Modify: `backend/internal/providers/llm.go`
- Modify: `backend/internal/providers/llm_test.go` (if existing tests break)

- [ ] **Step 1: Read current llm.go and understand structure**

```bash
cat backend/internal/providers/llm.go
```

- [ ] **Step 2: Replace the switch body with registry lookup**

Replace the entire `NewLLMProvider` function body. Keep `PantheonLLM` struct and its `Stream`/`Complete`/`LanguageModel` methods unchanged. Keep `noopLLM` unchanged.

The new `llm.go` should look like:

```go
package providers

import (
	"context"
	"fmt"
	"strings"

	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/mlog"
	"github.com/odysseythink/pantheon/core"
)

// LLMChunk is a single chunk from a streaming LLM response.
type LLMChunk struct {
	TextDelta      string
	ReasoningDelta string
	Usage          *core.Usage
	FinishReason   string
	Err            error
}

// LLMProvider is the interface for LLM streaming.
type LLMProvider interface {
	Stream(ctx context.Context, messages []core.Message, systemPrompt string, temperature *float64) (<-chan LLMChunk, error)
	Complete(ctx context.Context, messages []core.Message, systemPrompt string, temperature *float64) (string, error)
	LanguageModel() core.LanguageModel
}

// PantheonLLM wraps a Pantheon core.LanguageModel for streaming.
type PantheonLLM struct {
	model core.LanguageModel
	cfg   *config.Config
}

// providerBuilder creates a Pantheon LanguageModel for a specific provider.
type providerBuilder func(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error)

// providerRegistry maps provider names to their builder functions.
// Populated in builders.go.
var providerRegistry = map[string]providerBuilder{}

// NewLLMProvider creates a Pantheon-based LLM provider.
func NewLLMProvider(cfg *config.Config, settings map[string]string) LLMProvider {
	providerName := resolveProviderName(cfg, settings)
	modelID := resolveModelID(providerName, cfg, settings)

	mlog.Info("NewLLMProvider: provider=", providerName, " model=", modelID)

	builder, ok := providerRegistry[providerName]
	if !ok {
		mlog.Error("NewLLMProvider: unsupported provider ", providerName)
		return &noopLLM{err: fmt.Errorf("unsupported LLM provider: %s", providerName)}
	}

	model, err := builder(context.Background(), cfg, settings, modelID)
	if err != nil {
		mlog.Error("NewLLMProvider: builder failed for ", providerName, ": ", err)
		return &noopLLM{err: fmt.Errorf("create %s provider: %w", providerName, err)}
	}

	mlog.Info("NewLLMProvider: created PantheonLLM (", providerName, ") with model ", modelID)
	return &PantheonLLM{model: model, cfg: cfg}
}

// Stream implements LLMProvider by calling Pantheon's streaming API.
func (p *PantheonLLM) Stream(ctx context.Context, messages []core.Message, systemPrompt string, temperature *float64) (<-chan LLMChunk, error) {
	// ... existing implementation, unchanged ...
}

// Complete implements LLMProvider with a synchronous (non-streaming) call.
func (p *PantheonLLM) Complete(ctx context.Context, messages []core.Message, systemPrompt string, temperature *float64) (string, error) {
	// ... existing implementation, unchanged ...
}

// LanguageModel returns the underlying Pantheon core.LanguageModel.
func (p *PantheonLLM) LanguageModel() core.LanguageModel { return p.model }

// noopLLM is a fallback provider that returns an error.
type noopLLM struct {
	err error
}

func (n *noopLLM) Stream(ctx context.Context, messages []core.Message, systemPrompt string, temperature *float64) (<-chan LLMChunk, error) {
	return nil, n.err
}

func (n *noopLLM) Complete(ctx context.Context, messages []core.Message, systemPrompt string, temperature *float64) (string, error) {
	return "", n.err
}

func (n *noopLLM) LanguageModel() core.LanguageModel { return nil }
```

**Important:** Remove the old switch-based `NewLLMProvider` entirely. Remove the `ollama` and `openai` imports from `llm.go` (they move to `builders.go`). Remove the old inline ollama/openai construction logic.

- [ ] **Step 3: Remove unused imports from llm.go**

After removing the switch body, `llm.go` only needs:
- `context`
- `fmt`
- `strings` (keep, used by Complete)
- `config` (local)
- `mlog`
- `core` (pantheon)

Remove `github.com/odysseythink/pantheon/providers/ollama` and `github.com/odysseythink/pantheon/providers/openai`.

- [ ] **Step 4: Run existing tests to ensure no breakage**

```bash
cd backend && go test -tags "fts5 nolancedb" ./internal/providers/ -v
```

Expected: At this point `providerRegistry` is empty, so any test that calls `NewLLMProvider` with a real provider name will get `noopLLM` with "unsupported provider". Update existing tests if they depend on ollama/openai working through the old switch.

If `llm_test.go` has tests that instantiate `PantheonLLM` directly (not through `NewLLMProvider`), they should still pass.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/providers/llm.go
git commit -m "refactor(providers): extract NewLLMProvider to registry pattern"
```

---

## Task 3: config.go 扩展 — 新增 36 个 Provider 的配置字段

**Files:**
- Modify: `backend/internal/config/config.go`

- [ ] **Step 1: Add all provider-specific fields to Config struct**

Insert the following fields into `Config` struct in `backend/internal/config/config.go`, after the existing `LLMMaxTokens` field and before the `EmbeddingApiKey` field:

```go
	// === OpenAI ===
	OpenAiKey       string `env:"OPEN_AI_KEY"`
	OpenAiModelPref string `env:"OPEN_MODEL_PREF"`

	// === Azure ===
	AzureOpenAiEndpoint         string `env:"AZURE_OPENAI_ENDPOINT"`
	AzureOpenAiKey              string `env:"AZURE_OPENAI_KEY"`
	AzureOpenAiModelPref        string `env:"AZURE_OPENAI_MODEL_PREF"`
	AzureOpenAiModelType        string `env:"AZURE_OPENAI_MODEL_TYPE" envDefault:"default"`
	AzureOpenAiTokenLimit       int    `env:"AZURE_OPENAI_TOKEN_LIMIT"`
	AzureOpenAiResourceName     string `env:"AZURE_OPENAI_RESOURCE_NAME"`
	AzureOpenAiDeployment       string `env:"AZURE_OPENAI_DEPLOYMENT"`

	// === Anthropic ===
	AnthropicApiKey       string `env:"ANTHROPIC_API_KEY"`
	AnthropicModelPref    string `env:"ANTHROPIC_MODEL_PREF"`
	AnthropicCacheControl string `env:"ANTHROPIC_CACHE_CONTROL" envDefault:"none"`

	// === Gemini (Google) ===
	GeminiApiKey        string `env:"GEMINI_API_KEY"`
	GeminiModelPref     string `env:"GEMINI_LLM_MODEL_PREF"`
	GeminiSafetySetting string `env:"GEMINI_SAFETY_SETTING"`

	// === LMStudio ===
	LMStudioBasePath   string `env:"LMSTUDIO_BASE_PATH"`
	LMStudioModelPref  string `env:"LMSTUDIO_MODEL_PREF"`
	LMStudioTokenLimit int    `env:"LMSTUDIO_MODEL_TOKEN_LIMIT"`
	LMStudioAuthToken  string `env:"LMSTUDIO_AUTH_TOKEN"`

	// === LocalAI ===
	LocalAiBasePath   string `env:"LOCAL_AI_BASE_PATH"`
	LocalAiModelPref  string `env:"LOCAL_AI_MODEL_PREF"`
	LocalAiTokenLimit int    `env:"LOCAL_AI_MODEL_TOKEN_LIMIT"`
	LocalAiApiKey     string `env:"LOCAL_AI_API_KEY"`

	// === Ollama ===
	OllamaBasePath     string `env:"OLLAMA_BASE_PATH"`
	OllamaModelPref    string `env:"OLLAMA_MODEL_PREF"`
	OllamaTokenLimit   int    `env:"OLLAMA_MODEL_TOKEN_LIMIT"`
	OllamaKeepAliveSec int    `env:"OLLAMA_KEEP_ALIVE_TIMEOUT"`
	OllamaAuthToken    string `env:"OLLAMA_AUTH_TOKEN"`

	// === TogetherAI ===
	TogetherAiApiKey    string `env:"TOGETHER_AI_API_KEY"`
	TogetherAiModelPref string `env:"TOGETHER_AI_MODEL_PREF"`

	// === FireworksAI ===
	FireworksApiKey    string `env:"FIREWORKS_API_KEY"`
	FireworksModelPref string `env:"FIREWORKS_MODEL_PREF"`

	// === Mistral ===
	MistralApiKey    string `env:"MISTRAL_API_KEY"`
	MistralModelPref string `env:"MISTRAL_MODEL_PREF"`

	// === HuggingFace ===
	HuggingFaceEndpoint   string `env:"HUGGING_FACE_LLM_ENDPOINT"`
	HuggingFaceApiKey     string `env:"HUGGING_FACE_LLM_API_KEY"`
	HuggingFaceTokenLimit int    `env:"HUGGING_FACE_LLM_TOKEN_LIMIT"`

	// === Perplexity ===
	PerplexityApiKey    string `env:"PERPLEXITY_API_KEY"`
	PerplexityModelPref string `env:"PERPLEXITY_MODEL_PREF"`

	// === OpenRouter ===
	OpenRouterApiKey    string `env:"OPENROUTER_API_KEY"`
	OpenRouterModelPref string `env:"OPENROUTER_MODEL_PREF"`

	// === Novita ===
	NovitaApiKey    string `env:"NOVITA_API_KEY"`
	NovitaModelPref string `env:"NOVITA_MODEL_PREF"`

	// === Groq ===
	GroqApiKey    string `env:"GROQ_API_KEY"`
	GroqModelPref string `env:"GROQ_MODEL_PREF"`

	// === KoboldCPP ===
	KoboldBasePath   string `env:"KOBOLD_CPP_BASE_PATH"`
	KoboldModelPref  string `env:"KOBOLD_CPP_MODEL_PREF"`
	KoboldTokenLimit int    `env:"KOBOLD_CPP_MODEL_TOKEN_LIMIT"`
	KoboldMaxTokens  int    `env:"KOBOLD_CPP_MAX_TOKENS"`

	// === TextGenWebUI ===
	TextGenBasePath   string `env:"TEXT_GEN_WEB_UI_BASE_PATH"`
	TextGenTokenLimit int    `env:"TEXT_GEN_WEB_UI_MODEL_TOKEN_LIMIT"`
	TextGenApiKey     string `env:"TEXT_GEN_WEB_UI_API_KEY"`

	// === Cohere ===
	CohereApiKey    string `env:"COHERE_API_KEY"`
	CohereModelPref string `env:"COHERE_MODEL_PREF"`

	// === LiteLLM ===
	LiteLLMModelPref string `env:"LITE_LLM_MODEL_PREF"`
	LiteLLMBasePath  string `env:"LITE_LLM_BASE_PATH"`
	LiteLLMApiKey    string `env:"LITE_LLM_API_KEY"`

	// === GenericOpenAI ===
	GenericOpenAiBasePath  string `env:"GENERIC_OPEN_AI_BASE_PATH"`
	GenericOpenAiModelPref string `env:"GENERIC_OPEN_AI_MODEL_PREF"`
	GenericOpenAiApiKey    string `env:"GENERIC_OPEN_AI_API_KEY"`
	GenericOpenAiMaxTokens int    `env:"GENERIC_OPEN_AI_MAX_TOKENS"`

	// === Bedrock ===
	AWSBedrockAccessKeyID  string `env:"AWS_BEDROCK_LLM_ACCESS_KEY_ID"`
	AWSBedrockSecretKey    string `env:"AWS_BEDROCK_LLM_ACCESS_KEY"`
	AWSBedrockRegion       string `env:"AWS_BEDROCK_LLM_REGION"`
	AWSBedrockSessionToken string `env:"AWS_BEDROCK_LLM_SESSION_TOKEN"`
	AWSBedrockModelPref    string `env:"AWS_BEDROCK_LLM_MODEL_PREFERENCE"`

	// === DeepSeek ===
	DeepSeekApiKey    string `env:"DEEPSEEK_API_KEY"`
	DeepSeekModelPref string `env:"DEEPSEEK_MODEL_PREF"`

	// === ApiPie ===
	ApiPieApiKey    string `env:"APIPIE_API_KEY"`
	ApiPieModelPref string `env:"APIPIE_MODEL_PREF"`

	// === XAI ===
	XaiApiKey    string `env:"XAI_API_KEY"`
	XaiModelPref string `env:"XAI_MODEL_PREF"`

	// === NVIDIA NIM ===
	NvidiaNimApiKey    string `env:"NVIDIA_NIM_API_KEY"`
	NvidiaNimModelPref string `env:"NVIDIA_NIM_MODEL_PREF"`

	// === PPIO ===
	PpioApiKey    string `env:"PPIO_API_KEY"`
	PpioModelPref string `env:"PPIO_MODEL_PREF"`

	// === Dell Pro AI Studio ===
	DellProBasePath   string `env:"DELL_PRO_AI_STUDIO_BASE_PATH"`
	DellProModelPref  string `env:"DELL_PRO_AI_STUDIO_MODEL_PREF"`
	DellProTokenLimit int    `env:"DELL_PRO_AI_STUDIO_MODEL_TOKEN_LIMIT"`

	// === MoonshotAI (Kimi) ===
	MoonshotApiKey    string `env:"MOONSHOT_API_KEY"`
	MoonshotModelPref string `env:"MOONSHOT_MODEL_PREF"`

	// === CometAPI ===
	CometApiKey    string `env:"COMET_API_KEY"`
	CometApiModelPref string `env:"COMET_API_MODEL_PREF"`

	// === Foundry ===
	FoundryApiKey    string `env:"FOUNDRY_API_KEY"`
	FoundryModelPref string `env:"FOUNDRY_MODEL_PREF"`

	// === ZAI ===
	ZaiApiKey    string `env:"ZAI_API_KEY"`
	ZaiModelPref string `env:"ZAI_MODEL_PREF"`

	// === GiteeAI ===
	GiteeApiKey    string `env:"GITEE_API_KEY"`
	GiteeModelPref string `env:"GITEE_MODEL_PREF"`

	// === Docker Model Runner ===
	DockerModelBasePath  string `env:"DOCKER_MODEL_RUNNER_BASE_PATH"`
	DockerModelModelPref string `env:"DOCKER_MODEL_RUNNER_MODEL_PREF"`

	// === PrivateMode ===
	PrivateModeApiKey    string `env:"PRIVATE_MODE_API_KEY"`
	PrivateModeModelPref string `env:"PRIVATE_MODE_MODEL_PREF"`

	// === SambaNova ===
	SambaNovaApiKey    string `env:"SAMBANOVA_API_KEY"`
	SambaNovaModelPref string `env:"SAMBANOVA_MODEL_PREF"`

	// === Lemonade ===
	LemonadeApiKey    string `env:"LEMONADE_API_KEY"`
	LemonadeModelPref string `env:"LEMONADE_MODEL_PREF"`
```

- [ ] **Step 2: Verify config compiles**

```bash
cd backend && go build -tags "fts5 nolancedb" ./internal/config/...
```

Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add backend/internal/config/config.go
git commit -m "feat(config): add 36 LLM provider-specific configuration fields"
```

---

## Task 4: builders.go — Batch 1 (OpenAI, Azure, Anthropic, Gemini, LMStudio)

**Files:**
- Create: `backend/internal/providers/builders.go`
- Create: `backend/internal/providers/builders_test.go` (skeleton, filled in Task 10-11)

- [ ] **Step 1: Write builders.go header and imports**

```go
package providers

import (
	"context"
	"fmt"
	"strings"

	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/mlog"
	"github.com/odysseythink/pantheon/core"
	pantheonAnthropic "github.com/odysseythink/pantheon/providers/anthropic"
	pantheonAzure "github.com/odysseythink/pantheon/providers/azure"
	pantheonGoogle "github.com/odysseythink/pantheon/providers/google"
	pantheonLMStudio "github.com/odysseythink/pantheon/providers/lmstudio"
	pantheonOllama "github.com/odysseythink/pantheon/providers/ollama"
	pantheonOpenAI "github.com/odysseythink/pantheon/providers/openai"
)
```

**Note:** More imports will be added in subsequent tasks. Keep them alphabetically ordered by package alias.

- [ ] **Step 2: Register Batch 1 in providerRegistry**

```go
func init() {
	providerRegistry["openai"] = buildOpenAI
	providerRegistry["azure"] = buildAzure
	providerRegistry["anthropic"] = buildAnthropic
	providerRegistry["gemini"] = buildGoogle
	providerRegistry["lmstudio"] = buildLMStudio
}
```

- [ ] **Step 3: Write buildOpenAI**

```go
func buildOpenAI(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error) {
	apiKey := firstNonEmpty(
		settings["OpenAiKey"],
		cfg.OpenAiKey,
		cfg.LLMApiKey,
	)
	if apiKey == "" {
		return nil, fmt.Errorf("no OpenAI API key configured")
	}
	p, err := pantheonOpenAI.New(apiKey)
	if err != nil {
		return nil, err
	}
	return p.LanguageModel(ctx, modelID)
}
```

- [ ] **Step 4: Write buildAzure**

```go
func buildAzure(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error) {
	apiKey := firstNonEmpty(
		settings["AzureOpenAiKey"],
		cfg.AzureOpenAiKey,
		cfg.LLMApiKey,
	)
	resourceName := firstNonEmpty(
		settings["AzureOpenAiResourceName"],
		cfg.AzureOpenAiResourceName,
	)
	deployment := firstNonEmpty(
		settings["AzureOpenAiDeployment"],
		cfg.AzureOpenAiDeployment,
	)

	if resourceName == "" || deployment == "" {
		endpoint := firstNonEmpty(
			settings["AzureOpenAiEndpoint"],
			cfg.AzureOpenAiEndpoint,
		)
		if endpoint != "" {
			rn, dep, err := parseAzureEndpoint(endpoint)
			if err == nil {
				if resourceName == "" {
					resourceName = rn
				}
				if deployment == "" {
					deployment = dep
				}
			} else {
				mlog.Warning("buildAzure: failed to parse endpoint: ", err)
			}
		}
	}

	if apiKey == "" || resourceName == "" || deployment == "" {
		return nil, fmt.Errorf("azure: apiKey, resourceName, and deployment are required")
	}

	opts := []pantheonAzure.Option{}
	if endpoint := cfg.AzureOpenAiEndpoint; endpoint != "" {
		opts = append(opts, pantheonAzure.WithBaseURL(endpoint))
	}

	p, err := pantheonAzure.New(apiKey, resourceName, deployment, opts...)
	if err != nil {
		return nil, err
	}
	return p.LanguageModel(ctx, modelID)
}
```

- [ ] **Step 5: Write buildAnthropic**

```go
func buildAnthropic(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error) {
	apiKey := firstNonEmpty(
		settings["AnthropicApiKey"],
		cfg.AnthropicApiKey,
		cfg.LLMApiKey,
	)
	if apiKey == "" {
		return nil, fmt.Errorf("no Anthropic API key configured")
	}
	p, err := pantheonAnthropic.New(apiKey)
	if err != nil {
		return nil, err
	}
	return p.LanguageModel(ctx, modelID)
}
```

- [ ] **Step 6: Write buildGoogle (Gemini)**

```go
func buildGoogle(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error) {
	apiKey := firstNonEmpty(
		settings["GeminiLLMApiKey"],
		cfg.GeminiApiKey,
		cfg.LLMApiKey,
	)
	if apiKey == "" {
		return nil, fmt.Errorf("no Gemini API key configured")
	}
	p, err := pantheonGoogle.New(apiKey)
	if err != nil {
		return nil, err
	}
	return p.LanguageModel(ctx, modelID)
}
```

- [ ] **Step 7: Write buildLMStudio**

```go
func buildLMStudio(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error) {
	baseURL := firstNonEmpty(
		settings["LMStudioBasePath"],
		cfg.LMStudioBasePath,
	)
	if baseURL == "" {
		return nil, fmt.Errorf("no LMStudio base path configured")
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	p, err := pantheonLMStudio.New("", pantheonLMStudio.WithBaseURL(baseURL))
	if err != nil {
		return nil, err
	}
	return p.LanguageModel(ctx, modelID)
}
```

- [ ] **Step 8: Verify build**

```bash
cd backend && go build -tags "fts5 nolancedb" ./internal/providers/...
```

Expected: no errors

- [ ] **Step 9: Commit**

```bash
git add backend/internal/providers/builders.go
git commit -m "feat(providers): add Batch 1 builders (openai, azure, anthropic, gemini, lmstudio)"
```

---

## Task 5: builders.go — Batch 2 (LocalAI, Ollama, TogetherAI, FireworksAI, Mistral, HuggingFace)

**Files:**
- Modify: `backend/internal/providers/builders.go`

- [ ] **Step 1: Add imports**

Add to the import block:
```go
	pantheonFireworks "github.com/odysseythink/pantheon/providers/fireworks"
	pantheonHuggingFace "github.com/odysseythink/pantheon/providers/huggingface"
	pantheonLocalAI "github.com/odysseythink/pantheon/providers/localai"
	pantheonMistral "github.com/odysseythink/pantheon/providers/mistral"
	pantheonTogether "github.com/odysseythink/pantheon/providers/together"
```

- [ ] **Step 2: Register Batch 2**

Add to `init()`:
```go
	providerRegistry["localai"] = buildLocalAI
	providerRegistry["ollama"] = buildOllama
	providerRegistry["togetherai"] = buildTogether
	providerRegistry["fireworksai"] = buildFireworks
	providerRegistry["mistral"] = buildMistral
	providerRegistry["huggingface"] = buildHuggingFace
```

- [ ] **Step 3: Write buildLocalAI**

```go
func buildLocalAI(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error) {
	baseURL := firstNonEmpty(
		settings["LocalAiBasePath"],
		cfg.LocalAiBasePath,
	)
	if baseURL == "" {
		return nil, fmt.Errorf("no LocalAI base path configured")
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	apiKey := firstNonEmpty(
		settings["LocalAiApiKey"],
		cfg.LocalAiApiKey,
	)

	opts := []pantheonLocalAI.Option{pantheonLocalAI.WithBaseURL(baseURL)}
	p, err := pantheonLocalAI.New(apiKey, opts...)
	if err != nil {
		return nil, err
	}
	return p.LanguageModel(ctx, modelID)
}
```

- [ ] **Step 4: Write buildOllama**

```go
func buildOllama(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error) {
	baseURL := firstNonEmpty(
		settings["OllamaLLMBasePath"],
		cfg.OllamaBasePath,
	)
	if baseURL == "" {
		baseURL = "http://127.0.0.1:11434"
	}
	baseURL = strings.TrimSuffix(baseURL, "/")
	baseURL = strings.TrimSuffix(baseURL, "/v1")

	p, err := pantheonOllama.New("", pantheonOllama.WithBaseURL(baseURL))
	if err != nil {
		return nil, err
	}
	return p.LanguageModel(ctx, modelID)
}
```

- [ ] **Step 5: Write buildTogether**

```go
func buildTogether(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error) {
	apiKey := firstNonEmpty(
		settings["TogetherAiApiKey"],
		cfg.TogetherAiApiKey,
		cfg.LLMApiKey,
	)
	if apiKey == "" {
		return nil, fmt.Errorf("no TogetherAI API key configured")
	}
	p, err := pantheonTogether.New(apiKey)
	if err != nil {
		return nil, err
	}
	return p.LanguageModel(ctx, modelID)
}
```

- [ ] **Step 6: Write buildFireworks**

```go
func buildFireworks(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error) {
	apiKey := firstNonEmpty(
		settings["FireworksApiKey"],
		cfg.FireworksApiKey,
		cfg.LLMApiKey,
	)
	if apiKey == "" {
		return nil, fmt.Errorf("no Fireworks API key configured")
	}
	p, err := pantheonFireworks.New(apiKey)
	if err != nil {
		return nil, err
	}
	return p.LanguageModel(ctx, modelID)
}
```

- [ ] **Step 7: Write buildMistral**

```go
func buildMistral(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error) {
	apiKey := firstNonEmpty(
		settings["MistralApiKey"],
		cfg.MistralApiKey,
		cfg.LLMApiKey,
	)
	if apiKey == "" {
		return nil, fmt.Errorf("no Mistral API key configured")
	}
	p, err := pantheonMistral.New(apiKey)
	if err != nil {
		return nil, err
	}
	return p.LanguageModel(ctx, modelID)
}
```

- [ ] **Step 8: Write buildHuggingFace**

```go
func buildHuggingFace(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error) {
	endpoint := firstNonEmpty(
		settings["HuggingFaceLLMEndpoint"],
		cfg.HuggingFaceEndpoint,
	)
	if endpoint == "" {
		return nil, fmt.Errorf("no HuggingFace endpoint configured")
	}
	apiKey := firstNonEmpty(
		settings["HuggingFaceLLMAccessToken"],
		cfg.HuggingFaceApiKey,
		cfg.LLMApiKey,
	)

	opts := []pantheonHuggingFace.Option{pantheonHuggingFace.WithBaseURL(endpoint)}
	p, err := pantheonHuggingFace.New(apiKey, opts...)
	if err != nil {
		return nil, err
	}
	return p.LanguageModel(ctx, modelID)
}
```

- [ ] **Step 9: Verify build**

```bash
cd backend && go build -tags "fts5 nolancedb" ./internal/providers/...
```

Expected: no errors

- [ ] **Step 10: Commit**

```bash
git add backend/internal/providers/builders.go
git commit -m "feat(providers): add Batch 2 builders (localai, ollama, togetherai, fireworksai, mistral, huggingface)"
```

---

## Task 6: builders.go — Batch 3 (Perplexity, OpenRouter, Novita, Groq, KoboldCPP, TextGenWebUI)

**Files:**
- Modify: `backend/internal/providers/builders.go`

- [ ] **Step 1: Add imports**

```go
	pantheonGroq "github.com/odysseythink/pantheon/providers/groq"
	pantheonKoboldCPP "github.com/odysseythink/pantheon/providers/koboldcpp"
	pantheonNovita "github.com/odysseythink/pantheon/providers/novita"
	pantheonOpenRouter "github.com/odysseythink/pantheon/providers/openrouter"
	pantheonPerplexity "github.com/odysseythink/pantheon/providers/perplexity"
	pantheonTextGenWebUI "github.com/odysseythink/pantheon/providers/textgenwebui"
```

- [ ] **Step 2: Register Batch 3**

```go
	providerRegistry["perplexity"] = buildPerplexity
	providerRegistry["openrouter"] = buildOpenRouter
	providerRegistry["novita"] = buildNovita
	providerRegistry["groq"] = buildGroq
	providerRegistry["koboldcpp"] = buildKoboldCPP
	providerRegistry["textgenwebui"] = buildTextGenWebUI
```

- [ ] **Step 3: Write buildPerplexity**

```go
func buildPerplexity(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error) {
	apiKey := firstNonEmpty(
		settings["PerplexityApiKey"],
		cfg.PerplexityApiKey,
		cfg.LLMApiKey,
	)
	if apiKey == "" {
		return nil, fmt.Errorf("no Perplexity API key configured")
	}
	p, err := pantheonPerplexity.New(apiKey)
	if err != nil {
		return nil, err
	}
	return p.LanguageModel(ctx, modelID)
}
```

- [ ] **Step 4: Write buildOpenRouter**

```go
func buildOpenRouter(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error) {
	apiKey := firstNonEmpty(
		settings["OpenRouterApiKey"],
		cfg.OpenRouterApiKey,
		cfg.LLMApiKey,
	)
	if apiKey == "" {
		return nil, fmt.Errorf("no OpenRouter API key configured")
	}
	p, err := pantheonOpenRouter.New(apiKey)
	if err != nil {
		return nil, err
	}
	return p.LanguageModel(ctx, modelID)
}
```

- [ ] **Step 5: Write buildNovita**

```go
func buildNovita(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error) {
	apiKey := firstNonEmpty(
		settings["NovitaApiKey"],
		cfg.NovitaApiKey,
		cfg.LLMApiKey,
	)
	if apiKey == "" {
		return nil, fmt.Errorf("no Novita API key configured")
	}
	p, err := pantheonNovita.New(apiKey)
	if err != nil {
		return nil, err
	}
	return p.LanguageModel(ctx, modelID)
}
```

- [ ] **Step 6: Write buildGroq**

```go
func buildGroq(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error) {
	apiKey := firstNonEmpty(
		settings["GroqApiKey"],
		cfg.GroqApiKey,
		cfg.LLMApiKey,
	)
	if apiKey == "" {
		return nil, fmt.Errorf("no Groq API key configured")
	}
	p, err := pantheonGroq.New(apiKey)
	if err != nil {
		return nil, err
	}
	return p.LanguageModel(ctx, modelID)
}
```

- [ ] **Step 7: Write buildKoboldCPP**

```go
func buildKoboldCPP(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error) {
	baseURL := firstNonEmpty(
		settings["KoboldCPPBasePath"],
		cfg.KoboldBasePath,
	)
	if baseURL == "" {
		return nil, fmt.Errorf("no KoboldCPP base path configured")
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	apiKey := firstNonEmpty(
		settings["KoboldCPPApiKey"],
		cfg.LLMApiKey,
	)

	opts := []pantheonKoboldCPP.Option{pantheonKoboldCPP.WithBaseURL(baseURL)}
	p, err := pantheonKoboldCPP.New(apiKey, opts...)
	if err != nil {
		return nil, err
	}
	return p.LanguageModel(ctx, modelID)
}
```

- [ ] **Step 8: Write buildTextGenWebUI**

```go
func buildTextGenWebUI(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error) {
	baseURL := firstNonEmpty(
		settings["TextGenWebUIBasePath"],
		cfg.TextGenBasePath,
	)
	if baseURL == "" {
		return nil, fmt.Errorf("no TextGenWebUI base path configured")
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	apiKey := firstNonEmpty(
		settings["TextGenWebUIAPIKey"],
		cfg.TextGenApiKey,
		cfg.LLMApiKey,
	)

	opts := []pantheonTextGenWebUI.Option{pantheonTextGenWebUI.WithBaseURL(baseURL)}
	p, err := pantheonTextGenWebUI.New(apiKey, opts...)
	if err != nil {
		return nil, err
	}
	return p.LanguageModel(ctx, modelID)
}
```

- [ ] **Step 9: Verify build**

```bash
cd backend && go build -tags "fts5 nolancedb" ./internal/providers/...
```

- [ ] **Step 10: Commit**

```bash
git add backend/internal/providers/builders.go
git commit -m "feat(providers): add Batch 3 builders (perplexity, openrouter, novita, groq, koboldcpp, textgenwebui)"
```

---

## Task 7: builders.go — Batch 4 (Cohere, LiteLLM, GenericOpenAI, DeepSeek, ApiPie, XAI)

**Files:**
- Modify: `backend/internal/providers/builders.go`

- [ ] **Step 1: Add imports**

```go
	pantheonApiPie "github.com/odysseythink/pantheon/providers/apipie"
	pantheonCohere "github.com/odysseythink/pantheon/providers/cohere"
	pantheonDeepSeek "github.com/odysseythink/pantheon/providers/deepseek"
	pantheonGenericOpenAI "github.com/odysseythink/pantheon/providers/genericopenai"
	pantheonLiteLLM "github.com/odysseythink/pantheon/providers/litellm"
	pantheonXAI "github.com/odysseythink/pantheon/providers/xai"
```

- [ ] **Step 2: Register Batch 4**

```go
	providerRegistry["cohere"] = buildCohere
	providerRegistry["litellm"] = buildLiteLLM
	providerRegistry["generic-openai"] = buildGenericOpenAI
	providerRegistry["deepseek"] = buildDeepSeek
	providerRegistry["apipie"] = buildApiPie
	providerRegistry["xai"] = buildXAI
```

- [ ] **Step 3: Write buildCohere**

```go
func buildCohere(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error) {
	apiKey := firstNonEmpty(
		settings["CohereApiKey"],
		cfg.CohereApiKey,
		cfg.LLMApiKey,
	)
	if apiKey == "" {
		return nil, fmt.Errorf("no Cohere API key configured")
	}
	p, err := pantheonCohere.New(apiKey)
	if err != nil {
		return nil, err
	}
	return p.LanguageModel(ctx, modelID)
}
```

- [ ] **Step 4: Write buildLiteLLM**

```go
func buildLiteLLM(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error) {
	baseURL := firstNonEmpty(
		settings["LiteLLMBasePath"],
		cfg.LiteLLMBasePath,
	)
	if baseURL == "" {
		return nil, fmt.Errorf("no LiteLLM base path configured")
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	apiKey := firstNonEmpty(
		settings["LiteLLMApiKey"],
		cfg.LiteLLMApiKey,
		cfg.LLMApiKey,
	)

	opts := []pantheonLiteLLM.Option{pantheonLiteLLM.WithBaseURL(baseURL)}
	p, err := pantheonLiteLLM.New(apiKey, opts...)
	if err != nil {
		return nil, err
	}
	return p.LanguageModel(ctx, modelID)
}
```

- [ ] **Step 5: Write buildGenericOpenAI**

```go
func buildGenericOpenAI(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error) {
	baseURL := firstNonEmpty(
		settings["GenericOpenAiBasePath"],
		cfg.GenericOpenAiBasePath,
	)
	if baseURL == "" {
		return nil, fmt.Errorf("no GenericOpenAI base path configured")
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	apiKey := firstNonEmpty(
		settings["GenericOpenAiApiKey"],
		cfg.GenericOpenAiApiKey,
		cfg.LLMApiKey,
	)

	opts := []pantheonGenericOpenAI.Option{pantheonGenericOpenAI.WithBaseURL(baseURL)}
	p, err := pantheonGenericOpenAI.New(apiKey, opts...)
	if err != nil {
		return nil, err
	}
	return p.LanguageModel(ctx, modelID)
}
```

- [ ] **Step 6: Write buildDeepSeek**

```go
func buildDeepSeek(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error) {
	apiKey := firstNonEmpty(
		settings["DeepSeekApiKey"],
		cfg.DeepSeekApiKey,
		cfg.LLMApiKey,
	)
	if apiKey == "" {
		return nil, fmt.Errorf("no DeepSeek API key configured")
	}
	p, err := pantheonDeepSeek.New(apiKey)
	if err != nil {
		return nil, err
	}
	return p.LanguageModel(ctx, modelID)
}
```

- [ ] **Step 7: Write buildApiPie**

```go
func buildApiPie(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error) {
	apiKey := firstNonEmpty(
		settings["ApiPieApiKey"],
		cfg.ApiPieApiKey,
		cfg.LLMApiKey,
	)
	if apiKey == "" {
		return nil, fmt.Errorf("no ApiPie API key configured")
	}
	p, err := pantheonApiPie.New(apiKey)
	if err != nil {
		return nil, err
	}
	return p.LanguageModel(ctx, modelID)
}
```

- [ ] **Step 8: Write buildXAI**

```go
func buildXAI(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error) {
	apiKey := firstNonEmpty(
		settings["XaiApiKey"],
		cfg.XaiApiKey,
		cfg.LLMApiKey,
	)
	if apiKey == "" {
		return nil, fmt.Errorf("no XAI API key configured")
	}
	p, err := pantheonXAI.New(apiKey)
	if err != nil {
		return nil, err
	}
	return p.LanguageModel(ctx, modelID)
}
```

- [ ] **Step 9: Verify build**

```bash
cd backend && go build -tags "fts5 nolancedb" ./internal/providers/...
```

- [ ] **Step 10: Commit**

```bash
git add backend/internal/providers/builders.go
git commit -m "feat(providers): add Batch 4 builders (cohere, litellm, generic-openai, deepseek, apipie, xai)"
```

---

## Task 8: builders.go — Batch 5 (NvidiaNIM, PPIO, DellPro, Moonshot, CometAPI, Foundry)

**Files:**
- Modify: `backend/internal/providers/builders.go`

- [ ] **Step 1: Add imports**

```go
	pantheonCometAPI "github.com/odysseythink/pantheon/providers/cometapi"
	pantheonDellPro "github.com/odysseythink/pantheon/providers/dellproaistudio"
	pantheonFoundry "github.com/odysseythink/pantheon/providers/foundry"
	pantheonKimi "github.com/odysseythink/pantheon/providers/kimi"
	pantheonNvidiaNIM "github.com/odysseythink/pantheon/providers/nvidianim"
	pantheonPPIO "github.com/odysseythink/pantheon/providers/ppio"
```

- [ ] **Step 2: Register Batch 5**

```go
	providerRegistry["nvidia-nim"] = buildNvidiaNIM
	providerRegistry["ppio"] = buildPPIO
	providerRegistry["dpaiStudio"] = buildDellPro
	providerRegistry["moonshotai"] = buildMoonshot
	providerRegistry["cometapi"] = buildCometAPI
	providerRegistry["foundry"] = buildFoundry
```

- [ ] **Step 3: Write buildNvidiaNIM**

```go
func buildNvidiaNIM(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error) {
	apiKey := firstNonEmpty(
		settings["NvidiaNimApiKey"],
		cfg.NvidiaNimApiKey,
		cfg.LLMApiKey,
	)
	if apiKey == "" {
		return nil, fmt.Errorf("no NVIDIA NIM API key configured")
	}
	p, err := pantheonNvidiaNIM.New(apiKey)
	if err != nil {
		return nil, err
	}
	return p.LanguageModel(ctx, modelID)
}
```

- [ ] **Step 4: Write buildPPIO**

```go
func buildPPIO(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error) {
	apiKey := firstNonEmpty(
		settings["PpioApiKey"],
		cfg.PpioApiKey,
		cfg.LLMApiKey,
	)
	if apiKey == "" {
		return nil, fmt.Errorf("no PPIO API key configured")
	}
	p, err := pantheonPPIO.New(apiKey)
	if err != nil {
		return nil, err
	}
	return p.LanguageModel(ctx, modelID)
}
```

- [ ] **Step 5: Write buildDellPro**

```go
func buildDellPro(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error) {
	baseURL := firstNonEmpty(
		settings["DellProBasePath"],
		cfg.DellProBasePath,
	)
	if baseURL == "" {
		return nil, fmt.Errorf("no Dell Pro AI Studio base path configured")
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	apiKey := firstNonEmpty(
		settings["DellProApiKey"],
		cfg.LLMApiKey,
	)

	opts := []pantheonDellPro.Option{pantheonDellPro.WithBaseURL(baseURL)}
	p, err := pantheonDellPro.New(apiKey, opts...)
	if err != nil {
		return nil, err
	}
	return p.LanguageModel(ctx, modelID)
}
```

- [ ] **Step 6: Write buildMoonshot (Kimi)**

```go
func buildMoonshot(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error) {
	apiKey := firstNonEmpty(
		settings["MoonshotApiKey"],
		cfg.MoonshotApiKey,
		cfg.LLMApiKey,
	)
	if apiKey == "" {
		return nil, fmt.Errorf("no Moonshot API key configured")
	}
	p, err := pantheonKimi.New(apiKey)
	if err != nil {
		return nil, err
	}
	return p.LanguageModel(ctx, modelID)
}
```

- [ ] **Step 7: Write buildCometAPI**

```go
func buildCometAPI(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error) {
	apiKey := firstNonEmpty(
		settings["CometApiKey"],
		cfg.CometApiKey,
		cfg.LLMApiKey,
	)
	if apiKey == "" {
		return nil, fmt.Errorf("no CometAPI API key configured")
	}
	p, err := pantheonCometAPI.New(apiKey)
	if err != nil {
		return nil, err
	}
	return p.LanguageModel(ctx, modelID)
}
```

- [ ] **Step 8: Write buildFoundry**

```go
func buildFoundry(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error) {
	apiKey := firstNonEmpty(
		settings["FoundryApiKey"],
		cfg.FoundryApiKey,
		cfg.LLMApiKey,
	)
	if apiKey == "" {
		return nil, fmt.Errorf("no Foundry API key configured")
	}
	p, err := pantheonFoundry.New(apiKey)
	if err != nil {
		return nil, err
	}
	return p.LanguageModel(ctx, modelID)
}
```

- [ ] **Step 9: Verify build**

```bash
cd backend && go build -tags "fts5 nolancedb" ./internal/providers/...
```

- [ ] **Step 10: Commit**

```bash
git add backend/internal/providers/builders.go
git commit -m "feat(providers): add Batch 5 builders (nvidia-nim, ppio, dellpro, moonshot, cometapi, foundry)"
```

---

## Task 9: builders.go — Batch 6 (ZAI, GiteeAI, DockerModelRunner, PrivateMode, SambaNova, Lemonade) + Bedrock

**Files:**
- Modify: `backend/internal/providers/builders.go`

- [ ] **Step 1: Add imports**

```go
	pantheonBedrock "github.com/odysseythink/pantheon/providers/bedrock"
	pantheonDockerModelRunner "github.com/odysseythink/pantheon/providers/dockermodelrunner"
	pantheonGiteeAI "github.com/odysseythink/pantheon/providers/giteeai"
	pantheonLemonade "github.com/odysseythink/pantheon/providers/lemonade"
	pantheonPrivateMode "github.com/odysseythink/pantheon/providers/privatemode"
	pantheonSambaNova "github.com/odysseythink/pantheon/providers/sambanova"
	pantheonZAI "github.com/odysseythink/pantheon/providers/zai"
```

- [ ] **Step 2: Register Batch 6 + Bedrock**

```go
	providerRegistry["zai"] = buildZAI
	providerRegistry["giteeai"] = buildGiteeAI
	providerRegistry["docker-model-runner"] = buildDockerModelRunner
	providerRegistry["privatemode"] = buildPrivateMode
	providerRegistry["sambanova"] = buildSambaNova
	providerRegistry["lemonade"] = buildLemonade
	providerRegistry["bedrock"] = buildBedrock
```

- [ ] **Step 3: Write buildZAI**

```go
func buildZAI(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error) {
	apiKey := firstNonEmpty(
		settings["ZaiApiKey"],
		cfg.ZaiApiKey,
		cfg.LLMApiKey,
	)
	if apiKey == "" {
		return nil, fmt.Errorf("no ZAI API key configured")
	}
	p, err := pantheonZAI.New(apiKey)
	if err != nil {
		return nil, err
	}
	return p.LanguageModel(ctx, modelID)
}
```

- [ ] **Step 4: Write buildGiteeAI**

```go
func buildGiteeAI(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error) {
	apiKey := firstNonEmpty(
		settings["GiteeApiKey"],
		cfg.GiteeApiKey,
		cfg.LLMApiKey,
	)
	if apiKey == "" {
		return nil, fmt.Errorf("no GiteeAI API key configured")
	}
	p, err := pantheonGiteeAI.New(apiKey)
	if err != nil {
		return nil, err
	}
	return p.LanguageModel(ctx, modelID)
}
```

- [ ] **Step 5: Write buildDockerModelRunner**

```go
func buildDockerModelRunner(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error) {
	baseURL := firstNonEmpty(
		settings["DockerModelRunnerBasePath"],
		cfg.DockerModelBasePath,
	)
	if baseURL == "" {
		return nil, fmt.Errorf("no Docker Model Runner base path configured")
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	apiKey := firstNonEmpty(
		settings["DockerModelRunnerApiKey"],
		cfg.LLMApiKey,
	)

	opts := []pantheonDockerModelRunner.Option{pantheonDockerModelRunner.WithBaseURL(baseURL)}
	p, err := pantheonDockerModelRunner.New(apiKey, opts...)
	if err != nil {
		return nil, err
	}
	return p.LanguageModel(ctx, modelID)
}
```

- [ ] **Step 6: Write buildPrivateMode**

```go
func buildPrivateMode(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error) {
	apiKey := firstNonEmpty(
		settings["PrivateModeApiKey"],
		cfg.PrivateModeApiKey,
		cfg.LLMApiKey,
	)
	if apiKey == "" {
		return nil, fmt.Errorf("no PrivateMode API key configured")
	}
	p, err := pantheonPrivateMode.New(apiKey)
	if err != nil {
		return nil, err
	}
	return p.LanguageModel(ctx, modelID)
}
```

- [ ] **Step 7: Write buildSambaNova**

```go
func buildSambaNova(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error) {
	apiKey := firstNonEmpty(
		settings["SambaNovaApiKey"],
		cfg.SambaNovaApiKey,
		cfg.LLMApiKey,
	)
	if apiKey == "" {
		return nil, fmt.Errorf("no SambaNova API key configured")
	}
	p, err := pantheonSambaNova.New(apiKey)
	if err != nil {
		return nil, err
	}
	return p.LanguageModel(ctx, modelID)
}
```

- [ ] **Step 8: Write buildLemonade**

```go
func buildLemonade(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error) {
	apiKey := firstNonEmpty(
		settings["LemonadeApiKey"],
		cfg.LemonadeApiKey,
		cfg.LLMApiKey,
	)
	if apiKey == "" {
		return nil, fmt.Errorf("no Lemonade API key configured")
	}
	p, err := pantheonLemonade.New(apiKey)
	if err != nil {
		return nil, err
	}
	return p.LanguageModel(ctx, modelID)
}
```

- [ ] **Step 9: Write buildBedrock**

```go
func buildBedrock(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error) {
	accessKeyID := firstNonEmpty(
		settings["AWSBedrockAccessKeyID"],
		cfg.AWSBedrockAccessKeyID,
	)
	secretKey := firstNonEmpty(
		settings["AWSBedrockSecretKey"],
		cfg.AWSBedrockSecretKey,
	)
	region := firstNonEmpty(
		settings["AWSBedrockRegion"],
		cfg.AWSBedrockRegion,
	)
	sessionToken := firstNonEmpty(
		settings["AWSBedrockSessionToken"],
		cfg.AWSBedrockSessionToken,
	)

	if accessKeyID == "" || secretKey == "" || region == "" {
		return nil, fmt.Errorf("bedrock: accessKeyID, secretKey, and region are required")
	}

	opts := []pantheonBedrock.Option{}
	if sessionToken != "" {
		opts = append(opts, pantheonBedrock.WithSessionToken(sessionToken))
	}

	p, err := pantheonBedrock.New(accessKeyID, secretKey, region, opts...)
	if err != nil {
		return nil, err
	}
	return p.LanguageModel(ctx, modelID)
}
```

- [ ] **Step 10: Verify build**

```bash
cd backend && go build -tags "fts5 nolancedb" ./internal/providers/...
```

- [ ] **Step 11: Commit**

```bash
git add backend/internal/providers/builders.go
git commit -m "feat(providers): add Batch 6 builders + Bedrock (zai, giteeai, dockermodelrunner, privatemode, sambanova, lemonade, bedrock)"
```

---

## Task 10: builders_test.go — Registry 完整性 + Missing-Key 测试

**Files:**
- Create: `backend/internal/providers/builders_test.go`

- [ ] **Step 1: Write registry completeness test**

```go
package providers

import (
	"context"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProviderRegistry_Complete(t *testing.T) {
	expected := []string{
		"openai",
		"azure",
		"anthropic",
		"gemini",
		"lmstudio",
		"localai",
		"ollama",
		"togetherai",
		"fireworksai",
		"mistral",
		"huggingface",
		"perplexity",
		"openrouter",
		"novita",
		"groq",
		"koboldcpp",
		"textgenwebui",
		"cohere",
		"litellm",
		"generic-openai",
		"bedrock",
		"deepseek",
		"apipie",
		"xai",
		"nvidia-nim",
		"ppio",
		"dpaiStudio",
		"moonshotai",
		"cometapi",
		"foundry",
		"zai",
		"giteeai",
		"docker-model-runner",
		"privatemode",
		"sambanova",
		"lemonade",
	}

	for _, name := range expected {
		t.Run(name, func(t *testing.T) {
			b, ok := providerRegistry[name]
			require.True(t, ok, "provider %s not in registry", name)
			require.NotNil(t, b)
		})
	}

	assert.Equal(t, len(expected), len(providerRegistry), "registry size mismatch: expected %d, got %d", len(expected), len(providerRegistry))
}
```

- [ ] **Step 2: Write missing-key tests for standard API-key providers**

Use a table-driven test for all providers that require an API key. The test verifies that when no key is configured, the builder returns an error containing "no .* API key configured" or "no .* base path configured".

```go
func TestBuilders_MissingConfig(t *testing.T) {
	cfg := &config.Config{}
	settings := map[string]string{}
	ctx := context.Background()

	// Standard API-key providers
	apiKeyProviders := []struct {
		name    string
		modelID string
	}{
		{"openai", "gpt-4o"},
		{"anthropic", "claude-3"},
		{"gemini", "gemini-pro"},
		{"togetherai", "llama-3"},
		{"fireworksai", "llama-3"},
		{"mistral", "mistral-large"},
		{"perplexity", "llama-3"},
		{"openrouter", "gpt-4o"},
		{"novita", "llama-3"},
		{"groq", "llama-3"},
		{"cohere", "command-r"},
		{"deepseek", "deepseek-chat"},
		{"apipie", "gpt-4o"},
		{"xai", "grok-beta"},
		{"nvidia-nim", "llama-3"},
		{"ppio", "llama-3"},
		{"moonshotai", "moonshot-v1"},
		{"cometapi", "gpt-4o"},
		{"foundry", "gpt-4o"},
		{"zai", "gpt-4o"},
		{"giteeai", "gpt-4o"},
		{"privatemode", "gpt-4o"},
		{"sambanova", "llama-3"},
		{"lemonade", "gpt-4o"},
	}

	for _, tc := range apiKeyProviders {
		t.Run(tc.name+"_missing_api_key", func(t *testing.T) {
			builder, ok := providerRegistry[tc.name]
			require.True(t, ok, "provider %s not found in registry", tc.name)
			_, err := builder(ctx, cfg, settings, tc.modelID)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "configured")
		})
	}

	// BaseURL-required providers
	baseURLProviders := []struct {
		name    string
		modelID string
	}{
		{"lmstudio", "local-model"},
		{"localai", "local-model"},
		{"ollama", "llama3"},
		{"koboldcpp", "local-model"},
		{"textgenwebui", "local-model"},
		{"huggingface", "tgi-model"},
		{"litellm", "local-model"},
		{"generic-openai", "local-model"},
		{"dellproaistudio", "local-model"},
		{"docker-model-runner", "local-model"},
	}

	for _, tc := range baseURLProviders {
		t.Run(tc.name+"_missing_base_url", func(t *testing.T) {
			builder, ok := providerRegistry[tc.name]
			require.True(t, ok, "provider %s not found in registry", tc.name)
			_, err := builder(ctx, cfg, settings, tc.modelID)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "configured")
		})
	}
}
```

- [ ] **Step 3: Run registry + missing-key tests**

```bash
cd backend && go test -tags "fts5 nolancedb" ./internal/providers/ -run "TestProviderRegistry_Complete|TestBuilders_MissingConfig" -v
```

Expected: PASS (1 + ~34 subtests)

- [ ] **Step 4: Commit**

```bash
git add backend/internal/providers/builders_test.go
git commit -m "test(providers): add registry completeness and missing-config tests"
```

---

## Task 11: builders_test.go — Azure + Bedrock 边界测试

**Files:**
- Modify: `backend/internal/providers/builders_test.go`

- [ ] **Step 1: Write Azure endpoint parsing test**

Append to `builders_test.go`:

```go
func TestBuildAzure_EndpointParsing(t *testing.T) {
	ctx := context.Background()

	t.Run("resource_and_deployment_from_endpoint", func(t *testing.T) {
		cfg := &config.Config{
			AzureOpenAiKey:      "test-key",
			AzureOpenAiEndpoint: "https://myresource.openai.azure.com/openai/deployments/mydeployment",
		}
		// This will fail at LanguageModel because we have no real provider,
		// but it should NOT fail at the parsing stage.
		builder := providerRegistry["azure"]
		_, err := builder(ctx, cfg, map[string]string{}, "gpt-4o")
		// Error from LanguageModel (no real Azure) is acceptable;
		// we just verify it got past the config check.
		require.Error(t, err)
		assert.NotContains(t, err.Error(), "apiKey, resourceName, and deployment are required")
	})

	t.Run("explicit_fields_override_endpoint", func(t *testing.T) {
		cfg := &config.Config{
			AzureOpenAiKey:          "test-key",
			AzureOpenAiResourceName: "explicit-resource",
			AzureOpenAiDeployment:   "explicit-deployment",
		}
		builder := providerRegistry["azure"]
		_, err := builder(ctx, cfg, map[string]string{}, "gpt-4o")
		require.Error(t, err)
		assert.NotContains(t, err.Error(), "apiKey, resourceName, and deployment are required")
	})

	t.Run("missing_all_config", func(t *testing.T) {
		cfg := &config.Config{}
		builder := providerRegistry["azure"]
		_, err := builder(ctx, cfg, map[string]string{}, "gpt-4o")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "azure: apiKey, resourceName, and deployment are required")
	})
}
```

- [ ] **Step 2: Write Bedrock missing fields test**

Append to `builders_test.go`:

```go
func TestBuildBedrock_MissingFields(t *testing.T) {
	ctx := context.Background()
	cfg := &config.Config{}

	t.Run("missing_all", func(t *testing.T) {
		builder := providerRegistry["bedrock"]
		_, err := builder(ctx, cfg, map[string]string{}, "claude-3")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "bedrock: accessKeyID, secretKey, and region are required")
	})

	t.Run("missing_region", func(t *testing.T) {
		cfg2 := &config.Config{
			AWSBedrockAccessKeyID: "AKIAIOSFODNN7EXAMPLE",
			AWSBedrockSecretKey:   "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
		}
		builder := providerRegistry["bedrock"]
		_, err := builder(ctx, cfg2, map[string]string{}, "claude-3")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "bedrock: accessKeyID, secretKey, and region are required")
	})
}
```

- [ ] **Step 3: Write unsupported provider test**

Append to `builders_test.go`:

```go
func TestNewLLMProvider_UnsupportedProvider(t *testing.T) {
	cfg := &config.Config{LLMProvider: "nonexistent"}
	prov := NewLLMProvider(cfg, map[string]string{})
	require.NotNil(t, prov)
	_, err := prov.Stream(context.Background(), nil, "", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported LLM provider")
}
```

- [ ] **Step 4: Run all new tests**

```bash
cd backend && go test -tags "fts5 nolancedb" ./internal/providers/ -run "TestBuildAzure|TestBuildBedrock|TestNewLLMProvider_Unsupported" -v
```

Expected: PASS (5 subtests)

- [ ] **Step 5: Commit**

```bash
git add backend/internal/providers/builders_test.go
git commit -m "test(providers): add Azure endpoint parsing and Bedrock config validation tests"
```

---

## Task 12: 回归验证与清理

**Files:**
- Modify: `backend/internal/providers/*.go` (gofmt)
- Modify: `backend/internal/config/config.go` (gofmt)

- [ ] **Step 1: Run gofmt**

```bash
cd backend && gofmt -w internal/providers/*.go internal/config/config.go
```

- [ ] **Step 2: Run go vet**

```bash
cd backend && go vet -tags "fts5 nolancedb" ./internal/providers/...
```

Expected: no output (clean)

- [ ] **Step 3: Run full provider package tests**

```bash
cd backend && go test -tags "fts5 nolancedb" ./internal/providers/ -v -count=1
```

Expected: All tests PASS

- [ ] **Step 4: Run full agent package tests (downstream consumer)**

```bash
cd backend && go test -tags "fts5 nolancedb" ./internal/agent/... -count=1 -timeout=60s
```

Expected: PASS (agent package uses `NewLLMProvider`, verify no breakage)

- [ ] **Step 5: Build the whole server**

```bash
cd backend && go build -tags "fts5 nolancedb" ./cmd/server/...
```

Expected: no errors

- [ ] **Step 6: Remove any leftover TODO comments**

Search for:
```bash
grep -rn "TODO\|FIXME\|XXX" backend/internal/providers/
```

Clean up any that are no longer relevant.

- [ ] **Step 7: Final commit**

```bash
git add backend/
git commit -m "style(providers): gofmt + go vet cleanup after LLM provider switch expansion"
```

---

## Self-Review Checklist

**1. Spec coverage:**

| Spec 要求 | 对应 Task |
|---|---|
| 36 个 provider 映射 | Task 4-9（全部覆盖） |
| Config 扩展（~110 字段） | Task 3 |
| Registry 模式替代 switch | Task 2 |
| resolve.go fallback 链 | Task 1 |
| Azure endpoint 解析 | Task 4 + Task 11 |
| Bedrock sessionToken | Task 9 + Task 11 |
| 纯单元测试 | Task 1, 10, 11 |
| noopLLM fallback 保持 | Task 2 |
| gofmt / go vet | Task 12 |

**2. Placeholder scan:** 无 TBD/TODO/"implement later"/"similar to"。每个 builder 的完整代码都在任务中。

**3. Type consistency:**
- `providerBuilder` 签名在 Task 2 定义：`func(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error)`
- 所有 36 个 builder 遵循同一签名
- `firstNonEmpty`、`resolveProviderName`、`resolveModelID` 在 Task 1 定义，后续任务一致使用
- Config 字段名在 Task 3 定义，resolve.go 中的 `cfgModelPref` 和 builders.go 中的访问一致

**4. 缺失检查：**
- `llm_test.go`（PantheonLLM Stream/Complete mock 测试）已在现有文件中存在，计划未要求新增（若现有测试不足，可在 Task 12 中补充）。
- `config.go` 的 env tag 需要和 Node `updateENV.js` 严格对齐，已在 Task 3 中列出。

---

**Plan complete and saved to `.gpowers/plans/2026-05-27-llm-provider-switch-full.md`.**

**Two execution options:**

**1. Subagent-Driven (recommended)** — I dispatch a fresh subagent per task, review between tasks, fast iteration

**2. Inline Execution** — Execute tasks in this session using executing-plans, batch execution with checkpoints for review

**Which approach?**
