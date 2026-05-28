# v3-A — Provider Breadth Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `gpowers:subagent-driven-development` (recommended) or `gpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the remaining gaps in provider breadth: (1)补齐 5 个 pantheon-only LLM provider (`minimax`/`qwen`/`wenxin`/`zhipu` + 评估 `native`);(2) 把 `embedder/pantheon.go` 从写死 openai 拓宽到 4-case 工厂(openai-compat / cohere / voyage);(3) 新建 `internal/reranker/` 包,把 `vectordb.SearchOptions.Rerank bool` 假 rerank 替换成真实模型 rerank(首发接 cohere)。

**先决条件**: 无 — `providers/builders.go` 已经实装 36 个 LLM provider,只剩补齐 + embedder/reranker。可以与任何 v1/v2 PR 并行。

**Source spec:** `.gpowers/designs/2026-05-27-provider-breadth-design.md` (design 设想 38-provider 全量改造,但实际 builders.go 已落地 36 个;plan 按真实剩余工作量裁剪,**design §3.3 的因素表只剩 5 行未覆盖**)。

**估算修订**: design 写 28h,**实际剩余 ~18h**(LLM 拓宽 6h + 4h 中绝大部分已完成,只补 3h)。

---

## Pre-task: 现状盘点(实测 5/27)

### LLM provider 已实装情况

`backend/internal/providers/builders.go` 在 init() 中注册了 **36 个 provider**:

```
openai, azure, anthropic, gemini, lmstudio, localai, ollama, togetherai,
fireworksai, mistral, huggingface, perplexity, openrouter, novita, groq,
koboldcpp, textgenwebui, cohere, litellm, generic-openai, bedrock, deepseek,
apipie, xai, nvidia-nim, ppio, dpaiStudio, moonshotai, cometapi, foundry,
zai, giteeai, docker-model-runner, privatemode, sambanova, lemonade
```

对照 design §3.3 表格 38 行,**未实装的 5 个**:

| Pantheon name | 备注 |
|---|---|
| `minimax` | pantheon-only (国内厂商) |
| `qwen` | pantheon-only (阿里通义) |
| `wenxin` | pantheon-only (百度文心),**需双密钥** `API_KEY + SECRET_KEY` |
| `zhipu` | pantheon-only (智谱 GLM) |
| `native` | pantheon 本地 GGUF —— **本 PR 显式 skip**,需要 llama.cpp 集成,独立 PR |

> 与 design 设想的差距:这 4 个华系 pantheon 独有 provider 都没在 Node 出现,所以 builders.go 之前的接线没覆盖。补齐这 4 个就完整了 v3-A 的 LLM 部分。

### Embedder 现状

`backend/internal/embedder/pantheon.go` 写死 `openai.New`,仅支持 OpenAI 1 个 embedder。pantheon 实际可用 embedding provider 共 **4 个**:

| Pantheon | Node 名等价集合 | 实际工作 |
|---|---|---|
| `openaicompat` | openAi / lmStudio / localAi / liteLLM / ollama / openRouter / azureOpenAi / lemonade / mistral / gemini / genericOpenAi (Node 14 个里 10+ 个) | 同一适配器 + 不同 BaseURL |
| `cohere` | cohere | 独立 provider |
| `voyage` | voyageAi | 独立 provider |
| `native` | native | skip |

### Reranker 现状

- `internal/reranker/` 包不存在
- `vectordb.SearchOptions.Rerank bool` 只在 `pgvector.go:146` 一处被使用(`limit *= 3` 然后取前 N,**不调真 reranker 模型**)
- 4 个调用点传 `SearchOptions{...}` 字面量需要同步改:
  - `chat_service.go:77`
  - `vector_search_service.go:53`
  - `embed_service.go:411`
  - 以及任何 test 文件里的 SearchOptions 字面量

### pantheon rerank 实装提供方

经实测,pantheon v0.0.9 中实现 `rerank.Provider` 接口的 provider 只有 **`cohere` 1 个**(看 `$PANTHEON/providers/cohere/` 有 `rerank.go`)。v3-A 接 cohere,其它走 Noop。

### Methods to ship (PR-V3A scope)

| # | Owner | Signature | Notes |
|---|---|---|---|
| 1 | `providers.buildMiniMax/Qwen/Wenxin/Zhipu` | `providerBuilder` signature | 加入 init() |
| 2 | `providers.normalizeProviderName(string) string` | Already exists (in `resolve.go`?); add 4 aliases if needed | |
| 3 | `embedder.NewEmbedder(cfg, settings) (Embedder, error)` | replaces `NewPantheonEmbedder`;factory 4-case | |
| 4 | `embedder.buildOpenAICompat/Cohere/Voyage` | per-vendor builders | |
| 5 | `reranker.Reranker` interface + `PantheonReranker` + `NoopReranker` | New package | |
| 6 | `reranker.NewReranker(cfg, settings) (Reranker, error)` | factory; "" / "noop" / "none" → Noop | |
| 7 | `vectordb.SearchOptions` | Remove `Rerank bool` field | breaking change to internal type only |
| 8 | `services.ChatService` | new field `Reranker reranker.Reranker`;injected in NewChatService | |
| 9 | `services.VectorSearchService` | same | |
| 10 | `services.EmbedService` | same | |

