package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/middleware"
	"github.com/odysseythink/hermind/backend/internal/services"
)

type APIUserHandler struct {
	adminSvc *services.AdminService
	tempSvc  *services.TemporaryAuthTokenService
	cfg      *config.Config
}

func NewAPIUserHandler(adminSvc *services.AdminService, tempSvc *services.TemporaryAuthTokenService, cfg *config.Config) *APIUserHandler {
	return &APIUserHandler{adminSvc: adminSvc, tempSvc: tempSvc, cfg: cfg}
}

func (h *APIUserHandler) List(c *gin.Context) {
	if !apiV1RequireMultiUser(c, h.cfg) {
		return
	}
	users, err := h.adminSvc.ListUsers(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	out := make([]gin.H, 0, len(users))
	for _, u := range users {
		username := ""
		if u.Username != nil {
			username = *u.Username
		}
		out = append(out, gin.H{"id": u.ID, "username": username, "role": u.Role})
	}
	c.JSON(http.StatusOK, gin.H{"users": out})
}

func (h *APIUserHandler) IssueAuthToken(c *gin.Context) {
	if !apiV1RequireMultiUser(c, h.cfg) {
		return
	}
	if !h.cfg.SimpleSSOEnabled {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"error": "Simple SSO is not enabled on this instance.",
		})
		return
	}
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}
	user, err := h.adminSvc.GetUserByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}
	token, err := h.tempSvc.Issue(c.Request.Context(), user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"token":     token,
		"loginPath": "/sso/simple?token=" + token,
	})
}

func RegisterAPIUserRoutes(r *gin.RouterGroup, apiKeySvc *services.APIKeyService, adminSvc *services.AdminService, tempSvc *services.TemporaryAuthTokenService, cfg *config.Config) {
	h := NewAPIUserHandler(adminSvc, tempSvc, cfg)
	r.GET("/v1/users", middleware.ValidAPIKey(apiKeySvc), h.List)
	r.GET("/v1/users/:id/issue-auth-token", middleware.ValidAPIKey(apiKeySvc), h.IssueAuthToken)
}
