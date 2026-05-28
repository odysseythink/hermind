# LLM Provider Switch 全量补齐设计文档

> 目标：将 backend 的 LLM provider 支持从当前的 2 个（ollama + openai）扩展到 Node 已有的 36 个，通过 Pantheon v0.0.9 的 provider 包实现。

---

## 1. 背景与现状

### 1.1 当前状态

- **Pantheon v0.0.9** 提供 43 个 LLM provider（`providers/` 子包）
- **Node server** 支持 36 个 LLM provider（`supportedLLM` 校验 + `updateENV.js` 独立配置键）
- **backend** 当前仅接入 2 个：`ollama` 和 `openai`（default）
- backend `Config` 结构只有 `LLMProvider`、`LLMModel`、`LLMApiKey`、`OpenAiKey` 四个 LLM 相关字段

### 1.2 核心问题

1. Provider switch 分支只有 2 个，远未覆盖 Node 的 36 个
2. `Config` 缺少 provider-specific 的 API key、base URL、model preference 字段
3. `NewLLMProvider` 是一个 100+ 行的函数，扩展为 36 个分支后维护成本极高
4. 名称映射不一致（Node `gemini` → Pantheon `google`，Node `generic-openai` → Pantheon `genericopenai` 等）

---

## 2. 设计决策

| 决策点 | 选择 | 理由 |
|---|---|---|
| 配置项扩展 | 逐个 provider 补齐独立配置字段 | 和 Node `updateENV.js` 的 `KEY_MAPPING` 一一对应，迁移理解成本最低 |
| 范围优先级 | 先补齐 Node 已有的 36 个 | 保持 100% 向后兼容；Pantheon 独有的 7 个（minimax/qwen/zhipu/wenxin 等）放到二期 |
| 测试策略 | 纯单元测试 | 不依赖真实 API key / 网络；mock interface 测路由 + 配置解析 |
| 架构模式 | Registry + Builder 模式 | 替代 giant switch，每个 provider 独立、可单独测试 |

---

## 3. Provider 名称映射

### 3.1 完整映射表（36 个）