### Out of scope (explicit)

- `native` LLM provider (local GGUF) — 独立 PR
- Reranker 多 provider (jina / voyage / 自建 SLM) — 暂只 cohere
- 配置加密落库(沿用现状) — OAuth `enc:` 模式扩展是独立 PR
- Per-workspace provider 选择(Workspace.AgentProvider 已有,但仍未读) — 独立 PR
- Modifying `Workspace.AgentProvider` resolution logic — 独立 PR
- Adaptive context-window detection (`pantheon/core.Model.ContextWindow()`) — 独立 PR

### TDD discipline

Each task lands as **one commit**. Failing test → impl → green → full suite green → commit.

---

## Task 0: 补 5 个 pantheon-only LLM env keys + decision artefact

**Files:**
- `backend/internal/config/config.go` (MODIFY — 加 4 个 pantheon-only env)
- `.gpowers/decisions/2026-05-27-pantheon-native-deferred.md` (NEW)
- `.gpowers/decisions/2026-05-27-reranker-cohere-only-v1.md` (NEW)
- `.env.example` (MODIFY — 加 4 个新 env 注释)

**Tests:** none yet (declarative only).

### Steps

- [ ] 加 4 个 env 到 `Config`:
  ```go
  // === MiniMax (pantheon-only) ===
  MinimaxAPIKey      string `env:"MINIMAX_API_KEY"`
  MinimaxModelPref   string `env:"MINIMAX_MODEL_PREF"`
  // === Qwen (pantheon-only) ===
  QwenAPIKey         string `env:"QWEN_API_KEY"`
  QwenModelPref      string `env:"QWEN_MODEL_PREF"`
  // === Wenxin (pantheon-only) — dual-key auth ===
  WenxinAPIKey       string `env:"WENXIN_API_KEY"`
  WenxinSecretKey    string `env:"WENXIN_SECRET_KEY"`
  WenxinModelPref    string `env:"WENXIN_MODEL_PREF"`
  // === Zhipu (pantheon-only) ===
  ZhipuAPIKey        string `env:"ZHIPU_API_KEY"`
  ZhipuModelPref     string `env:"ZHIPU_MODEL_PREF"`

  // === Reranker ===
  RerankProvider     string `env:"RERANK_PROVIDER" envDefault:""` // "" = noop
  RerankAPIKey       string `env:"RERANK_API_KEY"`
  RerankModelPref    string `env:"RERANK_MODEL" envDefault:"rerank-english-v3.0"`
  ```

- [ ] 写 `.gpowers/decisions/2026-05-27-pantheon-native-deferred.md`:
  ```markdown
  # Pantheon `native` Provider — Deferred

  **Date**: 2026-05-27
  **Status**: Adopted
  **Context**: pantheon 提供 `native` provider(本地 GGUF + llama.cpp 集成)。我们不在 v3-A 接入。
  **Reason**: 需要本地 llama.cpp shared library,影响 docker 镜像体积 + 跨平台编译(Mac arm64 / Linux amd64 / Windows)。
  **Mitigation**: 用户用 `ollama` 或 `lmstudio` provider 间接达成"本地 LLM"目标。
  ```

- [ ] 写 `.gpowers/decisions/2026-05-27-reranker-cohere-only-v1.md`:
  ```markdown
  # Reranker — Cohere Only for v1

  **Date**: 2026-05-27
  **Status**: Adopted
  **Context**: pantheon v0.0.9 中实现 `rerank.Provider` 接口的 provider 只有 `cohere`。
  **Decision**: v3-A 只接 cohere;其它 reranker provider("none"/"noop"/"")走 NoopReranker。
  **Future**: pantheon 后续如增加 jina / voyage 等 reranker,新增 case 即可。
  ```

- [ ] `.env.example` 加注释块,标注 4 个新 env + reranker 三件。

- [ ] `go build ./...` 干净;`go vet ./...` 干净。

### Acceptance

- 4 个 pantheon-only env + 3 个 reranker env 解析正常
- 2 个 decision artefact 落地
- `.env.example` 含新条目

### Commit

`feat(config): add pantheon-only LLM env keys + reranker config knobs`

---

## Task 1: 补 4 个 pantheon-only LLM builder

**Files:**
- `backend/internal/providers/builders.go` (MODIFY)
- `backend/internal/providers/builders_test.go` (MODIFY — add 4 test cases)
- `backend/internal/providers/resolve.go` (MODIFY if needed — verify alias map handles new names)
- `backend/internal/providers/resolve_test.go` (MODIFY)

**Tests:**
- `TestBuildMinimax_ReturnsLM`
- `TestBuildQwen_ReturnsLM`
- `TestBuildWenxin_DualKey_ReturnsLM` (验证 API_KEY + SECRET_KEY 都传)
- `TestBuildWenxin_MissingSecretKey_ReturnsError`
- `TestBuildZhipu_ReturnsLM`
- `TestResolveProviderName_PantheonOnly_NoAlias` (这 4 个名字本身就是 pantheon canonical,无 alias 需要)

