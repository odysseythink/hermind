# v3-A — Provider Breadth Design (LLM × 42 + Embedder × 4 + Rerank × 1)

**Date**: 2026-05-27
**Status**: Draft
**Author**: brainstorming session
**Scope**: 把 `backend/internal/providers/llm.go` 从只支持 ollama+openai 拓宽到 pantheon v0.0.9 的全部 42 个 LLM provider;把 `embedder/pantheon.go` 从只支持 openai 拓宽到 pantheon 提供的 4 个 embedding provider (cohere/native/openaicompat/voyage);新增 `internal/reranker/` 包导入 pantheon `extensions/rerank`,把 `vectordb` 现有的 "fetch-more-then-truncate" 假 rerank 换成真模型 rerank。**整体一个 PR-V3A 交付,~25-35h**。

**先决条件**: 无 (这是纯接线工作,不依赖 v1/v2)。可以与 v1/v2 主链路并行推进。

---

## 1. 现状盘点

### 1.1 pantheon v0.0.9 上游能力(已验证)

| 子系统 | 实际数量 | 备注 |
|---|---|---|
| LLM provider | 42 个 (`providers/*/`,扣掉 `openaicompat` 是底层 client) | 全部实现 `core.LanguageModel`,统一 `New(apiKey, ...Option) (core.Provider, error)` 入口 |
| Embedding provider | 4 个 (`cohere`/`native`/`openaicompat`/`voyage`) | 实现 `embed.Provider`;`openaicompat` 是 openai-style API 通配适配器 |
| Rerank provider | 1 个 (`extensions/rerank/` 抽象,`cohere` 同时实现 LM + rerank) | `rerank.Provider` interface;LLM provider 中实现该接口的就可用 |

### 1.2 Node 端 inventory(对比基线)

| 类别 | Node 名字数 | 列表 |
|---|---|---|
| AiProviders | 37 | anthropic, apipie, azureOpenAi, bedrock, cohere, cometapi, deepseek, dellProAiStudio, dockerModelRunner, fireworksAi, foundry, gemini, genericOpenAi, giteeai, groq, huggingface, koboldCPP, lemonade, liteLLM, lmStudio, localAi, mistral, modelMap, moonshotAi, novita, nvidiaNim, ollama, openAi, openRouter, perplexity, ppio, privatemode, sambanova, textGenWebUI, togetherAi, xai, zai |
| EmbeddingEngines | 14 | azureOpenAi, cohere, gemini, genericOpenAi, lemonade, liteLLM, lmstudio, localAi, mistral, native, ollama, openAi, openRouter, voyageAi |
| EmbeddingRerankers | 1 | native |

### 1.3 Go 端当前状态

```go
// internal/providers/llm.go (5/27 实测)
switch providerName {
case "ollama": /* ollama.New + WithBaseURL */
default:      /* openai.New */
}
```

只有 2 个 case。剩余 35 个 Node 已知 + 5 个 pantheon 独有(`kimi`/`minimax`/`qwen`/`wenxin`/`zhipu`/`zai` 这些华系)未接入。

```go
// internal/embedder/pantheon.go (5/27 实测)
provider, _ := openai.New(apiKey)  // 写死 openai
```

```go
// internal/vectordb/{pgvector,lance,...}.go
if opts.Rerank {
    limit = limit * 3  // fetch-more-then-truncate,不调真 reranker
}
```

### 1.4 Node ↔ pantheon ↔ Go 名字映射差异

