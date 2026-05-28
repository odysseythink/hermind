package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/middleware"
	"github.com/odysseythink/hermind/backend/internal/services"
)

type TelegramHandler struct {
	cfg *config.Config
}

func NewTelegramHandler(cfg *config.Config) *TelegramHandler {
	return &TelegramHandler{cfg: cfg}
}

func singleUserMode(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if cfg.MultiUserMode {
			c.JSON(http.StatusForbidden, gin.H{"error": "Telegram is only available in single-user mode"})
			c.Abort()
			return
		}
		c.Next()
	}
}

func (h *TelegramHandler) GetConfig(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"config": nil})
}

func (h *TelegramHandler) Connect(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": false, "error": "Telegram bot not implemented in Go server", "bot_username": nil})
}

func (h *TelegramHandler) Disconnect(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *TelegramHandler) Status(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"active": false, "bot_username": nil})
}

func (h *TelegramHandler) PendingUsers(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"users": []any{}})
}

func (h *TelegramHandler) ApprovedUsers(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"users": []any{}})
}

func (h *TelegramHandler) ApproveUser(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true, "error": nil})
}

func (h *TelegramHandler) DenyUser(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true, "error": nil})
}

func (h *TelegramHandler) UpdateConfig(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true, "error": nil})
}

func (h *TelegramHandler) RevokeUser(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true, "error": nil})
}

func RegisterTelegramRoutes(r *gin.RouterGroup, cfg *config.Config, authSvc *services.AuthService) {
	h := NewTelegramHandler(cfg)
	sum := singleUserMode(cfg)

	r.GET("/telegram/config", middleware.ValidatedRequest(authSvc), sum, h.GetConfig)
	r.POST("/telegram/connect", middleware.ValidatedRequest(authSvc), sum, h.Connect)
	r.POST("/telegram/disconnect", middleware.ValidatedRequest(authSvc), sum, h.Disconnect)
	r.GET("/telegram/status", middleware.ValidatedRequest(authSvc), sum, h.Status)
	r.GET("/telegram/pending-users", middleware.ValidatedRequest(authSvc), sum, h.PendingUsers)
	r.GET("/telegram/approved-users", middleware.ValidatedRequest(authSvc), sum, h.ApprovedUsers)
	r.POST("/telegram/approve-user", middleware.ValidatedRequest(authSvc), sum, h.ApproveUser)
	r.POST("/telegram/deny-user", middleware.ValidatedRequest(authSvc), sum, h.DenyUser)
	r.POST("/telegram/update-config", middleware.ValidatedRequest(authSvc), sum, h.UpdateConfig)
	r.POST("/telegram/revoke-user", middleware.ValidatedRequest(authSvc), sum, h.RevokeUser)
}
