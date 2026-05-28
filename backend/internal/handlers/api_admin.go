package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/middleware"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/services"
)

type APIAdminHandler struct {
	adminSvc  *services.AdminService
	sysSvc    *services.SystemService
	wsSvc     *services.WorkspaceService
	wsChatSvc *services.WorkspaceChatService
	cfg       *config.Config
}

func NewAPIAdminHandler(adminSvc *services.AdminService, sysSvc *services.SystemService, wsSvc *services.WorkspaceService, wsChatSvc *services.WorkspaceChatService, cfg *config.Config) *APIAdminHandler {
	return &APIAdminHandler{adminSvc: adminSvc, sysSvc: sysSvc, wsSvc: wsSvc, wsChatSvc: wsChatSvc, cfg: cfg}
}

func (h *APIAdminHandler) IsMultiUserMode(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"isMultiUser": h.cfg.MultiUserMode})
}

func (h *APIAdminHandler) ListUsers(c *gin.Context) {
	if !apiV1RequireMultiUser(c, h.cfg) {
		return
	}
	users, err := h.adminSvc.ListUsers(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"users": filterUsersForAdminAPI(users)})
}

func filterUsersForAdminAPI(users []models.User) []gin.H {
	out := make([]gin.H, 0, len(users))
	for _, u := range users {
		username := ""
		if u.Username != nil {
			username = *u.Username
		}
		bio := ""
		if u.Bio != nil {
			bio = *u.Bio
		}
		out = append(out, gin.H{
			"id":        u.ID,
			"username":  username,
			"role":      u.Role,
			"bio":       bio,
			"suspended": u.Suspended,
		})
	}
	return out
}

func (h *APIAdminHandler) CreateUser(c *gin.Context) {
	if !apiV1RequireMultiUser(c, h.cfg) {
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	user, bizErr, sysErr := h.adminSvc.CreateUser(c.Request.Context(), services.CreateUserInput{
		Username: req.Username, Password: req.Password, Role: req.Role,
	})
	if sysErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": sysErr.Error()})
		return
	}
	if bizErr != "" {
		c.JSON(http.StatusOK, gin.H{"user": nil, "error": bizErr})
		return
	}
	c.JSON(http.StatusOK, gin.H{"user": user, "error": nil})
}

func (h *APIAdminHandler) UpdateUser(c *gin.Context) {
	if !apiV1RequireMultiUser(c, h.cfg) {
		return
	}
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var updates map[string]any
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	bizErr, sysErr := h.adminSvc.UpdateUser(c.Request.Context(), id, updates)
	if sysErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": sysErr.Error()})
		return
	}
	if bizErr != "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": bizErr})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "error": nil})
}

func (h *APIAdminHandler) DeleteUser(c *gin.Context) {
	if !apiV1RequireMultiUser(c, h.cfg) {
		return
	}
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := h.adminSvc.DeleteUser(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "error": nil})
}

func (h *APIAdminHandler) ListInvites(c *gin.Context) {
	if !apiV1RequireMultiUser(c, h.cfg) {
		return
	}
	invites, err := h.adminSvc.ListInvites(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"invites": invites})
}

func (h *APIAdminHandler) CreateInvite(c *gin.Context) {
	if !apiV1RequireMultiUser(c, h.cfg) {
		return
	}
	var req struct {
		WorkspaceIDs []int `json:"workspaceIds"`
	}
	_ = c.ShouldBindJSON(&req)
	inv, err := h.adminSvc.CreateInvite(c.Request.Context(), 0, req.WorkspaceIDs)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"invite": nil, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"invite": inv, "error": nil})
}

func (h *APIAdminHandler) DeactivateInvite(c *gin.Context) {
	if !apiV1RequireMultiUser(c, h.cfg) {
		return
	}
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := h.adminSvc.DeactivateInvite(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "error": nil})
}

func (h *APIAdminHandler) ListWorkspaceUsers(c *gin.Context) {
	if !apiV1RequireMultiUser(c, h.cfg) {
		return
	}
	wsID, err := strconv.Atoi(c.Param("workspaceId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid workspaceId"})
		return
	}
	users, err := h.wsSvc.ListWorkspaceUsers(c.Request.Context(), wsID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"users": users})
}