| Node 名 (kebab) | pantheon 名 (lower) | 备注 |
|---|---|---|
| `azureOpenAi` | `azure` | 重命名 |
| `dellProAiStudio` | `dellproaistudio` | 仅大小写 |
| `dockerModelRunner` | `dockermodelrunner` | 仅大小写 |
| `fireworksAi` | `fireworks` | 去后缀 |
| `gemini` | `google` | **完全不同名** |
| `genericOpenAi` | `genericopenai` | 仅大小写 |
| `koboldCPP` | `koboldcpp` | 仅大小写 |
| `liteLLM` | `litellm` | 仅大小写 |
| `lmStudio` | `lmstudio` | 仅大小写 |
| `localAi` | `localai` | 仅大小写 |
| `moonshotAi` | `kimi` | **完全不同名** (kimi 是 moonshot 的产品名) |
| `nvidiaNim` | `nvidianim` | 仅大小写 |
| `openAi` | `openai` | 仅大小写 |
| `openRouter` | `openrouter` | 仅大小写 |
| `textGenWebUI` | `textgenwebui` | 仅大小写 |
| `togetherAi` | `together` | 去后缀 |
| `voyageAi` | `voyage` | 去后缀 |
| `modelMap` | (无对应,Node 内部静态映射) | 不是 provider,删除 |
| (Node 无) | `minimax`/`qwen`/`wenxin`/`zhipu`/`zai` | pantheon 独有,5 个华系 |

→ **Go 需要 38 个 case (37 Node + 5 pantheon-only - 1 modelMap - 3 重名 = 38)**。

---

## 2. 目标与边界

### 2.1 目标

- `providers.LLMProvider` 接口下,所有 38 个 case 都能从一个 `LLM_PROVIDER` env 字符串 + 对应 API key + base URL 配出来
- 配置键命名沿用 **Node `process.env.XXX`** 约定,双进程部署时同一份 `.env` Node + Go 都能读
- `embedder.Embedder` 接口下,4 个 pantheon embedding provider 都能配置使用 (Node 端 14 个里有 10 个其实是 "openai-compatible",pantheon 用 `openaicompat` 一个覆盖,所以 Go 实际只有 4 个具体实现)
- 引入 `reranker.Reranker` 接口,把 vectordb 假 rerank 换掉;首发只接 cohere (pantheon 唯一实现 rerank.Provider 的);其它 provider 走 noop
- **不破坏现有 2 个 case 的行为**:测试覆盖 ollama + openai 现状用例必须保留通过
- 每个 provider 至少有 1 个 smoke test (mock 或 build-only 取决于是否需要外部凭据)
- 完整可测:provider switch 是表驱动 + sub-test,新增 provider 不需要写新的测试模板

### 2.2 非目标 (此 PR)

- 不引入新的 provider 选择 UI(前端走现有的 "Embedding Settings / LLM Settings" 页,字段名匹配 Node 即可)
- 不实现 provider 凭据加密落库 (沿用 `SystemSetting` 现状)
- 不写 modelMap (Node 自己用,Go 用 pantheon 的 `Model` 直接调)
- 不支持 "Custom OpenAI-compatible URL"(`genericopenai` provider 已经覆盖了这一需求)
- 不调整向量库的"假 rerank" 路径以外的 SearchOptions schema
- 不接入 pantheon `native` provider 的本地模型 (需要 GGUF 文件 / llama.cpp 集成,独立 PR)
- 不处理 streaming 协议差异(pantheon 已经统一为 `core.StreamResponse`)
- 不补 modelMap (Node 静态映射模型 → context length,Go 走 pantheon 的 `Model()` 字符串,context length 在 chat_service 用静态 fallback)

---

## 3. 架构

### 3.1 包布局

```
backend/internal/providers/
├── llm.go                       # MODIFY — 大表 switch + WithBaseURL 选项构建
├── llm_test.go                  # NEW — 表驱动测试 38 个 provider
├── factory.go                   # NEW — newProviderByName(name, cfg, settings) (core.Provider, error)
├── factory_test.go              # NEW
├── env_keys.go                  # NEW — Node-parity env key 映射
└── env_keys_test.go             # NEW

backend/internal/embedder/
├── pantheon.go                  # MODIFY — 4-case switch
├── pantheon_test.go             # NEW
└── factory.go                   # NEW — newEmbedderByName(name, cfg, settings)

backend/internal/reranker/
├── doc.go                       # NEW
├── reranker.go                  # NEW — type Reranker interface + Pantheon impl + Noop
├── reranker_test.go             # NEW
└── factory.go                   # NEW — NewReranker(cfg, settings) Reranker

backend/internal/vectordb/
├── interface.go                 # MODIFY — Rerank 字段从 bool 换成 *RerankRequest
├── pgvector.go                  # MODIFY — 调用 reranker.Reranker.Rerank
├── lance.go                     # MODIFY — 同上
├── chroma.go                    # MODIFY — 同上
└── (other adapters)             # MODIFY — 同上

backend/internal/services/
├── chat_service.go              # MODIFY — 把 reranker 注入到 vectorSearch 调用
└── vector_search_service.go     # MODIFY — 接收 reranker

backend/internal/config/
└── config.go                    # MODIFY — 加 ~30 个 Node-parity env key

backend/cmd/server/main.go     # MODIFY — 构造 reranker + 注入
```

