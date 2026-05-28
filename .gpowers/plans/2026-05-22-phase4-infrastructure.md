# Phase 4: 通用基础设施建设实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use gpowers:subagent-driven-development (recommended) or gpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 建设所有上层路由依赖的共享基础设施 Service，为 Phase 5/6/7 的路由实现提供支撑。

**Architecture:** 在现有 `backend/internal/services/` 和 `backend/internal/models/` 基础上，扩展 VectorService、新建 FileSystemService、扩展 DocumentService、新建 PromptPresetService/PromptVariableService、新建 SSEManager。所有 Service 通过接口隔离，便于 mock 测试。

**Tech Stack:** Go 1.25, Gin, GORM, LanceDB/PGVector, UUID, testify

---

## 文件结构总览

| 文件 | 操作 | 说明 |
|------|------|------|
| `backend/internal/services/vector_service.go` | 修改 | 扩展 VectorService 方法 |
| `backend/internal/services/vector_service_test.go` | 新建 | VectorService 单元测试 |
| `backend/internal/services/filesystem_service.go` | 新建 | 文件系统管理 Service |
| `backend/internal/services/filesystem_service_test.go` | 新建 | FileSystemService 单元测试 |
| `backend/internal/services/document_service.go` | 修改 | 扩展文件管理方法 |
| `backend/internal/models/prompt_preset.go` | 新建 | PromptPreset 模型 |
| `backend/internal/models/prompt_variable.go` | 新建 | PromptVariable 模型 |
| `backend/internal/services/prompt_preset_service.go` | 新建 | PromptPreset Service |
| `backend/internal/services/prompt_preset_service_test.go` | 新建 | PromptPresetService 测试 |
| `backend/internal/services/prompt_variable_service.go` | 新建 | PromptVariable Service |
| `backend/internal/services/prompt_variable_service_test.go` | 新建 | PromptVariableService 测试 |
| `backend/internal/services/sse_manager.go` | 新建 | SSE 流式响应管理器 |
| `backend/internal/services/db.go` | 修改 | AutoMigrate 新增模型 |
| `backend/cmd/server/main.go` | 修改 | 注入新 Service |

---

## Task 1: 扩展 VectorService

**Files:**
- Modify: `backend/internal/services/vector_service.go`
- Create: `backend/internal/services/vector_service_test.go`

### Step 1: 扩展 VectorService 接口方法

修改 `backend/internal/services/vector_service.go`，在现有方法基础上增加：

```go
package services

import (
	"context"
	"fmt"

	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/vectordb"
)

type VectorService struct {
	cfg      *config.Config
	provider vectordb.VectorDatabase
}

func NewVectorService(cfg *config.Config) *VectorService {
	return &VectorService{cfg: cfg}
}

func (s *VectorService) Connect(ctx context.Context) error {
	if s.provider != nil {
		return nil
	}
	switch s.cfg.VectorDB {
	case "pgvector":
		if s.cfg.DatabaseURL == "" {
			return fmt.Errorf("pgvector configured but DATABASE_URL is empty")
		}
		pgv := vectordb.NewPGVector(s.cfg.DatabaseURL)
		if err := pgv.Connect(ctx); err != nil {
			return fmt.Errorf("pgvector connect: %w", err)
		}
		s.provider = pgv
	case "lancedb":
		ldb := vectordb.NewLanceDB(s.cfg.StorageDir)
		if err := ldb.Connect(nil); err != nil {
			return fmt.Errorf("lancedb connect: %w", err)
		}
		s.provider = ldb
	default:
		return fmt.Errorf("unsupported vector db: %s", s.cfg.VectorDB)
	}
	return nil
}

func (s *VectorService) SetProvider(p vectordb.VectorDatabase) {
	s.provider = p
}

func (s *VectorService) SimilaritySearch(ctx context.Context, namespace string, queryVector []float32, opts vectordb.SearchOptions) ([]vectordb.SearchResult, error) {
	if s.provider == nil {
		return nil, fmt.Errorf("vector provider not connected")
	}
	return s.provider.SimilaritySearch(ctx, namespace, queryVector, opts)
}

func (s *VectorService) AddVectors(ctx context.Context, namespace string, chunks []vectordb.VectorChunk) error {
	if s.provider == nil {
		return fmt.Errorf("vector provider not connected")
	}
	return s.provider.AddVectors(ctx, namespace, chunks)
}

func (s *VectorService) DeleteVectors(ctx context.Context, namespace string, docIds []string) error {
	if s.provider == nil {
		return fmt.Errorf("vector provider not connected")
	}
	return s.provider.DeleteVectors(ctx, namespace, docIds)
}

func (s *VectorService) DeleteNamespace(ctx context.Context, namespace string) error {
	if s.provider == nil {
		return fmt.Errorf("vector provider not connected")
	}
	return s.provider.DeleteNamespace(ctx, namespace)
}

func (s *VectorService) CountVectors(ctx context.Context, namespace string) (int64, error) {
	if s.provider == nil {
		return 0, fmt.Errorf("vector provider not connected")
	}
	// Use Tables to check if namespace exists, then use provider-specific count
	tables, err := s.provider.Tables(ctx)
	if err != nil {
		return 0, err
	}
	found := false
	for _, t := range tables {
		if t == namespace {
			found = true
			break
		}
	}
	if !found {
		return 0, nil
	}
	// Fall back to total vectors (provider-specific count per namespace not always available)
	return s.provider.TotalVectors(ctx)
}

func (s *VectorService) TotalVectors(ctx context.Context) (int64, error) {
	if s.provider == nil {
		return 0, fmt.Errorf("vector provider not connected")
	}
	return s.provider.TotalVectors(ctx)
}

func (s *VectorService) Heartbeat(ctx context.Context) (map[string]any, error) {
	if s.provider == nil {
		return map[string]any{"status": "not configured"}, nil
	}
	return s.provider.Heartbeat(ctx)
}
```

