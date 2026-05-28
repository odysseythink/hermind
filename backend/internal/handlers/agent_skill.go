package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/middleware"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/services"
)

type AgentSkillHandler struct {
	sysSvc *services.SystemService
}

func NewAgentSkillHandler(sysSvc *services.SystemService) *AgentSkillHandler {
	return &AgentSkillHandler{sysSvc: sysSvc}
}

func (h *AgentSkillHandler) FileSystemAgentAvailable(c *gin.Context) {
	// Go backend does not implement filesystem-agent plugin yet
	c.JSON(http.StatusOK, gin.H{"available": false})
}

func (h *AgentSkillHandler) CreateFilesAgentAvailable(c *gin.Context) {
	// Go backend does not implement create-files-agent plugin yet
	c.JSON(http.StatusOK, gin.H{"available": false})
}

func (h *AgentSkillHandler) AddToWhitelist(c *gin.Context) {
	var req struct {
		SkillName string `json:"skillName"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	if req.SkillName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Missing skillName"})
		return
	}

	user, _ := c.Get("user")
	var userID *int
	if u, ok := user.(*models.User); ok {
		userID = &u.ID
	}

	var label string
	if userID != nil {
		label = "user_" + strconv.Itoa(*userID) + "_whitelisted_agent_skills"
	} else {
		label = "whitelisted_agent_skills"
	}

	val, err := h.sysSvc.GetSetting(c.Request.Context(), label)
	if err != nil {
		val = ""
	}
	var list []string
	if val != "" {
		_ = json.Unmarshal([]byte(val), &list)
	}
	for _, s := range list {
		if s == req.SkillName {
			c.JSON(http.StatusOK, gin.H{"success": true, "error": nil})
			return
		}
	}
	list = append(list, req.SkillName)
	newVal, _ := json.Marshal(list)
	if err := h.sysSvc.SetSetting(c.Request.Context(), label, string(newVal)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "error": nil})
}

func RegisterAgentSkillRoutes(r *gin.RouterGroup, sysSvc *services.SystemService, authSvc *services.AuthService) {
	h := NewAgentSkillHandler(sysSvc)
	r.GET("/agent-skills/filesystem-agent/is-available",
		middleware.ValidatedRequest(authSvc),
		h.FileSystemAgentAvailable)
	r.GET("/agent-skills/create-files-agent/is-available",
		middleware.ValidatedRequest(authSvc),
		h.CreateFilesAgentAvailable)
	r.POST("/agent-skills/whitelist/add",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"all"}),
		h.AddToWhitelist)
}