### 3.2 接口签名

```go
// internal/providers/factory.go

// ProviderSpec 描述一个 provider 的构造参数。
type ProviderSpec struct {
    Name        string                       // canonical lower-case name (openai/anthropic/...)
    APIKey      string                       // 已解析的 key
    BaseURL     string                       // 可选,留空走 pantheon 默认
    ExtraOptions map[string]string           // 例如 azure 的 deployment id、bedrock 的 region
}

// newProviderByName 构造一个 pantheon core.Provider。
// 是 LLM / embedder / rerank 工厂的公共底座。
func newProviderByName(spec ProviderSpec) (core.Provider, error)

// internal/providers/llm.go (重写)
func NewLLMProvider(cfg *config.Config, settings map[string]string) LLMProvider {
    spec := buildProviderSpec(cfg, settings)
    prov, err := newProviderByName(spec)
    if err != nil { return &noopLLM{err: err} }
    modelID := resolveLLMModelID(spec.Name, cfg, settings)
    model, err := prov.LanguageModel(ctx, modelID)
    if err != nil { return &noopLLM{err: err} }
    return &PantheonLLM{model: model, cfg: cfg}
}

// internal/embedder/factory.go
func NewEmbedder(cfg *config.Config, settings map[string]string) (Embedder, error) {
    spec := buildEmbedderSpec(cfg, settings)
    prov, err := providers.NewProviderByName(spec)  // 共享底座
    if err != nil { return nil, err }
    embedProv, ok := prov.(embed.Provider)
    if !ok { return nil, fmt.Errorf("provider %q does not implement embed.Provider", spec.Name) }
    model, err := embedProv.EmbeddingModel(ctx, resolveEmbedModelID(spec.Name, cfg, settings))
    if err != nil { return nil, err }
    return &PantheonEmbedder{model: model, dimensions: deriveDimensions(spec.Name, model)}, nil
}

// internal/reranker/reranker.go
type Reranker interface {
    Rerank(ctx context.Context, query string, documents []string, topN int) ([]ScoredDocument, error)
}

type ScoredDocument struct {
    Index int
    Score float64
    Text  string
}

type PantheonReranker struct {
    model rerank.RerankModel
}

func NewReranker(cfg *config.Config, settings map[string]string) (Reranker, error) {
    name := pick("RerankProvider", settings, cfg.RerankProvider)
    if name == "" || name == "none" || name == "noop" {
        return &NoopReranker{}, nil
    }
    spec := buildRerankerSpec(name, cfg, settings)
    prov, err := providers.NewProviderByName(spec)
    if err != nil { return nil, err }
    rerankProv, ok := prov.(rerank.Provider)
    if !ok { return nil, fmt.Errorf("provider %q does not implement rerank.Provider", spec.Name) }
    model, err := rerankProv.RerankModel(ctx, resolveRerankModelID(spec.Name, cfg, settings))
    if err != nil { return nil, err }
    return &PantheonReranker{model: model}, nil
}

type NoopReranker struct{}
func (n *NoopReranker) Rerank(ctx context.Context, query string, docs []string, topN int) ([]ScoredDocument, error) {
    out := make([]ScoredDocument, 0, min(topN, len(docs)))
    for i, d := range docs {
        if i >= topN { break }
        out = append(out, ScoredDocument{Index: i, Score: 0, Text: d})
    }
    return out, nil
}
```

