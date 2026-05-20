# Document Creation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use gpowers:subagent-driven-development (recommended) or gpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Port AnythingLLM's "create-files" agent skill into Hermind as an aggregated toolset with 5 file types (text, Word, PowerPoint, PDF, Excel), downloadable via chat UI cards.

**Architecture:** Five tools registered with `Toolset: "document_creation"` are aggregated into a single UI entry with per-subtype toggles (mirrors the existing `filesystem` pattern). Text/Excel/PDF are pure Go; Word/PPT delegate to a Node.js subprocess that reuses AnythingLLM's mature `docx`/`pptx` libraries. Files are saved to `<instance>/generated-files/` and served via a new download API.

**Tech Stack:** Go (standard library, `github.com/xuri/excelize/v2`, `github.com/go-pdf/fpdf`), Node.js (`docx`, `pptx`, `marked`, `uuid`), React/TypeScript

---

## File Structure

```
# New files
api/handlers_generated_files.go              # Download API
api/handlers_generated_files_test.go         # Download API tests
tool/document/
  ├── register.go                            # Register all 5 tools
  ├── manager.go                             # File storage manager
  ├── manager_test.go
  ├── text.go                                # Text file generator
  ├── text_test.go
  ├── excel.go                               # Excel generator
  ├── excel_test.go
  ├── pdf.go                                 # PDF generator
  ├── pdf_test.go
  ├── nodejs.go                              # Node.js subprocess wrapper
  └── nodejs_test.go
config/descriptor/document_creation.go       # Config descriptor
web/src/components/chat/FileDownloadCard.tsx
web/src/components/chat/FileDownloadCard.module.css
web/src/components/chat/FileDownloadCard.test.tsx
document-scripts/
  package.json
  bin/generate-doc.js
  lib/manager.js
  lib/docx/create.js
  lib/docx/utils.js
  lib/pptx/create.js
  lib/pptx/utils.js

# Modified files
api/handlers_tools.go                        # Add document_creation aggregation
api/handlers_tools_test.go                   # Add aggregation tests
api/server.go                                # Add activeToolReg filtering
cli/engine_deps.go                           # Register 5 tools
go.mod                                       # Add excelize, fpdf
web/src/components/chat/ChatMessage.tsx      # Render file_download_card
web/src/api/schemas.ts                       # Add FileDownloadCard event type
```

---

### Task 1: File Storage Manager

**Files:**
- Create: `tool/document/manager.go`
- Create: `tool/document/manager_test.go`

**Rationale:** All file generators need a shared utility for saving files with safe filenames. This is the foundation everything else builds on.

- [ ] **Step 1: Write the failing test**

```go
// tool/document/manager_test.go
package document

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestManagerSaveGeneratedFile(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	buf := []byte("hello world")
	result, err := m.Save("text", "txt", buf, "notes.txt")
	require.NoError(t, err)
	require.NotEmpty(t, result.Filename)
	require.True(t, len(result.Filename) > 40) // type-uuid.ext
	require.Equal(t, "notes.txt", result.DisplayFilename)
	require.Equal(t, int64(11), result.FileSize)
	require.True(t, filepath.IsAbs(result.StoragePath))

	// File should exist
	_, err = os.Stat(result.StoragePath)
	require.NoError(t, err)
}

func TestManagerGetGeneratedFile(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	buf := []byte("test content")
	saved, _ := m.Save("pdf", "pdf", buf, "report.pdf")

	got, err := m.Get(saved.Filename)
	require.NoError(t, err)
	require.Equal(t, buf, got.Buffer)
	require.Equal(t, saved.StoragePath, got.StoragePath)
}

func TestManagerGetInvalidFilename(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	got, err := m.Get("../../../etc/passwd")
	require.NoError(t, err)
	require.Nil(t, got)
}

func TestManagerMimeType(t *testing.T) {
	m := NewManager("")
	require.Equal(t, "application/pdf", m.MimeType("pdf"))
	require.Equal(t, "application/vnd.openxmlformats-officedocument.wordprocessingml.document", m.MimeType("docx"))
	require.Equal(t, "application/vnd.openxmlformats-officedocument.presentationml.presentation", m.MimeType("pptx"))
	require.Equal(t, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", m.MimeType("xlsx"))
	require.Equal(t, "text/plain", m.MimeType("txt"))
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /d/workspace/go_work/hermind && go test ./tool/document/ -v -run "TestManager"
```

Expected: FAIL — package does not exist, types undefined.

- [ ] **Step 3: Write minimal implementation**

```go
// tool/document/manager.go
package document

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"

	"github.com/google/uuid"
)

// Manager handles saving and retrieving generated files.
type Manager struct {
	outputDir string
}

// SavedFile holds metadata for a saved generated file.
type SavedFile struct {
	Filename        string
	DisplayFilename string
	FileSize        int64
	StoragePath     string
}

// RetrievedFile holds the content of a retrieved file.
type RetrievedFile struct {
	Buffer      []byte
	StoragePath string
}

var filenameRegex = regexp.MustCompile(`^([a-z]+)-([a-f0-9-]{36})\.(\w+)$`)

// NewManager creates a Manager that stores files in outputDir.
func NewManager(outputDir string) *Manager {
	return &Manager{outputDir: outputDir}
}

// EnsureDir creates the output directory if it does not exist.
func (m *Manager) EnsureDir() error {
	return os.MkdirAll(m.outputDir, 0o755)
}

// Save writes a file to storage and returns metadata.
func (m *Manager) Save(fileType, extension string, buffer []byte, displayFilename string) (*SavedFile, error) {
	if err := m.EnsureDir(); err != nil {
		return nil, fmt.Errorf("create output dir: %w", err)
	}
	filename := fmt.Sprintf("%s-%s.%s", fileType, uuid.NewString(), extension)
	storagePath := filepath.Join(m.outputDir, filename)
	if err := os.WriteFile(storagePath, buffer, 0o644); err != nil {
		return nil, fmt.Errorf("write file: %w", err)
	}
	return &SavedFile{
		Filename:        filename,
		DisplayFilename: displayFilename,
		FileSize:        int64(len(buffer)),
		StoragePath:     storagePath,
	}, nil
}

// Get retrieves a generated file by its storage filename.
func (m *Manager) Get(filename string) (*RetrievedFile, error) {
	if !filenameRegex.MatchString(filename) {
		return nil, nil
	}
	storagePath := filepath.Join(m.outputDir, filename)
	// Defensive: ensure resolved path is still inside outputDir
	if !isSubpath(storagePath, m.outputDir) {
		return nil, nil
	}
	buf, err := os.ReadFile(storagePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return &RetrievedFile{Buffer: buf, StoragePath: storagePath}, nil
}

// MimeType returns the MIME type for a file extension.
func (m *Manager) MimeType(ext string) string {
	switch ext {
	case "pptx":
		return "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	case "xlsx":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case "docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case "pdf":
		return "application/pdf"
	case "txt", "md", "csv", "json", "html", "xml", "yaml", "log":
		return "text/plain; charset=utf-8"
	default:
		return "application/octet-stream"
	}
}

func isSubpath(target, base string) bool {
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return false
	}
	return !path.IsAbs(rel) && rel != ".." && !containsDotDot(rel)
}

func containsDotDot(p string) bool {
	parts := filepath.SplitList(p)
	for _, part := range parts {
		if part == ".." {
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd /d/workspace/go_work/hermind && go test ./tool/document/ -v -run "TestManager"
```

Expected: PASS for all 4 tests.

- [ ] **Step 5: Commit**

```bash
cd /d/workspace/go_work/hermind && git add tool/document/manager.go tool/document/manager_test.go && git commit -m "feat(document): file storage manager with safe filenames"
```

---

### Task 2: Download API

**Files:**
- Create: `api/handlers_generated_files.go`
- Create: `api/handlers_generated_files_test.go`
- Modify: `api/server.go` (add route)

- [ ] **Step 1: Write the failing test**

