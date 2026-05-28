# AnythingLLM Go Backend — Phase 2 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use gpowers:subagent-driven-development (recommended) or gpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace Phase 1 stubs with real Pantheon LLM integration, vector search + RAG, and Collector document pipeline.

**Architecture:** Three sequential milestones: (1) Collector client with signed payloads, (2) Pantheon embedder + text chunker + LanceDB/PGVector real implementations, (3) Pantheon LLM streaming with RAG context injection. Each milestone produces working, testable software.

**Tech Stack:** Go 1.23, Gin, GORM, Pantheon SDK (`github.com/odysseythink/pantheon`), LanceDB Go (`github.com/lancedb/lancedb-go`), pgvector, mlog

---

## File Structure

NEW:
  pkg/utils/communication_key.go          RSA-2048 signing + encryption for Collector auth
  internal/collector/types.go             Collector request/response DTOs
  internal/collector/client.go            HTTP client with X-Integrity / X-Payload-Signer headers
  internal/embedder/pantheon.go           Pantheon embed.EmbeddingModel wrapper
  internal/chunker/chunker.go             Text splitting with overlap

MODIFIED:
  pkg/utils/encryption_manager.go         + AES-256-CBC, XPayload(), SIG_KEY/SIG_SALT
  internal/config/config.go               + LLM, Embedding, Collector env vars
  internal/vectordb/interface.go          + SearchOptions, DeleteNamespace
  internal/vectordb/lancedb.go            + real Connect/AddVectors/SimilaritySearch/DeleteVectors
  internal/vectordb/pgvector.go           + real table mgmt, insert, search, delete
  internal/providers/llm.go               + PantheonProvider with core.LanguageModel.Stream()
  internal/services/document_service.go   + Collector pipeline, chunk+embed+store flow
  internal/services/chat_service.go       + RAG retrieval, real LLM streaming
  cmd/server/main.go                      + wire new services
  go.mod                                  + Pantheon SDK, lancedb-go

TESTS:
  tests/integration/collector_test.go     Collector client round-trip with mock server
  tests/integration/chunker_test.go       Chunk boundary + overlap validation
  tests/integration/vector_test.go        LanceDB/PGVector add + search round-trip
  tests/integration/chat_test.go          Chat streaming with mock LLM + mock vector search

---

## Milestone 1: Collector Client

### Task 1: CommunicationKey + EncryptionManager Extension

Files:
- Create: pkg/utils/communication_key.go
- Modify: pkg/utils/encryption_manager.go

Context: The Node Collector requires two auth headers on every request:
- X-Integrity: RSA-SHA256 signature of the JSON body (hex)
- X-Payload-Signer: RSA encryption of the AES key (base64)

Reference (Node):
- server/utils/comKey/index.js — RSA-2048 PKCS#1, private key signing + encryption
- server/utils/EncryptionManager/index.js — AES-256-CBC via scrypt(SIG_KEY, SIG_SALT)

- [ ] Step 1: Write CommunicationKey

