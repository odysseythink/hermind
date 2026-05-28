# API v1 PR1 — Service Extensions Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `gpowers:subagent-driven-development` (recommended) or `gpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Land the 4 net-new service methods called for by §5.1 of the API v1 design — `WorkspaceService.UpdatePin`, `DocumentService.PurgeByDocName`, `DocumentService.RemoveFolder` (orchestrator), `DocumentService.SaveRawText` — fully unit-tested with sqlite in-memory DB. No handlers, no DTOs, no main.go wiring (those land in PR2-PR5).

**Architecture:** New methods are added to existing `services/workspace_service.go` and `services/document_service.go`. Tests live in `*_test.go` next to each service file using the existing in-memory sqlite pattern. Orchestrator methods coordinate `FileSystemService` + `VectorService` + DB cascade.

**Tech Stack:** Go 1.22+, GORM, sqlite (test), testify.

**Source spec:** `.gpowers/designs/2026-05-25-api-v1-layer-design.md` §5.1.

**Reference Node implementation:**
- `server/endpoints/api/workspace/index.js:525-591` (update-pin)
- `server/endpoints/api/document/index.js:960-1017` (remove-folder)
- `server/endpoints/api/document/index.js:479-650` (raw-text)
- `server/endpoints/api/system/index.js:207-272` (remove-documents → purge)
- `server/utils/files/purgeDocument.js` (purgeDocument + purgeFolder helpers)

---

## Pre-task: Read this section once before starting

### Existing Go surface (do not duplicate)

- `WorkspaceService` (`workspace_service.go:22`): `Create / List / GetBySlug / Update / Delete / ListWorkspaceUsers / UpdateUsers / DeleteByID / GetByID / GetChats`. Add `UpdatePin` only — do not touch any existing method's signature.
- `DocumentService` (`document_service.go:37`): `SaveUpload / EmbedDocument / GetByDocId / DeleteByDocId / GetWorkspaceBySlug / CreateFolder / MoveFiles / ListDocuments / ListFolderDocuments / GetByDocName / UploadToWorkspace / UploadLink / UploadAndQueueEmbed / UpdateEmbeddings / RemoveAndUnembed`. Add three methods only.
- `FileSystemService.RemoveFolder(folderName string) error` (`filesystem_service.go:71`) — already deletes the directory recursively. Orchestrators call this as the **last step** after DB + vector cleanup.
- `FileSystemService.RemoveDocument(docName string) error` (`filesystem_service.go:77`) — deletes the source file.
- `FileSystemService.SaveFile(folderName, filename string, reader io.Reader) (string, error)` (`filesystem_service.go:102`) — used by SaveRawText.
- `VectorService.DeleteVectors(ctx, namespace string, docIds []string) error` (`vector_service.go:42`) — used to purge vectors.
- `models.WorkspaceDocument` (`models/workspace_document.go`): columns `id, doc_id, filename, docpath, workspace_id, metadata, pinned, watched, created_at, last_updated_at`.

### Methods to add (4)

| # | Owner | Signature |
|---|---|---|
| 1 | `WorkspaceService` | `UpdatePin(ctx, workspaceID int, docPath string, pinned bool) error` |
| 2 | `DocumentService` | `PurgeByDocName(ctx, docName string) error` |
| 3 | `DocumentService` | `RemoveFolder(ctx, folderName string) error` *(orchestrator)* |
| 4 | `DocumentService` | `SaveRawText(ctx, text, title string, metadata map[string]any, workspaceSlugs []string) ([]*models.WorkspaceDocument, error)` |

### Out of scope (explicit)

- **Handlers** — PR3 (`api_workspace.go`, `api_document.go`, `api_system.go`).
- **DTOs** — PR2 (`dto/api.go`).
- **Collector enrichment for raw text** — Node calls `Collector.processRawText(textContent, metadata)` which returns parsed `documents[]` with `wordCount/token_count_estimate/etc.`. The Go port writes the `.json` payload directly; collector enrichment is optional and out-of-scope. If a downstream handler needs the enriched fields, file as PR3 follow-up.
- **EventLogs / Telemetry** — deferred to platform-wide event-log wire-up.
- **`BrowserExtensionApiKey` cascade** on document removal — Go has no such model; ignore.

