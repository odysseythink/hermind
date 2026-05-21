# Design: Embedding Model Frontend-Configurable

## Background

Currently the embedding model name is hardcoded to `"text-embedding-3-small"` in `cli/engine_deps.go:179`. Users cannot change it via `config.yaml` or the web UI, even though the LLM model (`model`) is fully configurable through the same frontend.

Embedding is used by three consumers:
- `HybridRecaller` — vector search leg of hybrid retrieval
- `BoundaryDetector` — topic-shift detection via cosine similarity
- `SkillsRetriever` — semantic skill search

All three gracefully degrade when the provider is not `EmbedCapable`.

## Goal

Make the embedding model name configurable from the frontend, following the same pattern as the LLM `model` field.

## Decision: Option B (top-level `embed_model`)

Placed at the top level of `Config`, symmetric to `model`. Rationale:
- Embedding is created once in `cli/engine_deps.go` and shared across Memory Layer and Skills, not owned by either subsystem.
- Minimal intrusion: 3 code files + 3 test files.
- Future-proof: if per-provider or per-subsystem embedding models are needed later, a top-level default is the natural fallback.

Rejected options:
- **A (`memory_layer.embed_model`)**: embedding is also consumed by Skills, so Memory Layer is not the right owner.
- **C (`providers.<name>.embed_model`)**: over-engineered. Only the primary provider is used for embedding today; adding a field that is silently ignored on non-EmbedCapable providers creates confusion.

## Architecture

```
┌─────────────────┐     ┌─────────────┐     ┌─────────────────────┐
│  Web UI config  │────▶│ config.yaml │────▶│ config.Config       │
│  panel (models) │     │ embed_model │     │ .EmbedModel         │
└─────────────────┘     └─────────────┘     └─────────────────────┘
                                                      │
                                                      ▼
                                        ┌─────────────────────────┐
                                        │ cli/engine_deps.go      │
                                        │  modelName := cfg.      │
                                        │    EmbedModel           │
                                        │  if modelName == "" {  │
                                        │    modelName = default  │
                                        │  }                      │
                                        └─────────────────────────┘
                                                      │
                                                      ▼
                                        ┌─────────────────────────┐
                                        │ embedding.NewProvider   │
                                        │   Embedder(ec, model)   │
                                        └─────────────────────────┘
                                                      │
                          ┌───────────────────────────┼───────────────────────────┐
                          ▼                           ▼                           ▼
                 ┌─────────────────┐        ┌─────────────────┐        ┌─────────────────┐
                 │ HybridRecaller  │        │BoundaryDetector │        │ SkillsRetriever │
                 │  (vector search)│        │ (topic shift)   │        │ (skill search)  │
                 └─────────────────┘        └─────────────────┘        └─────────────────┘
```

## Changes

### 1. `config/config.go`

Add `EmbedModel` to `Config`:

```go
type Config struct {
    Model      string                    `yaml:"model"`
    EmbedModel string                    `yaml:"embed_model,omitempty"` // NEW
    Providers  map[string]ProviderConfig `yaml:"providers"`
    // ...
}
```

Set default in `Default()`:

```go
return &Config{
    Model:      "anthropic/claude-opus-4-6",
    EmbedModel: "text-embedding-3-small", // NEW
    Providers:  map[string]ProviderConfig{},
    // ...
}
```

### 2. `config/descriptor/embed_model.go` (new)

```go
package descriptor

func init() {
    Register(Section{
        Key:     "embed_model",
        Label:   "Embedding model",
        Summary: "Model used for text embeddings (hybrid search, topic shift detection, skill retrieval).",
        GroupID: "models",
        Shape:   ShapeScalar,
        Fields: []FieldSpec{
            {
                Name:    "embed_model",
                Label:   "Embedding model",
                Help:    "Provider-qualified id for the embedding model, e.g. openai/text-embedding-3-small. Only used when the active provider supports embeddings.",
                Kind:    FieldString,
                Default: "text-embedding-3-small",
            },
        },
    })
}
```

### 3. `cli/engine_deps.go`

Replace hardcoded string with config-driven value:

```go
var emb embedding.Embedder
if p != nil {
    if ec, ok := p.(provider.EmbedCapable); ok {
        modelName := app.Config.EmbedModel
        if modelName == "" {
            modelName = "text-embedding-3-small"
        }
        emb = embedding.NewProviderEmbedder(ec, modelName)
    }
    // TODO: pantheon core.LanguageModel does not expose embedding yet;
    // embedding-dependent features are disabled when the provider is not
    // EmbedCapable.
}
```

## Testing

| Test file | What it validates |
|---|---|
| `config/descriptor/embed_model_test.go` (new) | Section registered under key `"embed_model"`, GroupID `"models"`, one field with `Kind == FieldString`, `Default == "text-embedding-3-small"`. |
| `config/config_test.go` (append) | `Default().EmbedModel == "text-embedding-3-small"`. |
| `cli/engine_deps_test.go` (append) | When `Config.EmbedModel` is set to a non-default value, the embedder is created with that model name. |

## Rollback / Degradation

- **Empty config**: if `embed_model` is omitted or blank, fallback to `"text-embedding-3-small"`.
- **Non-EmbedCapable provider**: `EmbedModel` is ignored; embedding features silently degrade to BM25-only and hard-token boundaries. Existing behavior, unchanged.
- **Invalid model name**: provider returns error → embedding call fails → consumers treat as nil vector → graceful degradation. Existing behavior, unchanged.

## Out of Scope

- Independent embedding provider (separate base_url / api_key)
- Per-subsystem embedding model selection
- Embedding model validation or auto-discovery
- Migration of existing vector stores between models (changing model mid-flight is the user's responsibility)