| # | Node 名称 | Pantheon 包 | 构造函数 | 名称差异 |
|---|---|---|---|---|
| 1 | `openai` | `openai` | `New(apiKey, opts...)` | — |
| 2 | `azure` | `azure` | `New(apiKey, resourceName, deployment, opts...)` | **特殊** |
| 3 | `anthropic` | `anthropic` | `New(apiKey, opts...)` | — |
| 4 | `gemini` | `google` | `New(apiKey, opts...)` | Node 叫 gemini |
| 5 | `lmstudio` | `lmstudio` | `New(apiKey, opts...)` + `WithBaseURL` | — |
| 6 | `localai` | `localai` | `New(apiKey, opts...)` + `WithBaseURL` | — |
| 7 | `ollama` | `ollama` | `New(apiKey, opts...)` + `WithBaseURL` | 已存在 |
| 8 | `togetherai` | `together` | `New(apiKey, opts...)` + `WithBaseURL` | Node 叫 togetherai |
| 9 | `fireworksai` | `fireworks` | `New(apiKey, opts...)` + `WithBaseURL` | Node 叫 fireworksai |
| 10 | `mistral` | `mistral` | `New(apiKey, opts...)` + `WithBaseURL` | — |
| 11 | `huggingface` | `huggingface` | `New(apiKey, opts...)` + `WithBaseURL` | — |
| 12 | `perplexity` | `perplexity` | `New(apiKey, opts...)` + `WithBaseURL` | — |
| 13 | `openrouter` | `openrouter` | `New(apiKey, opts...)` + `WithBaseURL` | — |
| 14 | `novita` | `novita` | `New(apiKey, opts...)` + `WithBaseURL` | — |
| 15 | `groq` | `groq` | `New(apiKey, opts...)` + `WithBaseURL` | — |
| 16 | `koboldcpp` | `koboldcpp` | `New(apiKey, opts...)` + `WithBaseURL` | — |
| 17 | `textgenwebui` | `textgenwebui` | `New(apiKey, opts...)` + `WithBaseURL` | — |
| 18 | `cohere` | `cohere` | `New(apiKey, opts...)` + `WithBaseURL` | — |
| 19 | `litellm` | `litellm` | `New(apiKey, opts...)` + `WithBaseURL` | — |
| 20 | `generic-openai` | `genericopenai` | `New(apiKey, opts...)` + `WithBaseURL` | Node 带连字符 |
| 21 | `bedrock` | `bedrock` | `New(accessKeyID, secretKey, region, opts...)` | **特殊** |
| 22 | `deepseek` | `deepseek` | `New(apiKey, opts...)` + `WithBaseURL` | — |
| 23 | `apipie` | `apipie` | `New(apiKey, opts...)` + `WithBaseURL` | — |
| 24 | `xai` | `xai` | `New(apiKey, opts...)` + `WithBaseURL` | — |
| 25 | `nvidia-nim` | `nvidianim` | `New(apiKey, opts...)` + `WithBaseURL` | Node 带连字符 |
| 26 | `ppio` | `ppio` | `New(apiKey, opts...)` + `WithBaseURL` | — |
| 27 | `dpaiStudio` | `dellproaistudio` | `New(apiKey, opts...)` + `WithBaseURL` | Node 缩写 |
| 28 | `moonshotai` | `kimi` | `New(apiKey, opts...)` + `WithBaseURL` | Node 叫 moonshotai，Pantheon 叫 kimi（月之暗面） |
| 29 | `cometapi` | `cometapi` | `New(apiKey, opts...)` + `WithBaseURL` | — |
| 30 | `foundry` | `foundry` | `New(apiKey, opts...)` + `WithBaseURL` | — |
| 31 | `zai` | `zai` | `New(apiKey, opts...)` + `WithBaseURL` | — |
| 32 | `giteeai` | `giteeai` | `New(apiKey, opts...)` + `WithBaseURL` | — |
| 33 | `docker-model-runner` | `dockermodelrunner` | `New(apiKey, opts...)` + `WithBaseURL` | Node 带连字符 |
| 34 | `privatemode` | `privatemode` | `New(apiKey, opts...)` + `WithBaseURL` | — |
| 35 | `sambanova` | `sambanova` | `New(apiKey, opts...)` + `WithBaseURL` | — |
| 36 | `lemonade` | `lemonade` | `New(apiKey, opts...)` + `WithBaseURL` | — |

### 3.2 构造函数分类

| 模式 | 数量 | Provider |
|---|---|---|
| `New(apiKey, opts...)` + 可选 `WithBaseURL` | 33 | 除 Azure、Bedrock、Ollama 外的所有 |
| `New(apiKey, resourceName, deployment, opts...)` | 1 | Azure |
| `New(accessKeyID, secretKey, region, opts...)` | 1 | Bedrock |
| `New(apiKey, opts...)` + `WithBaseURL`（baseURL 必填） | 1 | Ollama |

---

## 4. Config 扩展

### 4.1 新增字段清单（按 provider 分组）

`config.go` 中 `Config` 结构体新增以下字段，每个字段的 env tag 与 Node `updateENV.js` 的 `envKey` 对齐：

