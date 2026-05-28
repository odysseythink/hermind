package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/dto"
	"github.com/odysseythink/hermind/backend/internal/middleware"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/services"
)

type APIDocumentHandler struct {
	docSvc      *services.DocumentService
	fs          *services.FileSystemService
	progressMgr *services.EmbeddingProgressManager
}

func NewAPIDocumentHandler(docSvc *services.DocumentService, fs *services.FileSystemService, progressMgr *services.EmbeddingProgressManager) *APIDocumentHandler {
	return &APIDocumentHandler{docSvc: docSvc, fs: fs, progressMgr: progressMgr}
}

// splitSlugs splits Node's comma-delimited addToWorkspaces string into a slice.
func splitSlugs(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func (h *APIDocumentHandler) Upload(c *gin.Context) {
	h.handleUpload(c, "custom-documents")
}

func (h *APIDocumentHandler) UploadToFolder(c *gin.Context) {
	h.handleUpload(c, c.Param("folderName"))
}

func (h *APIDocumentHandler) handleUpload(c *gin.Context, folder string) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "file field required"})
		return
	}
	addTo := c.PostForm("addToWorkspaces")
	slugs := splitSlugs(addTo)

	if len(slugs) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"success":   false,
			"error":     "addToWorkspaces required when no workspace binding is implicit",
			"documents": []any{},
		})
		return
	}

	var created []any
	for _, slug := range slugs {
		doc, err := h.docSvc.UploadToWorkspace(c.Request.Context(), slug, fileHeader, h.progressMgr)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false, "error": err.Error(), "documents": created,
			})
			return
		}
		created = append(created, doc)
	}
	_ = folder // folder parameter currently advisory
	c.JSON(http.StatusOK, gin.H{"success": true, "error": nil, "documents": created})
}

