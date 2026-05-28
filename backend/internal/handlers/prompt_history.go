package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/dto"
	"github.com/odysseythink/hermind/backend/internal/middleware"
	"github.com/odysseythink/hermind/backend/internal/services"
	"gorm.io/gorm"
)

type PromptHistoryHandler struct {
	phSvc *services.PromptHistoryService
	wsSvc *services.WorkspaceService
}

func NewPromptHistoryHandler(phSvc *services.PromptHistoryService, wsSvc *services.WorkspaceService) *PromptHistoryHandler {
	return &PromptHistoryHandler{phSvc: phSvc, wsSvc: wsSvc}
}

func (h *PromptHistoryHandler) List(c *gin.Context) {
	slug := c.Param("slug")
	ws, err := h.wsSvc.GetBySlug(c.Request.Context(), slug)
	if err != nil {
		c.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "workspace not found"})
		return
	}
	rows, err := h.phSvc.ListByWorkspace(c.Request.Context(), ws.ID, 50)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"history": rows})
}

func (h *PromptHistoryHandler) DeleteAll(c *gin.Context) {
	slug := c.Param("slug")
	ws, err := h.wsSvc.GetBySlug(c.Request.Context(), slug)
	if err != nil {
		c.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "workspace not found"})
		return
	}
	if err := h.phSvc.DeleteAll(c.Request.Context(), ws.ID); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *PromptHistoryHandler) DeleteOne(c *gin.Context) {
	slug := c.Param("slug")
	ws, err := h.wsSvc.GetBySlug(c.Request.Context(), slug)
	if err != nil {
		c.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "workspace not found"})
		return
	}
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid id"})
		return
	}
	// Scope check: verify the row belongs to this workspace.
	rows, err := h.phSvc.ListByWorkspace(c.Request.Context(), ws.ID, 50)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	found := false
	for _, r := range rows {
		if r.ID == id {
			found = true
			break
		}
	}
	if !found {
		c.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "prompt history entry not found in workspace"})
		return
	}
	if err := h.phSvc.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func RegisterPromptHistoryRoutes(r *gin.RouterGroup, phSvc *services.PromptHistoryService, wsSvc *services.WorkspaceService, authSvc *services.AuthService, db *gorm.DB) {
	h := NewPromptHistoryHandler(phSvc, wsSvc)
	r.GET("/workspace/:slug/prompt-history",
		middleware.ValidatedRequest(authSvc),
		middleware.ValidWorkspaceSlug(db),
		h.List)
	r.DELETE("/workspace/:slug/prompt-history",
		middleware.ValidatedRequest(authSvc),
		middleware.ValidWorkspaceSlug(db),
		h.DeleteAll)
	r.DELETE("/workspace/:slug/prompt-history/:id",
		middleware.ValidatedRequest(authSvc),
		middleware.ValidWorkspaceSlug(db),
		h.DeleteOne)
}
