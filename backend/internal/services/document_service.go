package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/odysseythink/hermind/backend/internal/chunker"
	"github.com/odysseythink/hermind/backend/internal/collector"
	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/dto"
	"github.com/odysseythink/hermind/backend/internal/embedder"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/vectordb"
	"github.com/odysseythink/hermind/backend/pkg/utils"
	"github.com/odysseythink/mlog"
	"gorm.io/gorm"
)

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

func (s *DocumentService) SaveUpload(ctx context.Context, workspaceID int, fileHeader *multipart.FileHeader) (*models.WorkspaceDocument, error) {
	src, err := fileHeader.Open()
	if err != nil {
		return nil, err
	}
	defer src.Close()

	docId := uuid.New().String()
	ext := filepath.Ext(fileHeader.Filename)
	destPath := filepath.Join(s.cfg.StorageDir, "documents", docId+ext)
	os.MkdirAll(filepath.Dir(destPath), 0755)

	dest, err := os.Create(destPath)
	if err != nil {
		return nil, err
	}
	defer dest.Close()

	if _, err := io.Copy(dest, src); err != nil {
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

	if s.coll != nil {
		resp, err := s.coll.ProcessDocument(ctx, destPath, nil)
		if err == nil && resp.Success && len(resp.Documents) > 0 {
			d := resp.Documents[0]
			meta := map[string]interface{}{
				"title":              d.Title,
				"docAuthor":          d.DocAuthor,
				"description":        d.Description,
				"docSource":          d.DocSource,
				"chunkSource":        d.ChunkSource,
				"published":          d.Published,
				"wordCount":          d.WordCount,
				"tokenCountEstimate": d.TokenCountEstimate,
			}
			metaJSON, _ := json.Marshal(meta)
			metaStr := string(metaJSON)
			doc.Metadata = &metaStr
			// Store extracted text from collector if available for embedding pipeline
			if len(resp.Documents) > 0 && resp.Documents[0].Title != "" {
				textMeta := map[string]interface{}{"extractedTitle": resp.Documents[0].Title}
				if doc.Metadata != nil {
					var existing map[string]interface{}
					json.Unmarshal([]byte(*doc.Metadata), &existing)
					for k, v := range textMeta {
						existing[k] = v
					}
					merged, _ := json.Marshal(existing)
					m := string(merged)
					doc.Metadata = &m
				}
			}
			s.db.Save(&doc)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "collector process document failed: %v\n", err)
		}
	}

	if s.embedder != nil && s.vectorDB != nil {
		bgCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		go func(docRef models.WorkspaceDocument) {
			if err := s.EmbedDocument(bgCtx, &docRef); err != nil {
				mlog.Error("embed document failed: ", mlog.Err(err))
			}
		}(doc)
	}

	return &doc, nil
}

func (s *DocumentService) EmbedDocument(ctx context.Context, doc *models.WorkspaceDocument) error {
	content := ""
	// Try to read the file; for binary formats (PDF, images) the Collector should
	// have extracted text. If available in metadata, use that instead.
	if doc.Metadata != nil {
		var meta map[string]interface{}
		if err := json.Unmarshal([]byte(*doc.Metadata), &meta); err == nil {
			if extracted, ok := meta["extractedText"].(string); ok && extracted != "" {
				content = extracted
			}
		}
	}
	if content == "" {
		data, err := os.ReadFile(doc.Docpath)
		if err != nil {
			return fmt.Errorf("read document file: %w", err)
		}
		content = string(data)
	}
	if content == "" {
		return nil
	}

	chunks := s.chunker.Split(content)
	if len(chunks) == 0 {
		return nil
	}

	embeddings, err := s.embedder.EmbedTexts(ctx, chunks)
	if err != nil {
		return fmt.Errorf("embed: %w", err)
	}
	if len(embeddings) != len(chunks) {
		return fmt.Errorf("embedding count mismatch")
	}

	var vectorChunks []vectordb.VectorChunk
	var docVectors []models.DocumentVector
	for i, text := range chunks {
		vid := uuid.New().String()
		vectorChunks = append(vectorChunks, vectordb.VectorChunk{
			ID:     vid,
			Vector: embeddings[i],
			Metadata: map[string]any{
				"docId": doc.DocId,
				"text":  text,
				"title": doc.Filename,
			},
		})
		docVectors = append(docVectors, models.DocumentVector{
			DocId:    doc.DocId,
			VectorId: vid,
		})
	}

	var ws models.Workspace
	if err := s.db.First(&ws, doc.WorkspaceID).Error; err != nil {
		return err
	}

	if err := s.vectorDB.AddVectors(ctx, ws.Slug, vectorChunks); err != nil {
		return fmt.Errorf("add vectors: %w", err)
	}

	if len(docVectors) > 0 {
		if err := s.db.CreateInBatches(docVectors, 100).Error; err != nil {
			return fmt.Errorf("save doc vectors: %w", err)
		}
	}

	return nil
}