```go
// === OpenAI ===
OpenAiKey       string `env:"OPEN_AI_KEY"`
OpenAiModelPref string `env:"OPEN_MODEL_PREF"`

// === Azure ===
AzureOpenAiEndpoint         string `env:"AZURE_OPENAI_ENDPOINT"`
AzureOpenAiKey              string `env:"AZURE_OPENAI_KEY"`
AzureOpenAiModelPref        string `env:"AZURE_OPENAI_MODEL_PREF"`
AzureOpenAiModelType        string `env:"AZURE_OPENAI_MODEL_TYPE" envDefault:"default"` // "default" | "reasoning"
AzureOpenAiTokenLimit       int    `env:"AZURE_OPENAI_TOKEN_LIMIT"`
AzureOpenAiResourceName     string `env:"AZURE_OPENAI_RESOURCE_NAME"` // Pantheon 需要
AzureOpenAiDeployment       string `env:"AZURE_OPENAI_DEPLOYMENT"`    // Pantheon 需要

// === Anthropic ===
AnthropicApiKey       string `env:"ANTHROPIC_API_KEY"`
AnthropicModelPref    string `env:"ANTHROPIC_MODEL_PREF"`
AnthropicCacheControl string `env:"ANTHROPIC_CACHE_CONTROL" envDefault:"none"` // "none" | "5m" | "1h"

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
OllamaBasePath       string `env:"OLLAMA_BASE_PATH"`
OllamaModelPref      string `env:"OLLAMA_MODEL_PREF"`
OllamaTokenLimit     int    `env:"OLLAMA_MODEL_TOKEN_LIMIT"`
OllamaKeepAliveSec   int    `env:"OLLAMA_KEEP_ALIVE_TIMEOUT"`
OllamaAuthToken      string `env:"OLLAMA_AUTH_TOKEN"`

// === TogetherAI ===
TogetherAiApiKey  string `env:"TOGETHER_AI_API_KEY"`
TogetherAiModelPref string `env:"TOGETHER_AI_MODEL_PREF"`

// === FireworksAI ===
FireworksApiKey   string `env:"FIREWORKS_API_KEY"`
FireworksModelPref string `env:"FIREWORKS_MODEL_PREF"`

// === Mistral ===
MistralApiKey    string `env:"MISTRAL_API_KEY"`
MistralModelPref string `env:"MISTRAL_MODEL_PREF"`

// === HuggingFace ===
HuggingFaceEndpoint  string `env:"HUGGING_FACE_LLM_ENDPOINT"`
HuggingFaceApiKey    string `env:"HUGGING_FACE_LLM_API_KEY"`
HuggingFaceTokenLimit int   `env:"HUGGING_FACE_LLM_TOKEN_LIMIT"`

// === Perplexity ===
PerplexityApiKey   string `env:"PERPLEXITY_API_KEY"`
PerplexityModelPref string `env:"PERPLEXITY_MODEL_PREF"`

// === OpenRouter ===
OpenRouterApiKey   string `env:"OPENROUTER_API_KEY"`
OpenRouterModelPref string `env:"OPENROUTER_MODEL_PREF"`

// === Novita ===
NovitaApiKey   string `env:"NOVITA_API_KEY"`
NovitaModelPref string `env:"NOVITA_MODEL_PREF"`

// === Groq ===
GroqApiKey   string `env:"GROQ_API_KEY"`
GroqModelPref string `env:"GROQ_MODEL_PREF"`

// === KoboldCPP ===
KoboldBasePath  string `env:"KOBOLD_CPP_BASE_PATH"`
KoboldModelPref string `env:"KOBOLD_CPP_MODEL_PREF"`
KoboldTokenLimit int   `env:"KOBOLD_CPP_MODEL_TOKEN_LIMIT"`
KoboldMaxTokens  int   `env:"KOBOLD_CPP_MAX_TOKENS"`

// === TextGenWebUI ===
TextGenBasePath  string `env:"TEXT_GEN_WEB_UI_BASE_PATH"`
TextGenTokenLimit int   `env:"TEXT_GEN_WEB_UI_MODEL_TOKEN_LIMIT"`
TextGenApiKey     string `env:"TEXT_GEN_WEB_UI_API_KEY"`

// === Cohere ===
CohereApiKey   string `env:"COHERE_API_KEY"`
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
AWSBedrockAccessKeyID string `env:"AWS_BEDROCK_LLM_ACCESS_KEY_ID"`
AWSBedrockSecretKey   string `env:"AWS_BEDROCK_LLM_ACCESS_KEY"`
AWSBedrockRegion      string `env:"AWS_BEDROCK_LLM_REGION"`
AWSBedrockSessionToken string `env:"AWS_BEDROCK_LLM_SESSION_TOKEN"`
AWSBedrockModelPref   string `env:"AWS_BEDROCK_LLM_MODEL_PREFERENCE"`

// === DeepSeek ===
DeepSeekApiKey   string `env:"DEEPSEEK_API_KEY"`
DeepSeekModelPref string `env:"DEEPSEEK_MODEL_PREF"`

// === ApiPie ===
ApiPieApiKey   string `env:"APIPIE_API_KEY"`
ApiPieModelPref string `env:"APIPIE_MODEL_PREF"`

// === XAI ===
XaiApiKey   string `env:"XAI_API_KEY"`
XaiModelPref string `env:"XAI_MODEL_PREF"`

// === NVIDIA NIM ===
NvidiaNimApiKey   string `env:"NVIDIA_NIM_API_KEY"`
NvidiaNimModelPref string `env:"NVIDIA_NIM_MODEL_PREF"`

// === PPIO ===
PpioApiKey   string `env:"PPIO_API_KEY"`
PpioModelPref string `env:"PPIO_MODEL_PREF"`

// === Dell Pro AI Studio ===
DellProBasePath  string `env:"DELL_PRO_AI_STUDIO_BASE_PATH"`
DellProModelPref string `env:"DELL_PRO_AI_STUDIO_MODEL_PREF"`
DellProTokenLimit int   `env:"DELL_PRO_AI_STUDIO_MODEL_TOKEN_LIMIT"`

// === MoonshotAI (Kimi) ===
MoonshotApiKey   string `env:"MOONSHOT_API_KEY"`
MoonshotModelPref string `env:"MOONSHOT_MODEL_PREF"`

// === CometAPI ===
CometApiKey   string `env:"COMET_API_KEY"`
CometModelPref string `env:"COMET_API_MODEL_PREF"`

// === Foundry ===
FoundryApiKey   string `env:"FOUNDRY_API_KEY"`
FoundryModelPref string `env:"FOUNDRY_MODEL_PREF"`

// === ZAI ===
ZaiApiKey   string `env:"ZAI_API_KEY"`
ZaiModelPref string `env:"ZAI_MODEL_PREF"`

// === GiteeAI ===
GiteeApiKey   string `env:"GITEE_API_KEY"`
GiteeModelPref string `env:"GITEE_MODEL_PREF"`

// === Docker Model Runner ===
DockerModelBasePath  string `env:"DOCKER_MODEL_RUNNER_BASE_PATH"`
DockerModelModelPref string `env:"DOCKER_MODEL_RUNNER_MODEL_PREF"`

// === PrivateMode ===
PrivateModeApiKey   string `env:"PRIVATE_MODE_API_KEY"`
PrivateModeModelPref string `env:"PRIVATE_MODE_MODEL_PREF"`

// === SambaNova ===
SambaNovaApiKey   string `env:"SAMBANOVA_API_KEY"`
SambaNovaModelPref string `env:"SAMBANOVA_MODEL_PREF"`

// === Lemonade ===
LemonadeApiKey   string `env:"LEMONADE_API_KEY"`
LemonadeModelPref string `env:"LEMONADE_MODEL_PREF"`
```