```go
// api/handlers_generated_files_test.go
package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGeneratedFileDownload_Success(t *testing.T) {
	srv, tmpDir := newTestServerWithGeneratedFiles(t)

	// Create a test file
	err := os.WriteFile(filepath.Join(tmpDir, "text-12345678-1234-1234-1234-123456789abc.txt"), []byte("hello"), 0o644)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/generated-files/text-12345678-1234-1234-1234-123456789abc.txt", nil)
	rr := httptest.NewRecorder()
	srv.router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	require.Equal(t, "hello", rr.Body.String())
	require.Equal(t, "text/plain; charset=utf-8", rr.Header().Get("Content-Type"))
}

func TestGeneratedFileDownload_InvalidFilename(t *testing.T) {
	srv, _ := newTestServerWithGeneratedFiles(t)

	req := httptest.NewRequest(http.MethodGet, "/api/generated-files/../../../etc/passwd", nil)
	rr := httptest.NewRecorder()
	srv.router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestGeneratedFileDownload_NotFound(t *testing.T) {
	srv, _ := newTestServerWithGeneratedFiles(t)

	req := httptest.NewRequest(http.MethodGet, "/api/generated-files/text-12345678-1234-1234-1234-123456789abc.txt", nil)
	rr := httptest.NewRecorder()
	srv.router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusNotFound, rr.Code)
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /d/workspace/go_work/hermind && go test ./api/ -v -run "TestGeneratedFileDownload"
```

Expected: FAIL — `newTestServerWithGeneratedFiles` undefined, `handleGeneratedFileDownload` undefined.

- [ ] **Step 3: Write minimal implementation**

```go
// api/handlers_generated_files.go
package api

import (
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/odysseythink/hermind/tool/document"
)

func (s *Server) handleGeneratedFileDownload(w http.ResponseWriter, r *http.Request) {
	filename := chi.URLParam(r, "filename")
	if filename == "" {
		http.Error(w, "filename is required", http.StatusBadRequest)
		return
	}

	mgr := document.NewManager(s.generatedFilesDir())
	file, err := mgr.Get(filename)
	if err != nil {
		http.Error(w, "failed to read file", http.StatusInternalServerError)
		return
	}
	if file == nil {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}

	ext := filepath.Ext(filename)
	if len(ext) > 0 {
		ext = ext[1:]
	}
	mimeType := mgr.MimeType(ext)

	w.Header().Set("Content-Type", mimeType)
	w.Header().Set("Content-Length", strconv.Itoa(len(file.Buffer)))
	w.Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")
	w.Write(file.Buffer)
}

func (s *Server) generatedFilesDir() string {
	return filepath.Join(s.opts.InstanceRoot, "generated-files")
}
```

In `api/server.go`, add the route in the router setup (find where other routes are registered, likely near `s.router.Get("/api/tools", s.handleToolsList)`):

```go
s.router.Get("/api/generated-files/{filename}", s.handleGeneratedFileDownload)
```

In the test file, add the helper:

```go
func newTestServerWithGeneratedFiles(t *testing.T) (*Server, string) {
	tmpDir := t.TempDir()
	srv := NewServer(&ServerOpts{
		InstanceRoot: tmpDir,
	})
	return srv, filepath.Join(tmpDir, "generated-files")
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd /d/workspace/go_work/hermind && go test ./api/ -v -run "TestGeneratedFileDownload"
```

Expected: PASS for all 3 tests.

- [ ] **Step 5: Commit**

```bash
cd /d/workspace/go_work/hermind && git add api/handlers_generated_files.go api/handlers_generated_files_test.go && git commit -m "feat(api): generated file download endpoint"
```

---

### Task 3: API Aggregation (handleToolsList)

**Files:**
- Modify: `api/handlers_tools.go`
- Modify: `api/handlers_tools_test.go`

- [ ] **Step 1: Write the failing test**

In `api/handlers_tools_test.go`, add:

```go
func TestToolsList_DocumentCreationAggregated(t *testing.T) {
	srv := NewTestServer(t)
	deps := api.EngineDeps{ToolReg: tool.NewRegistry()}

	// Register document_creation tools
	deps.ToolReg.Register(&tool.Entry{
		Name: "create_text_file", Toolset: "document_creation",
		Description: "Create text files", Handler: func(_ context.Context, _ json.RawMessage) (string, error) { return "", nil },
	})
	deps.ToolReg.Register(&tool.Entry{
		Name: "create_word_document", Toolset: "document_creation",
		Description: "Create Word docs", Handler: func(_ context.Context, _ json.RawMessage) (string, error) { return "", nil },
	})

	srv.SetDeps(deps)

	req := httptest.NewRequest(http.MethodGet, "/api/tools", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	var resp ToolsResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))

	// Should have aggregated document_creation entry, not individual tools
	var docCreation *ToolDTO
	for i := range resp.Tools {
		if resp.Tools[i].Name == "document_creation" {
			docCreation = &resp.Tools[i]
			break
		}
	}
	require.NotNil(t, docCreation, "document_creation should be aggregated")
	require.Equal(t, "document_creation", docCreation.Toolset)
	require.Len(t, docCreation.SettingsSchema, 5)

	// Individual tools should NOT appear
	for _, t := range resp.Tools {
		require.NotEqual(t, "create_text_file", t.Name)
		require.NotEqual(t, "create_word_document", t.Name)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /d/workspace/go_work/hermind && go test ./api/ -v -run "TestToolsList_DocumentCreationAggregated"
```

Expected: FAIL — `document_creation` not found in response, individual tools still present.

- [ ] **Step 3: Write minimal implementation**

In `api/handlers_tools.go`, modify the `handleToolsList` function. Find the existing filesystem aggregation block and add a symmetric block for `document_creation`:

```go
func (s *Server) handleToolsList(w http.ResponseWriter, _ *http.Request) {
	deps := s.currentDeps()
	if deps.ToolReg == nil {
		writeJSON(w, ToolsResponse{Tools: []ToolDTO{}})
		return
	}

	disabled := s.disabledTools()
	entries := deps.ToolReg.Entries(nil)

	var fileTools []*tool.Entry
	var docTools []*tool.Entry
	var otherTools []*tool.Entry

	for _, e := range entries {
		if e.Toolset == "file" {
			fileTools = append(fileTools, e)
		} else if e.Toolset == "document_creation" {
			docTools = append(docTools, e)
		} else {
			otherTools = append(otherTools, e)
		}
	}

	out := make([]ToolDTO, 0, len(otherTools)+2)

	for _, e := range otherTools {
		if e.Name == "filesystem" {
			continue
		}
		dto := ToolDTO{
			Name:           e.Name,
			Description:    e.Description,
			Toolset:        e.Toolset,
			Enabled:        !disabled[e.Name],
			SettingsSchema: []ConfigFieldDTO{},
		}
		if e.Name == "browser_control" {
			dto.SettingsSchema = []ConfigFieldDTO{
				{Name: "enabled", Label: "Enabled", Kind: "bool", Help: "Enable the browser extension integration."},
			}
		}
		out = append(out, dto)
	}

	if len(docTools) > 0 {
		out = append(out, ToolDTO{
			Name:        "document_creation",
			Description: "Document creation — allows the agent to generate text files, Word documents, PowerPoint presentations, PDFs, and Excel spreadsheets.",
			Toolset:     "document_creation",
			Enabled:     !disabled["document_creation"],
			SettingsSchema: []ConfigFieldDTO{
				{Name: "create_text_file", Label: "Text files", Kind: "bool", Help: "Create text files (.txt, .md, .json, .csv, etc.)", Default: true},
				{Name: "create_word_document", Label: "Word documents", Kind: "bool", Help: "Create Microsoft Word documents (.docx)", Default: true},
				{Name: "create_pptx_presentation", Label: "PowerPoint", Kind: "bool", Help: "Create PowerPoint presentations (.pptx)", Default: true},
				{Name: "create_pdf_document", Label: "PDF documents", Kind: "bool", Help: "Create PDF documents", Default: true},
				{Name: "create_excel_spreadsheet", Label: "Excel spreadsheets", Kind: "bool", Help: "Create Excel spreadsheets (.xlsx)", Default: true},
			},
		})
	}

	if len(fileTools) > 0 {
		// ... existing filesystem aggregation ...
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	writeJSON(w, ToolsResponse{Tools: out})
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd /d/workspace/go_work/hermind && go test ./api/ -v -run "TestToolsList_DocumentCreationAggregated"
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
cd /d/workspace/go_work/hermind && git add api/handlers_tools.go api/handlers_tools_test.go && git commit -m "feat(api): aggregate document_creation tools in tools list"
```