Create pkg/utils/communication_key.go:
- NewCommunicationKey(storageDir string) — generates or loads RSA-2048 PKCS#1 key pair from storageDir/comkey/ipc-priv.pem and ipc-pub.pem
- Sign(data string) string — RSA-SHA256 sign, hex encode
- Encrypt(data string) string — RSA private-key encrypt (PKCS#1 v1.5), base64 encode. In Go, use rsa.SignPKCS1v15(rand.Reader, c.privateKey, crypto.Hash(0), []byte(data))

- [ ] Step 2: Extend EncryptionManager with AES-CBC + XPayload

Modify pkg/utils/encryption_manager.go:
- Add imports: bytes, strings, golang.org/x/crypto/scrypt
- deriveCBCKey() []byte — reads SIG_KEY/SIG_SALT env (auto-generate 32-byte hex if missing), derives 32-byte key via scrypt.Key(key, salt, 32768, 8, 1, 32)
- XPayload() string — returns base64(deriveCBCKey())
- EncryptCBC(plaintext string) (string, error) — AES-256-CBC with PKCS#7 padding, format hex(ciphertext):hex(iv)
- DecryptCBC(ciphertext string) (string, error) — reverse of above

- [ ] Step 3: Verify compilation

Run: cd backend && go get golang.org/x/crypto/scrypt && go build ./pkg/utils/
Expected: PASS

- [ ] Step 4: Commit

git add backend/pkg/utils/communication_key.go backend/pkg/utils/encryption_manager.go backend/go.mod backend/go.sum
git commit -m "feat: add CommunicationKey and Node-compatible EncryptionManager CBC mode"

---

### Task 2: Collector Types + Client

Files:
- Create: internal/collector/types.go
- Create: internal/collector/client.go

- [ ] Step 1: Write internal/collector/types.go

Define DTOs matching Collector JSON responses:
- Options, OCROptions, RuntimeSettings
- ProcessResponse with Filename, Success, Reason, Documents []Document
- Document with all fields from Collector output (Location, Name, URL, Title, DocAuthor, Description, DocSource, ChunkSource, Published, WordCount, TokenCountEstimate)
- LinkContentResponse, ExtensionResponse, ParseOptions

- [ ] Step 2: Write internal/collector/client.go

Define Client struct with:
- NewClient(storageDir string) — creates CommunicationKey, EncryptionManager, sets endpoint to http://0.0.0.0:$COLLECTOR_PORT
- Online(ctx) bool — GET /, check 200
- AcceptedFileTypes(ctx) — GET /accepts
- ProcessDocument, ProcessLink, ProcessRawText, ParseDocument, GetLinkContent — POST with signAndPost
- ForwardExtensionRequest — raw body string, custom method, 15-min timeout

Internal signAndPost helper:
- Marshals body to JSON
- Sets Content-Type: application/json
- Sets X-Integrity: comKey.Sign(jsonString) (hex)
- Sets X-Payload-Signer: comKey.Encrypt(encMgr.XPayload()) (base64)
- Uses 10-min timeout for process/parse, 15-min for extensions

- [ ] Step 3: Verify compilation

Run: cd backend && go build ./internal/collector/
Expected: PASS

- [ ] Step 4: Commit

git add backend/internal/collector/
git commit -m "feat: add Collector client with signed payload headers"

---

### Task 3: Document Service Integration

Files:
- Modify: internal/services/document_service.go
- Modify: internal/handlers/document.go
- Modify: cmd/server/main.go

Context: Currently DocumentService only handles multipart upload and DB storage. Wire the Collector so upload -> save file -> Collector process -> store metadata.

- [ ] Step 1: Update DocumentService constructor and Upload handler

Modify internal/services/document_service.go:
- Add collector.Client field to DocumentService
- Update NewDocumentService to accept *collector.Client
- Add ProcessUpload(ctx, filename, data, workspaceSlug, metadata) method:
  1. Save file to storageDir/documents/filename
  2. Call collector.ProcessDocument(ctx, filename, metadata)
  3. Store first document result metadata in WorkspaceDocument record
  4. Return the created WorkspaceDocument

- [ ] Step 2: Update handler wiring

Modify internal/handlers/document.go:
- Update upload handler to call docSvc.ProcessUpload after receiving multipart file

Modify cmd/server/main.go:
- Create collector.Client and pass to NewDocumentService

- [ ] Step 3: Verify compilation

Run: cd backend && go build ./cmd/server/
Expected: PASS

- [ ] Step 4: Commit

git add backend/internal/services/document_service.go backend/internal/handlers/document.go backend/cmd/server/main.go
git commit -m "feat: wire Collector into document upload pipeline"

---

### Task 4: Collector Integration Tests

Files:
- Create: tests/integration/collector_test.go

- [ ] Step 1: Write mock Collector server test

Create tests/integration/collector_test.go:
- Start an httptest.Server on a random port that mimics Collector endpoints (/accepts, /process)
- Verify X-Integrity and X-Payload-Signer headers are present and well-formed
- Verify ProcessDocument returns correct ProcessResponse
- Use temp storage dir for CommunicationKey + EncryptionManager

- [ ] Step 2: Run tests

Run: cd backend && go test ./tests/integration/ -run TestCollector -v
Expected: PASS

- [ ] Step 3: Commit

git add backend/tests/integration/collector_test.go
git commit -m "test: add Collector client integration tests"

---

## Milestone 2: Embedding & Vector Search

### Task 5: Config + Dependencies

Files:
- Modify: internal/config/config.go
- Modify: go.mod

- [ ] Step 1: Add new env vars to Config

Modify internal/config/config.go, add fields:
- LLMProvider string env:"LLM_PROVIDER" envDefault:"openai"
- LLMModel string env:"LLM_MODEL" envDefault:"gpt-4o"
- LLMApiKey string env:"LLM_API_KEY"
- LLMTemperature float64 env:"LLM_TEMPERATURE"
- LLMMaxTokens *int env:"LLM_MAX_TOKENS"
- EmbeddingProvider string env:"EMBEDDING_PROVIDER" envDefault:"openai"
- EmbeddingModel string env:"EMBEDDING_MODEL" envDefault:"text-embedding-3-small"
- EmbeddingApiKey string env:"EMBEDDING_API_KEY"
- CollectorPort string env:"COLLECTOR_PORT" envDefault:"8888"

- [ ] Step 2: Add go.mod dependencies

Run: cd backend && go get github.com/odysseythink/pantheon
Run: cd backend && go get github.com/lancedb/lancedb-go
Add replace directive in go.mod:
replace github.com/odysseythink/pantheon => D:/workspace/go_work/pantheon

- [ ] Step 3: Verify compilation

Run: cd backend && go mod tidy && go build ./...
Expected: PASS

- [ ] Step 4: Commit

git add backend/internal/config/config.go backend/go.mod backend/go.sum
git commit -m "feat: add LLM/embedding/collector config and Pantheon SDK dependency"

---

### Task 6: Pantheon Embedder

Files:
- Create: internal/embedder/pantheon.go

- [ ] Step 1: Write PantheonEmbedder

Create internal/embedder/pantheon.go:
- Define Embedder interface: EmbedTexts(ctx, texts []string) ([][]float32, error), EmbedQuery(ctx, query string) ([]float32, error), Dimensions() int
- PantheonEmbedder struct holds embed.EmbeddingModel
- NewPantheonEmbedder(cfg) — creates provider via openai.New(cfg.EmbeddingApiKey) or other provider, gets EmbeddingModel
- EmbedTexts calls model.Embed(ctx, texts), returns embeddings
- EmbedQuery wraps single text in slice, returns first result
- Dimensions cached from first call

- [ ] Step 2: Verify compilation

Run: cd backend && go build ./internal/embedder/
Expected: PASS

- [ ] Step 3: Commit

git add backend/internal/embedder/
git commit -m "feat: add Pantheon embedder wrapper"

---

### Task 7: Text Chunker

Files:
- Create: internal/chunker/chunker.go
- Create: tests/integration/chunker_test.go

- [ ] Step 1: Write Chunker

Create internal/chunker/chunker.go:
- Chunker struct with ChunkSize, ChunkOverlap, ChunkPrefix
- NewChunker(chunkSize, overlap int, prefix string) *Chunker
- Split(text string) []string — splits on paragraphs first, then sentences, then words while maintaining overlap
- Default chunk size from system setting text_splitter_chunk_size, overlap from text_splitter_chunk_overlap (default 20)
- Prefix prepended to each chunk if set

- [ ] Step 2: Write chunker tests

Create tests/integration/chunker_test.go:
- Test long text splits into multiple chunks
- Test overlap between consecutive chunks
- Test prefix prepended

- [ ] Step 3: Run tests

Run: cd backend && go test ./tests/integration/ -run TestChunker -v
Expected: PASS

- [ ] Step 4: Commit

git add backend/internal/chunker/ backend/tests/integration/chunker_test.go
git commit -m "feat: add text chunker with overlap"

---

### Task 8: VectorDatabase Interface Update

Files:
- Modify: internal/vectordb/interface.go

- [ ] Step 1: Update interface

Modify internal/vectordb/interface.go:
- Add SearchOptions struct: SimilarityThreshold float64, TopN int, FilterIdentifiers []string, Rerank bool
- Add DeleteNamespace(ctx, namespace string) error to VectorDatabase interface
- Update SimilaritySearch signature to: SimilaritySearch(ctx, namespace string, queryVector []float32, opts SearchOptions) ([]SearchResult, error)

- [ ] Step 2: Verify compilation

Run: cd backend && go build ./internal/vectordb/
Expected: PASS (stubs will need temporary placeholder updates)

- [ ] Step 3: Commit

git add backend/internal/vectordb/interface.go
git commit -m "feat: extend VectorDatabase interface with SearchOptions and DeleteNamespace"

---

### Task 9: LanceDB Real Implementation

Files:
- Modify: internal/vectordb/lancedb.go

- [ ] Step 1: Implement real LanceDB

Modify internal/vectordb/lancedb.go:
- Connect: use lancedb.Connect(uri)
- AddVectors: open or create table by namespace, add records {id, vector, text, metadata...}
- SimilaritySearch: table.Search(queryVector).DistanceType("cosine").Limit(opts.TopN).Execute(), filter by similarity threshold
- DeleteVectors: use document_vectors table to map docId -> vectorIds, then table.Delete("id IN (...)")
- DeleteNamespace: client.DropTable(namespace)
- TotalVectors: iterate all tables, sum countRows

- [ ] Step 2: Verify compilation

Run: cd backend && go build ./internal/vectordb/
Expected: PASS

- [ ] Step 3: Commit

git add backend/internal/vectordb/lancedb.go
git commit -m "feat: implement real LanceDB vector operations"

---

### Task 10: PGVector Real Implementation

Files:
- Modify: internal/vectordb/pgvector.go

- [ ] Step 1: Implement real PGVector

Modify internal/vectordb/pgvector.go:
- Connect: ensure CREATE EXTENSION IF NOT EXISTS vector, create table if missing
- Table schema: anythingllm_vectors (or PGVECTOR_TABLE_NAME env) with id UUID PRIMARY KEY, namespace TEXT, embedding vector(N), metadata JSONB, created_at TIMESTAMP
- AddVectors: BEGIN transaction, INSERT for each chunk, COMMIT
- SimilaritySearch: SELECT embedding <=> $1 AS _distance, metadata FROM ... WHERE namespace = $2 ORDER BY _distance ASC LIMIT $3, filter by threshold
- DeleteVectors: DELETE WHERE id IN (vectorIds from document_vectors mapping)
- DeleteNamespace: DELETE WHERE namespace = $1
- TotalVectors: SELECT COUNT(id)
- Sanitize metadata for JSONB (strip NUL and control chars)

- [ ] Step 2: Verify compilation

Run: cd backend && go build ./internal/vectordb/
Expected: PASS

- [ ] Step 3: Commit

git add backend/internal/vectordb/pgvector.go
git commit -m "feat: implement real PGVector vector operations"

---

### Task 11: Document Embedding Pipeline

Files:
- Modify: internal/services/document_service.go
- Modify: internal/services/vector_service.go (if exists, or create)

Context: After Collector processes a document, we need to chunk, embed, and store vectors.

- [ ] Step 1: Add embedding pipeline to DocumentService

Modify internal/services/document_service.go:
- Add embedder.Embedder and vectordb.VectorDatabase fields
- Add EmbedDocument(ctx, workspaceSlug, docId, pageContent, metadata) error:
  1. Chunk pageContent using chunker.Split
  2. Embed chunks using embedder.EmbedTexts
  3. Build []vectordb.VectorChunk with UUID IDs
  4. Call vectorDB.AddVectors(ctx, workspaceSlug, chunks)
  5. Bulk insert document_vectors records mapping docId -> vectorId

- [ ] Step 2: Wire in main.go

Modify cmd/server/main.go:
- Create embedder, chunker
- Pass to DocumentService

- [ ] Step 3: Verify compilation

Run: cd backend && go build ./cmd/server/
Expected: PASS

- [ ] Step 4: Commit

git add backend/internal/services/document_service.go backend/cmd/server/main.go
git commit -m "feat: wire document chunking, embedding, and vector storage pipeline"

---

### Task 12: Vector DB Integration Tests

Files:
- Create: tests/integration/vector_test.go

- [ ] Step 1: Write vector round-trip tests

Create tests/integration/vector_test.go:
- TestLanceDBAddAndSearch: connect to temp dir, add vectors, search, verify results
- TestPGVectorAddAndSearch: connect to temp PostgreSQL (or skip if not available), add vectors, search
- TestDeleteVectors: add vectors, delete by docId, verify search returns empty

- [ ] Step 2: Run tests

Run: cd backend && go test ./tests/integration/ -run TestVector -v
Expected: PASS (PGVector tests may be skipped if no Postgres)

- [ ] Step 3: Commit

git add backend/tests/integration/vector_test.go
git commit -m "test: add vector database integration tests"

---

## Milestone 3: LLM Integration

### Task 13: Pantheon LLM Provider

Files:
- Modify: internal/providers/llm.go

- [ ] Step 1: Replace stub with PantheonProvider

Modify internal/providers/llm.go:
- Define LLMProvider interface: Stream(ctx, messages []core.Message, systemPrompt string) (<-chan LLMChunk, error)
- Define LLMChunk struct: TextDelta, ReasoningDelta, Usage, FinishReason, Err
- PantheonProvider struct holds core.LanguageModel
- NewPantheonProvider(cfg) — creates provider (openai.New, anthropic.New, etc.), gets LanguageModel
- Stream method:
  1. Build core.Request with Messages, SystemPrompt, Temperature, MaxTokens
  2. Call model.Stream(ctx, req) -> iter.Seq2[*StreamPart, error]
  3. Consume iterator in goroutine, map each StreamPart to LLMChunk on output channel

- [ ] Step 2: Verify compilation

Run: cd backend && go build ./internal/providers/
Expected: PASS

- [ ] Step 3: Commit

git add backend/internal/providers/llm.go
git commit -m "feat: implement Pantheon LLM provider with streaming"

---

### Task 14: Chat Service RAG + Real LLM

Files:
- Modify: internal/services/chat_service.go

- [ ] Step 1: Add RAG retrieval and real LLM streaming

Modify internal/services/chat_service.go:
- Add embedder.Embedder field
- Update Stream() method:
  1. Build history (existing logic)
  2. RAG retrieval:
     - Embed query: embedder.EmbedQuery(ctx, req.Message)
     - vectorDB.SimilaritySearch(ctx, ws.Slug, queryVector, SearchOptions{TopN: 4, SimilarityThreshold: 0.25})
     - Format RAG context string from search results
  3. Compose system prompt = ws.OpenAiPrompt + optional RAG context
  4. Call llm.Stream(ctx, history, systemPrompt) -> <-chan LLMChunk
  5. Map LLMChunk.TextDelta -> dto.StreamChatResponse{Type: "textResponseChunk", TextResponse: ...}
  6. On FinishReason, send Close: true and save to DB
  7. On Err, send abort event

- [ ] Step 2: Verify compilation

Run: cd backend && go build ./internal/services/
Expected: PASS

- [ ] Step 3: Commit

git add backend/internal/services/chat_service.go
git commit -m "feat: integrate RAG retrieval and real Pantheon LLM streaming into chat"

---

### Task 15: Main Entry Wiring

Files:
- Modify: cmd/server/main.go

- [ ] Step 1: Wire all new services

Modify cmd/server/main.go:
- Create collector.Client
- Create embedder.PantheonEmbedder
- Create chunker.Chunker
- Pass embedder + chunker + vectorDB to DocumentService
- Pass embedder to ChatService
- Create Pantheon LLM provider, pass to ChatService
- Update RegisterDocumentRoutes and RegisterChatRoutes signatures if needed

- [ ] Step 2: Verify compilation

Run: cd backend && go build ./cmd/server/
Expected: PASS

- [ ] Step 3: Commit

git add backend/cmd/server/main.go
git commit -m "feat: wire all Phase 2 services in main entry point"

---

### Task 16: Chat Integration Tests

Files:
- Create: tests/integration/chat_test.go

- [ ] Step 1: Write chat streaming tests

Create tests/integration/chat_test.go:
- TestChatStreamWithRAG:
  - Mock LLM provider that returns predefined chunks
  - Mock vector search that returns context
  - Verify SSE events contain text deltas
  - Verify system prompt in mock LLM contains RAG context
- TestChatStreamWithoutRAG:
  - Mock LLM with empty vector search
  - Verify clean system prompt
- TestChatStreamAbortOnError:
  - Mock LLM returns error
  - Verify SSE abort event

- [ ] Step 2: Run all integration tests

Run: cd backend && go test ./tests/integration/... -v
Expected: All PASS

- [ ] Step 3: Commit

git add backend/tests/integration/chat_test.go
git commit -m "test: add chat streaming integration tests with RAG and mock LLM"

---

## Self-Review Checklist

- [ ] Spec coverage: Every design doc section has at least one task. No gaps.
- [ ] Placeholder scan: No TBD, TODO, or "implement later" steps.
- [ ] Type consistency: VectorChunk, SearchResult, SearchOptions used consistently across all vector DB tasks. LLMChunk used consistently in chat tasks.
- [ ] Dependency order: M1 tasks before M2, M2 before M3. Each milestone ends with tests.
