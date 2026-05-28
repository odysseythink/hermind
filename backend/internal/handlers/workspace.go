package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/dto"
	"github.com/odysseythink/hermind/backend/internal/middleware"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/services"
	"gorm.io/gorm"
)

type WorkspaceHandler struct {
	wsSvc           *services.WorkspaceService
	searchSvc       *services.SearchService
	vectorSearchSvc *services.VectorSearchService
	docSvc          *services.DocumentService
	progressMgr     *services.EmbeddingProgressManager
}

func NewWorkspaceHandler(wsSvc *services.WorkspaceService, searchSvc *services.SearchService, vectorSearchSvc *services.VectorSearchService, docSvc *services.DocumentService, progressMgr *services.EmbeddingProgressManager) *WorkspaceHandler {
	return &WorkspaceHandler{wsSvc: wsSvc, searchSvc: searchSvc, vectorSearchSvc: vectorSearchSvc, docSvc: docSvc, progressMgr: progressMgr}
}

func (h *WorkspaceHandler) CreateWorkspace(c *gin.Context) {
	user := c.MustGet("user").(*models.User)
	var req dto.CreateWorkspaceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}
	ws, err := h.wsSvc.Create(c.Request.Context(), user.ID, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"workspace": ws, "message": "Workspace created"})
}

func (h *WorkspaceHandler) ListWorkspaces(c *gin.Context) {
	user := c.MustGet("user").(*models.User)
	workspaces, err := h.wsSvc.List(c.Request.Context(), user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, dto.WorkspaceListResponse{Workspaces: workspaces})
}

func (h *WorkspaceHandler) GetWorkspace(c *gin.Context) {
	ws := c.MustGet("workspace").(*models.Workspace)
	c.JSON(http.StatusOK, gin.H{"workspace": ws})
}

func (h *WorkspaceHandler) UpdateWorkspace(c *gin.Context) {
	user := c.MustGet("user").(*models.User)
	ws := c.MustGet("workspace").(*models.Workspace)
	var req dto.UpdateWorkspaceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}
	if err := h.wsSvc.Update(c.Request.Context(), ws.Slug, req, &user.ID); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *WorkspaceHandler) DeleteWorkspace(c *gin.Context) {
	ws := c.MustGet("workspace").(*models.Workspace)
	if err := h.wsSvc.Delete(c.Request.Context(), ws.Slug); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *WorkspaceHandler) GetWorkspaceChats(c *gin.Context) {
	ws := c.MustGet("workspace").(*models.Workspace)
	history, err := h.wsSvc.GetChats(c.Request.Context(), ws.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"history": history})
}

func (h *WorkspaceHandler) IsAgentCommandAvailable(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"showAgentCommand": false})
}

func (h *WorkspaceHandler) ParsedFiles(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"files":                    []any{},
		"contextWindow":            4096,
		"currentContextTokenCount": 0,
	})
}