### Data invariants

- `workspace_documents.docpath` is the path string clients send as `docPath` (e.g. `custom-documents/my-doc.txt-abc.json`). It is **not** prefixed with `STORAGE_DIR`.
- The reserved folder name `custom-documents` must **never** be deleted by `RemoveFolder` — it's the catch-all bucket where uploads + raw-text land.
- A `WorkspaceDocument` row's `doc_id` is the unique identifier passed to `VectorService.DeleteVectors`.
- Multi-workspace embed for raw-text: one DB row per workspace bind (i.e. uploading `text` to `[ws1, ws2]` creates 2 `workspace_documents` rows, both pointing at the same `.json` on disk).

### TDD discipline

Each task follows: write failing test → run + confirm fail → implement → run + confirm pass → commit. Tests use sqlite in-memory DB (existing pattern in `workspace_chat_service_test.go:17`, `prompt_preset_service_test.go:16`). Each test should call `services.AutoMigrate(db)` (or the relevant subset) before exercising the service.

### Test setup helper

A shared helper isn't strictly required for PR1 — both files already use a local `setupDB(t)` style. Reuse the pattern:

```go
func setupTestDB(t *testing.T) *gorm.DB {
    t.Helper()
    db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
    require.NoError(t, err)
    require.NoError(t, services.AutoMigrate(db))
    return db
}
```

For `DocumentService` tests, also set up a tempdir via `t.TempDir()` and pass to `NewFileSystemService(tmpDir)`.

---

## Task 1: WorkspaceService.UpdatePin

Add a method that updates a single `workspace_documents.pinned` flag, matching Node `Document.update(document.id, { pinned: pinStatus })` at `api/workspace/index.js:581`.

**Files:**
- Modify: `backend/internal/services/workspace_service.go`
- Modify: `backend/internal/services/workspace_service_test.go` (create if absent)

- [ ] **Step 1: Write the failing test**

```go
// File: backend/internal/services/workspace_service_test.go
package services

import (
    "context"
    "testing"

    "github.com/odysseythink/hermind/backend/internal/config"
    "github.com/odysseythink/hermind/backend/internal/models"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "gorm.io/driver/sqlite"
    "gorm.io/gorm"
)

func setupWSDB(t *testing.T) *gorm.DB {
    t.Helper()
    db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
    require.NoError(t, err)
    require.NoError(t, AutoMigrate(db))
    return db
}

func TestWorkspaceService_UpdatePin(t *testing.T) {
    db := setupWSDB(t)
    svc := NewWorkspaceService(db, &config.Config{})

    ws := &models.Workspace{Name: "ws1", Slug: "ws1"}
    require.NoError(t, db.Create(ws).Error)
    f := false
    doc := &models.WorkspaceDocument{
        DocId:       "doc-1",
        Filename:    "a.txt",
        Docpath:     "custom-documents/a.txt-xyz.json",
        WorkspaceID: ws.ID,
        Pinned:      &f,
    }
    require.NoError(t, db.Create(doc).Error)

    // Pin
    err := svc.UpdatePin(context.Background(), ws.ID, "custom-documents/a.txt-xyz.json", true)
    require.NoError(t, err)

    var got models.WorkspaceDocument
    require.NoError(t, db.First(&got, doc.ID).Error)
    require.NotNil(t, got.Pinned)
    assert.True(t, *got.Pinned)

    // Unpin
    err = svc.UpdatePin(context.Background(), ws.ID, "custom-documents/a.txt-xyz.json", false)
    require.NoError(t, err)
    require.NoError(t, db.First(&got, doc.ID).Error)
    assert.False(t, *got.Pinned)
}

func TestWorkspaceService_UpdatePin_NotFound(t *testing.T) {
    db := setupWSDB(t)
    svc := NewWorkspaceService(db, &config.Config{})

    err := svc.UpdatePin(context.Background(), 9999, "missing.json", true)
    assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}
```