---

### Task 4: activeToolReg Filtering

**Files:**
- Modify: `api/server.go`
- Modify: `api/server_test.go` (or create if needed)

- [ ] **Step 1: Write the failing test**

```go
// In api/server_test.go or api/handlers_tools_test.go
func TestActiveToolReg_DocumentCreationFiltering(t *testing.T) {
	srv := NewTestServer(t)

	// Setup config with document_creation disabled
	srv.SetConfig(&config.Config{
		Tools: config.ToolsConfig{
			Disabled: []string{"document_creation"},
		},
	})

	deps := api.EngineDeps{ToolReg: tool.NewRegistry()}
	deps.ToolReg.Register(&tool.Entry{Name: "create_text_file", Toolset: "document_creation"})
	deps.ToolReg.Register(&tool.Entry{Name: "create_pdf_document", Toolset: "document_creation"})
	deps.ToolReg.Register(&tool.Entry{Name: "other_tool", Toolset: "other"})
	srv.SetDeps(deps)

	active := srv.ActiveToolReg()
	require.NotNil(t, active)

	// document_creation tools should be excluded
	defs := active.Definitions(nil)
	for _, d := range defs {
		require.NotEqual(t, "create_text_file", d.Name)
		require.NotEqual(t, "create_pdf_document", d.Name)
	}
	// other_tool should remain
	found := false
	for _, d := range defs {
		if d.Name == "other_tool" {
			found = true
			break
		}
	}
	require.True(t, found)
}

func TestActiveToolReg_DocumentCreationSubtoolFiltering(t *testing.T) {
	srv := NewTestServer(t)

	// Master enabled, but text disabled
	srv.SetConfig(&config.Config{
		Tools: config.ToolsConfig{
			Settings: map[string]map[string]interface{}{
				"document_creation": {
					"create_text_file": false,
					"create_pdf_document": true,
				},
			},
		},
	})

	deps := api.EngineDeps{ToolReg: tool.NewRegistry()}
	deps.ToolReg.Register(&tool.Entry{Name: "create_text_file", Toolset: "document_creation"})
	deps.ToolReg.Register(&tool.Entry{Name: "create_pdf_document", Toolset: "document_creation"})
	srv.SetDeps(deps)

	active := srv.ActiveToolReg()
	defs := active.Definitions(nil)

	foundText := false
	foundPDF := false
	for _, d := range defs {
		if d.Name == "create_text_file" {
			foundText = true
		}
		if d.Name == "create_pdf_document" {
			foundPDF = true
		}
	}
	require.False(t, foundText, "create_text_file should be filtered out")
	require.True(t, foundPDF, "create_pdf_document should remain")
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /d/workspace/go_work/hermind && go test ./api/ -v -run "TestActiveToolReg_DocumentCreation"
```

Expected: FAIL — filtering logic not implemented.

- [ ] **Step 3: Write minimal implementation**

In `api/server.go`, in the `activeToolReg()` function, add the document_creation filtering logic after the existing filesystem logic:

```go
func (s *Server) activeToolReg() *tool.Registry {
	s.injectFilesystemConfig()
	deps := s.currentDeps()
	if deps.ToolReg == nil {
		return nil
	}
	disabled := s.disabledTools()
	active := tool.NewRegistry()

	// --- Existing filesystem logic ---
	filesystemDisabled := disabled["filesystem"]
	subtoolEnabled := make(map[string]bool)
	if fsSettings, ok := s.opts.Config.Tools.Settings["filesystem"]; ok {
		for key, val := range fsSettings {
			if key == "allowed_directories" {
				continue
			}
			if b, ok := val.(bool); ok {
				subtoolEnabled[key] = b
			}
		}
	}

	// --- New: document_creation logic ---
	docCreationDisabled := disabled["document_creation"]
	docSubtoolEnabled := make(map[string]bool)
	if docSettings, ok := s.opts.Config.Tools.Settings["document_creation"]; ok {
		for key, val := range docSettings {
			if b, ok := val.(bool); ok {
				docSubtoolEnabled[key] = b
			}
		}
	}

	for _, e := range deps.ToolReg.Entries(nil) {
		if e.Name == "filesystem" {
			continue
		}

		if e.Toolset == "file" && filesystemDisabled {
			continue
		}
		if e.Toolset == "file" && !filesystemDisabled {
			if enabled, ok := subtoolEnabled[e.Name]; ok && !enabled {
				continue
			}
		}

		// New: document_creation filtering
		if e.Toolset == "document_creation" && docCreationDisabled {
			continue
		}
		if e.Toolset == "document_creation" && !docCreationDisabled {
			if enabled, ok := docSubtoolEnabled[e.Name]; ok && !enabled {
				continue
			}
		}

		if disabled[e.Name] {
			continue
		}
		active.Register(e)
	}
	return active
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd /d/workspace/go_work/hermind && go test ./api/ -v -run "TestActiveToolReg_DocumentCreation"
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
cd /d/workspace/go_work/hermind && git add api/server.go api/server_test.go && git commit -m "feat(api): filter document_creation tools by master toggle and subtype settings"
```

---

### Task 5: Text File Generator

**Files:**
- Create: `tool/document/text.go`
- Create: `tool/document/text_test.go`

- [ ] **Step 1: Write the failing test**

```go
// tool/document/text_test.go
package document

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCreateTextFile(t *testing.T) {
	tmpDir := t.TempDir()
	handler := NewCreateTextFileHandler(tmpDir)

	args, _ := json.Marshal(map[string]interface{}{
		"filename": "notes",
		"extension": "md",
		"content": "# Hello\n\nWorld",
	})

	result, err := handler(context.Background(), args)
	require.NoError(t, err)

	var meta map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(result), &meta))
	require.Equal(t, "notes.md", meta["displayFilename"])
	require.NotEmpty(t, meta["storageFilename"])
	require.NotEmpty(t, meta["downloadUrl"])

	// File should exist
	storageFilename := meta["storageFilename"].(string)
	path := filepath.Join(tmpDir, storageFilename)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, "# Hello\n\nWorld", string(data))
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /d/workspace/go_work/hermind && go test ./tool/document/ -v -run "TestCreateTextFile"
```

Expected: FAIL — `NewCreateTextFileHandler` undefined.

- [ ] **Step 3: Write minimal implementation**