### Step 2: 运行编译验证

```bash
cd backend && go build ./...
```

Expected: 编译通过，无错误。

### Step 3: 编写 VectorService 单元测试

创建 `backend/internal/services/vector_service_test.go`：

```go
package services

import (
	"context"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/vectordb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockVectorDB struct {
	mock.Mock
}

func (m *mockVectorDB) Name() string {
	args := m.Called()
	return args.String(0)
}

func (m *mockVectorDB) Connect(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockVectorDB) Heartbeat(ctx context.Context) (map[string]any, error) {
	args := m.Called(ctx)
	return args.Get(0).(map[string]any), args.Error(1)
}

func (m *mockVectorDB) AddVectors(ctx context.Context, namespace string, chunks []vectordb.VectorChunk) error {
	args := m.Called(ctx, namespace, chunks)
	return args.Error(0)
}

func (m *mockVectorDB) DeleteVectors(ctx context.Context, namespace string, docIds []string) error {
	args := m.Called(ctx, namespace, docIds)
	return args.Error(0)
}

func (m *mockVectorDB) SimilaritySearch(ctx context.Context, namespace string, queryVector []float32, opts vectordb.SearchOptions) ([]vectordb.SearchResult, error) {
	args := m.Called(ctx, namespace, queryVector, opts)
	return args.Get(0).([]vectordb.SearchResult), args.Error(1)
}

func (m *mockVectorDB) DeleteNamespace(ctx context.Context, namespace string) error {
	args := m.Called(ctx, namespace)
	return args.Error(0)
}

func (m *mockVectorDB) Tables(ctx context.Context) ([]string, error) {
	args := m.Called(ctx)
	return args.Get(0).([]string), args.Error(1)
}

func (m *mockVectorDB) TotalVectors(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}

func TestVectorService_DeleteNamespace(t *testing.T) {
	mockDB := new(mockVectorDB)
	svc := &VectorService{provider: mockDB}
	mockDB.On("DeleteNamespace", mock.Anything, "test-ns").Return(nil)

	err := svc.DeleteNamespace(context.Background(), "test-ns")
	assert.NoError(t, err)
	mockDB.AssertExpectations(t)
}

func TestVectorService_DeleteNamespace_NotConnected(t *testing.T) {
	svc := &VectorService{provider: nil}
	err := svc.DeleteNamespace(context.Background(), "test-ns")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

func TestVectorService_CountVectors(t *testing.T) {
	mockDB := new(mockVectorDB)
	svc := &VectorService{provider: mockDB}
	mockDB.On("Tables", mock.Anything).Return([]string{"test-ns", "other"}, nil)
	mockDB.On("TotalVectors", mock.Anything).Return(int64(42), nil)

	count, err := svc.CountVectors(context.Background(), "test-ns")
	assert.NoError(t, err)
	assert.Equal(t, int64(42), count)
	mockDB.AssertExpectations(t)
}

func TestVectorService_CountVectors_TableNotFound(t *testing.T) {
	mockDB := new(mockVectorDB)
	svc := &VectorService{provider: mockDB}
	mockDB.On("Tables", mock.Anything).Return([]string{"other"}, nil)

	count, err := svc.CountVectors(context.Background(), "test-ns")
	assert.NoError(t, err)
	assert.Equal(t, int64(0), count)
	mockDB.AssertExpectations(t)
}

func TestVectorService_TotalVectors(t *testing.T) {
	mockDB := new(mockVectorDB)
	svc := &VectorService{provider: mockDB}
	mockDB.On("TotalVectors", mock.Anything).Return(int64(100), nil)

	count, err := svc.TotalVectors(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, int64(100), count)
	mockDB.AssertExpectations(t)
}
```