### Steps

- [ ] 实现 `buildMinimax`:
  ```go
  func buildMinimax(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error) {
      apiKey := pick("MinimaxApiKey", settings, cfg.MinimaxAPIKey)
      if apiKey == "" { return nil, fmt.Errorf("minimax: no API key (set MINIMAX_API_KEY)") }
      p, err := minimax.New(apiKey)
      if err != nil { return nil, fmt.Errorf("minimax provider: %w", err) }
      return p.LanguageModel(ctx, modelID)
  }
  ```

- [ ] 实现 `buildQwen`(同款 pattern):
  ```go
  func buildQwen(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error) {
      apiKey := pick("QwenApiKey", settings, cfg.QwenAPIKey)
      if apiKey == "" { return nil, fmt.Errorf("qwen: no API key (set QWEN_API_KEY)") }
      p, err := qwen.New(apiKey)
      if err != nil { return nil, fmt.Errorf("qwen provider: %w", err) }
      return p.LanguageModel(ctx, modelID)
  }
  ```

- [ ] 实现 `buildWenxin`(**双密钥特例**):
  ```go
  func buildWenxin(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error) {
      apiKey := pick("WenxinApiKey", settings, cfg.WenxinAPIKey)
      secretKey := pick("WenxinSecretKey", settings, cfg.WenxinSecretKey)
      if apiKey == "" || secretKey == "" {
          return nil, fmt.Errorf("wenxin: requires both WENXIN_API_KEY and WENXIN_SECRET_KEY")
      }
      // pantheon wenxin.New 接收 (apiKey, secretKey) 双参数 —— 验证签名后调整
      p, err := wenxin.New(apiKey, wenxin.WithSecretKey(secretKey))
      if err != nil { return nil, fmt.Errorf("wenxin provider: %w", err) }
      return p.LanguageModel(ctx, modelID)
  }
  ```

  > **签名验证**: 实现时先 `cat $PANTHEON/providers/wenxin/provider.go | grep "^func New"` 确认是 `New(apiKey, secretKey)` 还是 `New(apiKey, WithSecret(...))`。Plan 假设是 Option 风格但留 fallback。

- [ ] 实现 `buildZhipu`(同款):
  ```go
  func buildZhipu(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error) {
      apiKey := pick("ZhipuApiKey", settings, cfg.ZhipuAPIKey)
      if apiKey == "" { return nil, fmt.Errorf("zhipu: no API key (set ZHIPU_API_KEY)") }
      p, err := zhipu.New(apiKey)
      if err != nil { return nil, fmt.Errorf("zhipu provider: %w", err) }
      return p.LanguageModel(ctx, modelID)
  }
  ```

- [ ] 注册到 init():
  ```go
  providerRegistry["minimax"] = buildMinimax
  providerRegistry["qwen"]    = buildQwen
  providerRegistry["wenxin"]  = buildWenxin
  providerRegistry["zhipu"]   = buildZhipu
  ```

- [ ] 加 4 个 builder 的 import:
  ```go
  import (
      "github.com/odysseythink/pantheon/providers/minimax"
      "github.com/odysseythink/pantheon/providers/qwen"
      "github.com/odysseythink/pantheon/providers/wenxin"
      "github.com/odysseythink/pantheon/providers/zhipu"
  )
  ```

- [ ] 添加 model preference 解析支持(`resolve.go` 或就近):
  ```go
  // resolveModelID 内部 switch 加 4 个 case
  case "minimax": return pick("MinimaxModelPref", settings, cfg.MinimaxModelPref)
  case "qwen":    return pick("QwenModelPref",    settings, cfg.QwenModelPref)
  case "wenxin":  return pick("WenxinModelPref",  settings, cfg.WenxinModelPref)
  case "zhipu":   return pick("ZhipuModelPref",   settings, cfg.ZhipuModelPref)
  ```

- [ ] 写 5 个 build-only smoke test(模仿现有 builders_test.go 风格,只要构造不返回错误即可):
  ```go
  func TestBuildWenxin_DualKey_ReturnsLM(t *testing.T) {
      t.Setenv("WENXIN_API_KEY", "fake-api")
      t.Setenv("WENXIN_SECRET_KEY", "fake-secret")
      cfg, _ := config.Load()
      cfg.LLMProvider = "wenxin"
      lm := providers.NewLLMProvider(cfg, nil)
      _, isNoop := lm.(*providers.NoopLLM)
      require.False(t, isNoop, "wenxin should build, not noop")
      require.Equal(t, "wenxin", lm.LanguageModel().Provider())
  }

  func TestBuildWenxin_MissingSecretKey_ReturnsError(t *testing.T) {
      t.Setenv("WENXIN_API_KEY", "only-one")
      // SECRET_KEY 不设
      cfg, _ := config.Load()
      cfg.LLMProvider = "wenxin"
      lm := providers.NewLLMProvider(cfg, nil)
      _, isNoop := lm.(*providers.NoopLLM)
      require.True(t, isNoop, "wenxin without secret should be noop")
  }
  ```