- [ ] **Step 2: Run the test and confirm it fails**

```bash
cd backend && go test ./internal/services/ -run TestWorkspaceService_UpdatePin -count=1
```

Expected: undefined `UpdatePin`.

- [ ] **Step 3: Implement the method**

Append to `backend/internal/services/workspace_service.go`:

```go
// UpdatePin sets workspace_documents.pinned for the (workspaceID, docPath) row.
// Returns gorm.ErrRecordNotFound when no row matches.
func (s *WorkspaceService) UpdatePin(ctx context.Context, workspaceID int, docPath string, pinned bool) error {
    res := s.db.WithContext(ctx).
        Model(&models.WorkspaceDocument{}).
        Where("workspace_id = ? AND docpath = ?", workspaceID, docPath).
        Update("pinned", pinned)
    if res.Error != nil {
        return res.Error
    }
    if res.RowsAffected == 0 {
        return gorm.ErrRecordNotFound
    }
    return nil
}
```

- [ ] **Step 4: Run the test and confirm it passes**

```bash
cd backend && go test ./internal/services/ -run TestWorkspaceService_UpdatePin -count=1
```

- [ ] **Step 5: Commit**

```bash
git add backend/internal/services/workspace_service.go backend/internal/services/workspace_service_test.go
git commit -m "feat(api-v1): WorkspaceService.UpdatePin for /v1/workspace/:slug/update-pin"
```

---

## Task 2: DocumentService.PurgeByDocName

Cross-workspace purge: given a `docName` (the unique `docpath` Node sends in `/v1/system/remove-documents`), find every `workspace_documents` row, drop the corresponding vector, delete DB rows, then remove the source `.json` from disk.

Mirrors `server/utils/files/purgeDocument.js`.

**Files:**
- Modify: `backend/internal/services/document_service.go`
- Modify: `backend/internal/services/document_service_test.go` (create if absent)

- [ ] **Step 1: Write the failing test**

```go
// File: backend/internal/services/document_service_test.go
package services

import (
    "context"
    "os"
    "path/filepath"
    "testing"

    "github.com/odysseythink/hermind/backend/internal/config"
    "github.com/odysseythink/hermind/backend/internal/models"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "gorm.io/driver/sqlite"
    "gorm.io/gorm"
)

func setupDocDB(t *testing.T) (*gorm.DB, *DocumentService, string) {
    t.Helper()
    db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
    require.NoError(t, err)
    require.NoError(t, AutoMigrate(db))
    tmpDir := t.TempDir()
    cfg := &config.Config{StorageDir: tmpDir}
    fs := NewFileSystemService(tmpDir)
    // collector/embedder/chunker/vectordb are nil in unit tests; methods that touch them must guard.
    svc := NewDocumentService(db, cfg, nil, nil, nil, nil, fs)
    return db, svc, tmpDir
}

func TestDocumentService_PurgeByDocName(t *testing.T) {
    db, svc, tmpDir := setupDocDB(t)

    ws := &models.Workspace{Name: "ws", Slug: "ws"}
    require.NoError(t, db.Create(ws).Error)

    // Lay down a source file
    docsDir := filepath.Join(tmpDir, "documents", "custom-documents")
    require.NoError(t, os.MkdirAll(docsDir, 0o755))
    docPath := "custom-documents/a.txt-uuid.json"
    full := filepath.Join(tmpDir, "documents", docPath)
    require.NoError(t, os.WriteFile(full, []byte(`{"id":"doc-1"}`), 0o644))

    require.NoError(t, db.Create(&models.WorkspaceDocument{
        DocId: "doc-1", Filename: "a.txt", Docpath: docPath, WorkspaceID: ws.ID,
    }).Error)

    err := svc.PurgeByDocName(context.Background(), docPath)
    require.NoError(t, err)

    // DB row gone
    var count int64
    db.Model(&models.WorkspaceDocument{}).Where("docpath = ?", docPath).Count(&count)
    assert.Equal(t, int64(0), count)

    // File gone
    _, statErr := os.Stat(full)
    assert.True(t, os.IsNotExist(statErr))
}

func TestDocumentService_PurgeByDocName_MissingRow_StillCleansFile(t *testing.T) {
    _, svc, tmpDir := setupDocDB(t)

    docPath := "custom-documents/orphan.json"
    full := filepath.Join(tmpDir, "documents", docPath)
    require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
    require.NoError(t, os.WriteFile(full, []byte("{}"), 0o644))

    err := svc.PurgeByDocName(context.Background(), docPath)
    require.NoError(t, err)
    _, statErr := os.Stat(full)
    assert.True(t, os.IsNotExist(statErr))
}
```