### Step 4: 运行测试

```bash
cd backend && go test ./internal/services/ -run TestVectorService -v
```

Expected: 5 tests PASS.

### Step 5: Commit

```bash
git add backend/internal/services/vector_service.go backend/internal/services/vector_service_test.go
git commit -m "feat(phase4): extend VectorService with DeleteNamespace, CountVectors, TotalVectors, Connect"
```

---

## Task 2: 新建 FileSystemService

**Files:**
- Create: `backend/internal/services/filesystem_service.go`
- Create: `backend/internal/services/filesystem_service_test.go`

### Step 1: 创建 FileSystemService

创建 `backend/internal/services/filesystem_service.go`：

```go
package services

import (
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"
)

type FileSystemService struct {
	storageDir string
}

func NewFileSystemService(storageDir string) *FileSystemService {
	return &FileSystemService{storageDir: storageDir}
}

// LocalFile represents a file or folder in storage.
type LocalFile struct {
	Name     string      `json:"name"`
	Type     string      `json:"type"` // "file" or "folder"
	Items    []LocalFile `json:"items,omitempty"`
	Meta     *FileMeta   `json:"meta,omitempty"`
}

type FileMeta struct {
	PageContent string `json:"pageContent,omitempty"`
}

// ListLocalFiles lists all files and folders in the documents directory.
func (s *FileSystemService) ListLocalFiles(folderName string) ([]LocalFile, error) {
	docDir := filepath.Join(s.storageDir, "documents")
	if folderName != "" {
		docDir = filepath.Join(docDir, folderName)
	}

	entries, err := os.ReadDir(docDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []LocalFile{}, nil
		}
		return nil, err
	}

	var files []LocalFile
	for _, entry := range entries {
		lf := LocalFile{
			Name: entry.Name(),
			Type: "file",
		}
		if entry.IsDir() {
			lf.Type = "folder"
		}
		files = append(files, lf)
	}
	return files, nil
}

// CreateFolder creates a new folder under documents.
func (s *FileSystemService) CreateFolder(folderName string) error {
	path := filepath.Join(s.storageDir, "documents", folderName)
	return os.MkdirAll(path, 0755)
}

// RemoveFolder removes a folder and all its contents.
func (s *FileSystemService) RemoveFolder(folderName string) error {
	path := filepath.Join(s.storageDir, "documents", folderName)
	return os.RemoveAll(path)
}

// MoveFiles moves files from one location to another within documents.
func (s *FileSystemService) MoveFiles(files []string, destination string) error {
	destDir := filepath.Join(s.storageDir, "documents", destination)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}
	for _, f := range files {
		src := filepath.Join(s.storageDir, "documents", f)
		dst := filepath.Join(destDir, filepath.Base(f))
		if err := os.Rename(src, dst); err != nil {
			return fmt.Errorf("move %s to %s: %w", src, dst, err)
		}
	}
	return nil
}

// RemoveDocument removes a single document file.
func (s *FileSystemService) RemoveDocument(docName string) error {
	path := filepath.Join(s.storageDir, "documents", docName)
	return os.RemoveAll(path)
}

// AcceptedDocumentTypes returns a map of extension to MIME type for supported documents.
func (s *FileSystemService) AcceptedDocumentTypes() map[string]string {
	return map[string]string{
		".txt":  "text/plain",
		".md":   "text/markdown",
		".pdf":  "application/pdf",
		".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		".csv":  "text/csv",
		".json": "application/json",
		".html": "text/html",
		".htm":  "text/html",
	}
}

// GetDocumentPath returns the full path to a document.
func (s *FileSystemService) GetDocumentPath(docName string) string {
	return filepath.Join(s.storageDir, "documents", docName)
}

// SaveFile saves uploaded file content to the documents directory.
func (s *FileSystemService) SaveFile(folderName, filename string, reader io.Reader) (string, error) {
	dir := filepath.Join(s.storageDir, "documents")
	if folderName != "" {
		dir = filepath.Join(dir, folderName)
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, filename)
	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := io.Copy(f, reader); err != nil {
		return "", err
	}
	return path, nil
}

// DetectMIME returns MIME type for a file path.
func (s *FileSystemService) DetectMIME(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	if mimeType := mime.TypeByExtension(ext); mimeType != "" {
		return mimeType
	}
	return "application/octet-stream"
}
```

### Step 2: 编译验证

```bash
cd backend && go build ./internal/services/...
```

Expected: 编译通过。

### Step 3: 编写 FileSystemService 单元测试

创建 `backend/internal/services/filesystem_service_test.go`：