- [ ] Run all 5 tests; full suite green; total registered count now 40 (36 + 4).

### Acceptance

- 4 个 builder 注册到 registry
- 双密钥校验在 wenxin 上工作
- 5 个新测试通过
- `provider_breadth_design.md` 表格 38 行中,38 个实际全覆盖(40 - native skip = 39,留 1 个 native gap 由 decision artefact 承认)

### Commit

`feat(providers): add minimax + qwen + wenxin + zhipu LLM builders`

---

## Task 2: Embedder factory(4 case)

**Files:**
- `backend/internal/embedder/pantheon.go` (REFACTOR — 拆分构造逻辑)
- `backend/internal/embedder/factory.go` (NEW)
- `backend/internal/embedder/factory_test.go` (NEW)
- `backend/internal/config/config.go` (MODIFY — add `EmbeddingProvider` env)

**Tests:**
- `TestNewEmbedder_OpenAI_BuildsModel`
- `TestNewEmbedder_OpenAICompatViaOllama_BuildsModel` (override BaseURL)
- `TestNewEmbedder_LMStudio_BuildsModel`
- `TestNewEmbedder_LocalAI_BuildsModel`
- `TestNewEmbedder_Cohere_BuildsModel`
- `TestNewEmbedder_Voyage_BuildsModel`
- `TestNewEmbedder_NoAPIKey_ReturnsError`
- `TestNewEmbedder_UnknownProvider_FallsBackToOpenAICompat`

### Steps

- [ ] 加 config knob:
  ```go
  // === Embedding ===
  EmbeddingProvider  string `env:"EMBEDDING_ENGINE" envDefault:"openai"`
  EmbeddingBasePath  string `env:"EMBEDDING_BASE_PATH"`
  // (EmbeddingModel 和 EmbeddingApiKey 现存,沿用)
  ```

- [ ] 创建 `embedder/factory.go`:
  ```go
  package embedder

  import (
      "context"
      "fmt"
      "strings"

      "github.com/odysseythink/hermind/backend/internal/config"
      "github.com/odysseythink/pantheon/core"
      "github.com/odysseythink/pantheon/extensions/embed"
      "github.com/odysseythink/pantheon/providers/cohere"
      "github.com/odysseythink/pantheon/providers/openai"
      "github.com/odysseythink/pantheon/providers/voyage"
  )

  // NewEmbedder constructs an embedder based on cfg.EmbeddingProvider.
  // Settings override cfg fields where provided.
  func NewEmbedder(cfg *config.Config, settings map[string]string) (Embedder, error) {
      name := strings.ToLower(pickStr(settings, "EmbeddingEngine", cfg.EmbeddingProvider))
      apiKey := pickStr(settings, "EmbeddingApiKey", cfg.EmbeddingApiKey)
      if apiKey == "" { apiKey = pickStr(settings, "OpenAiKey", cfg.OpenAiKey) }
      baseURL := pickStr(settings, "EmbeddingBasePath", cfg.EmbeddingBasePath)
      modelID := pickStr(settings, "EmbeddingModelPref", cfg.EmbeddingModel)

      var prov core.Provider
      var err error
      switch name {
      case "cohere":
          if apiKey == "" { return nil, fmt.Errorf("cohere embedder: no API key") }
          prov, err = cohere.New(apiKey)
          if modelID == "" { modelID = "embed-english-v3.0" }
      case "voyage", "voyageai":
          if apiKey == "" { return nil, fmt.Errorf("voyage embedder: no API key") }
          prov, err = voyage.New(apiKey)
          if modelID == "" { modelID = "voyage-3" }
      default:
          // openai-compat: includes openai, ollama, lmstudio, localai, litellm, openrouter, azure, mistral, gemini, lemonade, genericopenai, etc.
          if apiKey == "" && requiresAPIKey(name) {
              return nil, fmt.Errorf("%s embedder: no API key", name)
          }
          opts := []openai.Option{}
          if baseURL != "" { opts = append(opts, openai.WithBaseURL(baseURL)) }
          prov, err = openai.New(apiKey, opts...)
          if modelID == "" { modelID = defaultEmbeddingModelFor(name) }
      }
      if err != nil { return nil, fmt.Errorf("create %s provider: %w", name, err) }

      embedProv, ok := prov.(embed.Provider)
      if !ok { return nil, fmt.Errorf("provider %q does not support embedding", name) }
      model, err := embedProv.EmbeddingModel(context.Background(), modelID)
      if err != nil { return nil, fmt.Errorf("create embedding model: %w", err) }

      return &PantheonEmbedder{model: model}, nil
  }

  func pickStr(settings map[string]string, key, fallback string) string {
      if v, ok := settings[key]; ok && v != "" { return v }
      return fallback
  }

  // requiresAPIKey returns true for hosted providers; false for local (ollama/lmstudio/localai).
  func requiresAPIKey(name string) bool {
      switch name {
      case "ollama", "lmstudio", "localai", "litellm", "lemonade":
          return false
      }
      return true
  }

  func defaultEmbeddingModelFor(name string) string {
      switch name {
      case "ollama":   return "nomic-embed-text"
      case "lmstudio", "localai": return "nomic-embed-text-v1.5"
      case "gemini":   return "text-embedding-004"
      case "mistral":  return "mistral-embed"
      default:         return "text-embedding-3-small"
      }
  }
  ```

