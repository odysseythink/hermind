# AnythingLLM Go Backend — Phase 2 Design

**Date:** 2026-05-22
**Topic:** Pantheon SDK Integration, Vector Search + RAG, Collector Pipeline
**Depends on:** Phase 1 MVP (completed)

---

## 1. Goal

Replace Phase 1 stubs with real implementations:

1. **Collector Client** — HTTP client to the Node Collector (port 8888) with signed payloads
2. **Embedding + Vector Search** — Pantheon embedder, text chunking, LanceDB/PGVector real implementations
3. **LLM Integration** — Pantheon `core.LanguageModel.Stream()` replacing simulated SSE

All changes must preserve 100% API compatibility with the existing React frontend.

---

## 2. Architecture & Data Flow

```
┌─────────────────────────────────────────────────────────────────────┐
│                        AnythingLLM Go Backend                        │
├─────────────────────────────────────────────────────────────────────┤
│  API Layer          │  /document/upload → save to hotdir            │
│  (Gin handlers)     │  /workspace/:slug/stream-chat → SSE stream    │
│                     │  /workspace/:slug/update-embeddings           │
├─────────────────────────────────────────────────────────────────────┤
│  Service Layer      │  DocumentService ──► CollectorClient          │
│                     │       │                    │                   │
│                     │       ▼                    ▼                   │
│                     │  [save file]        [process/parse]            │
│                     │       │                    │                   │
│                     │       ▼                    ▼                   │
│                     │  WorkspaceDocument  [pageContent + metadata]   │
│                     │       │                    │                   │
│                     │       ▼                    ▼                   │
│                     │  Chunker ──► Embedder ──► VectorDB            │
│                     │  (text split)  (Pantheon)  (LanceDB/PGVector) │
│                     │       ▲                    ▲                   │
│                     │       │         SimilaritySearch(queryVector)  │
│                     │       │                    │                   │
│                     │  ChatService ◄─────────────┘                   │
│                     │       │                                        │
│                     │       ▼                                        │
│                     │  PantheonLLM.Stream(ctx, messages, systemPrompt)│
│                     │       │                                        │
│                     │       ▼                                        │
│                     │  SSE (text_delta, finish)                      │
└─────────────────────────────────────────────────────────────────────┘
```

---

## 3. Milestones

| Milestone | Deliverable | Unlocks |
|-----------|-------------|---------|
| **M1** | Collector Client + `CommunicationKey` + Node-compatible `EncryptionManager` | Document upload pipeline works end-to-end |
| **M2** | Pantheon Embedder, Text Chunker, LanceDB/PGVector real implementations | Documents become searchable vectors |
| **M3** | Pantheon LLM `Stream()`, RAG context injection in chat | Real AI responses with retrieved context |

---

## 4. Milestone 1 — Collector Client

### 4.1 Components