func (s *DocumentService) GetByDocId(ctx context.Context, docId string) (*models.WorkspaceDocument, error) {
	var doc models.WorkspaceDocument
	if err := s.db.Where("doc_id = ?", docId).First(&doc).Error; err != nil {
		return nil, err
	}
	return &doc, nil
}

func (s *DocumentService) DeleteByDocId(ctx context.Context, docId string) error {
	return s.db.Where("doc_id = ?", docId).Delete(&models.WorkspaceDocument{}).Error
}

func (s *DocumentService) GetWorkspaceBySlug(ctx context.Context, slug string, ws *models.Workspace) error {
	return s.db.Where("slug = ?", slug).First(ws).Error
}

// CreateFolder creates a folder under documentsPath
func (s *DocumentService) CreateFolder(ctx context.Context, name string) error {
	storagePath := filepath.Join(s.cfg.StorageDir, "documents", name)
	// Prevent directory traversal — ensure path is within documents dir
	docDir := filepath.Join(s.cfg.StorageDir, "documents")
	base := filepath.Clean(docDir) + string(filepath.Separator)
	if !strings.HasPrefix(filepath.Clean(storagePath)+string(filepath.Separator), base) {
		return fmt.Errorf("invalid folder name")
	}
	if _, err := os.Stat(storagePath); err == nil {
		return fmt.Errorf("folder already exists")
	}
	return os.MkdirAll(storagePath, 0755)
}

// MoveFiles moves files within the documents directory.
// Embedded files are skipped and reported in the result.
func (s *DocumentService) MoveFiles(ctx context.Context, moves []dto.FileMove) (*dto.MoveFilesResult, error) {
	result := &dto.MoveFilesResult{Moved: []string{}, Skipped: []string{}}
	for _, move := range moves {
		// Check if document is embedded (has DocumentVector records)
		var count int64
		if err := s.db.WithContext(ctx).Model(&models.DocumentVector{}).Where("doc_id = ?", move.From).Count(&count).Error; err != nil {
			return result, fmt.Errorf("check embedding status for %s: %w", move.From, err)
		}
		if count > 0 {
			result.Skipped = append(result.Skipped, move.From)
			continue
		}
		oldPath := filepath.Join(s.cfg.StorageDir, "documents", move.From)
		newPath := filepath.Join(s.cfg.StorageDir, "documents", move.To)
		docDir := filepath.Join(s.cfg.StorageDir, "documents")
		base := filepath.Clean(docDir) + string(filepath.Separator)
		if !strings.HasPrefix(filepath.Clean(oldPath)+string(filepath.Separator), base) || !strings.HasPrefix(filepath.Clean(newPath)+string(filepath.Separator), base) {
			return result, fmt.Errorf("invalid file path")
		}
		if err := os.Rename(oldPath, newPath); err != nil {
			return result, fmt.Errorf("move %s to %s: %w", move.From, move.To, err)
		}
		result.Moved = append(result.Moved, move.From)
	}
	return result, nil
}

// ListDocuments returns all documents (optionally filtered by folder)
func (s *DocumentService) ListDocuments(ctx context.Context, folder string) ([]models.WorkspaceDocument, error) {
	var docs []models.WorkspaceDocument
	query := s.db.WithContext(ctx)
	if folder != "" {
		query = query.Where("docpath LIKE ?", "%"+utils.EscapeLike(folder)+"%")
	}
	if err := query.Find(&docs).Error; err != nil {
		return nil, err
	}
	return docs, nil
}

// ListFolderDocuments returns documents in a specific folder
func (s *DocumentService) ListFolderDocuments(ctx context.Context, folderName string) ([]models.WorkspaceDocument, error) {
	var docs []models.WorkspaceDocument
	if err := s.db.WithContext(ctx).Where("docpath LIKE ?", "%/"+utils.EscapeLike(folderName)+"/%").Find(&docs).Error; err != nil {
		return nil, err
	}
	return docs, nil
}

