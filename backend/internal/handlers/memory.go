package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/dto"
	"github.com/odysseythink/hermind/backend/internal/middleware"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/services"
)

type MemoryHandler struct {
	mem *services.MemoryService
	ws  *services.WorkspaceService
}

func NewMemoryHandler(mem *services.MemoryService, ws *services.WorkspaceService) *MemoryHandler {
	return &MemoryHandler{mem: mem, ws: ws}
}

type createMemReq struct {
	Scope       string `json:"scope"`
	WorkspaceID *int   `json:"workspaceId"`
	Content     string `json:"content"`
}

func userIDFromCtx(c *gin.Context) *int {
	v, ok := c.Get("user")
	if !ok {
		return nil
	}
	u, ok := v.(*models.User)
	if !ok {
		return nil
	}
	return &u.ID
}

func (h *MemoryHandler) Create(c *gin.Context) {
	var req createMemReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}
	m, err := h.mem.Create(c.Request.Context(), userIDFromCtx(c), req.WorkspaceID, req.Scope, req.Content)
	if err != nil {
		if errors.Is(err, services.ErrMemoryLimitReached) {
			c.JSON(http.StatusConflict, dto.ErrorResponse{Error: err.Error()})
			return
		}
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"memory": m})
}

func (h *MemoryHandler) List(c *gin.Context) {
	slug := c.Param("slug")
	ws, err := h.ws.GetBySlug(c.Request.Context(), slug)
	if err != nil {
		c.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "workspace not found"})
		return
	}
	uid := userIDFromCtx(c)
	ws_mems, _ := h.mem.ListWorkspace(c.Request.Context(), uid, ws.ID)
	glob, _ := h.mem.ListGlobal(c.Request.Context(), uid)
	c.JSON(http.StatusOK, gin.H{"workspace": ws_mems, "global": glob})
}

type updateMemReq struct {
	Content string `json:"content"`
}

func (h *MemoryHandler) Update(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "bad id"})
		return
	}
	var req updateMemReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}
	m, err := h.mem.Update(c.Request.Context(), id, req.Content)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"memory": m})
}

func (h *MemoryHandler) Delete(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "bad id"})
		return
	}
	if err := h.mem.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *MemoryHandler) Promote(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "bad id"})
		return
	}
	m, err := h.mem.PromoteToGlobal(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, services.ErrMemoryLimitReached) {
			c.JSON(http.StatusConflict, dto.ErrorResponse{Error: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"memory": m})
}

func (h *MemoryHandler) Demote(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	wsID, _ := strconv.Atoi(c.Param("workspaceId"))
	m, err := h.mem.DemoteToWorkspace(c.Request.Context(), id, wsID)
	if err != nil {
		if errors.Is(err, services.ErrMemoryLimitReached) {
			c.JSON(http.StatusConflict, dto.ErrorResponse{Error: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"memory": m})
}

type replaceMemReq struct {
	Memories []string `json:"memories"`
}

func (h *MemoryHandler) ReplaceWorkspace(c *gin.Context) {
	slug := c.Param("slug")
	ws, err := h.ws.GetBySlug(c.Request.Context(), slug)
	if err != nil {
		c.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "workspace not found"})
		return
	}
	var req replaceMemReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}
	if err := h.mem.ReplaceWorkspace(c.Request.Context(), userIDFromCtx(c), ws.ID, req.Memories); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func RegisterMemoryRoutes(r *gin.RouterGroup, mem *services.MemoryService, ws *services.WorkspaceService, authSvc *services.AuthService) {
	h := NewMemoryHandler(mem, ws)
	g := r.Group("", middleware.ValidatedRequest(authSvc))
	g.GET("/memory/workspace/:slug", h.List)
	g.POST("/memory", h.Create)
	g.PATCH("/memory/:id", h.Update)
	g.DELETE("/memory/:id", h.Delete)
	g.POST("/memory/:id/promote", h.Promote)
	g.POST("/memory/:id/demote/:workspaceId", h.Demote)
	g.PUT("/memory/workspace/:slug/replace", h.ReplaceWorkspace)
}