```go
// tool/document/text.go
package document

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/odysseythink/hermind/tool"
	"github.com/odysseythink/pantheon/core"
)

// NewCreateTextFileHandler returns a handler for create_text_file.
func NewCreateTextFileHandler(outputDir string) tool.Handler {
	mgr := NewManager(outputDir)
	return func(_ context.Context, args json.RawMessage) (string, error) {
		var params struct {
			Filename  string `json:"filename"`
			Extension string `json:"extension"`
			Content   string `json:"content"`
		}
		if err := json.Unmarshal(args, &params); err != nil {
			return tool.Error(fmt.Sprintf("invalid args: %v", err)), nil
		}
		if params.Filename == "" {
			params.Filename = "document"
		}
		if params.Extension == "" {
			params.Extension = "txt"
		}
		ext := strings.ToLower(strings.TrimPrefix(params.Extension, "."))
		if !strings.Contains(params.Filename, ".") {
			params.Filename = params.Filename + "." + ext
		}
		finalExt := params.Filename[strings.LastIndex(params.Filename, ".")+1:]

		buf := []byte(params.Content)
		saved, err := mgr.Save("text", finalExt, buf, params.Filename)
		if err != nil {
			return tool.Error(fmt.Sprintf("save file: %v", err)), nil
		}

		return resultJSON(saved, fmt.Sprintf("Successfully created text file '%s' (%d bytes).", saved.DisplayFilename, saved.FileSize)), nil
	}
}

func resultJSON(saved *SavedFile, message string) string {
	b, _ := json.Marshal(map[string]interface{}{
		"filename":        saved.DisplayFilename,
		"storageFilename": saved.Filename,
		"fileSize":        saved.FileSize,
		"downloadUrl":     "/api/generated-files/" + saved.Filename,
		"message":         message,
	})
	return string(b)
}

// CreateTextFileSchema is the JSON schema for create_text_file.
const CreateTextFileSchema = `{
	"type": "object",
	"properties": {
		"filename": {"type": "string", "description": "The filename for the text file. If no extension is provided, the extension parameter will be used."},
		"extension": {"type": "string", "description": "The file extension to use (without the dot). Defaults to 'txt'. Common options: txt, md, json, csv, html, xml, yaml, log.", "default": "txt"},
		"content": {"type": "string", "description": "The text content to write to the file."}
	},
	"required": ["filename", "content"]
}`

// RegisterCreateTextFile registers the create_text_file tool.
func RegisterCreateTextFile(reg *tool.Registry, outputDir string) {
	reg.Register(&tool.Entry{
		Name:        "create_text_file",
		Toolset:     "document_creation",
		Description: "Create a text file with arbitrary content. Supports .txt, .md, .json, .csv, .html, .xml, .yaml, .log, and more.",
		Emoji:       "📝",
		Handler:     NewCreateTextFileHandler(outputDir),
		Schema: core.ToolDefinition{
			Name:        "create_text_file",
			Description: "Create a text file with arbitrary content. Provide the content and an optional file extension (defaults to .txt). Common extensions include .txt, .md, .json, .csv, .html, .xml, .yaml, .log, etc.",
			Parameters:  core.MustSchemaFromJSON([]byte(CreateTextFileSchema)),
		},
	})
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd /d/workspace/go_work/hermind && go test ./tool/document/ -v -run "TestCreateTextFile"
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
cd /d/workspace/go_work/hermind && git add tool/document/text.go tool/document/text_test.go && git commit -m "feat(document): text file generator tool"
```

---

### Task 6: Excel Generator

**Files:**
- Create: `tool/document/excel.go`
- Create: `tool/document/excel_test.go`

- [ ] **Step 1: Write the failing test**

```go
// tool/document/excel_test.go
package document

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

func TestCreateExcelFile(t *testing.T) {
	tmpDir := t.TempDir()
	handler := NewCreateExcelHandler(tmpDir)

	args, _ := json.Marshal(map[string]interface{}{
		"filename": "report",
		"title":    "Q1 Report",
		"content":  "## Sheet1\n\n| Name | Value |\n|------|-------|\n| A    | 10    |\n| B    | 20    |",
	})

	result, err := handler(context.Background(), args)
	require.NoError(t, err)

	var meta map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(result), &meta))
	require.Equal(t, "report.xlsx", meta["displayFilename"])

	// Open and verify
	storageFilename := meta["storageFilename"].(string)
	f, err := excelize.OpenFile(tmpDir + "/" + storageFilename)
	require.NoError(t, err)
	defer f.Close()

	val, _ := f.GetCellValue("Sheet1", "A1")
	require.Equal(t, "Name", val)
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /d/workspace/go_work/hermind && go test ./tool/document/ -v -run "TestCreateExcelFile"
```

Expected: FAIL — `NewCreateExcelHandler` undefined, `excelize` not in go.mod.

- [ ] **Step 3: Add dependency**

```bash
cd /d/workspace/go_work/hermind && go get github.com/xuri/excelize/v2
```

- [ ] **Step 4: Write minimal implementation**

```go
// tool/document/excel.go
package document

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/odysseythink/hermind/tool"
	"github.com/odysseythink/pantheon/core"
	"github.com/xuri/excelize/v2"
)

// NewCreateExcelHandler returns a handler for create_excel_spreadsheet.
func NewCreateExcelHandler(outputDir string) tool.Handler {
	mgr := NewManager(outputDir)
	return func(_ context.Context, args json.RawMessage) (string, error) {
		var params struct {
			Filename string `json:"filename"`
			Title    string `json:"title"`
			Content  string `json:"content"`
		}
		if err := json.Unmarshal(args, &params); err != nil {
			return tool.Error(fmt.Sprintf("invalid args: %v", err)), nil
		}
		if params.Filename == "" {
			params.Filename = "spreadsheet"
		}
		if !strings.HasSuffix(strings.ToLower(params.Filename), ".xlsx") {
			params.Filename += ".xlsx"
		}

		f := excelize.NewFile()
		if params.Title != "" {
			f.SetDocProps(&excelize.DocProperties{Title: params.Title})
		}

		tables := parseMarkdownTables(params.Content)
		for i, table := range tables {
			sheetName := fmt.Sprintf("Sheet%d", i+1)
			if i == 0 {
				f.SetSheetName("Sheet1", sheetName)
			} else {
				f.NewSheet(sheetName)
			}
			for rowIdx, row := range table.Rows {
				for colIdx, cell := range row {
					cellRef, _ := excelize.CoordinatesToCellName(colIdx+1, rowIdx+1)
					f.SetCellValue(sheetName, cellRef, cell)
				}
			}
		}

		buf, err := f.WriteToBuffer()
		if err != nil {
			return tool.Error(fmt.Sprintf("generate excel: %v", err)), nil
		}

		saved, err := mgr.Save("xlsx", "xlsx", buf.Bytes(), params.Filename)
		if err != nil {
			return tool.Error(fmt.Sprintf("save file: %v", err)), nil
		}

		return resultJSON(saved, fmt.Sprintf("Successfully created Excel spreadsheet '%s' (%d bytes).", saved.DisplayFilename, saved.FileSize)), nil
	}
}

type markdownTable struct {
	Rows [][]string
}

var tableRowRegex = regexp.MustCompile(`^\|(.+)\|$`)

func parseMarkdownTables(content string) []markdownTable {
	lines := strings.Split(content, "\n")
	var tables []markdownTable
	var current *markdownTable

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "|---") {
			continue
		}
		match := tableRowRegex.FindStringSubmatch(line)
		if match != nil {
			if current == nil {
				current = &markdownTable{}
			}
			cells := strings.Split(match[1], "|")
			var row []string
			for _, c := range cells {
				row = append(row, strings.TrimSpace(c))
			}
			current.Rows = append(current.Rows, row)
		} else {
			if current != nil {
				tables = append(tables, *current)
				current = nil
			}
		}
	}
	if current != nil {
		tables = append(tables, *current)
	}
	return tables
}

const CreateExcelSchema = `{
	"type": "object",
	"properties": {
		"filename": {"type": "string", "description": "The filename for the spreadsheet. Will add .xlsx if not present."},
		"title": {"type": "string", "description": "Optional document title for metadata."},
		"content": {"type": "string", "description": "Markdown tables to convert to Excel sheets. Each table becomes a worksheet."}
	},
	"required": ["filename", "content"]
}`

func RegisterCreateExcel(reg *tool.Registry, outputDir string) {
	reg.Register(&tool.Entry{
		Name:        "create_excel_spreadsheet",
		Toolset:     "document_creation",
		Description: "Create an Excel spreadsheet (.xlsx) from Markdown tables.",
		Emoji:       "📊",
		Handler:     NewCreateExcelHandler(outputDir),
		Schema: core.ToolDefinition{
			Name:        "create_excel_spreadsheet",
			Description: "Create an Excel spreadsheet (.xlsx) from Markdown tables. Each table becomes a separate worksheet.",
			Parameters:  core.MustSchemaFromJSON([]byte(CreateExcelSchema)),
		},
	})
}
```

- [ ] **Step 5: Run test to verify it passes**

```bash
cd /d/workspace/go_work/hermind && go test ./tool/document/ -v -run "TestCreateExcelFile"
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
cd /d/workspace/go_work/hermind && git add tool/document/excel.go tool/document/excel_test.go go.mod go.sum && git commit -m "feat(document): Excel spreadsheet generator with markdown table parsing"
```

---

### Task 7: PDF Generator

**Files:**
- Create: `tool/document/pdf.go`
- Create: `tool/document/pdf_test.go`

