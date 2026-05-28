package handlers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/middleware"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/services"
)

// AgentSkillWhitelistHandler exposes CRUD for the per-user skill whitelist.
type AgentSkillWhitelistHandler struct {
	svc *services.AgentSkillWhitelistService
}

// NewAgentSkillWhitelistHandler creates a handler backed by the whitelist service.
func NewAgentSkillWhitelistHandler(svc *services.AgentSkillWhitelistService) *AgentSkillWhitelistHandler {
	return &AgentSkillWhitelistHandler{svc: svc}
}

// RegisterAgentSkillWhitelistRoutes wires the whitelist HTTP surface.
func RegisterAgentSkillWhitelistRoutes(r *gin.RouterGroup, h *AgentSkillWhitelistHandler, authSvc *services.AuthService) {
	g := r.Group("/agent-skill-whitelist")
	g.Use(middleware.ValidatedRequest(authSvc))
	g.GET("", h.Get)
	g.POST("", h.Add)
	g.DELETE("/:skill", h.Remove)

	r.GET("/admin/agent-skill-whitelist/:userId",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"admin"}),
		h.AdminGetForUser,
	)
}

func (h *AgentSkillWhitelistHandler) Get(c *gin.Context) {
	user := c.MustGet("user").(*models.User)
	list, err := h.svc.Get(c.Request.Context(), &user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"skills": list})
}

type addSkillReq struct {
	Skill string `json:"skill" binding:"required"`
}

func (h *AgentSkillWhitelistHandler) Add(c *gin.Context) {
	user := c.MustGet("user").(*models.User)
	var req addSkillReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	if err := h.svc.Add(c.Request.Context(), &user.ID, req.Skill); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *AgentSkillWhitelistHandler) Remove(c *gin.Context) {
	user := c.MustGet("user").(*models.User)
	skill := c.Param("skill")
	if err := h.svc.Remove(c.Request.Context(), &user.ID, skill); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *AgentSkillWhitelistHandler) AdminGetForUser(c *gin.Context) {
	var uid int
	if _, err := fmt.Sscanf(c.Param("userId"), "%d", &uid); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}
	list, err := h.svc.Get(c.Request.Context(), &uid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"skills": list})
}