### 4.2 Fallback 链设计

配置解析优先级（从高到低）：

1. `settings` map（DB `system_settings`，per-workspace / per-user 配置）
2. `Config` 字段（环境变量）
3. 通用 fallback（`LLMApiKey`、`LLMModel`）
4. Provider 默认值（如 `gpt-4o-mini`）

```go
// API Key fallback 示例（Anthropic）
apiKey := firstNonEmpty(
    cfg.AnthropicApiKey,           // 1. env
    settings["AnthropicApiKey"],   // 2. DB settings（优先于 env，和 Node 行为一致）
    cfg.LLMApiKey,                 // 3. 通用 fallback
)

// Model ID fallback 示例
modelID := firstNonEmpty(
    settings["AnthropicModelPref"], // 1. DB settings
    cfg.AnthropicModelPref,         // 2. env
    cfg.LLMModel,                   // 3. 通用 fallback
    "claude-3-5-sonnet-20241022",   // 4. 默认值
)
```

**注意**：`settings` map 优先于 `Config`，和 Node 的行为一致（Node 的 `process.env` 被 `workspace.setMetadata` 覆盖）。

---

## 5. 架构重构

### 5.1 文件拆分

```
backend/internal/providers/
├── llm.go           # 接口定义 + PantheonLLM 包装 + noopLLM + NewLLMProvider 入口
├── builders.go      # 36 个 providerBuilder 函数 + providerRegistry map
├── resolve.go       # 配置解析辅助函数
├── llm_test.go      # PantheonLLM Stream/Complete 测试
├── builders_test.go # Builder 注册表 + 各 builder 单元测试
└── resolve_test.go  # 配置解析测试
```

