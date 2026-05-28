package services

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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