- [ ] **`PantheonEmbedder` 保留**,但 `NewPantheonEmbedder` 函数标 deprecated:
  ```go
  // Deprecated: use NewEmbedder which supports multiple providers.
  func NewPantheonEmbedder(cfg *config.Config) (*PantheonEmbedder, error) {
      e, err := NewEmbedder(cfg, nil)
      if err != nil { return nil, err }
      return e.(*PantheonEmbedder), nil
  }
  ```

- [ ] 写 8 个测试,用 build-only smoke (不发真请求):
  ```go
  func TestNewEmbedder_Cohere_BuildsModel(t *testing.T) {
      t.Setenv("EMBEDDING_API_KEY", "test")
      cfg, _ := config.Load()
      cfg.EmbeddingProvider = "cohere"
      e, err := embedder.NewEmbedder(cfg, nil)
      require.NoError(t, err)
      require.NotNil(t, e)
  }
  ```

- [ ] 更新 `cmd/server/main.go` 用 `NewEmbedder` 替换 `NewPantheonEmbedder`:
  ```go
  // 之前
  emb, err := embedder.NewPantheonEmbedder(cfg)
  // 之后
  settings, _ := sysSvc.GetAllSettings(context.Background())
  emb, err := embedder.NewEmbedder(cfg, settings)
  ```
  > 注意 settings 加载顺序:如果 sysSvc 还没就绪,设 `settings = nil`,fall back 到 cfg only。

- [ ] Run all 8 tests; full suite green.

### Acceptance

- 8 个测试通过
- 现有 OpenAI embedder 用例不回归
- Ollama/LMStudio/LocalAI 走 `openai-compat` + 自定义 BaseURL 能构造
- Cohere / Voyage 独立 provider 能构造
- `NewPantheonEmbedder` deprecated 但仍可调用

### Commit

`feat(embedder): factory with openai-compat + cohere + voyage support`

---

## Task 3: Reranker 包(interface + Pantheon + Noop + factory)

**Files:**
- `backend/internal/reranker/doc.go` (NEW)
- `backend/internal/reranker/reranker.go` (NEW)
- `backend/internal/reranker/reranker_test.go` (NEW)
- `backend/internal/reranker/factory.go` (NEW)
- `backend/internal/reranker/factory_test.go` (NEW)

**Tests:**
- `TestNoopReranker_PassthroughTopN`
- `TestNoopReranker_TopNExceedsLen_ReturnsAll`
- `TestPantheonReranker_Cohere_BuildsModel` (build-only smoke)
- `TestPantheonReranker_Rerank_ReordersDocs` (httptest mock cohere API)
- `TestPantheonReranker_EmptyDocs_ReturnsEmpty`
- `TestNewReranker_EmptyProvider_ReturnsNoop`
- `TestNewReranker_NonePrefix_ReturnsNoop` (`""`/`"noop"`/`"none"`)
- `TestNewReranker_Cohere_BuildsPantheonReranker`
- `TestNewReranker_UnknownProvider_ReturnsError`
- `TestNewReranker_CohereWithoutKey_ReturnsError`

### Steps

- [ ] 写 `doc.go`:
  ```go
  // Package reranker provides relevance-based reordering of retrieved documents.
  //
  // The default NoopReranker passes through the input order (compatible with
  // the previous vectordb.SearchOptions.Rerank fetch-more-then-truncate
  // behavior). PantheonReranker delegates to pantheon's rerank.RerankModel,
  // which currently has only one implementation: Cohere.
  package reranker
  ```

