package handlers

import (
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/dto"
	"github.com/odysseythink/hermind/backend/internal/middleware"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/services"
)

type WebPushHandler struct {
	svc *services.WebPushService
}

func NewWebPushHandler(svc *services.WebPushService) *WebPushHandler {
	return &WebPushHandler{svc: svc}
}

func (h *WebPushHandler) Subscribe(c *gin.Context) {
	v, _ := c.Get("user")
	u, ok := v.(*models.User)
	if !ok {
		c.JSON(http.StatusUnauthorized, dto.ErrorResponse{Error: "auth required"})
		return
	}
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}
	if err := h.svc.RegisterSubscription(c.Request.Context(), u.ID, body); err != nil {
		status := http.StatusInternalServerError
		if isBadRequestErr(err) {
			status = http.StatusBadRequest
		}
		c.JSON(status, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{})
}

func (h *WebPushHandler) PubKey(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"publicKey": h.svc.PublicVAPIDKey()})
}

func isBadRequestErr(err error) bool {
	return errors.Is(err, services.ErrInvalidSubscription)
}

func RegisterWebPushRoutes(r *gin.RouterGroup, svc *services.WebPushService, authSvc *services.AuthService) {
	h := NewWebPushHandler(svc)
	g := r.Group("/web-push", middleware.ValidatedRequest(authSvc))
	g.POST("/subscribe", h.Subscribe)
	g.GET("/pubkey", h.PubKey)
}