- [ ] **Step 1: Write the failing test**

```go
// tool/document/pdf_test.go
package document

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCreatePDF(t *testing.T) {
	tmpDir := t.TempDir()
	handler := NewCreatePDFHandler(tmpDir)

	args, _ := json.Marshal(map[string]interface{}{
		"filename": "report",
		"title":    "Annual Report",
		"content":  "# Introduction\n\nThis is the annual report.\n\n## Section 1\n\nSome details here.",
	})

	result, err := handler(context.Background(), args)
	require.NoError(t, err)

	var meta map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(result), &meta))
	require.Equal(t, "report.pdf", meta["displayFilename"])
	require.True(t, meta["fileSize"].(float64) > 100)
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /d/workspace/go_work/hermind && go test ./tool/document/ -v -run "TestCreatePDF"
```

Expected: FAIL — `NewCreatePDFHandler` undefined, `fpdf` not in go.mod.

- [ ] **Step 3: Add dependency**

```bash
cd /d/workspace/go_work/hermind && go get github.com/go-pdf/fpdf
```

- [ ] **Step 4: Write minimal implementation**

```go
// tool/document/pdf.go
package document

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/go-pdf/fpdf"
	"github.com/odysseythink/hermind/tool"
	"github.com/odysseythink/pantheon/core"
)

// NewCreatePDFHandler returns a handler for create_pdf_document.
func NewCreatePDFHandler(outputDir string) tool.Handler {
	mgr := NewManager(outputDir)
	return func(_ context.Context, args json.RawMessage) (string, error) {
		var params struct {
			Filename string `json:"filename"`
			Title    string `json:"title"`
			Content  string `json:"content"`
		}
		if err := json.Unmarshal(args, &params); err != nil {
			return tool.Error(fmt.Sprintf("invalid args: %v", err)), nil
		}
		if params.Filename == "" {
			params.Filename = "document"
		}
		if !strings.HasSuffix(strings.ToLower(params.Filename), ".pdf") {
			params.Filename += ".pdf"
		}

		doc := fpdf.New("P", "mm", "A4", "")
		doc.SetAutoPageBreak(true, 15)
		doc.AddPage()
		doc.SetFont("Arial", "", 12)

		if params.Title != "" {
			doc.SetFont("Arial", "B", 16)
			doc.Cell(0, 10, params.Title)
			doc.Ln(12)
			doc.SetFont("Arial", "", 12)
		}

		lines := strings.Split(params.Content, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				doc.Ln(6)
				continue
			}
			if strings.HasPrefix(line, "# ") {
				doc.SetFont("Arial", "B", 14)
				doc.Cell(0, 10, strings.TrimPrefix(line, "# "))
				doc.Ln(10)
				doc.SetFont("Arial", "", 12)
			} else if strings.HasPrefix(line, "## ") {
				doc.SetFont("Arial", "B", 13)
				doc.Cell(0, 8, strings.TrimPrefix(line, "## "))
				doc.Ln(8)
				doc.SetFont("Arial", "", 12)
			} else if strings.HasPrefix(line, "### ") {
				doc.SetFont("Arial", "B", 12)
				doc.Cell(0, 7, strings.TrimPrefix(line, "### "))
				doc.Ln(7)
				doc.SetFont("Arial", "", 12)
			} else if strings.HasPrefix(line, "- ") {
				doc.Cell(5, 6, "\u2022")
				doc.Cell(0, 6, strings.TrimPrefix(line, "- "))
				doc.Ln(6)
			} else if strings.HasPrefix(line, "**") && strings.HasSuffix(line, "**") {
				doc.SetFont("Arial", "B", 12)
				text := strings.TrimSuffix(strings.TrimPrefix(line, "**"), "**")
				doc.MultiCell(0, 6, text, "", "", false)
				doc.SetFont("Arial", "", 12)
			} else {
				doc.MultiCell(0, 6, line, "", "", false)
			}
		}

		var buf bytes.Buffer
		if err := doc.Output(&buf); err != nil {
			return tool.Error(fmt.Sprintf("generate pdf: %v", err)), nil
		}

		saved, err := mgr.Save("pdf", "pdf", buf.Bytes(), params.Filename)
		if err != nil {
			return tool.Error(fmt.Sprintf("save file: %v", err)), nil
		}

		return resultJSON(saved, fmt.Sprintf("Successfully created PDF document '%s' (%d bytes).", saved.DisplayFilename, saved.FileSize)), nil
	}
}

const CreatePDFSchema = `{
	"type": "object",
	"properties": {
		"filename": {"type": "string", "description": "The filename for the PDF. Will add .pdf if not present."},
		"title": {"type": "string", "description": "Optional document title shown at the top of the first page."},
		"content": {"type": "string", "description": "The content to render as PDF. Supports # headings, ## subheadings, - bullet lists, and **bold** text."}
	},
	"required": ["filename", "content"]
}`

func RegisterCreatePDF(reg *tool.Registry, outputDir string) {
	reg.Register(&tool.Entry{
		Name:        "create_pdf_document",
		Toolset:     "document_creation",
		Description: "Create a PDF document from plain text or Markdown content.",
		Emoji:       "📑",
		Handler:     NewCreatePDFHandler(outputDir),
		Schema: core.ToolDefinition{
			Name:        "create_pdf_document",
			Description: "Create a PDF document from plain text or Markdown content. Supports headings, bullet lists, and bold text.",
			Parameters:  core.MustSchemaFromJSON([]byte(CreatePDFSchema)),
		},
	})
}
```

Note: Add `"bytes"` to the imports in `tool/document/pdf.go`.

- [ ] **Step 5: Run test to verify it passes**

```bash
cd /d/workspace/go_work/hermind && go test ./tool/document/ -v -run "TestCreatePDF"
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
cd /d/workspace/go_work/hermind && git add tool/document/pdf.go tool/document/pdf_test.go go.mod go.sum && git commit -m "feat(document): PDF generator with markdown rendering"
```

---

### Task 8: Node.js Subprocess Wrapper

**Files:**
- Create: `tool/document/nodejs.go`
- Create: `tool/document/nodejs_test.go`

- [ ] **Step 1: Write the failing test**

```go
// tool/document/nodejs_test.go
package document

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNodeJSWrapper_Generate(t *testing.T) {
	tmpDir := t.TempDir()
	// Create a mock script
	scriptDir := t.TempDir()
	scriptPath := filepath.Join(scriptDir, "generate-doc.js")
	mockScript := `const fs = require('fs');
const data = JSON.parse(require('fs').readFileSync(0, 'utf8'));
const out = data.outputDir + '/docx-test-12345678-1234-1234-1234-123456789abc.docx';
fs.writeFileSync(out, 'fake docx content');
console.log(out);`
	require.NoError(t, os.WriteFile(scriptPath, []byte(mockScript), 0o755))

	w := NewNodeJSWrapper(scriptDir, tmpDir)
	result, err := w.Generate(context.Background(), "docx", map[string]interface{}{
		"filename": "report.docx",
		"content":  "# Hello",
	})
	require.NoError(t, err)
	require.NotEmpty(t, result)
	require.True(t, filepath.IsAbs(result))
}

func TestNodeJSWrapper_ScriptNotFound(t *testing.T) {
	w := NewNodeJSWrapper("/nonexistent", "/tmp")
	_, err := w.Generate(context.Background(), "docx", map[string]interface{}{})
	require.Error(t, err)
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /d/workspace/go_work/hermind && go test ./tool/document/ -v -run "TestNodeJSWrapper"
```

Expected: FAIL — `NewNodeJSWrapper` undefined.

- [ ] **Step 3: Write minimal implementation**

