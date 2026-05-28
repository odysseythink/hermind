package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/dto"
	"github.com/odysseythink/hermind/backend/internal/middleware"
	"github.com/odysseythink/hermind/backend/internal/services"
	"gorm.io/gorm"
)

type APIEmbedHandler struct {
	embedSvc *services.EmbedService
}

func NewAPIEmbedHandler(embedSvc *services.EmbedService) *APIEmbedHandler {
	return &APIEmbedHandler{embedSvc: embedSvc}
}

func (h *APIEmbedHandler) ListEmbedConfigs(c *gin.Context) {
	embeds, err := h.embedSvc.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, dto.EmbedConfigListResponse{Embeds: embeds})
}

func (h *APIEmbedHandler) ListEmbedChats(c *gin.Context) {
	embedUUID := c.Param("embedUuid")
	embed, err := h.embedSvc.GetByUUID(c.Request.Context(), embedUUID)
	if err != nil {
		c.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "Embed not found"})
		return
	}
	chats, err := h.embedSvc.ListChats(c.Request.Context(), embed.ID, nil, 0, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	var safeChats []gin.H
	for _, chat := range chats {
		safeChats = append(safeChats, gin.H{
			"id":        chat.ID,
			"prompt":    chat.Prompt,
			"response":  chat.Response,
			"sessionId": chat.SessionID,
			"createdAt": chat.CreatedAt,
		})
	}
	c.JSON(http.StatusOK, gin.H{"chats": safeChats})
}

func (h *APIEmbedHandler) ListSessionChats(c *gin.Context) {
	embedUUID := c.Param("embedUuid")
	sessionUUID := c.Param("sessionUuid")
	embed, err := h.embedSvc.GetByUUID(c.Request.Context(), embedUUID)
	if err != nil {
		c.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "Embed not found"})
		return
	}
	chats, err := h.embedSvc.ListChats(c.Request.Context(), embed.ID, &sessionUUID, 0, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	var safeChats []gin.H
	for _, chat := range chats {
		safeChats = append(safeChats, gin.H{
			"id":        chat.ID,
			"prompt":    chat.Prompt,
			"response":  chat.Response,
			"sessionId": chat.SessionID,
			"createdAt": chat.CreatedAt,
		})
	}
	c.JSON(http.StatusOK, gin.H{"chats": safeChats})
}

func (h *APIEmbedHandler) CreateEmbedConfig(c *gin.Context) {
	var req dto.CreateEmbedConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}
	embed, err := h.embedSvc.Create(c.Request.Context(), req, nil)
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"embed": embed})
}

func (h *APIEmbedHandler) UpdateEmbedConfig(c *gin.Context) {
	embedUUID := c.Param("embedUuid")
	embed, err := h.embedSvc.GetByUUID(c.Request.Context(), embedUUID)
	if err != nil {
		c.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "Embed not found"})
		return
	}
	var req dto.UpdateEmbedConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}
	if err := h.embedSvc.Update(c.Request.Context(), embed.ID, req); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *APIEmbedHandler) DeleteEmbedConfig(c *gin.Context) {
	embedUUID := c.Param("embedUuid")
	embed, err := h.embedSvc.GetByUUID(c.Request.Context(), embedUUID)
	if err != nil {
		c.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "Embed not found"})
		return
	}
	if err := h.embedSvc.Delete(c.Request.Context(), embed.ID); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func RegisterAPIEmbedRoutes(r *gin.RouterGroup, svc *services.EmbedService, apiKeySvc *services.APIKeyService, db *gorm.DB) {
	h := NewAPIEmbedHandler(svc)
	r.GET("/v1/embed",
		middleware.ValidAPIKey(apiKeySvc),
		h.ListEmbedConfigs)
	r.GET("/v1/embed/:embedUuid/chats",
		middleware.ValidAPIKey(apiKeySvc),
		h.ListEmbedChats)
	r.GET("/v1/embed/:embedUuid/chats/:sessionUuid",
		middleware.ValidAPIKey(apiKeySvc),
		h.ListSessionChats)
	r.POST("/v1/embed/new",
		middleware.ValidAPIKey(apiKeySvc),
		h.CreateEmbedConfig)
	r.POST("/v1/embed/:embedUuid",
		middleware.ValidAPIKey(apiKeySvc),
		h.UpdateEmbedConfig)
	r.DELETE("/v1/embed/:embedUuid",
		middleware.ValidAPIKey(apiKeySvc),
		h.DeleteEmbedConfig)
}