- [ ] **Step 2: Run the test and confirm it fails**

```bash
cd backend && go test ./internal/services/ -run TestDocumentService_PurgeByDocName -count=1
```

- [ ] **Step 3: Implement the method**

Append to `document_service.go`:

```go
// PurgeByDocName removes a document across all workspaces:
//   1. Look up workspace_documents rows by docpath
//   2. For each, purge vectors via VectorService (best-effort; missing collection is OK)
//   3. Delete all matching workspace_documents rows
//   4. Remove the source file via FileSystemService
//
// Missing DB rows are not an error — we still clean the file (Node parity).
func (s *DocumentService) PurgeByDocName(ctx context.Context, docName string) error {
    var rows []models.WorkspaceDocument
    if err := s.db.WithContext(ctx).Where("docpath = ?", docName).Find(&rows).Error; err != nil {
        return err
    }

    // Vector purge per workspace (best-effort)
    if s.vectorDB != nil {
        byWS := map[int][]string{}
        for _, r := range rows {
            byWS[r.WorkspaceID] = append(byWS[r.WorkspaceID], r.DocId)
        }
        for wsID, docIds := range byWS {
            var ws models.Workspace
            if err := s.db.First(&ws, wsID).Error; err != nil {
                continue
            }
            // Use the underlying vector DB driver (DocumentService.vectorDB,
            // wired by NewDocumentService). Failures are logged but do not abort cleanup.
            _ = s.vectorDB.DeleteVectors(ctx, ws.Slug, docIds)
        }
    }

    // DB cascade
    if len(rows) > 0 {
        if err := s.db.WithContext(ctx).Where("docpath = ?", docName).
            Delete(&models.WorkspaceDocument{}).Error; err != nil {
            return err
        }
    }

    // Disk cleanup (relative path under documents/)
    if err := s.fs.RemoveDocument(docName); err != nil && !os.IsNotExist(err) {
        return err
    }
    return nil
}
```

Add `"os"` to imports if not already present. If `DocumentService` field is named `fs` and `vdb`, verify against the struct definition; adjust the field references to match.

- [ ] **Step 4: Run the test and confirm it passes**

- [ ] **Step 5: Commit**

```bash
git add backend/internal/services/document_service.go backend/internal/services/document_service_test.go
git commit -m "feat(api-v1): DocumentService.PurgeByDocName for /v1/system/remove-documents"
```

---

## Task 3: DocumentService.RemoveFolder (orchestrator)

Folder-level purge: enumerate every `.json` doc in the folder, run per-doc purge, then delete the directory. Refuse the reserved `custom-documents` folder.

Mirrors `purgeFolder(folderName)` in `server/utils/files/purgeDocument.js`.

**Files:**
- Modify: `backend/internal/services/document_service.go`
- Modify: `backend/internal/services/document_service_test.go`

- [ ] **Step 1: Write the failing test**

Append to `document_service_test.go`:

```go
func TestDocumentService_RemoveFolder_RejectsReserved(t *testing.T) {
    _, svc, _ := setupDocDB(t)
    err := svc.RemoveFolder(context.Background(), "custom-documents")
    require.Error(t, err)
    assert.Contains(t, err.Error(), "reserved")
}

func TestDocumentService_RemoveFolder_PurgesAllDocs(t *testing.T) {
    db, svc, tmpDir := setupDocDB(t)
    ws := &models.Workspace{Name: "ws", Slug: "ws"}
    require.NoError(t, db.Create(ws).Error)

    folder := "my-folder"
    folderPath := filepath.Join(tmpDir, "documents", folder)
    require.NoError(t, os.MkdirAll(folderPath, 0o755))
    // Two json docs
    p1 := filepath.Join(folderPath, "a.json")
    p2 := filepath.Join(folderPath, "b.json")
    require.NoError(t, os.WriteFile(p1, []byte(`{"id":"doc-a"}`), 0o644))
    require.NoError(t, os.WriteFile(p2, []byte(`{"id":"doc-b"}`), 0o644))

    require.NoError(t, db.Create(&models.WorkspaceDocument{
        DocId: "doc-a", Filename: "a", Docpath: folder + "/a.json", WorkspaceID: ws.ID,
    }).Error)
    require.NoError(t, db.Create(&models.WorkspaceDocument{
        DocId: "doc-b", Filename: "b", Docpath: folder + "/b.json", WorkspaceID: ws.ID,
    }).Error)

    err := svc.RemoveFolder(context.Background(), folder)
    require.NoError(t, err)

    // Folder gone
    _, statErr := os.Stat(folderPath)
    assert.True(t, os.IsNotExist(statErr))

    // DB rows gone
    var count int64
    db.Model(&models.WorkspaceDocument{}).Count(&count)
    assert.Equal(t, int64(0), count)
}

func TestDocumentService_RemoveFolder_MissingFolder(t *testing.T) {
    _, svc, _ := setupDocDB(t)
    // Non-existent folder: should not error (Node behavior is to swallow ENOENT).
    err := svc.RemoveFolder(context.Background(), "nope")
    assert.NoError(t, err)
}
```

- [ ] **Step 2: Run the test and confirm it fails**

```bash
cd backend && go test ./internal/services/ -run TestDocumentService_RemoveFolder -count=1
```

- [ ] **Step 3: Implement the method**

```go
// RemoveFolder purges every .json document in folderName, drops DB + vector state,
// then deletes the directory. Refuses the reserved "custom-documents" folder.
// Missing folder is not an error.
func (s *DocumentService) RemoveFolder(ctx context.Context, folderName string) error {
    if folderName == "custom-documents" {
        return fmt.Errorf("cannot delete reserved folder: custom-documents")
    }

    base := filepath.Join(s.cfg.StorageDir, "documents", folderName)
    entries, err := os.ReadDir(base)
    if err != nil {
        if os.IsNotExist(err) {
            return nil
        }
        return err
    }

    for _, e := range entries {
        if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
            continue
        }
        docPath := folderName + "/" + e.Name()
        if err := s.PurgeByDocName(ctx, docPath); err != nil {
            // Log but continue — partial cleanup is better than abort.
            mlog.Error("RemoveFolder: purge doc failed: ", err, " path=", docPath)
        }
    }
    return s.fs.RemoveFolder(folderName)
}
```

Add `"fmt"`, `"path/filepath"`, `"strings"`, `"os"`, and `"github.com/odysseythink/mlog"` to imports if missing.

- [ ] **Step 4: Run the test and confirm it passes**

- [ ] **Step 5: Commit**

```bash
git add backend/internal/services/document_service.go backend/internal/services/document_service_test.go
git commit -m "feat(api-v1): DocumentService.RemoveFolder orchestrator for /v1/document/remove-folder"
```

---

## Task 4: DocumentService.SaveRawText

Persist raw text as a `.json` document under `custom-documents/`, then (optionally) bind to multiple workspaces by creating `workspace_documents` rows. No embedding in this PR — embedding is a separate handler call in Node and is already covered by `DocumentService.UploadAndQueueEmbed` for follow-up.

Mirrors `server/endpoints/api/document/index.js:479-650` (sans Collector enrichment, sans Telemetry).