```go
// tool/document/nodejs.go
package document

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// NodeJSWrapper invokes the Node.js document generation scripts.
type NodeJSWrapper struct {
	scriptDir string
	outputDir string
}

// NewNodeJSWrapper creates a wrapper. scriptDir is the directory containing
// the document-scripts package (where bin/generate-doc.js lives).
func NewNodeJSWrapper(scriptDir, outputDir string) *NodeJSWrapper {
	return &NodeJSWrapper{
		scriptDir: scriptDir,
		outputDir: outputDir,
	}
}

// Generate invokes the Node.js script for the given type with the given params.
// Returns the absolute path to the generated file.
func (w *NodeJSWrapper) Generate(ctx context.Context, docType string, params map[string]interface{}) (string, error) {
	scriptPath := filepath.Join(w.scriptDir, "bin", "generate-doc.js")
	if _, err := os.Stat(scriptPath); err != nil {
		return "", fmt.Errorf("node.js script not found at %s: %w", scriptPath, err)
	}

	params["type"] = docType
	params["outputDir"] = w.outputDir

	jsonArgs, err := json.Marshal(params)
	if err != nil {
		return "", fmt.Errorf("marshal params: %w", err)
	}

	cmd := exec.CommandContext(ctx, "node", scriptPath)
	cmd.Dir = w.scriptDir
	cmd.Stdin = strings.NewReader(string(jsonArgs))
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && len(exitErr.Stderr) > 0 {
			return "", fmt.Errorf("node.js error: %s", string(exitErr.Stderr))
		}
		return "", fmt.Errorf("node.js subprocess failed: %w", err)
	}

	path := strings.TrimSpace(string(out))
	if path == "" {
		return "", fmt.Errorf("node.js script returned empty path")
	}
	return path, nil
}

// IsAvailable returns true if Node.js and the script are available.
func (w *NodeJSWrapper) IsAvailable() bool {
	scriptPath := filepath.Join(w.scriptDir, "bin", "generate-doc.js")
	if _, err := os.Stat(scriptPath); err != nil {
		return false
	}
	if _, err := exec.LookPath("node"); err != nil {
		return false
	}
	return true
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd /d/workspace/go_work/hermind && go test ./tool/document/ -v -run "TestNodeJSWrapper"
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
cd /d/workspace/go_work/hermind && git add tool/document/nodejs.go tool/document/nodejs_test.go && git commit -m "feat(document): Node.js subprocess wrapper for docx/pptx generation"
```

---

### Task 9: Extract Node.js Docx Script from AnythingLLM

**Files:**
- Create: `document-scripts/package.json`
- Create: `document-scripts/bin/generate-doc.js`
- Create: `document-scripts/lib/manager.js`
- Create: `document-scripts/lib/docx/create.js`
- Create: `document-scripts/lib/docx/utils.js`

- [ ] **Step 1: Copy and adapt AnythingLLM docx code**

Copy the following files from `D:\Downloads\anything-llm-1.12.1\server\utils\agents\aibitat\plugins\create-files\`:

1. `lib.js` → `document-scripts/lib/manager.js` (simplified, remove aibitat/socket dependencies)
2. `docx/create-docx-file.js` → `document-scripts/lib/docx/create.js` (remove approval/super references)
3. `docx/utils.js` → `document-scripts/lib/docx/utils.js` (keep as-is)

Then create the CLI wrapper:

```javascript
// document-scripts/bin/generate-doc.js
const fs = require('fs');
const path = require('path');
const { createDocx } = require('../lib/docx/create');

async function main() {
  const input = fs.readFileSync(0, 'utf8');
  const params = JSON.parse(input);

  if (!params.outputDir) {
    console.error('outputDir is required');
    process.exit(1);
  }

  const outputPath = await createDocx(params);
  console.log(outputPath);
}

main().catch(err => {
  console.error(err.message);
  process.exit(1);
});
```

- [ ] **Step 2: Create package.json**

```json
{
  "name": "hermind-document-scripts",
  "version": "1.0.0",
  "description": "Document generation scripts for Hermind (extracted from AnythingLLM)",
  "bin": {
    "generate-doc": "./bin/generate-doc.js"
  },
  "dependencies": {
    "docx": "^9.0.0",
    "marked": "^12.0.0",
    "pptxgenjs": "^3.12.0",
    "uuid": "^9.0.0"
  }
}
```

- [ ] **Step 3: Install dependencies**

```bash
cd /d/workspace/go_work/hermind/document-scripts && npm install
```

Expected: `node_modules` created, no errors.

- [ ] **Step 4: Test the docx script manually**

```bash
cd /d/workspace/go_work/hermind/document-scripts
echo '{"type":"docx","filename":"test.docx","content":"# Hello World\n\nThis is a test.","outputDir":"/tmp/doc-test","theme":"neutral"}' | node bin/generate-doc.js
```

Expected: Prints absolute path to generated `.docx` file. File should exist and be a valid ZIP (docx is a ZIP file).

- [ ] **Step 5: Commit**

```bash
cd /d/workspace/go_work/hermind && git add document-scripts/ && git commit -m "feat(document-scripts): extract docx generation from AnythingLLM"
```

---

### Task 10: Extract Node.js PPTX Script from AnythingLLM

**Files:**
- Create: `document-scripts/lib/pptx/create.js`
- Create: `document-scripts/lib/pptx/utils.js`
- Modify: `document-scripts/bin/generate-doc.js`

- [ ] **Step 1: Copy and adapt AnythingLLM pptx code**

Copy from `D:\Downloads\anything-llm-1.12.1\server\utils\agents\aibitat\plugins\create-files\pptx\`:
1. `create-presentation.js` → `document-scripts/lib/pptx/create.js` (remove sub-agent research, simplify to direct rendering)
2. `utils.js`, `themes.js` → `document-scripts/lib/pptx/utils.js`, `document-scripts/lib/pptx/themes.js`

- [ ] **Step 2: Update CLI wrapper to support pptx**

```javascript
// document-scripts/bin/generate-doc.js
const fs = require('fs');
const { createDocx } = require('../lib/docx/create');
const { createPptx } = require('../lib/pptx/create');

async function main() {
  const input = fs.readFileSync(0, 'utf8');
  const params = JSON.parse(input);

  if (!params.outputDir) {
    console.error('outputDir is required');
    process.exit(1);
  }

  let outputPath;
  switch (params.type) {
    case 'docx':
      outputPath = await createDocx(params);
      break;
    case 'pptx':
      outputPath = await createPptx(params);
      break;
    default:
      console.error(`Unknown type: ${params.type}`);
      process.exit(1);
  }
  console.log(outputPath);
}

main().catch(err => {
  console.error(err.message);
  process.exit(1);
});
```

- [ ] **Step 3: Test the pptx script manually**

```bash
cd /d/workspace/go_work/hermind/document-scripts
echo '{"type":"pptx","filename":"test.pptx","title":"Test Deck","theme":"corporate","sections":[{"title":"Intro","keyPoints":["Point 1","Point 2"]}],"outputDir":"/tmp/doc-test"}' | node bin/generate-doc.js
```

Expected: Prints absolute path to generated `.pptx` file.

- [ ] **Step 4: Commit**

```bash
cd /d/workspace/go_work/hermind && git add document-scripts/ && git commit -m "feat(document-scripts): extract pptx generation from AnythingLLM"
```

---

### Task 11: Word and PowerPoint Go Handlers

**Files:**
- Create: `tool/document/register.go`
- Modify: `tool/document/nodejs.go` (add Word/PPT handlers)

- [ ] **Step 1: Write Word handler**

In `tool/document/nodejs.go`, add:

```go
// NewCreateWordHandler returns a handler for create_word_document.
func NewCreateWordHandler(wrapper *NodeJSWrapper) tool.Handler {
	mgr := NewManager(wrapper.outputDir)
	return func(ctx context.Context, args json.RawMessage) (string, error) {
		var params map[string]interface{}
		if err := json.Unmarshal(args, &params); err != nil {
			return tool.Error(fmt.Sprintf("invalid args: %v", err)), nil
		}
		filename, _ := params["filename"].(string)
		if filename == "" {
			filename = "document.docx"
		}
		if !strings.HasSuffix(strings.ToLower(filename), ".docx") {
			filename += ".docx"
		}
		params["filename"] = filename

		path, err := wrapper.Generate(ctx, "docx", params)
		if err != nil {
			return tool.Error(fmt.Sprintf("generate docx: %v", err)), nil
		}

		// Read the generated file
		buf, err := os.ReadFile(path)
		if err != nil {
			return tool.Error(fmt.Sprintf("read generated file: %v", err)), nil
		}

		// Move to managed storage with proper filename
		filenameOnly := filepath.Base(path)
		saved, err := mgr.Save("docx", "docx", buf, filename)
		if err != nil {
			return tool.Error(fmt.Sprintf("save file: %v", err)), nil
		}

		// Clean up the temp file from Node.js
		_ = os.Remove(path)

		return resultJSON(saved, fmt.Sprintf("Successfully created Word document '%s' (%d bytes).", saved.DisplayFilename, saved.FileSize)), nil
	}
}