- [ ] 实现 `reranker.go`:
  ```go
  package reranker

  import (
      "context"

      "github.com/odysseythink/pantheon/extensions/rerank"
  )

  // ScoredDocument is a document with its relevance score.
  type ScoredDocument struct {
      Index int
      Score float64
      Text  string
  }

  // Reranker reorders a list of documents by relevance to a query.
  // Implementations may return a subset (topN) or all.
  type Reranker interface {
      Rerank(ctx context.Context, query string, documents []string, topN int) ([]ScoredDocument, error)
  }

  // NoopReranker is a passthrough reranker. Returns up to topN documents
  // in their original order with Score=0.
  type NoopReranker struct{}

  func (n *NoopReranker) Rerank(ctx context.Context, query string, docs []string, topN int) ([]ScoredDocument, error) {
      if topN <= 0 || topN > len(docs) { topN = len(docs) }
      out := make([]ScoredDocument, 0, topN)
      for i := 0; i < topN; i++ {
          out = append(out, ScoredDocument{Index: i, Score: 0, Text: docs[i]})
      }
      return out, nil
  }

  // PantheonReranker wraps a pantheon rerank.RerankModel.
  type PantheonReranker struct {
      model rerank.RerankModel
  }

  func NewPantheonReranker(model rerank.RerankModel) *PantheonReranker {
      return &PantheonReranker{model: model}
  }

  func (p *PantheonReranker) Rerank(ctx context.Context, query string, docs []string, topN int) ([]ScoredDocument, error) {
      if len(docs) == 0 { return nil, nil }
      if topN <= 0 || topN > len(docs) { topN = len(docs) }
      resp, err := p.model.Rerank(ctx, &rerank.RerankRequest{
          Query:           query,
          Documents:       docs,
          TopN:            topN,
          ReturnDocuments: true,
      })
      if err != nil { return nil, err }
      out := make([]ScoredDocument, 0, len(resp.Results))
      for _, r := range resp.Results {
          text := ""
          if r.Index >= 0 && r.Index < len(docs) { text = docs[r.Index] }
          out = append(out, ScoredDocument{Index: r.Index, Score: r.RelevanceScore, Text: text})
      }
      return out, nil
  }
  ```

  > **Field name check**: pantheon `RerankResult` 字段名实际可能是 `RelevanceScore` 还是 `Score`?执行时先 `grep -A 5 "type RerankResult" $PANTHEON/extensions/rerank/provider.go` 确认。Plan 假设 `RelevanceScore`,实测纠正。

- [ ] 实现 `factory.go`:
  ```go
  package reranker

  import (
      "context"
      "fmt"
      "strings"

      "github.com/odysseythink/hermind/backend/internal/config"
      "github.com/odysseythink/pantheon/extensions/rerank"
      "github.com/odysseythink/pantheon/providers/cohere"
  )

  // NewReranker returns a Reranker selected by cfg.RerankProvider.
  // Empty / "none" / "noop" → NoopReranker.
  // "cohere" → PantheonReranker wrapping cohere.
  // Other → error.
  func NewReranker(cfg *config.Config, settings map[string]string) (Reranker, error) {
      name := strings.ToLower(pickStr(settings, "RerankProvider", cfg.RerankProvider))
      switch name {
      case "", "none", "noop":
          return &NoopReranker{}, nil
      case "cohere":
          apiKey := pickStr(settings, "RerankApiKey", cfg.RerankAPIKey)
          if apiKey == "" { apiKey = pickStr(settings, "CohereApiKey", cfg.CohereAPIKey) }
          if apiKey == "" { return nil, fmt.Errorf("cohere reranker: no API key (set RERANK_API_KEY or COHERE_API_KEY)") }
          prov, err := cohere.New(apiKey)
          if err != nil { return nil, fmt.Errorf("create cohere provider: %w", err) }
          rprov, ok := prov.(rerank.Provider)
          if !ok { return nil, fmt.Errorf("cohere provider does not implement rerank.Provider") }
          modelID := pickStr(settings, "RerankModelPref", cfg.RerankModelPref)
          if modelID == "" { modelID = "rerank-english-v3.0" }
          model, err := rprov.RerankModel(context.Background(), modelID)
          if err != nil { return nil, fmt.Errorf("create rerank model: %w", err) }
          return NewPantheonReranker(model), nil
      default:
          return nil, fmt.Errorf("unknown rerank provider: %s", name)
      }
  }

  func pickStr(settings map[string]string, key, fallback string) string {
      if v, ok := settings[key]; ok && v != "" { return v }
      return fallback
  }
  ```

- [ ] 写 10 个测试。其中 `TestPantheonReranker_Rerank_ReordersDocs` 用 httptest mock cohere 端点:
  ```go
  func TestPantheonReranker_Rerank_ReordersDocs(t *testing.T) {
      srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
          // Cohere /v2/rerank response shape
          json.NewEncoder(w).Encode(map[string]any{
              "results": []map[string]any{
                  {"index": 2, "relevance_score": 0.95},
                  {"index": 0, "relevance_score": 0.7},
                  {"index": 1, "relevance_score": 0.3},
              },
          })
      }))
      defer srv.Close()
      // 用 cohere.WithBaseURL 重定向到 mock
      // ... 实现细节
      // 验证返回 indices [2, 0, 1]
  }
  ```

  > **mock 难度**:Cohere SDK 在 pantheon 内部用什么 endpoint path?实测时 `grep -rE "v2/rerank|/rerank" $PANTHEON/providers/cohere/`;httptest 桩按实际 path 暴露。

- [ ] Run all 10 tests; full suite green.

### Acceptance

- Reranker 接口、Noop + Pantheon 两实现就位
- 工厂 4 种输入(空/none/cohere/unknown)行为正确
- Mock 验证 Pantheon reranker 把 doc 顺序按 RelevanceScore 重排
- 缺 key 报清晰错误

### Commit

`feat(reranker): new package with Cohere + Noop implementations`

---

## Task 4: vectordb.SearchOptions.Rerank 下线 + ChatService 接入