func (h *WorkspaceHandler) SearchWorkspaces(c *gin.Context) {
	user := c.MustGet("user").(*models.User)
	var req struct {
		SearchTerm string `json:"searchTerm"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}
	results, err := h.searchSvc.SearchWorkspaceAndThreads(c.Request.Context(), req.SearchTerm, &user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, results)
}

func (h *WorkspaceHandler) VectorSearch(c *gin.Context) {
	ws := c.MustGet("workspace").(*models.Workspace)
	if h.vectorSearchSvc == nil {
		c.JSON(http.StatusServiceUnavailable, dto.ErrorResponse{Error: "vector search unavailable"})
		return
	}
	var req dto.VectorSearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}
	results, err := h.vectorSearchSvc.Search(c.Request.Context(), ws, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"results": results})
}

func (h *WorkspaceHandler) UploadToWorkspace(c *gin.Context) {
	ws := c.MustGet("workspace").(*models.Workspace)
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}
	doc, err := h.docSvc.UploadToWorkspace(c.Request.Context(), ws.Slug, file, h.progressMgr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "document": doc})
}

func (h *WorkspaceHandler) UploadLink(c *gin.Context) {
	ws := c.MustGet("workspace").(*models.Workspace)
	var req dto.UploadLinkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}
	docs, err := h.docSvc.UploadLink(c.Request.Context(), ws.Slug, req.Link, h.progressMgr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "documents": docs})
}

func (h *WorkspaceHandler) UploadAndEmbed(c *gin.Context) {
	ws := c.MustGet("workspace").(*models.Workspace)
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}
	doc, err := h.docSvc.UploadAndQueueEmbed(c.Request.Context(), ws.Slug, file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "document": doc})
}

func (h *WorkspaceHandler) UpdateEmbeddings(c *gin.Context) {
	ws := c.MustGet("workspace").(*models.Workspace)
	var req dto.UpdateEmbeddingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}
	if err := h.docSvc.UpdateEmbeddings(c.Request.Context(), ws.Slug, req.Adds, req.Removes); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *WorkspaceHandler) RemoveAndUnembed(c *gin.Context) {
	ws := c.MustGet("workspace").(*models.Workspace)
	docId := c.Query("docId")
	if docId == "" {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "docId required"})
		return
	}
	if err := h.docSvc.RemoveAndUnembed(c.Request.Context(), ws.Slug, docId); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *WorkspaceHandler) EmbedProgress(c *gin.Context) {
	ws := c.MustGet("workspace").(*models.Workspace)
	if h.progressMgr == nil {
		c.JSON(http.StatusServiceUnavailable, dto.ErrorResponse{Error: "progress manager not available"})
		return
	}
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")

	connID, done := h.progressMgr.AddConnection(ws.Slug, c.Writer)
	defer h.progressMgr.RemoveConnection(ws.Slug, connID)
	select {
	case <-done:
	case <-c.Request.Context().Done():
	}
}

func RegisterWorkspaceRoutes(r *gin.RouterGroup, wsSvc *services.WorkspaceService, authSvc *services.AuthService, db *gorm.DB, searchSvc *services.SearchService, vectorSearchSvc *services.VectorSearchService, docSvc *services.DocumentService, progressMgr *services.EmbeddingProgressManager) {
	h := NewWorkspaceHandler(wsSvc, searchSvc, vectorSearchSvc, docSvc, progressMgr)
	r.POST("/workspace/new",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"all"}),
		h.CreateWorkspace)
	r.GET("/workspace",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"all"}),
		h.ListWorkspaces)
	r.GET("/workspaces",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"all"}),
		h.ListWorkspaces)
	r.GET("/workspace/:slug",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"all"}),
		middleware.ValidWorkspaceSlug(db),
		h.GetWorkspace)
	r.POST("/workspace/:slug/update",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"all"}),
		middleware.ValidWorkspaceSlug(db),
		h.UpdateWorkspace)
	r.DELETE("/workspace/:slug",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"all"}),
		middleware.ValidWorkspaceSlug(db),
		h.DeleteWorkspace)
	r.GET("/workspace/:slug/chats",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"all"}),
		middleware.ValidWorkspaceSlug(db),
		h.GetWorkspaceChats)
	r.GET("/workspace/:slug/is-agent-command-available",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"all"}),
		middleware.ValidWorkspaceSlug(db),
		h.IsAgentCommandAvailable)
	r.GET("/workspace/:slug/parsed-files",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"all"}),
		middleware.ValidWorkspaceSlug(db),
		h.ParsedFiles)
	r.POST("/workspace/search",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"all"}),
		h.SearchWorkspaces)
	r.POST("/workspace/:slug/vector-search",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"all"}),
		middleware.ValidWorkspaceSlug(db),
		h.VectorSearch)
	r.POST("/workspace/:slug/upload",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"all"}),
		middleware.ValidWorkspaceSlug(db),
		h.UploadToWorkspace)
	r.POST("/workspace/:slug/upload-link",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"all"}),
		middleware.ValidWorkspaceSlug(db),
		h.UploadLink)
	r.POST("/workspace/:slug/upload-and-embed",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"all"}),
		middleware.ValidWorkspaceSlug(db),
		h.UploadAndEmbed)
	r.POST("/workspace/:slug/update-embeddings",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"all"}),
		middleware.ValidWorkspaceSlug(db),
		h.UpdateEmbeddings)
	r.DELETE("/workspace/:slug/remove-and-unembed",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"all"}),
		middleware.ValidWorkspaceSlug(db),
		h.RemoveAndUnembed)
	r.GET("/workspace/:slug/embed-progress",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"all"}),
		middleware.ValidWorkspaceSlug(db),
		h.EmbedProgress)
}