// GetByDocName returns a document by its filename
func (s *DocumentService) GetByDocName(ctx context.Context, docName string) (*models.WorkspaceDocument, error) {
	var doc models.WorkspaceDocument
	if err := s.db.WithContext(ctx).Where("filename = ?", docName).First(&doc).Error; err != nil {
		return nil, err
	}
	return &doc, nil
}

func (s *DocumentService) UploadToWorkspace(ctx context.Context, wsSlug string, fileHeader *multipart.FileHeader, progressMgr *EmbeddingProgressManager) (*models.WorkspaceDocument, error) {
	var ws models.Workspace
	if err := s.GetWorkspaceBySlug(ctx, wsSlug, &ws); err != nil {
		return nil, fmt.Errorf("workspace not found: %w", err)
	}
	if progressMgr != nil {
		progressMgr.BroadcastProgress(wsSlug, fileHeader.Filename, 0)
	}
	doc, err := s.SaveUpload(ctx, ws.ID, fileHeader)
	if err != nil {
		if progressMgr != nil {
			progressMgr.Broadcast(wsSlug, EmbedProgressEvent{Type: "error", Message: err.Error(), Document: fileHeader.Filename})
		}
		return nil, err
	}
	if progressMgr != nil {
		progressMgr.BroadcastProgress(wsSlug, fileHeader.Filename, 100)
		progressMgr.Broadcast(wsSlug, EmbedProgressEvent{Type: "complete", Message: "Upload complete", Document: fileHeader.Filename})
	}
	return doc, nil
}

func (s *DocumentService) UploadLink(ctx context.Context, wsSlug string, link string, progressMgr *EmbeddingProgressManager) ([]*models.WorkspaceDocument, error) {
	var ws models.Workspace
	if err := s.GetWorkspaceBySlug(ctx, wsSlug, &ws); err != nil {
		return nil, fmt.Errorf("workspace not found: %w", err)
	}
	if s.coll == nil {
		return nil, fmt.Errorf("collector not available")
	}
	resp, err := s.coll.ProcessLink(ctx, link, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("collector process link: %w", err)
	}
	if !resp.Success || len(resp.Documents) == 0 {
		return nil, fmt.Errorf("collector failed: %s", resp.Reason)
	}

	var docs []*models.WorkspaceDocument
	for i, d := range resp.Documents {
		if progressMgr != nil {
			progressMgr.BroadcastProgress(wsSlug, d.Name, (i*100)/len(resp.Documents))
		}
		docId := uuid.New().String()
		doc := models.WorkspaceDocument{
			WorkspaceID:   ws.ID,
			DocId:         docId,
			Filename:      d.Name,
			Docpath:       d.Location,
			CreatedAt:     time.Now(),
			LastUpdatedAt: time.Now(),
		}
		meta := map[string]interface{}{
			"url":                d.URL,
			"title":              d.Title,
			"docAuthor":          d.DocAuthor,
			"description":        d.Description,
			"docSource":          d.DocSource,
			"chunkSource":        d.ChunkSource,
			"published":          d.Published,
			"wordCount":          d.WordCount,
			"tokenCountEstimate": d.TokenCountEstimate,
		}
		metaJSON, _ := json.Marshal(meta)
		metaStr := string(metaJSON)
		doc.Metadata = &metaStr
		if err := s.db.Create(&doc).Error; err != nil {
			return nil, fmt.Errorf("save document record: %w", err)
		}
		docs = append(docs, &doc)
		if s.embedder != nil && s.vectorDB != nil {
			go func(docRef models.WorkspaceDocument) {
				bgCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
				defer cancel()
				if err := s.EmbedDocument(bgCtx, &docRef); err != nil {
					mlog.Error("embed document failed: ", mlog.Err(err))
				}
			}(doc)
		}
	}
	if progressMgr != nil {
		progressMgr.BroadcastProgress(wsSlug, link, 100)
	}
	return docs, nil
}

// UploadAndQueueEmbed uploads a file and queues it for background embedding.
// The embedding happens asynchronously; the returned document may not yet be embedded.
func (s *DocumentService) UploadAndQueueEmbed(ctx context.Context, wsSlug string, fileHeader *multipart.FileHeader) (*models.WorkspaceDocument, error) {
	var ws models.Workspace
	if err := s.GetWorkspaceBySlug(ctx, wsSlug, &ws); err != nil {
		return nil, fmt.Errorf("workspace not found: %w", err)
	}
	return s.SaveUpload(ctx, ws.ID, fileHeader)
}

