package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/middleware"
	"github.com/odysseythink/hermind/backend/internal/services"
)

type TelegramHandler struct {
	cfg   *config.Config
	tgSvc *services.TelegramBotService
}

func NewTelegramHandler(cfg *config.Config, tgSvc *services.TelegramBotService) *TelegramHandler {
	return &TelegramHandler{cfg: cfg, tgSvc: tgSvc}
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
	cfg, err := h.tgSvc.GetConfig(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"config": nil, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"config": cfg})
}

type connectRequest struct {
	BotToken string `json:"bot_token" binding:"required"`
}

func (h *TelegramHandler) Connect(c *gin.Context) {
	var req connectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	if err := h.tgSvc.Start(c.Request.Context(), req.BotToken); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": err.Error(), "bot_username": nil})
		return
	}
	cfg, _ := h.tgSvc.GetConfig(c.Request.Context())
	username := ""
	if cfg != nil {
		username = cfg.BotUsername
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "error": nil, "bot_username": username})
}

func (h *TelegramHandler) Disconnect(c *gin.Context) {
	if err := h.tgSvc.Stop(c.Request.Context()); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *TelegramHandler) Status(c *gin.Context) {
	active, username := h.tgSvc.Status()
	c.JSON(http.StatusOK, gin.H{"active": active, "bot_username": username})
}

func (h *TelegramHandler) PendingUsers(c *gin.Context) {
	users := h.tgSvc.PendingUsers()
	c.JSON(http.StatusOK, gin.H{"users": users})
}

func (h *TelegramHandler) ApprovedUsers(c *gin.Context) {
	users := h.tgSvc.ApprovedUsers()
	c.JSON(http.StatusOK, gin.H{"users": users})
}

type approveUserRequest struct {
	ChatID   string `json:"chatId" binding:"required"`
	Username string `json:"username"`
}

func (h *TelegramHandler) ApproveUser(c *gin.Context) {
	var req approveUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	if err := h.tgSvc.ApproveUser(c.Request.Context(), req.ChatID, req.Username); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "error": nil})
}

func (h *TelegramHandler) DenyUser(c *gin.Context) {
	var req approveUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	if err := h.tgSvc.DenyUser(req.ChatID); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "error": nil})
}

type updateConfigRequest struct {
	DefaultWorkspace  string `json:"default_workspace"`
	VoiceResponseMode string `json:"voice_response_mode"`
}

func (h *TelegramHandler) UpdateConfig(c *gin.Context) {
	var req updateConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	if err := h.tgSvc.UpdateConfig(c.Request.Context(), req.DefaultWorkspace, req.VoiceResponseMode); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "error": nil})
}

func (h *TelegramHandler) RevokeUser(c *gin.Context) {
	var req approveUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	if err := h.tgSvc.RevokeUser(c.Request.Context(), req.ChatID); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "error": nil})
}

func RegisterTelegramRoutes(r *gin.RouterGroup, cfg *config.Config, authSvc *services.AuthService, tgSvc *services.TelegramBotService) {
	h := NewTelegramHandler(cfg, tgSvc)
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
