package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/dto"
	"github.com/odysseythink/hermind/backend/internal/middleware"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/services"
)

type DocumentHandler struct {
	docSvc *services.DocumentService
}

func NewDocumentHandler(docSvc *services.DocumentService) *DocumentHandler {
	return &DocumentHandler{docSvc: docSvc}
}

func (h *DocumentHandler) UploadDocument(c *gin.Context) {
	workspaceSlug := c.PostForm("workspaceSlug")
	var ws models.Workspace
	if err := h.docSvc.GetWorkspaceBySlug(c.Request.Context(), workspaceSlug, &ws); err != nil {
		c.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "workspace not found"})
		return
	}
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}
	doc, err := h.docSvc.SaveUpload(c.Request.Context(), ws.ID, file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"document": doc, "message": "Document uploaded"})
}

func (h *DocumentHandler) GetDocument(c *gin.Context) {
	param := c.Param("docId")
	// Try docId first
	doc, err := h.docSvc.GetByDocId(c.Request.Context(), param)
	if err != nil {
		c.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "document not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"document": doc})
}

func (h *DocumentHandler) DeleteDocument(c *gin.Context) {
	docId := c.Param("docId")
	if err := h.docSvc.DeleteByDocId(c.Request.Context(), docId); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *DocumentHandler) AcceptedExtensions(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"accepted": []string{
		".txt", ".md", ".pdf", ".docx", ".csv", ".json", ".html",
	}})
}

func (h *DocumentHandler) CreateFolder(c *gin.Context) {
	var req struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}
	if err := h.docSvc.CreateFolder(c.Request.Context(), req.Name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": nil})
}

func (h *DocumentHandler) MoveFiles(c *gin.Context) {
	var req dto.MoveFilesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}
	result, err := h.docSvc.MoveFiles(c.Request.Context(), req.Files)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "moved": result.Moved, "skipped": result.Skipped})
}

func (h *DocumentHandler) ListDocuments(c *gin.Context) {
	folder := c.Query("folder")
	docs, err := h.docSvc.ListDocuments(c.Request.Context(), folder)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"documents": docs})
}

func (h *DocumentHandler) ListFolderDocuments(c *gin.Context) {
	folderName := c.Param("folderName")
	docs, err := h.docSvc.ListFolderDocuments(c.Request.Context(), folderName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"documents": docs})
}

func (h *DocumentHandler) GetDocumentByName(c *gin.Context) {
	docName := c.Param("docName")
	doc, err := h.docSvc.GetByDocName(c.Request.Context(), docName)
	if err != nil {
		c.JSON(http.StatusNotFound, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"document": doc})
}

func (h *DocumentHandler) AcceptedFileTypes(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"accepted": []string{
		".txt", ".md", ".pdf", ".docx", ".csv", ".json", ".html",
		".jpg", ".jpeg", ".png", ".gif", ".webp",
		".mp3", ".mp4", ".wav", ".mpeg",
	}})
}

func RegisterDocumentRoutes(r *gin.RouterGroup, docSvc *services.DocumentService, authSvc *services.AuthService) {
	h := NewDocumentHandler(docSvc)
	r.POST("/document/upload",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"all"}),
		h.UploadDocument)
	r.GET("/document/:docId",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"all"}),
		h.GetDocument)
	r.DELETE("/document/:docId",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"all"}),
		h.DeleteDocument)
	r.GET("/document/accepted-extensions",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"all"}),
		h.AcceptedExtensions)
	r.POST("/document/create-folder",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"all"}),
		h.CreateFolder)
	r.POST("/document/move-files",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"all"}),
		h.MoveFiles)
	r.GET("/documents",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"all"}),
		h.ListDocuments)
	r.GET("/documents/folder/:folderName",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"all"}),
		h.ListFolderDocuments)
	r.GET("/document/by-name/:docName",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"all"}),
		h.GetDocumentByName)
	r.GET("/document/accepted-file-types",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"all"}),
		h.AcceptedFileTypes)
}
