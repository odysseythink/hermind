import os

plan_path = "d:/workspace/go_work/go-anything-llm/.gpowers/plans/2026-05-22-anythingllm-go-backend-phase2.md"

# Part 1: Header + Milestone 1 Tasks 1-2
part1 = """# AnythingLLM Go Backend — Phase 2 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use gpowers:subagent-driven-development (recommended) or gpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace Phase 1 stubs with real Pantheon LLM integration, vector search + RAG, and Collector document pipeline.

**Architecture:** Three sequential milestones: (1) Collector client with signed payloads, (2) Pantheon embedder + text chunker + LanceDB/PGVector real implementations, (3) Pantheon LLM streaming with RAG context injection. Each milestone produces working, testable software.

**Tech Stack:** Go 1.23, Gin, GORM, Pantheon SDK (`github.com/odysseythink/pantheon`), LanceDB Go (`github.com/lancedb/lancedb-go`), pgvector, mlog

---

## File Structure

```
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
```

---

## Milestone 1: Collector Client

### Task 1: CommunicationKey + EncryptionManager Extension

**Files:**
- Create: `pkg/utils/communication_key.go`
- Modify: `pkg/utils/encryption_manager.go`

**Context:** The Node Collector requires two auth headers on every request:
- `X-Integrity`: RSA-SHA256 signature of the JSON body (hex)
- `X-Payload-Signer`: RSA encryption of the AES key (base64)

**Reference (Node):**
- `server/utils/comKey/index.js` — RSA-2048 PKCS#1, private key signing + encryption
- `server/utils/EncryptionManager/index.js` — AES-256-CBC via scrypt(SIG_KEY, SIG_SALT)

- [ ] **Step 1: Write `CommunicationKey`**

Create `pkg/utils/communication_key.go` with:
- `NewCommunicationKey(storageDir string)` — generates or loads RSA-2048 PKCS#1 key pair from `storageDir/comkey/ipc-priv.pem` and `ipc-pub.pem`
- `Sign(data string) string` — RSA-SHA256 sign, hex encode
- `Encrypt(data string) string` — RSA private-key encrypt (PKCS#1 v1.5), base64 encode. In Go, use `rsa.SignPKCS1v15(rand.Reader, c.privateKey, crypto.Hash(0), []byte(data))`

- [ ] **Step 2: Extend `EncryptionManager` with AES-CBC + XPayload**

Modify `pkg/utils/encryption_manager.go`:
- Add imports: `bytes`, `strings`, `golang.org/x/crypto/scrypt`
- `deriveCBCKey() []byte` — reads `SIG_KEY`/`SIG_SALT` env (auto-generate 32-byte hex if missing), derives 32-byte key via `scrypt.Key(key, salt, 32768, 8, 1, 32)`
- `XPayload() string` — returns `base64(deriveCBCKey())`
- `EncryptCBC(plaintext string) (string, error)` — AES-256-CBC with PKCS#7 padding, format `hex(ciphertext):hex(iv)`
- `DecryptCBC(ciphertext string) (string, error)` — reverse of above

- [ ] **Step 3: Verify compilation**

Run: `cd backend && go get golang.org/x/crypto/scrypt && go build ./pkg/utils/`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add backend/pkg/utils/communication_key.go backend/pkg/utils/encryption_manager.go backend/go.mod backend/go.sum
git commit -m "feat: add CommunicationKey and Node-compatible EncryptionManager CBC mode"
```

---

### Task 2: Collector Types + Client

**Files:**
- Create: `internal/collector/types.go`
- Create: `internal/collector/client.go`

- [ ] **Step 1: Write `internal/collector/types.go`**

Define DTOs matching Collector JSON responses:
- `Options`, `OCROptions`, `RuntimeSettings`
- `ProcessResponse` with `Filename`, `Success`, `Reason`, `Documents []Document`
- `Document` with all fields from Collector output (Location, Name, URL, Title, DocAuthor, Description, DocSource, ChunkSource, Published, WordCount, TokenCountEstimate)
- `LinkContentResponse`, `ExtensionResponse`, `ParseOptions`

- [ ] **Step 2: Write `internal/collector/client.go`**

Define `Client` struct with:
- `NewClient(storageDir string)` — creates `CommunicationKey`, `EncryptionManager`, sets endpoint to `http://0.0.0.0:$COLLECTOR_PORT`
- `Online(ctx) bool` — GET `/`, check 200
- `AcceptedFileTypes(ctx)` — GET `/accepts`
- `ProcessDocument`, `ProcessLink`, `ProcessRawText`, `ParseDocument`, `GetLinkContent` — POST with `signAndPost`
- `ForwardExtensionRequest` — raw body string, custom method, 15-min timeout

Internal `signAndPost` helper:
- Marshals body to JSON
- Sets `Content-Type: application/json`
- Sets `X-Integrity: comKey.Sign(jsonString)` (hex)
- Sets `X-Payload-Signer: comKey.Encrypt(encMgr.XPayload())` (base64)
- Uses 10-min timeout for process/parse, 15-min for extensions

- [ ] **Step 3: Verify compilation**

Run: `cd backend && go build ./internal/collector/`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add backend/internal/collector/
git commit -m "feat: add Collector client with signed payload headers"
```
"""

with open(plan_path, 'w', encoding='utf-8') as f:
    f.write(part1)
print('Part 1 done')