| File | Purpose |
|------|---------|
| `pkg/utils/communication_key.go` | RSA-2048 key pair (PKCS#1) in `storage/comkey/`. `Sign(data string) string` (RSA-SHA256 → hex), `Encrypt(data string) string` (RSA privateEncrypt → base64). Auto-generates on first use. |
| `pkg/utils/encryption_manager.go` *(extend)* | Add Node-compatible AES-256-CBC mode: `SIG_KEY`/`SIG_SALT` env vars (auto-generate if missing), scrypt-derived key, `EncryptCBC`/`DecryptCBC`. Add `XPayload()` = base64(aesKey). |
| `internal/collector/client.go` | HTTP client wrapping Collector endpoints |
| `internal/collector/types.go` | Request/response DTOs matching Collector JSON |

### 4.2 `CollectorClient` Interface

```go
type Client interface {
    Online(ctx context.Context) bool
    AcceptedFileTypes(ctx context.Context) ([]string, error)
    ProcessDocument(ctx context.Context, filename string, metadata map[string]string) (*ProcessResponse, error)
    ProcessLink(ctx context.Context, link string, scraperHeaders, metadata map[string]string) (*ProcessResponse, error)
    ProcessRawText(ctx context.Context, textContent string, metadata map[string]string) (*ProcessResponse, error)
    ParseDocument(ctx context.Context, filename string, opts ParseOptions) (*ProcessResponse, error)
    GetLinkContent(ctx context.Context, link string, captureAs string) (*LinkContentResponse, error)
    ForwardExtensionRequest(ctx context.Context, endpoint, method, body string) (*ExtensionResponse, error)
}
```

### 4.3 HTTP Details

- Base URL: `http://0.0.0.0:8888` (from `COLLECTOR_PORT` env, default 8888)
- Headers on every mutating request:
  - `Content-Type: application/json`
  - `X-Integrity`: `comKey.Sign(jsonBody)` (hex)
  - `X-Payload-Signer`: `comKey.Encrypt(encMgr.XPayload())` (base64)
- Timeouts: 10 min for `/process`, `/parse`; 15 min for extension forwarding

### 4.4 Response Types

```go
type ProcessResponse struct {
    Filename  string     `json:"filename"`
    Success   bool       `json:"success"`
    Reason    string     `json:"reason"`
    Documents []Document `json:"documents"`
}

type Document struct {
    Location           string `json:"location"`
    Name               string `json:"name"`
    URL                string `json:"url"`
    Title              string `json:"title"`
    DocAuthor          string `json:"docAuthor"`
    Description        string `json:"description"`
    DocSource          string `json:"docSource"`
    ChunkSource        string `json:"chunkSource"`
    Published          string `json:"published"`
    WordCount          int    `json:"wordCount"`
    TokenCountEstimate int    `json:"token_count_estimate"`
}
```

### 4.5 Handler Wiring

- `POST /api/document/upload` — save multipart file to `storage/documents/`, call `ProcessDocument`, store `WorkspaceDocument` record with metadata JSON
- `POST /api/document/link` — call `ProcessLink`
- `POST /api/document/raw-text` — call `ProcessRawText`

---

## 5. Milestone 2 — Embedding & Vector Search

### 5.1 Components

| File | Purpose |
|------|---------|
| `internal/embedder/pantheon.go` | Wraps Pantheon `embed.EmbeddingModel` |
| `internal/chunker/chunker.go` | Text splitting with configurable chunk size/overlap |
| `internal/vectordb/lancedb.go` *(replace stub)* | Real LanceDB via `github.com/lancedb/lancedb-go` |
| `internal/vectordb/pgvector.go` *(replace stub)* | Real PGVector using `pgvector` + raw SQL on `*pgxpool.Pool` |

### 5.2 Pantheon Embedder

```go
type Embedder interface {
    EmbedTexts(ctx context.Context, texts []string) ([][]float32, error)
    EmbedQuery(ctx context.Context, query string) ([]float32, error)
    Dimensions() int
}

type PantheonEmbedder struct {
    model embed.EmbeddingModel
}
```

- Config: `EMBEDDING_PROVIDER`, `EMBEDDING_MODEL`, `EMBEDDING_API_KEY`
- `EmbedTexts` calls `model.Embed(ctx, texts)` → `EmbeddingResponse.Embeddings`
- `Dimensions` returned from first embedding batch (or cached)

### 5.3 Text Chunker

```go
type Chunker struct {
    ChunkSize    int
    ChunkOverlap int
    ChunkPrefix  string
}

func (c *Chunker) Split(text string) []string
```

- Defaults: chunk size from system setting `text_splitter_chunk_size`, overlap from `text_splitter_chunk_overlap` (default 20)
- Strategy: split on paragraphs first, then sentences, then words — maintaining overlap between chunks
- Prefix: prepend `ChunkPrefix` to each chunk

### 5.4 `VectorDatabase` Interface Extension

```go
type SearchOptions struct {
    SimilarityThreshold float64
    TopN                int
    FilterIdentifiers   []string
    Rerank              bool
}

type VectorDatabase interface {
    Name() string
    Connect(ctx context.Context) error
    Heartbeat(ctx context.Context) (map[string]any, error)
    AddVectors(ctx context.Context, namespace string, chunks []VectorChunk) error
    DeleteVectors(ctx context.Context, namespace string, docIds []string) error
    SimilaritySearch(ctx context.Context, namespace string, queryVector []float32, opts SearchOptions) ([]SearchResult, error)
    DeleteNamespace(ctx context.Context, namespace string) error
    Tables(ctx context.Context) ([]string, error)
    TotalVectors(ctx context.Context) (int64, error)
}
```

### 5.5 LanceDB Implementation

Using `github.com/lancedb/lancedb-go`:
- `Connect`: `lancedb.Connect(uri)`
- Namespace = table name
- `AddVectors`: `table.Add(ctx, records)` where record = `{id, vector, text, ...metadata}`
- `SimilaritySearch`: `table.Search(queryVector).DistanceType("cosine").Limit(opts.TopN).Execute()`
- `DeleteVectors`: `table.Delete("id IN (...)", ids)` using `document_vectors` mapping
- `DeleteNamespace`: `client.DropTable(namespace)`

### 5.6 PGVector Implementation

Using raw SQL on existing `*pgxpool.Pool`:
- Schema: `anythingllm_vectors` (or `PGVECTOR_TABLE_NAME`)
  - `id UUID PRIMARY KEY, namespace TEXT, embedding vector(N), metadata JSONB, created_at TIMESTAMP`
- `Connect`: ensure `CREATE EXTENSION IF NOT EXISTS vector`, create table if missing
- `AddVectors`: transaction with `INSERT INTO ... (id, namespace, embedding, metadata)`
- `SimilaritySearch`: `SELECT embedding <=> $1 AS _distance, metadata FROM ... WHERE namespace = $2 ORDER BY _distance ASC LIMIT $3`
- `DeleteVectors`: delete by vector IDs from `document_vectors` mapping

### 5.7 Document Embedding Pipeline

When a document is uploaded/processed:

1. Collector returns `documents[]` with content
2. **Chunker** splits `pageContent` into `textChunks`
3. **Embedder** embeds all chunks: `EmbedTexts(ctx, textChunks)` → `[][]float32`
4. Build `VectorChunk` for each: `{ID: uuid, Vector: embedding, Metadata: {docId, text, title, ...}}`
5. **VectorDB** `AddVectors(ctx, workspaceSlug, chunks)`
6. **GORM** bulk insert into `document_vectors`: `{DocId, VectorId}`

### 5.8 RAG Context Retrieval

In `ChatService.Stream()`:
1. Embed user query: `EmbedQuery(ctx, req.Message)` → `[]float32`
2. `SimilaritySearch(ctx, workspace.Slug, queryVector, opts{TopN: 4, SimilarityThreshold: 0.25})`
3. Inject retrieved texts into system prompt
4. Stream response via Pantheon LLM (Milestone 3)

---

## 6. Milestone 3 — Pantheon LLM Integration

### 6.1 Components

| File | Purpose |
|------|---------|
| `internal/providers/llm.go` *(replace stub)* | `PantheonProvider` implementing `LLMProvider` interface |
| `internal/services/chat_service.go` *(update)* | Wire real LLM + RAG context |

### 6.2 `LLMProvider` Interface

```go
type LLMProvider interface {
    Stream(ctx context.Context, messages []core.Message, systemPrompt string) (<-chan LLMChunk, error)
}

type LLMChunk struct {
    TextDelta      string
    ReasoningDelta string
    Usage          *core.Usage
    FinishReason   string
    Err            error
}
```

### 6.3 `PantheonProvider`

```go
type PantheonProvider struct {
    model core.LanguageModel
}

func NewPantheonProvider(cfg *config.Config) (*PantheonProvider, error) {
    provider, err := openai.New(cfg.LLMApiKey)
    if err != nil { return nil, err }
    model, err := provider.LanguageModel(context.Background(), cfg.LLMModel)
    if err != nil { return nil, err }
    return &PantheonProvider{model: model}, nil
}
```

`Stream` calls `model.Stream(ctx, req)` where `req` is `*core.Request` with `Messages`, `SystemPrompt`, `Temperature`, `MaxTokens`.

The returned `iter.Seq2[*StreamPart, error]` is consumed in a goroutine and mapped to `LLMChunk` on a channel.

### 6.4 Chat Service Integration

In `ChatService.Stream()`:

1. Build history from `workspace_chats`
2. RAG retrieval (if workspace has documents)
3. Compose system prompt = `ws.OpenAiPrompt` + optional RAG context
4. Call `llm.Stream(ctx, history, systemPrompt)`
5. Map `LLMChunk.TextDelta` → `dto.StreamChatResponse{Type: "textResponseChunk", TextResponse: ...}`
6. On `FinishReason`, send `Close: true` and save full response to DB

---

## 7. Configuration Additions

```go
type Config struct {
    // ... existing fields ...

    // LLM
    LLMProvider    string  `env:"LLM_PROVIDER" envDefault:"openai"`
    LLMModel       string  `env:"LLM_MODEL" envDefault:"gpt-4o"`
    LLMApiKey      string  `env:"LLM_API_KEY"`
    LLMTemperature float64 `env:"LLM_TEMPERATURE"`
    LLMMaxTokens   *int    `env:"LLM_MAX_TOKENS"`

    // Embedding
    EmbeddingProvider string `env:"EMBEDDING_PROVIDER" envDefault:"openai"`
    EmbeddingModel    string `env:"EMBEDDING_MODEL" envDefault:"text-embedding-3-small"`
    EmbeddingApiKey   string `env:"EMBEDDING_API_KEY"`

    // Collector
    CollectorPort string `env:"COLLECTOR_PORT" envDefault:"8888"`
}
```

---

## 8. Error Handling

| Failure | Behavior |
|---------|----------|
| Collector offline | Upload returns `503` with `"collector unavailable"` |
| Embedding fails | Document saved but not vectorized; log error; return `success: true, vectorized: false` |
| Vector search fails | Chat proceeds without RAG context; log warning |
| LLM stream error | SSE sends `type: "abort"` with error message; partial response not saved |

---

## 9. Testing Strategy

| Test | Validates |
|------|-----------|
| `TestCollectorClientOnline` | Mock HTTP server on `:8888`, verify headers + signature |
| `TestChunkerSplit` | Chunk boundaries and overlap |
| `TestEmbedderMock` | Mock `embed.EmbeddingModel`, verify output shape |
| `TestVectorDBAddAndSearch` | LanceDB/PGVector round-trip with in-memory/test DB |
| `TestChatStreamWithRAG` | Mock LLM + mock vector search, verify system prompt contains context |
| `TestChatStreamWithoutRAG` | Mock LLM, verify clean system prompt |

---

## 10. Files Changed

```
NEW:
  pkg/utils/communication_key.go
  internal/collector/client.go
  internal/collector/types.go
  internal/embedder/pantheon.go
  internal/chunker/chunker.go

MODIFIED:
  pkg/utils/encryption_manager.go   (+ AES-CBC, XPayload)
  internal/providers/llm.go         (+ Pantheon integration)
  internal/vectordb/lancedb.go      (+ real implementation)
  internal/vectordb/pgvector.go     (+ real implementation)
  internal/vectordb/interface.go    (+ SearchOptions)
  internal/services/chat_service.go  (+ RAG, real LLM)
  internal/services/document_service.go (+ Collector pipeline)
  internal/config/config.go         (+ LLM/Embedding/Collector env)
  tests/integration/*_test.go       (+ new tests)
```

---

## 11. Dependencies

- `github.com/odysseythink/pantheon` — Pantheon SDK (local replace: `D:\workspace\go_work\pantheon`)
- `github.com/lancedb/lancedb-go` — LanceDB Go client
- `github.com/pgvector/pgvector-go` — PGVector Go types (optional, can use raw SQL)

---

## 12. Risks & Mitigations

| Risk | Mitigation |
|------|------------|
| LanceDB Go SDK unstable | Fallback to REST API if SDK issues arise |
| Pantheon SDK `iter.Seq2` requires Go 1.23+ | Already on Go 1.23, confirmed compatible |
| Node `EncryptionManager` AES-CBC vs Go AES-GCM mismatch | Implement CBC mode alongside existing GCM; use CBC only for Collector headers |
| Vector dimension mismatch (PGVector table creation) | Detect on first insert; recreate table if dimensions change |