func (s *DocumentService) UpdateEmbeddings(ctx context.Context, wsSlug string, adds []string, removes []string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Use a transactional document service for the add phase
		txSvc := &DocumentService{
			db:       tx,
			cfg:      s.cfg,
			coll:     s.coll,
			embedder: s.embedder,
			chunker:  s.chunker,
			vectorDB: s.vectorDB,
			fs:       s.fs,
		}
		for _, docId := range adds {
			doc, err := txSvc.GetByDocId(ctx, docId)
			if err != nil {
				return fmt.Errorf("find document %s: %w", docId, err)
			}
			if err := txSvc.EmbedDocument(ctx, doc); err != nil {
				return fmt.Errorf("embed document %s: %w", docId, err)
			}
		}
		if len(removes) > 0 {
			if s.vectorDB != nil {
				var docVectors []models.DocumentVector
				if err := tx.Where("doc_id IN ?", removes).Find(&docVectors).Error; err != nil {
					return fmt.Errorf("find document vectors: %w", err)
				}
				vectorIds := make([]string, len(docVectors))
				for i, dv := range docVectors {
					vectorIds[i] = dv.VectorId
				}
				if len(vectorIds) > 0 {
					if err := s.vectorDB.DeleteVectors(ctx, wsSlug, vectorIds); err != nil {
						return fmt.Errorf("delete vectors: %w", err)
					}
				}
			}
			if err := tx.Where("doc_id IN ?", removes).Delete(&models.DocumentVector{}).Error; err != nil {
				return fmt.Errorf("delete document vectors: %w", err)
			}
		}
		return nil
	})
}

func (s *DocumentService) RemoveAndUnembed(ctx context.Context, wsSlug string, docId string) error {
	var ws models.Workspace
	if err := s.GetWorkspaceBySlug(ctx, wsSlug, &ws); err != nil {
		return fmt.Errorf("workspace not found: %w", err)
	}
	doc, err := s.GetByDocId(ctx, docId)
	if err != nil {
		return fmt.Errorf("document not found: %w", err)
	}

	// 1. Query vectorIds first
	var docVectors []models.DocumentVector
	if err := s.db.Where("doc_id = ?", docId).Find(&docVectors).Error; err != nil {
		return fmt.Errorf("find document vectors: %w", err)
	}
	vectorIds := make([]string, len(docVectors))
	for i, dv := range docVectors {
		vectorIds[i] = dv.VectorId
	}

	// 2. Delete SQL records
	if err := s.db.Where("doc_id = ?", docId).Delete(&models.DocumentVector{}).Error; err != nil {
		return fmt.Errorf("delete document vectors: %w", err)
	}
	if err := s.db.Where("doc_id = ?", docId).Delete(&models.WorkspaceDocument{}).Error; err != nil {
		return fmt.Errorf("delete document record: %w", err)
	}

	// 3. Delete vectors from vector DB
	if s.vectorDB != nil && len(vectorIds) > 0 {
		if err := s.vectorDB.DeleteVectors(ctx, wsSlug, vectorIds); err != nil {
			return fmt.Errorf("delete vectors: %w", err)
		}
	}

	if err := os.Remove(doc.Docpath); err != nil && !os.IsNotExist(err) {
		mlog.Error("remove document file failed: ", mlog.Err(err))
	}
	return nil
}

// PurgeByDocName removes a document across all workspaces:
//  1. Look up workspace_documents rows by docpath
//  2. For each, purge vectors via VectorService (best-effort; missing collection is OK)
//  3. Delete all matching workspace_documents rows
//  4. Remove the source file via FileSystemService
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
			// Query vectorIds for these docIds
			var docVectors []models.DocumentVector
			if err := s.db.Where("doc_id IN ?", docIds).Find(&docVectors).Error; err != nil {
				continue
			}
			vectorIds := make([]string, len(docVectors))
			for i, dv := range docVectors {
				vectorIds[i] = dv.VectorId
			}
			if len(vectorIds) > 0 {
				_ = s.vectorDB.DeleteVectors(ctx, ws.Slug, vectorIds)
			}
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
			mlog.Error("RemoveFolder: purge doc failed: ", mlog.Err(err))
		}
	}
	return s.fs.RemoveFolder(folderName)
}

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