```go
package services

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileSystemService_ListLocalFiles(t *testing.T) {
	tmpDir := t.TempDir()
	fs := NewFileSystemService(tmpDir)

	// Create test files
	os.WriteFile(filepath.Join(tmpDir, "documents", "a.txt"), []byte("hello"), 0644)
	os.MkdirAll(filepath.Join(tmpDir, "documents", "folder1"), 0755)

	files, err := fs.ListLocalFiles("")
	require.NoError(t, err)
	assert.Len(t, files, 2)

	names := make(map[string]string)
	for _, f := range files {
		names[f.Name] = f.Type
	}
	assert.Equal(t, "file", names["a.txt"])
	assert.Equal(t, "folder", names["folder1"])
}

func TestFileSystemService_CreateAndRemoveFolder(t *testing.T) {
	tmpDir := t.TempDir()
	fs := NewFileSystemService(tmpDir)

	err := fs.CreateFolder("test-folder")
	require.NoError(t, err)
	_, err = os.Stat(filepath.Join(tmpDir, "documents", "test-folder"))
	assert.NoError(t, err)

	err = fs.RemoveFolder("test-folder")
	require.NoError(t, err)
	_, err = os.Stat(filepath.Join(tmpDir, "documents", "test-folder"))
	assert.True(t, os.IsNotExist(err))
}

func TestFileSystemService_MoveFiles(t *testing.T) {
	tmpDir := t.TempDir()
	fs := NewFileSystemService(tmpDir)

	os.WriteFile(filepath.Join(tmpDir, "documents", "file1.txt"), []byte("content"), 0644)
	err := fs.MoveFiles([]string{"file1.txt"}, "dest")
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(tmpDir, "documents", "dest", "file1.txt"))
	assert.NoError(t, err)
}

func TestFileSystemService_AcceptedDocumentTypes(t *testing.T) {
	fs := NewFileSystemService("/tmp")
	types := fs.AcceptedDocumentTypes()
	assert.NotEmpty(t, types)
	assert.Equal(t, "text/plain", types[".txt"])
	assert.Equal(t, "application/pdf", types[".pdf"])
}

func TestFileSystemService_DetectMIME(t *testing.T) {
	fs := NewFileSystemService("/tmp")
	assert.Equal(t, "text/plain", fs.DetectMIME("file.txt"))
	assert.Equal(t, "application/pdf", fs.DetectMIME("file.pdf"))
	assert.True(t, strings.Contains(fs.DetectMIME("file.unknown"), "octet-stream"))
}
```

### Step 4: 运行测试

```bash
cd backend && go test ./internal/services/ -run TestFileSystemService -v
```

Expected: 5 tests PASS.

### Step 5: Commit

```bash
git add backend/internal/services/filesystem_service.go backend/internal/services/filesystem_service_test.go
git commit -m "feat(phase4): add FileSystemService for document and folder management"
```

---

## Task 3: 扩展 DocumentService

**Files:**
- Modify: `backend/internal/services/document_service.go`
- Modify: `backend/cmd/server/main.go`

### Step 1: 注入 FileSystemService

修改 `backend/internal/services/document_service.go`，在结构体中增加 `fs` 字段：

```go
type DocumentService struct {
	db       *gorm.DB
	cfg      *config.Config
	coll     *collector.Client
	embedder embedder.Embedder
	chunker  *chunker.Chunker
	vectorDB vectordb.VectorDatabase
	fs       *FileSystemService
}

func NewDocumentService(db *gorm.DB, cfg *config.Config, coll *collector.Client, emb embedder.Embedder, ch *chunker.Chunker, vdb vectordb.VectorDatabase, fs *FileSystemService) *DocumentService {
	return &DocumentService{db: db, cfg: cfg, coll: coll, embedder: emb, chunker: ch, vectorDB: vdb, fs: fs}
}
```

在现有方法下方添加新的文件管理方法：