**Files:**
- `backend/internal/vectordb/interface.go` (MODIFY — remove `Rerank bool`)
- `backend/internal/vectordb/pgvector.go` (MODIFY — remove `limit *= 3` branch)
- `backend/internal/services/chat_service.go` (MODIFY — inject Reranker; rerank after similarity search)
- `backend/internal/services/vector_search_service.go` (MODIFY — same pattern)
- `backend/internal/services/embed_service.go` (MODIFY — same pattern)
- 所有 test 文件里使用 `SearchOptions{...Rerank: ...}` 字面量的(grep 检查)

**Tests:**
- `TestChatService_BuildRAGContext_RerankerInvoked` (mock reranker counts calls)
- `TestChatService_BuildRAGContext_NoopReranker_PreservesOrder`
- `TestChatService_BuildRAGContext_RerankerError_FallsBackToOriginalOrder`
- `TestVectorSearchService_RerankerInvokedWhenConfigured`
- `TestEmbedService_RerankerInvokedWhenConfigured`

### Steps

- [ ] 删 `SearchOptions.Rerank bool` 字段:
  ```go
  // vectordb/interface.go
  type SearchOptions struct {
      TopN                int
      SimilarityThreshold float64
      // 删除:Rerank bool
  }
  ```

- [ ] 删 pgvector.go 假 rerank 路径:
  ```go
  // 之前
  if opts.Rerank { limit = limit * 3 }
  // 之后
  // (删除)
  ```

- [ ] `ChatService` 注入 Reranker:
  ```go
  type ChatService struct {
      // ... existing ...
      reranker reranker.Reranker
  }

  func NewChatService(db *gorm.DB, cfg *config.Config, vectorSvc *VectorService, llmProv providers.LLMProvider, emb embedder.Embedder, agentInvoker AgentInvoker, rer reranker.Reranker) *ChatService {
      if rer == nil { rer = &reranker.NoopReranker{} }
      return &ChatService{ /*...*/ reranker: rer }
  }
  ```

- [ ] 改 `buildRAGContext`,take fetch-more + rerank topN:
  ```go
  // 现在
  topN := 4
  if ws.TopN != nil { topN = *ws.TopN }
  results, err := s.vectorSvc.SimilaritySearch(ctx, ws.Slug, queryVector, vectordb.SearchOptions{
      TopN:                topN,
      SimilarityThreshold: threshold,
  })

  // 改为
  topN := 4
  if ws.TopN != nil { topN = *ws.TopN }
  // 用 reranker 时 fetch 3× 候选,然后挑 topN
  fetchN := topN
  _, isNoop := s.reranker.(*reranker.NoopReranker)
  if !isNoop { fetchN = topN * 3 }

  results, err := s.vectorSvc.SimilaritySearch(ctx, ws.Slug, queryVector, vectordb.SearchOptions{
      TopN:                fetchN,
      SimilarityThreshold: threshold,
  })
  if err == nil && len(results) > 0 && !isNoop {
      texts := make([]string, len(results))
      for i, r := range results { texts[i] = r.Text }
      reranked, rerr := s.reranker.Rerank(ctx, message, texts, topN)
      if rerr != nil {
          mlog.Warning("rerank failed, falling back: ", rerr)
          // 保留原顺序,只取 topN
          if len(results) > topN { results = results[:topN] }
      } else {
          // 按 reranked.Index 重排
          newResults := make([]vectordb.SearchResult, 0, len(reranked))
          for _, sd := range reranked {
              if sd.Index >= 0 && sd.Index < len(results) {
                  newResults = append(newResults, results[sd.Index])
              }
          }
          results = newResults
      }
  } else if len(results) > topN {
      // Noop path: 取前 topN
      results = results[:topN]
  }
  ```

- [ ] 同款改 `VectorSearchService` 和 `EmbedService`(注入 reranker;buildRAGContext 之外的 SimilaritySearch 也走同款逻辑)。

  > **取舍**:把 rerank 抽出来做成 chat_service 的辅助函数 `(s *ChatService) rerankResults(ctx, query, results, topN)` 共享给 3 个 service?**实施时根据复用频率决定**;如果 3 处用法一致,放 `reranker` 包里写一个 `RerankSearchResults(ctx, r Reranker, query, results []vectordb.SearchResult, topN int) []vectordb.SearchResult` helper 更干净。

- [ ] 改所有 SearchOptions 字面量 callsite:
  ```bash
  grep -rn "Rerank:" backend/internal/ --include="*.go"
  ```
  逐个删除该字段。

- [ ] 写 5 个测试,确保 rerank 路径正确触发。

- [ ] Run full suite; **重点**确认现有 chat / vector search / embed 测试通过(零行为回归 — Noop 路径与之前行为等价)。

### Acceptance

- `SearchOptions.Rerank` 字段消失
- 5 个新测试通过
- 现有所有相关测试通过
- ChatService 在 NoopReranker 模式下行为与之前等价(零回归)
- Cohere reranker 模式下,SearchResult 按 RelevanceScore 重排