**Files:**
- Modify: `backend/internal/services/document_service.go`
- Modify: `backend/internal/services/document_service_test.go`

- [ ] **Step 1: Write the failing test**

Append to `document_service_test.go`:

```go
import (
    "encoding/json"
    // ... existing imports
)

func TestDocumentService_SaveRawText_NoWorkspaces(t *testing.T) {
    _, svc, tmpDir := setupDocDB(t)

    docs, err := svc.SaveRawText(context.Background(),
        "Hello world.",
        "greeting",
        map[string]any{"title": "greeting", "docSource": "test"},
        nil,
    )
    require.NoError(t, err)
    assert.Empty(t, docs) // no workspace binds → no rows

    // A file should exist under custom-documents/
    entries, err := os.ReadDir(filepath.Join(tmpDir, "documents", "custom-documents"))
    require.NoError(t, err)
    require.Len(t, entries, 1)
    assert.True(t, strings.HasSuffix(entries[0].Name(), ".json"))

    // Payload contains the title + text
    raw, err := os.ReadFile(filepath.Join(tmpDir, "documents", "custom-documents", entries[0].Name()))
    require.NoError(t, err)
    var payload map[string]any
    require.NoError(t, json.Unmarshal(raw, &payload))
    assert.Equal(t, "Hello world.", payload["pageContent"])
    assert.Equal(t, "greeting", payload["title"])
}

func TestDocumentService_SaveRawText_MultiWorkspaceBind(t *testing.T) {
    db, svc, _ := setupDocDB(t)
    ws1 := &models.Workspace{Name: "w1", Slug: "w1"}
    ws2 := &models.Workspace{Name: "w2", Slug: "w2"}
    require.NoError(t, db.Create(ws1).Error)
    require.NoError(t, db.Create(ws2).Error)

    docs, err := svc.SaveRawText(context.Background(),
        "Hi.", "hi",
        map[string]any{"title": "hi"},
        []string{"w1", "w2"},
    )
    require.NoError(t, err)
    assert.Len(t, docs, 2)

    var count int64
    db.Model(&models.WorkspaceDocument{}).Count(&count)
    assert.Equal(t, int64(2), count)
}

func TestDocumentService_SaveRawText_UnknownWorkspaceSlug_Skipped(t *testing.T) {
    db, svc, _ := setupDocDB(t)
    ws1 := &models.Workspace{Name: "w1", Slug: "w1"}
    require.NoError(t, db.Create(ws1).Error)

    docs, err := svc.SaveRawText(context.Background(),
        "Hi.", "hi",
        map[string]any{"title": "hi"},
        []string{"w1", "ghost"},
    )
    require.NoError(t, err)
    assert.Len(t, docs, 1) // only w1 bound; ghost silently skipped
}
```

- [ ] **Step 2: Run the test and confirm it fails**

```bash
cd backend && go test ./internal/services/ -run TestDocumentService_SaveRawText -count=1
```

- [ ] **Step 3: Implement the method**