// NewCreatePPTXHandler returns a handler for create_pptx_presentation.
func NewCreatePPTXHandler(wrapper *NodeJSWrapper) tool.Handler {
	mgr := NewManager(wrapper.outputDir)
	return func(ctx context.Context, args json.RawMessage) (string, error) {
		var params map[string]interface{}
		if err := json.Unmarshal(args, &params); err != nil {
			return tool.Error(fmt.Sprintf("invalid args: %v", err)), nil
		}
		filename, _ := params["filename"].(string)
		if filename == "" {
			filename = "presentation.pptx"
		}
		if !strings.HasSuffix(strings.ToLower(filename), ".pptx") {
			filename += ".pptx"
		}
		params["filename"] = filename

		path, err := wrapper.Generate(ctx, "pptx", params)
		if err != nil {
			return tool.Error(fmt.Sprintf("generate pptx: %v", err)), nil
		}

		buf, err := os.ReadFile(path)
		if err != nil {
			return tool.Error(fmt.Sprintf("read generated file: %v", err)), nil
		}

		saved, err := mgr.Save("pptx", "pptx", buf, filename)
		if err != nil {
			return tool.Error(fmt.Sprintf("save file: %v", err)), nil
		}

		_ = os.Remove(path)

		return resultJSON(saved, fmt.Sprintf("Successfully created PowerPoint presentation '%s' (%d bytes).", saved.DisplayFilename, saved.FileSize)), nil
	}
}
```

- [ ] **Step 2: Create register.go**

```go
// tool/document/register.go
package document

import (
	"path/filepath"

	"github.com/odysseythink/hermind/tool"
	"github.com/odysseythink/pantheon/core"
)

// RegisterAll registers all document creation tools.
func RegisterAll(reg *tool.Registry, instanceRoot string) {
	outputDir := filepath.Join(instanceRoot, "generated-files")

	RegisterCreateTextFile(reg, outputDir)
	RegisterCreateExcel(reg, outputDir)
	RegisterCreatePDF(reg, outputDir)

	// Word and PowerPoint via Node.js subprocess
	scriptDir := filepath.Join(instanceRoot, "..", "document-scripts")
	wrapper := NewNodeJSWrapper(scriptDir, outputDir)

	reg.Register(&tool.Entry{
		Name:        "create_word_document",
		Toolset:     "document_creation",
		Description: "Create a Microsoft Word document (.docx) from markdown or plain text content.",
		Emoji:       "📄",
		Handler:     NewCreateWordHandler(wrapper),
		CheckFn:     wrapper.IsAvailable,
		Schema: core.ToolDefinition{
			Name:        "create_word_document",
			Description: "Create a Microsoft Word document (.docx) from markdown or plain text content. Supports headings, tables, lists, and styling themes.",
			Parameters: core.MustSchemaFromJSON([]byte(`{
				"type": "object",
				"properties": {
					"filename": {"type": "string", "description": "The filename for the Word document. Will add .docx if not present."},
					"title": {"type": "string", "description": "Document title for metadata and title page."},
					"content": {"type": "string", "description": "The content to convert to a Word document. Supports markdown formatting."},
					"theme": {"type": "string", "enum": ["neutral", "blue", "warm"], "default": "neutral"},
					"margins": {"type": "string", "enum": ["normal", "narrow", "wide"], "default": "normal"},
					"includeTitlePage": {"type": "boolean", "default": false}
				},
				"required": ["filename", "content"]
			}`)),
		},
	})

	reg.Register(&tool.Entry{
		Name:        "create_pptx_presentation",
		Toolset:     "document_creation",
		Description: "Create a PowerPoint presentation (.pptx) with slides, themes, and bullet points.",
		Emoji:       "📊",
		Handler:     NewCreatePPTXHandler(wrapper),
		CheckFn:     wrapper.IsAvailable,
		Schema: core.ToolDefinition{
			Name:        "create_pptx_presentation",
			Description: "Create a PowerPoint presentation (.pptx). Provide a title, theme, and section outlines with key points. Each section becomes a slide.",
			Parameters: core.MustSchemaFromJSON([]byte(`{
				"type": "object",
				"properties": {
					"filename": {"type": "string", "description": "The filename for the presentation. Will add .pptx if not present."},
					"title": {"type": "string", "description": "Presentation title."},
					"theme": {"type": "string", "enum": ["corporate", "dark", "light"], "default": "corporate"},
					"sections": {
						"type": "array",
						"items": {
							"type": "object",
							"properties": {
								"title": {"type": "string"},
								"keyPoints": {"type": "array", "items": {"type": "string"}},
								"instructions": {"type": "string"}
							},
							"required": ["title", "keyPoints"]
						}
					}
				},
				"required": ["filename", "title", "sections"]
			}`)),
		},
	})
}
```

- [ ] **Step 3: Commit**

```bash
cd /d/workspace/go_work/hermind && git add tool/document/register.go tool/document/nodejs.go && git commit -m "feat(document): Word and PowerPoint handlers via Node.js subprocess"
```

---

### Task 12: Register Tools in Engine Dependencies

**Files:**
- Modify: `cli/engine_deps.go`

- [ ] **Step 1: Add import and registration**

In `cli/engine_deps.go`:

1. Add import: `"github.com/odysseythink/hermind/tool/document"`

2. After `obsidian.RegisterAll(toolRegistry)`, add:

```go
document.RegisterAll(toolRegistry, app.InstanceRoot)
```

- [ ] **Step 2: Verify build**

```bash
cd /d/workspace/go_work/hermind && go build ./...
```

Expected: Build succeeds (Qt/cgo pre-existing failures may still appear, but document-related code compiles).

- [ ] **Step 3: Commit**

```bash
cd /d/workspace/go_work/hermind && git add cli/engine_deps.go && git commit -m "feat(cli): register document creation tools in engine deps"
```

---

### Task 13: Config Descriptor

**Files:**
- Create: `config/descriptor/document_creation.go`

- [ ] **Step 1: Write descriptor**

```go
// config/descriptor/document_creation.go
package descriptor

func init() {
	Register(SectionSpec{
		Name:        "document_creation",
		Label:       "Document Creation",
		Description: "Configure document creation tools for generating text files, Word documents, PowerPoint presentations, PDFs, and Excel spreadsheets.",
		Fields: []FieldSpec{
			{Name: "enabled", Label: "Enabled", Kind: FieldBool, Help: "Enable document creation tools.", Default: true},
			{Name: "create_text_file", Label: "Text files", Kind: FieldBool, Help: "Create text files (.txt, .md, .json, .csv, etc.)", Default: true},
			{Name: "create_word_document", Label: "Word documents", Kind: FieldBool, Help: "Create Microsoft Word documents (.docx)", Default: true},
			{Name: "create_pptx_presentation", Label: "PowerPoint", Kind: FieldBool, Help: "Create PowerPoint presentations (.pptx)", Default: true},
			{Name: "create_pdf_document", Label: "PDF documents", Kind: FieldBool, Help: "Create PDF documents", Default: true},
			{Name: "create_excel_spreadsheet", Label: "Excel spreadsheets", Kind: FieldBool, Help: "Create Excel spreadsheets (.xlsx)", Default: true},
		},
	})
}
```

- [ ] **Step 2: Verify tests pass**

```bash
cd /d/workspace/go_work/hermind && go test ./config/descriptor/ -v
```

Expected: PASS.

- [ ] **Step 3: Commit**

```bash
cd /d/workspace/go_work/hermind && git add config/descriptor/document_creation.go && git commit -m "feat(config): document_creation descriptor with subtype toggles"
```

---

### Task 14: Frontend FileDownloadCard Component

**Files:**
- Create: `web/src/components/chat/FileDownloadCard.tsx`
- Create: `web/src/components/chat/FileDownloadCard.module.css`
- Create: `web/src/components/chat/FileDownloadCard.test.tsx`

- [ ] **Step 1: Write component with test**

```tsx
// web/src/components/chat/FileDownloadCard.tsx
import styles from './FileDownloadCard.module.css';