func (h *APIDocumentHandler) UploadLink(c *gin.Context) {
	var req struct {
		Link            string `json:"link"`
		AddToWorkspaces string `json:"addToWorkspaces"`
		Metadata        any    `json:"metadata"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	slugs := splitSlugs(req.AddToWorkspaces)
	if len(slugs) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"success": false, "error": "addToWorkspaces is required", "documents": []any{},
		})
		return
	}
	var created []any
	for _, slug := range slugs {
		docs, err := h.docSvc.UploadLink(c.Request.Context(), slug, req.Link, h.progressMgr)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"success": false, "error": err.Error(), "documents": created})
			return
		}
		for _, d := range docs {
			created = append(created, enrichDocumentResponse(d))
		}
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "error": nil, "documents": created})
}

func (h *APIDocumentHandler) RawText(c *gin.Context) {
	var req dto.APIRawTextRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	if req.Text == "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"success": false, "error": "The 'textContent' key cannot have an empty value."})
		return
	}
	// Require metadata.title (Node parity).
	md, _ := req.Metadata.(map[string]any)
	var title string
	if md != nil {
		if v, ok := md["title"].(string); ok {
			title = v
		}
	}
	if title == "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"success": false,
			"error":   "You are missing required metadata key:value pairs in your request. Required metadata key:values are 'title'",
		})
		return
	}
	slugs := splitSlugs(req.AddToWorkspaces)
	docs, err := h.docSvc.SaveRawText(c.Request.Context(), req.Text, title, md, slugs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "error": nil, "documents": docs})
}

func (h *APIDocumentHandler) CreateFolder(c *gin.Context) {
	var req struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	if req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "name required"})
		return
	}
	if err := h.docSvc.CreateFolder(c.Request.Context(), req.Name); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Folder created"})
}

func (h *APIDocumentHandler) MoveFiles(c *gin.Context) {
	var req dto.MoveFilesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	res, err := h.docSvc.MoveFiles(c.Request.Context(), req.Files)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": res})
}

func (h *APIDocumentHandler) RemoveFolder(c *gin.Context) {
	var req dto.APIDocumentRemoveFolderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	if err := h.docSvc.RemoveFolder(c.Request.Context(), req.Name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to remove folder: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Folder removed successfully"})
}

func (h *APIDocumentHandler) ListAll(c *gin.Context) {
	files, err := h.fs.ListLocalFiles("")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"localFiles": files})
}

func (h *APIDocumentHandler) ListFolder(c *gin.Context) {
	folder := c.Param("folderName")
	files, err := h.fs.ListLocalFiles(folder)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"localFiles": files})
}

func (h *APIDocumentHandler) GetByDocName(c *gin.Context) {
	docName := c.Param("docName")
	doc, err := h.docSvc.GetByDocName(c.Request.Context(), docName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Document not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"document": doc})
}

func (h *APIDocumentHandler) MetadataSchema(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"schema": gin.H{
			"url":         "string | nullable",
			"title":       "string",
			"docAuthor":   "string | nullable",
			"description": "string | nullable",
			"docSource":   "string | nullable",
			"chunkSource": "string | nullable",
			"published":   "epoch timestamp in ms | nullable",
		},
	})
}

func (h *APIDocumentHandler) AcceptedFileTypes(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"types": h.fs.AcceptedDocumentTypes()})
}

// reservedDocFields are keys that must not be overwritten by parsed metadata.
var reservedDocFields = map[string]bool{
	"id": true, "docId": true, "filename": true, "docpath": true,
	"workspaceId": true, "pinned": true, "watched": true,
	"createdAt": true, "lastUpdatedAt": true, "metadata": true,
}

// enrichDocumentResponse builds a map representation of a WorkspaceDocument with
// parsed metadata fields expanded (Node parity for upload-link responses).
func enrichDocumentResponse(doc *models.WorkspaceDocument) gin.H {
	resp := gin.H{
		"id":            doc.ID,
		"docId":         doc.DocId,
		"filename":      doc.Filename,
		"docpath":       doc.Docpath,
		"workspaceId":   doc.WorkspaceID,
		"pinned":        doc.Pinned,
		"watched":       doc.Watched,
		"createdAt":     doc.CreatedAt,
		"lastUpdatedAt": doc.LastUpdatedAt,
	}
	if doc.Metadata != nil {
		var meta map[string]any
		if err := json.Unmarshal([]byte(*doc.Metadata), &meta); err == nil {
			for k, v := range meta {
				if !reservedDocFields[k] {
					resp[k] = v
				}
			}
		}
		resp["metadata"] = *doc.Metadata
	}
	return resp
}

func RegisterAPIDocumentRoutes(
	r *gin.RouterGroup,
	apiKeySvc *services.APIKeyService,
	docSvc *services.DocumentService,
	fs *services.FileSystemService,
	progressMgr *services.EmbeddingProgressManager,
) {
	h := NewAPIDocumentHandler(docSvc, fs, progressMgr)

	// D1-D4: uploads
	r.POST("/v1/document/upload", middleware.ValidAPIKey(apiKeySvc), h.Upload)
	r.POST("/v1/document/upload/:folderName", middleware.ValidAPIKey(apiKeySvc), h.UploadToFolder)
	r.POST("/v1/document/upload-link", middleware.ValidAPIKey(apiKeySvc), h.UploadLink)
	r.POST("/v1/document/raw-text", middleware.ValidAPIKey(apiKeySvc), h.RawText)

	// D5-D6, D12: folder ops
	r.POST("/v1/document/create-folder", middleware.ValidAPIKey(apiKeySvc), h.CreateFolder)
	r.POST("/v1/document/move-files", middleware.ValidAPIKey(apiKeySvc), h.MoveFiles)
	r.DELETE("/v1/document/remove-folder", middleware.ValidAPIKey(apiKeySvc), h.RemoveFolder)

	// D10-D11: hardcoded responses (MUST be before D9 param route)
	r.GET("/v1/document/metadata-schema", middleware.ValidAPIKey(apiKeySvc), h.MetadataSchema)
	r.GET("/v1/document/accepted-file-types", middleware.ValidAPIKey(apiKeySvc), h.AcceptedFileTypes)

	// D7-D8: listing
	r.GET("/v1/documents", middleware.ValidAPIKey(apiKeySvc), h.ListAll)
	r.GET("/v1/documents/folder/:folderName", middleware.ValidAPIKey(apiKeySvc), h.ListFolder)

	// D9: param route LAST
	r.GET("/v1/document/:docName", middleware.ValidAPIKey(apiKeySvc), h.GetByDocName)
}