### 3.3 因素表 (38 个 LLM provider × 配置项)

> 这是 PR 实现的核心数据,**按行写代码**。

| Pantheon name | Node name | API key env | BaseURL env | Default base URL | 备注 |
|---|---|---|---|---|---|
| anthropic | anthropic | `ANTHROPIC_API_KEY` | `ANTHROPIC_BASE_URL`(可选) | api.anthropic.com | 标准 |
| apipie | apipie | `APIPIE_LLM_API_KEY` | - | - | apikey only |
| azure | azureOpenAi | `AZURE_OPENAI_KEY` | `AZURE_OPENAI_ENDPOINT` | (空) | 必须给 endpoint;还要 `deployment` 参数 |
| bedrock | bedrock | (AWS IAM) | - | - | 走 AWS SDK,key= access_key+secret_key+region |
| cohere | cohere | `COHERE_API_KEY` | - | api.cohere.ai | 同时 LLM+embed+rerank |
| cometapi | cometapi | `COMETAPI_LLM_API_KEY` | - | - | apikey only |
| deepseek | deepseek | `DEEPSEEK_API_KEY` | - | api.deepseek.com | |
| dellproaistudio | dellProAiStudio | `DELL_PRO_AI_STUDIO_API_KEY` | `DELL_PRO_AI_STUDIO_BASE_URL` | - | |
| dockermodelrunner | dockerModelRunner | (none) | `DOCKER_MODEL_RUNNER_BASE_URL` | localhost:12434 | 本地 |
| fireworks | fireworksAi | `FIREWORKS_AI_LLM_API_KEY` | - | api.fireworks.ai | |
| foundry | foundry | `FOUNDRY_API_KEY` | `FOUNDRY_BASE_URL` | - | Microsoft AI Foundry |
| genericopenai | genericOpenAi | `GENERIC_OPEN_AI_API_KEY` | `GENERIC_OPEN_AI_BASE_PATH` | - | 任意 openai-compat |
| giteeai | giteeai | `GITEE_AI_API_KEY` | - | api.gitee.ai | |
| google | gemini | `GEMINI_API_KEY` | `GEMINI_API_URL`(可选) | generativelanguage.googleapis.com | **改 Node 命名映射** |
| groq | groq | `GROQ_API_KEY` | - | api.groq.com | |
| huggingface | huggingface | `HUGGING_FACE_LLM_API_KEY` | `HUGGING_FACE_LLM_ENDPOINT` | - | |
| kimi | moonshotAi | `MOONSHOT_AI_API_KEY` | - | api.moonshot.cn | **改 Node 命名映射** |
| koboldcpp | koboldCPP | (none) | `KOBOLD_CPP_BASE_PATH` | localhost:5001 | 本地 |
| lemonade | lemonade | `LEMONADE_LLM_API_KEY`(可选) | `LEMONADE_LLM_BASE_PATH` | localhost:8000 | 本地+key |
| litellm | liteLLM | `LITE_LLM_API_KEY`(可选) | `LITE_LLM_BASE_PATH` | - | proxy |
| lmstudio | lmStudio | (none) | `LMSTUDIO_BASE_PATH` | localhost:1234 | 本地 |
| localai | localAi | `LOCAL_AI_API_KEY`(可选) | `LOCAL_AI_BASE_PATH` | localhost:8080 | 本地 |
| minimax | (Node 无) | `MINIMAX_API_KEY` | - | api.minimax.chat | pantheon-only |
| mistral | mistral | `MISTRAL_API_KEY` | - | api.mistral.ai | |
| native | (Node 无) | - | - | - | local GGUF;不接入 |
| novita | novita | `NOVITA_LLM_API_KEY` | - | api.novita.ai | |
| nvidianim | nvidiaNim | `NVIDIA_NIM_LLM_API_KEY` | `NVIDIA_NIM_LLM_BASE_PATH` | - | |
| ollama | ollama | `OLLAMA_AUTH_TOKEN`(可选) | `OLLAMA_BASE_PATH` | localhost:11434 | 已实现 |
| openai | openAi | `OPEN_AI_KEY` | - | api.openai.com | 已实现 |
| openrouter | openRouter | `OPENROUTER_API_KEY` | - | openrouter.ai | |
| perplexity | perplexity | `PERPLEXITY_API_KEY` | - | api.perplexity.ai | |
| ppio | ppio | `PPIO_API_KEY` | - | - | |
| privatemode | privatemode | `PRIVATEMODE_API_KEY` | `PRIVATEMODE_BASE_URL` | - | |
| qwen | (Node 无) | `QWEN_API_KEY` | - | dashscope.aliyuncs.com | pantheon-only |
| sambanova | sambanova | `SAMBA_NOVA_API_KEY` | - | api.sambanova.ai | |
| textgenwebui | textGenWebUI | (none) | `TEXT_GEN_WEB_UI_BASE_PATH` | - | 本地 |
| together | togetherAi | `TOGETHER_AI_API_KEY` | - | api.together.xyz | |
| wenxin | (Node 无) | `WENXIN_API_KEY` + `WENXIN_SECRET_KEY` | - | aip.baidubce.com | 百度;**双密钥** |
| xai | xai | `XAI_LLM_API_KEY` | - | api.x.ai | |
| zai | zai | `ZAI_API_KEY` | - | - | |
| zhipu | (Node 无) | `ZHIPU_API_KEY` | - | open.bigmodel.cn | pantheon-only |