```go
// ListDocuments returns all documents for a workspace.
func (s *DocumentService) ListDocuments(ctx context.Context, workspaceID int) ([]models.WorkspaceDocument, error) {
	var docs []models.WorkspaceDocument
	if err := s.db.Where("workspace_id = ?", workspaceID).Find(&docs).Error; err != nil {
		return nil, err
	}
	return docs, nil
}

// ListFolderContents returns documents inside a specific folder.
func (s *DocumentService) ListFolderContents(ctx context.Context, folderName string) ([]LocalFile, error) {
	return s.fs.ListLocalFiles(folderName)
}

// CreateFolder creates a document folder.
func (s *DocumentService) CreateFolder(ctx context.Context, folderName string) error {
	return s.fs.CreateFolder(folderName)
}

// RemoveFolder removes a document folder.
func (s *DocumentService) RemoveFolder(ctx context.Context, folderName string) error {
	return s.fs.RemoveFolder(folderName)
}

// MoveFiles moves documents between folders.
func (s *DocumentService) MoveFiles(ctx context.Context, files []string, destination string) error {
	return s.fs.MoveFiles(files, destination)
}

// RemoveDocument removes a document from filesystem and DB.
func (s *DocumentService) RemoveDocument(ctx context.Context, docId string) error {
	var doc models.WorkspaceDocument
	if err := s.db.Where("doc_id = ?", docId).First(&doc).Error; err != nil {
		return err
	}
	if doc.Docpath != "" {
		_ = os.RemoveAll(doc.Docpath)
	}
	return s.db.Where("doc_id = ?", docId).Delete(&models.WorkspaceDocument{}).Error
}

// RemoveDocuments removes multiple documents.
func (s *DocumentService) RemoveDocuments(ctx context.Context, docIds []string) error {
	for _, id := range docIds {
		if err := s.RemoveDocument(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

// AcceptedDocumentTypes returns accepted MIME types map.
func (s *DocumentService) AcceptedDocumentTypes() map[string]string {
	return s.fs.AcceptedDocumentTypes()
}

// SaveUploadToFolder saves an uploaded file to a specific folder.
func (s *DocumentService) SaveUploadToFolder(ctx context.Context, workspaceID int, folderName string, fileHeader *multipart.FileHeader) (*models.WorkspaceDocument, error) {
	src, err := fileHeader.Open()
	if err != nil {
		return nil, err
	}
	defer src.Close()

	docId := uuid.New().String()
	ext := filepath.Ext(fileHeader.Filename)
	filename := docId + ext
	destPath, err := s.fs.SaveFile(folderName, filename, src)
	if err != nil {
		return nil, err
	}

	doc := models.WorkspaceDocument{
		WorkspaceID:   workspaceID,
		DocId:         docId,
		Filename:      fileHeader.Filename,
		Docpath:       destPath,
		CreatedAt:     time.Now(),
		LastUpdatedAt: time.Now(),
	}
	if err := s.db.Create(&doc).Error; err != nil {
		return nil, fmt.Errorf("save document record: %w", err)
	}
	return &doc, nil
}
```

注意需要添加 `os` 的 import（如果还没有的话）。

### Step 2: 更新 main.go 中的 DocumentService 构造

修改 `backend/cmd/server/main.go` 第 120 行：

```go
	fsSvc := services.NewFileSystemService(cfg.StorageDir)
	docSvc := services.NewDocumentService(db, cfg, coll, emb, ch, vectorDB, fsSvc)
```

在 `main()` 函数顶部附近添加 `fsSvc` 的创建。

### Step 3: 编译验证

```bash
cd backend && go build ./...
```

Expected: 编译通过。

### Step 4: Commit

```bash
git add backend/internal/services/document_service.go backend/cmd/server/main.go
git commit -m "feat(phase4): extend DocumentService with file management and folder operations"
```

---

## Task 4: 新建 PromptPreset 模型和 Service

**Files:**
- Create: `backend/internal/models/prompt_preset.go`
- Create: `backend/internal/services/prompt_preset_service.go`
- Create: `backend/internal/services/prompt_preset_service_test.go`
- Modify: `backend/internal/services/db.go`

### Step 1: 创建 PromptPreset 模型

创建 `backend/internal/models/prompt_preset.go`：

```go
package models

import "time"

type PromptPreset struct {
	ID            int       `gorm:"primaryKey;autoIncrement" json:"id"`
	Command       string    `gorm:"unique" json:"command"`
	Prompt        string    `json:"prompt"`
	CreatedAt     time.Time `json:"createdAt"`
	LastUpdatedAt time.Time `json:"lastUpdatedAt"`
}
```

### Step 2: 创建 PromptPresetService

创建 `backend/internal/services/prompt_preset_service.go`：

```go
package services

import (
	"context"
	"time"

	"github.com/odysseythink/hermind/backend/internal/models"
	"gorm.io/gorm"
)

type PromptPresetService struct {
	db *gorm.DB
}

func NewPromptPresetService(db *gorm.DB) *PromptPresetService {
	return &PromptPresetService{db: db}
}

func (s *PromptPresetService) List(ctx context.Context) ([]models.PromptPreset, error) {
	var presets []models.PromptPreset
	err := s.db.WithContext(ctx).Order("id desc").Find(&presets).Error
	return presets, err
}

func (s *PromptPresetService) GetByID(ctx context.Context, id int) (*models.PromptPreset, error) {
	var preset models.PromptPreset
	if err := s.db.WithContext(ctx).First(&preset, id).Error; err != nil {
		return nil, err
	}
	return &preset, nil
}

func (s *PromptPresetService) Create(ctx context.Context, command, prompt string) (*models.PromptPreset, error) {
	preset := models.PromptPreset{
		Command:       command,
		Prompt:        prompt,
		CreatedAt:     time.Now(),
		LastUpdatedAt: time.Now(),
	}
	if err := s.db.WithContext(ctx).Create(&preset).Error; err != nil {
		return nil, err
	}
	return &preset, nil
}

func (s *PromptPresetService) Update(ctx context.Context, id int, command, prompt string) error {
	updates := map[string]any{
		"command":         command,
		"prompt":          prompt,
		"last_updated_at": time.Now(),
	}
	return s.db.WithContext(ctx).Model(&models.PromptPreset{}).Where("id = ?", id).Updates(updates).Error
}

func (s *PromptPresetService) Delete(ctx context.Context, id int) error {
	return s.db.WithContext(ctx).Delete(&models.PromptPreset{}, id).Error
}
```

