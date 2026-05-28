package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/dto"
	"github.com/odysseythink/hermind/backend/internal/middleware"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/services"
	"github.com/odysseythink/hermind/backend/pkg/utils"
	"gorm.io/gorm"
)

type APIThreadHandler struct {
	threadSvc *services.ThreadService
	chatSvc   *services.ChatService
}

func NewAPIThreadHandler(threadSvc *services.ThreadService, chatSvc *services.ChatService) *APIThreadHandler {
	return &APIThreadHandler{threadSvc: threadSvc, chatSvc: chatSvc}
}

func (h *APIThreadHandler) Create(c *gin.Context) {
	ws := c.MustGet("workspace").(*models.Workspace)
	var req dto.CreateThreadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"thread": nil, "message": err.Error()})
		return
	}
	thread, err := h.threadSvc.Create(c.Request.Context(), ws.ID, nil, req)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"thread": nil, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"thread": thread, "message": "Thread created"})
}

func (h *APIThreadHandler) Update(c *gin.Context) {
	thread := c.MustGet("thread").(*models.WorkspaceThread)
	var req dto.UpdateThreadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"thread": nil, "message": err.Error()})
		return
	}
	if err := h.threadSvc.Update(c.Request.Context(), thread, req); err != nil {
		c.JSON(http.StatusOK, gin.H{"thread": nil, "message": err.Error()})
		return
	}
	// Reload to reflect any side-effects (triggers, computed values).
	updated, _ := h.threadSvc.GetBySlug(c.Request.Context(), thread.WorkspaceID, thread.Slug)
	if updated != nil {
		thread = updated
	}
	c.JSON(http.StatusOK, gin.H{"thread": thread, "message": "Thread updated"})
}

func (h *APIThreadHandler) Delete(c *gin.Context) {
	ws := c.MustGet("workspace").(*models.Workspace)
	thread := c.MustGet("thread").(*models.WorkspaceThread)
	if err := h.threadSvc.Delete(c.Request.Context(), ws.ID, thread.Slug); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Thread deleted"})
}

func (h *APIThreadHandler) GetChats(c *gin.Context) {
	thread := c.MustGet("thread").(*models.WorkspaceThread)
	history, err := h.threadSvc.GetThreadChats(c.Request.Context(), thread.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"history": history})
}

func (h *APIThreadHandler) Chat(c *gin.Context) {
	ws := c.MustGet("workspace").(*models.Workspace)
	thread := c.MustGet("thread").(*models.WorkspaceThread)
	var req dto.ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ChatResponse{Type: "abort", Close: true, Error: err.Error()})
		return
	}
	resp, err := h.chatSvc.Complete(c.Request.Context(), ws, nil, &thread.ID, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ChatResponse{Type: "abort", Close: true, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *APIThreadHandler) StreamChat(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	ws := c.MustGet("workspace").(*models.Workspace)
	thread := c.MustGet("thread").(*models.WorkspaceThread)
	var req dto.StreamChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.StreamChatResponse{Type: "abort", Close: true, Error: utils.Ptr(err.Error())})
		return
	}
	stream, err := h.chatSvc.Stream(c.Request.Context(), ws, nil, &thread.ID, req)
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

func RegisterAPIThreadRoutes(
	r *gin.RouterGroup,
	apiKeySvc *services.APIKeyService,
	threadSvc *services.ThreadService,
	chatSvc *services.ChatService,
	db *gorm.DB,
) {
	h := NewAPIThreadHandler(threadSvc, chatSvc)
	// /thread/new only has :slug — use the single-slug middleware.
	r.POST("/v1/workspace/:slug/thread/new",
		middleware.ValidAPIKey(apiKeySvc),
		middleware.ValidWorkspaceSlug(db),
		h.Create)
	// Routes with both :slug and :threadSlug use the combined middleware,
	// which sets BOTH c.MustGet("workspace") and c.MustGet("thread").
	r.POST("/v1/workspace/:slug/thread/:threadSlug/update",
		middleware.ValidAPIKey(apiKeySvc),
		middleware.ValidWorkspaceAndThreadSlug(db),
		h.Update)
	r.DELETE("/v1/workspace/:slug/thread/:threadSlug",
		middleware.ValidAPIKey(apiKeySvc),
		middleware.ValidWorkspaceAndThreadSlug(db),
		h.Delete)
	r.GET("/v1/workspace/:slug/thread/:threadSlug/chats",
		middleware.ValidAPIKey(apiKeySvc),
		middleware.ValidWorkspaceAndThreadSlug(db),
		h.GetChats)
	r.POST("/v1/workspace/:slug/thread/:threadSlug/chat",
		middleware.ValidAPIKey(apiKeySvc),
		middleware.ValidWorkspaceAndThreadSlug(db),
		h.Chat)
	r.POST("/v1/workspace/:slug/thread/:threadSlug/stream-chat",
		middleware.ValidAPIKey(apiKeySvc),
		middleware.ValidWorkspaceAndThreadSlug(db),
		h.StreamChat)
}