> Model preference key 一律 `<PROVIDER>_MODEL_PREF` (跟 Node ` env.MODEL_PREF` 一致)。

### 3.4 配置加载顺序

```
1. settings[<Provider>_API_KEY]            # SystemSetting (UI 可改)
2. cfg.<Provider>APIKey (env)               # 通过 caarlos0/env 解析
3. cfg.LLMApiKey                            # 兜底
4. cfg.OpenAiKey                            # 兜底兜底
```

`buildProviderSpec` 内部用 `pick` 链:

```go
func buildProviderSpec(cfg *config.Config, settings map[string]string) ProviderSpec {
    name := strings.ToLower(pick("LLMProvider", settings, cfg.LLMProvider))
    name = normalizeProviderName(name)  // azureOpenAi → azure, gemini → google, moonshotAi → kimi
    keys := envKeysFor(name)
    return ProviderSpec{
        Name:    name,
        APIKey:  resolveKey(keys.APIKey, cfg, settings),
        BaseURL: resolveBaseURL(keys.BaseURL, cfg, settings),
        ExtraOptions: resolveExtra(name, cfg, settings),
    }
}
```

### 3.5 Embedder × 4 实际实现

| Pantheon embed provider | Node 名 | Default model | 用途 |
|---|---|---|---|
| `openaicompat` | openAi / lmStudio / localAi / liteLLM / ollama / openRouter / azureOpenAi / lemonade / mistral / gemini / genericOpenAi | text-embedding-3-small | 几乎所有 openai-style endpoints 走这个 + 不同 BaseURL |
| `cohere` | cohere | embed-english-v3.0 | |
| `voyage` | voyageAi | voyage-3 | |
| `native` | native | - | 不接入 |

Embedder 工厂关键 switch:

```go
switch embedderName {
case "openai":     // openaicompat + api.openai.com
case "ollama":     // openaicompat + localhost:11434/v1
case "lmstudio":   // openaicompat + localhost:1234/v1
case "localai":    // openaicompat + localhost:8080/v1
case "litellm":    // openaicompat + cfg-driven
case "openrouter": // openaicompat + openrouter.ai/api/v1
case "azure":      // openaicompat + azure endpoint
case "lemonade":   // openaicompat + cfg-driven
case "mistral":    // openaicompat + api.mistral.ai/v1
case "gemini":     // openaicompat + generativelanguage.googleapis.com/v1beta/openai
case "genericopenai": // openaicompat + cfg-driven
case "cohere":     // cohere.New
case "voyage":     // voyage.New
default:           // 同 openai-compat 默认
}
```