### Commit

`feat(reranker): integrate into ChatService/VectorSearch/EmbedService; remove vectordb.SearchOptions.Rerank`

---

## Task 5: main.go 接线 + 文档

**Files:**
- `backend/cmd/server/main.go` (MODIFY)
- `.env.example` (MODIFY — 已在 Task 0)
- `backend/README.md` 或 `docs/PROVIDERS.md` (NEW or MODIFY — list 40 providers)

**Tests:** none new (smoke covered by Task 1-4).

### Steps

- [ ] 在 `main.go` 适当位置(`cfg` 加载后,`chatSvc` 创建前)添加:
  ```go
  // Reranker (sits between vector retrieval and LLM)
  settingsForBoot, _ := sysSvc.GetAllSettings(context.Background())
  rer, rerErr := reranker.NewReranker(cfg, settingsForBoot)
  if rerErr != nil {
      mlog.Warning("reranker init failed (falling back to noop): ", rerErr)
      rer = &reranker.NoopReranker{}
  }
  ```

- [ ] 改 `NewChatService` 调用注入 `rer`:
  ```go
  chatSvc := services.NewChatService(db, cfg, vectorSvc, llmProv, emb, agentRuntime, rer)
  ```

- [ ] 同款给 `VectorSearchService` 和 `EmbedService` 注入。

- [ ] 写 `docs/PROVIDERS.md`(或更新 README):
  ```markdown
  # Supported Providers

  ## LLM (40 providers)
  | Provider | LLM_PROVIDER value | API key env |
  |---|---|---|
  | OpenAI | openai | OPEN_AI_KEY |
  | Anthropic | anthropic | ANTHROPIC_API_KEY |
  | Google Gemini | gemini | GEMINI_API_KEY |
  ... (40 行)

  ## Embedding (3 providers + openai-compat fan-out)
  ...

  ## Reranking (1 provider + noop)
  | Provider | RERANK_PROVIDER | Notes |
  |---|---|---|
  | None (default) | "" or "noop" or "none" | Passthrough, no reordering |
  | Cohere | cohere | Requires RERANK_API_KEY or COHERE_API_KEY |
  ```

- [ ] Boot manual:`go run ./cmd/server` 启动,无错;`curl /api/system/setup-status` 返回 ok。

### Acceptance

- main.go 构造 Reranker 成功(默认 Noop)
- ChatService / VectorSearch / Embed 三个 service 都收到注入
- 文档列 40 个 LLM provider + 3 embedder + 2 reranker
- 完整测试套 100% 通过 + `-race` 干净

### Commit

`feat(server): wire Reranker into ChatService + provider docs`

---

## Post-PR checklist

- [ ] `go build ./...` 干净
- [ ] `go vet ./...` 干净
- [ ] `go test ./... -race` 100% 绿
- [ ] `gofmt -l . | wc -l` 返回 0
- [ ] 2 个 decision artefact 落地
- [ ] 实际注册的 LLM provider 数: 40 (36 现存 + 4 pantheon-only)
- [ ] Embedder: 3 显式 case (openai-compat 默认 + cohere + voyage),其它 fall through 到 openai-compat
- [ ] Reranker: cohere + noop
- [ ] 文档 `docs/PROVIDERS.md` 列全
- [ ] `.env.example` 含新 env

## Risk notes

| Risk | Mitigation |
|---|---|
| pantheon wenxin.New 签名实测与假设不一致 | Task 1 实施时先 grep 验证;留 fallback 注释 |
| Cohere rerank 字段名 `RelevanceScore` vs `Score` | Task 3 grep 实测 `rerank.RerankResult` 字段名 |
| Pantheon 新版本提升其它 provider 的 rerank 能力 | 新 case 加入工厂,无破坏 |
| 现有 SearchOptions.Rerank=true 配置遗留 | 字段移除是源码改动,无 DB 字段;只删字段 + grep 改 callsite,本 PR 内一次性完成 |
| Per-workspace embedder/reranker(目前用全局) | 沿 ChatService 注入路径扩展,留给独立 PR |
| Cohere 国内访问受限 | 文档说明;NoopReranker 是默认 fall-back |
| 测试用 build-only smoke 检测不到真 API 不兼容 | 关键 provider 在 CI 加 `//go:build integration` 真请求测试,需 CI 凭据 |

## Estimate

| Task | Hours |
|---|---|
| 0. Config knobs + decision artefacts | 1.0 |
| 1. 4 个 pantheon-only LLM builders + 5 个测试 | 3.0 |
| 2. Embedder factory (4-case) + 8 个测试 | 4.0 |
| 3. Reranker 包 (interface + Pantheon + Noop + factory) + 10 个测试 | 4.0 |
| 4. SearchOptions.Rerank 下线 + 3 个 service 接入 reranker + 5 个测试 | 4.0 |
| 5. main.go 接线 + 文档 | 2.0 |
| **Total** | **18.0** (design 估 25-35h,但 LLM 拓宽已 36/38 就位,实际剩余 ~18h ✓) |

—— end of plan
