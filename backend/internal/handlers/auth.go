package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/dto"
	"github.com/odysseythink/hermind/backend/internal/middleware"
	"github.com/odysseythink/hermind/backend/internal/services"
	"github.com/odysseythink/hermind/backend/pkg/utils"
)

type AuthHandler struct {
	authSvc      *services.AuthService
	cfg          *config.Config
	eventLogSvc  *services.EventLogService
	tempTokenSvc *services.TemporaryAuthTokenService
}

func NewAuthHandler(authSvc *services.AuthService, cfg *config.Config, eventLogSvc *services.EventLogService, tempTokenSvc *services.TemporaryAuthTokenService) *AuthHandler {
	return &AuthHandler{authSvc: authSvc, cfg: cfg, eventLogSvc: eventLogSvc, tempTokenSvc: tempTokenSvc}
}

func (h *AuthHandler) RequestToken(c *gin.Context) {
	if !h.cfg.MultiUserMode {
		token, err := h.authSvc.CreateSingleUserToken(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
			return
		}
		c.JSON(http.StatusOK, dto.RequestTokenResponse{Token: token, Valid: true})
		return
	}

	if h.cfg.SimpleSSOEnabled && h.cfg.SimpleSSONoLogin {
		msg := "[005] Login via credentials has been disabled by the administrator."
		c.JSON(http.StatusForbidden, dto.RequestTokenResponse{Valid: false, Message: &msg})
		return
	}

	var req dto.RequestTokenMultiUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}

	resp, recoveryCodes, err := h.authSvc.RequestTokenMultiUser(c.Request.Context(), req.Username, req.Password, h.eventLogSvc)
	if err != nil {
		msg := err.Error()
		c.JSON(http.StatusOK, dto.RequestTokenResponse{Valid: false, Message: &msg})
		return
	}

	c.JSON(http.StatusOK, dto.RequestTokenResponse{
		Token:         resp.Token,
		Valid:         true,
		User:          resp.User,
		RecoveryCodes: recoveryCodes,
	})
}

func (h *AuthHandler) SSOSimple(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		msg := "missing token"
		c.JSON(http.StatusBadRequest, dto.RequestTokenResponse{Valid: false, Message: &msg})
		return
	}

	user, err := h.tempTokenSvc.Validate(c.Request.Context(), token)
	if err != nil {
		_ = h.eventLogSvc.LogEvent(c.Request.Context(), "failed_login_invalid_temporary_auth_token", map[string]any{"ip": "Unknown IP", "multiUserMode": true}, nil)
		msg := fmt.Sprintf("[001] An error occurred while validating the token: %s", err.Error())
		c.JSON(http.StatusUnauthorized, dto.RequestTokenResponse{Valid: false, Message: &msg})
		return
	}

	jwtToken, err := utils.GenerateJWT(h.cfg.JWTSecret, map[string]any{"userId": user.ID}, 24*time.Hour)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, dto.RequestTokenResponse{Token: jwtToken, Valid: true, User: user})
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req dto.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}
	resp, err := h.authSvc.Login(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusForbidden, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req dto.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}
	resp, err := h.authSvc.Register(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *AuthHandler) Logout(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *AuthHandler) RecoverAccount(c *gin.Context) {
	var req dto.RecoverAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}
	token, err := h.authSvc.RecoverAccount(c.Request.Context(), req.Username, req.RecoveryCodes)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "resetToken": token})
}

func (h *AuthHandler) ResetPassword(c *gin.Context) {
	var req dto.ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}
	if err := h.authSvc.ResetPassword(c.Request.Context(), req.Token, req.NewPassword, req.ConfirmPassword); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Password reset successful"})
}

func (h *AuthHandler) GetInvite(c *gin.Context) {
	code := c.Param("code")
	invite, err := h.authSvc.GetInvite(c.Request.Context(), code)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"invite": nil, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"invite": gin.H{"code": invite.Code, "status": invite.Status}, "error": nil})
}

func (h *AuthHandler) AcceptInvite(c *gin.Context) {
	code := c.Param("code")
	var req dto.AcceptInviteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}
	if err := h.authSvc.AcceptInvite(c.Request.Context(), code, req.Username, req.Password); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "error": nil})
}

func RegisterAuthRoutes(r *gin.RouterGroup, authSvc *services.AuthService, cfg *config.Config, eventLogSvc *services.EventLogService, tempTokenSvc *services.TemporaryAuthTokenService) {
	h := NewAuthHandler(authSvc, cfg, eventLogSvc, tempTokenSvc)
	r.POST("/request-token", h.RequestToken)
	r.POST("/login", h.Login)
	r.POST("/register", h.Register)
	r.POST("/logout", h.Logout)
	r.POST("/system/recover-account", middleware.IsMultiUserSetup(cfg), h.RecoverAccount)
	r.POST("/system/reset-password", middleware.IsMultiUserSetup(cfg), h.ResetPassword)
	r.GET("/invite/:code", h.GetInvite)
	r.POST("/invite/:code", h.AcceptInvite)
	r.GET("/request-token/sso/simple", middleware.IsMultiUserSetup(cfg), h.SSOSimple)
}
