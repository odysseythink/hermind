package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/dto"
	"github.com/odysseythink/hermind/backend/internal/middleware"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/services"
	"github.com/odysseythink/hermind/backend/pkg/utils"
	"gorm.io/gorm"
)

type APIWorkspaceHandler struct {
	wsSvc        *services.WorkspaceService
	chatSvc      *services.ChatService
	vectorSearch *services.VectorSearchService
	docSvc       *services.DocumentService
}

func NewAPIWorkspaceHandler(
	wsSvc *services.WorkspaceService,
	chatSvc *services.ChatService,
	vectorSearch *services.VectorSearchService,
	docSvc *services.DocumentService,
) *APIWorkspaceHandler {
	return &APIWorkspaceHandler{
		wsSvc:        wsSvc,
		chatSvc:      chatSvc,
		vectorSearch: vectorSearch,
		docSvc:       docSvc,
	}
}

func (h *APIWorkspaceHandler) List(c *gin.Context) {
	workspaces, err := h.wsSvc.List(c.Request.Context(), 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"workspaces": workspaces})
}

func (h *APIWorkspaceHandler) Create(c *gin.Context) {
	var req dto.CreateWorkspaceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ws, err := h.wsSvc.Create(c.Request.Context(), 0, req)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"workspace": nil, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"workspace": ws, "message": "Workspace created"})
}

func (h *APIWorkspaceHandler) Get(c *gin.Context) {
	ws := c.MustGet("workspace").(*models.Workspace)
	c.JSON(http.StatusOK, gin.H{"workspace": ws})
}

func (h *APIWorkspaceHandler) Update(c *gin.Context) {
	ws := c.MustGet("workspace").(*models.Workspace)
	var req dto.UpdateWorkspaceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.wsSvc.Update(c.Request.Context(), ws.Slug, req, nil); err != nil {
		c.JSON(http.StatusOK, gin.H{"workspace": nil, "message": err.Error()})
		return
	}
	updated, err := h.wsSvc.GetBySlug(c.Request.Context(), ws.Slug)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"workspace": nil, "message": "Workspace updated"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"workspace": updated, "message": "Workspace updated"})
}

func (h *APIWorkspaceHandler) UpdatePin(c *gin.Context) {
	ws := c.MustGet("workspace").(*models.Workspace)
	var req dto.APIUpdatePinRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.wsSvc.UpdatePin(c.Request.Context(), ws.ID, req.DocPath, req.PinValue); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "document not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Pin status updated successfully"})
}

func (h *APIWorkspaceHandler) Delete(c *gin.Context) {
	ws := c.MustGet("workspace").(*models.Workspace)
	if err := h.wsSvc.Delete(c.Request.Context(), ws.Slug); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Workspace " + ws.Slug + " deleted"})
}

func (h *APIWorkspaceHandler) Chats(c *gin.Context) {
	ws := c.MustGet("workspace").(*models.Workspace)
	history, err := h.wsSvc.GetChats(c.Request.Context(), ws.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"history": history})
}

func (h *APIWorkspaceHandler) VectorSearch(c *gin.Context) {
	if h.vectorSearch == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "embedder not configured"})
		return
	}
	ws := c.MustGet("workspace").(*models.Workspace)
	var req dto.VectorSearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	results, err := h.vectorSearch.Search(c.Request.Context(), ws, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"results": results})
}

func (h *APIWorkspaceHandler) UpdateEmbeddings(c *gin.Context) {
	ws := c.MustGet("workspace").(*models.Workspace)
	var req dto.UpdateEmbeddingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.docSvc.UpdateEmbeddings(c.Request.Context(), ws.Slug, req.Adds, req.Removes); err != nil {
		c.JSON(http.StatusOK, gin.H{"workspace": nil, "message": err.Error()})
		return
	}
	updated, _ := h.wsSvc.GetBySlug(c.Request.Context(), ws.Slug)
	c.JSON(http.StatusOK, gin.H{"workspace": updated, "message": "Workspace embeddings updated"})
}

func (h *APIWorkspaceHandler) Chat(c *gin.Context) {
	ws := c.MustGet("workspace").(*models.Workspace)
	var req dto.ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ChatResponse{Type: "abort", Close: true, Error: err.Error()})
		return
	}
	resp, err := h.chatSvc.Complete(c.Request.Context(), ws, nil, nil, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ChatResponse{Type: "abort", Close: true, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *APIWorkspaceHandler) StreamChat(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	ws := c.MustGet("workspace").(*models.Workspace)
	var req dto.StreamChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.StreamChatResponse{Type: "abort", Close: true, Error: utils.Ptr(err.Error())})
		return
	}

	stream, err := h.chatSvc.Stream(c.Request.Context(), ws, nil, nil, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.StreamChatResponse{Type: "abort", Close: true, Error: utils.Ptr(err.Error())})
		return
	}

	for chunk := range stream {
		if err := writeSSEChunk(c.Writer, chunk); err != nil {
			break
		}
		if f, ok := c.Writer.(http.Flusher); ok {
			f.Flush()
		}
	}
}

func RegisterAPIWorkspaceRoutes(
	r *gin.RouterGroup,
	apiKeySvc *services.APIKeyService,
	wsSvc *services.WorkspaceService,
	chatSvc *services.ChatService,
	vectorSearch *services.VectorSearchService,
	docSvc *services.DocumentService,
	db *gorm.DB,
) {
	h := NewAPIWorkspaceHandler(wsSvc, chatSvc, vectorSearch, docSvc)

	// W1-W6: CRUD
	r.GET("/v1/workspaces", middleware.ValidAPIKey(apiKeySvc), h.List)
	r.POST("/v1/workspace/new", middleware.ValidAPIKey(apiKeySvc), h.Create)
	r.GET("/v1/workspace/:slug", middleware.ValidAPIKey(apiKeySvc), middleware.ValidWorkspaceSlug(db), h.Get)
	r.POST("/v1/workspace/:slug/update", middleware.ValidAPIKey(apiKeySvc), middleware.ValidWorkspaceSlug(db), h.Update)
	r.POST("/v1/workspace/:slug/update-pin", middleware.ValidAPIKey(apiKeySvc), middleware.ValidWorkspaceSlug(db), h.UpdatePin)
	r.DELETE("/v1/workspace/:slug", middleware.ValidAPIKey(apiKeySvc), middleware.ValidWorkspaceSlug(db), h.Delete)

	// W7: chats
	r.GET("/v1/workspace/:slug/chats", middleware.ValidAPIKey(apiKeySvc), middleware.ValidWorkspaceSlug(db), h.Chats)

	// W10: vector-search
	r.POST("/v1/workspace/:slug/vector-search", middleware.ValidAPIKey(apiKeySvc), middleware.ValidWorkspaceSlug(db), h.VectorSearch)

	// W11: update-embeddings
	r.POST("/v1/workspace/:slug/update-embeddings", middleware.ValidAPIKey(apiKeySvc), middleware.ValidWorkspaceSlug(db), h.UpdateEmbeddings)

	// W8-W9: chat + stream-chat
	r.POST("/v1/workspace/:slug/chat", middleware.ValidAPIKey(apiKeySvc), middleware.ValidWorkspaceSlug(db), h.Chat)
	r.POST("/v1/workspace/:slug/stream-chat", middleware.ValidAPIKey(apiKeySvc), middleware.ValidWorkspaceSlug(db), h.StreamChat)
}