### 3.6 Rerank 集成路径

```
ChatService.buildRAGContext:
    1. embedder.EmbedQuery → queryVec
    2. vectorSvc.SimilaritySearch(queryVec, opts{TopN: 12})  # 取多
    3. reranker.Rerank(query, results.Texts(), topN=4)        # 真排序
    4. 取前 topN 拼 system prompt
```

`vectordb.SearchOptions` 改造:

```go
type SearchOptions struct {
    TopN                int
    SimilarityThreshold float64
    // 删除:Rerank bool
}
```

`vectordb.SimilaritySearch` 不再关心 rerank;reranker 在 service 层调用一次。这把 reranker 关注点从向量库剥离,清晰多了。

---

## 4. 配置项扩展

### 4.1 `config.Config` 字段(批量新增)

```go
type Config struct {
    // ... existing ...

    // === LLM provider envs (38 total; alphabetical) ===
    AnthropicAPIKey         string `env:"ANTHROPIC_API_KEY"`
    AnthropicBaseURL        string `env:"ANTHROPIC_BASE_URL"`
    ApipieLLMAPIKey         string `env:"APIPIE_LLM_API_KEY"`
    AzureOpenAIKey          string `env:"AZURE_OPENAI_KEY"`
    AzureOpenAIEndpoint     string `env:"AZURE_OPENAI_ENDPOINT"`
    AzureOpenAIDeployment   string `env:"AZURE_OPENAI_DEPLOYMENT"`
    BedrockAWSAccessKeyID   string `env:"AWS_BEDROCK_LLM_ACCESS_KEY_ID"`
    BedrockAWSSecretKey     string `env:"AWS_BEDROCK_LLM_ACCESS_KEY"`
    BedrockAWSRegion        string `env:"AWS_BEDROCK_LLM_REGION"`
    CohereAPIKey            string `env:"COHERE_API_KEY"`
    CometAPIKey             string `env:"COMETAPI_LLM_API_KEY"`
    DeepseekAPIKey          string `env:"DEEPSEEK_API_KEY"`
    // ... (一长串,按 §3.3 表格)

    // === Model preference envs (38 total) ===
    AnthropicModelPref      string `env:"ANTHROPIC_MODEL_PREF"`
    OpenAIModelPref         string `env:"OPEN_AI_MODEL_PREF"`
    // ... (一长串)

    // === Embedding ===
    EmbeddingProvider       string `env:"EMBEDDING_ENGINE" envDefault:"openai"`
    EmbeddingBasePath       string `env:"EMBEDDING_BASE_PATH"`
    EmbeddingModelPref      string `env:"EMBEDDING_MODEL_PREF"`

    // === Reranker ===
    RerankProvider          string `env:"RERANK_PROVIDER" envDefault:""`  // "" = noop
    RerankAPIKey            string `env:"RERANK_API_KEY"`
    RerankModel             string `env:"RERANK_MODEL" envDefault:"rerank-english-v3.0"`
}
```

约 60 个新字段(38 keys + 38 model prefs + base URLs + 3 rerank)。**全部走 caarlos0/env 标签,零样板代码**。

### 4.2 SystemSetting 覆盖

`buildProviderSpec` 优先读 settings(数据库),没值再读 env。Settings 命名与 Node SystemSetting 一致:

| Settings key | 对应 env |
|---|---|
| `LLMProvider` | `LLM_PROVIDER` |
| `OpenAiKey` | `OPEN_AI_KEY` |
| `AnthropicApiKey` | `ANTHROPIC_API_KEY` |
| `AnthropicModelPref` | `ANTHROPIC_MODEL_PREF` |
| ... | ... |

→ 与 Node 1:1 (因为 Node 走 prisma 表 SystemSettings,key 名固定)。**双进程部署期,SystemSetting 由 Node 写、Go 读,无任何兼容问题**。

---

## 5. 测试策略

### 5.1 表驱动 provider switch 测试