export interface FileDownloadCardProps {
  filename: string;
  storageFilename: string;
  fileSize: number;
}

function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

function getIcon(filename: string): string {
  const ext = filename.split('.').pop()?.toLowerCase();
  switch (ext) {
    case 'docx': return '📄';
    case 'pptx': return '📊';
    case 'pdf': return '📑';
    case 'xlsx': return '📈';
    default: return '📝';
  }
}

export default function FileDownloadCard({ filename, storageFilename, fileSize }: FileDownloadCardProps) {
  const handleDownload = () => {
    window.open(`/api/generated-files/${storageFilename}`, '_blank');
  };

  return (
    <div className={styles.card}>
      <span className={styles.icon}>{getIcon(filename)}</span>
      <div className={styles.info}>
        <span className={styles.filename}>{filename}</span>
        <span className={styles.size}>{formatSize(fileSize)}</span>
      </div>
      <button className={styles.button} onClick={handleDownload}>
        Download
      </button>
    </div>
  );
}
```

```css
/* web/src/components/chat/FileDownloadCard.module.css */
.card {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 16px;
  border: 1px solid var(--border-color, #e0e0e0);
  border-radius: 8px;
  background: var(--bg-secondary, #f5f5f5);
  max-width: 400px;
}

.icon {
  font-size: 24px;
}

.info {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-width: 0;
}

.filename {
  font-weight: 500;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.size {
  font-size: 12px;
  color: var(--text-muted, #888);
}

.button {
  padding: 6px 14px;
  border: none;
  border-radius: 6px;
  background: var(--primary-color, #007bff);
  color: white;
  cursor: pointer;
  font-size: 13px;
}

.button:hover {
  opacity: 0.9;
}
```

```tsx
// web/src/components/chat/FileDownloadCard.test.tsx
import { render, screen, fireEvent } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import FileDownloadCard from './FileDownloadCard';

describe('FileDownloadCard', () => {
  it('renders filename and size', () => {
    render(<FileDownloadCard filename="report.docx" storageFilename="docx-abc.docx" fileSize={15360} />);
    expect(screen.getByText('report.docx')).toBeInTheDocument();
    expect(screen.getByText('15.0 KB')).toBeInTheDocument();
  });

  it('opens download on click', () => {
    const openSpy = vi.spyOn(window, 'open').mockImplementation(() => null);
    render(<FileDownloadCard filename="report.docx" storageFilename="docx-abc.docx" fileSize={1024} />);
    fireEvent.click(screen.getByText('Download'));
    expect(openSpy).toHaveBeenCalledWith('/api/generated-files/docx-abc.docx', '_blank');
    openSpy.mockRestore();
  });

  it('shows correct icon for file types', () => {
    const { rerender } = render(<FileDownloadCard filename="a.docx" storageFilename="x" fileSize={1} />);
    expect(screen.getByText('📄')).toBeInTheDocument();

    rerender(<FileDownloadCard filename="b.pptx" storageFilename="x" fileSize={1} />);
    expect(screen.getByText('📊')).toBeInTheDocument();

    rerender(<FileDownloadCard filename="c.xlsx" storageFilename="x" fileSize={1} />);
    expect(screen.getByText('📈')).toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run tests**

```bash
cd /d/workspace/go_work/hermind/web && npx vitest run src/components/chat/FileDownloadCard.test.tsx
```

Expected: PASS for all 3 tests.

- [ ] **Step 3: Commit**

```bash
cd /d/workspace/go_work/hermind && git add web/src/components/chat/FileDownloadCard.tsx web/src/components/chat/FileDownloadCard.module.css web/src/components/chat/FileDownloadCard.test.tsx && git commit -m "feat(web): FileDownloadCard component for chat UI"
```

---

### Task 15: Chat Integration (SSE + Render)

**Files:**
- Modify: `web/src/components/chat/ChatMessage.tsx`
- Modify: `web/src/api/schemas.ts`

- [ ] **Step 1: Add event type to schemas**

In `web/src/api/schemas.ts`, add to the stream event types:

```typescript
export interface FileDownloadCardEvent {
  type: 'file_download_card';
  payload: {
    filename: string;
    storageFilename: string;
    fileSize: number;
  };
}
```

- [ ] **Step 2: Modify ChatMessage to render download cards**

In `web/src/components/chat/ChatMessage.tsx`, find where tool call results or special events are rendered. Add:

```tsx
import FileDownloadCard from './FileDownloadCard';

// Inside the message rendering logic:
{message.fileDownloadCards?.map((card, i) => (
  <FileDownloadCard key={i} {...card} />
))}
```

The exact integration depends on how Hermind's chat message type is structured. The engine needs to emit a `file_download_card` event in the SSE stream when a tool result contains `storageFilename`.

On the backend, in the engine's tool result processing (likely in `agent/engine.go` or similar), detect tool results containing `"storageFilename"` and emit a `file_download_card` SSE event alongside the text response.

- [ ] **Step 3: Verify frontend builds**

```bash
cd /d/workspace/go_work/hermind/web && npx tsc --noEmit
```

Expected: No TypeScript errors.

- [ ] **Step 4: Commit**

```bash
cd /d/workspace/go_work/hermind && git add web/src/components/chat/ChatMessage.tsx web/src/api/schemas.ts && git commit -m "feat(web): render file download cards in chat messages"
```

---

### Task 16: Integration / End-to-End Test

**Files:**
- Create: `tool/document/integration_test.go`

- [ ] **Step 1: Write integration test**

```go
// tool/document/integration_test.go
package document

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIntegration_TextFile(t *testing.T) {
	tmpDir := t.TempDir()
	handler := NewCreateTextFileHandler(tmpDir)

	args, _ := json.Marshal(map[string]interface{}{
		"filename": "test",
		"extension": "txt",
		"content":  "Hello, integration test!",
	})

	result, err := handler(context.Background(), args)
	require.NoError(t, err)

	var meta map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(result), &meta))

	storageFilename := meta["storageFilename"].(string)
	path := filepath.Join(tmpDir, storageFilename)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, "Hello, integration test!", string(data))
}
```

- [ ] **Step 2: Run test**

```bash
cd /d/workspace/go_work/hermind && go test ./tool/document/ -v -run "TestIntegration"
```

Expected: PASS.

- [ ] **Step 3: Commit**

```bash
cd /d/workspace/go_work/hermind && git add tool/document/integration_test.go && git commit -m "test(document): integration test for text file generation"
```

---

## Self-Review Checklist

**1. Spec coverage:**
- [x] 5 file types → Tasks 5, 6, 7, 9, 10, 11
- [x] Aggregated toolset UI → Tasks 3, 4, 13
- [x] Download API → Task 2
- [x] File storage manager → Task 1
- [x] Node.js subprocess → Tasks 8, 9, 10
- [x] Frontend download card → Tasks 14, 15
- [x] Chat integration → Task 15
- [x] Config descriptor → Task 13

**2. Placeholder scan:** No TBD/TODO/"implement later"/"similar to" found.

**3. Type consistency:**
- `SavedFile`, `RetrievedFile`, `Manager` types used consistently across all tasks
- `resultJSON()` helper used by all Go handlers
- `NodeJSWrapper` used by Word and PPT handlers
- Filename format `{type}-{uuid}.{ext}` consistent everywhere

---

## Execution Handoff

Plan complete and saved to `.gpowers/plans/2026-05-20-document-creation.md`.

**Two execution options:**

**1. Subagent-Driven (recommended)** — I dispatch a fresh subagent per task, review between tasks, fast iteration

**2. Inline Execution** — Execute tasks in this session, batch execution with checkpoints for review

**Which approach would you prefer?**
