package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/dto"
	"github.com/odysseythink/hermind/backend/internal/middleware"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/services"
	"gorm.io/gorm"
)

type AgentSkillsHandler struct {
	skillSvc services.AgentSkillManager
}

func NewAgentSkillsHandler(skillSvc services.AgentSkillManager) *AgentSkillsHandler {
	return &AgentSkillsHandler{skillSvc: skillSvc}
}

// ListSkills lists all skills for a workspace.
func (h *AgentSkillsHandler) ListSkills(c *gin.Context) {
	ws := c.MustGet("workspace").(*models.Workspace)
	includeArchived := c.Query("include_archived") == "true"

	skills, err := h.skillSvc.List(c.Request.Context(), ws.ID, includeArchived)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}

	categories := make(map[string]bool)
	for _, s := range skills {
		if s.Category != "" {
			categories[s.Category] = true
		}
	}
	catList := make([]string, 0, len(categories))
	for c := range categories {
		catList = append(catList, c)
	}

	c.JSON(http.StatusOK, dto.AgentSkillListResponse{
		Skills:     skills,
		Categories: catList,
		Count:      len(skills),
	})
}

// GetSkill returns a single skill.
func (h *AgentSkillsHandler) GetSkill(c *gin.Context) {
	ws := c.MustGet("workspace").(*models.Workspace)
	skillSlug := c.Param("skillSlug")

	skill, err := h.skillSvc.GetBySlug(c.Request.Context(), ws.ID, skillSlug)
	if err != nil {
		c.JSON(http.StatusNotFound, dto.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, skill)
}

// CreateSkill creates a new skill.
func (h *AgentSkillsHandler) CreateSkill(c *gin.Context) {
	ws := c.MustGet("workspace").(*models.Workspace)
	var req dto.CreateAgentSkillRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}

	skill, err := h.skillSvc.Create(c.Request.Context(), ws.ID, req)
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "skill": skill})
}

// UpdateSkill updates a skill.
func (h *AgentSkillsHandler) UpdateSkill(c *gin.Context) {
	ws := c.MustGet("workspace").(*models.Workspace)
	skillSlug := c.Param("skillSlug")
	var req dto.UpdateAgentSkillRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}

	skill, err := h.skillSvc.Update(c.Request.Context(), ws.ID, skillSlug, req)
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "skill": skill})
}

// PatchSkill patches a skill's content.
func (h *AgentSkillsHandler) PatchSkill(c *gin.Context) {
	ws := c.MustGet("workspace").(*models.Workspace)
	skillSlug := c.Param("skillSlug")
	var req dto.PatchAgentSkillRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}

	skill, err := h.skillSvc.Patch(c.Request.Context(), ws.ID, skillSlug, req)
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "skill": skill})
}

// DeleteSkill deletes a skill.
func (h *AgentSkillsHandler) DeleteSkill(c *gin.Context) {
	ws := c.MustGet("workspace").(*models.Workspace)
	skillSlug := c.Param("skillSlug")

	if err := h.skillSvc.Delete(c.Request.Context(), ws.ID, skillSlug); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// WriteSkillFile writes a supporting file.
func (h *AgentSkillsHandler) WriteSkillFile(c *gin.Context) {
	ws := c.MustGet("workspace").(*models.Workspace)
	skillSlug := c.Param("skillSlug")
	var req dto.WriteSkillFileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}

	if err := h.skillSvc.WriteFile(c.Request.Context(), ws.ID, skillSlug, req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// RemoveSkillFile removes a supporting file.
func (h *AgentSkillsHandler) RemoveSkillFile(c *gin.Context) {
	ws := c.MustGet("workspace").(*models.Workspace)
	skillSlug := c.Param("skillSlug")
	filePath := strings.TrimPrefix(c.Param("filePath"), "/")

	if err := h.skillSvc.RemoveFile(c.Request.Context(), ws.ID, skillSlug, filePath); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// GetSkillFile returns a supporting file.
func (h *AgentSkillsHandler) GetSkillFile(c *gin.Context) {
	ws := c.MustGet("workspace").(*models.Workspace)
	skillSlug := c.Param("skillSlug")
	filePath := strings.TrimPrefix(c.Param("filePath"), "/")

	skill, err := h.skillSvc.GetBySlug(c.Request.Context(), ws.ID, skillSlug)
	if err != nil {
		c.JSON(http.StatusNotFound, dto.ErrorResponse{Error: err.Error()})
		return
	}

	file, err := h.skillSvc.GetFile(c.Request.Context(), skill.ID, filePath)
	if err != nil {
		c.JSON(http.StatusNotFound, dto.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, dto.AgentSkillFileResponse{
		FilePath: file.FilePath,
		Content:  file.Content,
	})
}

// ListSkillFiles lists all supporting files for a skill.
func (h *AgentSkillsHandler) ListSkillFiles(c *gin.Context) {
	ws := c.MustGet("workspace").(*models.Workspace)
	skillSlug := c.Param("skillSlug")

	skill, err := h.skillSvc.GetBySlug(c.Request.Context(), ws.ID, skillSlug)
	if err != nil {
		c.JSON(http.StatusNotFound, dto.ErrorResponse{Error: err.Error()})
		return
	}

	files, err := h.skillSvc.ListFiles(c.Request.Context(), skill.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"files": files})
}

// RegisterAgentSkillsRoutes registers the agent skills REST routes.
func RegisterAgentSkillsRoutes(r *gin.RouterGroup, skillSvc services.AgentSkillManager, authSvc *services.AuthService, db *gorm.DB) {
	h := NewAgentSkillsHandler(skillSvc)

	group := r.Group("/workspace/:slug/agent-skills")
	group.Use(middleware.ValidatedRequest(authSvc), middleware.FlexUserRoleValid([]string{"all"}), middleware.ValidWorkspaceSlug(db))

	group.GET("", h.ListSkills)
	group.POST("", h.CreateSkill)
	group.GET("/:skillSlug", h.GetSkill)
	group.PUT("/:skillSlug", h.UpdateSkill)
	group.PATCH("/:skillSlug", h.PatchSkill)
	group.DELETE("/:skillSlug", h.DeleteSkill)
	group.GET("/:skillSlug/files", h.ListSkillFiles)
	group.POST("/:skillSlug/files", h.WriteSkillFile)
	group.GET("/:skillSlug/files/*filePath", h.GetSkillFile)
	group.DELETE("/:skillSlug/files/*filePath", h.RemoveSkillFile)
}