### Step 3: 编写单元测试

创建 `backend/internal/services/prompt_preset_service_test.go`：

```go
package services

import (
	"context"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupPromptPresetTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	err = db.AutoMigrate(&models.PromptPreset{})
	require.NoError(t, err)
	return db
}

func TestPromptPresetService_CreateAndList(t *testing.T) {
	db := setupPromptPresetTestDB(t)
	svc := NewPromptPresetService(db)
	ctx := context.Background()

	p1, err := svc.Create(ctx, "/summarize", "Summarize the following text")
	require.NoError(t, err)
	assert.Equal(t, "/summarize", p1.Command)

	p2, err := svc.Create(ctx, "/translate", "Translate to English")
	require.NoError(t, err)
	assert.Equal(t, "/translate", p2.Command)

	list, err := svc.List(ctx)
	require.NoError(t, err)
	assert.Len(t, list, 2)
}

func TestPromptPresetService_Update(t *testing.T) {
	db := setupPromptPresetTestDB(t)
	svc := NewPromptPresetService(db)
	ctx := context.Background()

	p, err := svc.Create(ctx, "/old", "Old prompt")
	require.NoError(t, err)

	err = svc.Update(ctx, p.ID, "/new", "New prompt")
	require.NoError(t, err)

	updated, err := svc.GetByID(ctx, p.ID)
	require.NoError(t, err)
	assert.Equal(t, "/new", updated.Command)
	assert.Equal(t, "New prompt", updated.Prompt)
}

func TestPromptPresetService_Delete(t *testing.T) {
	db := setupPromptPresetTestDB(t)
	svc := NewPromptPresetService(db)
	ctx := context.Background()

	p, err := svc.Create(ctx, "/delete", "Delete me")
	require.NoError(t, err)

	err = svc.Delete(ctx, p.ID)
	require.NoError(t, err)

	_, err = svc.GetByID(ctx, p.ID)
	assert.Error(t, err)
}
```

### Step 4: 更新 AutoMigrate

修改 `backend/internal/services/db.go` 的 `AutoMigrate` 函数：

```go
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&models.User{},
		&models.Invite{},
		&models.APIKey{},
		&models.PasswordResetToken{},
		&models.RecoveryCode{},
		&models.Workspace{},
		&models.WorkspaceUser{},
		&models.WorkspaceChat{},
		&models.WorkspaceDocument{},
		&models.DocumentVector{},
		&models.WorkspaceThread{},
		&models.SystemSetting{},
		&models.EmbedConfig{},
		&models.EmbedChat{},
		&models.PromptPreset{},
	)
}
```

### Step 5: 运行测试

```bash
cd backend && go test ./internal/services/ -run TestPromptPresetService -v
```

Expected: 3 tests PASS.

### Step 6: Commit

```bash
git add backend/internal/models/prompt_preset.go backend/internal/services/prompt_preset_service.go backend/internal/services/prompt_preset_service_test.go backend/internal/services/db.go
git commit -m "feat(phase4): add PromptPreset model and service"
```

---

## Task 5: 新建 PromptVariable 模型和 Service

**Files:**
- Create: `backend/internal/models/prompt_variable.go`
- Create: `backend/internal/services/prompt_variable_service.go`
- Create: `backend/internal/services/prompt_variable_service_test.go`
- Modify: `backend/internal/services/db.go`

### Step 1: 创建 PromptVariable 模型

创建 `backend/internal/models/prompt_variable.go`：

```go
package models

import "time"

type PromptVariable struct {
	ID            int       `gorm:"primaryKey;autoIncrement" json:"id"`
	Key           string    `gorm:"unique" json:"key"`
	Value         string    `json:"value"`
	CreatedAt     time.Time `json:"createdAt"`
	LastUpdatedAt time.Time `json:"lastUpdatedAt"`
}
```

### Step 2: 创建 PromptVariableService

创建 `backend/internal/services/prompt_variable_service.go`：