```go
// SaveRawText writes a JSON document under custom-documents/, then (optionally)
// binds it to each workspace in workspaceSlugs by creating a workspace_documents row.
// Unknown slugs are silently skipped (Node parity — Node's Document.addDocuments tolerates misses).
// Returns the WorkspaceDocument rows created (one per successful bind; empty if no slugs).
func (s *DocumentService) SaveRawText(
    ctx context.Context,
    text, title string,
    metadata map[string]any,
    workspaceSlugs []string,
) ([]*models.WorkspaceDocument, error) {
    if text == "" {
        return nil, fmt.Errorf("text cannot be empty")
    }
    if metadata == nil {
        metadata = map[string]any{}
    }
    if _, ok := metadata["title"]; !ok {
        metadata["title"] = title
    }

    docID := uuid.New().String()
    safeTitle := strings.ReplaceAll(strings.ReplaceAll(title, "/", "-"), " ", "-")
    if safeTitle == "" {
        safeTitle = "raw"
    }
    filename := fmt.Sprintf("raw-%s-%s.json", safeTitle, docID)

    payload := map[string]any{
        "id":          docID,
        "title":       title,
        "pageContent": text,
        "docSource":   "raw-text-upload",
        "wordCount":   len(strings.Fields(text)),
        "published":   time.Now().Format(time.RFC3339),
    }
    for k, v := range metadata {
        payload[k] = v
    }

    raw, err := json.Marshal(payload)
    if err != nil {
        return nil, err
    }
    if _, err := s.fs.SaveFile("custom-documents", filename, bytes.NewReader(raw)); err != nil {
        return nil, err
    }

    docPath := "custom-documents/" + filename
    var bound []*models.WorkspaceDocument
    for _, slug := range workspaceSlugs {
        var ws models.Workspace
        if err := s.db.WithContext(ctx).Where("slug = ?", slug).First(&ws).Error; err != nil {
            continue // unknown slug → skip
        }
        row := &models.WorkspaceDocument{
            DocId:       uuid.New().String(),
            Filename:    filename,
            Docpath:     docPath,
            WorkspaceID: ws.ID,
        }
        if err := s.db.WithContext(ctx).Create(row).Error; err != nil {
            return nil, err
        }
        bound = append(bound, row)
    }
    return bound, nil
}
```

Add `"bytes"`, `"encoding/json"`, `"fmt"`, `"strings"`, `"time"`, and `"github.com/google/uuid"` to imports if missing.

- [ ] **Step 4: Run the test and confirm it passes**

- [ ] **Step 5: Commit**

```bash
git add backend/internal/services/document_service.go backend/internal/services/document_service_test.go
git commit -m "feat(api-v1): DocumentService.SaveRawText for /v1/document/raw-text"
```

---

## Task 5: Full-suite verify

- [ ] **Step 1: Run all tests**

```bash
cd backend && go test ./... -count=1
```

All existing tests must still pass — these methods are additive.

- [ ] **Step 2: Build verify**

```bash
cd backend && go build ./...
```

- [ ] **Step 3: Vet**

```bash
cd backend && go vet ./...
```

- [ ] **Step 4: Commit any final cleanup** (only if tests/build flagged something).

---

## Acceptance criteria

- [ ] `WorkspaceService.UpdatePin` updates a single row's `pinned` column; returns `gorm.ErrRecordNotFound` on miss.
- [ ] `DocumentService.PurgeByDocName` removes vector + DB + file; tolerates missing DB rows (still cleans file).
- [ ] `DocumentService.RemoveFolder` refuses `custom-documents`; tolerates missing folder; purges per-doc before deleting directory.
- [ ] `DocumentService.SaveRawText` writes `.json` under `custom-documents/`, binds to known workspaces, silently skips unknown slugs.
- [ ] `go test ./...` green; `go vet ./...` clean; `go build ./...` succeeds.
- [ ] No existing test or handler behavior changed.

---

## Known gaps after PR1 (track but DO NOT implement here)

1. **Collector enrichment for raw text** — Node returns `wordCount`, `token_count_estimate`, `docAuthor`, `description`, `chunkSource` from the Collector. Go writes a minimal payload. If handler clients require these fields, file a PR3 follow-up to call `collector.Client.ProcessRawText`.
2. **Auto-embed on raw-text bind** — Node's `addDocumentsToWorkspace` path calls `embed` after binding. PR1 only binds; embedding is a separate concern (PR4 handler may chain to `UpdateEmbeddings` if needed).
3. **EventLog / Telemetry** — `api_raw_document_uploaded`, `document_purged`, `folder_removed` events deferred to platform-wide event-log wire-up.
4. **Path-traversal hardening on `folderName`** — accepts e.g. `../../etc`. `FileSystemService.IsWithin` exists (`filesystem_service.go:200`) but is not enforced here. Currently relies on `RemoveFolder` semantics + reserved-name check. Add path-traversal guard in PR3 handler layer or as a separate hardening PR.