```go
// internal/providers/llm_test.go

func TestNewLLMProvider_AllProviders(t *testing.T) {
    cases := []struct{
        name        string         // pantheon canonical name
        nodeAlias   string         // node-style name
        envKey      string         // primary api key env
        envValue    string         // fake key
        baseURL     string         // optional override
        expectProvider string      // pantheon Provider().String()
    }{
        {"openai", "openAi", "OPEN_AI_KEY", "sk-test", "", "openai"},
        {"anthropic", "anthropic", "ANTHROPIC_API_KEY", "sk-ant-test", "", "anthropic"},
        {"google", "gemini", "GEMINI_API_KEY", "AIza-test", "", "google"},
        {"ollama", "ollama", "", "", "http://127.0.0.1:11434", "ollama"},
        // ... 38 行
    }
    for _, c := range cases {
        t.Run(c.name, func(t *testing.T) {
            t.Setenv(c.envKey, c.envValue)
            cfg, _ := config.Load()
            cfg.LLMProvider = c.nodeAlias  // 模拟用户填 Node 名
            settings := map[string]string{}
            lm := providers.NewLLMProvider(cfg, settings)
            require.NotNil(t, lm)
            // 不要求能真请求,只验证不返回 noopLLM 即可
            _, isNoop := lm.(*providers.NoopLLM)
            require.False(t, isNoop, "provider %q built noop", c.name)
            require.Equal(t, c.expectProvider, lm.LanguageModel().Provider())
        })
    }
}
```

**测试只做 "build-only smoke" — 不发实际 API 请求**。覆盖率: 38 个 case × 1 个 assert = 38 个 sub-test。

### 5.2 Embedder 测试

同款 build-only,4 个 case:

```go
{"openai", "OPEN_AI_KEY", "sk-test", "openai", "text-embedding-3-small"},
{"cohere", "COHERE_API_KEY", "test", "cohere", "embed-english-v3.0"},
{"voyage", "VOYAGE_API_KEY", "test", "voyage", "voyage-3"},
{"ollama", "", "", "openaicompat", "nomic-embed-text"},
```

### 5.3 Rerank 测试

```go
func TestReranker_NoopByDefault(t *testing.T) {
    cfg := &config.Config{}
    r, err := reranker.NewReranker(cfg, nil)
    require.NoError(t, err)
    require.IsType(t, &reranker.NoopReranker{}, r)
}

func TestReranker_Cohere_BuildsPantheonReranker(t *testing.T) {
    t.Setenv("COHERE_API_KEY", "test")
    cfg := &config.Config{RerankProvider: "cohere"}
    r, err := reranker.NewReranker(cfg, nil)
    require.NoError(t, err)
    require.IsType(t, &reranker.PantheonReranker{}, r)
}

func TestReranker_Rerank_ReordersDocs(t *testing.T) {
    // mock cohere endpoint via httptest;
    // 返回 [{index:2,score:0.9},{index:0,score:0.7},{index:1,score:0.3}];
    // 验证返回顺序
}
```

### 5.4 Node-alias 映射测试

```go
func TestNormalizeProviderName(t *testing.T) {
    cases := []struct{ in, out string }{
        {"openAi", "openai"},
        {"azureOpenAi", "azure"},
        {"gemini", "google"},
        {"moonshotAi", "kimi"},
        {"togetherAi", "together"},
        {"fireworksAi", "fireworks"},
        {"voyageAi", "voyage"},
    }
    for _, c := range cases {
        require.Equal(t, c.out, providers.NormalizeProviderName(c.in))
    }
}
```

### 5.5 端到端冒烟(可选,需凭据)

如果 CI 环境有 `OPEN_AI_KEY`,跑一个 hello-world 真请求测试,放 build tag `integration` 后面:

```go
//go:build integration

func TestLLM_OpenAI_RealAPI_HelloWorld(t *testing.T) {
    if os.Getenv("OPEN_AI_KEY") == "" { t.Skip("OPEN_AI_KEY not set") }
    cfg, _ := config.Load()
    lm := providers.NewLLMProvider(cfg, nil)
    resp, err := lm.Complete(context.Background(), []core.Message{
        core.NewTextMessage(core.MESSAGE_ROLE_USER, "say hi in one word"),
    }, "", nil)
    require.NoError(t, err)
    require.NotEmpty(t, resp)
}
```

---

## 6. 分期内容(此文档对应一个 PR-V3A)

| Task | 内容 | 工时 |
|---|---|---|
| 0 | 接线 + `normalizeProviderName` + 配置 envs(60 字段) + decision artefact | 3h |
| 1 | `providers/factory.go` + `env_keys.go`(38 provider 名 + key 映射表) | 4h |
| 2 | `providers/llm.go` 重写(switch 表 + WithBaseURL 选项构造)+ 表驱动测试 | 6h |
| 3 | `embedder/factory.go` + 4 case + 测试 | 4h |
| 4 | `reranker/` 包(interface + Pantheon impl + Noop + factory)+ 测试 | 4h |
| 5 | `vectordb.SearchOptions.Rerank` 字段下线 + `chat_service` 接入 reranker | 4h |
| 6 | `cmd/server/main.go` 接线 + 总集成测试 + 文档(`.env.example` 更新) | 3h |

**总计 28h**(估算 25-35h,中段)。

---

## 7. 风险与权衡

| 风险 | 缓解 |
|---|---|
| 38 个 provider 中,部分 (bedrock / wenxin) 凭据形式复杂(多字段) | 给这两个 case 单独的 `ExtraOptions` 字段;其它统一 `apiKey + baseURL` |
| pantheon 某个 provider 与 Node 行为不一致(streaming 协议) | pantheon 自己已经统一到 `core.StreamResponse`;Node-pantheon 行为差异不暴露给上层 |
| Azure 走 `openai-compat` 子路径,deployment 不放 BaseURL | 用 `ExtraOptions["azure_deployment"]`;pantheon `azure.New` 已经支持 |
| Bedrock 需要 AWS SDK,可能引入大依赖 | 验证: `cd $PANTHEON/providers/bedrock && go list -deps . | grep aws-sdk-go-v2 | wc -l` — pantheon 自己引了;Go 端不重复引,直接用 pantheon 包装 |
| 测试环境无凭据,build-only smoke 测不到真正问题 | 给关键 provider (openai/anthropic/ollama) 加 `httptest` mock + 真请求;其它 build-only 即可 |
| 用户填 "azure" 还是 "azureOpenAi"? | `normalizeProviderName` 双向兼容,大小写不敏感 |
| Embedder dimensions 与 chat_service / vectordb 假设硬编码 1536 | 在 PantheonEmbedder.Dimensions() 实测一次后缓存;chat_service 不再硬编码 |
| 删除 `vectordb.SearchOptions.Rerank` 是破坏性改动 | 同 PR 内一次性改完所有 adapter;搜 grep 后逐个改 |
| 旧的 fetch-more-then-truncate 路径完全失效 | 默认 `RerankProvider=""` → NoopReranker 直接 fall-through,等价旧路径 (但少了 ×3 multiplier) |
| 用户在 SystemSetting 里存 Node 风格 key,Go 读不到 | `pick` 函数同时尝试 Node + Go 两种命名(camelCase ↔ snake_case) |

---

## 8. 后续(不在 PR-V3A 范围)

- `native` provider 接入 (local GGUF / llama.cpp,需要独立设计)
- Provider 凭据加密落库(沿 OAuth `enc:` 前缀模式扩展)
- 自适应模型上下文长度(`pantheon/core.Model().ContextWindow()`,目前 Go 走静态 4096 兜底)
- 多 provider fallback 链(主 provider 失败自动切备用)
- Per-workspace provider 选择(Workspace.AgentProvider 已有,但目前未读)
- Rerank 多 provider 支持(jina / voyage 等)

---

## 9. 实施计划文件

- `.gpowers/plans/2026-05-28-provider-breadth-v3a.md`(待写)

—— end of design