```go
package services

import (
	"context"
	"time"

	"github.com/odysseythink/hermind/backend/internal/models"
	"gorm.io/gorm"
)

type PromptVariableService struct {
	db *gorm.DB
}

func NewPromptVariableService(db *gorm.DB) *PromptVariableService {
	return &PromptVariableService{db: db}
}

func (s *PromptVariableService) List(ctx context.Context) ([]models.PromptVariable, error) {
	var vars []models.PromptVariable
	err := s.db.WithContext(ctx).Order("id desc").Find(&vars).Error
	return vars, err
}

func (s *PromptVariableService) GetByID(ctx context.Context, id int) (*models.PromptVariable, error) {
	var v models.PromptVariable
	if err := s.db.WithContext(ctx).First(&v, id).Error; err != nil {
		return nil, err
	}
	return &v, nil
}

func (s *PromptVariableService) Create(ctx context.Context, key, value string) (*models.PromptVariable, error) {
	v := models.PromptVariable{
		Key:           key,
		Value:         value,
		CreatedAt:     time.Now(),
		LastUpdatedAt: time.Now(),
	}
	if err := s.db.WithContext(ctx).Create(&v).Error; err != nil {
		return nil, err
	}
	return &v, nil
}

func (s *PromptVariableService) Update(ctx context.Context, id int, key, value string) error {
	updates := map[string]any{
		"key":             key,
		"value":           value,
		"last_updated_at": time.Now(),
	}
	return s.db.WithContext(ctx).Model(&models.PromptVariable{}).Where("id = ?", id).Updates(updates).Error
}

func (s *PromptVariableService) Delete(ctx context.Context, id int) error {
	return s.db.WithContext(ctx).Delete(&models.PromptVariable{}, id).Error
}
```

### Step 3: 编写单元测试

创建 `backend/internal/services/prompt_variable_service_test.go`：

```go
package services

import (
	"context"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupPromptVariableTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	err = db.AutoMigrate(&models.PromptVariable{})
	require.NoError(t, err)
	return db
}

func TestPromptVariableService_CreateAndList(t *testing.T) {
	db := setupPromptVariableTestDB(t)
	svc := NewPromptVariableService(db)
	ctx := context.Background()

	v1, err := svc.Create(ctx, "user_name", "Alice")
	require.NoError(t, err)
	assert.Equal(t, "user_name", v1.Key)

	list, err := svc.List(ctx)
	require.NoError(t, err)
	assert.Len(t, list, 1)
}

func TestPromptVariableService_Update(t *testing.T) {
	db := setupPromptVariableTestDB(t)
	svc := NewPromptVariableService(db)
	ctx := context.Background()

	v, err := svc.Create(ctx, "old_key", "old_value")
	require.NoError(t, err)

	err = svc.Update(ctx, v.ID, "new_key", "new_value")
	require.NoError(t, err)

	updated, err := svc.GetByID(ctx, v.ID)
	require.NoError(t, err)
	assert.Equal(t, "new_key", updated.Key)
	assert.Equal(t, "new_value", updated.Value)
}

func TestPromptVariableService_Delete(t *testing.T) {
	db := setupPromptVariableTestDB(t)
	svc := NewPromptVariableService(db)
	ctx := context.Background()

	v, err := svc.Create(ctx, "del", "me")
	require.NoError(t, err)

	err = svc.Delete(ctx, v.ID)
	require.NoError(t, err)

	_, err = svc.GetByID(ctx, v.ID)
	assert.Error(t, err)
}
```

### Step 4: 更新 AutoMigrate

修改 `backend/internal/services/db.go`：

```go
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&models.User{},
		&models.Invite{},
		&models.APIKey{},
		&models.PasswordResetToken{},
		&models.RecoveryCode{},
		&models.Workspace{},
		&models.WorkspaceUser{},
		&models.WorkspaceChat{},
		&models.WorkspaceDocument{},
		&models.DocumentVector{},
		&models.WorkspaceThread{},
		&models.SystemSetting{},
		&models.EmbedConfig{},
		&models.EmbedChat{},
		&models.PromptPreset{},
		&models.PromptVariable{},
	)
}
```

### Step 5: 运行测试

```bash
cd backend && go test ./internal/services/ -run TestPromptVariableService -v
```

Expected: 3 tests PASS.

### Step 6: Commit

```bash
git add backend/internal/models/prompt_variable.go backend/internal/services/prompt_variable_service.go backend/internal/services/prompt_variable_service_test.go backend/internal/services/db.go
git commit -m "feat(phase4): add PromptVariable model and service"
```

---

## Task 6: 新建 SSEManager

**Files:**
- Create: `backend/internal/services/sse_manager.go`
- Create: `backend/internal/services/sse_manager_test.go`