func (h *APIAdminHandler) UpdateWorkspaceUsers(c *gin.Context) {
	if !apiV1RequireMultiUser(c, h.cfg) {
		return
	}
	wsID, err := strconv.Atoi(c.Param("workspaceId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid workspaceId"})
		return
	}
	var req struct {
		UserIDs []int `json:"userIds"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.wsSvc.UpdateUsers(c.Request.Context(), wsID, req.UserIDs); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "error": nil})
}

func (h *APIAdminHandler) ManageWorkspaceUsers(c *gin.Context) {
	if !apiV1RequireMultiUser(c, h.cfg) {
		return
	}
	// Param may be int ID or string slug — try ID first, then slug.
	var ws *models.Workspace
	var err error
	if id, parseErr := strconv.Atoi(c.Param("workspaceId")); parseErr == nil {
		ws, err = h.wsSvc.GetByID(c.Request.Context(), id)
	} else {
		ws, err = h.wsSvc.GetBySlug(c.Request.Context(), c.Param("workspaceId"))
	}
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "workspace not found"})
		return
	}

	// Node parity: userIds may contain raw ints OR {username, password, role} objects.
	var req struct {
		UserIDs []json.RawMessage `json:"userIds"`
		Reset   bool              `json:"reset"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userIDs := make([]int, 0, len(req.UserIDs))
	for _, raw := range req.UserIDs {
		// Try int first.
		var id int
		if err := json.Unmarshal(raw, &id); err == nil {
			userIDs = append(userIDs, id)
			continue
		}
		// Otherwise treat as user-creation object.
		var userObj struct {
			Username string `json:"username"`
			Password string `json:"password"`
			Role     string `json:"role"`
		}
		if err := json.Unmarshal(raw, &userObj); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "userIds items must be integers or user objects"})
			return
		}
		if userObj.Username == "" || userObj.Password == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "User object requires username and password"})
			return
		}
		u, bizErr, sysErr := h.adminSvc.CreateUser(c.Request.Context(), services.CreateUserInput{
			Username: userObj.Username,
			Password: userObj.Password,
			Role:     userObj.Role,
		})
		if sysErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": sysErr.Error()})
			return
		}
		if bizErr != "" {
			c.JSON(http.StatusOK, gin.H{"success": false, "error": bizErr})
			return
		}
		userIDs = append(userIDs, u.ID)
	}

	if err := h.wsSvc.UpdateUsers(c.Request.Context(), ws.ID, userIDs); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "error": nil})
}

func (h *APIAdminHandler) WorkspaceChats(c *gin.Context) {
	if !apiV1RequireMultiUser(c, h.cfg) {
		return
	}
	var req struct {
		Offset int `json:"offset"`
	}
	_ = c.ShouldBindJSON(&req)
	const pageSize = 20
	chats, _, err := h.wsChatSvc.ListChats(c.Request.Context(), req.Offset*pageSize, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	total, err := h.wsChatSvc.CountChats(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	hasPages := total > int64((req.Offset+1)*pageSize)
	c.JSON(http.StatusOK, gin.H{"chats": chats, "hasPages": hasPages})
}

func (h *APIAdminHandler) UpdatePreferences(c *gin.Context) {
	if !apiV1RequireMultiUser(c, h.cfg) {
		return
	}
	var updates map[string]any
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	for k, v := range updates {
		if err := h.sysSvc.SetSetting(c.Request.Context(), k, fmt.Sprintf("%v", v)); err != nil {
			c.JSON(http.StatusOK, gin.H{"success": false, "error": err.Error()})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "error": nil})
}

func RegisterAPIAdminRoutes(r *gin.RouterGroup, apiKeySvc *services.APIKeyService, adminSvc *services.AdminService, sysSvc *services.SystemService, wsSvc *services.WorkspaceService, wsChatSvc *services.WorkspaceChatService, cfg *config.Config) {
	h := NewAPIAdminHandler(adminSvc, sysSvc, wsSvc, wsChatSvc, cfg)

	r.GET("/v1/admin/is-multi-user-mode", middleware.ValidAPIKey(apiKeySvc), h.IsMultiUserMode)
	r.GET("/v1/admin/users", middleware.ValidAPIKey(apiKeySvc), h.ListUsers)
	r.POST("/v1/admin/users/new", middleware.ValidAPIKey(apiKeySvc), h.CreateUser)
	r.POST("/v1/admin/users/:id", middleware.ValidAPIKey(apiKeySvc), h.UpdateUser)
	r.DELETE("/v1/admin/users/:id", middleware.ValidAPIKey(apiKeySvc), h.DeleteUser)
	r.GET("/v1/admin/invites", middleware.ValidAPIKey(apiKeySvc), h.ListInvites)
	r.POST("/v1/admin/invite/new", middleware.ValidAPIKey(apiKeySvc), h.CreateInvite)
	r.DELETE("/v1/admin/invite/:id", middleware.ValidAPIKey(apiKeySvc), h.DeactivateInvite)
	r.GET("/v1/admin/workspaces/:workspaceId/users", middleware.ValidAPIKey(apiKeySvc), h.ListWorkspaceUsers)
	r.POST("/v1/admin/workspaces/:workspaceId/update-users", middleware.ValidAPIKey(apiKeySvc), h.UpdateWorkspaceUsers)
	r.POST("/v1/admin/workspaces/:workspaceId/manage-users", middleware.ValidAPIKey(apiKeySvc), h.ManageWorkspaceUsers)
	r.POST("/v1/admin/workspace-chats", middleware.ValidAPIKey(apiKeySvc), h.WorkspaceChats)
	r.POST("/v1/admin/preferences", middleware.ValidAPIKey(apiKeySvc), h.UpdatePreferences)
}
