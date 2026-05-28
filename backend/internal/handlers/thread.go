package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/dto"
	"github.com/odysseythink/hermind/backend/internal/middleware"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/services"
	"gorm.io/gorm"
)

type ThreadHandler struct {
	threadSvc *services.ThreadService
}

func NewThreadHandler(threadSvc *services.ThreadService) *ThreadHandler {
	return &ThreadHandler{threadSvc: threadSvc}
}

func (h *ThreadHandler) CreateThread(c *gin.Context) {
	ws := c.MustGet("workspace").(*models.Workspace)
	user, _ := c.Get("user")
	var userID *int
	if u, ok := user.(*models.User); ok {
		userID = &u.ID
	}
	var req dto.CreateThreadRequest
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
			return
		}
	}
	thread, err := h.threadSvc.Create(c.Request.Context(), ws.ID, userID, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"thread": thread, "message": "Thread created"})
}

func (h *ThreadHandler) ListThreads(c *gin.Context) {
	ws := c.MustGet("workspace").(*models.Workspace)
	threads, err := h.threadSvc.List(c.Request.Context(), ws.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"threads": threads})
}

func (h *ThreadHandler) DeleteThread(c *gin.Context) {
	ws := c.MustGet("workspace").(*models.Workspace)
	threadSlug := c.Param("threadSlug")
	if err := h.threadSvc.Delete(c.Request.Context(), ws.ID, threadSlug); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *ThreadHandler) BulkDeleteThreads(c *gin.Context) {
	ws := c.MustGet("workspace").(*models.Workspace)
	var req struct {
		Slugs []string `json:"slugs"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}
	if err := h.threadSvc.BulkDelete(c.Request.Context(), ws.ID, req.Slugs); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *ThreadHandler) GetThreadChats(c *gin.Context) {
	thread := c.MustGet("thread").(*models.WorkspaceThread)
	history, err := h.threadSvc.GetThreadChats(c.Request.Context(), thread.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"history": history})
}

func (h *ThreadHandler) UpdateThread(c *gin.Context) {
	thread := c.MustGet("thread").(*models.WorkspaceThread)
	var req dto.UpdateThreadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}
	if err := h.threadSvc.Update(c.Request.Context(), thread, req); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *ThreadHandler) DeleteThreadEditedChats(c *gin.Context) {
	thread := c.MustGet("thread").(*models.WorkspaceThread)
	if err := h.threadSvc.DeleteThreadEditedChats(c.Request.Context(), thread.ID); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *ThreadHandler) UpdateThreadChat(c *gin.Context) {
	chatID, err := strconv.Atoi(c.Param("chatId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid chat id"})
		return
	}
	var req dto.UpdateChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}
	if err := h.threadSvc.UpdateThreadChat(c.Request.Context(), chatID, req); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func RegisterThreadRoutes(r *gin.RouterGroup, threadSvc *services.ThreadService, authSvc *services.AuthService, db *gorm.DB) {
	h := NewThreadHandler(threadSvc)
	r.POST("/workspace/:slug/thread/new",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"all"}),
		middleware.ValidWorkspaceSlug(db),
		h.CreateThread)
	r.GET("/workspace/:slug/threads",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"all"}),
		middleware.ValidWorkspaceSlug(db),
		h.ListThreads)
	r.DELETE("/workspace/:slug/thread/:threadSlug",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"all"}),
		middleware.ValidWorkspaceSlug(db),
		middleware.ValidWorkspaceAndThreadSlug(db),
		h.DeleteThread)
	r.DELETE("/workspace/:slug/thread-bulk-delete",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"all"}),
		middleware.ValidWorkspaceSlug(db),
		h.BulkDeleteThreads)
	r.GET("/workspace/:slug/thread/:threadSlug/chats",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"all"}),
		middleware.ValidWorkspaceSlug(db),
		middleware.ValidWorkspaceAndThreadSlug(db),
		h.GetThreadChats)
	r.POST("/workspace/:slug/thread/:threadSlug/update",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"all"}),
		middleware.ValidWorkspaceSlug(db),
		middleware.ValidWorkspaceAndThreadSlug(db),
		h.UpdateThread)
	r.DELETE("/workspace/:slug/thread/:threadSlug/delete-edited-chats",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"all"}),
		middleware.ValidWorkspaceSlug(db),
		middleware.ValidWorkspaceAndThreadSlug(db),
		h.DeleteThreadEditedChats)
	r.POST("/workspace/:slug/thread/:threadSlug/update-chat",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"all"}),
		middleware.ValidWorkspaceSlug(db),
		middleware.ValidWorkspaceAndThreadSlug(db),
		h.UpdateThreadChat)
}