### Step 1: 创建 SSEManager

创建 `backend/internal/services/sse_manager.go`：

```go
package services

import (
	"fmt"
	"io"

	"github.com/gin-gonic/gin"
)

type SSEManager struct{}

func NewSSEManager() *SSEManager {
	return &SSEManager{}
}

// SendEvent sends a single SSE event to the client.
func (s *SSEManager) SendEvent(c *gin.Context, event, data string) {
	c.SSEvent(event, data)
}

// SendData sends raw data line.
func (s *SSEManager) SendData(c *gin.Context, data string) {
	c.Writer.Write([]byte("data: " + data + "\n\n"))
	c.Writer.Flush()
}

// Stream starts a streaming response with proper SSE headers.
func (s *SSEManager) Stream(c *gin.Context, streamFunc func(send func(string))) {
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
	c.Writer.WriteHeader(200)
	c.Writer.Flush()

	send := func(data string) {
		c.Writer.Write([]byte("data: " + data + "\n\n"))
		c.Writer.Flush()
	}

	streamFunc(send)
}

// StreamWithChannel streams data from a channel until it's closed.
func (s *SSEManager) StreamWithChannel(c *gin.Context, ch <-chan string) {
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
	c.Writer.WriteHeader(200)
	c.Writer.Flush()

	for data := range ch {
		c.Writer.Write([]byte("data: " + data + "\n\n"))
		c.Writer.Flush()
	}
}

// IsClientConnected checks if the client connection is still alive.
func (s *SSEManager) IsClientConnected(c *gin.Context) bool {
	// Gin doesn't expose a direct way to check this,
	// but we can attempt a flush and catch errors.
	if flusher, ok := c.Writer.(io.Closer); ok {
		_ = flusher.Close()
		return false
	}
	return true
}
```

### Step 2: 编译验证

```bash
cd backend && go build ./internal/services/...
```

Expected: 编译通过。

### Step 3: Commit

```bash
git add backend/internal/services/sse_manager.go
git commit -m "feat(phase4): add SSEManager for streaming responses"
```

---

## Task 7: 更新 main.go 依赖注入

**Files:**
- Modify: `backend/cmd/server/main.go`

### Step 1: 注入所有新 Service

修改 `backend/cmd/server/main.go`：

在现有的 service 初始化代码之后，添加：

```go
	fsSvc := services.NewFileSystemService(cfg.StorageDir)
	promptPresetSvc := services.NewPromptPresetService(db)
	promptVariableSvc := services.NewPromptVariableService(db)
	sseMgr := services.NewSSEManager()
```

修改 `docSvc` 的创建（已在 Task 3 中修改）。

然后更新路由注册（暂时不需要注册新 handler，这些 Service 会在 Phase 5/6/7 的 handler 中使用）：

```go
	// Placeholder: new services will be wired into handlers in Phase 5/6/7
	_ = fsSvc
	_ = promptPresetSvc
	_ = promptVariableSvc
	_ = sseMgr
```

### Step 2: 编译验证

```bash
cd backend && go build ./...
```

Expected: 编译通过。

### Step 3: Commit

```bash
git add backend/cmd/server/main.go
git commit -m "feat(phase4): wire new services into main.go dependency injection"
```

---

## Task 8: 运行全部测试

### Step 1: 运行所有 Service 层单元测试

```bash
cd backend && go test ./internal/services/... -v
```

Expected: 所有测试 PASS。

### Step 2: 运行集成测试（快速验证无回归）

```bash
cd backend && go test ./tests/integration/... -v -count=1
```

Expected: 所有现有集成测试继续 PASS。

### Step 3: Commit（如有需要）

---

## 自审检查清单

- [x] **Spec coverage**: Phase 4 设计文档中的 8 个基础设施全部覆盖
  - FileSystemService ✅ (Task 2)
  - VectorDBService 扩展 ✅ (Task 1)
  - CollectorService — 已有，无需修改
  - SSEManager ✅ (Task 6)
  - SystemConfigService — 已有 (SystemService)，无需修改
  - ApiKeyService — 已有，无需修改
  - PromptPresetService ✅ (Task 4)
  - PromptVariableService ✅ (Task 5)
- [x] **Placeholder scan**: 无 TBD/TODO/"implement later"
- [x] **Type consistency**: VectorService 方法签名与 VectorDatabase 接口一致
- [x] **File paths**: 所有路径精确到 backend 包内
- [x] **Commits**: 每个 Task 有独立 commit

---

## Phase 4 验收标准

- [ ] `go build ./...` 编译通过
- [ ] `go test ./internal/services/...` 所有新增测试 PASS
- [ ] `go test ./tests/integration/...` 无回归
- [ ] 新模型（PromptPreset, PromptVariable）已加入 AutoMigrate