### 5.2 核心类型与入口

```go
// llm.go

type providerBuilder func(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error)

var providerRegistry = map[string]providerBuilder{
    "openai":        buildOpenAI,
    "azure":         buildAzure,
    "anthropic":     buildAnthropic,
    "gemini":        buildGoogle,
    "lmstudio":      buildLMStudio,
    "localai":       buildLocalAI,
    "ollama":        buildOllama,
    "togetherai":    buildTogether,
    "fireworksai":   buildFireworks,
    "mistral":       buildMistral,
    "huggingface":   buildHuggingFace,
    "perplexity":    buildPerplexity,
    "openrouter":    buildOpenRouter,
    "novita":        buildNovita,
    "groq":          buildGroq,
    "koboldcpp":     buildKoboldCPP,
    "textgenwebui":  buildTextGenWebUI,
    "cohere":        buildCohere,
    "litellm":       buildLiteLLM,
    "generic-openai": buildGenericOpenAI,
    "bedrock":       buildBedrock,
    "deepseek":      buildDeepSeek,
    "apipie":        buildApiPie,
    "xai":           buildXAI,
    "nvidia-nim":    buildNvidiaNIM,
    "ppio":          buildPPIO,
    "dpaiStudio":    buildDellPro,
    "moonshotai":    buildMoonshot,
    "cometapi":      buildCometAPI,
    "foundry":       buildFoundry,
    "zai":           buildZAI,
    "giteeai":       buildGiteeAI,
    "docker-model-runner": buildDockerModelRunner,
    "privatemode":   buildPrivateMode,
    "sambanova":     buildSambaNova,
    "lemonade":      buildLemonade,
}

func NewLLMProvider(cfg *config.Config, settings map[string]string) LLMProvider {
    providerName := resolveProviderName(cfg, settings)
    modelID := resolveModelID(providerName, cfg, settings)

    builder, ok := providerRegistry[providerName]
    if !ok {
        return &noopLLM{err: fmt.Errorf("unsupported LLM provider: %s", providerName)}
    }

    model, err := builder(context.Background(), cfg, settings, modelID)
    if err != nil {
        mlog.Error("NewLLMProvider: builder failed for ", providerName, ": ", err)
        return &noopLLM{err: fmt.Errorf("create %s provider: %w", providerName, err)}
    }

    return &PantheonLLM{model: model, cfg: cfg}
}
```

### 5.3 Standard Builder 模板（33 个）

```go
// builders.go

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

// ... 其余 31 个 standard builder 类似
```

### 5.4 resolve.go 辅助函数

```go
// resolve.go

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
    if v := fieldByProvider(cfg, provider, "ModelPref"); v != "" {
        return v
    }
    if cfg.LLMModel != "" {
        return cfg.LLMModel
    }
    return defaultModelForProvider(provider)
}

func firstNonEmpty(values ...string) string {
    for _, v := range values {
        if v != "" {
            return v
        }
    }
    return ""
}
```

**设计决策**：`modelPrefKeyForProvider` 和 `fieldByProvider` 使用字符串映射而非反射，避免运行时开销和 `go vet` 的 `sync.Once` 复制警告。

---

## 6. 特殊 Provider 处理

### 6.1 Azure

Pantheon 的 `azure.New` 需要 `resourceName` + `deployment`，Node 只用 `AZURE_OPENAI_ENDPOINT`。新增两个 env 字段，同时支持从 endpoint URL 自动解析。

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
    
    // Fallback: parse from AZURE_OPENAI_ENDPOINT
    if resourceName == "" || deployment == "" {
        endpoint := firstNonEmpty(
            settings["AzureOpenAiEndpoint"],
            cfg.AzureOpenAiEndpoint,
        )
        if endpoint != "" {
            rn, dep, err := parseAzureEndpoint(endpoint)
            if err == nil {
                if resourceName == "" { resourceName = rn }
                if deployment == "" { deployment = dep }
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
    if apiVersion := cfg.AzureOpenAiModelType; apiVersion == "reasoning" {
        // reasoning models may need different API version; handled by caller
    }
    
    p, err := pantheonAzure.New(apiKey, resourceName, deployment, opts...)
    if err != nil {
        return nil, err
    }
    return p.LanguageModel(ctx, modelID)
}
```

`parseAzureEndpoint` 从 `https://myresource.openai.azure.com/openai/deployments/mydep` 提取 resourceName（hostname 前缀）和 deployment（path 段）。

### 6.2 Bedrock

Pantheon 的 `bedrock.New(accessKeyID, secretKey, region, opts...)`，Node 支持多种认证方式。backend 首期只支持 accessKey+secretKey（默认方式），sessionToken 通过 `WithSessionToken` 可选。

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

---

## 7. 错误处理

### 7.1 策略

| 场景 | 行为 |
|---|---|
| 配置缺失（API key / base URL / required field） | Builder 返回 error → `NewLLMProvider` 返回 `noopLLM` |
| Provider 初始化失败（如网络、无效 key） | Builder 返回 error → `noopLLM` |
| Stream 过程中出错 | 通过 `LLMChunk.Err` 传播到调用方（已有行为，不改动） |
| 不支持的 provider 名称 | `noopLLM` with `fmt.Errorf("unsupported LLM provider: %s", name)` |
| Context 取消 | Stream goroutine 检测到 `ctx.Done()` 后退出（已有行为） |

### 7.2 noopLLM 保持现有行为

```go
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

---

## 8. 测试方案

### 8.1 策略

纯单元测试，不依赖真实 API key / 网络。测试分三层：

1. **Resolve 层**：验证配置解析的 fallback 链
2. **Registry 层**：验证 36 个 provider 都在 registry 中，无重复
3. **Builder 层**：验证缺失配置时返回 error（触网路径 skip）
4. **Interface 层**：mock `core.LanguageModel` 测试 `PantheonLLM.Stream/Complete`

### 8.2 测试清单

| 测试文件 | 测试内容 | 数量估算 |
|---|---|---|
| `resolve_test.go` | `resolveProviderName`（settings 优先、env fallback、默认值） | ~5 |
| `resolve_test.go` | `resolveModelID`（provider-specific → generic → default） | ~10 |
| `resolve_test.go` | `firstNonEmpty`、`parseAzureEndpoint` | ~8 |
| `builders_test.go` | Registry 完整性：36 个 key 都在 map 中，builder 非 nil | 2 |
| `builders_test.go` | 每个 provider 缺失 API key → error（用 subtest 循环） | ~36 |
| `builders_test.go` | Azure：endpoint 解析成功 / 失败 / 字段覆盖 | ~4 |
| `builders_test.go` | Bedrock：缺失字段 / sessionToken 选项 | ~3 |
| `llm_test.go` | `PantheonLLM.Stream`（mock `core.LanguageModel`） | ~5 |
| `llm_test.go` | `PantheonLLM.Complete`（mock `core.LanguageModel`） | ~3 |
| `llm_test.go` | `NewLLMProvider`：unsupported provider → noopLLM | ~2 |

**总计：~78 个 test case。**

### 8.3 Mock 策略

```go
// mockLanguageModel 实现 core.LanguageModel
type mockLanguageModel struct {
    streamFunc func(ctx context.Context, req *core.Request) (iter.Seq2[*core.ResponseChunk, error], error)
}

func (m *mockLanguageModel) Stream(ctx context.Context, req *core.Request) (iter.Seq2[*core.ResponseChunk, error], error) {
    return m.streamFunc(ctx, req)
}

func (m *mockLanguageModel) Complete(ctx context.Context, req *core.Request) (*core.Response, error) {
    // 从 Stream 聚合
}
```

Pantheon 的 `core.LanguageModel` 是 interface，可直接 mock。`core.Provider` 不是测试重点（Pantheon 内部实现），builder 层测试只验证 error 路径（配置缺失时返回 error，不触网）。

### 8.4 Builder 层测试模式

```go
func TestBuildAnthropic_MissingKey(t *testing.T) {
    cfg := &config.Config{}
    settings := map[string]string{}
    _, err := buildAnthropic(context.Background(), cfg, settings, "claude-3")
    require.Error(t, err)
    assert.Contains(t, err.Error(), "no Anthropic API key")
}

func TestProviderRegistry_Complete(t *testing.T) {
    expected := []string{
        "openai", "azure", "anthropic", "gemini", /* ... 36 个 ... */,
    }
    for _, name := range expected {
        t.Run(name, func(t *testing.T) {
            b, ok := providerRegistry[name]
            require.True(t, ok, "provider %s not in registry", name)
            require.NotNil(t, b)
        })
    }
    assert.Equal(t, len(expected), len(providerRegistry), "registry size mismatch")
}
```

---

## 9. 实施步骤

### Phase 1: 基础设施（~1 天）

1. 创建 `resolve.go` + `resolve_test.go`（配置解析辅助函数）
2. 重构 `llm.go`：提取 `NewLLMProvider` 入口 + `providerRegistry`
3. 验证现有测试（ollama + openai）仍通过

### Phase 2: Config 扩展（~1 天）

1. 在 `config.go` 中添加 36 个 provider 的配置字段（~110 个字段）
2. 更新 `.env.example` 文档

### Phase 3: Standard Builders（~2 天）

1. 实现 33 个 standard provider builder（`New(apiKey, opts...)` 模式）
2. 每个 builder 配一个 "missing key" 测试
3. 实现 `providerRegistry` 完整性测试

### Phase 4: Special Builders（~1 天）

1. 实现 Azure builder（endpoint 解析 + resourceName/deployment）
2. 实现 Bedrock builder（accessKey/secretKey/region + sessionToken）
3. 特殊 builder 的边界测试

### Phase 5: 回归与清理（~0.5 天）

1. `go vet` / `gofmt`
2. 全量测试运行
3. 删除旧的 TODO / 注释清理

**总估算：~5.5 天（约 1 个工作周）。**

---

## 10. 风险与缓解

| 风险 | 可能性 | 影响 | 缓解 |
|---|---|---|---|
| Pantheon provider API 签名变化 | 低 | 中 | Builder 模式隔离了变化范围，只需改对应 builder |
| Config 字段膨胀导致 struct 过大 | 低 | 低 | Go struct 字段数无硬限制；如未来超过 100 个可考虑分组嵌套 |
| Azure endpoint 解析鲁棒性 | 中 | 中 | 新增独立字段 `AZURE_OPENAI_RESOURCE_NAME`/`DEPLOYMENT` 作为 primary，解析作为 fallback |
| Node provider 名称未来新增 | 中 | 低 | Registry 模式新增一行即可，已有测试框架可复用 |
| settings map key 命名不一致 | 中 | 中 | 文档中明确列出每个 provider 的 settings key，测试覆盖验证 |

---

## 11. 未解决问题（二期）

1. **Pantheon 独有的 7 个 provider**：`minimax`、`qwen`、`zhipu`、`wenxin`、`native`、`voyage`、`openaicompat`。`native` 是本地模型（非 API），`voyage` 是嵌入模型，其余 5 个是真正的 LLM API。如需支持，需要新增 UI 选项 + 前端翻译 + config 字段。
2. **Embed provider switch**：当前 `embedder/pantheon.go` 只接了 `openai-embed`，需类似的补齐计划。
3. **Reranker**：`extensions/rerank` 未导入，内部 `Rerank bool` 只是 fetch-more-then-truncate。
4. **Temperature / MaxTokens 透传**：当前 `PantheonLLM.Stream` 已支持，但部分 provider 可能不支持某些参数（如 Bedrock 的某些模型不支持 temperature）。Pantheon 内部应已处理，backend 无需额外逻辑。
